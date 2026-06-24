// Package tui implements the ChainHub terminal user interface powered by the
// Bubbletea framework. This file contains the main application model, message
// types, and the Init/Update/View lifecycle.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/core"
	"github.com/khurafati/chainhub/internal/eventbus"
	"github.com/khurafati/chainhub/internal/monitor"
)

// ─── Custom Messages ────────────────────────────────────────────────────────

// tickMsg fires once per second to drive metric and event refreshes.
type tickMsg time.Time

// eventMsg wraps an event received from the EventBus subscriber channel.
type eventMsg eventbus.Event

// ─── AppModel ───────────────────────────────────────────────────────────────

// AppModel is the top-level Bubbletea model for the ChainHub dashboard.
type AppModel struct {
	engine   *core.Engine
	registry *adapter.Registry
	monitor  *monitor.SystemMonitor

	events   []eventbus.Event
	eventSub *eventbus.Subscriber
	metrics  monitor.SystemMetrics

	width  int
	height int

	activeTab int // 0 = dashboard, 1 = pipeline, 2 = logs
	logLines  []string
	quitting  bool
}

// NewApp creates a new AppModel wired to the provided engine, adapter registry,
// and system monitor. Call this before tea.NewProgram.
func NewApp(engine *core.Engine, registry *adapter.Registry, mon *monitor.SystemMonitor) AppModel {
	sub := engine.Bus().SubscribeAll()
	return AppModel{
		engine:   engine,
		registry: registry,
		monitor:  mon,
		eventSub: sub,
		events:   make([]eventbus.Event, 0, 128),
		logLines: make([]string, 0, 256),
		width:    120,
		height:   40,
	}
}

// ─── Bubbletea Lifecycle ────────────────────────────────────────────────────

// Init returns the initial batch of commands: a periodic tick and the event
// listener.
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		listenForEvents(m.eventSub),
	)
}

// Update processes incoming messages and returns the updated model with any
// follow-up commands.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Keyboard ────────────────────────────────────────────────────
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
		case "1":
			m.activeTab = 0
		case "2":
			m.activeTab = 1
		case "3":
			m.activeTab = 2
		case "n", " ":
			_ = m.engine.AdvancePipeline()
		}

	// ── Terminal resize ─────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// ── Periodic tick ───────────────────────────────────────────────
	case tickMsg:
		if m.monitor != nil {
			m.metrics = m.monitor.GetMetrics()
		}
		// Pull any recent events from history.
		history := m.engine.Bus().History(100)
		if len(history) > len(m.events) {
			m.events = history
		}
		return m, tickCmd()

	// ── EventBus event ──────────────────────────────────────────────
	case eventMsg:
		m.events = append(m.events, eventbus.Event(msg))
		return m, listenForEvents(m.eventSub)
	}

	return m, nil
}

// View renders the entire TUI based on the active tab.
func (m AppModel) View() string {
	if m.quitting {
		return lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Render("\n  👋 ChainHub shutting down…\n\n")
	}

	switch m.activeTab {
	case 1:
		return m.renderPipelineView()
	case 2:
		return m.renderLogView()
	default:
		return m.renderDashboard()
	}
}

// ─── Tab Bar ────────────────────────────────────────────────────────────────

// renderTabBar renders the horizontal tab selector.
func (m AppModel) renderTabBar() string {
	tabs := []string{"Dashboard", "Pipeline", "Logs"}
	rendered := make([]string, len(tabs))
	for i, t := range tabs {
		if i == m.activeTab {
			rendered[i] = TabActiveStyle.Render(t)
		} else {
			rendered[i] = TabInactiveStyle.Render(t)
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return lipgloss.NewStyle().
		MarginBottom(1).
		Render(bar)
}

// ─── Commands ───────────────────────────────────────────────────────────────

// tickCmd returns a tea.Cmd that fires a tickMsg after one second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// listenForEvents blocks on the subscriber channel and converts the received
// event into an eventMsg for the Update loop.
func listenForEvents(sub *eventbus.Subscriber) tea.Cmd {
	return func() tea.Msg {
		ev := <-sub.Channel
		return eventMsg(ev)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// truncate shortens s to maxLen characters, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 2 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
