package ui

import tea "github.com/charmbracelet/bubbletea"

// updateConfirm handles key events for the delete-confirmation dialog.
func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	nucleusID := m.confirmID
	neuronID := m.confirmNeuronID

	cancelState := stateList
	if neuronID != "" {
		cancelState = stateNucleusDetail
	}

	switch {
	case matchKey(msg, m.keys.Confirm):
		if neuronID != "" {
			svc := m.services.RemoveNeuron
			m.state = stateNucleusDetail
			m.confirmNeuronID = ""
			return m, func() tea.Msg {
				return actionDoneMsg{err: svc(nucleusID, neuronID)}
			}
		}
		svc := m.services.RemoveNucleus
		m.state = stateList
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID)}
		}
	default:
		m.confirmNeuronID = ""
		m.state = cancelState
		return m, nil
	}
}

// viewConfirm renders the delete-confirmation dialog content.
func (m Model) viewConfirm() string {
	if m.confirmNeuronID != "" {
		return StyleTitle.Render("Remove neuron?") + "\n\n" +
			StyleValue.Render("Neuron ") + StyleError.Render(m.confirmNeuronID) +
			StyleValue.Render(" in nucleus ") + StyleError.Render(m.confirmID) + StyleValue.Render(" will be removed.\n") +
			StyleValue.Render("Its tmux pane and worktree (if any) will be deleted.\n\n") +
			StyleMuted.Render("y") + "  confirm   " +
			StyleMuted.Render("any other key") + " cancel"
	}
	return StyleTitle.Render("Remove nucleus?") + "\n\n" +
		StyleValue.Render("Nucleus ") + StyleError.Render(m.confirmID) + StyleValue.Render(" will be removed.\n") +
		StyleValue.Render("All tmux panes and the git worktree will be deleted.\n\n") +
		StyleMuted.Render("y") + "  confirm   " +
		StyleMuted.Render("any other key") + " cancel"
}
