package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NucleusModalMode indicates the type of nucleus being created.
type NucleusModalMode string

const (
	ModeDevelop NucleusModalMode = "develop"
	ModeReview  NucleusModalMode = "review"
)

// ModalField identifies a focusable field within the unified nucleus modal.
type ModalField int

const (
	ModalFieldMode     ModalField = iota // mode toggle (develop / review)
	ModalFieldRepo                       // inline repo picker (skipped when ≤1 repo)
	ModalFieldProfile                    // inline profile picker (skipped when no profiles)
	ModalFieldTask                       // task description text input
	ModalFieldBranch                     // branch name (text input) or branch search (list)
	ModalFieldWorktree                   // create-worktree toggle
)

// NucleusModalContext carries pre-filled data when the modal is opened.
type NucleusModalContext struct {
	Mode        NucleusModalMode // "" defaults to ModeDevelop
	JiraKey     string
	JiraSummary string
	PRNumber    int
	PRRepo      string
	PRTitle     string // pre-fills the task description in review mode
	PRBranch    string // pre-fills the branch filter in review mode
}

// ModalRequest signals to the parent model what action to take after an Update.
// At most one field is non-zero per call.
type ModalRequest struct {
	LoadBranches bool         // parent should fire loadBranchesCmd for the selected repo
	Submit       *ModalSubmit // non-nil: user confirmed the form
	Cancel       bool         // user pressed Esc / Ctrl-C
}

// ModalSubmit carries the confirmed form values.
type ModalSubmit struct {
	Mode           NucleusModalMode
	Task           string
	Repo           string
	Branch         string
	Profile        string
	CreateWorktree bool
	JiraKey        string
	PRNumber       int
	PRRepo         string
}

// NucleusModal is a self-contained form widget for creating a nucleus.
// It does not call any services; it communicates with the parent model via
// ModalRequest return values from Update.
type NucleusModal struct {
	mode    NucleusModalMode
	focused ModalField

	taskInput   textinput.Model
	branchInput textinput.Model // develop mode only

	repos         []string
	repoCursor    int
	reposReady    bool // true once SetRepos has been called

	profileNames  []string
	profilePaths  map[string]string
	profileCursor int
	profilesReady bool

	branchList    []string // review mode: all available branches
	branchFilter  string   // review mode: type-to-filter text
	branchCursor  int
	branchesReady bool

	createWorktree bool

	jiraKey  string
	prNumber int
	prRepo   string

	err string

	width int
}

// NewNucleusModal creates an initialised modal with default values.
func NewNucleusModal(width int) NucleusModal {
	task := textinput.New()
	task.Placeholder = "describe the task…"
	task.CharLimit = 120

	branch := textinput.New()
	branch.Placeholder = "branch name (auto-generated if blank)"
	branch.CharLimit = 80

	return NucleusModal{
		mode:           ModeDevelop,
		focused:        ModalFieldMode,
		taskInput:      task,
		branchInput:    branch,
		createWorktree: true,
		width:          width,
	}
}

// Open resets the modal state and pre-fills it from ctx. It returns the updated
// modal and a tea.Cmd to start cursor blinking in the active text input.
func (m NucleusModal) Open(ctx NucleusModalContext) (NucleusModal, tea.Cmd) {
	m.mode = ModeDevelop
	if ctx.Mode == ModeReview {
		m.mode = ModeReview
	}
	m.jiraKey = ctx.JiraKey
	m.prNumber = ctx.PRNumber
	m.prRepo = ctx.PRRepo
	m.err = ""
	m.branchesReady = false
	m.branchList = nil
	m.branchCursor = 0
	m.focused = ModalFieldMode
	m.repoCursor = 0
	m.profileCursor = 0

	// Reset inputs.
	m.taskInput.Reset()
	m.branchInput.Reset()
	m.taskInput.Blur()
	m.branchInput.Blur()

	// Pre-fill from context.
	if ctx.JiraSummary != "" {
		m.taskInput.SetValue(ctx.JiraSummary)
		m.taskInput.CursorEnd()
	}
	if ctx.JiraKey != "" {
		m.branchInput.SetValue("task/" + ctx.JiraKey + "/")
		m.branchInput.CursorEnd()
	}
	if ctx.Mode == ModeReview {
		m.branchFilter = ctx.PRBranch
		m.createWorktree = false
		// Auto-fill task description so the form can submit without extra typing.
		if ctx.PRTitle != "" {
			m.taskInput.SetValue("Review: " + ctx.PRTitle)
			m.taskInput.CursorEnd()
		} else if ctx.PRNumber != 0 {
			m.taskInput.SetValue(fmt.Sprintf("Review PR #%d", ctx.PRNumber))
			m.taskInput.CursorEnd()
		}
	} else {
		m.branchFilter = ""
	}

	return m, textinput.Blink
}

