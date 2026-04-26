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
	// CreateNucleus creates an empty nucleus (no neurons). Neurons are added later via AddNeuron.
	// profile is the CLAUDE_CONFIG_DIR path (from nucleus-level profile selection).
	CreateNucleus func(task, jiraKey, profile string) error
	RemoveNucleus func(id string) error
	RemoveNeuron  func(nucleusID, neuronID string) error // nil disables neuron delete
	GotoNucleus   func(id string) error
	GotoNeuron    func(nucleusID, neuronID string) error            // nil falls back to GotoNucleus
	OpenNvim      func(id string) error
	CloseNvim     func(id string) error  // nil disables binding
	RespawnNucleus func(id string) error // nil disables binding
	AddNeuron     func(nucleusID, neuronType, repoPath, branch, baseBranch string, createWorktree, createBranch bool) error // nil disables neuron add
	AddPullRequest func(nucleusID string, pr registry.PullRequest) error    // nil disables PR add
	LoadBranchInfo func(worktreePath string) (modified []string, aheadCommits []string, err error) // nil = no branch stats
	// LoadGitHubPRs fetches open PRs matching the given filter; nil disables the GitHub view.
	LoadGitHubPRs func(filter github.PRFilter) ([]github.PR, error)
	// LoadGitHubFilterConfig returns the static data needed to populate the filter modal.
	// Returns (myLogin, teammates, repoNames). nil disables the filter modal.
	LoadGitHubFilterConfig func() (myLogin string, teammates []string, repoNames []string, err error)
	// LoadGitHubPR fetches full detail for one PR; nil disables the detail view.
	LoadGitHubPR func(repo string, number int) (*github.PRDetail, error)
	// ListBranches returns local branch names for a repo; used in the review workflow.
	ListBranches func(repoPath string) ([]string, error)
	// BaseBranchesForRepo returns the configured base branches for a repo path.
	// Returns nil when the repo is not configured or has no base branches set.
	BaseBranchesForRepo func(repoPath string) []string
	// AddNvimNeuronFromPR appends an nvim neuron to an existing nucleus on the PR branch.
	// Mirrors AddClaudeNeuronFromPR but launches nvim. nil disables the "n" binding.
	AddNvimNeuronFromPR func(nucleusID, repo, branch string, createWorktree bool) error
	// OpenNvimFile opens a specific file at a given line in the nucleus's nvim window.
	// nucleusID is found by matching PRNumber/PRRepo; nil disables the binding.
	OpenNvimFile func(nucleusID, filePath string, line int) error
	// BrowserOpen opens the given URL in the system browser (e.g. xdg-open).
	// nil disables the binding.
	BrowserOpen func(url string) error
	// AddJiraKey appends a Jira key to an existing nucleus (deduplicates).
	// nil disables the "a" binding in the Jira board.
	AddJiraKey func(nucleusID, key string) error
	// OpenJiraKey opens the Jira browse URL for the given key.
	// Constructs the URL from config. nil disables the "o" binding in nucleus views.
	OpenJiraKey func(key string) error
	// AddClaudeNeuronFromPR creates a Claude neuron on an existing PR branch and appends
	// it to the named nucleus. repo is the short "org/name" form resolved by the caller.
	// profile is the CLAUDE_CONFIG_DIR display name (empty = no profile override).
	// createWorktree=true isolates the checkout in a dedicated worktree; false opens the
	// neuron directly in the repo directory. nil disables the "c" binding in GitHub views.
	AddClaudeNeuronFromPR func(nucleusID, repo, branch, profile string, createWorktree bool) error
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
	confirmID       string // nucleus ID to delete
	confirmNeuronID string // neuron ID to delete (empty = nucleus deletion)

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
	neuronAddNucleusID   string
	neuronAddCursor      int
	neuronAddPhase       int             // 0=type, 1=repo, 2=branchMode, 3=branchDetail
	neuronAddRepos       []string        // loaded when entering phase 1
	neuronAddRepoCursor  int
	neuronAddBranch      textinput.Model // branch name input (phase 3 new)
	neuronAddBranchMode  int             // 0=new branch, 1=existing branch
	neuronAddBaseBranches []string       // configured base branches for selected repo
	neuronAddBaseCursor  int
	neuronAddBaseChosen  bool            // true once base branch is selected
	neuronAddSelectedBase string         // chosen base branch (for new branch flow)
	neuronAddExisting    []string        // existing branches (loaded async for mode 1)
	neuronAddFilter      string          // filter text for existing branch list
	neuronAddExistCursor int

	// PR add state
	prAddNucleusID string
	prAdd          prAddForm

	// github PR list state
	githubPRs      []github.PR
	githubPRCursor int
	githubLoading  bool
	githubErr      string

	// github profile picker (shown when n is pressed on a PR and multiple profiles exist)
	githubProfilePick    bool
	githubProfileCursor  int
	githubProfileNames   []string    // display names for the picker
	githubPickerPendingPR github.PR // PR waiting for a profile selection or nucleus selection

	// github nucleus picker (shown when n/c is pressed on a PR)
	githubNucleusPick     bool
	githubNucleusPickMode string // "add_nvim" | "add_claude"
	githubPickerFilter    string
	githubPickerCursor    int
	githubPickerNuclei    []registry.Nucleus

	// github profile picker mode discrimination and pending nucleus ID
	githubProfilePickMode     string // "add_claude" (only remaining mode)
	githubClaudePickNucleusID string // nucleus ID chosen during add_claude nucleus pick
	githubClaudePickProfile   string // profile chosen during add_claude profile pick

	// worktree picker (final step in add_claude and add_nvim flows)
	githubClaudeWorktreePick   bool
	githubClaudeWorktreeCursor int    // 0=no worktree (default), 1=new worktree
	githubWorktreeMode         string // "add_claude" | "add_nvim" — set before worktree picker opens

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

	// github filter modal state
	githubFilter       github.PRFilter  // committed filter applied on every load
	githubFilterDraft  github.PRFilter  // in-progress edits while modal is open
	githubFilterItems  []filterItem     // flat item list built when modal opens
	githubFilterCursor int              // cursor within githubFilterItems

	// jira board → nucleus picker (attach ticket to an existing nucleus)
	// Shares githubPickerFilter / githubPickerCursor / githubPickerNuclei with the
	// GitHub nucleus picker — the two overlays cannot be open simultaneously.
	jiraNucleusPick bool
	jiraPendingKey  string

	// nucleus → jira key picker (open browser; shown only when >1 key linked)
	jiraKeyPickActive bool
	jiraKeyPickCursor int

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
		prAdd:          newPRAddForm(),
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
		// Kick off the preview tick once on first load (used by stateNucleusDetail).
		if firstLoad {
			return m, m.tickCmd()
		}
		return m, nil

	case profilesLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.nucleusModal = m.nucleusModal.SetProfiles(msg.names, msg.paths)
			m.githubProfileNames = msg.names
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

	case githubFilterConfigLoadedMsg:
		if msg.err != nil {
			m.lastErr = msg.err.Error()
			return m, nil
		}
		m.githubFilterItems = buildFilterItems(msg.myLogin, msg.teammates, msg.repoNames, m.githubFilter)
		m.githubFilterCursor = firstSelectableIdx(m.githubFilterItems)
		m.githubFilterDraft = m.githubFilter
		m.state = stateGitHubFilter
		return m, nil

	case githubFilterConfirmedMsg:
		m.githubFilter = msg.filter
		m.githubLoading = true
		m.state = stateGitHubView
		return m, m.loadGitHubPRsCmd()

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

	case neuronAddReposLoadedMsg:
		if msg.err == nil && len(msg.repos) > 0 {
			m.neuronAddRepos = msg.repos
		}
		return m, nil

	case neuronAddBranchesLoadedMsg:
		if msg.err == nil {
			m.neuronAddExisting = msg.branches
			m.neuronAddExistCursor = 0
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
		case statePRAdd:
			return m.updatePRAdd(msg)
		case stateGitHubView:
			return m.updateGitHubView(msg)
		case stateGitHubPRDetail:
			return m.updateGitHubPRDetail(msg)
		case stateGitHubFilter:
			return m.updateGitHubFilter(msg)
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
		if m.jiraKeyPickActive {
			return m.renderOverlay(m.viewNucleusDetailDashboard(), m.viewJiraKeyPicker())
		}
		return m.viewNucleusDetailDashboard()
	case stateNeuronAdd:
		return m.renderOverlay(m.viewNucleusDetailDashboard(), m.viewNeuronAdd())
	case statePRAdd:
		return m.renderOverlay(m.viewNucleusDetailDashboard(), m.viewPRAdd())
	case stateGitHubView:
		if m.githubClaudeWorktreePick {
			return m.renderOverlay(m.viewGitHubView(), m.viewGitHubClaudeWorktreePicker())
		}
		if m.githubProfilePick {
			return m.renderOverlay(m.viewGitHubView(), m.viewGitHubProfilePicker())
		}
		if m.githubNucleusPick {
			return m.renderOverlay(m.viewGitHubView(), m.viewGitHubNucleusPicker())
		}
		return m.viewGitHubView()
	case stateGitHubPRDetail:
		if m.githubClaudeWorktreePick {
			return m.renderOverlay(m.viewGitHubPRDetail(), m.viewGitHubClaudeWorktreePicker())
		}
		if m.githubProfilePick {
			return m.renderOverlay(m.viewGitHubPRDetail(), m.viewGitHubProfilePicker())
		}
		if m.githubNucleusPick {
			return m.renderOverlay(m.viewGitHubPRDetail(), m.viewGitHubNucleusPicker())
		}
		return m.viewGitHubPRDetail()
	case stateGitHubFilter:
		return m.renderOverlay(m.viewGitHubView(), m.viewGitHubFilter())
	default:
		base := m.viewMain()
		switch m.state {
		case stateNucleusModal:
			return m.renderOverlay(base, m.nucleusModal.View())
		case stateConfirmDelete:
			return m.renderOverlay(base, m.viewConfirm())
		}
		if m.jiraKeyPickActive {
			return m.renderOverlay(base, m.viewJiraKeyPicker())
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

	if req.Submit != nil {
		sub := req.Submit
		m.state = stateList
		svc := m.services.CreateNucleus
		return m, tea.Batch(cmd, func() tea.Msg {
			return actionDoneMsg{err: svc(sub.Task, sub.JiraKey, sub.Profile)}
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

	if m.services.LoadProfiles != nil {
		cmds = append(cmds, m.loadProfilesCmd())
	} else {
		m.nucleusModal = m.nucleusModal.SetProfiles(nil, nil)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) viewMain() string {
	listWidth := clamp(m.width/3, 28, 48)
	detailWidth := m.width - listWidth - 1 // -1 for border
	h := m.contentHeight()

	header := m.viewHeader()
	list := clipLines(m.viewNucleusList(listWidth), h)
	detail := clipLines(m.viewNucleusSummary(detailWidth), h)

	listPane := StyleListPane.Height(h).Width(listWidth).Render(list)
	detailPane := StyleDetailPane.Height(h).Width(detailWidth).Render(detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailPane)

	statusBar := m.viewStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, statusBar)
}

func (m Model) renderOverlay(_ string, content string) string {
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

func (m Model) loadNucleiCmd() tea.Cmd {
	svc := m.services.LoadNuclei
	return func() tea.Msg {
		nuclei, err := svc()
		return nucleiLoadedMsg{nuclei: nuclei, err: err}
	}
}

// captureActivePaneCmd captures the relevant pane based on current state.
// Only fires in stateNucleusDetail — the list view no longer shows a preview.
func (m Model) captureActivePaneCmd() tea.Cmd {
	if m.state == stateNucleusDetail {
		return m.captureDetailPaneCmd()
	}
	return nil
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

// loadBranchInfoCmd fires an async git status/log fetch for the selected neuron.
func (m Model) loadBranchInfoCmd() tea.Cmd {
	if m.services.LoadBranchInfo == nil || len(m.nuclei) == 0 {
		return nil
	}
	n := m.nuclei[m.cursor]
	idx := m.detailNeuronIdx
	if idx >= len(n.Neurons) {
		idx = 0
	}
	worktreePath := ""
	if len(n.Neurons) > 0 {
		worktreePath = n.Neurons[idx].Workdir()
	}
	svc := m.services.LoadBranchInfo
	return func() tea.Msg {
		modified, ahead, err := svc(worktreePath)
		return branchInfoLoadedMsg{modified: modified, aheadCommits: ahead, err: err}
	}
}

// loadNeuronAddBranchesCmd fetches existing branches for the neuron add existing-branch picker.
func (m Model) loadNeuronAddBranchesCmd(repoPath string) tea.Cmd {
	if m.services.ListBranches == nil {
		return func() tea.Msg { return neuronAddBranchesLoadedMsg{} }
	}
	svc := m.services.ListBranches
	return func() tea.Msg {
		branches, err := svc(repoPath)
		return neuronAddBranchesLoadedMsg{branches: branches, err: err}
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

// loadGitHubPRsCmd fires an async fetch of PRs from GitHub using the active filter.
func (m Model) loadGitHubPRsCmd() tea.Cmd {
	if m.services.LoadGitHubPRs == nil {
		return nil
	}
	svc := m.services.LoadGitHubPRs
	f := m.githubFilter
	return func() tea.Msg {
		prs, err := svc(f)
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

// loadGitHubFilterConfigCmd fires an async load of the static config needed
// to populate the GitHub filter modal.
func (m Model) loadGitHubFilterConfigCmd() tea.Cmd {
	if m.services.LoadGitHubFilterConfig == nil {
		return nil
	}
	svc := m.services.LoadGitHubFilterConfig
	return func() tea.Msg {
		myLogin, teammates, repoNames, err := svc()
		return githubFilterConfigLoadedMsg{myLogin: myLogin, teammates: teammates, repoNames: repoNames, err: err}
	}
}

// loadGitHubPRMetaCmd fires an async fetch of PR metadata for the nucleus
// detail panel.
func (m Model) loadGitHubPRMetaCmd() tea.Cmd {
	if m.services.LoadGitHubPR == nil || len(m.nuclei) == 0 {
		return nil
	}
	n := m.nuclei[m.cursor]
	if len(n.PullRequests) == 0 {
		return nil
	}
	svc := m.services.LoadGitHubPR
	repo := n.PullRequests[0].Repo
	number := n.PullRequests[0].Number
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
	keys := m.nuclei[m.cursor].JiraKeys
	if len(keys) == 0 {
		return nil
	}
	key := keys[0]
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
