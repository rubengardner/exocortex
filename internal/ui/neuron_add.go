package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// neuronTypes is the ordered list of types shown in the neuron add picker.
var neuronTypes = []string{"claude", "nvim", "shell"}

// updateNeuronAdd routes key events to the correct phase handler.
func (m Model) updateNeuronAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.neuronAddPhase {
	case 1:
		return m.updateNeuronAddPhase1(msg)
	case 2:
		return m.updateNeuronAddPhase2(msg)
	case 3:
		return m.updateNeuronAddPhase3(msg)
	default:
		return m.updateNeuronAddPhase0(msg)
	}
}

// Phase 0: type picker.
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
		// All types advance to phase 1 (repo picker).
		m.neuronAddPhase = 1
		m.neuronAddRepoCursor = 0
		var cmds []tea.Cmd
		if m.services.LoadRepos != nil {
			svc := m.services.LoadRepos
			cmds = append(cmds, func() tea.Msg {
				repos, err := svc()
				return neuronAddReposLoadedMsg{repos: repos, err: err}
			})
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// Phase 1: repo picker (all types).
func (m Model) updateNeuronAddPhase1(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.neuronAddPhase = 0
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.neuronAddRepoCursor > 0 {
			m.neuronAddRepoCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.neuronAddRepoCursor < len(m.neuronAddRepos)-1 {
			m.neuronAddRepoCursor++
		}

	case matchKey(msg, m.keys.Submit):
		neuronType := neuronTypes[m.neuronAddCursor]
		repoPath := "."
		if m.neuronAddRepoCursor < len(m.neuronAddRepos) {
			repoPath = m.neuronAddRepos[m.neuronAddRepoCursor]
		}

		if neuronType == "shell" {
			// Shell skips branch selection.
			nucleusID := m.neuronAddNucleusID
			svc := m.services.AddNeuron
			m.state = stateNucleusDetail
			m.neuronAddPhase = 0
			return m, func() tea.Msg {
				return actionDoneMsg{err: svc(nucleusID, neuronType, repoPath, "", "", false)}
			}
		}

		// Claude / nvim → advance to phase 2 (branch mode).
		m.neuronAddPhase = 2
		m.neuronAddBranchMode = 0
		m.neuronAddBaseBranches = m.services.BaseBranchesForRepo(repoPath)
		m.neuronAddBaseCursor = 0
		m.neuronAddBaseChosen = false
		m.neuronAddSelectedBase = ""
		m.neuronAddExisting = nil
		m.neuronAddFilter = ""
		m.neuronAddExistCursor = 0
	}
	return m, nil
}

// Phase 2: branch mode picker (new / existing).
func (m Model) updateNeuronAddPhase2(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.neuronAddPhase = 1
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.neuronAddBranchMode > 0 {
			m.neuronAddBranchMode--
		}

	case matchKey(msg, m.keys.Down):
		if m.neuronAddBranchMode < 1 {
			m.neuronAddBranchMode++
		}

	case matchKey(msg, m.keys.Submit):
		m.neuronAddPhase = 3
		m.neuronAddBaseCursor = 0
		m.neuronAddBaseChosen = false
		m.neuronAddSelectedBase = ""

		if m.neuronAddBranchMode == 0 {
			// New branch: initialise branch name input.
			bi := textinput.New()
			bi.Placeholder = "feature/branch-name"
			bi.CharLimit = 80
			m.neuronAddBranch = bi
			// Auto-select base if only one option.
			if len(m.neuronAddBaseBranches) == 1 {
				m.neuronAddSelectedBase = m.neuronAddBaseBranches[0]
				m.neuronAddBaseChosen = true
				return m, m.neuronAddBranch.Focus()
			}
			if len(m.neuronAddBaseBranches) == 0 {
				m.neuronAddBaseChosen = true
				return m, m.neuronAddBranch.Focus()
			}
			return m, nil
		}

		// Existing branch: fire async load.
		repoPath := "."
		if m.neuronAddRepoCursor < len(m.neuronAddRepos) {
			repoPath = m.neuronAddRepos[m.neuronAddRepoCursor]
		}
		return m, m.loadNeuronAddBranchesCmd(repoPath)
	}
	return m, nil
}

// Phase 3: branch details (new or existing).
func (m Model) updateNeuronAddPhase3(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.neuronAddBranchMode == 1 {
		return m.updateNeuronAddPhase3Existing(msg)
	}
	return m.updateNeuronAddPhase3New(msg)
}

