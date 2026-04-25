package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// ── StateNucleusDetail (full-screen 3-panel dashboard) ────────────────────────

// updateNucleusDetail handles key events for the full-screen nucleus dashboard.
func (m Model) updateNucleusDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateList
		m.paneContent = ""
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.detailNeuronIdx > 0 {
			m.detailNeuronIdx--
			m.paneContent = ""
			m.branchModified = nil
			m.branchAheadCommits = nil
		}
		return m, tea.Batch(m.captureDetailPaneCmd(), m.loadBranchInfoCmd())

	case matchKey(msg, m.keys.Down):
		if len(m.nuclei) > 0 {
			n := m.nuclei[m.cursor]
			if m.detailNeuronIdx < len(n.Neurons)-1 {
				m.detailNeuronIdx++
				m.paneContent = ""
				m.branchModified = nil
				m.branchAheadCommits = nil
			}
		}
		return m, tea.Batch(m.captureDetailPaneCmd(), m.loadBranchInfoCmd())

	case matchKey(msg, m.keys.Goto):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		n := m.nuclei[m.cursor]
		if len(n.Neurons) == 0 {
			return m, nil
		}
		if m.services.GotoNeuron != nil {
			idx := m.detailNeuronIdx
			if idx >= len(n.Neurons) {
				idx = 0
			}
			neuronID := n.Neurons[idx].ID
			nucleusID := n.ID
			svc := m.services.GotoNeuron
			return m, func() tea.Msg {
				return actionDoneMsg{err: svc(nucleusID, neuronID)}
			}
		}
		// Fall back to nucleus-level goto.
		id := n.ID
		svc := m.services.GotoNucleus
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.AddNeuron):
		if len(m.nuclei) == 0 || m.services.AddNeuron == nil {
			return m, nil
		}
		m.neuronAddNucleusID = m.nuclei[m.cursor].ID
		m.neuronAddCursor = 0
		m.neuronAddPhase = 0
		m.neuronAddRepos = nil
		m.state = stateNeuronAdd
		return m, nil

	case matchKey(msg, m.keys.AddPR):
		if len(m.nuclei) == 0 || m.services.AddPullRequest == nil {
			return m, nil
		}
		m.prAddNucleusID = m.nuclei[m.cursor].ID
		m.prAdd = newPRAddForm()
		m.state = statePRAdd
		cmd := m.prAdd.repoInput.Focus()
		return m, cmd

	case matchKey(msg, m.keys.CloseNvim):
		if len(m.nuclei) == 0 || m.services.CloseNvim == nil {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.CloseNvim
		m.state = stateNucleusDetail
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.New):
		return m.openNucleusModal(NucleusModalContext{})

	case matchKey(msg, m.keys.Refresh):
		m.branchModified = nil
		m.branchAheadCommits = nil
		return m, m.loadBranchInfoCmd()

	case matchKey(msg, m.keys.Delete):
		if len(m.nuclei) == 0 || m.services.RemoveNeuron == nil {
			return m, nil
		}
		n := m.nuclei[m.cursor]
		if len(n.Neurons) == 0 {
			return m, nil
		}
		idx := m.detailNeuronIdx
		if idx >= len(n.Neurons) {
			idx = 0
		}
		m.confirmID = n.ID
		m.confirmNeuronID = n.Neurons[idx].ID
		m.state = stateConfirmDelete
		return m, nil

	case matchKey(msg, m.keys.TogglePreview):
		m.previewEnabled = !m.previewEnabled
		if m.previewEnabled {
			m.paneContent = ""
			return m, m.captureDetailPaneCmd()
		}
		return m, nil
	}
	return m, nil
}

