package ui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// neuronTypes is the ordered list of types shown in the neuron add picker.
var neuronTypes = []string{"claude", "nvim", "shell"}

// updateNeuronAdd handles key events for the neuron-type picker overlay (phase 0)
// and the repo+branch form (phase 1, claude only).
func (m Model) updateNeuronAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.neuronAddPhase == 1 {
		return m.updateNeuronAddPhase1(msg)
	}
	return m.updateNeuronAddPhase0(msg)
}

func (m Model) updateNeuronAddPhase0(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

		if neuronType == "claude" {
			// Advance to phase 1: repo + branch picker.
			m.neuronAddPhase = 1
			m.neuronAddRepoCursor = 0
			bi := textinput.New()
			bi.Placeholder = "feature/branch-name"
			bi.CharLimit = 80
			m.neuronAddBranch = bi
			var cmds []tea.Cmd
			cmds = append(cmds, m.neuronAddBranch.Focus())
			if m.services.LoadRepos != nil {
				svc := m.services.LoadRepos
				cmds = append(cmds, func() tea.Msg {
					repos, err := svc()
					return neuronAddReposLoadedMsg{repos: repos, err: err}
				})
			}
			return m, tea.Batch(cmds...)
		}

		// nvim / shell — no repo/branch needed.
		nucleusID := m.neuronAddNucleusID
		svc := m.services.AddNeuron
		m.state = stateNucleusDetail
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, neuronType, "", "")}
		}
	}
	return m, nil
}

func (m Model) updateNeuronAddPhase1(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.neuronAddPhase = 0
		m.neuronAddBranch.Blur()
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.neuronAddRepoCursor > 0 {
			m.neuronAddRepoCursor--
		}
		return m, nil

	case matchKey(msg, m.keys.Down):
		if m.neuronAddRepoCursor < len(m.neuronAddRepos)-1 {
			m.neuronAddRepoCursor++
		}
		return m, nil

	case matchKey(msg, m.keys.Submit):
		if m.services.AddNeuron == nil {
			m.state = stateNucleusDetail
			return m, nil
		}
		repoPath := "."
		if m.neuronAddRepoCursor < len(m.neuronAddRepos) {
			repoPath = m.neuronAddRepos[m.neuronAddRepoCursor]
		}
		branch := strings.TrimSpace(m.neuronAddBranch.Value())
		nucleusID := m.neuronAddNucleusID
		svc := m.services.AddNeuron
		m.state = stateNucleusDetail
		m.neuronAddPhase = 0
		m.neuronAddBranch.Blur()
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, "claude", repoPath, branch)}
		}
	}

	// Forward remaining keys to the branch text input.
	var cmd tea.Cmd
	m.neuronAddBranch, cmd = m.neuronAddBranch.Update(msg)
	return m, cmd
}

// neuronAddReposLoadedMsg carries the result of loading repos for neuron add phase 1.
type neuronAddReposLoadedMsg struct {
	repos []string
	err   error
}

// viewNeuronAdd renders the neuron-type picker overlay content (phase 0 or 1).
func (m Model) viewNeuronAdd() string {
	if m.neuronAddPhase == 1 {
		return m.viewNeuronAddPhase1()
	}
	return m.viewNeuronAddPhase0()
}

func (m Model) viewNeuronAddPhase0() string {
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

func (m Model) viewNeuronAddPhase1() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Add Neuron · Claude") + "\n\n")

	if len(m.neuronAddRepos) > 0 {
		sb.WriteString(StyleLabel.Render("Repo") + "\n")
		const maxShow = 4
		for i, r := range m.neuronAddRepos {
			if i >= maxShow {
				sb.WriteString(StyleDim.Render("  … more") + "\n")
				break
			}
			base := filepath.Base(r)
			if i == m.neuronAddRepoCursor {
				sb.WriteString(StyleSelected.Render("  > "+truncate(base, 30)) + "\n")
			} else {
				sb.WriteString("    " + truncate(base, 30) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(StyleLabel.Render("Branch") + "\n")
	sb.WriteString(m.neuronAddBranch.View())

	sb.WriteString("\n\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " repo   " +
			StyleDim.Render("enter") + " add   " +
			StyleDim.Render("esc") + " back",
	)
	return sb.String()
}
