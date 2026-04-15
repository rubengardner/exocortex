package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/github"
)

// ── StateGitHubView ──────────────────────────────────────────────────────────

// updateGitHubView handles key events for the GitHub PR list.
func (m Model) updateGitHubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateList
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubPRCursor > 0 {
			m.githubPRCursor--
			return m.startPreviewLoad()
		}

	case matchKey(msg, m.keys.Down):
		if m.githubPRCursor < len(m.githubPRs)-1 {
			m.githubPRCursor++
			return m.startPreviewLoad()
		}

	case matchKey(msg, m.keys.Refresh):
		if m.services.LoadGitHubPRs != nil && !m.githubLoading {
			m.githubLoading = true
			return m, m.loadGitHubPRsCmd()
		}

	case matchKey(msg, m.keys.Submit), msg.Type == tea.KeyRight:
		if len(m.githubPRs) == 0 || m.services.LoadGitHubPR == nil {
			return m, nil
		}
		pr := m.githubPRs[m.githubPRCursor]
		// Re-use the hover preview if it's already loaded for this PR.
		if m.githubPreviewPR != nil &&
			m.githubPreviewPR.Number == pr.Number &&
			m.githubPreviewPR.Repo == pr.Repo {
			m.githubDetailPR = m.githubPreviewPR
			m.githubDetailScroll = 0
			m.githubDetailFileCursor = 0
			m.githubFileExpanded = make([]bool, len(m.githubDetailPR.Files))
			m.state = stateGitHubPRDetail
			return m, nil
		}
		m.githubDetailLoading = true
		m.githubDetailPR = nil
		return m, m.loadGitHubPRDetailCmd(pr.Repo, pr.Number)

	case matchKey(msg, m.keys.OpenBrowser):
		if len(m.githubPRs) == 0 || m.services.BrowserOpen == nil {
			return m, nil
		}
		url := m.githubPRs[m.githubPRCursor].URL
		if url == "" {
			return m, nil
		}
		svc := m.services.BrowserOpen
		return m, func() tea.Msg {
			_ = svc(url)
			return nil
		}

	case matchKey(msg, m.keys.Respawn): // R = start review workflow on selected PR
		if len(m.githubPRs) == 0 || m.services.CreateReviewNucleus == nil {
			return m, nil
		}
		pr := m.githubPRs[m.githubPRCursor]
		return m.startReviewWorkflow(pr.Number, pr.Repo, pr.Branch)
	}
	return m, nil
}

// startPreviewLoad updates preview state on m and returns the load cmd.
// Must be called after the cursor has already been updated on m.
func (m Model) startPreviewLoad() (tea.Model, tea.Cmd) {
	if m.services.LoadGitHubPR == nil || len(m.githubPRs) == 0 {
		return m, nil
	}
	pr := m.githubPRs[m.githubPRCursor]
	if m.githubPreviewPR != nil && m.githubPreviewPR.Number == pr.Number {
		return m, nil // already loaded
	}
	m.githubPreviewNum = pr.Number
	m.githubPreviewLoading = true
	m.githubPreviewPR = nil
	return m, m.loadGitHubPRPreviewCmd(pr.Repo, pr.Number)
}

// ── StateGitHubPRDetail ───────────────────────────────────────────────────────