// viewNucleusDetailDashboard renders the full-screen 3-panel nucleus view.
func (m Model) viewNucleusDetailDashboard() string {
	if len(m.nuclei) == 0 {
		return "no nuclei"
	}
	n := m.nuclei[m.cursor]

	header := m.viewDetailHeader(n)
	h := m.contentHeight()

	leftW := clamp(m.width/4, 22, 32)
	midW := clamp(m.width/3, 26, 42)
	rightW := m.width - leftW - midW - 2 // 2 separators

	left := clipLines(m.viewNeuronCluster(n, leftW), h)
	mid := clipLines(m.viewContextPanel(n, midW), h)
	right := clipLines(m.viewDetailPreview(rightW), h)

	leftPane := StyleListPane.Height(h).Width(leftW).Render(left)
	midPane := StyleListPane.Height(h).Width(midW).Render(mid)
	rightPane := StyleDetailPane.Height(h).Width(rightW).Render(right)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, midPane, rightPane)
	statusBar := m.viewDetailStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

// viewDetailHeader renders the one-line header for the detail dashboard.
func (m Model) viewDetailHeader(n registry.Nucleus) string {
	left := StyleHeader.Render("◈  NUCLEUS " + n.ID + "  •  " + truncate(n.PrimaryBranch(), 40) + "  •  " + n.Status)
	right := StyleMuted.Render(fmtAge(n.CreatedAt))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// viewNeuronCluster renders the left panel: the neuron list.
func (m Model) viewNeuronCluster(n registry.Nucleus, width int) string {
	title := fmt.Sprintf("NEURONS (%d)", len(n.Neurons))
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render(truncate(title, width-2)) + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-2, 4, 60))) + "\n")

	if len(n.Neurons) == 0 {
		sb.WriteString(StyleDim.Render("  no neurons") + "\n")
		return sb.String()
	}

	for i, neu := range n.Neurons {
		dot := StatusDot(neu.Status)
		typeStr := string(neu.Type)
		idStr := neu.ID
		row := fmt.Sprintf(" %s %-6s %-8s %s", dot, idStr, typeStr, truncate(neu.TmuxTarget, width-22))

		if i == m.detailNeuronIdx {
			sb.WriteString(StyleSelected.Width(width).Render("▶"+row[1:]) + "\n")
		} else {
			sb.WriteString(row + "\n")
		}

		// Dim second line: branch + repo basename (omitted when both empty).
		if neu.Branch != "" || neu.RepoPath != "" {
			repoBase := filepath.Base(neu.RepoPath)
			line2 := "  " + truncate(neu.Branch, width/2-4) + "  " + StyleDim.Render(repoBase)
			sb.WriteString(StyleDim.Render(line2) + "\n")
		}
	}
	return sb.String()
}

