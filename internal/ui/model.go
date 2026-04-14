package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/jira"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

const previewTickInterval = time.Second

// ViewState is the active UI layer. Exported so tests can inspect it.
type ViewState int

const (
	StateList          ViewState = iota
	StateNewOverlay              // new-agent form overlay
	StateConfirmDelete           // delete confirmation dialog
	StateHelp                    // full-page help
	StateRepoSelect              // repo picker shown before the new-agent form
	StateProfileSelect           // profile picker shown after repo select, before form
	StateJiraBoard               // live Jira kanban view
	StateJiraDetail              // single-issue description overlay
)

// viewState is an internal alias for ViewState.
type viewState = ViewState

// internal aliases for use inside this file.
const (
	stateList          = StateList
	stateNewOverlay    = StateNewOverlay
	stateConfirmDelete = StateConfirmDelete
	stateHelp          = StateHelp
	stateRepoSelect    = StateRepoSelect
	stateProfileSelect = StateProfileSelect
	stateJiraBoard     = StateJiraBoard
	stateJiraDetail    = StateJiraDetail
)

// Services holds the injectable side-effect functions the model calls.
// Populated by cmd/ui.go with real infrastructure; replaced in tests with stubs.
type Services struct {
	LoadNuclei   func() ([]registry.Nucleus, error)
	LoadRepos    func() ([]string, error)          // nil disables the repo picker
	LoadProfiles func() (map[string]string, error) // nil disables the profile picker
	// LoadJiraBoard fetches the kanban board. Returns ordered column names alongside the issues map.
	LoadJiraBoard func() (columns []string, issues map[string][]jira.Issue, err error)
	// LoadJiraIssue fetches a single issue's description as Markdown. nil disables the detail view.
	LoadJiraIssue func(key string) (markdown string, err error)
	CapturePane   func(tmuxTarget string) (string, error) // nil disables live preview
	CreateNucleus func(task, repo, branch, profile string) error
	RemoveNucleus func(id string) error
	GotoNucleus   func(id string) error
	OpenNvim      func(id string) error
	CloseNvim     func(id string) error // nil disables binding
	RespawnNucleus func(id string) error // reopen tmux window; nil disables binding
}

// --- messages ----------------------------------------------------------------

type nucleiLoadedMsg struct {
	nuclei []registry.Nucleus
	err    error
}

type reposLoadedMsg struct {
	repos []string
	err   error
}

type profilesLoadedMsg struct {
	names []string          // sorted
	paths map[string]string // name → path
	err   error
}

type actionDoneMsg struct {
	err       error
	quitAfter bool // quit the TUI on success (used by Goto)
}

type tickMsg struct{}

type paneCapturedMsg struct {
	content string
	err     error
}

type jiraBoardLoadedMsg struct {
	columns []string
	issues  map[string][]jira.Issue
	err     error
}

type jiraIssueLoadedMsg struct {
	key      string
	title    string
	markdown string
	err      error
}

// --- Model -------------------------------------------------------------------

// Model is the root Bubble Tea model.
type Model struct {
	services Services
	keys     KeyMap
	help     help.Model
	nuclei   []registry.Nucleus

	cursor int
	state  viewState
	width  int
	height int

	// new-agent form
	formTask    textinput.Model
	formBranch  textinput.Model
	formFocused int // 0=task, 1=branch
	formErr     string

	// confirm-delete
	confirmID string

	// repo picker state
	repos        []string
	repoCursor   int
	selectedRepo string // set when user picks a repo, or "." when no picker

	// profile picker state
	profileNames    []string          // sorted display names
	profilePaths    map[string]string // name → CLAUDE_CONFIG_DIR path
	profileCursor   int
	selectedProfile string // profile name chosen by user, or "" when no picker

	// live pane preview
	previewEnabled bool   // global toggle; default true
	paneContent    string // latest capture-pane output for the selected agent

	// jira board
	jiraColumns     []string // ordered status names (from config)
	jiraIssues      map[string][]jira.Issue
	jiraColIdx      int   // focused column (0–numCols-1)
	jiraRowIdx      int   // focused row within column
	jiraScrollOffs  []int // per-column scroll offset (top visible row index)
	jiraLoading     bool
	jiraErr         string
	jiraLastRefresh time.Time

	// jira issue detail
	jiraDetailKey     string // issue key being shown ("" = closed)
	jiraDetailTitle   string // "KEY — Summary"
	jiraDetailMD      string // raw markdown (ADF-converted)
	jiraDetailScroll  int    // top visible line index
	jiraDetailLoading bool

	// transient status bar message
	lastErr string
}

