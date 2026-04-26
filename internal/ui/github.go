package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/github"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// ── filterItem ────────────────────────────────────────────────────────────────

// filterItem is a single row in the GitHub filter modal.
type filterItem struct {
	label    string // display text
	value    string // author login, "!me", "owner/repo", or "" for headers
	kind     string // "author" | "repo" | "header" — headers are non-selectable
	selected bool
}

// ── StateGitHubView ──────────────────────────────────────────────────────────

// updateGitHubView handles key events for the GitHub PR list.
func (m Model) updateGitHubView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.githubClaudeWorktreePick {
		return m.updateGitHubClaudeWorktreePicker(msg)
	}
	if m.githubProfilePick {
		return m.updateGitHubProfilePicker(msg)
	}
	if m.githubNucleusPick {
		return m.updateGitHubNucleusPicker(msg)
	}
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

	case matchKey(msg, m.keys.New):
		if len(m.githubPRs) == 0 || m.services.AddNvimNeuronFromPR == nil {
			return m, nil
		}
		pr := m.githubPRs[m.githubPRCursor]
		return m.startGitHubNvimNeuronPick(pr)

	case msg.String() == "c":
		if len(m.githubPRs) == 0 || m.services.AddClaudeNeuronFromPR == nil {
			return m, nil
		}
		pr := m.githubPRs[m.githubPRCursor]
		return m.startGitHubClaudeNeuronPick(pr)

	case matchKey(msg, m.keys.Filter):
		if m.services.LoadGitHubFilterConfig == nil {
			return m, nil
		}
		return m, m.loadGitHubFilterConfigCmd()
	}
	return m, nil
}

// startGitHubNvimNeuronPick opens the nucleus picker for adding an nvim neuron from a PR.
func (m Model) startGitHubNvimNeuronPick(pr github.PR) (tea.Model, tea.Cmd) {
	m.githubPickerPendingPR = pr
	m.githubNucleusPick = true
	m.githubNucleusPickMode = "add_nvim"
	m.githubPickerFilter = ""
	m.githubPickerCursor = 0
	m.githubPickerNuclei = m.nuclei
	return m, nil
}

// updateGitHubProfilePicker handles key events for the inline profile picker overlay.
func (m Model) updateGitHubProfilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.githubProfilePick = false
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubProfileCursor > 0 {
			m.githubProfileCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.githubProfileCursor < len(m.githubProfileNames)-1 {
			m.githubProfileCursor++
		}

	case matchKey(msg, m.keys.Submit):
		profile := m.githubProfileNames[m.githubProfileCursor]
		m.githubProfilePick = false
		return m.startClaudeNeuronWorktreePick(m.githubClaudePickNucleusID, profile)
	}
	return m, nil
}