// SetRepos provides the available repositories. When there is exactly one it is
// auto-selected and the repo field is hidden from the form. When prRepo is set,
// the cursor is advanced to the repo whose base name matches the PR's repo name.
func (m NucleusModal) SetRepos(repos []string) NucleusModal {
	m.repos = repos
	m.reposReady = true
	m.repoCursor = 0
	if m.prRepo != "" {
		prShort := m.prRepo
		if idx := strings.LastIndex(m.prRepo, "/"); idx >= 0 {
			prShort = m.prRepo[idx+1:]
		}
		for i, r := range repos {
			if filepath.Base(r) == prShort {
				m.repoCursor = i
				break
			}
		}
	}
	return m
}

// IsReviewMode reports whether the modal is currently in review mode.
func (m NucleusModal) IsReviewMode() bool { return m.mode == ModeReview }

// SetProfiles provides the available Claude profiles.
func (m NucleusModal) SetProfiles(names []string, paths map[string]string) NucleusModal {
	m.profileNames = names
	m.profilePaths = paths
	m.profilesReady = true
	if m.profileCursor >= len(names) {
		m.profileCursor = 0
	}
	return m
}

// SetBranches provides the available branches for review-mode selection. The
// existing branchFilter is preserved so the list is immediately filtered.
func (m NucleusModal) SetBranches(branches []string) NucleusModal {
	m.branchList = branches
	m.branchesReady = true
	m.branchCursor = 0
	return m
}

// SelectedRepo returns the currently selected repository path, or "." when no
// repos are configured.
func (m NucleusModal) SelectedRepo() string {
	if len(m.repos) == 0 {
		return "."
	}
	if m.repoCursor < len(m.repos) {
		return m.repos[m.repoCursor]
	}
	return m.repos[0]
}

// visibleFields returns the ordered list of fields shown in the modal, based on
// how many repos and profiles are available.
func (m NucleusModal) visibleFields() []ModalField {
	fields := []ModalField{ModalFieldMode}
	if len(m.repos) > 1 {
		fields = append(fields, ModalFieldRepo)
	}
	if len(m.profileNames) > 0 {
		fields = append(fields, ModalFieldProfile)
	}
	fields = append(fields, ModalFieldTask, ModalFieldBranch, ModalFieldWorktree)
	return fields
}

// advanceFocus moves focus to the next visible field (wrapping).
func (m NucleusModal) advanceFocus() (NucleusModal, tea.Cmd) {
	return m.moveFocus(1)
}

// retreatFocus moves focus to the previous visible field (wrapping).
func (m NucleusModal) retreatFocus() (NucleusModal, tea.Cmd) {
	return m.moveFocus(-1)
}

func (m NucleusModal) moveFocus(dir int) (NucleusModal, tea.Cmd) {
	fields := m.visibleFields()
	if len(fields) == 0 {
		return m, nil
	}
	cur := 0
	for i, f := range fields {
		if f == m.focused {
			cur = i
			break
		}
	}
	next := ((cur + dir) % len(fields) + len(fields)) % len(fields)
	m = m.blurField()
	m.focused = fields[next]
	return m.focusField()
}

// blurField blurs the currently focused text input (if any).
func (m NucleusModal) blurField() NucleusModal {
	switch m.focused {
	case ModalFieldTask:
		m.taskInput.Blur()
	case ModalFieldBranch:
		if m.mode == ModeDevelop {
			m.branchInput.Blur()
		}
	}
	return m
}

