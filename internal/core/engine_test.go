package core

import (
	"context"
	"testing"
	"time"

	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/eventbus"
)

func TestEngine_ProblemSubmissionAndOrchestration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ChainHub.LogLevel = "error" // mute logger for test run

	bus := eventbus.NewEventBus()
	engine := NewEngine(cfg, bus)
	registry := adapter.NewRegistry()
	engine.SetRegistry(registry)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}
	defer func() { _ = engine.Stop() }()

	// Subscribe to events.
	sub := bus.SubscribeAll()
	defer bus.Unsubscribe(sub.ID)

	// Submit a problem.
	problem := "build a calculator CLI"
	pipeline, err := engine.SubmitProblem(problem)
	if err != nil {
		t.Fatalf("failed to submit problem: %v", err)
	}

	if pipeline == nil {
		t.Fatal("expected pipeline, got nil")
	}

	// Verify PipelinePhaseChanged event is emitted for PhasePlanning.
	select {
	case evt := <-sub.Channel:
		if evt.Type != eventbus.EventPipelinePhaseChanged {
			t.Errorf("expected EventPipelinePhaseChanged, got %s", evt.Type)
		}
		if evt.Payload["phase"] != "planning" {
			t.Errorf("expected first phase planning, got %v", evt.Payload["phase"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for pipeline phase changed event")
	}

	// Since registry is empty, the planning phase has no tools, and it should auto-advance to research.
	// Verify that the pipeline advances to research automatically.
	select {
	case evt := <-sub.Channel:
		if evt.Type != eventbus.EventPipelinePhaseChanged {
			t.Errorf("expected EventPipelinePhaseChanged (auto-advanced), got %s", evt.Type)
		}
		if evt.Payload["phase"] != "research" {
			t.Errorf("expected auto-advance to research, got %v", evt.Payload["phase"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for auto-advance event")
	}
}

func TestEngine_ManualModeOrchestration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ChainHub.LogLevel = "error"
	cfg.ChainHub.Pipeline.Mode = "manual"
	cfg.ChainHub.Pipeline.PhaseAssignments = map[string]string{
		"planning": "mimo-code",
	}

	bus := eventbus.NewEventBus()
	engine := NewEngine(cfg, bus)
	registry := adapter.NewRegistry()
	engine.SetRegistry(registry)

	info := adapter.ToolInfo{
		Name:        "mimo-code",
		DisplayName: "Mimo Code",
		Command:     "echo",
		Args:        []string{"mimo"},
		Specialties: []adapter.ToolCapability{adapter.CapPlanning},
	}
	mimo := adapter.NewGenericAdapter(info)
	_ = registry.Register(mimo)
	mimo.SetEventBus(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}
	defer func() { _ = engine.Stop() }()

	sub := bus.SubscribeAll()
	defer bus.Unsubscribe(sub.ID)

	pipeline, err := engine.SubmitProblem("test manual problem")
	if err != nil {
		t.Fatalf("failed to submit problem: %v", err)
	}

	if pipeline == nil {
		t.Fatal("expected pipeline, got nil")
	}

	// First event: PipelinePhaseChanged (planning)
	select {
	case evt := <-sub.Channel:
		if evt.Type != eventbus.EventPipelinePhaseChanged || evt.Payload["phase"] != "planning" {
			t.Errorf("expected planning event, got %v", evt)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for planning phase event")
	}

	// Verify auto-advance to research occurs after the mimo-code adapter completes (echo exits).
	select {
	case evt := <-sub.Channel:
		foundResearch := false
		for i := 0; i < 20; i++ {
			if evt.Type == eventbus.EventPipelinePhaseChanged && evt.Payload["phase"] == "research" {
				foundResearch = true
				break
			}
			select {
			case evt = <-sub.Channel:
			case <-time.After(2 * time.Second):
				break
			}
		}
		if !foundResearch {
			t.Fatal("pipeline did not advance to research phase in manual mode")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for next phase event in manual mode")
	}
}
