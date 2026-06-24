package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderInputOverlay renders a prompt overlay on top of the current view.
func (m AppModel) renderInputOverlay(baseView string) string {
	if m.pendingPrompt == nil {
		return baseView
	}

	var b strings.Builder

	// Title bar
	title := HeaderStyle.Render("  Tool Needs Input")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Tool name and prompt
	tool := BoldTextStyle.Render(m.pendingPrompt.Tool)
	b.WriteString(fmt.Sprintf("  Tool: %s\n\n", tool))

	// Prompt text - wrap long lines
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

	// Input area or quick actions
	if m.inputMode {
		b.WriteString(StatusActiveStyle.Render("  > "))
		b.WriteString(m.inputBuffer)
		b.WriteString("_")
		b.WriteString("\n\n")
		b.WriteString(DimTextStyle.Render("  enter: send  |  escape: cancel"))
	} else {
		b.WriteString("  Quick actions:\n")
		b.WriteString(fmt.Sprintf("    %s  Accept (y)\n", StatusActiveStyle.Render("[y]")))
		b.WriteString(fmt.Sprintf("    %s  Reject (escape)\n", StatusErrorStyle.Render("[esc]")))
		b.WriteString(fmt.Sprintf("    %s  Type custom response\n", StatusWatchingStyle.Render("[i]")))
		b.WriteString(fmt.Sprintf("    %s  Toggle autopilot\n", StatusActiveStyle.Render("[a]")))
	}

	// Autopilot status
	if m.autopilotMode {
		b.WriteString("\n")
		b.WriteString(WarningTextStyle.Render("  AUTOPILOT ON"))
		b.WriteString(DimTextStyle.Render(" - Tool will auto-respond to prompts"))
	}

	// Panel
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(b.String())

	return lipgloss.JoinVertical(lipgloss.Left, baseView, panel)
}