// updateGitHubPRDetail handles key events for the PR file-accordion detail.
func (m Model) updateGitHubPRDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rightH := m.contentHeight()

	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateGitHubView
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubDetailFileCursor > 0 {
			m.githubDetailFileCursor--
			m.githubAccordionAdjustScroll(rightH)
		}

	case matchKey(msg, m.keys.Down):
		if m.githubDetailPR != nil && m.githubDetailFileCursor < len(m.githubDetailPR.Files)-1 {
			m.githubDetailFileCursor++
			m.githubAccordionAdjustScroll(rightH)
		}

	case msg.Type == tea.KeySpace:
		if m.githubDetailPR == nil || m.githubDetailFileCursor >= len(m.githubFileExpanded) {
			break
		}
		m.githubFileExpanded[m.githubDetailFileCursor] = !m.githubFileExpanded[m.githubDetailFileCursor]
		m.githubAccordionAdjustScroll(rightH)

	case msg.String() == "pgdown":
		m.githubDetailScroll += rightH / 2
	case msg.String() == "pgup":
		m.githubDetailScroll -= rightH / 2
		if m.githubDetailScroll < 0 {
			m.githubDetailScroll = 0
		}

	case matchKey(msg, m.keys.Respawn): // R = start review workflow on current PR
		if m.githubDetailPR == nil || m.services.CreateReviewNucleus == nil {
			return m, nil
		}
		d := m.githubDetailPR
		return m.startReviewWorkflow(d.Number, d.Repo, d.Branch)

	case matchKey(msg, m.keys.Nvim):
		if m.githubDetailPR == nil || m.services.OpenNvimFile == nil {
			return m, nil
		}
		nucleusID := m.prLinkedNucleusID(m.githubDetailPR)
		if nucleusID == "" {
			m.lastErr = "no nucleus linked to this PR"
			return m, nil
		}
		files := m.githubDetailPR.Files
		if len(files) == 0 {
			return m, nil
		}
		idx := m.githubDetailFileCursor
		if idx >= len(files) {
			idx = 0
		}
		f := files[idx]
		svc := m.services.OpenNvimFile
		nid := nucleusID
		path := f.Path
		line := firstHunkLine(f.Patch)
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nid, path, line)}
		}

	case matchKey(msg, m.keys.OpenBrowser):
		if m.githubDetailPR == nil || m.githubDetailPR.URL == "" || m.services.BrowserOpen == nil {
			return m, nil
		}
		svc := m.services.BrowserOpen
		url := m.githubDetailPR.URL
		return m, func() tea.Msg {
			_ = svc(url)
			return nil
		}
	}
	return m, nil
}

// githubAccordionAdjustScroll ensures the selected file is within the visible
// scroll window of the accordion right panel.
func (m *Model) githubAccordionAdjustScroll(rightH int) {
	if m.githubDetailPR == nil {
		return
	}
	start := githubAccordionFileStartLine(m.githubDetailPR.Files, m.githubFileExpanded, m.githubDetailFileCursor)
	if start < m.githubDetailScroll {
		m.githubDetailScroll = start
	} else if start >= m.githubDetailScroll+rightH {
		m.githubDetailScroll = start - rightH + 2
	}
	if m.githubDetailScroll < 0 {
		m.githubDetailScroll = 0
	}
}

// githubAccordionFileStartLine returns the zero-based line index where file idx
// begins in the rendered accordion (after the 5-line header block).
func githubAccordionFileStartLine(files []github.PRFile, expanded []bool, idx int) int {
	const headerLines = 5 // blank + title + meta + divider + blank
	line := headerLines
	for i := 0; i < idx && i < len(files); i++ {
		line++ // file row
		if i < len(expanded) && expanded[i] && files[i].Patch != "" {
			line += len(strings.Split(files[i].Patch, "\n"))
		}
		line++ // blank separator
	}
	return line
}

// ── View helpers ──────────────────────────────────────────────────────────────

func (m Model) viewGitHubView() string {
	header := m.viewGitHubHeader()
	body := m.viewGitHubSplitBody(false)
	status := m.viewGitHubStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m Model) viewGitHubPRDetail() string {
	header := m.viewGitHubHeader()
	body := m.viewGitHubSplitBody(true)
	status := m.viewGitHubDetailStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

// viewGitHubSplitBody builds the two-panel body: PR list on the left, and either
// description preview (accordionMode=false) or file accordion (accordionMode=true)
// on the right.
func (m Model) viewGitHubSplitBody(accordionMode bool) string {
	h := m.contentHeight()
	listW := clamp(m.width*2/5, 28, 55)
	rightW := m.width - listW - 1 // -1 for the border

	left := clipLines(m.renderGitHubListPanel(listW), h)
	var right string
	if accordionMode {
		lines := m.renderGitHubAccordionLines(rightW)
		// Apply scroll window.
		start := m.githubDetailScroll
		if start > len(lines) {
			start = len(lines)
		}
		end := start + h
		if end > len(lines) {
			end = len(lines)
		}
		right = strings.Join(lines[start:end], "\n")
	} else {
		right = clipLines(m.renderGitHubPreviewPanel(rightW), h)
	}

	listPane := StyleListPane.Height(h).Width(listW).Render(left)
	detailPane := StyleDetailPane.Height(h).Width(rightW).Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)
}

