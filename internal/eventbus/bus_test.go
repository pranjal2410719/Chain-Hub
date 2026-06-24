package eventbus

import (
	"sync"
	"testing"
	"time"
)

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := NewEventBus()
	bus.Start()
	defer bus.Stop()

	// Subscribe to a specific event type.
	sub := bus.Subscribe(EventTaskAssigned)
	defer bus.Unsubscribe(sub.ID)

	// Subscribe to all events.
	subAll := bus.SubscribeAll()
	defer bus.Unsubscribe(subAll.ID)

	// Publish matching event.
	payload := map[string]interface{}{"task": "test-task"}
	evt := NewEvent(EventTaskAssigned, "test-source", payload)
	bus.Publish(evt)

	// Verify specific subscriber received it.
	select {
	case received := <-sub.Channel:
		if received.ID != evt.ID {
			t.Errorf("expected event ID %s, got %s", evt.ID, received.ID)
		}
		if received.Payload["task"] != "test-task" {
			t.Errorf("expected payload task 'test-task', got %v", received.Payload["task"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event on specific subscriber")
	}

	// Verify broadcast subscriber received it.
	select {
	case received := <-subAll.Channel:
		if received.ID != evt.ID {
			t.Errorf("expected event ID %s on broadcast, got %s", evt.ID, received.ID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event on broadcast subscriber")
	}

	// Publish non-matching event.
	evt2 := NewEvent(EventTaskCompleted, "test-source", nil)
	bus.Publish(evt2)

	// Verify specific subscriber did NOT receive it.
	select {
	case received := <-sub.Channel:
		t.Fatalf("unexpected event received on specific subscriber: %v", received.Type)
	case <-time.After(100 * time.Millisecond):
		// Expected timeout.
	}

	// Verify broadcast subscriber DID receive it.
	select {
	case received := <-subAll.Channel:
		if received.ID != evt2.ID {
			t.Errorf("expected event ID %s on broadcast, got %s", evt2.ID, received.ID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event2 on broadcast subscriber")
	}
}

func TestEventBus_History(t *testing.T) {
	bus := NewEventBus()
	bus.Start()
	defer bus.Stop()

	evt1 := NewEvent(EventTaskAssigned, "src", nil)
	evt2 := NewEvent(EventTaskCompleted, "src", nil)
	evt3 := NewEvent(EventTaskAssigned, "src", nil)

	bus.Publish(evt1)
	bus.Publish(evt2)
	bus.Publish(evt3)

	history := bus.History(5)
	if len(history) != 3 {
		t.Errorf("expected history size 3, got %d", len(history))
	}

	historyByType := bus.HistoryByType(EventTaskAssigned, 5)
	if len(historyByType) != 2 {
		t.Errorf("expected 2 task.assigned events, got %d", len(historyByType))
	}
}

func TestEventBus_NonBlocking(t *testing.T) {
	bus := NewEventBus()
	bus.Start()
	defer bus.Stop()

	// Create a subscriber but do not read from its channel.
	// Since channel buffer is 100, we publish 105 events.
	// It should not block the publisher.
	sub := bus.Subscribe(EventTaskAssigned)
	defer bus.Unsubscribe(sub.ID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 105; i++ {
			bus.Publish(NewEvent(EventTaskAssigned, "test", nil))
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Succeeded without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("publisher blocked by slow subscriber")
	}
}