// viewGitHubProfilePicker renders the profile selection overlay content.
func (m Model) viewGitHubProfilePicker() string {
	var sb strings.Builder
	pr := m.githubPickerPendingPR
	title := pr.Title
	if title == "" {
		title = fmt.Sprintf("PR #%d", pr.Number)
	}
	sb.WriteString(StyleTitle.Render("Review PR") + "\n")
	sb.WriteString(StyleDim.Render(truncate(title, 40)) + "\n\n")
	sb.WriteString(StyleLabel.Render("Profile") + "\n")
	for i, name := range m.githubProfileNames {
		if i == m.githubProfileCursor {
			sb.WriteString(StyleSelected.Render("  ▶ "+name) + "\n")
		} else {
			sb.WriteString("    " + name + "\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(StyleDim.Render("↑/k") + " up   " +
		StyleDim.Render("↓/j") + " down   " +
		StyleDim.Render("enter") + " select   " +
		StyleDim.Render("esc") + " cancel")
	return sb.String()
}

// startGitHubClaudeNeuronPick opens the nucleus picker for adding a Claude neuron from a PR.
func (m Model) startGitHubClaudeNeuronPick(pr github.PR) (tea.Model, tea.Cmd) {
	m.githubPickerPendingPR = pr
	m.githubNucleusPick = true
	m.githubNucleusPickMode = "add_claude"
	m.githubPickerFilter = ""
	m.githubPickerCursor = 0
	m.githubPickerNuclei = m.nuclei
	return m, nil
}

// startClaudeNeuronForNucleus moves to profile selection (if needed) then worktree selection.
// githubPickerPendingPR must already be set before calling this.
func (m Model) startClaudeNeuronForNucleus(nucleusID string) (tea.Model, tea.Cmd) {
	m.githubWorktreeMode = "add_claude"
	switch len(m.githubProfileNames) {
	case 0:
		return m.startClaudeNeuronWorktreePick(nucleusID, "")
	case 1:
		return m.startClaudeNeuronWorktreePick(nucleusID, m.githubProfileNames[0])
	default:
		m.githubClaudePickNucleusID = nucleusID
		m.githubProfilePickMode = "add_claude"
		m.githubProfilePick = true
		m.githubProfileCursor = 0
		return m, nil
	}
}

// startClaudeNeuronWorktreePick shows a 2-item picker: no worktree (default) vs new worktree.
// githubPickerPendingPR must already be set before calling this.
func (m Model) startClaudeNeuronWorktreePick(nucleusID, profile string) (tea.Model, tea.Cmd) {
	m.githubClaudePickNucleusID = nucleusID
	m.githubClaudePickProfile = profile
	m.githubClaudeWorktreePick = true
	m.githubClaudeWorktreeCursor = 0 // default: no worktree
	return m, nil
}

// updateGitHubClaudeWorktreePicker handles key events for the worktree choice overlay.
func (m Model) updateGitHubClaudeWorktreePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.githubClaudeWorktreePick = false
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.githubClaudeWorktreeCursor > 0 {
			m.githubClaudeWorktreeCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.githubClaudeWorktreeCursor < 1 {
			m.githubClaudeWorktreeCursor++
		}

	case matchKey(msg, m.keys.Submit):
		createWorktree := m.githubClaudeWorktreeCursor == 1
		m.githubClaudeWorktreePick = false
		nucleusID := m.githubClaudePickNucleusID
		pr := m.githubPickerPendingPR
		if m.githubWorktreeMode == "add_nvim" {
			svc := m.services.AddNvimNeuronFromPR
			return m, func() tea.Msg {
				return actionDoneMsg{err: svc(nucleusID, pr.Repo, pr.Branch, createWorktree)}
			}
		}
		profile := m.githubClaudePickProfile
		svc := m.services.AddClaudeNeuronFromPR
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(nucleusID, pr.Repo, pr.Branch, profile, createWorktree)}
		}
	}
	return m, nil
}

