package eventbus

import (
	"sync"
)

// maxHistorySize is the maximum number of events retained in the in-memory
// history ring. Older events are discarded when this limit is exceeded.
const maxHistorySize = 1000

// EventBus is a thread-safe, in-process publish-subscribe event router.
// Components publish Events which are delivered to all Subscribers whose filter
// matches the event type. Delivery is non-blocking: if a subscriber's channel
// buffer is full the event is silently dropped for that subscriber to prevent
// a slow consumer from stalling the entire bus.
type EventBus struct {
	subscribers map[string]*Subscriber
	mu          sync.RWMutex
	history     []Event
	running     bool
}

// NewEventBus creates an EventBus ready for use. Call Start before publishing
// events in production; however Publish will work regardless of the running flag
// so that unit tests can operate without the lifecycle methods.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]*Subscriber),
		history:     make([]Event, 0, maxHistorySize),
		running:     false,
	}
}

// Subscribe registers a new Subscriber that receives only events matching the
// given filter type. The returned Subscriber's Channel can be ranged over to
// consume events. Call Unsubscribe or Subscriber.Close when done.
func (eb *EventBus) Subscribe(filter EventType) *Subscriber {
	sub := NewSubscriber(filter)
	eb.mu.Lock()
	eb.subscribers[sub.ID] = sub
	eb.mu.Unlock()
	return sub
}

// SubscribeAll registers a broadcast Subscriber that receives every event
// regardless of type.
func (eb *EventBus) SubscribeAll() *Subscriber {
	sub := NewBroadcastSubscriber()
	eb.mu.Lock()
	eb.subscribers[sub.ID] = sub
	eb.mu.Unlock()
	return sub
}

// Unsubscribe removes a subscriber by ID and closes its channel. If the
// subscriber does not exist the call is a no-op.
func (eb *EventBus) Unsubscribe(subscriberID string) {
	eb.mu.Lock()
	if sub, ok := eb.subscribers[subscriberID]; ok {
		if sub.Active {
			sub.Close()
		}
		delete(eb.subscribers, subscriberID)
	}
	eb.mu.Unlock()
}

// Publish sends an event to every matching subscriber using a non-blocking send.
// A subscriber matches if its filter is empty (broadcast) or equals the event's
// Type. Targeted events (non-empty Target) are additionally restricted to the
// subscriber whose ID matches the target.
//
// The event is also appended to the in-memory history, which is trimmed to the
// most recent maxHistorySize entries.
func (eb *EventBus) Publish(event Event) {
	eb.mu.Lock()
	// Append to history, trimming if necessary.
	eb.history = append(eb.history, event)
	if len(eb.history) > maxHistorySize {
		// Keep the most recent maxHistorySize events.
		excess := len(eb.history) - maxHistorySize
		eb.history = eb.history[excess:]
	}

	// Snapshot subscriber list under lock to minimise hold time.
	subs := make([]*Subscriber, 0, len(eb.subscribers))
	for _, sub := range eb.subscribers {
		subs = append(subs, sub)
	}
	eb.mu.Unlock()

	// Deliver outside the write lock to prevent deadlocks.
	for _, sub := range subs {
		if !sub.Active {
			continue
		}
		// If the event is targeted, only deliver to the matching subscriber.
		if event.Target != "" && sub.ID != event.Target {
			continue
		}
		// Filter check: empty filter means broadcast subscriber.
		if sub.Filter != "" && sub.Filter != event.Type {
			continue
		}
		// Non-blocking send — drop if the subscriber's buffer is full.
		select {
		case sub.Channel <- event:
		default:
		}
	}
}

// History returns the last n events from the in-memory history. If n exceeds
// the number of stored events, all available events are returned. The returned
// slice is a copy and safe to mutate.
func (eb *EventBus) History(n int) []Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	total := len(eb.history)
	if n > total {
		n = total
	}
	if n <= 0 {
		return nil
	}
	result := make([]Event, n)
	copy(result, eb.history[total-n:])
	return result
}

// HistoryByType returns the last n events that match the given eventType. The
// returned slice is a copy.
func (eb *EventBus) HistoryByType(eventType EventType, n int) []Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	var matches []Event
	// Walk backwards to collect the most recent matches first.
	for i := len(eb.history) - 1; i >= 0 && len(matches) < n; i-- {
		if eb.history[i].Type == eventType {
			matches = append(matches, eb.history[i])
		}
	}
	// Reverse so the oldest of the selected events comes first.
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}
	return matches
}

// Start marks the EventBus as running. In the current implementation the bus
// operates synchronously within Publish, so Start simply flips the flag. Future
// versions may spin up background workers here.
func (eb *EventBus) Start() {
	eb.mu.Lock()
	eb.running = true
	eb.mu.Unlock()
}

// Stop marks the EventBus as stopped and closes all active subscribers.
func (eb *EventBus) Stop() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.running = false
	for id, sub := range eb.subscribers {
		if sub.Active {
			sub.Close()
		}
		delete(eb.subscribers, id)
	}
}
