// Package tui provides a terminal user interface for ChainHub using the
// Bubbletea framework. This file defines the shared lipgloss style
// definitions and color palette used across every TUI view.
package tui

import "github.com/charmbracelet/lipgloss"

// ─── Color Palette ──────────────────────────────────────────────────────────

const (
	// Primary brand colors.
	ColorPrimary   = lipgloss.Color("#7C3AED") // violet
	ColorSecondary = lipgloss.Color("#06B6D4") // cyan

	// Semantic status colors.
	ColorSuccess = lipgloss.Color("#10B981") // green
	ColorWarning = lipgloss.Color("#F59E0B") // amber
	ColorError   = lipgloss.Color("#EF4444") // red

	// Backgrounds.
	ColorBgDark  = lipgloss.Color("#1E1E2E")
	ColorBgPanel = lipgloss.Color("#2E2E3E")

	// Text colors.
	ColorTextPrimary = lipgloss.Color("#E2E8F0")
	ColorTextDim     = lipgloss.Color("#94A3B8")

	// Extra accent colors used for contrast.
	ColorAccent     = lipgloss.Color("#A78BFA") // light violet
	ColorBorder     = lipgloss.Color("#4C4C6D")
	ColorHighlight  = lipgloss.Color("#818CF8") // indigo
	ColorBgFooter   = lipgloss.Color("#252538")
)

// ─── Title & Header Styles ─────────────────────────────────────────────────

// TitleStyle is used for the main dashboard title.
var TitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorPrimary).
	PaddingLeft(1).
	PaddingRight(1)

// HeaderStyle is used for panel section headings.
var HeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Underline(true).
	Foreground(ColorSecondary).
	MarginBottom(1)

// SubHeaderStyle is a lighter heading used inside panels.
var SubHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorAccent)

// ─── Panel Styles ───────────────────────────────────────────────────────────

// PanelStyle defines a bordered panel with a dark background.
var PanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorBorder).
	Background(ColorBgPanel).
	Padding(1, 2).
	MarginBottom(1)

// ActivePanelStyle highlights the currently selected panel.
var ActivePanelStyle = PanelStyle.
	BorderForeground(ColorPrimary)

// FooterStyle styles the bottom key-binding bar.
var FooterStyle = lipgloss.NewStyle().
	Background(ColorBgFooter).
	Foreground(ColorTextDim).
	Padding(0, 1).
	MarginTop(1)

// ─── Status Indicator Styles ────────────────────────────────────────────────

// StatusActiveStyle represents a healthy / active status.
var StatusActiveStyle = lipgloss.NewStyle().
	Foreground(ColorSuccess).
	Bold(true)

// StatusIdleStyle represents an idle or waiting status.
var StatusIdleStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim)

// StatusErrorStyle represents an error status.
var StatusErrorStyle = lipgloss.NewStyle().
	Foreground(ColorError).
	Bold(true)

// StatusWatchingStyle represents a watching/monitoring status.
var StatusWatchingStyle = lipgloss.NewStyle().
	Foreground(ColorWarning)

// StatusStoppedStyle represents a stopped tool.
var StatusStoppedStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim).
	Faint(true)

// ─── Progress & Metrics Styles ──────────────────────────────────────────────

// ProgressBarStyle colors the filled portion of progress bars.
var ProgressBarStyle = lipgloss.NewStyle().
	Foreground(ColorPrimary)

// ProgressBarEmptyStyle colors the unfilled portion of progress bars.
var ProgressBarEmptyStyle = lipgloss.NewStyle().
	Foreground(ColorBorder)

// MetricLabelStyle is used for metric labels (CPU, RAM, etc.).
var MetricLabelStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim).
	Width(6)

// MetricValueStyle is used to display metric values.
var MetricValueStyle = lipgloss.NewStyle().
	Foreground(ColorTextPrimary).
	Bold(true)

// ─── Event & Log Styles ────────────────────────────────────────────────────

// EventStyle is used for event log entries.
var EventStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim)

// LogStyle is used for general log output.
var LogStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim)

// LogTimestampStyle highlights timestamps in log lines.
var LogTimestampStyle = lipgloss.NewStyle().
	Foreground(ColorBorder)

// LogSourceStyle highlights the source tool in log lines.
var LogSourceStyle = lipgloss.NewStyle().
	Foreground(ColorSecondary).
	Bold(true)

// ─── Tab Bar Styles ─────────────────────────────────────────────────────────

// TabActiveStyle styles the currently selected tab.
var TabActiveStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorPrimary).
	Border(lipgloss.NormalBorder(), false, false, true, false).
	BorderForeground(ColorPrimary).
	Padding(0, 2)

// TabInactiveStyle styles unselected tabs.
var TabInactiveStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim).
	Padding(0, 2)

// ─── Misc Styles ────────────────────────────────────────────────────────────

// DimTextStyle dims secondary information.
var DimTextStyle = lipgloss.NewStyle().
	Foreground(ColorTextDim)

// BoldTextStyle is a convenience for bold white text.
var BoldTextStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorTextPrimary)

// SuccessTextStyle colors text green for success messages.
var SuccessTextStyle = lipgloss.NewStyle().
	Foreground(ColorSuccess)

// WarningTextStyle colors text amber for warnings.
var WarningTextStyle = lipgloss.NewStyle().
	Foreground(ColorWarning)

// ErrorTextStyle colors text red for errors.
var ErrorTextStyle = lipgloss.NewStyle().
	Foreground(ColorError)

// ─── Helper Functions ───────────────────────────────────────────────────────

// RenderProgressBar draws a horizontal progress bar of the given width and
// completion fraction (0.0–1.0).
func RenderProgressBar(width int, percent float64) string {
	if width < 4 {
		width = 4
	}
	// Reserve 2 chars for brackets.
	inner := width - 2
	filled := int(float64(inner) * percent)
	if filled > inner {
		filled = inner
	}
	empty := inner - filled

	bar := "["
	bar += ProgressBarStyle.Render(repeatChar('█', filled))
	bar += ProgressBarEmptyStyle.Render(repeatChar('░', empty))
	bar += "]"
	return bar
}

// repeatChar repeats a rune n times and returns the resulting string.
func repeatChar(ch rune, n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]rune, n)
	for i := range buf {
		buf[i] = ch
	}
	return string(buf)
}
