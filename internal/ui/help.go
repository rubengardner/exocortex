package ui

import "github.com/charmbracelet/lipgloss"

// viewHelp renders the full-screen keyboard shortcuts page.
func (m Model) viewHelp() string {
	m.help.ShowAll = true
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		StyleOverlay.Render(
			StyleTitle.Render("Keyboard Shortcuts")+"\n\n"+
				m.help.View(m.keys),
		),
	)
}
