package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ToolOutput represents a line of output from a tool process.
type ToolOutput struct {
	Tool  string
	Line  string
	IsErr bool
}

// renderTerminalView produces a live terminal-style view showing real-time
// output from the currently running tools.
func (m AppModel) renderTerminalView() string {
	var b strings.Builder

	// ─── Title ──────────────────────────────────────────────────────
	title := HeaderStyle.Render("🖥  Live Terminal")
	b.WriteString(title)
	b.WriteString("\n\n")

	// ─── Tool selector tabs ────────────────────────────────────────
	if len(m.terminalTools) == 0 {
		b.WriteString(DimTextStyle.Render("  No active tools. Start a pipeline to see output.\n"))
		return m.wrapTerminalPanel(b.String())
	}

	// Tool tabs
	toolTabs := make([]string, 0, len(m.terminalTools))
	for name := range m.terminalTools {
		if name == m.selectedTerminalTool {
			toolTabs = append(toolTabs, TabActiveStyle.Render(" "+name+" "))
		} else {
			toolTabs = append(toolTabs, TabInactiveStyle.Render(" "+name+" "))
		}
	}
	toolBar := lipgloss.JoinHorizontal(lipgloss.Top, toolTabs...)
	b.WriteString(toolBar)
	b.WriteString("\n\n")

	// ─── Terminal output ───────────────────────────────────────────
	selected := m.selectedTerminalTool
	if selected == "" && len(m.terminalTools) > 0 {
		for name := range m.terminalTools {
			selected = name
			break
		}
	}

	lines, exists := m.terminalTools[selected]
	if !exists || len(lines) == 0 {
		b.WriteString(DimTextStyle.Render("  Waiting for output...\n"))
		return m.wrapTerminalPanel(b.String())
	}

	// Show last N lines that fit the screen
	maxLines := m.height - 14
	if maxLines < 5 {
		maxLines = 5
	}
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}

	for i := start; i < len(lines); i++ {
		line := lines[i]
		styled := renderTerminalLine(line, m.width-8)
		b.WriteString(styled)
		b.WriteString("\n")
	}

	// Status bar
	b.WriteString("\n")
	status := fmt.Sprintf("  %d lines | %s", len(lines), selected)
	b.WriteString(DimTextStyle.Render(status))

	return m.wrapTerminalPanel(b.String())
}

// renderTerminalLine styles a single terminal output line.
func renderTerminalLine(line ToolOutput, maxWidth int) string {
	ts := LogTimestampStyle.Render(line.Line[:min(8, len(line.Line))])

	var content string
	if line.IsErr {
		content = StatusErrorStyle.Render(truncate(line.Line, maxWidth-20))
	} else {
		content = LogStyle.Render(truncate(line.Line, maxWidth-20))
	}

	return fmt.Sprintf("  %s  %s", ts, content)
}

// wrapTerminalPanel wraps the terminal content in a styled panel.
func (m AppModel) wrapTerminalPanel(content string) string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(content)

	tabs := m.renderTabBar()
	footer := FooterStyle.Width(m.width).Render(
		"  q quit • tab switch view • ← → switch tool • 1-4 jump to tab",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, panel, footer)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
