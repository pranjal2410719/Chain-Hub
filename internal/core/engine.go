package core

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/eventbus"
)

// Engine is the central orchestration component of ChainHub. It owns the
// configuration, event bus, and the active pipeline. External callers interact
// with the system through the Engine's public API, while internal coordination
// happens via events published on the bus.
type Engine struct {
	config   *Config
	bus      *eventbus.EventBus
	registry *adapter.Registry
	pipeline *Pipeline
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	logger   zerolog.Logger
	running  bool
}

// NewEngine creates an Engine bound to the given configuration and event bus.
// The engine is not started automatically — call Start to begin processing.
func NewEngine(cfg *Config, bus *eventbus.EventBus) *Engine {
	level, err := zerolog.ParseLevel(cfg.ChainHub.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("component", "engine").
		Logger().
		Level(level)

	return &Engine{
		config: cfg,
		bus:    bus,
		logger: logger,
	}
}

// SetRegistry associates a tool registry with this engine.
func (e *Engine) SetRegistry(r *adapter.Registry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registry = r
}

// Start initialises the engine, starts the event bus, and launches the
// background event-processing goroutine. It is safe to call Start only once;
// subsequent calls return an error.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("engine is already running")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.bus.Start()
	e.running = true

	sub := e.bus.SubscribeAll()
	go e.processEvents(sub)

	e.logger.Info().Msg("engine started")
	return nil
}

// Stop gracefully shuts down the engine. It cancels the context (which
// terminates the event-processing goroutine) and stops the event bus.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return fmt.Errorf("engine is not running")
	}

	e.cancel()
	e.bus.Stop()
	e.running = false

	e.logger.Info().Msg("engine stopped")
	return nil
}

// SubmitProblem creates a new default Pipeline for the given problem statement,
// sets it as the active pipeline, and emits a PipelinePhaseChanged event so
// that other components know work has begun.
func (e *Engine) SubmitProblem(problem string) (*Pipeline, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil, fmt.Errorf("engine is not running — call Start first")
	}

	pipeline := DefaultPipeline(problem)
	e.pipeline = pipeline

	e.logger.Info().
		Str("pipeline_id", pipeline.ID).
		Str("problem", problem).
		Msg("pipeline created")

	phase := pipeline.CurrentPhaseConfig()
	phaseName := ""
	if phase != nil {
		phaseName = string(phase.Phase)
	}

	e.bus.Publish(eventbus.NewEvent(
		eventbus.EventPipelinePhaseChanged,
		"engine",
		map[string]interface{}{
			"pipeline_id": pipeline.ID,
			"phase":       phaseName,
			"status":      string(pipeline.Status),
			"problem":     problem,
		},
	))

	return pipeline, nil
}

// GetPipeline returns the currently active pipeline, or nil if no problem has
// been submitted yet.
func (e *Engine) GetPipeline() *Pipeline {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pipeline
}

// AdvancePipeline moves the active pipeline to its next phase and publishes a
// PipelinePhaseChanged event. Returns an error if no pipeline is active or
// the pipeline is already complete.
func (e *Engine) AdvancePipeline() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.pipeline == nil {
		return fmt.Errorf("no active pipeline")
	}

	if err := e.pipeline.AdvancePhase(); err != nil {
		return err
	}

	phase := e.pipeline.CurrentPhaseConfig()
	phaseName := "complete"
	if phase != nil {
		phaseName = string(phase.Phase)
	}

	e.logger.Info().
		Str("pipeline_id", e.pipeline.ID).
		Str("phase", phaseName).
		Float64("progress", e.pipeline.Progress()).
		Msg("pipeline advanced")

	e.bus.Publish(eventbus.NewEvent(
		eventbus.EventPipelinePhaseChanged,
		"engine",
		map[string]interface{}{
			"pipeline_id": e.pipeline.ID,
			"phase":       phaseName,
			"status":      string(e.pipeline.Status),
			"progress":    e.pipeline.Progress(),
		},
	))

	return nil
}

// Bus returns the EventBus used by this engine.
func (e *Engine) Bus() *eventbus.EventBus {
	return e.bus
}

// Config returns the configuration bound to this engine.
func (e *Engine) Config() *Config {
	return e.config
}

// IsRunning reports whether the engine is currently active.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// processEvents is a long-running goroutine that subscribes to all events on
// the bus and logs them. It also reacts to specific event types for internal
// coordination (e.g. pipeline phase changes). The goroutine exits when the
// engine's context is cancelled.
func (e *Engine) processEvents(sub *eventbus.Subscriber) {
	defer e.bus.Unsubscribe(sub.ID)

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Debug().Msg("event processor shutting down")
			return
		case evt, ok := <-sub.Channel:
			if !ok {
				return
			}
			e.handleEvent(evt)
		}
	}
}

