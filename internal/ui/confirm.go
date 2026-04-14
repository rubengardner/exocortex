package ui

import tea "github.com/charmbracelet/bubbletea"

// updateConfirm handles key events for the delete-confirmation dialog.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Confirm):
		id := m.confirmID
		svc := m.services.RemoveNucleus
		m.state = stateList
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}
	default:
		// esc, n, or any other key cancels
		m.state = stateList
		return m, nil
	}
}

// viewConfirm renders the delete-confirmation dialog content.
func (m Model) viewConfirm() string {
	id := m.confirmID
	return StyleTitle.Render("Remove nucleus?") + "\n\n" +
		StyleValue.Render("Nucleus ") + StyleError.Render(id) + StyleValue.Render(" will be removed.\n") +
		StyleValue.Render("All tmux panes and the git worktree will be deleted.\n\n") +
		StyleMuted.Render("y") + "  confirm   " +
		StyleMuted.Render("any other key") + " cancel"
}
