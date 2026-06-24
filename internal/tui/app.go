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

	activeTab int // 0 = dashboard, 1 = pipeline, 2 = logs, 3 = terminal
	logLines  []string
	quitting  bool

	// Terminal view state
	terminalTools       map[string][]ToolOutput
	selectedTerminalTool string

	// Input prompt state
	pendingPrompt  *PendingPrompt
	inputMode      bool
	inputBuffer    string
	autopilotMode  bool
}

// PendingPrompt represents a tool waiting for user input.
type PendingPrompt struct {
	Tool   string
	Prompt string
	Event  eventbus.Event
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
		terminalTools: make(map[string][]ToolOutput),
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
		// Handle input mode first
		if m.inputMode {
			switch msg.String() {
			case "enter":
				if m.inputBuffer != "" {
					m.sendInputResponse(m.inputBuffer)
				}
				m.inputBuffer = ""
				m.inputMode = false
				m.pendingPrompt = nil
				return m, nil
			case "escape":
				m.inputBuffer = ""
				m.inputMode = false
				m.pendingPrompt = nil
				return m, nil
			case "backspace":
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.inputBuffer += msg.String()
				}
				return m, nil
			}
		}

		// Normal mode
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 4
		case "1":
			m.activeTab = 0
		case "2":
			m.activeTab = 1
		case "3":
			m.activeTab = 2
		case "4":
			m.activeTab = 3
		case "n", " ":
			_ = m.engine.AdvancePipeline()
		case "left", "h":
			if m.activeTab == 3 {
				m.cycleTerminalTool(-1)
			}
		case "right", "l":
			if m.activeTab == 3 {
				m.cycleTerminalTool(1)
			}
		case "a":
			// Toggle autopilot mode
			m.autopilotMode = !m.autopilotMode
			m.toggleAutopilot()
			return m, nil
		case "i":
			// Enter input mode if there's a pending prompt
			if m.pendingPrompt != nil {
				m.inputMode = true
				m.inputBuffer = ""
				return m, nil
			}
		case "y":
			// Quick accept
			if m.pendingPrompt != nil {
				m.sendInputResponse("y")
				m.pendingPrompt = nil
				return m, nil
			}
		case "escape":
			// Quick reject
			if m.pendingPrompt != nil {
				m.sendInputResponse("n")
				m.pendingPrompt = nil
				return m, nil
			}
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
		ev := eventbus.Event(msg)

		// Handle input needed event
		if ev.Type == eventbus.EventInputNeeded {
			tool, _ := ev.Payload["tool"].(string)
			prompt, _ := ev.Payload["prompt"].(string)
			m.pendingPrompt = &PendingPrompt{
				Tool:   tool,
				Prompt: prompt,
				Event:  ev,
			}
			// Auto-respond if autopilot is on
			if m.autopilotMode {
				m.sendInputResponse("y")
				m.pendingPrompt = nil
			}
		}

		// Cap events list
		if len(m.events) > 200 {
			m.events = m.events[len(m.events)-200:]
		}
		m.events = append(m.events, ev)

		const maxTerminalLines = 500

		// Capture tool output for terminal view
		switch ev.Type {
		case eventbus.EventToolOutput:
			if tool, ok := ev.Payload["tool"].(string); ok {
				if line, ok := ev.Payload["line"].(string); ok {
					m.terminalTools[tool] = append(m.terminalTools[tool], ToolOutput{
						Tool:  tool,
						Line:  line,
						IsErr: false,
					})
					if m.selectedTerminalTool == "" {
						m.selectedTerminalTool = tool
					}
				}
			}
		case eventbus.EventToolError:
			if tool, ok := ev.Payload["tool"].(string); ok {
				if line, ok := ev.Payload["line"].(string); ok {
					m.terminalTools[tool] = append(m.terminalTools[tool], ToolOutput{
						Tool:  tool,
						Line:  line,
						IsErr: true,
					})
				}
			}
		case eventbus.EventToolStatusChanged:
			if tool, ok := ev.Payload["tool"].(string); ok {
				if _, exists := m.terminalTools[tool]; !exists {
					m.terminalTools[tool] = make([]ToolOutput, 0)
					if m.selectedTerminalTool == "" {
						m.selectedTerminalTool = tool
					}
				}
			}
		}

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
			Render("\n  ChainHub shutting down...\n\n")
	}

	var view string
	switch m.activeTab {
	case 1:
		view = m.renderPipelineView()
	case 2:
		view = m.renderLogView()
	case 3:
		view = m.renderTerminalView()
	default:
		view = m.renderDashboard()
	}

	// Append input prompt overlay if there's a pending prompt
	if m.pendingPrompt != nil || m.inputMode {
		view = m.renderInputOverlay(view)
	}

	return view
}

// ─── Tab Bar ────────────────────────────────────────────────────────────────

// renderTabBar renders the horizontal tab selector.
func (m AppModel) renderTabBar() string {
	tabs := []string{"Dashboard", "Pipeline", "Logs", "Terminal"}
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
		ev, ok := <-sub.Channel
		if !ok {
			return nil
		}
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

// cycleTerminalTool switches to the next/previous tool in the terminal view.
func (m *AppModel) cycleTerminalTool(direction int) {
	if len(m.terminalTools) == 0 {
		return
	}

	// Get sorted tool names
	names := make([]string, 0, len(m.terminalTools))
	for name := range m.terminalTools {
		names = append(names, name)
	}

	// Find current index
	currentIdx := 0
	for i, name := range names {
		if name == m.selectedTerminalTool {
			currentIdx = i
			break
		}
	}

	// Cycle
	newIdx := (currentIdx + direction + len(names)) % len(names)
	m.selectedTerminalTool = names[newIdx]
}

// sendInputResponse sends a response to the pending tool prompt.
func (m *AppModel) sendInputResponse(response string) {
	if m.pendingPrompt == nil {
		return
	}

	toolName := m.pendingPrompt.Tool
	if a, err := m.registry.Get(toolName); err == nil {
		if adapter, ok := a.(interface{ RespondToPrompt(string) error }); ok {
			_ = adapter.RespondToPrompt(response)
		}
	}

	// Log the response
	m.events = append(m.events, eventbus.Event{
		Type:      eventbus.EventInputResponse,
		Source:    "user",
		Payload:   map[string]interface{}{"tool": toolName, "response": response},
		Timestamp: time.Now(),
	})
}

// toggleAutopilot toggles autopilot mode on all registered tools.
func (m *AppModel) toggleAutopilot() {
	for _, a := range m.registry.List() {
		if adapter, ok := a.(interface{ SetAutopilot(bool) }); ok {
			adapter.SetAutopilot(m.autopilotMode)
		}
	}

	m.events = append(m.events, eventbus.Event{
		Type:      eventbus.EventAutopilotToggle,
		Source:    "user",
		Payload:   map[string]interface{}{"enabled": m.autopilotMode},
		Timestamp: time.Now(),
	})
}
