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
	if m.jiraKeyPickActive {
		return m.updateJiraKeyPicker(msg)
	}
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

	case matchKey(msg, m.keys.OpenBrowser):
		if len(m.nuclei) == 0 || m.services.OpenJiraKey == nil {
			return m, nil
		}
		keys := m.nuclei[m.cursor].JiraKeys
		if len(keys) == 0 {
			return m, nil
		}
		if len(keys) == 1 {
			svc := m.services.OpenJiraKey
			key := keys[0]
			return m, func() tea.Msg { _ = svc(key); return nil }
		}
		m.jiraKeyPickActive = true
		m.jiraKeyPickCursor = 0
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
		m.detailJiraLoading = len(n.JiraKeys) > 0 && m.services.LoadJiraIssueMeta != nil
		m.detailPRDetail = nil
		m.detailPRLoading = len(n.PullRequests) > 0 && m.services.LoadGitHubPR != nil
		m.state = stateNucleusDetail
		return m, tea.Batch(m.loadBranchInfoCmd(), m.captureDetailPaneCmd(), m.loadJiraIssueMetaCmd(), m.loadGitHubPRMetaCmd())
	}
	return m, nil
}

// viewHeader renders the top bar with the app name and nucleus count.
func (m Model) viewHeader() string {
	count := fmt.Sprintf("%d nuclei", len(m.nuclei))
	dot := lipgloss.NewStyle().Foreground(ColorAccent).Render("◈")
	left := dot + StyleDim.Render("  exocortex")
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
		return StyleDim.Render("  No nuclei yet.\n  Press n to get started.")
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

		// Construct lines with left-bar indicator for selection
		indicator := " "
		if i == m.cursor {
			indicator = lipgloss.NewStyle().Foreground(ColorAccent).Render("▌")
		}
		taskStr := lipgloss.NewStyle().Foreground(ColorText).Render(task)
		line1 := fmt.Sprintf("%s %s %s", indicator, dots, taskStr)

		// Colored metadata: accent ID, muted age
		idStr := lipgloss.NewStyle().Foreground(ColorAccent).Render(n.ID)
		ageStr := lipgloss.NewStyle().Foreground(ColorDim).Render(age)
		meta := fmt.Sprintf("  %s · %s", idStr, ageStr)
		line2 := meta + badges

		row := line1 + "\n" + line2
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
	if len(n.JiraKeys) > 0 {
		s += " · " + lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render("["+n.JiraKeys[0]+"]")
		if len(n.JiraKeys) > 1 {
			s += lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(fmt.Sprintf("+%d", len(n.JiraKeys)-1))
		}
	}
	switch len(n.PullRequests) {
	case 0:
	case 1:
		s += " · " + lipgloss.NewStyle().Foreground(ColorAccent).Render(fmt.Sprintf("#%d", n.PullRequests[0].Number))
	default:
		s += " · " + lipgloss.NewStyle().Foreground(ColorAccent).Render(fmt.Sprintf("%d PRs", len(n.PullRequests)))
	}
	return s
}

// nucleusBadgesPlain returns the plain-text width of badges (for layout math).
func nucleusBadgesPlain(n registry.Nucleus) string {
	var s string
	if len(n.JiraKeys) > 0 {
		s += " · [" + n.JiraKeys[0] + "]"
		if len(n.JiraKeys) > 1 {
			s += fmt.Sprintf("+%d", len(n.JiraKeys)-1)
		}
	}
	switch len(n.PullRequests) {
	case 0:
	case 1:
		s += fmt.Sprintf(" · #%d", n.PullRequests[0].Number)
	default:
		s += fmt.Sprintf(" · %d PRs", len(n.PullRequests))
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
	return strings.Repeat("◆", count)
}

// viewStatusBar renders the bottom status/help bar for the main view.
func (m Model) viewStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	// Context-sensitive key hints for the nucleus list
	hints := "  q quit · n new · j/k move · g go · e edit · d delete · r refresh"
	if len(m.nuclei) > 0 {
		hints = "  q quit · n new · j/k move · g go · e edit · d delete · b jira · r refresh"
	}
	count := fmt.Sprintf("%d nuclei", len(m.nuclei))
	right := StyleMuted.Render(count)
	gap := m.width - lipgloss.Width(hints) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return StyleHelp.Render(hints) + strings.Repeat(" ", gap) + right
}

// updateJiraKeyPicker handles key events when the Jira key picker overlay is active.
func (m Model) updateJiraKeyPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.nuclei) == 0 {
		m.jiraKeyPickActive = false
		return m, nil
	}
	keys := m.nuclei[m.cursor].JiraKeys
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.jiraKeyPickActive = false
		return m, nil
	case matchKey(msg, m.keys.Up):
		if m.jiraKeyPickCursor > 0 {
			m.jiraKeyPickCursor--
		}
	case matchKey(msg, m.keys.Down):
		if m.jiraKeyPickCursor < len(keys)-1 {
			m.jiraKeyPickCursor++
		}
	case matchKey(msg, m.keys.Submit):
		if m.services.OpenJiraKey == nil || m.jiraKeyPickCursor >= len(keys) {
			m.jiraKeyPickActive = false
			return m, nil
		}
		svc := m.services.OpenJiraKey
		key := keys[m.jiraKeyPickCursor]
		m.jiraKeyPickActive = false
		return m, func() tea.Msg { _ = svc(key); return nil }
	}
	return m, nil
}

// viewJiraKeyPicker renders the Jira key selection overlay.
func (m Model) viewJiraKeyPicker() string {
	if len(m.nuclei) == 0 {
		return ""
	}
	keys := m.nuclei[m.cursor].JiraKeys
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Open Jira ticket") + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", 26)) + "\n")
	for i, key := range keys {
		if i == m.jiraKeyPickCursor {
			sb.WriteString(StyleSelected.Render("▶ "+key) + "\n")
		} else {
			sb.WriteString("  " + key + "\n")
		}
	}
	sb.WriteString(StyleDim.Render(strings.Repeat("─", 26)) + "\n")
	sb.WriteString(StyleHelp.Render("  enter open   esc cancel"))
	return sb.String()
}
