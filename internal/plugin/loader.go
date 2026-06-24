package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/khurafati/chainhub/internal/adapter"
)

// Loader discovers, loads, and caches plugin manifests from one or more
// directories.  It can also instantiate GenericAdapter instances from any
// loaded manifest.
type Loader struct {
	pluginDirs []string
	manifests  map[string]*Manifest
	mu         sync.RWMutex
}

// NewLoader creates a Loader that will scan the given directories for plugin
// manifests.
func NewLoader(dirs ...string) *Loader {
	return &Loader{
		pluginDirs: dirs,
		manifests:  make(map[string]*Manifest),
	}
}

// LoadAll scans every configured plugin directory for subdirectories containing
// a `manifest.yaml` file and loads them.  Already-loaded manifests are
// replaced if they are found again.
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error
	for _, dir := range l.pluginDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to read plugin directory %s: %w", dir, err)
			}
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDir := filepath.Join(dir, entry.Name())
			m, err := l.loadFromDirLocked(pluginDir)
			if err != nil {
				// Non-fatal: skip broken plugins but remember first error.
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			l.manifests[m.Name] = m
		}
	}

	return firstErr
}

// LoadFromDir loads a single plugin manifest from the given directory.
func (l *Loader) LoadFromDir(dir string) (*Manifest, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	m, err := l.loadFromDirLocked(dir)
	if err != nil {
		return nil, err
	}
	l.manifests[m.Name] = m
	return m, nil
}

// loadFromDirLocked performs the actual manifest loading.  Caller must hold
// l.mu.
func (l *Loader) loadFromDirLocked(dir string) (*Manifest, error) {
	manifestPath := filepath.Join(dir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		// Try .yml extension as a fallback.
		manifestPath = filepath.Join(dir, "manifest.yml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("no manifest.yaml found in %s", dir)
		}
	}

	return LoadManifest(manifestPath)
}

// GetManifest retrieves a previously loaded manifest by plugin name.
func (l *Loader) GetManifest(name string) (*Manifest, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m, ok := l.manifests[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not loaded", name)
	}
	return m, nil
}

// ListManifests returns all loaded manifests in arbitrary order.
func (l *Loader) ListManifests() []*Manifest {
	l.mu.RLock()
	defer l.mu.RUnlock()

	list := make([]*Manifest, 0, len(l.manifests))
	for _, m := range l.manifests {
		list = append(list, m)
	}
	return list
}

// CreateAdapter creates a GenericAdapter from a previously loaded manifest
// identified by name.
func (l *Loader) CreateAdapter(name string) (adapter.ToolAdapter, error) {
	m, err := l.GetManifest(name)
	if err != nil {
		return nil, err
	}

	info := m.ToToolInfo()
	return adapter.NewGenericAdapter(info), nil
}
