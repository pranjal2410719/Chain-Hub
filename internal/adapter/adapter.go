// Package adapter provides the core types and interface for wrapping CLI-based
// AI coding tools as managed adapters within the ChainHub orchestrator.
//
// Every external tool (Claude Code, Antigravity, OpenCode, etc.) is represented
// by a ToolAdapter implementation that handles process lifecycle, I/O streaming,
// health monitoring, and event-bus integration.
package adapter

import (
	"context"

	"github.com/khurafati/chainhub/internal/eventbus"
)

// ---------------------------------------------------------------------------
// Tool status
// ---------------------------------------------------------------------------

// ToolStatus represents the lifecycle state of a managed tool process.
type ToolStatus string

const (
	// ToolStatusIdle means the adapter is registered but its process has not been started.
	ToolStatusIdle ToolStatus = "idle"
	// ToolStatusActive means the underlying process is running and accepting input.
	ToolStatusActive ToolStatus = "active"
	// ToolStatusWatching means the tool is in a passive monitoring / watch mode.
	ToolStatusWatching ToolStatus = "watching"
	// ToolStatusError means the tool encountered a fatal error and is no longer usable.
	ToolStatusError ToolStatus = "error"
	// ToolStatusStopped means the tool has been gracefully shut down.
	ToolStatusStopped ToolStatus = "stopped"
)

// ---------------------------------------------------------------------------
// Tool capabilities
// ---------------------------------------------------------------------------

// ToolCapability describes a high-level functional speciality that a tool
// provides (e.g. coding, testing, scanning).
type ToolCapability string

const (
	// CapPlanning indicates the tool can produce architectural or task plans.
	CapPlanning ToolCapability = "planning"
	// CapCoding indicates the tool can write or modify source code.
	CapCoding ToolCapability = "coding"
	// CapScanning indicates the tool can perform security or static-analysis scans.
	CapScanning ToolCapability = "scanning"
	// CapTesting indicates the tool can generate or run tests.
	CapTesting ToolCapability = "testing"
	// CapResearch indicates the tool can search for information or documentation.
	CapResearch ToolCapability = "research"
	// CapDebugging indicates the tool can help debug runtime issues.
	CapDebugging ToolCapability = "debugging"
	// CapReview indicates the tool can perform code reviews.
	CapReview ToolCapability = "review"
	// CapRefactoring indicates the tool can restructure existing code.
	CapRefactoring ToolCapability = "refactoring"
	// CapBrowsing indicates the tool can browse the web or local resources.
	CapBrowsing ToolCapability = "browsing"
)

// ---------------------------------------------------------------------------
// Tool metadata
// ---------------------------------------------------------------------------

// ToolInfo carries the static and runtime metadata for a managed tool.
type ToolInfo struct {
	// Name is the unique machine-readable identifier (e.g. "claude-code").
	Name string `json:"name" yaml:"name"`
	// DisplayName is a human-friendly label shown in the TUI.
	DisplayName string `json:"display_name" yaml:"display_name"`
	// Specialties lists the capabilities this tool supports.
	Specialties []ToolCapability `json:"specialties" yaml:"specialties"`
	// Command is the executable name or path.
	Command string `json:"command" yaml:"command"`
	// Args are the default command-line arguments.
	Args []string `json:"args" yaml:"args"`
	// Status is the current lifecycle state.
	Status ToolStatus `json:"status"`
	// Priority indicates scheduling priority ("high", "medium", "low").
	Priority string `json:"priority" yaml:"priority"`
	// StatusText is a short human-readable status description.
	StatusText string `json:"status_text"`
}

// ---------------------------------------------------------------------------
// ToolAdapter interface
// ---------------------------------------------------------------------------

// ToolAdapter is the interface that all CLI tool wrappers must implement.
// It encapsulates process management, I/O, health checking, and integration
// with the ChainHub event bus.
type ToolAdapter interface {
	// Name returns the unique machine-readable identifier.
	Name() string
	// DisplayName returns the human-friendly label.
	DisplayName() string
	// Specialties returns the list of capabilities.
	Specialties() []ToolCapability
	// Status returns the current lifecycle state.
	Status() ToolStatus
	// StatusText returns a short human-readable status message.
	StatusText() string
	// Info returns a snapshot of the tool's metadata and runtime state.
	Info() ToolInfo

	// Start launches the underlying process within the given context.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the underlying process.
	Stop() error
	// SendInput writes a line of text to the tool's standard input.
	SendInput(input string) error

	// OutputChan returns a read-only channel that emits lines from stdout.
	OutputChan() <-chan string
	// ErrorChan returns a read-only channel that emits lines from stderr.
	ErrorChan() <-chan string

	// HealthCheck probes the underlying process and returns an error if unhealthy.
	HealthCheck() error
	// IsAlive returns true when the process is still running.
	IsAlive() bool

	// SetEventBus attaches the shared event bus for publishing / subscribing.
	SetEventBus(bus *eventbus.EventBus)
	// OnEvent is called by the event bus when an event targets this adapter.
	OnEvent(event eventbus.Event)

	// HasCapability returns true if the tool supports the given capability.
	HasCapability(cap ToolCapability) bool
}