// focusField activates the text input for the currently focused field.
// Returns the updated modal and a blink cmd when a text input is focused.
func (m NucleusModal) focusField() (NucleusModal, tea.Cmd) {
	switch m.focused {
	case ModalFieldTask:
		cmd := m.taskInput.Focus()
		return m, cmd
	case ModalFieldBranch:
		if m.mode == ModeDevelop {
			cmd := m.branchInput.Focus()
			return m, cmd
		}
	}
	return m, nil
}

// selectedProfile returns the display name of the currently selected profile,
// or "" when no profiles are configured.
func (m NucleusModal) selectedProfile() string {
	if len(m.profileNames) == 0 || m.profileCursor >= len(m.profileNames) {
		return ""
	}
	return m.profileNames[m.profileCursor]
}

// filteredBranches returns the subset of branchList matching branchFilter.
func (m NucleusModal) filteredBranches() []string {
	if m.branchFilter == "" {
		return m.branchList
	}
	filter := strings.ToLower(m.branchFilter)
	var out []string
	for _, b := range m.branchList {
		if strings.Contains(strings.ToLower(b), filter) {
			out = append(out, b)
		}
	}
	return out
}

// Update handles a single key event and returns the updated modal, a request for
// the parent, and any cmd produced (e.g. cursor blinking from text inputs).
func (m NucleusModal) Update(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m = m.blurField()
		return m, ModalRequest{Cancel: true}, nil

	case "enter":
		return m.trySubmit()

	case "tab":
		updated, cmd := m.advanceFocus()
		return updated, ModalRequest{}, cmd

	case "shift+tab":
		updated, cmd := m.retreatFocus()
		return updated, ModalRequest{}, cmd
	}

	switch m.focused {
	case ModalFieldMode:
		return m.updateModeField(msg)
	case ModalFieldRepo:
		return m.updateRepoField(msg)
	case ModalFieldProfile:
		return m.updateProfileField(msg)
	case ModalFieldTask:
		return m.updateTaskField(msg)
	case ModalFieldBranch:
		return m.updateBranchField(msg)
	case ModalFieldWorktree:
		return m.updateWorktreeField(msg)
	}

	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateModeField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case " ", "h", "l":
		return m.toggleMode()
	}
	switch msg.Type {
	case tea.KeySpace, tea.KeyLeft, tea.KeyRight:
		return m.toggleMode()
	}
	return m, ModalRequest{}, nil
}

func (m NucleusModal) toggleMode() (NucleusModal, ModalRequest, tea.Cmd) {
	if m.mode == ModeDevelop {
		m.mode = ModeReview
		m.branchCursor = 0
		return m, ModalRequest{LoadBranches: true}, nil
	}
	m.mode = ModeDevelop
	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateRepoField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case "k":
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case "j":
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.repoCursor > 0 {
			m.repoCursor--
		}
	case tea.KeyDown:
		if m.repoCursor < len(m.repos)-1 {
			m.repoCursor++
		}
	}
	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateProfileField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	switch msg.String() {
	case "k":
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case "j":
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	}
	switch msg.Type {
	case tea.KeyUp:
		if m.profileCursor > 0 {
			m.profileCursor--
		}
	case tea.KeyDown:
		if m.profileCursor < len(m.profileNames)-1 {
			m.profileCursor++
		}
	}
	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateTaskField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	var cmd tea.Cmd
	m.taskInput, cmd = m.taskInput.Update(msg)
	return m, ModalRequest{}, cmd
}

func (m NucleusModal) updateBranchField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	if m.mode == ModeDevelop {
		var cmd tea.Cmd
		m.branchInput, cmd = m.branchInput.Update(msg)
		return m, ModalRequest{}, cmd
	}

	// Review mode: typing filters, j/k navigate the list.
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.branchFilter) > 0 {
			m.branchFilter = m.branchFilter[:len(m.branchFilter)-1]
			m.branchCursor = 0
		}
		return m, ModalRequest{}, nil

	case tea.KeyUp:
		if m.branchCursor > 0 {
			m.branchCursor--
		}
		return m, ModalRequest{}, nil

	case tea.KeyDown:
		if m.branchCursor < len(m.filteredBranches())-1 {
			m.branchCursor++
		}
		return m, ModalRequest{}, nil

	case tea.KeyRunes:
		switch msg.String() {
		case "k":
			if m.branchCursor > 0 {
				m.branchCursor--
			}
		case "j":
			if m.branchCursor < len(m.filteredBranches())-1 {
				m.branchCursor++
			}
		default:
			m.branchFilter += string(msg.Runes)
			m.branchCursor = 0
		}
		return m, ModalRequest{}, nil
	}

	return m, ModalRequest{}, nil
}

