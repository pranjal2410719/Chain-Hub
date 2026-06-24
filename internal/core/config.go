// Package core provides the central orchestration primitives for ChainHub,
// including configuration management, the development pipeline model, and the
// main Engine that ties everything together.
package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration container for ChainHub.
type Config struct {
	ChainHub ChainHubConfig `yaml:"chainhub" json:"chainhub"`
}

// ChainHubConfig holds all tunable settings for the orchestrator.
type ChainHubConfig struct {
	// Version is the semantic version of this configuration schema.
	Version string `yaml:"version" json:"version"`
	// Workspace is the root directory for all ChainHub runtime artefacts.
	Workspace string `yaml:"workspace" json:"workspace"`
	// LogLevel controls the global logging verbosity (debug, info, warn, error).
	LogLevel string `yaml:"log_level" json:"log_level"`
	// Pipeline configures the default development pipeline behaviour.
	Pipeline PipelineConfig `yaml:"pipeline" json:"pipeline"`
	// Monitor configures the system resource monitor.
	Monitor MonitorConfig `yaml:"monitor" json:"monitor"`
	// EventBus configures the internal event bus.
	EventBus EventBusConfig `yaml:"event_bus" json:"event_bus"`
	// Context configures inter-tool context sharing.
	Context ContextConfig `yaml:"context" json:"context"`
}

// PipelineConfig controls how development pipelines are constructed and run.
type PipelineConfig struct {
	// DefaultPhases lists the phases included when creating a default pipeline.
	DefaultPhases []string `yaml:"default_phases" json:"default_phases"`
	// FeedbackLoops enables iterative refinement between phases.
	FeedbackLoops bool `yaml:"feedback_loops" json:"feedback_loops"`
	// MaxConcurrentTools limits the number of tools running simultaneously.
	MaxConcurrentTools int `yaml:"max_concurrent_tools" json:"max_concurrent_tools"`
	// Mode determines orchestration mode: "auto" (default) or "manual".
	Mode string `yaml:"mode" json:"mode"`
	// PhaseAssignments maps phase names (e.g. "planning") to assigned tool names.
	PhaseAssignments map[string]string `yaml:"phase_assignments" json:"phase_assignments"`
}

// MonitorConfig controls system resource monitoring.
type MonitorConfig struct {
	// Interval is the polling interval as a duration string (e.g. "5s").
	Interval string `yaml:"interval" json:"interval"`
	// Alerts defines the thresholds that trigger system alerts.
	Alerts AlertConfig `yaml:"alerts" json:"alerts"`
}

// AlertConfig defines resource usage thresholds (percentages) that trigger alerts.
type AlertConfig struct {
	// CPUThreshold is the CPU usage percentage above which an alert fires.
	CPUThreshold int `yaml:"cpu_threshold" json:"cpu_threshold"`
	// MemoryThreshold is the memory usage percentage above which an alert fires.
	MemoryThreshold int `yaml:"memory_threshold" json:"memory_threshold"`
	// DiskThreshold is the disk usage percentage above which an alert fires.
	DiskThreshold int `yaml:"disk_threshold" json:"disk_threshold"`
}

// EventBusConfig configures the event bus subsystem.
type EventBusConfig struct {
	// Type selects the event bus implementation ("memory" for in-process).
	Type string `yaml:"type" json:"type"`
	// BufferSize is the per-subscriber channel buffer size.
	BufferSize int `yaml:"buffer_size" json:"buffer_size"`
}

// ContextConfig configures inter-tool context and report sharing.
type ContextConfig struct {
	// ShareMethod determines how context is shared between tools ("file" or "event").
	ShareMethod string `yaml:"share_method" json:"share_method"`
	// ReportsDir is the directory where phase reports are written.
	ReportsDir string `yaml:"reports_dir" json:"reports_dir"`
}

// DefaultConfig returns a Config populated with production-ready defaults.
func DefaultConfig() *Config {
	return &Config{
		ChainHub: ChainHubConfig{
			Version:   "1.0.0",
			Workspace: ".chainhub",
			LogLevel:  "info",
			Pipeline: PipelineConfig{
				DefaultPhases:      []string{"planning", "research", "implementation", "scanning", "testing"},
				FeedbackLoops:      true,
				MaxConcurrentTools: 3,
				Mode:               "auto",
				PhaseAssignments:   make(map[string]string),
			},
			Monitor: MonitorConfig{
				Interval: "5s",
				Alerts: AlertConfig{
					CPUThreshold:    85,
					MemoryThreshold: 80,
					DiskThreshold:   90,
				},
			},
			EventBus: EventBusConfig{
				Type:       "memory",
				BufferSize: 100,
			},
			Context: ContextConfig{
				ShareMethod: "file",
				ReportsDir:  "reports",
			},
		},
	}
}

// LoadConfig reads a YAML configuration file from the given path and returns
// the parsed Config. Fields not present in the file retain their zero values;
// callers should merge with DefaultConfig if fall-through defaults are desired.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}
	return cfg, nil
}

// SaveConfig serialises the given Config to YAML and writes it to path,
// creating parent directories as needed. The file is written with mode 0644.
func SaveConfig(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file %s: %w", path, err)
	}
	return nil
}

// EnsureWorkspace creates the workspace directory tree required by ChainHub.
// The following subdirectories are created under cfg.ChainHub.Workspace:
//
//	workspace/          — root workspace directory
//	workspace/reports/  — phase output reports
//	workspace/logs/     — runtime log files
//	workspace/context/  — shared context files
func EnsureWorkspace(cfg *Config) error {
	base := cfg.ChainHub.Workspace
	dirs := []string{
		base,
		filepath.Join(base, "reports"),
		filepath.Join(base, "logs"),
		filepath.Join(base, "context"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating workspace directory %s: %w", dir, err)
		}
	}
	return nil
}
