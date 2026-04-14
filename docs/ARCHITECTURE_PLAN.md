# Exocortex — Architecture Plan

This document describes the target architecture for the next evolution of Exocortex,
derived from the vision in `docs/PRD.md`. It serves as the authoritative reference for
all development work until it is superseded.

---

## 1. Direction Summary

The current product manages "Agents" — one Claude Code process per git worktree. The
PRD reframes this as a richer hierarchy:

| Concept | Was | Now |
|---------|-----|-----|
| Top-level unit | Agent (one process + one worktree) | **Nucleus** (one worktree + N processes) |
| Running process | (implicit) | **Neuron** (typed: claude, nvim, shell) |
| Creation trigger | Manual form | Jira ticket or GitHub PR or ad-hoc |
| Branch naming | Free-form | `task/<jira-key>/<slug>` (dev), existing branch (review) |
| Detail view | Live pane preview | Holistic dashboard: ticket + neurons + PR stats |
| External views | Jira board only | Jira board + GitHub PR list |

The architecture must support this without becoming a monolith. Each concern stays in
its own package; the TUI is the only place that assembles them.

---

## 2. Core Data Model

### 2.1 Nucleus

A Nucleus is the primary unit of work. It owns exactly one git worktree and one branch.
It may be linked to a Jira ticket, a GitHub PR, both, or neither.

```go
// internal/registry/nucleus.go

type Nucleus struct {
    ID              string    // e.g. "fixaut"
    RepoPath        string    // absolute path to repo root
    WorktreePath    string    // <RepoPath>/.worktrees/<ID>
    Branch          string    // e.g. "task/PROJ-42/fix-auth"
    TaskDescription string    // free-form description

    // Optional external linkage
    JiraKey  string // "PROJ-42" or ""
    PRNumber int    // GitHub PR number or 0
    PRRepo   string // "owner/repo" or ""

    Neurons []Neuron

    Status    string    // "idle" | "working" | "waiting" | "blocked"
    CreatedAt time.Time
}
```

### 2.2 Neuron

A Neuron is a running process inside a Nucleus. Multiple Neurons can run concurrently
in the same worktree.

```go
// internal/registry/nucleus.go

type NeuronType string

const (
    NeuronClaude NeuronType = "claude"
    NeuronNvim   NeuronType = "nvim"
    NeuronShell  NeuronType = "shell"
)

type Neuron struct {
    ID         string     // unique within the Nucleus, e.g. "c1", "nvim", "sh1"
    Type       NeuronType
    TmuxTarget string // "session:window.pane"
    Profile    string // CLAUDE_CONFIG_DIR path (claude neurons only)
    Status     string // "idle" | "working" | "waiting" | "blocked"
}
```

### 2.3 Registry

The Registry file format changes from `[]Agent` to `[]Nucleus`. A one-time migration
function reads the old format and up-converts it on first load.

```go
// internal/registry/registry.go

type Registry struct {
    Version int       `json:"version"` // bumped to 2 for migration
    Nuclei  []Nucleus `json:"nuclei"`
}
```

**Migration rule**: Each old `Agent` becomes a `Nucleus` with one `Neuron` of type
`claude`. `Agent.TmuxTarget` → `Neuron.TmuxTarget`; `Agent.NvimTarget` (if set) →
second `Neuron` of type `nvim`.

---

## 3. Package Architecture