// handleEvent processes a single event received by the event-processing goroutine.
func (e *Engine) handleEvent(evt eventbus.Event) {
	logEvent := e.logger.Debug().
		Str("event_id", evt.ID).
		Str("event_type", string(evt.Type)).
		Str("source", evt.Source)

	if evt.Target != "" {
		logEvent = logEvent.Str("target", evt.Target)
	}

	logEvent.Msg("event received")

	switch evt.Type {
	case eventbus.EventPipelinePhaseChanged:
		if phase, ok := evt.Payload["phase"].(string); ok {
			e.logger.Info().
				Str("phase", phase).
				Msg("pipeline phase changed")

			// Trigger tools for the new phase if it's active.
			if phase != "complete" && phase != "failed" {
				e.triggerPhaseTools(PipelinePhase(phase))
			}
		}
	case eventbus.EventToolError:
		if errMsg, ok := evt.Payload["error"].(string); ok {
			e.logger.Error().
				Str("tool_source", evt.Source).
				Str("error", errMsg).
				Msg("tool reported error")
		}
	case eventbus.EventSystemAlert:
		if msg, ok := evt.Payload["message"].(string); ok {
			e.logger.Warn().
				Str("alert", msg).
				Msg("system alert")
		}
	case eventbus.EventTaskCompleted:
		toolName, _ := evt.Payload["tool"].(string)
		e.checkPhaseCompletion(toolName)
	case eventbus.EventToolStatusChanged:
		toolName, _ := evt.Payload["tool"].(string)
		status, _ := evt.Payload["status"].(string)
		if status == string(adapter.ToolStatusStopped) || status == string(adapter.ToolStatusError) {
			e.checkPhaseCompletion(toolName)
		}
	}
}

// triggerPhaseTools matches tools for the active phase and assigns tasks.
func (e *Engine) triggerPhaseTools(phaseName PipelinePhase) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.pipeline == nil {
		return
	}

	phase := e.pipeline.GetPhase(phaseName)
	if phase == nil {
		return
	}

	var reqCap adapter.ToolCapability
	switch phaseName {
	case PhasePlanning:
		reqCap = adapter.CapPlanning
	case PhaseResearch:
		reqCap = adapter.CapResearch
	case PhaseImplementation:
		reqCap = adapter.CapCoding
	case PhaseScanning:
		reqCap = adapter.CapScanning
	case PhaseTesting:
		reqCap = adapter.CapTesting
	default:
		return
	}

	if e.registry == nil {
		e.logger.Warn().Msg("no tool registry set on engine")
		return
	}

	var tools []adapter.ToolAdapter
	if e.config != nil && e.config.ChainHub.Pipeline.Mode == "manual" {
		toolName := e.config.ChainHub.Pipeline.PhaseAssignments[string(phaseName)]
		if toolName != "" {
			if t, err := e.registry.Get(toolName); err == nil {
				tools = append(tools, t)
			} else {
				e.logger.Warn().Str("phase", string(phaseName)).Str("tool", toolName).Msg("assigned manual tool not found in registry")
			}
		}
	} else {
		tools = e.registry.GetByCapability(reqCap)
	}

	for _, t := range tools {
		_ = e.pipeline.AssignToolToPhase(phaseName, t.Name())

		// Start tool process if not alive.
		if !t.IsAlive() {
			if err := t.Start(e.ctx); err != nil {
				e.logger.Error().Err(err).Str("tool", t.Name()).Msg("failed to start tool for phase")
				continue
			}
		}

		// Publish task assignment event.
		e.bus.Publish(eventbus.NewEvent(
			eventbus.EventTaskAssigned,
			"engine",
			map[string]interface{}{
				"tool":    t.Name(),
				"task":    e.pipeline.Problem,
				"phase":   string(phaseName),
				"details": fmt.Sprintf("Execute the %s phase for: %s", phaseName, e.pipeline.Problem),
			},
		))
	}

	// If no tools are assigned, auto-advance the pipeline after a brief delay.
	if len(phase.AssignedTools) == 0 {
		e.logger.Info().Str("phase", string(phaseName)).Msg("no tools found for phase, auto-advancing")
		go func() {
			time.Sleep(1 * time.Second)
			e.mu.Lock()
			currentPhase := e.pipeline.CurrentPhaseConfig()
			e.mu.Unlock()
			if currentPhase != nil && currentPhase.Phase == phaseName {
				_ = e.AdvancePipeline()
			}
		}()
	}
}

// checkPhaseCompletion checks if all tools assigned to the current phase are done.
func (e *Engine) checkPhaseCompletion(finishedTool string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.pipeline == nil || e.pipeline.IsComplete() {
		return
	}

	phase := e.pipeline.CurrentPhaseConfig()
	if phase == nil {
		return
	}

	// Verify the tool is assigned to the current phase.
	assigned := false
	for _, name := range phase.AssignedTools {
		if name == finishedTool {
			assigned = true
			break
		}
	}
	if !assigned {
		return
	}

	// Check if all assigned tools have stopped or completed.
	allDone := true
	if e.registry != nil {
		for _, name := range phase.AssignedTools {
			t, err := e.registry.Get(name)
			if err != nil {
				continue
			}
			// If any tool is still alive and not stopped, it's not done.
			if t.IsAlive() && t.Status() == adapter.ToolStatusActive {
				allDone = false
				break
			}
		}
	}

	if allDone {
		e.logger.Info().Str("phase", string(phase.Phase)).Msg("all tools done for phase, advancing")
		go func() {
			_ = e.AdvancePipeline()
		}()
	}
}
