package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// updateJiraBoard handles key events on the Jira kanban board.
func (m Model) updateJiraBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.jiraNucleusPick {
		return m.updateJiraNucleusPicker(msg)
	}
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateList
		return m, nil

	case matchKey(msg, m.keys.Down):
		if len(m.jiraColumns) > 0 {
			col := m.jiraColumns[m.jiraColIdx]
			if m.jiraRowIdx < len(m.jiraIssues[col])-1 {
				m.jiraRowIdx++
			}
		}

	case matchKey(msg, m.keys.Up):
		if m.jiraRowIdx > 0 {
			m.jiraRowIdx--
		}

	case msg.String() == "h" || msg.Type == tea.KeyLeft:
		if m.jiraColIdx > 0 {
			m.jiraColIdx--
			m.jiraRowIdx = 0
		}

	case msg.String() == "l" || msg.Type == tea.KeyRight:
		if m.jiraColIdx < len(m.jiraColumns)-1 {
			m.jiraColIdx++
			m.jiraRowIdx = 0
		}

	case matchKey(msg, m.keys.Refresh):
		m.jiraLoading = true
		return m, m.loadJiraBoardCmd()

	case msg.Type == tea.KeySpace:
		if m.services.LoadJiraIssue == nil || len(m.jiraColumns) == 0 {
			break
		}
		col := m.jiraColumns[m.jiraColIdx]
		issues := m.jiraIssues[col]
		if m.jiraRowIdx >= len(issues) {
			break
		}
		issue := issues[m.jiraRowIdx]
		m.jiraDetailURL = issue.URL
		m.jiraDetailLoading = true
		return m, m.loadJiraIssueCmd(issue.Key, issue.Summary)

	case matchKey(msg, m.keys.OpenBrowser):
		if m.services.BrowserOpen == nil || len(m.jiraColumns) == 0 {
			break
		}
		col := m.jiraColumns[m.jiraColIdx]
		issues := m.jiraIssues[col]
		if m.jiraRowIdx >= len(issues) {
			break
		}
		url := issues[m.jiraRowIdx].URL
		if url == "" {
			break
		}
		svc := m.services.BrowserOpen
		return m, func() tea.Msg {
			_ = svc(url)
			return nil
		}

	case msg.String() == "N", matchKey(msg, m.keys.New):
		// Create a Nucleus from the selected Jira issue: pre-fill the modal
		// with the issue summary (task) and a branch prefix of task/<key>/.
		if len(m.jiraColumns) == 0 {
			break
		}
		col := m.jiraColumns[m.jiraColIdx]
		issues := m.jiraIssues[col]
		if m.jiraRowIdx >= len(issues) {
			break
		}
		issue := issues[m.jiraRowIdx]
		return m.openNucleusModal(NucleusModalContext{
			JiraKey:     issue.Key,
			JiraSummary: issue.Summary,
		})

	case msg.String() == "a":
		// Attach selected Jira issue to an existing nucleus.
		if m.services.AddJiraKey == nil || len(m.jiraColumns) == 0 {
			break
		}
		col := m.jiraColumns[m.jiraColIdx]
		issues := m.jiraIssues[col]
		if m.jiraRowIdx >= len(issues) {
			break
		}
		m.jiraPendingKey = issues[m.jiraRowIdx].Key
		m.jiraNucleusPick = true
		m.githubPickerFilter = ""
		m.githubPickerCursor = 0
		m.githubPickerNuclei = m.nuclei
		return m, nil
	}
	return m.jiraAdjustScroll(), nil
}

