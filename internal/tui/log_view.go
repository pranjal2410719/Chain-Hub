package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/eventbus"
)

func (m AppModel) renderLogView() string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("  Activity Log"))
	b.WriteString("\n\n")

	events := m.events
	if len(events) == 0 {
		b.WriteString(DimTextStyle.Render("  No activity yet. Start a pipeline to see events.\n"))
		return m.wrapLogPanel(b.String())
	}

	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}

	start := 0
	if len(events) > maxLines {
		start = len(events) - maxLines
	}

	for i := start; i < len(events); i++ {
		ev := events[i]
		if ev.Type == eventbus.EventToolOutput || ev.Type == eventbus.EventToolError {
			continue
		}
		line := formatFriendlyLogEntry(ev, m.width-8)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(DimTextStyle.Render(fmt.Sprintf("  %d events", len(events))))

	return m.wrapLogPanel(b.String())
}

func formatFriendlyLogEntry(ev eventbus.Event, maxWidth int) string {
	ts := DimTextStyle.Render(ev.Timestamp.Format("15:04:05"))

	switch ev.Type {
	case eventbus.EventToolStatusChanged:
		return formatToolStatus(ts, ev)
	case eventbus.EventPipelinePhaseChanged:
		return formatPhaseChange(ts, ev)
	case eventbus.EventTaskAssigned:
		return formatTaskAssigned(ts, ev)
	case eventbus.EventTaskCompleted:
		return formatTaskCompleted(ts, ev)
	case eventbus.EventInputNeeded:
		return formatInputNeeded(ts, ev, maxWidth)
	case eventbus.EventInputResponse:
		return formatInputResponse(ts, ev)
	case eventbus.EventAutopilotToggle:
		return formatAutopilotToggle(ts, ev)
	case eventbus.EventSystemAlert:
		return formatSystemAlert(ts, ev, maxWidth)
	default:
		return formatGenericEvent(ts, ev, maxWidth)
	}
}

func (m AppModel) wrapLogPanel(content string) string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(content)

	tabs := m.renderTabBar()
	footer := FooterStyle.Width(m.width).Render(
		"  q quit  \u2022  tab switch  \u2022  space next  \u2022  a autopilot  \u2022  1-4 tabs",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, panel, footer)
}
