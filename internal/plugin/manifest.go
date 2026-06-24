// Package plugin provides types and utilities for loading ChainHub plugin
// manifests from YAML files.  A manifest describes a third-party CLI tool —
// its command, arguments, capabilities, health-check configuration, and
// resource limits — so that ChainHub can launch and manage it like any
// built-in adapter.
package plugin

import (
	"fmt"
	"os"

	"github.com/khurafati/chainhub/internal/adapter"
	"gopkg.in/yaml.v3"
)

// Manifest represents the declarative configuration of a ChainHub plugin,
// typically stored as `manifest.yaml` inside a plugin directory.
type Manifest struct {
	// Name is the unique machine-readable identifier for the plugin.
	Name string `yaml:"name"`
	// DisplayName is a human-friendly label shown in the TUI.
	DisplayName string `yaml:"display_name"`
	// Version is the semantic version of the plugin.
	Version string `yaml:"version"`
	// Specialty lists broad functional categories (kept for backward compat).
	Specialty []string `yaml:"specialty"`
	// Command is the executable path or name.
	Command string `yaml:"command"`
	// Args are the default command-line arguments.
	Args []string `yaml:"args"`
	// Capabilities lists fine-grained ToolCapability strings.
	Capabilities []string `yaml:"capabilities"`
	// HealthCheck configures the health-check probe.
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	// Priority indicates scheduling priority ("high", "medium", "low").
	Priority string `yaml:"priority"`
	// ResourceLimits constrains the resources the plugin may consume.
	ResourceLimits ResourceLimitsConfig `yaml:"resource_limits"`
	// Env holds additional environment variables for the process.
	Env map[string]string `yaml:"env"`
}

// HealthCheckConfig describes how ChainHub should probe the plugin's health.
type HealthCheckConfig struct {
	// Command is the shell command to run for health checking.
	Command string `yaml:"command"`
	// Interval is the duration string between consecutive checks (e.g. "30s").
	Interval string `yaml:"interval"`
}

// ResourceLimitsConfig constrains the resources a plugin process may consume.
type ResourceLimitsConfig struct {
	// MaxCPUPercent is the maximum CPU percentage the plugin should use.
	MaxCPUPercent int `yaml:"max_cpu_percent"`
	// MaxMemoryMB is the maximum memory in megabytes the plugin should use.
	MaxMemoryMB int `yaml:"max_memory_mb"`
}

// LoadManifest reads and parses a YAML manifest file from the given path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest %s: %w", path, err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", path, err)
	}

	return &m, nil
}

// ToToolInfo converts the manifest into an adapter.ToolInfo suitable for
// creating a GenericAdapter.
func (m *Manifest) ToToolInfo() adapter.ToolInfo {
	caps := make([]adapter.ToolCapability, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		caps = append(caps, adapter.ToolCapability(c))
	}

	// Fall back to Specialty if Capabilities is empty.
	if len(caps) == 0 {
		for _, s := range m.Specialty {
			caps = append(caps, adapter.ToolCapability(s))
		}
	}

	return adapter.ToolInfo{
		Name:        m.Name,
		DisplayName: m.DisplayName,
		Specialties: caps,
		Command:     m.Command,
		Args:        m.Args,
		Priority:    m.Priority,
		Env:         m.Env,
	}
}

// Validate checks that all required fields are present and returns an error
// describing any missing values.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest field 'name' is required")
	}
	if m.Command == "" {
		return fmt.Errorf("manifest field 'command' is required")
	}
	if m.DisplayName == "" {
		m.DisplayName = m.Name
	}
	if m.Priority == "" {
		m.Priority = "medium"
	}
	return nil
}
