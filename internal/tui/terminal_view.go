package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/eventbus"
)

type ToolOutput struct {
	Tool  string
	Line  string
	IsErr bool
}

func isToolPrompt(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	prompts := []string{
		"yes, i trust",
		"no, exit",
		"do you want",
		"press enter",
		"continue?",
		"proceed?",
		"allow",
		"confirm",
		"accept",
		"reject",
		"[y/n]",
		"[yes/no]",
	}
	for _, p := range prompts {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func (m AppModel) renderTerminalView() string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("  Live Terminal"))
	b.WriteString("\n\n")

	if len(m.terminalTools) == 0 {
		b.WriteString(DimTextStyle.Render("  No active tools. Start a pipeline to see output.\n"))
		return m.wrapTerminalPanel(b.String())
	}

	names := make([]string, 0, len(m.terminalTools))
	for name := range m.terminalTools {
		names = append(names, name)
	}
	sort.Strings(names)

	toolTabs := make([]string, 0, len(names))
	for _, name := range names {
		lines := m.terminalTools[name]
		count := fmt.Sprintf("(%d)", len(lines))
		if name == m.selectedTerminalTool {
			toolTabs = append(toolTabs, TabActiveStyle.Render(" "+name+" "+count+" "))
		} else {
			toolTabs = append(toolTabs, TabInactiveStyle.Render(" "+name+" "+count+" "))
		}
	}
	toolBar := lipgloss.JoinHorizontal(lipgloss.Top, toolTabs...)
	b.WriteString(toolBar)
	b.WriteString("\n\n")

	selected := m.selectedTerminalTool
	if selected == "" && len(names) > 0 {
		selected = names[0]
	}

	lines, exists := m.terminalTools[selected]
	if !exists || len(lines) == 0 {
		b.WriteString(DimTextStyle.Render("  Waiting for output from " + selected + "...\n"))
		return m.wrapTerminalPanel(b.String())
	}

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
		text := line.Line

		if isToolPrompt(text) {
			continue
		}

		text = strings.TrimSpace(text)
		if text == "" || text == "\n" || text == "\r" {
			continue
		}

		maxWidth := m.width - 16
		if len(text) > maxWidth {
			text = text[:maxWidth-3] + "..."
		}

		var styled string
		if line.IsErr {
			styled = StatusErrorStyle.Render(text)
		} else {
			styled = LogStyle.Render(text)
		}
		b.WriteString("  ")
		b.WriteString(styled)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	status := fmt.Sprintf("  %d lines | %s", len(lines), selected)
	b.WriteString(DimTextStyle.Render(status))

	return m.wrapTerminalPanel(b.String())
}

func (m AppModel) wrapTerminalPanel(content string) string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(content)

	tabs := m.renderTabBar()
	footer := FooterStyle.Width(m.width).Render(
		"  q quit  \u2022  tab switch  \u2022  \u2190\u2192 tools  \u2022  1-4 tabs",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, panel, footer)
}

