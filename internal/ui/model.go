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
	// LoadJiraIssueMeta fetches lightweight issue metadata for the Nucleus detail panel; nil disables it.
	LoadJiraIssueMeta func(key string) (*jira.Issue, error)
	CapturePane       func(tmuxTarget string) (string, error) // nil disables live preview
	// CreateNucleus creates a new development nucleus. createWorktree=false skips git worktree creation.
	CreateNucleus func(task, repo, branch, profile, jiraKey string, createWorktree bool) error
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
	// createWorktree=false skips git worktree creation.
	// nil disables the review workflow.
	CreateReviewNucleus func(task, repo, branch, profile string, prNumber int, prRepo string, createWorktree bool) error
	// OpenNvimFile opens a specific file at a given line in the nucleus's nvim window.
	// nucleusID is found by matching PRNumber/PRRepo; nil disables the binding.
	OpenNvimFile func(nucleusID, filePath string, line int) error
	// BrowserOpen opens the given URL in the system browser (e.g. xdg-open).
	// nil disables the binding.
	BrowserOpen func(url string) error
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

	// unified nucleus-creation modal
	nucleusModal NucleusModal
	prevState    viewState // state to restore when modal is cancelled

	// confirm-delete
	confirmID string

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
	jiraDetailURL     string // web URL for "open in browser" (from issue.URL)
	jiraDetailScroll  int    // top visible line index
	jiraDetailLoading bool

	// Jira metadata for the nucleus detail middle panel.
	detailJiraIssue   *jira.Issue // nil until loaded (or if no JiraKey)
	detailJiraLoading bool

	// GitHub PR metadata for the nucleus detail middle panel.
	detailPRDetail  *github.PRDetail // nil until loaded (or if no PRNumber)
	detailPRLoading bool

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

	// github PR hover preview (right panel in stateGitHubView)
	githubPreviewPR      *github.PRDetail // nil until loaded
	githubPreviewLoading bool
	githubPreviewNum     int // PR number of the in-flight / last preview load

	// github PR detail state (right panel in stateGitHubPRDetail)
	githubDetailPR         *github.PRDetail
	githubDetailScroll     int
	githubDetailLoading    bool
	githubDetailFileCursor int    // selected file index within d.Files
	githubFileExpanded     []bool // per-file accordion expansion state

	// transient status bar message
	lastErr string
}

// New returns an initialised Model.
func New(svc Services) Model {
	h := help.New()
	h.ShowAll = false

	return Model{
		services:       svc,
		keys:           DefaultKeys(),
		help:           h,
		nucleusModal:   NewNucleusModal(80),
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
		m.nucleusModal.width = msg.Width
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
			if m.state == stateNucleusModal {
				m.state = m.prevState
			}
		} else {
			repos := msg.repos
			if len(repos) == 0 {
				repos = []string{"."}
			}
			m.nucleusModal = m.nucleusModal.SetRepos(repos)
		}
		return m, nil

	case profilesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.nucleusModal = m.nucleusModal.SetProfiles(msg.names, msg.paths)
		}
		return m, nil

	case branchesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.nucleusModal = m.nucleusModal.SetBranches(msg.branches)
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
			// Kick off hover preview for the first PR.
			if len(msg.prs) > 0 && m.services.LoadGitHubPR != nil {
				pr := msg.prs[0]
				m.githubPreviewNum = pr.Number
				m.githubPreviewLoading = true
				m.githubPreviewPR = nil
				return m, m.loadGitHubPRPreviewCmd(pr.Repo, pr.Number)
			}
		}
		return m, nil

	case githubPRPreviewLoadedMsg:
		m.githubPreviewLoading = false
		if msg.err == nil && msg.number == m.githubPreviewNum {
			m.githubPreviewPR = msg.detail
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
			m.githubDetailFileCursor = 0
			m.githubFileExpanded = make([]bool, len(msg.detail.Files))
			m.state = stateGitHubPRDetail
		}
		return m, nil

	case githubPRMetaLoadedMsg:
		m.detailPRLoading = false
		if msg.err == nil {
			m.detailPRDetail = msg.detail
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

	case jiraIssueMetaLoadedMsg:
		m.detailJiraLoading = false
		if msg.err == nil {
			m.detailJiraIssue = msg.issue
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
		case stateNucleusModal:
			return m.updateNucleusModal(msg)
		case stateConfirmDelete:
			return m.updateConfirm(msg)
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
		case stateNucleusModal:
			return m.renderOverlay(base, m.nucleusModal.View())
		case stateConfirmDelete:
			return m.renderOverlay(base, m.viewConfirm())
		}
		return base
	}
}

