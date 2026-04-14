package ui

import (
	"fmt"
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
		}
		return m, m.captureDetailPaneCmd()

	case matchKey(msg, m.keys.Down):
		if len(m.nuclei) > 0 {
			n := m.nuclei[m.cursor]
			if m.detailNeuronIdx < len(n.Neurons)-1 {
				m.detailNeuronIdx++
				m.paneContent = ""
			}
		}
		return m, m.captureDetailPaneCmd()

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
		m.state = stateNeuronAdd
		return m, nil

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

	case matchKey(msg, m.keys.Refresh):
		m.branchModified = nil
		m.branchAheadCommits = nil
		return m, m.loadBranchInfoCmd()

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
	left := StyleHeader.Render("◈  NUCLEUS " + n.ID + "  •  " + truncate(n.Branch, 40) + "  •  " + n.Status)
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
	return StyleHelp.Render("  q back   j/k neurons   g goto   a add neuron   p preview   r refresh")
}

// ── Right panel in the main list view (unchanged) ─────────────────────────────

// viewNucleusDetail renders the right-panel detail for the selected nucleus
// in the main list view (StateList).
func (m Model) viewNucleusDetail(width int) string {
	if len(m.nuclei) == 0 {
		return ""
	}
	n := m.nuclei[m.cursor]

	var sb strings.Builder

	sb.WriteString(StyleTitle.Render(truncate(n.TaskDescription, width-4)) + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-4, 4, 60))) + "\n")
	field := func(label, value string) string {
		return StyleLabel.Render(label) + StyleValue.Render(truncate(value, width-16)) + "\n"
	}
	sb.WriteString(field("ID", n.ID))
	sb.WriteString(field("Branch", n.Branch))
	sb.WriteString(StyleLabel.Render("Status") + StatusDot(n.Status) + " " + n.Status + "\n")
	primaryTarget := "—"
	if primary := n.PrimaryNeuron(); primary != nil {
		primaryTarget = primary.TmuxTarget
	}
	sb.WriteString(field("Claude", primaryTarget))
	nvimVal := "—"
	if nvim := n.NvimNeuron(); nvim != nil && nvim.TmuxTarget != "" {
		nvimVal = nvim.TmuxTarget
	}
	sb.WriteString(field("Nvim", nvimVal))

	headerLines := 7

	previewHeaderLines := 2
	previewLines := m.contentHeight() - headerLines - previewHeaderLines
	if previewLines < 1 {
		previewLines = 1
	}

	sb.WriteString("\n")
	previewLabel := "── Preview "
	if !m.previewEnabled {
		previewLabel = "── Preview [off] "
	}
	sb.WriteString(StyleDim.Render(previewLabel+strings.Repeat("─", clamp(width-len(previewLabel)-4, 4, 40))) + "\n")

	if !m.previewEnabled {
		sb.WriteString(StyleDim.Render("  press p to enable"))
	} else if m.services.CapturePane == nil {
		sb.WriteString(StyleDim.Render("  (preview not available)"))
	} else if m.paneContent == "" {
		sb.WriteString(StyleDim.Render("  loading…"))
	} else {
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
			sb.WriteString(fmt.Sprintf("%s\n", truncate(l, width-2)))
		}
	}

	return sb.String()
}
