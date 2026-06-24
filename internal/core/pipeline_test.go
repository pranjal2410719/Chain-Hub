package core

import (
	"testing"
)

func TestPipeline_Lifecycle(t *testing.T) {
	p := DefaultPipeline("test problem statement")

	if p.Problem != "test problem statement" {
		t.Errorf("expected problem statement 'test problem statement', got %s", p.Problem)
	}

	if p.Status != PipelineStatusRunning {
		t.Errorf("expected running status, got %s", p.Status)
	}

	if len(p.Phases) != 5 {
		t.Errorf("expected 5 default phases, got %d", len(p.Phases))
	}

	// Verify first phase is active.
	firstPhase := p.CurrentPhaseConfig()
	if firstPhase == nil || firstPhase.Phase != PhasePlanning || firstPhase.Status != PhaseStatusActive {
		t.Errorf("expected first phase to be active planning, got %v", firstPhase)
	}

	if p.Progress() != 0.0 {
		t.Errorf("expected 0%% progress, got %.2f", p.Progress())
	}

	// Advance planning -> research.
	if err := p.AdvancePhase(); err != nil {
		t.Fatalf("failed to advance phase: %v", err)
	}

	if p.CurrentPhase != 1 {
		t.Errorf("expected current phase index 1, got %d", p.CurrentPhase)
	}

	prev := p.Phases[0]
	if prev.Status != PhaseStatusCompleted {
		t.Errorf("expected first phase to be completed, got %s", prev.Status)
	}

	current := p.CurrentPhaseConfig()
	if current.Phase != PhaseResearch || current.Status != PhaseStatusActive {
		t.Errorf("expected current phase to be active research, got %v", current)
	}

	// Advance through all.
	// 1: research -> active implementation (2)
	_ = p.AdvancePhase()
	// 2: implementation -> active scanning (3)
	_ = p.AdvancePhase()
	// 3: scanning -> active testing (4)
	_ = p.AdvancePhase()
	// 4: testing -> complete (5)
	_ = p.AdvancePhase()

	if !p.IsComplete() {
		t.Errorf("expected pipeline to be complete, got status %s", p.Status)
	}

	if p.Progress() != 1.0 {
		t.Errorf("expected 100%% progress, got %.2f", p.Progress())
	}
}

func TestPipeline_FailPhase(t *testing.T) {
	p := DefaultPipeline("fail test")

	err := p.FailPhase("compilation error")
	if err != nil {
		t.Fatalf("failed to fail phase: %v", err)
	}

	if p.Status != PipelineStatusFailed {
		t.Errorf("expected pipeline status failed, got %s", p.Status)
	}

	curr := p.CurrentPhaseConfig()
	if curr.Status != PhaseStatusFailed {
		t.Errorf("expected current phase status failed, got %s", curr.Status)
	}

	if curr.Output != "compilation error" {
		t.Errorf("expected error output, got %s", curr.Output)
	}

	if !p.IsComplete() {
		t.Errorf("expected pipeline to report complete (terminal), got false")
	}
}