// viewContextPanel renders the middle panel: Jira metadata (when linked) followed
// by branch info.
func (m Model) viewContextPanel(n registry.Nucleus, width int) string {
	var sb strings.Builder

	// ── Jira section ──────────────────────────────────────────────────────────
	if n.JiraKey != "" {
		title := "JIRA " + n.JiraKey
		sb.WriteString(StyleTitle.Render(truncate(title, width-2)) + "\n")
		sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-2, 4, 60))) + "\n")

		switch {
		case m.detailJiraLoading:
			sb.WriteString(StyleDim.Render("  loading…") + "\n")
		case m.detailJiraIssue != nil:
			issue := m.detailJiraIssue
			sb.WriteString(StyleValue.Render("  "+truncate(issue.Summary, width-4)) + "\n")
			sb.WriteString(StyleLabel.Render("Status") + StyleValue.Render(issue.Status) + "\n")
			if issue.Assignee != "" {
				first := strings.Fields(issue.Assignee)
				if len(first) > 0 {
					sb.WriteString(StyleDim.Render("  @"+first[0]) + "\n")
				}
			}
			sb.WriteString(StyleDim.Render("  "+truncate(issue.URL, width-4)) + "\n")
		default:
			// Linked but metadata not yet loaded or unavailable.
			sb.WriteString(StyleDim.Render("  "+n.JiraKey) + "\n")
		}
		sb.WriteString("\n")
	}

	// ── GitHub PR section ─────────────────────────────────────────────────────
	for i, pr := range n.PullRequests {
		title := fmt.Sprintf("PR #%d", pr.Number)
		sb.WriteString(StyleTitle.Render(truncate(title, width-2)) + "\n")
		sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-2, 4, 60))) + "\n")
		if i == 0 {
			switch {
			case m.detailPRLoading:
				sb.WriteString(StyleDim.Render("  loading…") + "\n")
			case m.detailPRDetail != nil:
				d := m.detailPRDetail
				diff := fmt.Sprintf("+%d -%d • %d file(s)", d.Additions, d.Deletions, d.ChangedFiles)
				sb.WriteString(StyleValue.Render("  "+truncate(diff, width-4)) + "\n")
				sb.WriteString(StyleLabel.Render("Repo") + StyleValue.Render(truncate(pr.Repo, width-10)) + "\n")
				if d.Body != "" {
					first := firstLine(d.Body)
					sb.WriteString(StyleDim.Render("  "+truncate(first, width-4)) + "\n")
				}
			default:
				sb.WriteString(StyleDim.Render(fmt.Sprintf("  #%d  %s", pr.Number, truncate(pr.Repo, width-12))) + "\n")
			}
		} else {
			sb.WriteString(StyleDim.Render(fmt.Sprintf("  #%d  %s", pr.Number, truncate(pr.Repo, width-12))) + "\n")
		}
		sb.WriteString("\n")
	}

	// ── Branch info section ───────────────────────────────────────────────────
	sb.WriteString(StyleTitle.Render("BRANCH INFO") + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-2, 4, 60))) + "\n")

	if m.services.LoadBranchInfo == nil {
		sb.WriteString(StyleDim.Render("  (unavailable)") + "\n")
		return sb.String()
	}
	if m.branchModified == nil && m.branchAheadCommits == nil {
		sb.WriteString(StyleDim.Render("  loading…") + "\n")
		return sb.String()
	}

	modCount := len(m.branchModified)
	sb.WriteString(StyleLabel.Render("Modified") + StyleValue.Render(fmt.Sprintf("%d file(s)", modCount)) + "\n")
	for _, f := range m.branchModified {
		sb.WriteString(StyleDim.Render("  "+truncate(f, width-4)) + "\n")
	}

	aheadCount := len(m.branchAheadCommits)
	sb.WriteString("\n")
	sb.WriteString(StyleLabel.Render("Ahead") + StyleValue.Render(fmt.Sprintf("%d commit(s)", aheadCount)) + "\n")
	for _, c := range m.branchAheadCommits {
		sb.WriteString(StyleDim.Render("  "+truncate(c, width-4)) + "\n")
	}

	return sb.String()
}

// viewDetailPreview renders the right panel: live pane preview of the selected neuron.
func (m Model) viewDetailPreview(width int) string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("LIVE PREVIEW") + "\n")

	previewLabel := "─"
	if !m.previewEnabled {
		previewLabel = "─ [off] "
	}
	sb.WriteString(StyleDim.Render(previewLabel+strings.Repeat("─", clamp(width-len(previewLabel)-2, 4, 60))) + "\n")

	if !m.previewEnabled {
		sb.WriteString(StyleDim.Render("  press p to enable"))
		return sb.String()
	}
	if m.services.CapturePane == nil {
		sb.WriteString(StyleDim.Render("  (preview not available)"))
		return sb.String()
	}
	if m.paneContent == "" {
		sb.WriteString(StyleDim.Render("  loading…"))
		return sb.String()
	}

	headerLines := 3 // title + separator + padding
	previewLines := m.contentHeight() - headerLines
	if previewLines < 1 {
		previewLines = 1
	}

	lines := strings.Split(strings.TrimRight(m.paneContent, "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > previewLines {
		lines = lines[len(lines)-previewLines:]
	}
	for _, l := range lines {
		sb.WriteString(truncate(l, width-2) + "\n")
	}
	return sb.String()
}

// viewDetailStatusBar renders the status bar for the detail dashboard.
func (m Model) viewDetailStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	return StyleHelp.Render("  q back   j/k neurons   g goto   a add neuron   d del neuron   P add PR   p preview   r refresh")
}

