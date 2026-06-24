package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/khurafati/chainhub/internal/adapter"
	"github.com/khurafati/chainhub/internal/core"
)

// renderDashboard produces the main dashboard view with four panels:
// header bar, pipeline overview, connected tools, and recent events.
func (m AppModel) renderDashboard() string {
	var sections []string

	// ─── Tab bar ────────────────────────────────────────────────────
	sections = append(sections, m.renderTabBar())

	// ─── Header bar ─────────────────────────────────────────────────
	sections = append(sections, m.renderHeaderBar())

	// ─── Body: pipeline + tools side-by-side, events below ─────────
	panelWidth := m.width - 6
	if panelWidth < 40 {
		panelWidth = 40
	}

	leftWidth := panelWidth * 55 / 100
	rightWidth := panelWidth - leftWidth - 2

	pipelinePanel := m.renderPipelinePanel(leftWidth)
	toolsPanel := m.renderToolsPanel(rightWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, pipelinePanel, "  ", toolsPanel)
	sections = append(sections, body)

	// ─── Events panel ───────────────────────────────────────────────
	sections = append(sections, m.renderEventsPanel(panelWidth))

	// ─── Footer ─────────────────────────────────────────────────────
	sections = append(sections, m.renderFooter())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// ─── Header Bar ─────────────────────────────────────────────────────────────

// renderHeaderBar produces the top bar with the ChainHub title on the left
// and live system metrics on the right.
func (m AppModel) renderHeaderBar() string {
	title := TitleStyle.Render("🔗 ChainHub Dashboard")

	cpu := fmt.Sprintf("CPU %.0f%%", m.metrics.CPUPercent)
	ram := fmt.Sprintf("RAM %.0f%%", m.metrics.MemoryPercent)
	disk := fmt.Sprintf("Disk %.0f%%", m.metrics.DiskPercent)

	cpuStyled := colorizeMetric(cpu, m.metrics.CPUPercent)
	ramStyled := colorizeMetric(ram, m.metrics.MemoryPercent)
	diskStyled := colorizeMetric(disk, m.metrics.DiskPercent)

	metrics := lipgloss.JoinHorizontal(lipgloss.Top,
		cpuStyled, "  ", ramStyled, "  ", diskStyled,
	)

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(metrics) - 4
	if gap < 1 {
		gap = 1
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		title,
		strings.Repeat(" ", gap),
		metrics,
	)

	return lipgloss.NewStyle().
		Background(ColorBgDark).
		Width(m.width).
		Padding(0, 1).
		MarginBottom(1).
		Render(bar)
}

// colorizeMetric styles a metric string green/yellow/red based on its value.
func colorizeMetric(label string, value float64) string {
	switch {
	case value >= 90:
		return StatusErrorStyle.Render(label)
	case value >= 70:
		return StatusWatchingStyle.Render(label)
	default:
		return StatusActiveStyle.Render(label)
	}
}

// ─── Pipeline Panel ─────────────────────────────────────────────────────────

// renderPipelinePanel shows the current pipeline's problem statement, phase
// list with status icons, and a progress bar.
func (m AppModel) renderPipelinePanel(width int) string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("⚡ Pipeline"))
	b.WriteString("\n")

	pipeline := m.engine.GetPipeline()
	if pipeline == nil {
		b.WriteString(DimTextStyle.Render("No active pipeline\n"))
		return PanelStyle.Width(width).Render(b.String())
	}

	// Problem.
	b.WriteString(BoldTextStyle.Render("Problem: "))
	prob := pipeline.Problem
	if len(prob) > width-12 {
		prob = prob[:width-15] + "..."
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Render(prob))
	b.WriteString("\n\n")

	// Phases.
	for i, phase := range pipeline.Phases {
		icon := dashboardPhaseIcon(phase.Status)
		name := string(phase.Phase)
		marker := " "
		if i == pipeline.CurrentPhase {
			marker = "►"
		}
		b.WriteString(fmt.Sprintf(" %s %s %-18s %s\n",
			DimTextStyle.Render(marker),
			icon,
			SubHeaderStyle.Render(name),
			renderDashboardPhaseStatus(phase.Status),
		))
	}

	// Progress bar.
	b.WriteString("\n")
	progress := pipeline.Progress()
	barW := width - 12
	if barW < 10 {
		barW = 10
	}
	b.WriteString(fmt.Sprintf(" %s %3.0f%%\n",
		RenderProgressBar(barW, progress),
		progress*100,
	))

	return PanelStyle.Width(width).Render(b.String())
}

