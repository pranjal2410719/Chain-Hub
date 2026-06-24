package adapter

import (
	"context"
	"fmt"
	"sync"
)

// Registry is a thread-safe store of named ToolAdapter instances.  It provides
// lookup, capability-based filtering, and batch lifecycle operations (start /
// stop / health-check all).
type Registry struct {
	tools map[string]ToolAdapter
	mu    sync.RWMutex
}

// NewRegistry creates an empty Registry ready for use.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]ToolAdapter),
	}
}

// Register adds a ToolAdapter to the registry.  An error is returned if an
// adapter with the same name is already registered.
func (r *Registry) Register(a ToolAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("adapter %q is already registered", name)
	}
	r.tools[name] = a
	return nil
}

// Unregister removes a ToolAdapter from the registry by name.  An error is
// returned if no adapter with that name exists.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("adapter %q is not registered", name)
	}
	delete(r.tools, name)
	return nil
}

// Get retrieves a ToolAdapter by its unique name.
func (r *Registry) Get(name string) (ToolAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not found", name)
	}
	return a, nil
}

// GetByCapability returns every registered adapter that reports the given
// capability.
func (r *Registry) GetByCapability(cap ToolCapability) []ToolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ToolAdapter
	for _, a := range r.tools {
		if a.HasCapability(cap) {
			result = append(result, a)
		}
	}
	return result
}

// List returns a snapshot of all registered adapters in arbitrary order.
func (r *Registry) List() []ToolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]ToolAdapter, 0, len(r.tools))
	for _, a := range r.tools {
		list = append(list, a)
	}
	return list
}

// Count returns the number of registered adapters.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// StartAll starts every registered adapter using the given context.  Errors
// from individual adapters are collected but do not prevent other adapters
// from starting.  If any adapter fails to start the returned error describes
// all failures.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, a := range r.tools {
		if err := a.Start(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start %d adapter(s): %v", len(errs), errs)
	}
	return nil
}

// StopAll stops every registered adapter.  Like StartAll, errors are collected
// so that one failure does not block the rest.
func (r *Registry) StopAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, a := range r.tools {
		if err := a.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to stop %d adapter(s): %v", len(errs), errs)
	}
	return nil
}

// HealthCheckAll runs a health check on every registered adapter and returns
// a map of adapter name → error (nil when healthy).
func (r *Registry) HealthCheckAll() map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error, len(r.tools))
	for name, a := range r.tools {
		results[name] = a.HealthCheck()
	}
	return results
}
