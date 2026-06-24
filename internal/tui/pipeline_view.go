package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/core"
)

// renderPipelineView produces a detailed pipeline breakdown showing every
// phase, assigned tools, status indicators, and overall progress.
func (m AppModel) renderPipelineView() string {
	var b strings.Builder

	pipeline := m.engine.GetPipeline()

	// ─── Title ──────────────────────────────────────────────────────
	title := HeaderStyle.Render("🔗 Pipeline Detail")
	b.WriteString(title)
	b.WriteString("\n\n")

	if pipeline == nil {
		b.WriteString(DimTextStyle.Render("  No active pipeline. Use `chainhub run` to start one.\n"))
		return m.wrapPipelinePanel(b.String())
	}

	// ─── Pipeline meta ──────────────────────────────────────────────
	b.WriteString(BoldTextStyle.Render("  Pipeline ID: "))
	b.WriteString(DimTextStyle.Render(pipeline.ID))
	b.WriteString("\n")

	b.WriteString(BoldTextStyle.Render("  Problem:     "))
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Render(pipeline.Problem))
	b.WriteString("\n")

	b.WriteString(BoldTextStyle.Render("  Status:      "))
	b.WriteString(renderPipelineStatus(pipeline.Status))
	b.WriteString("\n")

	b.WriteString(BoldTextStyle.Render("  Created:     "))
	b.WriteString(DimTextStyle.Render(pipeline.CreatedAt.Format("2006-01-02 15:04:05")))
	b.WriteString("\n\n")

	// ─── Progress bar ───────────────────────────────────────────────
	progress := pipeline.Progress()
	progressPct := fmt.Sprintf(" %.0f%%", progress*100)
	barWidth := m.width - 20
	if barWidth < 20 {
		barWidth = 20
	}
	b.WriteString("  " + RenderProgressBar(barWidth, progress) + progressPct)
	b.WriteString("\n\n")

	// ─── Phases ─────────────────────────────────────────────────────
	b.WriteString(SubHeaderStyle.Render("  Phases"))
	b.WriteString("\n")
	b.WriteString(DimTextStyle.Render("  " + strings.Repeat("─", 60)))
	b.WriteString("\n\n")

	for i, phase := range pipeline.Phases {
		b.WriteString(renderPhaseDetail(i, phase, pipeline.CurrentPhase))
		b.WriteString("\n")
	}

	// ─── Feedback loop indicators ───────────────────────────────────
	b.WriteString("\n")
	b.WriteString(SubHeaderStyle.Render("  Feedback Loops"))
	b.WriteString("\n")
	if pipeline.IsComplete() {
		b.WriteString(SuccessTextStyle.Render("  ✅ Pipeline complete — all phases finished."))
	} else {
		cur := pipeline.CurrentPhaseConfig()
		if cur != nil {
			b.WriteString(DimTextStyle.Render(fmt.Sprintf(
				"  🔄 Active feedback loop on phase %d: %s",
				pipeline.CurrentPhase+1, cur.Phase,
			)))
		} else {
			b.WriteString(DimTextStyle.Render("  ⏳ Awaiting phase activation..."))
		}
	}
	b.WriteString("\n")

	return m.wrapPipelinePanel(b.String())
}

// renderPhaseDetail renders a single phase as a multi-line detail block.
func renderPhaseDetail(index int, phase *core.PhaseConfig, currentPhase int) string {
	var b strings.Builder

	icon := phaseStatusIcon(phase.Status)
	phaseName := string(phase.Phase)

	// Highlight the current phase.
	nameStyle := DimTextStyle
	if index == currentPhase {
		nameStyle = BoldTextStyle
	}

	b.WriteString(fmt.Sprintf("  %s  %s %s\n",
		icon,
		nameStyle.Render(fmt.Sprintf("Phase %d:", index+1)),
		SubHeaderStyle.Render(phaseName),
	))

	// Status.
	b.WriteString(fmt.Sprintf("       Status: %s\n", renderPhaseStatusText(phase.Status)))

	// Assigned tools.
	if len(phase.AssignedTools) > 0 {
		tools := strings.Join(phase.AssignedTools, ", ")
		b.WriteString(fmt.Sprintf("       Tools:  %s\n", DimTextStyle.Render(tools)))
	} else {
		b.WriteString(fmt.Sprintf("       Tools:  %s\n", DimTextStyle.Render("(none assigned)")))
	}

	return b.String()
}

// renderPipelineStatus returns a styled string for a pipeline-level status.
func renderPipelineStatus(status core.PipelineStatus) string {
	switch status {
	case core.PipelineStatusRunning:
		return StatusActiveStyle.Render("● Running")
	case core.PipelineStatusComplete:
		return SuccessTextStyle.Render("✅ Complete")
	default:
		return StatusIdleStyle.Render("○ Pending")
	}
}

// renderPhaseStatusText returns a styled string for a phase status.
func renderPhaseStatusText(status core.PhaseStatus) string {
	switch status {
	case core.PhaseStatusCompleted:
		return SuccessTextStyle.Render("Completed")
	case core.PhaseStatusActive:
		return StatusActiveStyle.Render("Active")
	case core.PhaseStatusFailed:
		return StatusErrorStyle.Render("Failed")
	default:
		return StatusIdleStyle.Render("Pending")
	}
}

// phaseStatusIcon maps a PhaseStatus to a unicode icon string.
func phaseStatusIcon(status core.PhaseStatus) string {
	switch status {
	case core.PhaseStatusCompleted:
		return "✅"
	case core.PhaseStatusActive:
		return "🔄"
	case core.PhaseStatusFailed:
		return "❌"
	default:
		return "⏳"
	}
}

// wrapPipelinePanel wraps the pipeline content in a styled panel sized to the
// terminal, including the tab bar and footer.
func (m AppModel) wrapPipelinePanel(content string) string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	panel := PanelStyle.Width(w).Render(content)

	tabs := m.renderTabBar()
	footer := FooterStyle.Width(m.width).Render(
		"  q quit • tab switch view • space/n next phase • 1-3 jump to tab",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, panel, footer)
}