```
main.go
cmd/
    root.go               — rootCmd, Execute(), tmux guard
    new.go                — `exocortex new` (create Nucleus, ad-hoc)
    list.go               — `exocortex list`
    goto.go               — `exocortex goto <id>`
    remove.go             — `exocortex remove <id>`
    respawn.go            — `exocortex respawn <id> [neuron-id]`
    neuron.go             — `exocortex neuron add/rm` sub-commands
    services.go           — interfaces: nucleusSvc, gitSvc, tmuxSvc, githubSvc
    registry_adapter.go   — binds registry funcs to nucleusSvc interface
    slug.go               — slugify() + uniqueID() (unchanged)
    ui.go                 — buildServices(), runTUI()

internal/
    config/config.go      — add GitHubConfig; keep JiraConfig
    registry/
        nucleus.go        — Nucleus + Neuron types
        registry.go       — Load/Save/Add/Delete/Update + v1→v2 migration
        registry_test.go
    git/git.go            — add: ListBranches(), CheckoutExisting()
    tmux/tmux.go          — unchanged
    jira/jira.go          — add: FetchIssue() for Nucleus creation metadata
    github/
        github.go         — Client, PR, PRDetail, ListPRs, FetchPRDetail
        github_test.go
    hooks/hooks.go        — update hook target from agent→nucleus IDs (logic unchanged)
    ui/
        model.go          — Model struct, Init, Update (router), View (router)
        states.go         — ViewState constants + state transition helpers
        messages.go       — all tea.Msg types (extracted from model.go)
        nucleus_list.go   — StateNucleusList: render + update
        nucleus_detail.go — StateNucleusDetail: holistic dashboard render + update
        nucleus_form.go   — StateNucleusCreate + StateRepoSelect + StateProfileSelect
        neuron_add.go     — StateNeuronAdd: type picker + profile picker
        jira.go           — StateJiraBoard + StateJiraDetail (extracted from model.go)
        github.go         — StateGitHubView + StateGitHubPRDetail
        confirm.go        — StateConfirmDelete (extracted)
        help.go           — StateHelp (extracted)
        keys.go           — KeyMap (update for new actions)
        styles.go         — styles (unchanged)
```

### 3.1 Separation of Concerns

| Layer | Responsibility | Must NOT |
|-------|---------------|----------|
| `internal/*` | Domain logic, data, API clients | Import `cmd/` or `ui/` |
| `cmd/` | Wire infrastructure to interfaces; CLI surface | Contain business logic |
| `internal/ui/` | Display and user input | Call infra directly; use interfaces via `Services` |
| `Services` struct | Function pointers that bridge `cmd/` wiring to `ui/` | Hold state |

Side effects inside `Update()` are forbidden — they must be wrapped in `tea.Cmd` closures.

---

## 4. Service Interfaces

### 4.1 `nucleusSvc` (replaces `registrySvc`)

```go
type nucleusSvc interface {
    Load() (*registry.Registry, error)
    Add(n registry.Nucleus) error
    Delete(id string) error
    AddNeuron(nucleusID string, neuron registry.Neuron) error
    RemoveNeuron(nucleusID, neuronID string) error
    UpdateStatus(id, status string) error
    UpdateNeuronTarget(nucleusID, neuronID, target string) error
}
```

### 4.2 `gitSvc` (extended)

```go
type gitSvc interface {
    AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error
    RemoveWorktree(repoPath, worktreePath string) error
    ModifiedFiles(worktreePath string) ([]string, error)
    BranchExists(repoPath, branch string) (bool, error)
    ListBranches(repoPath string) ([]string, error)         // new: for review workflow
    CheckoutExisting(repoPath, worktreePath, branch string) error // new: review workflow
}
```

### 4.3 `githubSvc` (new)

```go
type githubSvc interface {
    ListPRs() ([]github.PR, error)
    FetchPRDetail(repo string, number int) (*github.PRDetail, error)
}
```

### 4.4 TUI `Services` struct (updated)