func formatToolStatus(ts string, ev eventbus.Event) string {
	tool, _ := ev.Payload["tool"].(string)
	status, _ := ev.Payload["status"].(string)

	var icon, msg string
	switch adapter.ToolStatus(status) {
	case adapter.ToolStatusActive:
		icon = StatusActiveStyle.Render("  \u25cf")
		msg = StatusActiveStyle.Render(tool + " started")
	case adapter.ToolStatusStopped:
		icon = DimTextStyle.Render("  \u25cb")
		msg = DimTextStyle.Render(tool + " stopped")
	case adapter.ToolStatusError:
		icon = StatusErrorStyle.Render("  \u2717")
		text, _ := ev.Payload["text"].(string)
		if text == "" {
			text = "error"
		}
		msg = StatusErrorStyle.Render(tool + " " + text)
	default:
		icon = DimTextStyle.Render("  \u00b7")
		msg = DimTextStyle.Render(tool + " " + status)
	}

	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatPhaseChange(ts string, ev eventbus.Event) string {
	phase, _ := ev.Payload["phase"].(string)
	status, _ := ev.Payload["status"].(string)

	var icon string
	switch status {
	case "running":
		icon = StatusActiveStyle.Render("  \u25b6")
	case "complete":
		icon = SuccessTextStyle.Render("  \u2713")
	case "failed":
		icon = StatusErrorStyle.Render("  \u2717")
	default:
		icon = DimTextStyle.Render("  \u25cb")
	}

	msg := fmt.Sprintf("Phase %s %s", BoldTextStyle.Render(phase), DimTextStyle.Render(status))
	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatTaskAssigned(ts string, ev eventbus.Event) string {
	tool, _ := ev.Payload["tool"].(string)
	phase, _ := ev.Payload["phase"].(string)

	icon := StatusWatchingStyle.Render("  \u2192")
	msg := fmt.Sprintf("%s assigned to %s",
		DimTextStyle.Render(phase),
		BoldTextStyle.Render(tool),
	)
	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatTaskCompleted(ts string, ev eventbus.Event) string {
	tool, _ := ev.Payload["tool"].(string)

	icon := SuccessTextStyle.Render("  \u2713")
	msg := fmt.Sprintf("%s completed", BoldTextStyle.Render(tool))
	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatInputNeeded(ts string, ev eventbus.Event, maxWidth int) string {
	tool, _ := ev.Payload["tool"].(string)
	prompt, _ := ev.Payload["prompt"].(string)

	icon := WarningTextStyle.Render("  ?")
	toolStr := BoldTextStyle.Render(tool)

	promptText := prompt
	if len(promptText) > maxWidth-40 {
		promptText = promptText[:maxWidth-43] + "..."
	}

	return fmt.Sprintf("  %s  %s  %s needs input: %s", ts, icon, toolStr, DimTextStyle.Render(promptText))
}

func formatInputResponse(ts string, ev eventbus.Event) string {
	tool, _ := ev.Payload["tool"].(string)
	response, _ := ev.Payload["response"].(string)

	icon := StatusActiveStyle.Render("  \u2190")
	msg := fmt.Sprintf("%s responded: %s", BoldTextStyle.Render(tool), StatusActiveStyle.Render(response))
	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatAutopilotToggle(ts string, ev eventbus.Event) string {
	enabled, _ := ev.Payload["enabled"].(bool)

	icon := StatusWatchingStyle.Render("  \u2699")
	var msg string
	if enabled {
		msg = WarningTextStyle.Render("Autopilot ON") + DimTextStyle.Render(" - auto-responding")
	} else {
		msg = StatusActiveStyle.Render("Autopilot OFF") + DimTextStyle.Render(" - manual input")
	}
	return fmt.Sprintf("  %s  %s  %s", ts, icon, msg)
}

func formatSystemAlert(ts string, ev eventbus.Event, maxWidth int) string {
	msg, _ := ev.Payload["message"].(string)

	if msg == "" {
		msg = "system alert"
	}
	if len(msg) > maxWidth-25 {
		msg = msg[:maxWidth-28] + "..."
	}

	icon := WarningTextStyle.Render("  \u26a0")
	content := WarningTextStyle.Render(msg)
	return fmt.Sprintf("  %s  %s  %s", ts, icon, content)
}

func formatGenericEvent(ts string, ev eventbus.Event, maxWidth int) string {
	icon := DimTextStyle.Render("  \u00b7")
	typeStr := DimTextStyle.Render(string(ev.Type))

	var detail string
	if len(ev.Payload) > 0 {
		parts := make([]string, 0, len(ev.Payload))
		for k, v := range ev.Payload {
			val := fmt.Sprintf("%v", v)
			if len(val) > 30 {
				val = val[:27] + "..."
			}
			parts = append(parts, fmt.Sprintf("%s=%s", k, val))
		}
		detail = strings.Join(parts, " ")
		if len(detail) > maxWidth-40 {
			detail = detail[:maxWidth-43] + "..."
		}
	}

	if detail != "" {
		return fmt.Sprintf("  %s  %s  %s  %s", ts, icon, typeStr, DimTextStyle.Render(detail))
	}
	return fmt.Sprintf("  %s  %s  %s", ts, icon, typeStr)
}