// viewGitHubClaudeWorktreePicker renders the worktree selection overlay.
func (m Model) viewGitHubClaudeWorktreePicker() string {
	var sb strings.Builder
	pr := m.githubPickerPendingPR
	title := pr.Title
	if title == "" {
		title = fmt.Sprintf("PR #%d", pr.Number)
	}
	heading := "Add Claude Neuron"
	if m.githubWorktreeMode == "add_nvim" {
		heading = "Add Nvim Neuron"
	}
	sb.WriteString(StyleTitle.Render(heading) + "\n")
	sb.WriteString(StyleDim.Render(truncate(title, 40)) + "\n\n")
	sb.WriteString(StyleLabel.Render("Checkout mode") + "\n")

	options := []string{
		"existing repo  (no branch switch, reuse sibling worktree if available)",
		"new worktree   (isolated checkout on PR branch)",
	}
	for i, opt := range options {
		if i == m.githubClaudeWorktreeCursor {
			sb.WriteString(StyleSelected.Render("  ▶ "+opt) + "\n")
		} else {
			sb.WriteString("    " + opt + "\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(StyleDim.Render("↑/k") + " up   " +
		StyleDim.Render("↓/j") + " down   " +
		StyleDim.Render("enter") + " confirm   " +
		StyleDim.Render("esc") + " cancel")
	return sb.String()
}

// updateGitHubNucleusPicker handles key events for the nucleus selection overlay.
func (m Model) updateGitHubNucleusPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := githubFilteredNuclei(m.githubPickerNuclei, m.githubPickerFilter)

	switch {
	case matchKey(msg, m.keys.Cancel):
		m.githubNucleusPick = false
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
		m.githubNucleusPick = false
		m.githubPickerFilter = ""
		if m.githubNucleusPickMode == "add_claude" {
			return m.startClaudeNeuronForNucleus(nucleus.ID)
		}
		// "add_nvim": go directly to worktree picker (no profile step)
		m.githubWorktreeMode = "add_nvim"
		return m.startClaudeNeuronWorktreePick(nucleus.ID, "")

	default:
		if msg.Type == tea.KeyRunes {
			m.githubPickerFilter += string(msg.Runes)
			m.githubPickerCursor = 0
		}
	}
	return m, nil
}

// viewGitHubNucleusPicker renders the nucleus selection overlay content.
func (m Model) viewGitHubNucleusPicker() string {
	var sb strings.Builder
	pr := m.githubPickerPendingPR
	title := pr.Title
	if title == "" {
		title = fmt.Sprintf("PR #%d", pr.Number)
	}
	heading := "Add Nvim Neuron to Nucleus"
	if m.githubNucleusPickMode == "add_claude" {
		heading = "Add Claude Neuron to Nucleus"
	}
	sb.WriteString(StyleTitle.Render(heading) + "\n")
	sb.WriteString(StyleDim.Render(truncate(title, 40)) + "\n\n")

	filtered := githubFilteredNuclei(m.githubPickerNuclei, m.githubPickerFilter)

	sb.WriteString(StyleLabel.Render("Filter: ") + m.githubPickerFilter + "█\n\n")

	if len(filtered) == 0 {
		sb.WriteString(StyleDim.Render("  (no nuclei match)") + "\n")
	} else {
		for i, n := range filtered {
			desc := truncate(n.TaskDescription, 32)
			row := fmt.Sprintf("  %-8s  %s", n.ID, desc)
			if i == m.githubPickerCursor {
				sb.WriteString(StyleSelected.Render("▶ "+row) + "\n")
			} else {
				sb.WriteString("  " + row + "\n")
			}
		}
	}
	sb.WriteString("\n")
	action := "add nvim"
	if m.githubNucleusPickMode == "add_claude" {
		action = "add claude"
	}
	sb.WriteString(StyleDim.Render("type") + " filter   " +
		StyleDim.Render("↑/k") + " up   " +
		StyleDim.Render("↓/j") + " down   " +
		StyleDim.Render("enter") + " " + action + "   " +
		StyleDim.Render("esc") + " cancel")
	return sb.String()
}

// githubFilteredNuclei returns nuclei whose ID or task description contains filter (case-insensitive).
func githubFilteredNuclei(nuclei []registry.Nucleus, filter string) []registry.Nucleus {
	if filter == "" {
		return nuclei
	}
	low := strings.ToLower(filter)
	var result []registry.Nucleus
	for _, n := range nuclei {
		if strings.Contains(strings.ToLower(n.ID), low) ||
			strings.Contains(strings.ToLower(n.TaskDescription), low) {
			result = append(result, n)
		}
	}
	return result
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
	if m.githubClaudeWorktreePick {
		return m.updateGitHubClaudeWorktreePicker(msg)
	}
	if m.githubProfilePick {
		return m.updateGitHubProfilePicker(msg)
	}
	if m.githubNucleusPick {
		return m.updateGitHubNucleusPicker(msg)
	}
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

	case matchKey(msg, m.keys.New):
		if m.githubDetailPR == nil || m.services.AddNvimNeuronFromPR == nil {
			return m, nil
		}
		pr := github.PR{
			Number: m.githubDetailPR.Number,
			Repo:   m.githubDetailPR.Repo,
			Title:  m.githubDetailPR.Title,
			Branch: m.githubDetailPR.Branch,
		}
		return m.startGitHubNvimNeuronPick(pr)

	case msg.String() == "c":
		if m.githubDetailPR == nil || m.services.AddClaudeNeuronFromPR == nil {
			return m, nil
		}
		pr := github.PR{
			Number: m.githubDetailPR.Number,
			Repo:   m.githubDetailPR.Repo,
			Title:  m.githubDetailPR.Title,
			Branch: m.githubDetailPR.Branch,
		}
		return m.startGitHubClaudeNeuronPick(pr)

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
	indicator := m.viewGitHubFilterIndicator()
	var label string
	if indicator != "" {
		label = fmt.Sprintf("%d PR(s)  %s", len(m.githubPRs), StyleMuted.Render(indicator))
	} else {
		label = fmt.Sprintf("%d PR(s)", len(m.githubPRs))
	}
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
		repoShort := pr.Repo
		if idx := strings.LastIndex(pr.Repo, "/"); idx >= 0 {
			repoShort = pr.Repo[idx+1:]
		}
		branchStr := truncate(pr.Branch, listW-16)

		line1 := fmt.Sprintf(" %s %-7s %s", dot, numStr, title)
		line2 := "   " + repoShort + "  " + branchStr + "  " + age

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
	if m.services.AddNvimNeuronFromPR != nil {
		hint += "   n nvim"
	}
	if m.services.AddClaudeNeuronFromPR != nil {
		hint += "   c claude"
	}
	if m.services.BrowserOpen != nil {
		hint += "   o browser"
	}
	hint += "   r refresh"
	if m.services.LoadGitHubFilterConfig != nil {
		hint += "   f filter"
	}
	return StyleHelp.Render(hint)
}

func (m Model) viewGitHubDetailStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	hint := "  esc back   j/k file   space expand   pgdn/pgup scroll"
	if m.services.AddNvimNeuronFromPR != nil {
		hint += "   n nvim"
	}
	if m.services.AddClaudeNeuronFromPR != nil {
		hint += "   c claude"
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
		for _, linked := range n.PullRequests {
			if linked.Number == pr.Number && linked.Repo == pr.Repo {
				return n.ID
			}
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

// ── StateGitHubFilter (filter modal) ─────────────────────────────────────────

// buildFilterItems constructs the flat item list for the filter modal from the
// static config values and the currently committed filter (to pre-tick selections).
func buildFilterItems(myLogin string, teammates, repoNames []string, committed github.PRFilter) []filterItem {
	authorSet := make(map[string]bool)
	for _, a := range committed.Authors {
		authorSet[a] = true
	}
	repoSet := make(map[string]bool)
	for _, r := range committed.Repos {
		repoSet[r] = true
	}

	var items []filterItem

	// AUTHORS section — only when myLogin is known or teammates are configured.
	if myLogin != "" || len(teammates) > 0 {
		items = append(items, filterItem{label: "AUTHORS", kind: "header"})
		if myLogin != "" {
			items = append(items, filterItem{
				label:    "me  (your own PRs)",
				value:    myLogin,
				kind:     "author",
				selected: authorSet[myLogin],
			})
			items = append(items, filterItem{
				label:    "others  (everyone but you)",
				value:    "!me",
				kind:     "author",
				selected: authorSet["!me"],
			})
		}
		for _, t := range teammates {
			items = append(items, filterItem{
				label:    t,
				value:    t,
				kind:     "author",
				selected: authorSet[t],
			})
		}
	}

	// REPOSITORIES section — only when repo names are available.
	if len(repoNames) > 0 {
		items = append(items, filterItem{label: "REPOSITORIES", kind: "header"})
		for _, r := range repoNames {
			items = append(items, filterItem{
				label:    r,
				value:    r,
				kind:     "repo",
				selected: repoSet[r],
			})
		}
	}

	return items
}

// firstSelectableIdx returns the index of the first non-header item, or 0.
func firstSelectableIdx(items []filterItem) int {
	for i, it := range items {
		if it.kind != "header" {
			return i
		}
	}
	return 0
}

// collectFilter builds a PRFilter from the selected items in the modal.
// "!me" sentinel is preserved as-is; the service layer handles expansion.
func collectFilter(items []filterItem) github.PRFilter {
	var f github.PRFilter
	for _, it := range items {
		if !it.selected {
			continue
		}
		switch it.kind {
		case "author":
			f.Authors = append(f.Authors, it.value)
		case "repo":
			f.Repos = append(f.Repos, it.value)
		}
	}
	return f
}

// updateGitHubFilter handles key events for the filter modal.
func (m Model) updateGitHubFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel), matchKey(msg, m.keys.Quit):
		m.state = stateGitHubView
		return m, nil

	case matchKey(msg, m.keys.Up):
		m.githubFilterCursor = prevSelectableIdx(m.githubFilterItems, m.githubFilterCursor)

	case matchKey(msg, m.keys.Down):
		m.githubFilterCursor = nextSelectableIdx(m.githubFilterItems, m.githubFilterCursor)

	case msg.Type == tea.KeySpace:
		if m.githubFilterCursor < len(m.githubFilterItems) {
			it := &m.githubFilterItems[m.githubFilterCursor]
			if it.kind != "header" {
				it.selected = !it.selected
			}
		}

	case matchKey(msg, m.keys.Submit):
		f := collectFilter(m.githubFilterItems)
		return m, func() tea.Msg { return githubFilterConfirmedMsg{filter: f} }

	case msg.String() == "c":
		for i := range m.githubFilterItems {
			m.githubFilterItems[i].selected = false
		}
	}
	return m, nil
}