// firstLine returns the first non-empty line of s, useful for PR body previews.
func firstLine(s string) string {
	for _, l := range strings.SplitN(s, "\n", 10) {
		l = strings.TrimSpace(l)
		if l != "" {
			return l
		}
	}
	return s
}

// ── Right panel in the main list view ────────────────────────────────────────

// viewNucleusSummary renders the right-panel summary card for the selected
// nucleus in StateList. Shows agent/nvim counts, per-neuron branch+repo, and
// any linked Jira ticket or PRs. No network calls — data comes from registry.
func (m Model) viewNucleusSummary(width int) string {
	if len(m.nuclei) == 0 {
		return ""
	}
	n := m.nuclei[m.cursor]
	sep := StyleDim.Render(strings.Repeat("─", clamp(width-4, 4, 60)))

	var sb strings.Builder

	// ── Title ─────────────────────────────────────────────────────────────────
	sb.WriteString(StyleTitle.Render(truncate(n.TaskDescription, width-4)) + "\n")
	sb.WriteString(sep + "\n")

	// ── Counts ────────────────────────────────────────────────────────────────
	var claudeCount, nvimCount int
	var claudeDots strings.Builder
	for _, neu := range n.Neurons {
		switch neu.Type {
		case registry.NeuronClaude:
			claudeCount++
			claudeDots.WriteString(StatusDot(neu.Status))
		case registry.NeuronNvim:
			nvimCount++
		}
	}

	dotsStr := claudeDots.String()
	if claudeCount == 0 {
		dotsStr = StyleDim.Render("─")
	}
	agentsStr := fmt.Sprintf("%d agent(s)  %s", claudeCount, dotsStr)
	nvimStr := fmt.Sprintf("nvim  %d", nvimCount)
	sb.WriteString(" " + agentsStr + "\n")
	sb.WriteString(" " + StyleDim.Render(nvimStr) + "\n")

	// ── Neurons ───────────────────────────────────────────────────────────────
	if len(n.Neurons) > 0 {
		sb.WriteString("\n")
		sb.WriteString(" " + StyleDim.Render("── Neurons "+strings.Repeat("─", clamp(width-14, 2, 50))) + "\n")
		for _, neu := range n.Neurons {
			var icon string
			switch neu.Type {
			case registry.NeuronClaude:
				icon = StatusDot(neu.Status)
			case registry.NeuronNvim:
				icon = StyleDim.Render("◆")
			default:
				icon = StyleDim.Render("○")
			}
			repoBase := filepath.Base(neu.RepoPath)
			branch := neu.Branch
			if branch == "" {
				branch = "—"
			}
			if repoBase == "" || repoBase == "." {
				repoBase = "—"
			}
			idStr := truncate(neu.ID, 6)
			branchStr := truncate(branch, width/2-8)
			repoStr := StyleDim.Render(truncate(repoBase, width/3))
			sb.WriteString(fmt.Sprintf("  %s %-6s  %s  %s\n", icon, idStr, branchStr, repoStr))
		}
	}

	// ── Links (Jira + PRs) ────────────────────────────────────────────────────
	if n.JiraKey != "" || len(n.PullRequests) > 0 {
		sb.WriteString("\n")
		sb.WriteString(" " + StyleDim.Render("── Links "+strings.Repeat("─", clamp(width-12, 2, 50))) + "\n")

		if n.JiraKey != "" {
			jiraStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
			sb.WriteString("  " + jiraStyle.Render(n.JiraKey) + "\n")
		}
		for _, pr := range n.PullRequests {
			prStyle := lipgloss.NewStyle().Foreground(ColorAccent)
			label := fmt.Sprintf("#%d", pr.Number)
			if pr.Repo != "" {
				label += "  " + pr.Repo
			}
			sb.WriteString("  " + prStyle.Render(truncate(label, width-6)) + "\n")
		}
	}

	return sb.String()
}
