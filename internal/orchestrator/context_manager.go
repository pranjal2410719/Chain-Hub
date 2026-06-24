package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SharedContext represents the shared workspace state accessible by all tools.
type SharedContext struct {
	WorkspaceDir string        `json:"workspace_dir"`
	Problem      string        `json:"problem"`
	CurrentPhase string        `json:"current_phase"`
	Phases       []PhaseInfo   `json:"phases"`
	ToolStatus   []ToolStatus  `json:"tool_status"`
	StartedAt    time.Time     `json:"started_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// PhaseInfo contains information about a pipeline phase.
type PhaseInfo struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	AssignedTo string   `json:"assigned_to,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result     string   `json:"result,omitempty"`
}

// ToolStatus contains the status of an external tool.
type ToolStatus struct {
	Name       string    `json:"name"`
	Binary     string    `json:"binary"`
	Status     string    `json:"status"` // idle, running, stopped
	LastSeen   time.Time `json:"last_seen"`
	PID        int       `json:"pid,omitempty"`
}

// ContextManager handles the shared workspace context.
type ContextManager struct {
	workspaceDir string
}

// NewContextManager creates a ContextManager for the given workspace.
func NewContextManager(workspaceDir string) *ContextManager {
	return &ContextManager{workspaceDir: workspaceDir}
}

// Initialize creates the shared context file.
func (cm *ContextManager) Initialize(problem string) (*SharedContext, error) {
	ctx := &SharedContext{
		WorkspaceDir: cm.workspaceDir,
		Problem:      problem,
		CurrentPhase: "planning",
		Phases: []PhaseInfo{
			{Name: "planning", Status: "pending"},
			{Name: "research", Status: "pending"},
			{Name: "implementation", Status: "pending"},
			{Name: "scanning", Status: "pending"},
			{Name: "testing", Status: "pending"},
		},
		ToolStatus: make([]ToolStatus, 0),
		StartedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := cm.Save(ctx); err != nil {
		return nil, err
	}

	return ctx, nil
}

// Load reads the shared context from disk.
func (cm *ContextManager) Load() (*SharedContext, error) {
	path := cm.contextPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading context: %w", err)
	}

	var ctx SharedContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("unmarshalling context: %w", err)
	}

	return &ctx, nil
}

// Save writes the shared context to disk.
func (cm *ContextManager) Save(ctx *SharedContext) error {
	ctx.UpdatedAt = time.Now()

	dir := filepath.Dir(cm.contextPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating context dir: %w", err)
	}

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling context: %w", err)
	}

	if err := os.WriteFile(cm.contextPath(), data, 0644); err != nil {
		return fmt.Errorf("writing context: %w", err)
	}

	return nil
}

// UpdatePhase updates a specific phase's status.
func (cm *ContextManager) UpdatePhase(phaseName, status, assignedTo string) error {
	ctx, err := cm.Load()
	if err != nil {
		return err
	}

	now := time.Now()
	for i, phase := range ctx.Phases {
		if phase.Name == phaseName {
			ctx.Phases[i].Status = status
			ctx.Phases[i].AssignedTo = assignedTo
			if status == "running" && ctx.Phases[i].StartedAt == nil {
				ctx.Phases[i].StartedAt = &now
			}
			if status == "completed" || status == "failed" {
				ctx.Phases[i].CompletedAt = &now
			}
			break
		}
	}

	ctx.CurrentPhase = phaseName
	return cm.Save(ctx)
}

// UpdateToolStatus updates the status of an external tool.
func (cm *ContextManager) UpdateToolStatus(name, binary, status string, pid int) error {
	ctx, err := cm.Load()
	if err != nil {
		return err
	}

	found := false
	for i, tool := range ctx.ToolStatus {
		if tool.Name == name {
			ctx.ToolStatus[i].Status = status
			ctx.ToolStatus[i].LastSeen = time.Now()
			ctx.ToolStatus[i].PID = pid
			found = true
			break
		}
	}

	if !found {
		ctx.ToolStatus = append(ctx.ToolStatus, ToolStatus{
			Name:     name,
			Binary:   binary,
			Status:   status,
			LastSeen: time.Now(),
			PID:      pid,
		})
	}

	return cm.Save(ctx)
}

// GetAvailableTools returns tools that are idle and available for work.
func (cm *ContextManager) GetAvailableTools() ([]ToolStatus, error) {
	ctx, err := cm.Load()
	if err != nil {
		return nil, err
	}

	var available []ToolStatus
	for _, tool := range ctx.ToolStatus {
		if tool.Status == "idle" {
			available = append(available, tool)
		}
	}

	return available, nil
}

func (cm *ContextManager) contextPath() string {
	return filepath.Join(cm.workspaceDir, ".chainhub", "context", "shared.json")
}
