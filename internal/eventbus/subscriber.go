package eventbus

import (
	"github.com/google/uuid"
)

// subscriberBufferSize is the default capacity of every subscriber's event channel.
// A generous buffer prevents slow consumers from blocking the publisher while
// still bounding memory usage.
const subscriberBufferSize = 100

// Subscriber represents a single consumer registered with the EventBus.
// Each subscriber has a buffered channel on which matching events are delivered.
type Subscriber struct {
	// ID uniquely identifies this subscriber (UUID v4).
	ID string
	// Filter restricts which event types are delivered. An empty string means
	// the subscriber receives every event (broadcast mode).
	Filter EventType
	// Channel is the buffered channel on which matching events are sent.
	Channel chan Event
	// Active indicates whether the subscriber is still listening. Once set to
	// false the EventBus will skip delivery and the channel will be closed.
	Active bool
}

// NewSubscriber creates a Subscriber that only receives events matching the
// given filter type. The channel is created with a buffer of 100 events.
func NewSubscriber(filter EventType) *Subscriber {
	return &Subscriber{
		ID:      uuid.New().String(),
		Filter:  filter,
		Channel: make(chan Event, subscriberBufferSize),
		Active:  true,
	}
}

// NewBroadcastSubscriber creates a Subscriber that receives every event
// regardless of type. Internally the filter is set to the empty string.
func NewBroadcastSubscriber() *Subscriber {
	return &Subscriber{
		ID:      uuid.New().String(),
		Filter:  "",
		Channel: make(chan Event, subscriberBufferSize),
		Active:  true,
	}
}

// Close marks the subscriber as inactive and closes its event channel.
// Subsequent sends from the EventBus will be silently skipped.
func (s *Subscriber) Close() {
	s.Active = false
	close(s.Channel)
}