func (m NucleusModal) updateWorktreeField(msg tea.KeyMsg) (NucleusModal, ModalRequest, tea.Cmd) {
	if msg.Type == tea.KeySpace || msg.String() == " " {
		m.createWorktree = !m.createWorktree
	}
	return m, ModalRequest{}, nil
}

// trySubmit validates the form and returns a Submit ModalRequest on success.
func (m NucleusModal) trySubmit() (NucleusModal, ModalRequest, tea.Cmd) {
	task := strings.TrimSpace(m.taskInput.Value())
	if task == "" {
		m.err = "task description is required"
		m = m.blurField()
		m.focused = ModalFieldTask
		updated, cmd := m.focusField()
		return updated, ModalRequest{}, cmd
	}

	var branch string
	if m.mode == ModeDevelop {
		branch = strings.TrimSpace(m.branchInput.Value())
	} else {
		filtered := m.filteredBranches()
		switch {
		case len(filtered) > 0:
			if m.branchCursor >= len(filtered) {
				m.branchCursor = 0
			}
			branch = filtered[m.branchCursor]
		case m.branchFilter != "":
			// Branch pre-filled from PR context — use it directly even if the
			// async branch list hasn't loaded yet or doesn't contain a match.
			branch = m.branchFilter
		default:
			m.err = "select a branch to review"
			m.focused = ModalFieldBranch
			return m, ModalRequest{}, nil
		}
	}

	return m, ModalRequest{Submit: &ModalSubmit{
		Mode:           m.mode,
		Task:           task,
		Repo:           m.SelectedRepo(),
		Branch:         branch,
		Profile:        m.selectedProfile(),
		CreateWorktree: m.createWorktree,
		JiraKey:        m.jiraKey,
		PRNumber:       m.prNumber,
		PRRepo:         m.prRepo,
	}}, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the modal content. The parent wraps this in renderOverlay.
func (m NucleusModal) View() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("New Nucleus") + "\n\n")

	m.renderModeField(&sb)
	sb.WriteString("\n\n")

	if len(m.repos) > 1 {
		m.renderRepoField(&sb)
		sb.WriteString("\n\n")
	}

	if len(m.profileNames) > 0 {
		m.renderProfileField(&sb)
		sb.WriteString("\n\n")
	}

	if m.mode == ModeReview && m.prNumber != 0 {
		sb.WriteString(StyleLabel.Render("PR") +
			StyleValue.Render(fmt.Sprintf("#%d  %s", m.prNumber, m.prRepo)) + "\n\n")
	}

	if m.jiraKey != "" {
		sb.WriteString(StyleLabel.Render("Jira") + StyleValue.Render(m.jiraKey) + "\n\n")
	}

	m.renderTaskField(&sb)
	sb.WriteString("\n\n")

	m.renderBranchField(&sb)
	sb.WriteString("\n\n")

	m.renderWorktreeField(&sb)

	if m.err != "" {
		sb.WriteString("\n\n")
		sb.WriteString(StyleError.Render(m.err))
	}

	sb.WriteString("\n\n")
	sb.WriteString(
		StyleDim.Render("tab") + " next   " +
			StyleDim.Render("shift+tab") + " prev   " +
			StyleDim.Render("enter") + " create   " +
			StyleDim.Render("esc") + " cancel",
	)

	return sb.String()
}

// fieldLabel renders the label for a field, highlighted when focused.
func (m NucleusModal) fieldLabel(field ModalField, text string) string {
	if m.focused == field {
		return lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(text)
	}
	return StyleLabel.Render(text)
}