// New returns an initialised Model.
func New(svc Services) Model {
	task := textinput.New()
	task.Placeholder = "describe the task…"
	task.CharLimit = 120
	task.Focus()

	branch := textinput.New()
	branch.Placeholder = "branch name (optional, auto-generated if blank)"
	branch.CharLimit = 80

	h := help.New()
	h.ShowAll = false

	return Model{
		services:       svc,
		keys:           DefaultKeys(),
		help:           h,
		formTask:       task,
		formBranch:     branch,
		previewEnabled: true,
	}
}

// LastErr returns the last transient error message, if any.
func (m Model) LastErr() string { return m.lastErr }

// Cursor returns the index of the currently selected nucleus.
func (m Model) Cursor() int { return m.cursor }

// State returns the current view state.
func (m Model) State() ViewState { return m.state }

// Init issues the first data load. The preview tick starts automatically
// after the first successful nucleus load so it doesn't interfere with tests.
func (m Model) Init() tea.Cmd {
	return m.loadNucleiCmd()
}

// --- Update ------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case nucleiLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			return m, nil
		}
		firstLoad := len(m.nuclei) == 0 && len(msg.nuclei) > 0
		m.nuclei = msg.nuclei
		if m.cursor >= len(m.nuclei) && len(m.nuclei) > 0 {
			m.cursor = len(m.nuclei) - 1
		}
		// Kick off the preview tick once, on first successful load.
		// The tick is self-perpetuating — it reschedules itself each fire.
		if firstLoad {
			return m, tea.Batch(m.tickCmd(), m.capturePaneCmd())
		}
		return m, nil

	case reposLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.state = stateList
		} else {
			m.repos = msg.repos
			if len(m.repos) == 0 {
				// Config present but no repos listed — skip picker.
				m.selectedRepo = "."
				return m.transitionAfterRepo()
			}
		}
		return m, nil

	case profilesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.state = stateList
		} else {
			m.profileNames = msg.names
			m.profilePaths = msg.paths
			if len(m.profileNames) == 0 {
				// No profiles configured — skip picker.
				m.selectedProfile = ""
				m.state = stateNewOverlay
				return m, textinput.Blink
			}
			m.state = stateProfileSelect
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.capturePaneCmd(), m.tickCmd())

	case paneCapturedMsg:
		if msg.err == nil {
			m.paneContent = msg.content
		}
		return m, nil

	case jiraIssueLoadedMsg:
		m.jiraDetailLoading = false
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.state = stateJiraBoard
		} else {
			m.jiraDetailKey = msg.key
			m.jiraDetailTitle = msg.title
			m.jiraDetailMD = msg.markdown
			m.jiraDetailScroll = 0
			m.state = stateJiraDetail
		}
		return m, nil

	case jiraBoardLoadedMsg:
		m.jiraLoading = false
		if msg.err != nil {
			m.jiraErr = msg.err.Error()
		} else {
			m.jiraColumns = msg.columns
			m.jiraIssues = msg.issues
			m.jiraLastRefresh = time.Now()
			m.jiraErr = ""
			// Clamp cursor in case column count changed.
			if m.jiraColIdx >= len(m.jiraColumns) {
				m.jiraColIdx = 0
			}
			// Resize or initialise per-column scroll offsets.
			if len(m.jiraScrollOffs) != len(m.jiraColumns) {
				m.jiraScrollOffs = make([]int, len(m.jiraColumns))
			}
		}
		return m, nil

	case actionDoneMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			return m, m.loadNucleiCmd()
		}
		if msg.quitAfter {
			return m, tea.Quit
		}
		return m, m.loadNucleiCmd()

	case tea.KeyMsg:
		// Clear transient error on any keypress.
		m.lastErr = ""

		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateNewOverlay:
			return m.updateNewForm(msg)
		case stateConfirmDelete:
			return m.updateConfirm(msg)
		case stateRepoSelect:
			return m.updateRepoSelect(msg)
		case stateProfileSelect:
			return m.updateProfileSelect(msg)
		case stateJiraBoard:
			return m.updateJiraBoard(msg)
		case stateJiraDetail:
			return m.updateJiraDetail(msg)
		case stateHelp:
			// Any key dismisses help.
			m.state = stateList
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Quit):
		return m, tea.Quit

	case matchKey(msg, m.keys.Help):
		m.state = stateHelp
		return m, nil

	case matchKey(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
			m.paneContent = ""
		}
		return m, m.capturePaneCmd()

	case matchKey(msg, m.keys.Down):
		if m.cursor < len(m.nuclei)-1 {
			m.cursor++
			m.paneContent = ""
		}
		return m, m.capturePaneCmd()

	case matchKey(msg, m.keys.Refresh):
		return m, m.loadNucleiCmd()

	case matchKey(msg, m.keys.New):
		m.formTask.Reset()
		m.formBranch.Reset()
		m.formTask.Focus()
		m.formBranch.Blur()
		m.formFocused = 0
		m.formErr = ""
		if m.services.LoadRepos != nil {
			m.repos = nil
			m.repoCursor = 0
			m.state = stateRepoSelect
			return m, m.loadReposCmd()
		}
		m.selectedRepo = "."
		return m.transitionAfterRepo()

	case matchKey(msg, m.keys.Delete):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		m.confirmID = m.nuclei[m.cursor].ID
		m.state = stateConfirmDelete
		return m, nil

	case matchKey(msg, m.keys.Goto):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.GotoNucleus
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.Nvim):
		if len(m.nuclei) == 0 {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.OpenNvim
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.CloseNvim):
		if len(m.nuclei) == 0 || m.services.CloseNvim == nil {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.CloseNvim
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.Respawn):
		if len(m.nuclei) == 0 || m.services.RespawnNucleus == nil {
			return m, nil
		}
		id := m.nuclei[m.cursor].ID
		svc := m.services.RespawnNucleus
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}

	case matchKey(msg, m.keys.TogglePreview):
		m.previewEnabled = !m.previewEnabled
		if m.previewEnabled {
			m.paneContent = ""
			return m, m.capturePaneCmd()
		}
		return m, nil

	case matchKey(msg, m.keys.Board):
		m.state = stateJiraBoard
		if m.jiraIssues == nil && !m.jiraLoading {
			m.jiraLoading = true
			return m, m.loadJiraBoardCmd()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateNewForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, m.keys.Confirm):
		id := m.confirmID
		svc := m.services.RemoveNucleus
		m.state = stateList
		return m, func() tea.Msg {
			return actionDoneMsg{err: svc(id)}
		}
	default:
		// esc, n, or any other key cancels
		m.state = stateList
		return m, nil
	}
}

