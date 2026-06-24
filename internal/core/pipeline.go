package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// PipelinePhase represents a named stage in the software development pipeline.
type PipelinePhase string

const (
	// PhasePlanning is the initial phase where the problem is decomposed.
	PhasePlanning PipelinePhase = "planning"
	// PhaseResearch is the phase where background research is conducted.
	PhaseResearch PipelinePhase = "research"
	// PhaseImplementation is the phase where code is written.
	PhaseImplementation PipelinePhase = "implementation"
	// PhaseScanning is the phase where code is analysed for issues.
	PhaseScanning PipelinePhase = "scanning"
	// PhaseTesting is the phase where automated tests are executed.
	PhaseTesting PipelinePhase = "testing"
	// PhaseComplete is a terminal pseudo-phase indicating the pipeline finished.
	PhaseComplete PipelinePhase = "complete"
)

// PhaseStatus describes the current state of a single pipeline phase.
type PhaseStatus string

const (
	// PhaseStatusPending means the phase has not started yet.
	PhaseStatusPending PhaseStatus = "pending"
	// PhaseStatusActive means the phase is currently executing.
	PhaseStatusActive PhaseStatus = "active"
	// PhaseStatusCompleted means the phase finished successfully.
	PhaseStatusCompleted PhaseStatus = "completed"
	// PhaseStatusFailed means the phase encountered an unrecoverable error.
	PhaseStatusFailed PhaseStatus = "failed"
	// PhaseStatusSkipped means the phase was intentionally bypassed.
	PhaseStatusSkipped PhaseStatus = "skipped"
)

// PhaseConfig holds the configuration and runtime state for a single pipeline phase.
type PhaseConfig struct {
	// Phase is the identifier for this pipeline stage.
	Phase PipelinePhase `yaml:"phase" json:"phase"`
	// AssignedTools lists the tool names responsible for executing this phase.
	AssignedTools []string `yaml:"assigned_tools" json:"assigned_tools"`
	// Dependencies lists phases that must complete before this one can start.
	Dependencies []PipelinePhase `yaml:"dependencies" json:"dependencies"`
	// Status tracks the runtime state of this phase.
	Status PhaseStatus `json:"status"`
	// StartedAt records when the phase began executing.
	StartedAt time.Time `json:"started_at,omitempty"`
	// CompletedAt records when the phase finished (successfully or not).
	CompletedAt time.Time `json:"completed_at,omitempty"`
	// Output stores any textual output or summary produced by the phase.
	Output string `json:"output,omitempty"`
}

// PipelineStatus describes the overall state of a Pipeline.
type PipelineStatus string

const (
	// PipelineStatusPending means the pipeline has been created but not started.
	PipelineStatusPending PipelineStatus = "pending"
	// PipelineStatusRunning means the pipeline is actively executing phases.
	PipelineStatusRunning PipelineStatus = "running"
	// PipelineStatusComplete means all phases have finished successfully.
	PipelineStatusComplete PipelineStatus = "complete"
	// PipelineStatusFailed means the pipeline stopped due to a phase failure.
	PipelineStatusFailed PipelineStatus = "failed"
	// PipelineStatusPaused means the pipeline is temporarily suspended.
	PipelineStatusPaused PipelineStatus = "paused"
)

