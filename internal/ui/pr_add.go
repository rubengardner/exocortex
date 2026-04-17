package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// prAddForm is a self-contained two-field form for adding a PR to a nucleus.
type prAddForm struct {
	repoInput   textinput.Model
	numberInput textinput.Model
	focusIdx    int // 0 = repo, 1 = number
	err         string
}

func newPRAddForm() prAddForm {
	repo := textinput.New()
	repo.Placeholder = "owner/repo-name"
	repo.CharLimit = 80

	num := textinput.New()
	num.Placeholder = "42"
	num.CharLimit = 10

	return prAddForm{repoInput: repo, numberInput: num}
}

// updatePRAdd handles key events while in the PR add overlay state.
func (m Model) updatePRAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.state = stateNucleusDetail
		return m, nil

	case matchKey(msg, m.keys.NextField):
		m.prAdd.focusIdx = (m.prAdd.focusIdx + 1) % 2
		if m.prAdd.focusIdx == 0 {
			m.prAdd.numberInput.Blur()
			cmd := m.prAdd.repoInput.Focus()
			return m, cmd
		}
		m.prAdd.repoInput.Blur()
		cmd := m.prAdd.numberInput.Focus()
		return m, cmd

	case matchKey(msg, m.keys.Submit):
		repo := strings.TrimSpace(m.prAdd.repoInput.Value())
		numStr := strings.TrimSpace(m.prAdd.numberInput.Value())
		if repo == "" {
			m.prAdd.err = "repo is required"
			return m, nil
		}
		n, err := strconv.Atoi(numStr)
		if err != nil || n <= 0 {
			m.prAdd.err = "PR number must be a positive integer"
			return m, nil
		}
		nucleusID := m.prAddNucleusID
		pr := registry.PullRequest{Number: n, Repo: repo}
		svc := m.services.AddPullRequest
		m.state = stateNucleusDetail
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, pr)}
		}
	}

	var cmd tea.Cmd
	if m.prAdd.focusIdx == 0 {
		m.prAdd.repoInput, cmd = m.prAdd.repoInput.Update(msg)
	} else {
		m.prAdd.numberInput, cmd = m.prAdd.numberInput.Update(msg)
	}
	return m, cmd
}

// viewPRAdd renders the PR add overlay content.
func (m Model) viewPRAdd() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Add Pull Request") + "\n\n")

	repoLabel := StyleLabel.Render("Repo")
	if m.prAdd.focusIdx == 0 {
		repoLabel = StyleTitle.Render("Repo")
	}
	sb.WriteString(repoLabel + "\n")
	sb.WriteString(m.prAdd.repoInput.View() + "\n\n")

	numLabel := StyleLabel.Render("Number")
	if m.prAdd.focusIdx == 1 {
		numLabel = StyleTitle.Render("Number")
	}
	sb.WriteString(numLabel + "\n")
	sb.WriteString(m.prAdd.numberInput.View())

	if m.prAdd.err != "" {
		sb.WriteString("\n\n")
		sb.WriteString(StyleError.Render(m.prAdd.err))
	}

	sb.WriteString("\n\n")
	sb.WriteString(
		StyleDim.Render("tab") + " next   " +
			StyleDim.Render("enter") + " add   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}
