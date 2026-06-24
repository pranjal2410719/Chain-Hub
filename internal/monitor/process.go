package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/eventbus"
)

// ProcessMonitor periodically health-checks a set of tracked ToolAdapters and
// publishes EventToolStatusChanged events when a tool's liveness changes.
type ProcessMonitor struct {
	adapters map[string]adapter.ToolAdapter
	bus      *eventbus.EventBus
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
}

// NewProcessMonitor creates a ProcessMonitor that checks adapter health at the
// given interval.  Events are published via bus (may be nil to disable).
func NewProcessMonitor(interval time.Duration, bus *eventbus.EventBus) *ProcessMonitor {
	return &ProcessMonitor{
		adapters: make(map[string]adapter.ToolAdapter),
		bus:      bus,
		interval: interval,
	}
}

// Track adds a ToolAdapter to the set of monitored adapters.
func (pm *ProcessMonitor) Track(a adapter.ToolAdapter) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.adapters[a.Name()] = a
}

// Untrack removes a ToolAdapter from monitoring.
func (pm *ProcessMonitor) Untrack(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.adapters, name)
}

// Start begins the periodic health-check goroutine.
func (pm *ProcessMonitor) Start(ctx context.Context) error {
	pm.mu.Lock()
	pm.ctx, pm.cancel = context.WithCancel(ctx)
	pm.mu.Unlock()

	go pm.loop()
	return nil
}

// Stop terminates the health-check goroutine.
func (pm *ProcessMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.cancel != nil {
		pm.cancel()
	}
}

// GetStatus returns a map of adapter name → alive status for every tracked
// adapter.
func (pm *ProcessMonitor) GetStatus() map[string]bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make(map[string]bool, len(pm.adapters))
	for name, a := range pm.adapters {
		status[name] = a.IsAlive()
	}
	return status
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (pm *ProcessMonitor) loop() {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			pm.checkAll()
		}
	}
}

// checkAll runs HealthCheck on every tracked adapter and publishes events for
// failures.
func (pm *ProcessMonitor) checkAll() {
	pm.mu.RLock()
	adapters := make(map[string]adapter.ToolAdapter, len(pm.adapters))
	for k, v := range pm.adapters {
		adapters[k] = v
	}
	bus := pm.bus
	pm.mu.RUnlock()

	for name, a := range adapters {
		if err := a.HealthCheck(); err != nil {
			if bus != nil {
				bus.Publish(eventbus.NewEvent(
					eventbus.EventToolStatusChanged,
					"process-monitor",
					map[string]interface{}{
						"tool":    name,
						"alive":   false,
						"error":   err.Error(),
						"message": fmt.Sprintf("health check failed for %s: %v", name, err),
					},
				))
			}
		}
	}
}
