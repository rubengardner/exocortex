package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateNucleusForm handles key events for the new-nucleus form overlay.
func (m Model) updateNucleusForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case matchKey(msg, m.keys.Cancel):
		returnState := m.formCancelState()
		m.pendingJiraKey = ""
		m.pendingJiraSummary = ""
		m.state = returnState
		return m, nil

	case matchKey(msg, m.keys.NextField):
		if m.formFocused == 0 {
			m.formFocused = 1
			m.formTask.Blur()
			cmds = append(cmds, m.formBranch.Focus())
		} else {
			m.formFocused = 0
			m.formBranch.Blur()
			cmds = append(cmds, m.formTask.Focus())
		}
		return m, tea.Batch(cmds...)

	case matchKey(msg, m.keys.Submit):
		task := strings.TrimSpace(m.formTask.Value())
		if task == "" {
			m.formErr = "task is required"
			return m, nil
		}
		branch := strings.TrimSpace(m.formBranch.Value())
		svc := m.services.CreateNucleus
		repo := m.selectedRepo
		profile := m.selectedProfile
		jiraKey := m.pendingJiraKey
		m.pendingJiraKey = ""
		m.pendingJiraSummary = ""
		m.state = stateList
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(task, repo, branch, profile, jiraKey)}
		}
	}

	// Route keypresses to the focused input.
	var cmd tea.Cmd
	if m.formFocused == 0 {
		m.formTask, cmd = m.formTask.Update(msg)
	} else {
		m.formBranch, cmd = m.formBranch.Update(msg)
	}
	return m, cmd
}

// viewNewForm renders the new-nucleus form overlay content.
func (m Model) viewNewForm() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("New Nucleus") + "\n\n")
	if m.pendingJiraKey != "" {
		sb.WriteString(StyleLabel.Render("Jira") + StyleValue.Render(m.pendingJiraKey) + "\n\n")
	}
	if m.selectedProfile != "" {
		profilePath := m.profilePaths[m.selectedProfile]
		sb.WriteString(StyleLabel.Render("Profile") + StyleValue.Render(m.selectedProfile))
		if profilePath != "" {
			sb.WriteString(StyleDim.Render("  " + profilePath))
		}
		sb.WriteString("\n\n")
	}
	sb.WriteString(StyleLabel.Render("Task") + "\n")
	sb.WriteString(m.formTask.View() + "\n\n")
	sb.WriteString(StyleLabel.Render("Branch") + "\n")
	sb.WriteString(m.formBranch.View() + "\n\n")
	if m.formErr != "" {
		sb.WriteString(StyleError.Render(m.formErr) + "\n\n")
	}
	sb.WriteString(StyleDim.Render("tab") + " switch field   " +
		StyleDim.Render("enter") + " create   " +
		StyleDim.Render("esc") + " cancel")
	return sb.String()
}

// updateRepoSelect handles key events for the repo-picker overlay.
func (m Model) updateRepoSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		returnState := m.formCancelState()
		m.pendingJiraKey = ""
		m.pendingJiraSummary = ""
		m.state = returnState
		return m, nil
	case matchKey(msg, m.keys.Up):
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case matchKey(msg, m.keys.Down):
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	case matchKey(msg, m.keys.Submit):
		m.selectedRepo = m.repos[m.repoCursor]
		return m.transitionAfterRepo()
	}
	return m, nil
}