// updateJiraNucleusPicker handles key events for the nucleus selection overlay
// shown when attaching a Jira ticket to an existing nucleus.
func (m Model) updateJiraNucleusPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := githubFilteredNuclei(m.githubPickerNuclei, m.githubPickerFilter)
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.jiraNucleusPick = false
		m.jiraPendingKey = ""
		m.githubPickerFilter = ""
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubPickerCursor > 0 {
			m.githubPickerCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.githubPickerCursor < len(filtered)-1 {
			m.githubPickerCursor++
		}

	case msg.Type == tea.KeyBackspace:
		if len(m.githubPickerFilter) > 0 {
			m.githubPickerFilter = m.githubPickerFilter[:len(m.githubPickerFilter)-1]
			m.githubPickerCursor = 0
		}

	case matchKey(msg, m.keys.Submit):
		if len(filtered) == 0 {
			return m, nil
		}
		if m.githubPickerCursor >= len(filtered) {
			m.githubPickerCursor = 0
		}
		nucleus := filtered[m.githubPickerCursor]
		key := m.jiraPendingKey
		svc := m.services.AddJiraKey
		m.jiraNucleusPick = false
		m.jiraPendingKey = ""
		m.githubPickerFilter = ""
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleus.ID, key)}
		}

	default:
		if len(msg.Runes) > 0 {
			m.githubPickerFilter += string(msg.Runes)
			m.githubPickerCursor = 0
		}
	}
	return m, nil
}

// updateJiraDetail handles key events on the Jira issue detail overlay.
func (m Model) updateJiraDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	contentH := m.height - 5 // visible lines in the detail body

	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.jiraDetailKey = ""
		m.state = stateJiraBoard
		return m, nil

	case matchKey(msg, m.keys.Down):
		m.jiraDetailScroll++

	case matchKey(msg, m.keys.Up):
		if m.jiraDetailScroll > 0 {
			m.jiraDetailScroll--
		}

	case msg.Type == tea.KeyPgDown:
		m.jiraDetailScroll += contentH

	case msg.Type == tea.KeyPgUp:
		m.jiraDetailScroll -= contentH
		if m.jiraDetailScroll < 0 {
			m.jiraDetailScroll = 0
		}

	case matchKey(msg, m.keys.OpenBrowser):
		if m.jiraDetailURL == "" || m.services.BrowserOpen == nil {
			return m, nil
		}
		svc := m.services.BrowserOpen
		url := m.jiraDetailURL
		return m, func() tea.Msg {
			_ = svc(url)
			return nil
		}

	case matchKey(msg, m.keys.New):
		return m.openNucleusModal(NucleusModalContext{})
	}
	return m, nil
}

// viewJiraBoard renders the full Jira board screen, with the nucleus-picker
// overlay when attaching a ticket to an existing nucleus.
func (m Model) viewJiraBoard() string {
	header := m.viewHeader()
	body := m.viewJiraBoardBody()
	statusBar := m.viewJiraBoardStatusBar()
	base := lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
	if m.jiraNucleusPick {
		return m.renderOverlay(base, m.viewJiraNucleusPicker())
	}
	return base
}