// nextSelectableIdx returns the index of the next non-header item after cur,
// wrapping around. Returns cur if there are no selectable items.
func nextSelectableIdx(items []filterItem, cur int) int {
	n := len(items)
	for i := 1; i <= n; i++ {
		idx := (cur + i) % n
		if items[idx].kind != "header" {
			return idx
		}
	}
	return cur
}

// prevSelectableIdx returns the index of the previous non-header item before cur,
// wrapping around. Returns cur if there are no selectable items.
func prevSelectableIdx(items []filterItem, cur int) int {
	n := len(items)
	for i := 1; i <= n; i++ {
		idx := (cur - i + n) % n
		if items[idx].kind != "header" {
			return idx
		}
	}
	return cur
}

// viewGitHubFilter renders the filter modal content.
func (m Model) viewGitHubFilter() string {
	if len(m.githubFilterItems) == 0 {
		return StyleDim.Render("  filter not configured")
	}

	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Filter PRs") + "\n\n")

	for i, it := range m.githubFilterItems {
		switch it.kind {
		case "header":
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(StyleDim.Render("  "+it.label) + "\n")
			sb.WriteString(StyleDim.Render("  "+strings.Repeat("─", clamp(len(it.label)+2, 4, 36))) + "\n")
		default:
			check := "[ ]"
			if it.selected {
				check = "[x]"
			}
			row := fmt.Sprintf("  %s %s", check, it.label)
			if i == m.githubFilterCursor {
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(row + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(StyleHelp.Render("  space toggle · enter confirm · esc cancel · c clear"))

	return sb.String()
}

// viewGitHubFilterIndicator returns a short "[filtered: ...]" label or "" when
// the active filter is zero. Used in the PR list header.
func (m Model) viewGitHubFilterIndicator() string {
	f := m.githubFilter
	if f.IsZero() {
		return ""
	}
	var parts []string
	if len(f.Authors) > 0 {
		parts = append(parts, fmt.Sprintf("%d author(s)", len(f.Authors)))
	}
	if len(f.Repos) > 0 {
		parts = append(parts, fmt.Sprintf("%d repo(s)", len(f.Repos)))
	}
	return "[filtered: " + strings.Join(parts, " · ") + "]"
}