```go
type Services struct {
    // Nucleus management
    LoadNuclei    func() ([]registry.Nucleus, error)
    CreateNucleus func(task, repo, branch, profile, jiraKey string) error
    RemoveNucleus func(id string) error
    GotoNucleus   func(id string) error
    RespawnNeuron func(nucleusID, neuronID string) error

    // Neuron management
    AddNeuron     func(nucleusID string, neuronType, profile string) error
    RemoveNeuron  func(nucleusID, neuronID string) error
    OpenNvim      func(nucleusID string) error
    CloseNvim     func(nucleusID string) error

    // Infrastructure
    LoadRepos     func() ([]string, error)                       // nil = no picker
    LoadProfiles  func() (map[string]string, error)              // nil = no picker
    CapturePane   func(tmuxTarget string) (string, error)        // nil = no preview

    // Jira
    LoadJiraBoard func() (columns []string, issues map[string][]jira.Issue, err error)
    LoadJiraIssue func(key string) (markdown string, err error)  // nil = no detail

    // GitHub
    LoadGitHubPRs   func() ([]github.PR, error)                 // nil = no GitHub view
    LoadGitHubPR    func(repo string, number int) (*github.PRDetail, error) // nil = no detail
}
```

---

## 5. New GitHub Package

### 5.1 Types

```go
// internal/github/github.go

type PR struct {
    Number    int
    Title     string
    Author    string
    Repo      string    // "owner/repo"
    Branch    string    // head branch
    Base      string    // base branch
    State     string    // "open" | "closed" | "merged"
    IsDraft   bool
    UpdatedAt time.Time
    URL       string
}

type PRDetail struct {
    PR
    Body         string      // PR description (Markdown)
    Additions    int
    Deletions    int
    ChangedFiles int
    Files        []PRFile
}

type PRFile struct {
    Path      string
    Status    string // "added" | "modified" | "deleted" | "renamed"
    Additions int
    Deletions int
    Patch     string // unified diff patch
}
```

### 5.2 Client

```go
type Client struct {
    token string
    org   string // optional; for listing team PRs
}

func New(token, org string) *Client
func (c *Client) ListPRs() ([]PR, error)      // personal + team (if org set)
func (c *Client) FetchPRDetail(repo string, number int) (*PRDetail, error)
```

Uses GitHub REST API v3. Auth via `Authorization: Bearer <token>`. No external
dependencies — standard `net/http`.

### 5.3 Config

```json
{
  "github": {
    "token": "ghp_...",
    "org":   "myorg"
  }
}
```

`org` is optional. When set, PRs from all org repos where the user is a reviewer are
included. Token is read from config (not env) to keep consistent with Jira auth pattern.

---

## 6. UI State Machine

### 6.1 New State Constants

```go
const (
    // Existing (kept)
    StateNucleusList    // was StateList
    StateConfirmDelete
    StateHelp
    StateRepoSelect
    StateProfileSelect
    StateJiraBoard
    StateJiraDetail

    // Refactored
    StateNucleusCreate  // was StateNewOverlay; now supports dev + review modes

    // New
    StateNucleusDetail  // holistic dashboard for one Nucleus
    StateNeuronAdd      // add a Neuron to an open Nucleus
    StateBranchSearch   // branch picker for review workflow
    StateGitHubView     // GitHub PR list
    StateGitHubPRDetail // PR stats + file browser
)
```

### 6.2 State Transition Map

```
StateNucleusList
  ├─ n           → StateRepoSelect → StateProfileSelect → StateNucleusCreate
  ├─ enter/→     → StateNucleusDetail
  ├─ d           → StateConfirmDelete
  ├─ b           → StateJiraBoard
  ├─ G           → StateGitHubView          (if LoadGitHubPRs != nil)
  └─ ?           → StateHelp

StateNucleusDetail
  ├─ a           → StateNeuronAdd
  ├─ e           → open nvim neuron
  ├─ E           → close nvim neuron
  ├─ g           → goto active claude neuron
  ├─ j/k         → select neuron in cluster
  ├─ p           → toggle pane preview
  └─ esc/q       → StateNucleusList

StateJiraBoard
  ├─ N           → StateNucleusCreate (mode=dev, pre-filled JiraKey)
  ├─ space       → StateJiraDetail
  └─ esc/q       → StateNucleusList

StateGitHubView
  ├─ R           → StateNucleusCreate (mode=review, pre-filled PRNumber+branch)
  ├─ space/enter → StateGitHubPRDetail
  └─ esc/q       → StateNucleusList

StateGitHubPRDetail
  ├─ R           → StateNucleusCreate (mode=review)
  ├─ e           → open changed file in nvim (deep-link)
  └─ esc/q       → StateGitHubView

StateNucleusCreate
  ├─ mode=dev    → create new branch task/<jiraKey>/<slug>; git worktree add -b
  ├─ mode=review → StateBranchSearch (pick/confirm existing branch); git worktree add (no -b)
  └─ mode=adhoc  → create free-form branch; git worktree add -b

StateBranchSearch
  ├─ type to filter branches
  ├─ enter       → confirm → create Nucleus with checked-out branch
  └─ esc         → StateNucleusCreate
```