// --- View --------------------------------------------------------------------

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}

	switch m.state {
	case stateHelp:
		return m.viewHelp()
	case stateJiraBoard:
		return m.viewJiraBoard()
	case stateJiraDetail:
		return m.viewJiraDetail()
	default:
		base := m.viewMain()
		switch m.state {
		case stateNewOverlay:
			return m.renderOverlay(base, m.viewNewForm())
		case stateConfirmDelete:
			return m.renderOverlay(base, m.viewConfirm())
		case stateRepoSelect:
			return m.renderOverlay(base, m.viewRepoSelect())
		case stateProfileSelect:
			return m.renderOverlay(base, m.viewProfileSelect())
		}
		return base
	}
}

func (m Model) viewMain() string {
	listWidth := clamp(m.width/3, 28, 48)
	detailWidth := m.width - listWidth - 1 // -1 for border
	h := m.contentHeight()

	header := m.viewHeader()
	// Clip both panels to exactly h lines before handing to lipgloss so that
	// variable-length content (e.g. long pane captures) can never push the
	// header or status bar off screen.
	list := clipLines(m.viewList(listWidth), h)
	detail := clipLines(m.viewDetail(detailWidth), h)

	// Join panels side by side. The list panel has a right border.
	listPane := StyleListPane.Height(h).Width(listWidth).Render(list)
	detailPane := StyleDetailPane.Height(h).Width(detailWidth).Render(detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	statusBar := m.viewStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

func (m Model) viewHeader() string {
	count := fmt.Sprintf("%d nucleus(i)", len(m.nuclei))
	left := StyleHeader.Render("◈  EXOCORTEX")
	right := StyleMuted.Render(count)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) viewList(width int) string {
	if len(m.nuclei) == 0 {
		return StyleDim.Render("  no nuclei yet\n  press n to create one")
	}
	var sb strings.Builder
	for i, n := range m.nuclei {
		dot := StatusDot(n.Status)
		task := truncate(n.TaskDescription, width-14)
		age := fmtAge(n.CreatedAt)

		line1 := fmt.Sprintf(" %s %-*s", dot, width-10, task)
		line2 := StyleDim.Render(fmt.Sprintf("   %-*s %s", width-12, n.ID, age))

		row := line1 + "\n" + line2
		if i == m.cursor {
			row = StyleSelected.Width(width).Render(line1) + "\n" +
				StyleSelected.Width(width).Foreground(ColorDim).Render("  "+n.ID+" "+age)
		}
		sb.WriteString(row)
		if i < len(m.nuclei)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (m Model) viewDetail(width int) string {
	if len(m.nuclei) == 0 {
		return ""
	}
	n := m.nuclei[m.cursor]

	var sb strings.Builder

	// Compact header: id, task, branch, status.
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

	headerLines := 7 // title + divider + id + branch + status + claude + nvim

	// Live preview section.
	previewHeaderLines := 2 // blank + separator
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
		// Strip trailing spaces from each line (capture-pane pads to pane width).
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
		}
		// Trim trailing empty lines.
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		// Show only the last N lines that fit.
		if len(lines) > previewLines {
			lines = lines[len(lines)-previewLines:]
		}
		for _, l := range lines {
			sb.WriteString(truncate(l, width-2) + "\n")
		}
	}

	return sb.String()
}

func (m Model) viewStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	return StyleHelp.Render(m.help.View(m.keys))
}

func (m Model) viewHelp() string {
	m.help.ShowAll = true
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		StyleOverlay.Render(
			StyleTitle.Render("Keyboard Shortcuts")+"\n\n"+
				m.help.View(m.keys),
		),
	)
}

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