// updateNucleusModal routes key events into the unified modal and acts on the
// resulting ModalRequest.
func (m Model) updateNucleusModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modal, req, cmd := m.nucleusModal.Update(msg)
	m.nucleusModal = modal

	if req.Cancel {
		m.state = m.prevState
		return m, nil
	}

	if req.LoadBranches {
		return m, tea.Batch(cmd, m.loadBranchesForModalCmd())
	}

	if req.Submit != nil {
		sub := req.Submit
		m.state = stateList
		if sub.Mode == ModeReview {
			svc := m.services.CreateReviewNucleus
			if svc == nil {
				m.lastErr = "review nucleus creation not configured"
				m.state = m.prevState
				return m, cmd
			}
			return m, tea.Batch(cmd, func() tea.Msg {
				return actionDoneMsg{err: svc(sub.Task, sub.Repo, sub.Branch, sub.Profile, sub.PRNumber, sub.PRRepo, sub.CreateWorktree)}
			})
		}
		svc := m.services.CreateNucleus
		return m, tea.Batch(cmd, func() tea.Msg {
			return actionDoneMsg{err: svc(sub.Task, sub.Repo, sub.Branch, sub.Profile, sub.JiraKey, sub.CreateWorktree)}
		})
	}

	return m, cmd
}

// openNucleusModal transitions to stateNucleusModal, saves the current state
// so it can be restored on cancel, and fires async data loads.
func (m Model) openNucleusModal(ctx NucleusModalContext) (Model, tea.Cmd) {
	m.prevState = m.state

	modal, initCmd := m.nucleusModal.Open(ctx)
	m.nucleusModal = modal
	m.state = stateNucleusModal

	var cmds []tea.Cmd
	cmds = append(cmds, initCmd)

	if m.services.LoadRepos != nil {
		cmds = append(cmds, m.loadReposCmd())
	} else {
		m.nucleusModal = m.nucleusModal.SetRepos([]string{"."})
	}

	if m.services.LoadProfiles != nil {
		cmds = append(cmds, m.loadProfilesCmd())
	} else {
		m.nucleusModal = m.nucleusModal.SetProfiles(nil, nil)
	}

	// For review mode, load branches immediately (repo is already known as ".").
	if ctx.Mode == ModeReview && m.services.LoadRepos == nil {
		cmds = append(cmds, m.loadBranchesForModalCmd())
	}

	return m, tea.Batch(cmds...)
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

// loadBranchesForModalCmd fires an async fetch of local branches for the repo
// currently selected in the modal.
func (m Model) loadBranchesForModalCmd() tea.Cmd {
	repo := m.nucleusModal.SelectedRepo()
	if m.services.ListBranches == nil || repo == "" {
		return func() tea.Msg { return branchesLoadedMsg{branches: []string{}} }
	}
	svc := m.services.ListBranches
	return func() tea.Msg {
		branches, err := svc(repo)
		return branchesLoadedMsg{branches: branches, err: err}
	}
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
		// Sort for stable display order.
		for i := 1; i < len(names); i++ {
			for j := i; j > 0 && names[j] < names[j-1]; j-- {
				names[j], names[j-1] = names[j-1], names[j]
			}
		}
		return profilesLoadedMsg{names: names, paths: paths}
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

// loadGitHubPRPreviewCmd fires an async fetch for the hover-preview panel.
func (m Model) loadGitHubPRPreviewCmd(repo string, number int) tea.Cmd {
	if m.services.LoadGitHubPR == nil {
		return nil
	}
	svc := m.services.LoadGitHubPR
	return func() tea.Msg {
		detail, err := svc(repo, number)
		return githubPRPreviewLoadedMsg{number: number, detail: detail, err: err}
	}
}

// loadGitHubPRMetaCmd fires an async fetch of PR metadata for the nucleus
// detail panel.
func (m Model) loadGitHubPRMetaCmd() tea.Cmd {
	if m.services.LoadGitHubPR == nil || len(m.nuclei) == 0 {
		return nil
	}
	n := m.nuclei[m.cursor]
	if n.PRNumber == 0 || n.PRRepo == "" {
		return nil
	}
	svc := m.services.LoadGitHubPR
	repo := n.PRRepo
	number := n.PRNumber
	return func() tea.Msg {
		detail, err := svc(repo, number)
		return githubPRMetaLoadedMsg{detail: detail, err: err}
	}
}

// loadJiraIssueMetaCmd fires an async fetch of issue metadata for the nucleus
// detail panel.
func (m Model) loadJiraIssueMetaCmd() tea.Cmd {
	if m.services.LoadJiraIssueMeta == nil || len(m.nuclei) == 0 {
		return nil
	}
	key := m.nuclei[m.cursor].JiraKey
	if key == "" {
		return nil
	}
	svc := m.services.LoadJiraIssueMeta
	return func() tea.Msg {
		issue, err := svc(key)
		return jiraIssueMetaLoadedMsg{issue: issue, err: err}
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

// textinput is imported only for the Blink command used in openNucleusModal.
var _ = textinput.Blink