// viewJiraBoardBody renders the multi-column kanban body.
func (m Model) viewJiraBoardBody() string {
	if m.services.LoadJiraBoard == nil {
		return "\n" + StyleDim.Render("  Jira not configured — add a 'jira' block to ~/.config/exocortex/config.json")
	}
	if m.jiraLoading && m.jiraIssues == nil {
		return "\n" + StyleDim.Render("  Loading board…")
	}
	if m.jiraErr != "" && m.jiraIssues == nil {
		return "\n" + StyleError.Render("  ✗ "+m.jiraErr)
	}

	numCols := len(m.jiraColumns)
	if numCols == 0 {
		return "\n" + StyleDim.Render("  No columns configured")
	}

	h := m.contentHeight()
	// Divide width equally; account for numCols-1 separator characters.
	colWidth := (m.width - (numCols - 1)) / numCols

	// Build separator: h lines of "║" (double vertical) spanning the full body height, accent-colored.
	sepLines := make([]string, h)
	for i := range sepLines {
		sepLines[i] = "║"
	}
	sep := lipgloss.NewStyle().Foreground(ColorAccent).Render(strings.Join(sepLines, "\n"))

	parts := make([]string, 0, numCols*2-1)
	for ci, status := range m.jiraColumns {
		content := m.renderJiraColumn(ci, status, colWidth, h)
		clipped := clipLines(content, h)
		box := lipgloss.NewStyle().Width(colWidth).Height(h).Render(clipped)
		parts = append(parts, box)
		if ci < numCols-1 {
			parts = append(parts, sep)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderJiraColumn renders a single kanban column.
func (m Model) renderJiraColumn(colIdx int, status string, width, height int) string {
	allIssues := m.jiraIssues[status]

	var sb strings.Builder

	// Column header with accent bar and double line separator.
	bar := lipgloss.NewStyle().Foreground(ColorAccent).Render("▌")
	titleStyle := StyleDim
	if colIdx == m.jiraColIdx {
		titleStyle = StyleValue
	}
	count := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(fmt.Sprintf("%d", len(allIssues)))
	lowerStatus := strings.ToLower(status)
	statusStr := strings.ToUpper(lowerStatus[:1]) + lowerStatus[1:]
	headerStr := bar + " " + titleStyle.Render(statusStr) + "  " + count
	sb.WriteString(truncate(headerStr, width-2) + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render(strings.Repeat("═", clamp(width-2, 4, 60))) + "\n")

	// Compute the visible window.
	off := 0
	if colIdx < len(m.jiraScrollOffs) {
		off = m.jiraScrollOffs[colIdx]
	}
	maxVis := m.jiraMaxVisible()
	end := off + maxVis
	if end > len(allIssues) {
		end = len(allIssues)
	}
	visible := allIssues[off:end]

	// Issue rows (3 lines each + blank line between issues).
	for vi, issue := range visible {
		actualIdx := vi + off
		selected := colIdx == m.jiraColIdx && actualIdx == m.jiraRowIdx

		keyLine := issue.Key
		summaryLine := "  " + issue.Summary
		assigneeLine := ""
		if issue.Assignee != "" {
			first := strings.Fields(issue.Assignee)
			if len(first) > 0 {
				assigneeLine = "  @" + first[0]
			}
		}

		if selected {
			sb.WriteString(StyleSelected.Width(width).Render("▶ "+truncate(keyLine, width-4)) + "\n")
			sb.WriteString(StyleSelected.Width(width).Foreground(ColorDim).Render(truncate(summaryLine, width-2)) + "\n")
			sb.WriteString(StyleSelected.Width(width).Foreground(ColorDim).Render(truncate(assigneeLine, width-2)) + "\n")
		} else {
			sb.WriteString("  " + truncate(keyLine, width-4) + "\n")
			sb.WriteString(StyleDim.Render(truncate(summaryLine, width-2)) + "\n")
			sb.WriteString(StyleDim.Render(truncate(assigneeLine, width-2)) + "\n")
		}
		if vi < len(visible)-1 {
			sb.WriteString(StyleDim.Render("  ·") + "\n")
		}
	}

	return clipLines(sb.String(), height)
}

// viewJiraBoardStatusBar renders the status bar for the board view.
func (m Model) viewJiraBoardStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	if m.jiraLoading {
		return StyleHelp.Render("  refreshing…")
	}
	hint := "  b/esc back   j/k row   h/l column   space detail   o browser   N new   a attach   r refresh"
	if !m.jiraLastRefresh.IsZero() {
		return StyleHelp.Render(fmt.Sprintf("  updated %s ·%s", fmtAge(m.jiraLastRefresh), hint))
	}
	return StyleHelp.Render(hint)
}

// viewJiraDetail renders the full-screen Jira issue description overlay.
func (m Model) viewJiraDetail() string {
	renderWidth := m.width - 4
	if renderWidth < 20 {
		renderWidth = 20
	}

	content := m.jiraDetailMD
	if m.jiraDetailLoading {
		content = "_Loading…_\n"
	} else if content == "" {
		content = "_No description._\n"
	}

	rendered := content
	if r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(renderWidth),
	); err == nil {
		if out, err := r.Render(content); err == nil {
			rendered = out
		}
	}

	lines := strings.Split(rendered, "\n")

	// Clamp scroll.
	contentH := m.height - 5
	if contentH < 1 {
		contentH = 1
	}
	maxScroll := len(lines) - contentH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.jiraDetailScroll > maxScroll {
		m.jiraDetailScroll = maxScroll
	}
	if m.jiraDetailScroll < 0 {
		m.jiraDetailScroll = 0
	}

	end := m.jiraDetailScroll + contentH
	if end > len(lines) {
		end = len(lines)
	}
	visible := strings.Join(lines[m.jiraDetailScroll:end], "\n")

	// Header.
	title := StyleTitle.Render(truncate(m.jiraDetailTitle, m.width-4))
	scrollInfo := ""
	if len(lines) > contentH && maxScroll > 0 {
		scrollInfo = StyleDim.Render(fmt.Sprintf(" %d%%", 100*m.jiraDetailScroll/maxScroll))
	}
	divider := StyleDim.Render(strings.Repeat("─", m.width-2))
	header := title + scrollInfo + "\n" + divider + "\n"

	var browserHint string
	if m.jiraDetailURL != "" && m.services.BrowserOpen != nil {
		browserHint = "   o browser"
	}
	statusBar := StyleHelp.Render("  esc back   j/k scroll   pgdn/pgup page" + browserHint)

	return lipgloss.JoinVertical(lipgloss.Left, header, visible, statusBar)
}

// loadJiraBoardCmd fires an async Jira board fetch.
func (m Model) loadJiraBoardCmd() tea.Cmd {
	svc := m.services.LoadJiraBoard
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		columns, issues, err := svc()
		return jiraBoardLoadedMsg{columns: columns, issues: issues, err: err}
	}
}