func (m Model) viewGitHubHeader() string {
	label := fmt.Sprintf("%d PR(s)", len(m.githubPRs))
	left := StyleHeader.Render("◈  GITHUB PULL REQUESTS")
	right := StyleMuted.Render(label)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// renderGitHubListPanel renders the PR list into a panel of width listW.
func (m Model) renderGitHubListPanel(listW int) string {
	if m.githubLoading {
		return StyleDim.Render("  loading…")
	}
	if m.githubErr != "" {
		return StyleError.Render("  ✗ " + m.githubErr)
	}
	if m.services.LoadGitHubPRs == nil {
		return StyleDim.Render("  GitHub not configured")
	}
	if len(m.githubPRs) == 0 {
		return StyleDim.Render("  no open PRs")
	}

	var sb strings.Builder
	for i, pr := range m.githubPRs {
		dot := prStateDot(pr.State)
		numStr := fmt.Sprintf("#%d", pr.Number)
		titleW := listW - 12
		if titleW < 8 {
			titleW = 8
		}
		title := truncate(pr.Title, titleW)
		age := fmtAge(pr.UpdatedAt)
		branchStr := truncate(pr.Branch, listW-16)

		line1 := fmt.Sprintf(" %s %-7s %s", dot, numStr, title)
		line2 := "   " + pr.State + "  " + branchStr + "  " + age

		if i == m.githubPRCursor {
			sb.WriteString(StyleSelected.Width(listW).Render(line1) + "\n")
			sb.WriteString(StyleSelected.Width(listW).Foreground(ColorDim).Render(line2) + "\n")
		} else {
			sb.WriteString(line1 + "\n")
			sb.WriteString(StyleDim.Render(line2) + "\n")
		}
		if i < len(m.githubPRs)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderGitHubPreviewPanel renders the description + meta for the hovered PR.
func (m Model) renderGitHubPreviewPanel(rightW int) string {
	if len(m.githubPRs) == 0 {
		return ""
	}
	pr := m.githubPRs[m.githubPRCursor]
	contentW := rightW - 2
	if contentW < 10 {
		contentW = 10
	}

	var sb strings.Builder

	// Title + divider
	sb.WriteString(StyleTitle.Render(truncate(pr.Title, contentW)) + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(contentW, 4, 60))) + "\n")
	sb.WriteString("\n")

	// Meta
	sb.WriteString(StyleDim.Render(fmt.Sprintf("  %s  ·  by %s", pr.State, pr.Author)) + "\n")
	sb.WriteString(StyleDim.Render(fmt.Sprintf("  %s → %s", pr.Branch, pr.Base)) + "\n")
	sb.WriteString(StyleDim.Render(fmt.Sprintf("  updated %s", fmtAge(pr.UpdatedAt))) + "\n")
	sb.WriteString("\n")

	// Body from async preview
	switch {
	case m.githubPreviewLoading && (m.githubPreviewPR == nil || m.githubPreviewPR.Number != pr.Number):
		sb.WriteString(StyleDim.Render("  loading description…") + "\n")
	case m.githubPreviewPR != nil && m.githubPreviewPR.Number == pr.Number:
		body := strings.TrimSpace(m.githubPreviewPR.Body)
		if body == "" {
			sb.WriteString(StyleDim.Render("  (no description)") + "\n")
		} else {
			for _, line := range githubWordWrap(body, contentW-2) {
				sb.WriteString(StyleDim.Render("  "+line) + "\n")
			}
		}
	}

	return sb.String()
}

// renderGitHubAccordionLines builds the full list of rendered lines for the
// file accordion panel. The caller applies the scroll window.
func (m Model) renderGitHubAccordionLines(rightW int) []string {
	if m.githubDetailLoading {
		return []string{"", StyleDim.Render("  loading…")}
	}
	if m.githubDetailPR == nil {
		return []string{"", StyleDim.Render("  no detail available")}
	}
	d := m.githubDetailPR
	contentW := rightW - 2
	if contentW < 10 {
		contentW = 10
	}

	var lines []string

	// Header block (5 lines matching githubAccordionFileStartLine's headerLines=5)
	lines = append(lines, "")
	lines = append(lines, StyleTitle.Render(truncate(fmt.Sprintf("#%d  %s  [%s]", d.Number, d.Title, d.State), contentW)))
	lines = append(lines, StyleDim.Render(fmt.Sprintf("  +%d -%d  %d file(s)  by %s", d.Additions, d.Deletions, d.ChangedFiles, d.Author)))
	lines = append(lines, StyleDim.Render("  "+strings.Repeat("─", clamp(contentW-2, 4, 60))))
	lines = append(lines, "")

	for i, f := range d.Files {
		expanded := i < len(m.githubFileExpanded) && m.githubFileExpanded[i]
		selected := i == m.githubDetailFileCursor

		var indicator string
		switch {
		case selected && expanded:
			indicator = "▼ "
		case selected:
			indicator = "▶ "
		case expanded:
			indicator = "╴ "
		default:
			indicator = "  "
		}

		statusChar := prFileStatusChar(f.Status)
		pathW := contentW - len(indicator) - 2 - 12 // room for "+NNN -NNN"
		if pathW < 8 {
			pathW = 8
		}
		fileRow := indicator + statusChar + " " + truncate(f.Path, pathW) +
			fmt.Sprintf("  +%d -%d", f.Additions, f.Deletions)

		var fileLine string
		if selected {
			fileLine = StyleSelected.Width(rightW).Render(fileRow)
		} else {
			fileLine = lipgloss.NewStyle().Foreground(prFileStatusColor(f.Status)).Render(fileRow)
		}
		lines = append(lines, fileLine)

		if expanded && f.Patch != "" {
			for _, pl := range strings.Split(f.Patch, "\n") {
				lines = append(lines, githubPatchLine(truncate(pl, contentW-2)))
			}
		}

		lines = append(lines, "") // blank separator between files
	}
	return lines
}

// githubPatchLine colours a single unified-diff line.
func githubPatchLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+"):
		return lipgloss.NewStyle().Foreground(ColorWorking).Render("  " + line)
	case strings.HasPrefix(line, "-"):
		return lipgloss.NewStyle().Foreground(ColorBlocked).Render("  " + line)
	default:
		return StyleDim.Render("  " + line)
	}
}

