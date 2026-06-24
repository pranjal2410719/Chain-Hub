// Package sharedctx provides a shared context layer that enables inter-tool
// communication through a managed filesystem workspace.  Reports, key-value
// context pairs, and shared files are stored under a common directory tree
// and changes are broadcast via the EventBus so every tool can react in
// real time.
//
// The package name is "sharedctx" (not "context") to avoid conflict with the
// Go standard library context package.
package sharedctx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/khurafati/chainhub/internal/eventbus"
)

// SharedContext manages the filesystem-based workspace used for inter-tool
// communication.  It organises reports, context values, and shared files into
// well-known directories and publishes EventContextUpdated events whenever the
// state changes.
type SharedContext struct {
	workspaceDir string
	reportsDir   string
	contextDir   string
	logsDir      string
	bus          *eventbus.EventBus
	mu           sync.RWMutex
}

// NewSharedContext creates a SharedContext rooted at workspaceDir.  Required
// subdirectories are created automatically if they do not exist.
func NewSharedContext(workspaceDir string, bus *eventbus.EventBus) (*SharedContext, error) {
	reportsDir := filepath.Join(workspaceDir, "reports")
	contextDir := filepath.Join(workspaceDir, "context")
	logsDir := filepath.Join(workspaceDir, "logs")

	for _, dir := range []string{workspaceDir, reportsDir, contextDir, logsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &SharedContext{
		workspaceDir: workspaceDir,
		reportsDir:   reportsDir,
		contextDir:   contextDir,
		logsDir:      logsDir,
		bus:          bus,
	}, nil
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// WriteReport writes a markdown report to
// <reportsDir>/<toolName>/<reportName>.md.  An EventContextUpdated event is
// published so other tools can react.
func (sc *SharedContext) WriteReport(toolName, reportName, content string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	dir := filepath.Join(sc.reportsDir, sanitizePath(toolName))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create report directory for %s: %w", toolName, err)
	}

	path := filepath.Join(dir, sanitizePath(reportName)+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write report %s: %w", path, err)
	}

	if sc.bus != nil {
		sc.bus.Publish(eventbus.NewEvent(
			eventbus.EventContextUpdated,
			toolName,
			map[string]interface{}{
				"type":   "report",
				"tool":   toolName,
				"report": reportName,
				"path":   path,
			},
		))
	}
	return nil
}

// ReadReport reads a previously written report.
func (sc *SharedContext) ReadReport(toolName, reportName string) (string, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	path := filepath.Join(sc.reportsDir, sanitizePath(toolName), sanitizePath(reportName)+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read report %s: %w", path, err)
	}
	return string(data), nil
}

// ListReports returns the names of all reports written by the given tool.
func (sc *SharedContext) ListReports(toolName string) ([]string, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	dir := filepath.Join(sc.reportsDir, sanitizePath(toolName))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list reports for %s: %w", toolName, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			names = append(names, strings.TrimSuffix(name, ".md"))
		}
	}
	return names, nil
}

// ---------------------------------------------------------------------------
// Key-value context
// ---------------------------------------------------------------------------

// WriteContext stores a key-value pair as <contextDir>/<key>.txt.
func (sc *SharedContext) WriteContext(key, value string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	path := filepath.Join(sc.contextDir, sanitizePath(key)+".txt")
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("failed to write context key %s: %w", key, err)
	}

	if sc.bus != nil {
		sc.bus.Publish(eventbus.NewEvent(
			eventbus.EventContextUpdated,
			"shared-context",
			map[string]interface{}{
				"type": "context",
				"key":  key,
				"path": path,
			},
		))
	}
	return nil
}

// ReadContext reads a previously stored context value.
func (sc *SharedContext) ReadContext(key string) (string, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	path := filepath.Join(sc.contextDir, sanitizePath(key)+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read context key %s: %w", key, err)
	}
	return string(data), nil
}

// ---------------------------------------------------------------------------
// File sharing
// ---------------------------------------------------------------------------

// ShareFile copies a source file into the workspace area for the given tool.
// The destination directory <workspaceDir>/<toolName>/ is created if needed.
func (sc *SharedContext) ShareFile(src, toolName string) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	destDir := filepath.Join(sc.workspaceDir, sanitizePath(toolName))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create share directory for %s: %w", toolName, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	destPath := filepath.Join(destDir, filepath.Base(src))
	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file to %s: %w", destPath, err)
	}

	if sc.bus != nil {
		sc.bus.Publish(eventbus.NewEvent(
			eventbus.EventContextUpdated,
			"shared-context",
			map[string]interface{}{
				"type":     "file_shared",
				"tool":     toolName,
				"src":      src,
				"dest":     destPath,
			},
		))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// GetWorkspacePath returns the root workspace directory.
func (sc *SharedContext) GetWorkspacePath() string {
	return sc.workspaceDir
}

// GetReportsPath returns the reports directory.
func (sc *SharedContext) GetReportsPath() string {
	return sc.reportsDir
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// sanitizePath replaces characters that are unsafe in file paths.
func sanitizePath(s string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		" ", "_",
	)
	return replacer.Replace(s)
}
