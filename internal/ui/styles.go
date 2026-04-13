package ui

import "github.com/charmbracelet/lipgloss"

// Palette
const (
	ColorAccent  = lipgloss.Color("#7C3AED") // purple
	ColorWorking = lipgloss.Color("#22C55E") // green
	ColorWaiting = lipgloss.Color("#F97316") // orange — distinct from working amber
	ColorIdle    = lipgloss.Color("#F59E0B") // amber
	ColorBlocked = lipgloss.Color("#EF4444") // red
	ColorMuted   = lipgloss.Color("#64748B") // slate-500
	ColorBorder  = lipgloss.Color("#334155") // slate-700
	ColorSel     = lipgloss.Color("#1E293B") // slate-800
	ColorText    = lipgloss.Color("#E2E8F0") // slate-200
	ColorDim     = lipgloss.Color("#94A3B8") // slate-400
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

// StatusDot returns a coloured bullet for the given status.
func StatusDot(status string) string {
	return lipgloss.NewStyle().Foreground(StatusColor(status)).Render("●")
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
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(ColorBorder)

	StyleDetailPane = lipgloss.NewStyle().
			PaddingLeft(2)

	StyleOverlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAccent).
			Padding(1, 3)

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