// Phase 3a: new branch (base picker then name input).
func (m Model) updateNeuronAddPhase3New(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		if !m.neuronAddBaseChosen && len(m.neuronAddBaseBranches) > 1 {
			m.neuronAddPhase = 2
			return m, nil
		}
		m.neuronAddPhase = 2
		m.neuronAddBaseChosen = false
		m.neuronAddBranch.Blur()
		return m, nil

	case matchKey(msg, m.keys.Up):
		if !m.neuronAddBaseChosen && m.neuronAddBaseCursor > 0 {
			m.neuronAddBaseCursor--
		}

	case matchKey(msg, m.keys.Down):
		if !m.neuronAddBaseChosen && m.neuronAddBaseCursor < len(m.neuronAddBaseBranches)-1 {
			m.neuronAddBaseCursor++
		}

	case matchKey(msg, m.keys.Submit):
		if !m.neuronAddBaseChosen && len(m.neuronAddBaseBranches) > 1 {
			m.neuronAddSelectedBase = m.neuronAddBaseBranches[m.neuronAddBaseCursor]
			m.neuronAddBaseChosen = true
			return m, m.neuronAddBranch.Focus()
		}

		// Submit the new branch.
		branch := strings.TrimSpace(m.neuronAddBranch.Value())
		if branch == "" {
			return m, nil
		}
		repoPath := "."
		if m.neuronAddRepoCursor < len(m.neuronAddRepos) {
			repoPath = m.neuronAddRepos[m.neuronAddRepoCursor]
		}
		neuronType := neuronTypes[m.neuronAddCursor]
		nucleusID := m.neuronAddNucleusID
		baseBranch := m.neuronAddSelectedBase
		svc := m.services.AddNeuron
		m.state = stateNucleusDetail
		m.neuronAddPhase = 0
		m.neuronAddBranch.Blur()
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, neuronType, repoPath, branch, baseBranch, true)}
		}
	}

	if m.neuronAddBaseChosen {
		var cmd tea.Cmd
		m.neuronAddBranch, cmd = m.neuronAddBranch.Update(msg)
		return m, cmd
	}
	return m, nil
}

// Phase 3b: existing branch (filter + list).
func (m Model) updateNeuronAddPhase3Existing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.neuronAddPhase = 2
		m.neuronAddFilter = ""
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.neuronAddExistCursor > 0 {
			m.neuronAddExistCursor--
		}

	case matchKey(msg, m.keys.Down):
		filtered := neuronAddFilteredBranches(m.neuronAddExisting, m.neuronAddFilter)
		if m.neuronAddExistCursor < len(filtered)-1 {
			m.neuronAddExistCursor++
		}

	case matchKey(msg, m.keys.Submit):
		filtered := neuronAddFilteredBranches(m.neuronAddExisting, m.neuronAddFilter)
		if len(filtered) == 0 {
			return m, nil
		}
		branch := filtered[m.neuronAddExistCursor]
		repoPath := "."
		if m.neuronAddRepoCursor < len(m.neuronAddRepos) {
			repoPath = m.neuronAddRepos[m.neuronAddRepoCursor]
		}
		neuronType := neuronTypes[m.neuronAddCursor]
		nucleusID := m.neuronAddNucleusID
		svc := m.services.AddNeuron
		m.state = stateNucleusDetail
		m.neuronAddPhase = 0
		m.neuronAddFilter = ""
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, neuronType, repoPath, branch, "", false)}
		}

	case msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete:
		if len(m.neuronAddFilter) > 0 {
			m.neuronAddFilter = m.neuronAddFilter[:len(m.neuronAddFilter)-1]
			m.neuronAddExistCursor = 0
		}

	default:
		if msg.Type == tea.KeyRunes {
			m.neuronAddFilter += string(msg.Runes)
			m.neuronAddExistCursor = 0
		}
	}
	return m, nil
}

// neuronAddFilteredBranches filters the branch list by the given prefix/substring.
func neuronAddFilteredBranches(branches []string, filter string) []string {
	if filter == "" {
		return branches
	}
	lo := strings.ToLower(filter)
	var out []string
	for _, b := range branches {
		if strings.Contains(strings.ToLower(b), lo) {
			out = append(out, b)
		}
	}
	return out
}

// neuronAddReposLoadedMsg carries the result of loading repos for neuron add phase 1.
type neuronAddReposLoadedMsg struct {
	repos []string
	err   error
}