func (m Model) viewConfirm() string {
	id := m.confirmID
	return StyleTitle.Render("Remove nucleus?") + "\n\n" +
		StyleValue.Render("Nucleus ") + StyleError.Render(id) + StyleValue.Render(" will be removed.\n") +
		StyleValue.Render("All tmux panes and the git worktree will be deleted.\n\n") +
		StyleMuted.Render("y") + "  confirm   " +
		StyleMuted.Render("any other key") + " cancel"
}

func (m Model) renderOverlay(base, content string) string {
	box := StyleOverlay.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("")))
}

// --- helpers -----------------------------------------------------------------

func (m Model) contentHeight() int {
	// header (2 lines) + status bar (1 line)
	return clamp(m.height-3, 1, m.height)
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(previewTickInterval, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m Model) capturePaneCmd() tea.Cmd {
	if m.services.CapturePane == nil || len(m.nuclei) == 0 || !m.previewEnabled {
		return nil
	}
	n := m.nuclei[m.cursor]
	primary := n.PrimaryNeuron()
	if primary == nil {
		return nil
	}
	target := primary.TmuxTarget
	svc := m.services.CapturePane
	return func() tea.Msg {
		content, err := svc(target)
		return paneCapturedMsg{content: content, err: err}
	}
}

func (m Model) loadNucleiCmd() tea.Cmd {
	svc := m.services.LoadNuclei
	return func() tea.Msg {
		nuclei, err := svc()
		return nucleiLoadedMsg{nuclei: nuclei, err: err}
	}
}

func (m Model) loadReposCmd() tea.Cmd {
	svc := m.services.LoadRepos
	return func() tea.Msg {
		repos, err := svc()
		return reposLoadedMsg{repos: repos, err: err}
	}
}

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
		// Sort for stable display order.
		for i := 1; i < len(names); i++ {
			for j := i; j > 0 && names[j] < names[j-1]; j-- {
				names[j], names[j-1] = names[j-1], names[j]
			}
		}
		return profilesLoadedMsg{names: names, paths: paths}
	}
}

// transitionAfterRepo advances past the repo picker.
// If profiles are configured, opens the profile picker; otherwise opens the form directly.
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