// dashboardPhaseIcon maps PhaseStatus to a compact icon.
func dashboardPhaseIcon(status core.PhaseStatus) string {
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

// renderDashboardPhaseStatus returns a styled one-word status.
func renderDashboardPhaseStatus(status core.PhaseStatus) string {
	switch status {
	case core.PhaseStatusCompleted:
		return SuccessTextStyle.Render("done")
	case core.PhaseStatusActive:
		return StatusActiveStyle.Render("active")
	case core.PhaseStatusFailed:
		return StatusErrorStyle.Render("failed")
	default:
		return StatusIdleStyle.Render("pending")
	}
}

// ─── Tools Panel ────────────────────────────────────────────────────────────

// renderToolsPanel lists every registered tool with a status indicator and
// specialty tags.
func (m AppModel) renderToolsPanel(width int) string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("🛠  Connected Tools"))
	b.WriteString("\n")

	tools := m.registry.List()
	if len(tools) == 0 {
		b.WriteString(DimTextStyle.Render("No tools connected\n"))
		return PanelStyle.Width(width).Render(b.String())
	}

	for _, tool := range tools {
		info := tool.Info()
		icon := toolStatusIcon(info.Status)
		name := fmt.Sprintf("%-16s", info.DisplayName)

		// Specialty tags.
		specs := make([]string, len(info.Specialties))
		for j, s := range info.Specialties {
			specs[j] = string(s)
		}
		specStr := strings.Join(specs, ", ")

		statusText := info.StatusText
		if statusText == "" {
			statusText = string(info.Status)
		}

		b.WriteString(fmt.Sprintf(" %s %s %s\n",
			icon,
			BoldTextStyle.Render(name),
			toolStatusStyle(info.Status).Render(statusText),
		))
		if specStr != "" {
			b.WriteString(fmt.Sprintf("     %s\n", DimTextStyle.Render(specStr)))
		}
	}

	return PanelStyle.Width(width).Render(b.String())
}

// toolStatusIcon maps a ToolStatus to an emoji icon.
func toolStatusIcon(status adapter.ToolStatus) string {
	switch status {
	case adapter.ToolStatusActive:
		return "🟢"
	case adapter.ToolStatusWatching:
		return "👀"
	case adapter.ToolStatusIdle:
		return "⏳"
	case adapter.ToolStatusError:
		return "🔴"
	case adapter.ToolStatusStopped:
		return "⏹️"
	default:
		return "❓"
	}
}

// toolStatusStyle returns the lipgloss style matching a ToolStatus.
func toolStatusStyle(status adapter.ToolStatus) lipgloss.Style {
	switch status {
	case adapter.ToolStatusActive:
		return StatusActiveStyle
	case adapter.ToolStatusWatching:
		return StatusWatchingStyle
	case adapter.ToolStatusError:
		return StatusErrorStyle
	case adapter.ToolStatusStopped:
		return StatusStoppedStyle
	default:
		return StatusIdleStyle
	}
}

// ─── Events Panel ───────────────────────────────────────────────────────────

// renderEventsPanel shows the last 10 events in a compact list.
func (m AppModel) renderEventsPanel(width int) string {
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("📡 Recent Events"))
	b.WriteString("\n")

	if len(m.events) == 0 {
		b.WriteString(DimTextStyle.Render("Waiting for events…\n"))
		return PanelStyle.Width(width).Render(b.String())
	}

	start := 0
	if len(m.events) > 10 {
		start = len(m.events) - 10
	}

	for i := start; i < len(m.events); i++ {
		ev := m.events[i]
		ts := LogTimestampStyle.Render(ev.Timestamp.Format("15:04:05"))
		src := LogSourceStyle.Render(fmt.Sprintf("%-12s", truncate(ev.Source, 12)))
		etype := DimTextStyle.Render(string(ev.Type))
		b.WriteString(fmt.Sprintf(" %s  %s  %s\n", ts, src, etype))
	}

	return PanelStyle.Width(width).Render(b.String())
}

// ─── Footer ─────────────────────────────────────────────────────────────────

// renderFooter renders the bottom key-bindings bar.
func (m AppModel) renderFooter() string {
	keys := []string{
		"q: quit",
		"tab: views",
		"space: next",
		"a: autopilot",
	}
	if m.pendingPrompt != nil {
		keys = append(keys, "y: accept", "i: type", "esc: reject")
	}
		return FooterStyle.Width(m.width).Render("  " + strings.Join(keys, "  •  "))
}