---

## 7. Nucleus Dashboard (StateNucleusDetail)

Layout (full screen, 3-panel):

```
 NUCLEUS fixaut  •  task/PROJ-42/fix-auth  •  working        [↑↓ neurons] [g goto] [a add] [q back]
 ─────────────────────────────────────────────────────────────────────────────────────────────────
 NEURONS (3)              │ JIRA PROJ-42                │ LIVE PREVIEW
 ─────────────────        │ ──────────────              │ ─────────────────────────────────────────
▶ c1   claude  working    │ Fix authentication bug      │ > Running tests...
  nvim  nvim    open      │ Status: In Progress         │   PASS TestAuth (1.2s)
  sh1   shell  idle       │ @Alice                      │   PASS TestSession (0.4s)
                          │ https://jira.co/PROJ-42     │ > All tests passing.
 ─────────────────        │                             │
 BRANCH INFO              │ PR #123                     │
 Modified: 3 files        │ +142 -38 • 7 files          │
 Ahead: 2 commits         │ github.com/.../pull/123     │
```

- Left panel: Neuron cluster (selectable with j/k; g focuses the selected neuron's pane)
- Middle panel: Ticket metadata (Jira) + PR summary (GitHub) if linked; otherwise task description
- Right panel: Live pane preview of the selected Neuron (CapturePane tick)

When neither Jira nor GitHub is linked, the middle panel shows branch diff stats from
`git status` and `git log --oneline origin/main..HEAD`.

---

## 8. Model File Decomposition

The current `internal/ui/model.go` (1291 lines) is split as follows:

| New file | Extracted content |
|----------|------------------|
| `model.go` | `Model` struct, `Init()`, `Update()` router, `View()` router |
| `states.go` | `ViewState` constants, `state transition helpers` |
| `messages.go` | All `tea.Msg` type definitions (currently inline in model.go) |
| `nucleus_list.go` | `updateNucleusList()`, `viewNucleusList()` |
| `nucleus_detail.go` | `updateNucleusDetail()`, `viewNucleusDetail()` |
| `nucleus_form.go` | `updateNucleusCreate()`, `viewNucleusCreate()`, repo/profile pickers |
| `neuron_add.go` | `updateNeuronAdd()`, `viewNeuronAdd()` |
| `jira.go` | `updateJiraBoard()`, `viewJiraBoard()`, `updateJiraDetail()`, `viewJiraDetail()` |
| `github.go` | `updateGitHubView()`, `viewGitHubView()`, `updateGitHubPRDetail()`, `viewGitHubPRDetail()` |
| `confirm.go` | `updateConfirmDelete()`, `viewConfirmDelete()` |
| `help.go` | `updateHelp()`, `viewHelp()` |

Each file is a standalone set of functions on `*Model`. No new types needed — the router
in `model.go` delegates by `switch m.state`. Files stay cohesive and small (<300 lines each).

---

## 9. Configuration Changes

`~/.config/exocortex/config.json` gains a `github` block:

```json
{
  "repos": ["..."],
  "profiles": { "work": "~/.claude-work" },
  "github": {
    "token": "ghp_...",
    "org":   "myorg"
  },
  "jira": { "..." }
}
```

`GitHubConfig` struct in `internal/config/config.go`:

```go
type GitHubConfig struct {
    Token string `json:"token"`
    Org   string `json:"org,omitempty"`
}
```

`LoadGitHubPRs` / `LoadGitHubPR` are `nil` in `buildServices()` when `GitHubConfig.Token`
is empty — disables GitHub view gracefully, identical to how Jira is disabled.

---

## 10. Branch Naming Convention

| Workflow | Branch format | Example |
|----------|---------------|---------|
| Dev (from Jira) | `task/<jira-key>/<slug>` | `task/PROJ-42/fix-auth` |
| Dev (ad-hoc) | `task/<slug>` | `task/fix-auth` |
| Review | Existing branch (unchanged) | `feature/oauth-refactor` |

The branch name is shown in `StateNucleusCreate` pre-filled and editable before
confirmation. slugify() is applied to the task description portion only.

---

## 11. Implementation Phases

Work is broken into phases. Each phase is independently shippable and leaves the
codebase in a working state.

### Phase 1 — Core Model Refactor *(no new features, pure rename + multi-neuron)*

Goal: Rename Agent → Nucleus/Neuron, support multiple Neurons per Nucleus.
Existing behaviour is preserved 100%.

1. Add `internal/registry/nucleus.go` with `Nucleus` + `Neuron` types.
2. Update `internal/registry/registry.go`: `Registry.Agents` → `Registry.Nuclei`;
   add `version` field; write `migrateV1toV2()`.
3. Update `registrySvc` → `nucleusSvc` interface; update `registry_adapter.go`.
4. Update all `cmd/*.go` to use `Nucleus`/`Neuron` vocabulary.
5. Update `internal/ui/model.go`: rename `agents` → `nuclei`, `StateList` → `StateNucleusList`.
6. Update `internal/hooks/hooks.go` for new IDs.
7. Update all tests.

**Acceptance**: `go build ./... && go test ./...` passes. TUI behaviour is identical.

### Phase 2 — UI File Decomposition *(no new features, split model.go)*

Goal: Make `model.go` maintainable before adding new states.

1. Extract `states.go`, `messages.go`.
2. Extract `nucleus_list.go`, `jira.go`, `confirm.go`, `help.go`.
3. Extract `nucleus_form.go` (new/repo/profile pickers).
4. Keep `model.go` as the router only.

**Acceptance**: `go build ./... && go test ./...` passes. No behaviour change.

### Phase 3 — Nucleus Detail Dashboard *(StateNucleusDetail)*

Goal: Pressing enter on a Nucleus opens the holistic 3-panel view.

1. Add `nucleus_detail.go` with 3-panel layout.
2. Left panel: Neuron cluster list (j/k to select, g to focus in tmux).
3. Right panel: Reuse existing pane preview (`CapturePane` tick).
4. Middle panel: Branch info from `git status`/`git log` (no Jira/GitHub yet).
5. Wire `enter` key in `nucleus_list.go` → `StateNucleusDetail`.
6. Wire `a` key → `StateNeuronAdd` with type picker (claude/nvim/shell).
7. Update `Services` with `AddNeuron`, `RemoveNeuron`.

**Acceptance**: Can open nucleus detail, see neurons, add a Claude neuron, switch preview.

### Phase 4 — GitHub Integration

Goal: GitHub PR list + PR detail view.

1. Implement `internal/github/github.go` with `Client`, `ListPRs`, `FetchPRDetail`.
2. Add `github_test.go` using `httptest.Server`.
3. Add `GitHubConfig` to `internal/config/config.go`.
4. Add `StateGitHubView` in `internal/ui/github.go`: PR list, `G` from nucleus list.
5. Add `StateGitHubPRDetail`: stats, file list, scroll.
6. Wire `LoadGitHubPRs` / `LoadGitHubPR` in `cmd/ui.go` (nil-safe).

**Acceptance**: `G` opens PR list. Space/enter opens PR detail. Works with nil config (graceful skip).

### Phase 5 — Review Workflow

Goal: From a PR, spawn a Nucleus on the PR's branch.

1. Add `ListBranches()` + `CheckoutExisting()` to `internal/git/git.go`.
2. Add `StateBranchSearch` in `nucleus_form.go`: filterable branch list.
3. `StateNucleusCreate` gets a `mode` field (`dev` | `review` | `adhoc`).
4. Review mode: no `-b` flag to `git worktree add`; branch must exist.
5. Wire `R` key in `StateGitHubView` + `StateGitHubPRDetail` → review mode creation.
6. Store `PRNumber` + `PRRepo` on the created Nucleus.

**Acceptance**: Can spawn a Nucleus from a GitHub PR; worktree is on the PR branch.

### Phase 6 — Jira ↔ Nucleus Integration

Goal: From the Jira board, spawn a Nucleus with proper branch naming and ticket linking.

1. Add `FetchIssue()` to `internal/jira/jira.go` for metadata at Nucleus create time.
2. Wire `N` key in `StateJiraBoard` → `StateNucleusCreate` (mode=dev, pre-filled JiraKey).
3. Branch name pre-filled as `task/<jira-key>/` (user types the suffix).
4. Store `JiraKey` on the created Nucleus.
5. Display Jira metadata in Nucleus detail middle panel when `JiraKey != ""`.

**Acceptance**: Can create a Nucleus from Jira board; detail view shows ticket info.

### Phase 7 — Polish & Deep-links

Goal: Close remaining UX gaps from the PRD.

1. Nucleus detail middle panel: show PR diff summary when `PRNumber != 0`.
2. `StateGitHubPRDetail`: press `e` on a changed file to open it in nvim at the diff
   location (pass `+<line>` arg to nvim; reuse `OpenNvim` with file override).
3. Jira ticket detail: add "Open in browser" shortcut (`o` → `xdg-open <URL>`).
4. `StateNucleusList`: show Jira key and PR badge inline per row.
5. Status bar hook: update to show Nucleus ID + active Neuron count.
6. `exocortex init` output: update tmux snippet for multi-neuron window names.

---

## 12. Key Invariants (Preserved)

All invariants from the current CLAUDE.md remain:

- All commands require `$TMUX` (except `nvim`).
- Worktrees live at `<repoPath>/.worktrees/<id>`.
- Registry writes are atomic (temp + rename).
- `syscall.Exec` for nvim — no orphaned processes.
- `remove` is best-effort for tmux/git; registry deletion always succeeds.
- Side effects never inside `Update()` — always `tea.Cmd` closures.
- `clipLines()` before variable-height content in the TUI.

---

## 13. Testing Strategy (Additions)

| Package | New tests |
|---------|-----------|
| `internal/registry` | Migration from v1 (Agent) to v2 (Nucleus), multi-Neuron CRUD |
| `internal/github` | `ListPRs`, `FetchPRDetail` with `httptest.Server` |
| `internal/git` | `ListBranches`, `CheckoutExisting` with fake Runner |
| `internal/ui` | `StateNucleusDetail` render/update; `StateGitHubView`; review workflow state transitions |

No mocking libraries. All fakes implement the same interfaces used in production.

---

## 14. What Does Not Change

- `cmd/slug.go` — ID generation is unchanged.
- `internal/tmux/tmux.go` — Tmux wrapper is unchanged.
- `internal/jira/jira.go` — ADF→Markdown, board fetch, issue fetch unchanged (additive only).
- `internal/hooks/hooks.go` — Hook logic unchanged; only IDs update.
- `internal/ui/keys.go` and `styles.go` — Extended but not rewritten.
- Go version, module path, binary name — unchanged.
- Config file location — unchanged.

---

*Last updated: 2026-04-14*
