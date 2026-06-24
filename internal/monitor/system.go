// Package monitor provides system-level and process-level health monitoring
// for the ChainHub orchestrator.
package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/khurafati/chainhub/internal/eventbus"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemMetrics is a point-in-time snapshot of the host's resource usage.
type SystemMetrics struct {
	// CPUPercent is the average CPU utilization across all cores.
	CPUPercent float64 `json:"cpu_percent"`
	// MemoryPercent is the percentage of used physical memory.
	MemoryPercent float64 `json:"memory_percent"`
	// MemoryUsedMB is the amount of physical memory in use (MiB).
	MemoryUsedMB uint64 `json:"memory_used_mb"`
	// MemoryTotalMB is the total physical memory installed (MiB).
	MemoryTotalMB uint64 `json:"memory_total_mb"`
	// DiskPercent is the percentage of used disk space on the root volume.
	DiskPercent float64 `json:"disk_percent"`
	// DiskUsedGB is the used disk space on the root volume (GiB).
	DiskUsedGB float64 `json:"disk_used_gb"`
	// DiskTotalGB is the total disk capacity on the root volume (GiB).
	DiskTotalGB float64 `json:"disk_total_gb"`
	// Timestamp records when the snapshot was taken.
	Timestamp time.Time `json:"timestamp"`
}

// AlertThresholds configures when the system monitor should fire alerts.
// A threshold of 0 disables the corresponding alert.
type AlertThresholds struct {
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
}

// SystemMonitor collects host resource metrics at a configurable interval and
// publishes EventSystemAlert events when any threshold is exceeded.
type SystemMonitor struct {
	metrics   SystemMetrics
	mu        sync.RWMutex
	interval  time.Duration
	bus       *eventbus.EventBus
	ctx       context.Context
	cancel    context.CancelFunc
	alerts    AlertThresholds
	metricsCh chan SystemMetrics
}

// NewSystemMonitor creates a SystemMonitor that collects metrics at the given
// interval.  If bus is non-nil, alerts are published when thresholds are
// exceeded.
func NewSystemMonitor(interval time.Duration, bus *eventbus.EventBus, thresholds AlertThresholds) *SystemMonitor {
	return &SystemMonitor{
		interval:  interval,
		bus:       bus,
		alerts:    thresholds,
		metricsCh: make(chan SystemMetrics, 16),
	}
}

// Start begins the background metrics collection goroutine.
func (m *SystemMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()

	// Collect once immediately so GetMetrics returns something useful.
	m.collect()

	go m.loop()
	return nil
}

// Stop terminates the background collection goroutine.
func (m *SystemMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// GetMetrics returns the most recent system metrics snapshot.
func (m *SystemMonitor) GetMetrics() SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

// MetricsChan returns a read-only channel that emits a SystemMetrics value
// after every collection cycle.
func (m *SystemMonitor) MetricsChan() <-chan SystemMetrics {
	return m.metricsCh
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// loop runs the periodic collection until the context is cancelled.
func (m *SystemMonitor) loop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

// collect gathers CPU, memory, and disk metrics from the host OS and fires
// alerts when thresholds are breached.
func (m *SystemMonitor) collect() {
	now := time.Now()
	metrics := SystemMetrics{Timestamp: now}

	// CPU — sample over a very short window so we don't block long.
	if cpuPercents, err := cpu.Percent(0, false); err == nil && len(cpuPercents) > 0 {
		metrics.CPUPercent = cpuPercents[0]
	}

	// Memory.
	if vmStat, err := mem.VirtualMemory(); err == nil {
		metrics.MemoryPercent = vmStat.UsedPercent
		metrics.MemoryUsedMB = vmStat.Used / (1024 * 1024)
		metrics.MemoryTotalMB = vmStat.Total / (1024 * 1024)
	}

	// Disk (root volume).
	if diskStat, err := disk.Usage("/"); err == nil {
		metrics.DiskPercent = diskStat.UsedPercent
		metrics.DiskUsedGB = float64(diskStat.Used) / (1024 * 1024 * 1024)
		metrics.DiskTotalGB = float64(diskStat.Total) / (1024 * 1024 * 1024)
	}

	// Store and broadcast.
	m.mu.Lock()
	m.metrics = metrics
	bus := m.bus
	alerts := m.alerts
	m.mu.Unlock()

	// Non-blocking send on metrics channel.
	select {
	case m.metricsCh <- metrics:
	default:
	}

	// Threshold alerts.
	if bus != nil {
		m.checkAlerts(bus, metrics, alerts)
	}
}

// checkAlerts publishes EventSystemAlert for every breached threshold.
func (m *SystemMonitor) checkAlerts(bus *eventbus.EventBus, metrics SystemMetrics, thresholds AlertThresholds) {
	if thresholds.CPUPercent > 0 && metrics.CPUPercent > thresholds.CPUPercent {
		bus.Publish(eventbus.NewEvent(
			eventbus.EventSystemAlert,
			"system-monitor",
			map[string]interface{}{
				"alert":     "cpu_threshold_exceeded",
				"value":     metrics.CPUPercent,
				"threshold": thresholds.CPUPercent,
				"message":   fmt.Sprintf("CPU usage %.1f%% exceeds threshold %.1f%%", metrics.CPUPercent, thresholds.CPUPercent),
			},
		))
	}

	if thresholds.MemoryPercent > 0 && metrics.MemoryPercent > thresholds.MemoryPercent {
		bus.Publish(eventbus.NewEvent(
			eventbus.EventSystemAlert,
			"system-monitor",
			map[string]interface{}{
				"alert":     "memory_threshold_exceeded",
				"value":     metrics.MemoryPercent,
				"threshold": thresholds.MemoryPercent,
				"message":   fmt.Sprintf("Memory usage %.1f%% exceeds threshold %.1f%%", metrics.MemoryPercent, thresholds.MemoryPercent),
			},
		))
	}

	if thresholds.DiskPercent > 0 && metrics.DiskPercent > thresholds.DiskPercent {
		bus.Publish(eventbus.NewEvent(
			eventbus.EventSystemAlert,
			"system-monitor",
			map[string]interface{}{
				"alert":     "disk_threshold_exceeded",
				"value":     metrics.DiskPercent,
				"threshold": thresholds.DiskPercent,
				"message":   fmt.Sprintf("Disk usage %.1f%% exceeds threshold %.1f%%", metrics.DiskPercent, thresholds.DiskPercent),
			},
		))
	}
}
