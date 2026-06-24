package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m AppModel) renderInputOverlay(baseView string) string {
	if m.pendingPrompt == nil {
		return baseView
	}

	var b strings.Builder

	b.WriteString(HeaderStyle.Render("  Tool Needs Input"))
	b.WriteString("\n\n")

	tool := BoldTextStyle.Render(m.pendingPrompt.Tool)
	b.WriteString(fmt.Sprintf("  Tool: %s\n\n", tool))

	prompt := m.pendingPrompt.Prompt
	maxWidth := m.width - 20
	if len(prompt) > maxWidth {
		words := strings.Fields(prompt)
		line := ""
		for _, word := range words {
			if len(line)+len(word)+1 > maxWidth {
				b.WriteString(fmt.Sprintf("    %s\n", line))
				line = word
			} else {
				if line == "" {
					line = word
				} else {
					line += " " + word
				}
			}
		}
		if line != "" {
			b.WriteString(fmt.Sprintf("    %s\n", line))
		}
	} else {
		b.WriteString(fmt.Sprintf("    %s\n", prompt))
	}

	b.WriteString("\n")

	if m.inputMode {
		b.WriteString(StatusActiveStyle.Render("  > "))
		b.WriteString(m.inputBuffer)
		b.WriteString("_")
		b.WriteString("\n\n")
		b.WriteString(DimTextStyle.Render("  enter: send  |  esc: cancel"))
	} else {
		b.WriteString("  Actions:\n")
		b.WriteString(fmt.Sprintf("    %s  Accept (y)\n", StatusActiveStyle.Render("[y]")))
		b.WriteString(fmt.Sprintf("    %s  Reject (esc)\n", StatusErrorStyle.Render("[esc]")))
		b.WriteString(fmt.Sprintf("    %s  Custom input (i)\n", StatusWatchingStyle.Render("[i]")))
		b.WriteString(fmt.Sprintf("    %s  Toggle autopilot (a)\n", StatusActiveStyle.Render("[a]")))
	}

	if m.autopilotMode {
		b.WriteString("\n")
		b.WriteString(WarningTextStyle.Render("  AUTOPILOT ON"))
		b.WriteString(DimTextStyle.Render(" - auto-responding"))
	}

	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(b.String())

	return lipgloss.JoinVertical(lipgloss.Left, baseView, panel)
}