// viewRepoSelect renders the repo-picker overlay content.
func (m Model) viewRepoSelect() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Select Repository") + "\n\n")

	if len(m.repos) == 0 {
		sb.WriteString(StyleDim.Render("  loading…") + "\n\n")
	} else {
		for i, r := range m.repos {
			base := filepath.Base(r)
			parent := filepath.Dir(r)
			if i == m.repoCursor {
				row := "  > " + truncate(base, 22) + "  " + parent
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString("    " + truncate(base, 22) + "  " + StyleDim.Render(parent) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}

// updateProfileSelect handles key events for the profile-picker overlay.
func (m Model) updateProfileSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		returnState := m.formCancelState()
		m.pendingJiraKey = ""
		m.pendingJiraSummary = ""
		m.state = returnState
		return m, nil
	case matchKey(msg, m.keys.Up):
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case matchKey(msg, m.keys.Down):
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	case matchKey(msg, m.keys.Submit):
		if len(m.profileNames) > 0 {
			m.selectedProfile = m.profileNames[m.profileCursor]
		}
		return m.transitionToFormDest()
	}
	return m, nil
}

// viewProfileSelect renders the profile-picker overlay content.
func (m Model) viewProfileSelect() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Select Profile") + "\n\n")

	if len(m.profileNames) == 0 {
		sb.WriteString(StyleDim.Render("  loading…") + "\n\n")
	} else {
		for i, name := range m.profileNames {
			path := m.profilePaths[name]
			if i == m.profileCursor {
				row := "  > " + truncate(name, 22) + "  " + path
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString("    " + truncate(name, 22) + "  " + StyleDim.Render(path) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}

// transitionAfterRepo advances past the repo picker.
// If profiles are configured, opens the profile picker; otherwise goes to the form destination.
func (m Model) transitionAfterRepo() (Model, tea.Cmd) {
	if m.services.LoadProfiles != nil {
		m.profileNames = nil
		m.profileCursor = 0
		// State will be set to stateProfileSelect when profilesLoadedMsg arrives.
		return m, m.loadProfilesCmd()
	}
	m.selectedProfile = ""
	return m.transitionToFormDest()
}

// transitionToFormDest routes to the correct form state based on formMode.
// In "review" mode it opens the branch search overlay; otherwise the new-nucleus form.
func (m Model) transitionToFormDest() (Model, tea.Cmd) {
	if m.formMode == "review" {
		m.state = stateBranchSearch
		m.branchSearchBranches = nil
		m.branchSearchFilter = m.reviewPRBranch
		m.branchSearchCursor = 0
		m.branchSearchLoading = true
		return m, m.loadBranchesCmd()
	}
	return m.openNucleusForm()
}

// formCancelState returns the state to return to when the user cancels any
// step of the new-nucleus creation flow. When a Jira issue is pending, the
// user is returned to the Jira board instead of the main list.
func (m Model) formCancelState() viewState {
	if m.pendingJiraKey != "" {
		return stateJiraBoard
	}
	return stateList
}

// startReviewWorkflow stores the PR context and begins the repo → profile → branch-search flow.
func (m Model) startReviewWorkflow(prNumber int, prRepo, prBranch string) (tea.Model, tea.Cmd) {
	m.formMode = "review"
	m.reviewPRNumber = prNumber
	m.reviewPRRepo = prRepo
	m.reviewPRBranch = prBranch
	if m.services.LoadRepos != nil {
		m.repos = nil
		m.repoCursor = 0
		m.state = stateRepoSelect
		return m, m.loadReposCmd()
	}
	m.selectedRepo = "."
	return m.transitionAfterRepo()
}

// loadReposCmd fires an async repo-list fetch.
func (m Model) loadReposCmd() tea.Cmd {
	svc := m.services.LoadRepos
	return func() tea.Msg {
		repos, err := svc()
		return reposLoadedMsg{repos: repos, err: err}
	}
}

// ── StateBranchSearch ─────────────────────────────────────────────────────────

// updateBranchSearch handles key events for the branch-search overlay.
func (m Model) updateBranchSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Cancel):
		m.state = stateList
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.branchSearchCursor > 0 {
			m.branchSearchCursor--
		}

	case matchKey(msg, m.keys.Down):
		if m.branchSearchCursor < len(m.filteredBranches())-1 {
			m.branchSearchCursor++
		}

	case matchKey(msg, m.keys.Submit):
		filtered := m.filteredBranches()
		if len(filtered) == 0 {
			return m, nil
		}
		return m.submitReviewNucleus(filtered[m.branchSearchCursor])

	case msg.Type == tea.KeyBackspace:
		if len(m.branchSearchFilter) > 0 {
			m.branchSearchFilter = m.branchSearchFilter[:len(m.branchSearchFilter)-1]
			m.branchSearchCursor = 0
		}

	default:
		if msg.Type == tea.KeyRunes {
			m.branchSearchFilter += string(msg.Runes)
			m.branchSearchCursor = 0
		}
	}
	return m, nil
}

// viewBranchSearch renders the branch-search overlay content.
func (m Model) viewBranchSearch() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Select Branch") + "\n\n")

	if m.reviewPRNumber != 0 {
		sb.WriteString(
			StyleLabel.Render("PR") +
				StyleValue.Render(fmt.Sprintf("#%d  %s", m.reviewPRNumber, m.reviewPRRepo)) +
				"\n\n",
		)
	}

	sb.WriteString(StyleLabel.Render("Filter") + m.branchSearchFilter + "█\n\n")

	if m.branchSearchLoading {
		sb.WriteString(StyleDim.Render("  loading branches…") + "\n")
	} else {
		filtered := m.filteredBranches()
		if len(filtered) == 0 {
			sb.WriteString(StyleDim.Render("  no matching branches") + "\n")
		} else {
			for i, b := range filtered {
				if i == m.branchSearchCursor {
					sb.WriteString(StyleSelected.Render("  > "+truncate(b, 50)) + "\n")
				} else {
					sb.WriteString("    " + truncate(b, 50) + "\n")
				}
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(
		StyleDim.Render("↑/k") + " up   " +
			StyleDim.Render("↓/j") + " down   " +
			StyleDim.Render("enter") + " select   " +
			StyleDim.Render("esc") + " cancel",
	)
	return sb.String()
}

// filteredBranches returns the subset of branchSearchBranches matching branchSearchFilter.
func (m Model) filteredBranches() []string {
	if m.branchSearchFilter == "" {
		return m.branchSearchBranches
	}
	filter := strings.ToLower(m.branchSearchFilter)
	var out []string
	for _, b := range m.branchSearchBranches {
		if strings.Contains(strings.ToLower(b), filter) {
			out = append(out, b)
		}
	}
	return out
}

// submitReviewNucleus fires CreateReviewNucleus for the given branch.
func (m Model) submitReviewNucleus(branch string) (tea.Model, tea.Cmd) {
	svc := m.services.CreateReviewNucleus
	if svc == nil {
		m.lastErr = "review nucleus creation not configured"
		m.state = stateList
		return m, nil
	}
	prNumber := m.reviewPRNumber
	prRepo := m.reviewPRRepo
	repo := m.selectedRepo
	profile := m.selectedProfile
	task := fmt.Sprintf("Review PR #%d", prNumber)
	if prNumber == 0 {
		task = "Review " + branch
	}
	m.state = stateList
	return m, func() tea.Msg {
		return actionDoneMsg{err: svc(task, repo, branch, profile, prNumber, prRepo)}
	}
}

// loadProfilesCmd fires an async profile-list fetch.
func (m Model) loadProfilesCmd() tea.Cmd {
	svc := m.services.LoadProfiles
	return func() tea.Msg {
		paths, err := svc()
		if err != nil {
			return profilesLoadedMsg{err: err}
		}
		names := make([]string, 0, len(paths))
		for name := range paths {
			names = append(names, name)
		}
		// Sort for stable display order (insertion sort; small N).
		for i := 1; i < len(names); i++ {
			for j := i; j > 0 && names[j] < names[j-1]; j-- {
				names[j], names[j-1] = names[j-1], names[j]
			}
		}
		return profilesLoadedMsg{names: names, paths: paths}
	}
}