func (m NucleusModal) renderModeField(sb *strings.Builder) {
	develop := "Develop"
	review := "Review"
	if m.mode == ModeDevelop {
		develop = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("[" + develop + "]")
		review = StyleDim.Render(review)
	} else {
		develop = StyleDim.Render(develop)
		review = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("[" + review + "]")
	}
	focusHint := ""
	if m.focused == ModalFieldMode {
		focusHint = StyleDim.Render("  space/←/→")
	}
	sb.WriteString(m.fieldLabel(ModalFieldMode, "Mode") + "  " + develop + "  " + review + focusHint)
}

func (m NucleusModal) renderRepoField(sb *strings.Builder) {
	sb.WriteString(m.fieldLabel(ModalFieldRepo, "Repo") + "\n")
	const maxShow = 4
	for i, r := range m.repos {
		if i >= maxShow {
			sb.WriteString(StyleDim.Render(fmt.Sprintf("  … %d more", len(m.repos)-maxShow)) + "\n")
			break
		}
		base := filepath.Base(r)
		if i == m.repoCursor {
			row := "  > " + truncate(base, 30)
			if m.focused == ModalFieldRepo {
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render(row) + "\n")
			}
		} else {
			sb.WriteString("    " + truncate(base, 30) + "\n")
		}
	}
}

func (m NucleusModal) renderProfileField(sb *strings.Builder) {
	sb.WriteString(m.fieldLabel(ModalFieldProfile, "Profile") + "\n")
	for i, name := range m.profileNames {
		path := ""
		if m.profilePaths != nil {
			path = m.profilePaths[name]
		}
		if i == m.profileCursor {
			row := "  > " + truncate(name, 22) + "  " + path
			if m.focused == ModalFieldProfile {
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Render(row) + "\n")
			}
		} else {
			sb.WriteString("    " + truncate(name, 22) + "  " + StyleDim.Render(path) + "\n")
		}
	}
}

func (m NucleusModal) renderTaskField(sb *strings.Builder) {
	sb.WriteString(m.fieldLabel(ModalFieldTask, "Task") + "\n")
	sb.WriteString(m.taskInput.View())
}

func (m NucleusModal) renderBranchField(sb *strings.Builder) {
	// branchContentLines is the fixed number of content lines after the label.
	// Review mode max = 1 filter line + 5 branch rows. Develop mode pads to match.
	const branchContentLines = 6

	sb.WriteString(m.fieldLabel(ModalFieldBranch, "Branch") + "\n")

	linesUsed := 0
	if m.mode == ModeDevelop {
		sb.WriteString(m.branchInput.View())
		linesUsed = 1
	} else {
		// Review mode: filter text + branch list.
		sb.WriteString(StyleDim.Render("Filter: ") + m.branchFilter + "█\n")
		linesUsed++
		if !m.branchesReady {
			sb.WriteString(StyleDim.Render("  loading branches…"))
			linesUsed++
		} else {
			filtered := m.filteredBranches()
			if len(filtered) == 0 {
				sb.WriteString(StyleDim.Render("  no matching branches"))
				linesUsed++
			} else {
				const maxShow = 5
				for i, b := range filtered {
					if i >= maxShow {
						sb.WriteString(StyleDim.Render(fmt.Sprintf("  … %d more", len(filtered)-maxShow)))
						linesUsed++
						break
					}
					if i == m.branchCursor {
						sb.WriteString(StyleSelected.Render("  > " + truncate(b, 50)))
					} else {
						sb.WriteString("    " + truncate(b, 50))
					}
					linesUsed++
					if i < len(filtered)-1 && i < maxShow-1 {
						sb.WriteString("\n")
					}
				}
			}
		}
	}

	// Pad with empty lines to reach fixed height so the modal doesn't resize on mode switch.
	for linesUsed < branchContentLines {
		sb.WriteString("\n")
		linesUsed++
	}
}

func (m NucleusModal) renderWorktreeField(sb *strings.Builder) {
	check := "[ ]"
	if m.createWorktree {
		check = "[✓]"
	}
	checkStr := check
	if m.focused == ModalFieldWorktree {
		checkStr = lipgloss.NewStyle().Foreground(ColorAccent).Render(check)
	}
	hint := ""
	if m.focused == ModalFieldWorktree {
		hint = StyleDim.Render("  space to toggle")
	}
	sb.WriteString(m.fieldLabel(ModalFieldWorktree, "Worktree") + "  " + checkStr + " create git worktree" + hint)
}