func (m Model) viewGitHubStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	hint := "  q back   j/k select   enter detail"
	if m.services.BrowserOpen != nil {
		hint += "   o browser"
	}
	if m.services.CreateReviewNucleus != nil {
		hint += "   R review"
	}
	hint += "   r refresh"
	return StyleHelp.Render(hint)
}

func (m Model) viewGitHubDetailStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	hint := "  esc back   j/k file   space expand   pgdn/pgup scroll"
	if m.services.CreateReviewNucleus != nil {
		hint += "   R review"
	}
	if m.githubDetailPR != nil {
		if linked := m.prLinkedNucleusID(m.githubDetailPR); m.services.OpenNvimFile != nil && linked != "" {
			hint += "   e nvim"
		}
	}
	if m.services.BrowserOpen != nil {
		hint += "   o browser"
	}
	return StyleHelp.Render(hint)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// prLinkedNucleusID returns the ID of the nucleus linked to the given PR,
// or "" when none is found.
func (m Model) prLinkedNucleusID(pr *github.PRDetail) string {
	for _, n := range m.nuclei {
		if n.PRNumber == pr.Number && n.PRRepo == pr.Repo {
			return n.ID
		}
	}
	return ""
}

// firstHunkLine parses a unified diff patch and returns the target line number
// of the first added or context line, so nvim can jump to the right place.
// Falls back to 1 when the patch is empty or unparseable.
func firstHunkLine(patch string) int {
	for _, l := range strings.SplitN(patch, "\n", 20) {
		if strings.HasPrefix(l, "@@") {
			parts := strings.Fields(l)
			for _, p := range parts {
				if strings.HasPrefix(p, "+") && p != "+++ " {
					num := strings.TrimPrefix(p, "+")
					if comma := strings.IndexByte(num, ','); comma != -1 {
						num = num[:comma]
					}
					if n := parseInt(num); n > 0 {
						return n
					}
				}
			}
		}
	}
	return 1
}

// parseInt parses a decimal string, returning 0 on error.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// prStateDot returns a coloured bullet for a PR state.
func prStateDot(state string) string {
	var c lipgloss.Color
	switch state {
	case "open":
		c = ColorWorking
	case "draft":
		c = ColorMuted
	case "merged":
		c = ColorAccent
	case "closed":
		c = ColorBlocked
	default:
		c = ColorDim
	}
	return lipgloss.NewStyle().Foreground(c).Render("●")
}

func prFileStatusChar(status string) string {
	switch status {
	case "added":
		return "A"
	case "removed":
		return "D"
	case "renamed":
		return "R"
	case "modified":
		return "M"
	default:
		return "?"
	}
}

func prFileStatusColor(status string) lipgloss.Color {
	switch status {
	case "added":
		return ColorWorking
	case "removed":
		return ColorBlocked
	case "renamed":
		return ColorIdle
	default:
		return ColorDim
	}
}

// githubWordWrap wraps text at word boundaries to fit within width chars per line.
// Input newlines are respected as paragraph breaks.
func githubWordWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		if len(paragraph) == 0 {
			result = append(result, "")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := words[0]
		for _, word := range words[1:] {
			if len(line)+1+len(word) <= width {
				line += " " + word
			} else {
				result = append(result, line)
				line = word
			}
		}
		result = append(result, line)
	}
	return result
}
