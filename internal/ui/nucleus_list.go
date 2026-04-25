package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// updateNucleusList handles key events when the main nucleus list is active.
func (m Model) updateNucleusList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Quit):
		return m, tea.Quit

	case matchKey(msg, m.keys.Help):
		m.state = stateHelp
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case matchKey(msg, m.keys.Down):
		if m.cursor < len(m.nuclei)-1 {
			m.cursor++
		}
		return m, nil

	case matchKey(msg, m.keys.Refresh):
		return m, m.loadNucleiCmd()

	case matchKey(msg, m.keys.New):
		return m.openNucleusModal(NucleusModalContext{})

	case matchKey(msg, m.keys.Delete):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		m.confirmID = m.nuclei[m.cursor].ID
		m.state = stateConfirmDelete
		return m, nil

	case matchKey(msg, m.keys.Goto):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.GotoNucleus
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.Nvim):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.OpenNvim
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.CloseNvim):
		if len(m.nuclei) == 0 || m.services.CloseNvim == nil {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.CloseNvim
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.Respawn):
		if len(m.nuclei) == 0 || m.services.RespawnNucleus == nil {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.RespawnNucleus
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.Board):
		m.state = stateJiraBoard
		if m.jiraIssues == nil && !m.jiraLoading {
			m.jiraLoading = true
			return m, m.loadJiraBoardCmd()
		}
		return m, nil

	case matchKey(msg, m.keys.GitHub):
		m.state = stateGitHubView
		if m.githubPRs == nil && !m.githubLoading && m.services.LoadGitHubPRs != nil {
			m.githubLoading = true
			return m, m.loadGitHubPRsCmd()
		}
		return m, nil

	case matchKey(msg, m.keys.Submit), msg.Type == tea.KeyRight:
		if len(m.nuclei) == 0 {
			return m, nil
		}
		n := m.nuclei[m.cursor]
		m.detailNeuronIdx = 0
		m.paneContent = ""
		m.branchModified = nil
		m.branchAheadCommits = nil
		m.detailJiraIssue = nil
		m.detailJiraLoading = n.JiraKey != "" && m.services.LoadJiraIssueMeta != nil
		m.detailPRDetail = nil
		m.detailPRLoading = len(n.PullRequests) > 0 && m.services.LoadGitHubPR != nil
		m.state = stateNucleusDetail
		return m, tea.Batch(m.loadBranchInfoCmd(), m.captureDetailPaneCmd(), m.loadJiraIssueMetaCmd(), m.loadGitHubPRMetaCmd())
	}
	return m, nil
}

// viewHeader renders the top bar with the app name and nucleus count.
func (m Model) viewHeader() string {
	count := fmt.Sprintf("%d nucleus(i)", len(m.nuclei))
	left := StyleHeader.Render("◈  EXOCORTEX")
	right := StyleMuted.Render(count)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// viewNucleusList renders the left-panel list of nuclei.
func (m Model) viewNucleusList(width int) string {
	if len(m.nuclei) == 0 {
		return StyleDim.Render("  no nuclei yet\n  press n to create one")
	}
	var sb strings.Builder
	for i, n := range m.nuclei {
		dots := claudeNeuronDots(n)
		dotsPlain := claudeNeuronDotsPlain(n)
		badges := nucleusBadges(n)
		// Reserve space for badges (each badge is max 10 chars) + age (4) + separators.
		badgesPlain := nucleusBadgesPlain(n) // plain text width for layout math
		taskWidth := width - 10 - len(badgesPlain) - (len(dotsPlain) - 1)
		if taskWidth < 8 {
			taskWidth = 8
		}
		task := truncate(n.TaskDescription, taskWidth)
		age := fmtAge(n.CreatedAt)

		line1 := fmt.Sprintf(" %s %-*s", dots, width-10, task)
		meta := fmt.Sprintf("   %s  %s", n.ID, age)
		line2 := StyleDim.Render(meta) + badges

		row := line1 + "\n" + line2
		if i == m.cursor {
			row = StyleSelected.Width(width).Render(line1) + "\n" +
				StyleSelected.Width(width).Foreground(ColorDim).Render("  "+n.ID+"  "+age) + badges
		}
		sb.WriteString(row)
		if i < len(m.nuclei)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// nucleusBadges returns styled inline badges for Jira key and PR numbers.
func nucleusBadges(n registry.Nucleus) string {
	var s string
	if n.JiraKey != "" {
		s += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render("["+n.JiraKey+"]")
	}
	switch len(n.PullRequests) {
	case 0:
	case 1:
		s += " " + lipgloss.NewStyle().Foreground(ColorAccent).Render(fmt.Sprintf("[#%d]", n.PullRequests[0].Number))
	default:
		s += " " + lipgloss.NewStyle().Foreground(ColorAccent).Render(fmt.Sprintf("[%d PRs]", len(n.PullRequests)))
	}
	return s
}

// nucleusBadgesPlain returns the plain-text width of badges (for layout math).
func nucleusBadgesPlain(n registry.Nucleus) string {
	var s string
	if n.JiraKey != "" {
		s += " [" + n.JiraKey + "]"
	}
	switch len(n.PullRequests) {
	case 0:
	case 1:
		s += fmt.Sprintf(" [#%d]", n.PullRequests[0].Number)
	default:
		s += fmt.Sprintf(" [%d PRs]", len(n.PullRequests))
	}
	return s
}

// claudeNeuronDots returns one styled status dot per Claude neuron, or a dim dash if none.
func claudeNeuronDots(n registry.Nucleus) string {
	var sb strings.Builder
	for _, neu := range n.Neurons {
		if neu.Type == registry.NeuronClaude {
			sb.WriteString(StatusDot(neu.Status))
		}
	}
	if sb.Len() == 0 {
		return StyleDim.Render("─")
	}
	return sb.String()
}

// claudeNeuronDotsPlain returns plain-text dots (for width math).
func claudeNeuronDotsPlain(n registry.Nucleus) string {
	count := 0
	for _, neu := range n.Neurons {
		if neu.Type == registry.NeuronClaude {
			count++
		}
	}
	if count == 0 {
		return "─"
	}
	return strings.Repeat("●", count)
}

// viewStatusBar renders the bottom status/help bar for the main view.
func (m Model) viewStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	return StyleHelp.Render(m.help.View(m.keys))
}