// Pipeline represents a complete development workflow composed of ordered phases.
// It tracks progress through the phases and provides methods for advancement and
// status inspection.
type Pipeline struct {
	// ID uniquely identifies this pipeline instance (UUID v4).
	ID string `json:"id"`
	// Problem is the original problem statement that initiated this pipeline.
	Problem string `json:"problem"`
	// Phases is the ordered list of phase configurations.
	Phases []*PhaseConfig `json:"phases"`
	// CurrentPhase is the zero-based index into Phases of the active phase.
	CurrentPhase int `json:"current_phase"`
	// Status is the overall pipeline state.
	Status PipelineStatus `json:"status"`
	// CreatedAt records when the pipeline was created.
	CreatedAt time.Time `json:"created_at"`
	// CompletedAt records when the pipeline reached a terminal state.
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// NewPipeline creates a Pipeline with the given phases, each initialised to pending status.
func NewPipeline(problem string, phases []PipelinePhase) *Pipeline {
	configs := make([]*PhaseConfig, len(phases))
	for i, phase := range phases {
		configs[i] = &PhaseConfig{
			Phase:         phase,
			AssignedTools: []string{},
			Dependencies:  []PipelinePhase{},
			Status:        PhaseStatusPending,
		}
	}

	// Mark the first phase as active so the pipeline is ready to run.
	if len(configs) > 0 {
		configs[0].Status = PhaseStatusActive
		configs[0].StartedAt = time.Now()
	}

	return &Pipeline{
		ID:           uuid.New().String(),
		Problem:      problem,
		Phases:       configs,
		CurrentPhase: 0,
		Status:       PipelineStatusRunning,
		CreatedAt:    time.Now(),
	}
}

// DefaultPipeline creates a Pipeline with the standard phase sequence:
// planning → research → implementation → scanning → testing.
func DefaultPipeline(problem string) *Pipeline {
	return NewPipeline(problem, []PipelinePhase{
		PhasePlanning,
		PhaseResearch,
		PhaseImplementation,
		PhaseScanning,
		PhaseTesting,
	})
}

// CurrentPhaseConfig returns a pointer to the PhaseConfig for the currently
// active phase, or nil if the pipeline has no phases.
func (p *Pipeline) CurrentPhaseConfig() *PhaseConfig {
	if p.CurrentPhase < 0 || p.CurrentPhase >= len(p.Phases) {
		return nil
	}
	return p.Phases[p.CurrentPhase]
}

// AdvancePhase marks the current phase as completed and activates the next one.
// If all phases are already complete an error is returned.
func (p *Pipeline) AdvancePhase() error {
	if p.IsComplete() {
		return fmt.Errorf("pipeline %s is already complete", p.ID)
	}
	if p.CurrentPhase >= len(p.Phases) {
		return fmt.Errorf("pipeline %s has no more phases to advance", p.ID)
	}

	// Complete the current phase.
	now := time.Now()
	current := p.Phases[p.CurrentPhase]
	current.Status = PhaseStatusCompleted
	current.CompletedAt = now

	// Move to the next phase.
	p.CurrentPhase++

	if p.CurrentPhase >= len(p.Phases) {
		// All phases done — mark pipeline complete.
		p.Status = PipelineStatusComplete
		p.CompletedAt = now
	} else {
		// Activate the next phase.
		next := p.Phases[p.CurrentPhase]
		next.Status = PhaseStatusActive
		next.StartedAt = now
	}

	return nil
}

// FailPhase marks the current phase and the overall pipeline as failed,
// recording the given reason in the phase output.
func (p *Pipeline) FailPhase(reason string) error {
	if p.CurrentPhase < 0 || p.CurrentPhase >= len(p.Phases) {
		return fmt.Errorf("pipeline %s: no active phase to fail", p.ID)
	}

	now := time.Now()
	current := p.Phases[p.CurrentPhase]
	current.Status = PhaseStatusFailed
	current.CompletedAt = now
	current.Output = reason

	p.Status = PipelineStatusFailed
	p.CompletedAt = now
	return nil
}

// IsComplete returns true when the pipeline has reached a terminal state
// (all phases completed or the pipeline has been marked complete).
func (p *Pipeline) IsComplete() bool {
	return p.Status == PipelineStatusComplete || p.Status == PipelineStatusFailed
}

// Progress returns a value between 0.0 and 1.0 representing how far through
// the pipeline execution has progressed. Completed and failed phases both count
// towards progress.
func (p *Pipeline) Progress() float64 {
	if len(p.Phases) == 0 {
		return 1.0
	}
	completed := 0
	for _, phase := range p.Phases {
		if phase.Status == PhaseStatusCompleted || phase.Status == PhaseStatusSkipped {
			completed++
		}
	}
	return float64(completed) / float64(len(p.Phases))
}

// AssignToolToPhase associates a tool with the given phase. Returns an error if
// the phase is not found in this pipeline.
func (p *Pipeline) AssignToolToPhase(phase PipelinePhase, toolName string) error {
	pc := p.GetPhase(phase)
	if pc == nil {
		return fmt.Errorf("pipeline %s: phase %q not found", p.ID, phase)
	}
	// Avoid duplicate assignments.
	for _, t := range pc.AssignedTools {
		if t == toolName {
			return nil
		}
	}
	pc.AssignedTools = append(pc.AssignedTools, toolName)
	return nil
}

// GetPhase looks up a PhaseConfig by its PipelinePhase name. Returns nil if the
// phase is not part of this pipeline.
func (p *Pipeline) GetPhase(phase PipelinePhase) *PhaseConfig {
	for _, pc := range p.Phases {
		if pc.Phase == phase {
			return pc
		}
	}
	return nil
}

// Save persists the pipeline state to a JSON file in the session directory.
func (p *Pipeline) Save(workspaceDir string) error {
	sessionDir := filepath.Join(workspaceDir, ".chainhub", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling pipeline: %w", err)
	}

	path := filepath.Join(sessionDir, p.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	// Also save as "latest.json" for easy resume
	latestPath := filepath.Join(sessionDir, "latest.json")
	if err := os.WriteFile(latestPath, data, 0644); err != nil {
		return fmt.Errorf("writing latest session: %w", err)
	}

	return nil
}

// LoadPipeline loads a pipeline state from a JSON file.
func LoadPipeline(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var p Pipeline
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshalling pipeline: %w", err)
	}

	return &p, nil
}

// LoadLatestPipeline loads the most recent pipeline session from the workspace.
func LoadLatestPipeline(workspaceDir string) (*Pipeline, error) {
	sessionDir := filepath.Join(workspaceDir, ".chainhub", "sessions")
	latestPath := filepath.Join(sessionDir, "latest.json")

	if _, err := os.Stat(latestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no session found")
	}

	return LoadPipeline(latestPath)
}

// ListSessions returns all saved pipeline sessions in the workspace.
func ListSessions(workspaceDir string) ([]Pipeline, error) {
	sessionDir := filepath.Join(workspaceDir, ".chainhub", "sessions")

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session dir: %w", err)
	}

	var sessions []Pipeline
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "latest.json" {
			continue
		}

		path := filepath.Join(sessionDir, entry.Name())
		p, err := LoadPipeline(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, *p)
	}

	return sessions, nil
}
