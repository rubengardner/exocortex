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
		}

	case matchKey(msg, m.keys.Down):
		if m.githubPRCursor < len(m.githubPRs)-1 {
			m.githubPRCursor++
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
		m.githubDetailLoading = true
		m.githubDetailPR = nil
		return m, m.loadGitHubPRDetailCmd(pr.Repo, pr.Number)

	case matchKey(msg, m.keys.Respawn): // R = start review workflow on selected PR
		if len(m.githubPRs) == 0 || m.services.CreateReviewNucleus == nil {
			return m, nil
		}
		pr := m.githubPRs[m.githubPRCursor]
		return m.startReviewWorkflow(pr.Number, pr.Repo, pr.Branch)
	}
	return m, nil
}

// viewGitHubView renders the GitHub PR list.
func (m Model) viewGitHubView() string {
	header := m.viewGitHubHeader()
	body := m.viewGitHubList()
	status := m.viewGitHubStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
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

func (m Model) viewGitHubList() string {
	h := m.contentHeight()

	if m.githubLoading {
		return StyleDim.Render("  loading…")
	}
	if m.githubErr != "" {
		return StyleError.Render("  ✗ " + m.githubErr)
	}
	if m.services.LoadGitHubPRs == nil {
		return StyleDim.Render("  GitHub not configured  (add github.token to config.json)")
	}
	if len(m.githubPRs) == 0 {
		return StyleDim.Render("  no open PRs")
	}

	var sb strings.Builder
	for i, pr := range m.githubPRs {
		dot := prStateDot(pr.State)
		repoShort := truncate(pr.Repo, 22)
		title := truncate(pr.Title, m.width-36)
		age := fmtAge(pr.UpdatedAt)

		line1 := fmt.Sprintf(" %s #%-5d %-22s %s", dot, pr.Number, repoShort, title)
		line2 := StyleDim.Render(fmt.Sprintf("   %-8s  %s  %s", pr.State, truncate(pr.Branch, m.width-30), age))

		if i == m.githubPRCursor {
			sb.WriteString(StyleSelected.Width(m.width).Render(line1) + "\n")
			sb.WriteString(StyleSelected.Width(m.width).Foreground(ColorDim).Render("  "+pr.State+"  "+truncate(pr.Branch, m.width-12)+"  "+age) + "\n")
		} else {
			sb.WriteString(line1 + "\n")
			sb.WriteString(line2 + "\n")
		}
		if i < len(m.githubPRs)-1 {
			sb.WriteString("\n")
		}
	}
	return clipLines(sb.String(), h)
}

func (m Model) viewGitHubStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	hint := "  q back   j/k select   enter detail"
	if m.services.CreateReviewNucleus != nil {
		hint += "   R review"
	}
	hint += "   r refresh"
	return StyleHelp.Render(hint)
}

// ── StateGitHubPRDetail ───────────────────────────────────────────────────────

// updateGitHubPRDetail handles key events for the PR detail view.
// j/k navigate between changed files; pgdn/pgup scroll the patch body.
// e opens the selected file in the linked nucleus's nvim window.
func (m Model) updateGitHubPRDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateGitHubView
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubDetailFileCursor > 0 {
			m.githubDetailFileCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.githubDetailPR != nil && m.githubDetailFileCursor < len(m.githubDetailPR.Files)-1 {
			m.githubDetailFileCursor++
		}

	case msg.String() == "pgdown":
		if m.githubDetailPR != nil {
			lines := prDetailLines(m.githubDetailPR, m.width, -1)
			maxScroll := len(lines) - m.contentHeight() + 4
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.githubDetailScroll += m.contentHeight() / 2
			if m.githubDetailScroll > maxScroll {
				m.githubDetailScroll = maxScroll
			}
		}

	case msg.String() == "pgup":
		m.githubDetailScroll -= m.contentHeight() / 2
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
	}
	return m, nil
}

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
		// @@ -a,b +c,d @@ — extract c
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

// viewGitHubPRDetail renders the full-screen PR detail.
func (m Model) viewGitHubPRDetail() string {
	if m.githubDetailLoading {
		return StyleDim.Render("  loading PR detail…")
	}
	if m.githubDetailPR == nil {
		return StyleDim.Render("  no detail available")
	}
	d := m.githubDetailPR

	header := StyleHeader.Render(fmt.Sprintf("◈  PR #%d  %s  [%s]", d.Number, truncate(d.Title, m.width-30), d.State))
	meta := StyleDim.Render(fmt.Sprintf("  %s  →  %s  by %s   +%d -%d  %d file(s)",
		d.Branch, d.Base, d.Author, d.Additions, d.Deletions, d.ChangedFiles))

	cursor := m.githubDetailFileCursor
	lines := prDetailLines(d, m.width, cursor)
	h := m.contentHeight() - 3 // header + meta + status
	if h < 1 {
		h = 1
	}

	start := m.githubDetailScroll
	if start > len(lines) {
		start = len(lines)
	}
	end := start + h
	if end > len(lines) {
		end = len(lines)
	}
	body := strings.Join(lines[start:end], "\n")

	hint := "  esc back   j/k file   pgdn/pgup scroll"
	if m.services.CreateReviewNucleus != nil {
		hint += "   R review"
	}
	linked := m.prLinkedNucleusID(d)
	if m.services.OpenNvimFile != nil && linked != "" {
		hint += "   e open in nvim"
	}
	statusBar := StyleHelp.Render(hint)
	return lipgloss.JoinVertical(lipgloss.Left, header, meta, body, statusBar)
}

// prDetailLines builds the scrollable content lines for a PR detail.
// selectedFile highlights the file at that index (pass -1 for no highlight).
func prDetailLines(d *github.PRDetail, width, selectedFile int) []string {
	var lines []string

	// Body
	if d.Body != "" {
		lines = append(lines, "")
		for _, l := range strings.Split(d.Body, "\n") {
			lines = append(lines, truncate(l, width-4))
		}
	}

	// Files
	lines = append(lines, "")
	lines = append(lines, StyleTitle.Render(fmt.Sprintf("Changed Files (%d)", len(d.Files))))
	lines = append(lines, StyleDim.Render(strings.Repeat("─", clamp(width-4, 4, 80))))

	for i, f := range d.Files {
		statusColor := prFileStatusColor(f.Status)
		cursor := "  "
		if i == selectedFile {
			cursor = "▶ "
		}
		fileRow := fmt.Sprintf("%s%s %-*s +%d -%d", cursor, prFileStatusChar(f.Status), width-26, truncate(f.Path, width-26), f.Additions, f.Deletions)
		var fileLine string
		if i == selectedFile {
			fileLine = StyleSelected.Width(width).Render(fileRow)
		} else {
			fileLine = lipgloss.NewStyle().Foreground(statusColor).Render(fileRow)
		}
		lines = append(lines, fileLine)
		if f.Patch != "" {
			for _, pl := range strings.Split(f.Patch, "\n") {
				lines = append(lines, StyleDim.Render("    "+truncate(pl, width-8)))
			}
		}
	}
	return lines
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
