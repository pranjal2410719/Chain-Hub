// Package eventbus provides a publish-subscribe event system for inter-component
// communication within the ChainHub orchestrator. Events are the primary mechanism
// for decoupled coordination between the pipeline engine, tool adapters, and the
// monitoring subsystem.
package eventbus

import (
	"time"

	"github.com/google/uuid"
)

// EventType represents the category of an event flowing through the bus.
type EventType string

const (
	// EventTaskAssigned is emitted when a task is delegated to a tool.
	EventTaskAssigned EventType = "task.assigned"
	// EventTaskCompleted is emitted when a tool finishes its assigned task.
	EventTaskCompleted EventType = "task.completed"
	// EventContextUpdated is emitted when shared context is modified.
	EventContextUpdated EventType = "context.updated"
	// EventReviewRequested is emitted when a phase output needs human review.
	EventReviewRequested EventType = "review.requested"
	// EventInputNeeded is emitted when user input is required to proceed.
	EventInputNeeded EventType = "input.needed"
	// EventInputResponse is emitted when user provides input in response to a prompt.
	EventInputResponse EventType = "input.response"
	// EventSystemAlert is emitted for system-level alerts (resource usage, errors).
	EventSystemAlert EventType = "system.alert"
	// EventToolOutput is emitted when a tool produces output.
	EventToolOutput EventType = "tool.output"
	// EventToolError is emitted when a tool encounters an error.
	EventToolError EventType = "tool.error"
	// EventToolStatusChanged is emitted when a tool's operational status changes.
	EventToolStatusChanged EventType = "tool.status_changed"
	// EventPipelinePhaseChanged is emitted when the pipeline transitions between phases.
	EventPipelinePhaseChanged EventType = "pipeline.phase_changed"
	// EventUserNotification is emitted to surface information to the user.
	EventUserNotification EventType = "user.notification"
	// EventAutopilotToggle is emitted when autopilot mode is toggled.
	EventAutopilotToggle EventType = "autopilot.toggle"
)

// Event represents a single occurrence within the ChainHub system. Events carry
// a typed payload and are routed through the EventBus to interested subscribers.
type Event struct {
	// ID is a unique identifier for this event instance (UUID v4).
	ID string `json:"id"`
	// Type categorises the event for filtering and routing.
	Type EventType `json:"type"`
	// Source identifies the component that emitted the event (e.g. "engine", "tool:claude").
	Source string `json:"source"`
	// Target optionally restricts delivery to a single subscriber ID. An empty
	// string means the event is broadcast to all matching subscribers.
	Target string `json:"target,omitempty"`
	// Payload carries arbitrary key-value data specific to the event type.
	Payload map[string]interface{} `json:"payload,omitempty"`
	// Timestamp records when the event was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewEvent creates a broadcast Event with a fresh UUID and the current timestamp.
func NewEvent(eventType EventType, source string, payload map[string]interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}

// NewTargetedEvent creates an Event directed at a specific subscriber identified
// by target. Only the subscriber whose ID matches target will receive it.
func NewTargetedEvent(eventType EventType, source, target string, payload map[string]interface{}) Event {
	return Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    source,
		Target:    target,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}
