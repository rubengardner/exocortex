package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ruben_gardner/exocortex/internal/github"
	"github.com/ruben_gardner/exocortex/internal/jira"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

const previewTickInterval = time.Second

// Services holds the injectable side-effect functions the model calls.
// Populated by cmd/ui.go with real infrastructure; replaced in tests with stubs.
type Services struct {
	LoadNuclei   func() ([]registry.Nucleus, error)
	LoadRepos    func() ([]string, error)          // nil disables the repo picker
	LoadProfiles func() (map[string]string, error) // nil disables the profile picker
	// LoadJiraBoard fetches the kanban board; returns ordered column names alongside the issues map.
	LoadJiraBoard func() (columns []string, issues map[string][]jira.Issue, err error)
	// LoadJiraIssue fetches a single issue's description as Markdown; nil disables the detail view.
	LoadJiraIssue func(key string) (markdown string, err error)
	CapturePane   func(tmuxTarget string) (string, error) // nil disables live preview
	CreateNucleus func(task, repo, branch, profile string) error
	RemoveNucleus func(id string) error
	GotoNucleus   func(id string) error
	GotoNeuron    func(nucleusID, neuronID string) error            // nil falls back to GotoNucleus
	OpenNvim      func(id string) error
	CloseNvim     func(id string) error  // nil disables binding
	RespawnNucleus func(id string) error // nil disables binding
	AddNeuron     func(nucleusID, neuronType, profile string) error // nil disables neuron add
	LoadBranchInfo func(worktreePath string) (modified []string, aheadCommits []string, err error) // nil = no branch stats
	// LoadGitHubPRs fetches open PRs involving the current user; nil disables the GitHub view.
	LoadGitHubPRs func() ([]github.PR, error)
	// LoadGitHubPR fetches full detail for one PR; nil disables the detail view.
	LoadGitHubPR func(repo string, number int) (*github.PRDetail, error)
	// ListBranches returns local branch names for a repo; used in the review workflow.
	ListBranches func(repoPath string) ([]string, error)
	// CreateReviewNucleus creates a nucleus on an existing branch for PR review.
	// nil disables the R key in the GitHub views.
	CreateReviewNucleus func(task, repo, branch, profile string, prNumber int, prRepo string) error
}

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

	// new-nucleus form
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
	paneContent    string // latest capture-pane output for the selected nucleus

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

	// nucleus detail state
	detailNeuronIdx int // selected neuron index within the detail view

	// branch info (loaded async when detail view opens)
	branchModified     []string
	branchAheadCommits []string

	// neuron add state
	neuronAddNucleusID string
	neuronAddCursor    int

	// github PR list state
	githubPRs      []github.PR
	githubPRCursor int
	githubLoading  bool
	githubErr      string

	// github PR detail state
	githubDetailPR      *github.PRDetail
	githubDetailScroll  int
	githubDetailLoading bool

	// review workflow state
	formMode       string // "" (adhoc) or "review"
	reviewPRNumber int
	reviewPRRepo   string
	reviewPRBranch string // head branch of the PR being reviewed

	// branch search state (StateBranchSearch)
	branchSearchBranches []string
	branchSearchFilter   string
	branchSearchCursor   int
	branchSearchLoading  bool

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

// DetailNeuronIdx returns the selected neuron index in StateNucleusDetail.
func (m Model) DetailNeuronIdx() int { return m.detailNeuronIdx }

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
				return m.transitionToFormDest()
			}
			m.state = stateProfileSelect
		}
		return m, nil

	case branchesLoadedMsg:
		m.branchSearchLoading = false
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			m.state = stateList
		} else {
			m.branchSearchBranches = msg.branches
			m.branchSearchCursor = 0
		}
		return m, nil

	case branchInfoLoadedMsg:
		if msg.err == nil {
			m.branchModified = msg.modified
			m.branchAheadCommits = msg.aheadCommits
		}
		return m, nil

	case githubPRsLoadedMsg:
		m.githubLoading = false
		if msg.err != nil {
			m.githubErr = msg.err.Error()
		} else {
			m.githubPRs = msg.prs
			m.githubErr = ""
			if m.githubPRCursor >= len(m.githubPRs) {
				m.githubPRCursor = 0
			}
		}
		return m, nil

	case githubPRDetailLoadedMsg:
		m.githubDetailLoading = false
		if msg.err != nil {
			m.githubErr = msg.err.Error()
			m.state = stateGitHubView
		} else {
			m.githubDetailPR = msg.detail
			m.githubDetailScroll = 0
			m.state = stateGitHubPRDetail
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.captureActivePaneCmd(), m.tickCmd())

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
			return m.updateNucleusList(msg)
		case stateNewOverlay:
			return m.updateNucleusForm(msg)
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
		case stateNucleusDetail:
			return m.updateNucleusDetail(msg)
		case stateNeuronAdd:
			return m.updateNeuronAdd(msg)
		case stateGitHubView:
			return m.updateGitHubView(msg)
		case stateGitHubPRDetail:
			return m.updateGitHubPRDetail(msg)
		case stateBranchSearch:
			return m.updateBranchSearch(msg)
		case stateHelp:
			// Any key dismisses help.
			m.state = stateList
			return m, nil
		}
	}
	return m, nil
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
	case stateNucleusDetail:
		return m.viewNucleusDetailDashboard()
	case stateNeuronAdd:
		return m.renderOverlay(m.viewNucleusDetailDashboard(), m.viewNeuronAdd())
	case stateGitHubView:
		return m.viewGitHubView()
	case stateGitHubPRDetail:
		return m.viewGitHubPRDetail()
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
		case stateBranchSearch:
			return m.renderOverlay(base, m.viewBranchSearch())
		}
		return base
	}
}