func (m Model) viewProfileSelect() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Select Profile") + "\n\n")

	if len(m.profileNames) == 0 {
		sb.WriteString(StyleDim.Render("  loading…") + "\n\n")
	} else {
		for i, name := range m.profileNames {
			path := m.profilePaths[name]
			if i == m.profileCursor {
				row := fmt.Sprintf("  > %-22s  %s", name, path)
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(fmt.Sprintf("    %-22s  %s\n", name, StyleDim.Render(path)))
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
				row := fmt.Sprintf("  > %-22s  %s", base, parent)
				sb.WriteString(StyleSelected.Render(row) + "\n")
			} else {
				sb.WriteString(fmt.Sprintf("    %-22s  %s\n", base, StyleDim.Render(parent)))
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

func (m Model) loadJiraIssueCmd(key, summary string) tea.Cmd {
	svc := m.services.LoadJiraIssue
	return func() tea.Msg {
		md, err := svc(key)
		return jiraIssueLoadedMsg{key: key, title: key + " — " + summary, markdown: md, err: err}
	}
}

func (m Model) updateJiraDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Total rendered lines (approximated from raw markdown line count; exact
	// count is computed in viewJiraDetail via glamour).
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
	}
	return m, nil
}

func (m Model) viewJiraDetail() string {
	renderWidth := m.width - 4
	if renderWidth < 20 {
		renderWidth = 20
	}

	// Render markdown with glamour.
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
	if len(lines) > contentH {
		scrollInfo = StyleDim.Render(fmt.Sprintf(" %d%%", 100*m.jiraDetailScroll/maxScroll))
	}
	divider := StyleDim.Render(strings.Repeat("─", m.width-2))
	header := title + scrollInfo + "\n" + divider + "\n"

	// Status bar.
	statusBar := StyleHelp.Render("  esc back   j/k scroll   pgdn/pgup page")

	body := visible
	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

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

func (m Model) updateJiraBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.jiraDetailLoading = true
		return m, m.loadJiraIssueCmd(issue.Key, issue.Summary)
	}
	return m.jiraAdjustScroll(), nil
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

func (m Model) viewJiraBoard() string {
	header := m.viewHeader()
	body := m.viewJiraBoardBody()
	statusBar := m.viewJiraBoardStatusBar()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

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

	// Build separator: h lines of "│" so it spans the full body height.
	sepLines := make([]string, h)
	for i := range sepLines {
		sepLines[i] = "│"
	}
	sep := StyleDim.Render(strings.Join(sepLines, "\n"))

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

func (m Model) renderJiraColumn(colIdx int, status string, width, height int) string {
	allIssues := m.jiraIssues[status]

	var sb strings.Builder

	// Column header.
	title := fmt.Sprintf(" %s (%d)", strings.ToUpper(status), len(allIssues))
	if colIdx == m.jiraColIdx {
		sb.WriteString(StyleTitle.Render(truncate(title, width-2)) + "\n")
	} else {
		sb.WriteString(StyleDim.Render(truncate(title, width-2)) + "\n")
	}
	sb.WriteString(StyleDim.Render(" "+strings.Repeat("─", clamp(width-4, 4, 60))) + "\n")

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

	// Issue rows (3 lines each: key, summary, assignee; blank line between issues).
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
			sb.WriteString("\n")
		}
	}

	return clipLines(sb.String(), height)
}

func (m Model) viewJiraBoardStatusBar() string {
	if m.lastErr != "" {
		return StyleError.Render(" ✗ " + m.lastErr)
	}
	if m.jiraLoading {
		return StyleHelp.Render("  refreshing…")
	}
	hint := "  b/esc back   j/k row   h/l column   r refresh"
	if !m.jiraLastRefresh.IsZero() {
		return StyleHelp.Render(fmt.Sprintf("  updated %s ·%s", fmtAge(m.jiraLastRefresh), hint))
	}
	return StyleHelp.Render(hint)
}

// matchKey returns true if the tea.KeyMsg matches the given binding.
func matchKey(msg tea.KeyMsg, b interface{ Keys() []string }) bool {
	for _, bound := range b.Keys() {
		if msg.String() == bound {
			return true
		}
	}
	return false
}

// clipLines returns at most max lines of s, discarding any excess.
// This keeps panel content from overflowing its allocated height.
func clipLines(s string, max int) string {
	if max <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > max {
		lines = lines[:max]
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func fmtAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
