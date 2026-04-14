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
func (m Model) updateGitHubPRDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateGitHubView
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubDetailScroll > 0 {
			m.githubDetailScroll--
		}

	case matchKey(msg, m.keys.Down):
		if m.githubDetailPR != nil {
			lines := prDetailLines(m.githubDetailPR, m.width)
			maxScroll := len(lines) - m.contentHeight() + 4
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.githubDetailScroll < maxScroll {
				m.githubDetailScroll++
			}
		}

	case msg.String() == "pgdown":
		if m.githubDetailPR != nil {
			lines := prDetailLines(m.githubDetailPR, m.width)
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
	}
	return m, nil
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

	lines := prDetailLines(d, m.width)
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

	hint := "  esc back   j/k scroll   pgdn/pgup page"
	if m.services.CreateReviewNucleus != nil {
		hint += "   R review"
	}
	statusBar := StyleHelp.Render(hint)
	return lipgloss.JoinVertical(lipgloss.Left, header, meta, body, statusBar)
}

// prDetailLines builds the scrollable content lines for a PR detail.
func prDetailLines(d *github.PRDetail, width int) []string {
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

	for _, f := range d.Files {
		statusColor := prFileStatusColor(f.Status)
		fileLine := lipgloss.NewStyle().Foreground(statusColor).Render(
			fmt.Sprintf("  %s %-*s +%d -%d", prFileStatusChar(f.Status), width-24, truncate(f.Path, width-24), f.Additions, f.Deletions),
		)
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