func (m Model) viewMain() string {
	listWidth := clamp(m.width/3, 28, 48)
	detailWidth := m.width - listWidth - 1 // -1 for border
	h := m.contentHeight()

	header := m.viewHeader()
	list := clipLines(m.viewNucleusList(listWidth), h)
	detail := clipLines(m.viewNucleusDetail(detailWidth), h)

	listPane := StyleListPane.Height(h).Width(listWidth).Render(list)
	detailPane := StyleDetailPane.Height(h).Width(detailWidth).Render(detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	statusBar := m.viewStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
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

// captureActivePaneCmd captures the relevant pane based on current state:
// in StateNucleusDetail it captures the selected neuron; otherwise the primary neuron.
func (m Model) captureActivePaneCmd() tea.Cmd {
	if m.state == stateNucleusDetail {
		return m.captureDetailPaneCmd()
	}
	return m.capturePaneCmd()
}

// captureDetailPaneCmd captures the tmux pane for the selected neuron in the detail view.
func (m Model) captureDetailPaneCmd() tea.Cmd {
	if m.services.CapturePane == nil || len(m.nuclei) == 0 || !m.previewEnabled {
		return nil
	}
	n := m.nuclei[m.cursor]
	if len(n.Neurons) == 0 || m.detailNeuronIdx >= len(n.Neurons) {
		return nil
	}
	target := n.Neurons[m.detailNeuronIdx].TmuxTarget
	if target == "" {
		return nil
	}
	svc := m.services.CapturePane
	return func() tea.Msg {
		content, err := svc(target)
		return paneCapturedMsg{content: content, err: err}
	}
}

// loadBranchInfoCmd fires an async git status/log fetch for the current nucleus.
func (m Model) loadBranchInfoCmd() tea.Cmd {
	if m.services.LoadBranchInfo == nil || len(m.nuclei) == 0 {
		return nil
	}
	worktreePath := m.nuclei[m.cursor].WorktreePath
	svc := m.services.LoadBranchInfo
	return func() tea.Msg {
		modified, ahead, err := svc(worktreePath)
		return branchInfoLoadedMsg{modified: modified, aheadCommits: ahead, err: err}
	}
}

// loadBranchesCmd fires an async fetch of local branches for the selected repo.
func (m Model) loadBranchesCmd() tea.Cmd {
	if m.services.ListBranches == nil || m.selectedRepo == "" {
		return func() tea.Msg { return branchesLoadedMsg{branches: []string{}} }
	}
	svc := m.services.ListBranches
	repo := m.selectedRepo
	return func() tea.Msg {
		branches, err := svc(repo)
		return branchesLoadedMsg{branches: branches, err: err}
	}
}

// loadGitHubPRsCmd fires an async fetch of PRs from GitHub.
func (m Model) loadGitHubPRsCmd() tea.Cmd {
	if m.services.LoadGitHubPRs == nil {
		return nil
	}
	svc := m.services.LoadGitHubPRs
	return func() tea.Msg {
		prs, err := svc()
		return githubPRsLoadedMsg{prs: prs, err: err}
	}
}

// loadGitHubPRDetailCmd fires an async fetch of a single PR's detail.
func (m Model) loadGitHubPRDetailCmd(repo string, number int) tea.Cmd {
	if m.services.LoadGitHubPR == nil {
		return nil
	}
	svc := m.services.LoadGitHubPR
	return func() tea.Msg {
		detail, err := svc(repo, number)
		return githubPRDetailLoadedMsg{detail: detail, err: err}
	}
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
