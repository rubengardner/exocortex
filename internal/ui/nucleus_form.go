package ui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// updateNucleusForm handles key events for the new-nucleus form overlay.
func (m Model) updateNucleusForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch {
	case matchKey(msg, m.keys.Cancel):
		m.state = stateList
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
		m.state = stateList
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(task, repo, branch, profile)}
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
		m.state = stateList
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
		m.state = stateList
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
		m.state = stateNewOverlay
		return m, textinput.Blink
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
// If profiles are configured, opens the profile picker; otherwise opens the form.
func (m Model) transitionAfterRepo() (Model, tea.Cmd) {
	if m.services.LoadProfiles != nil {
		m.profileNames = nil
		m.profileCursor = 0
		// State will be set to stateProfileSelect when profilesLoadedMsg arrives.
		return m, m.loadProfilesCmd()
	}
	m.selectedProfile = ""
	m.state = stateNewOverlay
	return m, textinput.Blink
}

// loadReposCmd fires an async repo-list fetch.
func (m Model) loadReposCmd() tea.Cmd {
	svc := m.services.LoadRepos
	return func() tea.Msg {
		repos, err := svc()
		return reposLoadedMsg{repos: repos, err: err}
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