// loadJiraIssueCmd fires an async fetch for a single Jira issue description.
func (m Model) loadJiraIssueCmd(key, summary string) tea.Cmd {
	svc := m.services.LoadJiraIssue
	return func() tea.Msg {
		md, err := svc(key)
		return jiraIssueLoadedMsg{key: key, title: key + " — " + summary, markdown: md, err: err}
	}
}

// jiraMaxVisible returns how many issues fit in the visible column area.
// Each issue renders as 3 lines; issues are separated by 1 blank line.
// Column header takes 2 lines (title + divider), leaving h-2 lines for issues.
// n issues need 4n-1 lines (for n>0), so max n = (h-1)/4.
func (m Model) jiraMaxVisible() int {
	h := m.contentHeight() - 2
	if h <= 0 {
		return 1
	}
	v := (h + 1) / 4
	if v < 1 {
		return 1
	}
	return v
}

// viewJiraNucleusPicker renders the nucleus selection overlay for attaching a
// Jira ticket. Reuses the githubPickerFilter/Cursor/Nuclei fields.
func (m Model) viewJiraNucleusPicker() string {
	filtered := githubFilteredNuclei(m.githubPickerNuclei, m.githubPickerFilter)

	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Attach  "+m.jiraPendingKey+"  to nucleus") + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", 36)) + "\n")
	sb.WriteString(StyleLabel.Render("Filter: ") + m.githubPickerFilter + "█\n\n")

	if len(filtered) == 0 {
		sb.WriteString(StyleDim.Render("  no matching nuclei") + "\n")
	} else {
		for i, n := range filtered {
			label := n.ID + "  " + truncate(n.TaskDescription, 28)
			if i == m.githubPickerCursor {
				sb.WriteString(StyleSelected.Render("▶ "+label) + "\n")
			} else {
				sb.WriteString("  " + label + "\n")
			}
		}
	}
	sb.WriteString("\n" + StyleDim.Render(strings.Repeat("─", 36)) + "\n")
	sb.WriteString(StyleHelp.Render("  enter attach   esc cancel   type to filter"))
	return sb.String()
}

// viewJiraBoardStatusBar renders the status bar for the Jira board.
// (declared here to keep the attach hint close to the handler)

// jiraAdjustScroll updates the per-column scroll offset so the selected row
// stays within the visible window.
func (m Model) jiraAdjustScroll() Model {
	if len(m.jiraScrollOffs) != len(m.jiraColumns) {
		m.jiraScrollOffs = make([]int, len(m.jiraColumns))
	}
	ci := m.jiraColIdx
	maxVis := m.jiraMaxVisible()
	off := m.jiraScrollOffs[ci]
	if m.jiraRowIdx < off {
		off = m.jiraRowIdx
	} else if m.jiraRowIdx >= off+maxVis {
		off = m.jiraRowIdx - maxVis + 1
	}
	if off < 0 {
		off = 0
	}
	m.jiraScrollOffs[ci] = off
	return m
}
