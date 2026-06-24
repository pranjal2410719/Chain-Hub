package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/eventbus"
)

// renderLogView produces a full-screen log view of all recorded events,
// color-coded by event type, with timestamps, source tool names, and payload
// summaries.
func (m AppModel) renderLogView() string {
	var b strings.Builder

	// ─── Title ──────────────────────────────────────────────────────
	title := HeaderStyle.Render("📜 Event Log")
	b.WriteString(title)
	b.WriteString("\n\n")

	// ─── Events ─────────────────────────────────────────────────────
	events := m.events
	if len(events) == 0 {
		b.WriteString(DimTextStyle.Render("  No events recorded yet. Start a pipeline to see activity.\n"))
		return m.wrapLogPanel(b.String())
	}

	// Show newest first, capped at visible lines.
	maxLines := m.height - 10
	if maxLines < 5 {
		maxLines = 5
	}
	start := 0
	if len(events) > maxLines {
		start = len(events) - maxLines
	}

	for i := start; i < len(events); i++ {
		ev := events[i]
		b.WriteString(formatLogEntry(ev, m.width-8))
		b.WriteString("\n")
	}

	// ─── Footer hint ────────────────────────────────────────────────
	b.WriteString("\n")
	b.WriteString(DimTextStyle.Render(fmt.Sprintf("  Showing %d of %d events", len(events)-start, len(events))))

	return m.wrapLogPanel(b.String())
}

// formatLogEntry formats a single event as a coloured log line.
func formatLogEntry(ev eventbus.Event, maxWidth int) string {
	ts := LogTimestampStyle.Render(ev.Timestamp.Format("15:04:05"))
	src := LogSourceStyle.Render(fmt.Sprintf("%-14s", truncate(ev.Source, 14)))
	typeStr := formatEventType(ev.Type)
	payload := formatPayloadSummary(ev.Payload, maxWidth-40)

	return fmt.Sprintf("  %s  %s  %s  %s", ts, src, typeStr, payload)
}

// formatEventType returns a coloured representation of an event type.
func formatEventType(t eventbus.EventType) string {
	s := string(t)
	// Pick a style by event semantics.
	switch {
	case strings.Contains(s, "Completed"), strings.Contains(s, "Success"):
		return StatusActiveStyle.Render(fmt.Sprintf("%-22s", s))
	case strings.Contains(s, "Error"), strings.Contains(s, "Failed"):
		return StatusErrorStyle.Render(fmt.Sprintf("%-22s", s))
	case strings.Contains(s, "Assigned"), strings.Contains(s, "Started"):
		return StatusWatchingStyle.Render(fmt.Sprintf("%-22s", s))
	case strings.Contains(s, "Alert"), strings.Contains(s, "Warning"):
		return WarningTextStyle.Render(fmt.Sprintf("%-22s", s))
	default:
		return DimTextStyle.Render(fmt.Sprintf("%-22s", s))
	}
}

// formatPayloadSummary produces a compact one-line summary of a payload map.
func formatPayloadSummary(payload map[string]interface{}, maxLen int) string {
	if len(payload) == 0 {
		return DimTextStyle.Render("—")
	}

	parts := make([]string, 0, len(payload))
	for k, v := range payload {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	summary := strings.Join(parts, " ")
	if len(summary) > maxLen && maxLen > 3 {
		summary = summary[:maxLen-3] + "..."
	}
	return LogStyle.Render(summary)
}

// wrapLogPanel wraps the log content in a styled panel sized to the terminal.
func (m AppModel) wrapLogPanel(content string) string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(content)

	// Tab bar + panel.
	tabs := m.renderTabBar()
	footer := FooterStyle.Width(m.width).Render(
		"  q quit • tab switch view • space/n next phase • 1-3 jump to tab",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, panel, footer)
}
