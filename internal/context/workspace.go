package sharedctx

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace manages the on-disk directory structure for a ChainHub project.
// Each project gets a dedicated tree with per-tool directories and a shared
// area.
type Workspace struct {
	rootDir     string
	projectName string
	initialized bool
}

// NewWorkspace creates a Workspace descriptor.  No directories are created
// until Init is called.
func NewWorkspace(rootDir, projectName string) *Workspace {
	return &Workspace{
		rootDir:     rootDir,
		projectName: projectName,
	}
}

// Init creates the full project directory hierarchy:
//
//	<rootDir>/<projectName>/
//	├── shared/
//	├── reports/
//	├── context/
//	└── logs/
func (w *Workspace) Init() error {
	if w.initialized {
		return nil
	}

	dirs := []string{
		w.ProjectDir(),
		w.SharedDir(),
		filepath.Join(w.ProjectDir(), "reports"),
		filepath.Join(w.ProjectDir(), "context"),
		filepath.Join(w.ProjectDir(), "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create workspace directory %s: %w", dir, err)
		}
	}

	w.initialized = true
	return nil
}

// ProjectDir returns the root directory for this project.
func (w *Workspace) ProjectDir() string {
	return filepath.Join(w.rootDir, sanitizePath(w.projectName))
}

// ToolDir returns (and creates if necessary) the per-tool directory within
// the project workspace.
func (w *Workspace) ToolDir(toolName string) string {
	dir := filepath.Join(w.ProjectDir(), "tools", sanitizePath(toolName))
	// Best-effort creation; callers should check Init was called first.
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// SharedDir returns the shared directory that all tools can read and write to.
func (w *Workspace) SharedDir() string {
	return filepath.Join(w.ProjectDir(), "shared")
}

// Clean removes the entire project workspace from disk.
func (w *Workspace) Clean() error {
	if err := os.RemoveAll(w.ProjectDir()); err != nil {
		return fmt.Errorf("failed to clean workspace %s: %w", w.ProjectDir(), err)
	}
	w.initialized = false
	return nil
}
