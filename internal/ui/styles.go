package ui

import "github.com/charmbracelet/lipgloss"

// Palette
const (
	ColorAccent  = lipgloss.Color("#818CF8") // indigo-400
	ColorWorking = lipgloss.Color("#34D399") // emerald-400
	ColorWaiting = lipgloss.Color("#FB923C") // orange-400
	ColorIdle    = lipgloss.Color("#94A3B8") // slate-400
	ColorBlocked = lipgloss.Color("#F87171") // red-400
	ColorMuted   = lipgloss.Color("#475569") // slate-600
	ColorBorder  = lipgloss.Color("#1E293B") // slate-800
	ColorSel     = lipgloss.Color("#0F172A") // slate-900
	ColorText    = lipgloss.Color("#CBD5E1") // slate-300
	ColorDim     = lipgloss.Color("#64748B") // slate-500
)

// StatusColor returns the colour for a given agent status string.
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "working":
		return ColorWorking
	case "waiting":
		return ColorWaiting
	case "blocked":
		return ColorBlocked
	case "idle":
		return ColorIdle
	default:
		return ColorMuted
	}
}

// StatusDot returns a coloured diamond for the given status.
func StatusDot(status string) string {
	glyph := "◆"
	if status == "idle" {
		glyph = "◇"
	}
	return lipgloss.NewStyle().Foreground(StatusColor(status)).Render(glyph)
}

// Component styles — defined once, reused everywhere.
var (
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorDim)

	StyleMuted = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleSelected = lipgloss.NewStyle().
			Background(ColorSel).
			Foreground(ColorText).
			Bold(true)

	StyleListPane = lipgloss.NewStyle().
			BorderStyle(lipgloss.Border{Right: "║"}).
			BorderRight(true).
			BorderForeground(ColorAccent).
			PaddingLeft(1)

	StyleDetailPane = lipgloss.NewStyle().
			PaddingLeft(1)

	StyleOverlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			MaxWidth(48)

	StyleLabel = lipgloss.NewStyle().
			Foreground(ColorDim).
			Width(12)

	StyleValue = lipgloss.NewStyle().
			Foreground(ColorText)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorBlocked).
			Bold(true)

	StyleHeader = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			PaddingBottom(1)
)
