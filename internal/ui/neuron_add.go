package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// neuronTypes is the ordered list of types shown in the neuron add picker.
var neuronTypes = []string{"claude", "nvim", "shell"}

// updateNeuronAdd handles key events for the neuron-type picker overlay.
func (m Model) updateNeuronAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.state = stateNucleusDetail
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.neuronAddCursor > 0 {
			m.neuronAddCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.neuronAddCursor < len(neuronTypes)-1 {
			m.neuronAddCursor++
		}

	case matchKey(msg, m.keys.Submit):
		if m.services.AddNeuron == nil {
			m.state = stateNucleusDetail
			return m, nil
		}
		neuronType := neuronTypes[m.neuronAddCursor]
		nucleusID := m.neuronAddNucleusID
		svc := m.services.AddNeuron
		// Return to detail immediately; actionDoneMsg will reload nuclei.
		m.state = stateNucleusDetail
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, neuronType, "")}
		}
	}
	return m, nil
}

// viewNeuronAdd renders the neuron-type picker overlay content.
func (m Model) viewNeuronAdd() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Add Neuron") + "\n\n")

	for i, t := range neuronTypes {
		if i == m.neuronAddCursor {
			sb.WriteString(StyleSelected.Render("  ▶ "+t) + "\n")
		} else {
			sb.WriteString("    " + t + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " add   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}