// viewNeuronAdd renders the neuron-type picker overlay content.
func (m Model) viewNeuronAdd() string {
	switch m.neuronAddPhase {
	case 1:
		return m.viewNeuronAddPhase1()
	case 2:
		return m.viewNeuronAddPhase2()
	case 3:
		return m.viewNeuronAddPhase3()
	default:
		return m.viewNeuronAddPhase0()
	}
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
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}

func (m Model) viewNeuronAddPhase1() string {
	var sb strings.Builder
	neuronType := neuronTypes[m.neuronAddCursor]
	sb.WriteString(StyleTitle.Render("Add Neuron · "+neuronType) + "\n\n")
	sb.WriteString(StyleLabel.Render("Repo") + "\n")

	const maxShow = 5
	repos := m.neuronAddRepos
	if len(repos) == 0 {
		repos = []string{"."}
	}
	for i, r := range repos {
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
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " back",
	)
	return sb.String()
}

func (m Model) viewNeuronAddPhase2() string {
	var sb strings.Builder
	neuronType := neuronTypes[m.neuronAddCursor]
	sb.WriteString(StyleTitle.Render("Add Neuron · "+neuronType) + "\n\n")
	sb.WriteString(StyleLabel.Render("Branch") + "\n")

	modes := []string{"new branch", "existing branch"}
	for i, mode := range modes {
		if i == m.neuronAddBranchMode {
			sb.WriteString(StyleSelected.Render("  ▶ "+mode) + "\n")
		} else {
			sb.WriteString("    " + mode + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " back",
	)
	return sb.String()
}

func (m Model) viewNeuronAddPhase3() string {
	if m.neuronAddBranchMode == 1 {
		return m.viewNeuronAddPhase3Existing()
	}
	return m.viewNeuronAddPhase3New()
}

func (m Model) viewNeuronAddPhase3New() string {
	var sb strings.Builder
	neuronType := neuronTypes[m.neuronAddCursor]
	sb.WriteString(StyleTitle.Render("Add Neuron · "+neuronType) + "\n\n")

	if !m.neuronAddBaseChosen && len(m.neuronAddBaseBranches) > 1 {
		sb.WriteString(StyleLabel.Render("Base branch") + "\n")
		const maxShow = 5
		for i, b := range m.neuronAddBaseBranches {
			if i >= maxShow {
				sb.WriteString(StyleDim.Render("  … more") + "\n")
				break
			}
			if i == m.neuronAddBaseCursor {
				sb.WriteString(StyleSelected.Render("  ▶ "+b) + "\n")
			} else {
				sb.WriteString("    " + b + "\n")
			}
		}
		sb.WriteString("\n")
		sb.WriteString(
			StyleDim.Render("↑/k") + " up   " +
				StyleDim.Render("↓/j") + " down   " +
				StyleDim.Render("enter") + " select   " +
				StyleDim.Render("esc") + " back",
		)
		return sb.String()
	}

	if m.neuronAddSelectedBase != "" {
		sb.WriteString(StyleDim.Render("from: "+m.neuronAddSelectedBase) + "\n\n")
	}
	sb.WriteString(StyleLabel.Render("Branch name") + "\n")
	sb.WriteString(m.neuronAddBranch.View())
	sb.WriteString("\n\n")
	sb.WriteString(
		StyleDim.Render("enter") + " create   " +
			StyleDim.Render("esc") + " back",
	)
	return sb.String()
}

func (m Model) viewNeuronAddPhase3Existing() string {
	var sb strings.Builder
	neuronType := neuronTypes[m.neuronAddCursor]
	sb.WriteString(StyleTitle.Render("Add Neuron · "+neuronType) + "\n\n")
	sb.WriteString(StyleLabel.Render("Filter") + "  " + m.neuronAddFilter + "█\n\n")

	filtered := neuronAddFilteredBranches(m.neuronAddExisting, m.neuronAddFilter)
	if len(filtered) == 0 {
		sb.WriteString(StyleDim.Render("  no branches") + "\n")
	} else {
		const maxShow = 6
		for i, b := range filtered {
			if i >= maxShow {
				sb.WriteString(StyleDim.Render(fmt.Sprintf("  … %d more", len(filtered)-maxShow)) + "\n")
				break
			}
			if i == m.neuronAddExistCursor {
				sb.WriteString(StyleSelected.Render("  > "+truncate(b, 35)) + "\n")
			} else {
				sb.WriteString("    " + truncate(b, 35) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " back",
	)
	return sb.String()
}
