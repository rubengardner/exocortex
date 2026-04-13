# exocortex — Claude Code Reference

This file is the single source of truth for Claude Code sessions on this project.
Reading it is sufficient to understand the full codebase without opening individual files.

---

## Purpose

`exocortex` is a CLI tool that manages parallel AI coding agents, each isolated in its own
git worktree and tmux pane. The user switches context between agents in sub-second time
using tmux and Neovim. The tool does **not** orchestrate agent logic — it only routes the
user's attention. It also provides a live read-only Jira kanban board view.

Binary name: `exocortex`  
Module path: `github.com/ruben_gardner/exocortex`  
Language: Go 1.24, compiled to a single binary  
CLI framework: `github.com/spf13/cobra`  
TUI framework: `github.com/charmbracelet/bubbletea` + `lipgloss` + `glamour`  
Storage: JSON at `~/.config/exocortex/registry.json`

---

## Directory Layout

```
main.go                   — calls cmd.Execute()
cmd/
  root.go                 — rootCmd, Execute(), PersistentPreRunE (tmux guard)
  new.go                  — `exocortex new --task <desc> [--repo .] [--branch name]`
  list.go                 — `exocortex list`
  goto.go                 — `exocortex goto <id>`
  nvim.go                 — `exocortex nvim <id>` + `nvim-close <id>`
  remove.go               — `exocortex remove <id>`
  respawn.go              — `exocortex respawn <id>`
  services.go             — gitSvc / tmuxSvc / registrySvc interfaces
  registry_adapter.go     — registryAdapter: binds path-based registry funcs to registrySvc
  slug.go                 — slugify() + uniqueID() for generating short agent IDs
  ui.go                   — runTUI(), buildServices() — wires real infra into ui.Services
internal/
  config/config.go        — Config, JiraConfig types; Load/Save; DefaultPath
  registry/registry.go    — Agent/Registry types; Load/Save/Add/Delete/UpdateStatus/UpdateNvimTarget
  git/git.go              — Git struct with Runner interface, worktree operations
  tmux/tmux.go            — Tmux struct with Runner interface, pane + capture operations
  jira/
    jira.go               — Client, Issue, FetchBoard, FetchIssueDescription, ADF→Markdown
    jira_test.go          — table-driven tests with httptest.Server
  ui/
    model.go              — Bubble Tea Model, Update, View, Services struct
    keys.go               — KeyMap / DefaultKeys()
    styles.go             — Lip Gloss styles and StatusDot()
docs/
  todo-jira.md            — original Jira integration design document
```

---

## Core Data Model (`internal/registry`)

```go
type Registry struct {
    Agents []Agent `json:"agents"`
}

type Agent struct {
    ID              string    // e.g. "fixaut", "fixaut2"
    RepoPath        string    // absolute path to git repo root
    WorktreePath    string    // absolute path: <RepoPath>/.worktrees/<ID>
    Branch          string    // e.g. "agent/fixaut"
    TaskDescription string
    TmuxTarget      string    // "session:window.pane" — Claude pane
    NvimTarget      string    // "session:window.pane" — Nvim pane; "" if not open
    Profile         string    // CLAUDE_CONFIG_DIR path used to launch claude (e.g. "~/.claude-work")
    Status          string    // "idle" | "working" | "blocked"
    CreatedAt       time.Time
    LastFile        string    // optional; last opened file
}
```

Registry file is always written atomically via temp-file + `os.Rename`.

---

## Service Interfaces and Dependency Injection (`cmd/services.go`)

Every command that touches infrastructure accepts interfaces, not concrete types.

```go
type gitSvc interface {
    AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error
    RemoveWorktree(repoPath, worktreePath string) error
    ModifiedFiles(worktreePath string) ([]string, error)
    BranchExists(repoPath, branch string) (bool, error)
}

type tmuxSvc interface {
    SplitWindow(workdir string) (string, error)  // returns "session:window.pane"
    SelectPane(target string) error
    KillPane(target string) error
    CapturePane(target string) (string, error)   // plain-text pane content
}

type registrySvc interface {
    Load() (*registry.Registry, error)
    Add(a registry.Agent) error
    Delete(id string) error
    UpdateNvimTarget(id, target string) error
}
```

The `Runner` interface in both `internal/git` and `internal/tmux` abstracts
`os/exec` — `ExecRunner{}` for production, a fake in tests.

---

## Command Flow Summaries

### `new`
1. Resolve `--repo` to an absolute path.
2. Load registry; call `uniqueID(task, agents)` → short slug (max 6 alphanum chars).
3. Auto-set branch to `"agent/<id>"` if `--branch` not provided.
4. `git worktree add [-b <branch>] .worktrees/<id> <branch>`
5. `tmux split-window -h -c <worktreePath> -P -F "#{session_name}:..."` → `TmuxTarget`
6. SendKeys: `claude` or `CLAUDE_CONFIG_DIR=<path> claude` depending on profile.
7. On tmux failure: best-effort `git worktree remove --force` before returning error.
8. `agent.Profile = claudeConfigDir` stored in registry.
9. Append `Agent` to registry; print `"created agent <id>"`.

### `list`
Read registry → `text/tabwriter` table: ID, BRANCH, TASK, STATUS, TMUX TARGET.

### `goto <id>`
Load registry → `FindByID` → `tmux select-pane -t <TmuxTarget>`.

### `nvim <id>`
1. `prepareNvimExec`: Load registry → `FindByID` → `git ls-files -m` in worktree.
2. If modified files exist, target is `files[0]`; otherwise `"."`.
3. `exec.LookPath("nvim")` → return `NvimSpec{Dir, Binary, Argv}`.
4. `realExec`: `os.Chdir(dir)` then `syscall.Exec(binary, argv, os.Environ())`.
   Never returns on success (process is replaced).

`nvim` is the **only** command exempt from the tmux guard (`PersistentPreRunE`).

### `nvim-close <id>`
Best-effort `tm.KillPane(agent.NvimTarget)` → `reg.UpdateNvimTarget(id, "")`.

### `respawn <id>`
Re-opens the agent's tmux pane (used when a window was closed accidentally). Reads
`agent.Profile` → resolves to `CLAUDE_CONFIG_DIR` path from config → re-sends keys.

### `remove <id>`
Load → `FindByID` → kill tmux pane (warn, don't abort) → remove git worktree
(warn, don't abort) → `reg.Delete(id)`.

---

## TUI Architecture (`internal/ui/model.go`)

Follows Elm Architecture. Eight view states:

| Constant            | Meaning                                              |
|---------------------|------------------------------------------------------|
| `StateList`         | Main agent list + detail panel with live preview     |
| `StateRepoSelect`   | Repo picker overlay (before the new-agent form)      |
| `StateProfileSelect`| Profile picker overlay (after repo, before form)     |
| `StateNewOverlay`   | New-agent form (modal overlay)                       |
| `StateConfirmDelete`| Delete confirmation dialog                           |
| `StateHelp`         | Full-page keyboard shortcuts                         |
| `StateJiraBoard`    | Live 3-column Jira kanban view                       |
| `StateJiraDetail`   | Full-screen description for a selected Jira issue    |

`Services` struct (injected by `cmd/ui.go`):

```go
type Services struct {
    LoadAgents    func() ([]registry.Agent, error)
    LoadRepos     func() ([]string, error)                          // nil = skip picker
    LoadProfiles  func() (map[string]string, error)                 // nil = skip picker
    LoadJiraBoard func() (columns []string, issues map[string][]jira.Issue, err error)
    LoadJiraIssue func(key string) (markdown string, err error)     // nil = disable detail
    CapturePane   func(tmuxTarget string) (string, error)           // nil = no preview
    CreateAgent   func(task, repo, branch, profile string) error
    RemoveAgent   func(id string) error
    GotoAgent     func(id string) error
    OpenNvim      func(id string) error
    CloseNvim     func(id string) error                             // nil = disable
    RespawnAgent  func(id string) error                             // nil = disable
}
```

Side effects are never performed inside `Update` directly — they return `tea.Cmd`
closures that emit message types (`actionDoneMsg`, `agentsLoadedMsg`, etc.).

### Key bindings (StateList)

| Key    | Action                              |
|--------|-------------------------------------|
| `j/↓`  | cursor down                         |
| `k/↑`  | cursor up                           |
| `g`    | goto agent's Claude tmux pane       |
| `e`    | open nvim in agent worktree         |
| `E`    | close nvim pane                     |
| `n`    | new agent (opens repo → profile → form flow) |
| `d`    | delete agent (confirmation required)|
| `r`    | refresh agent list                  |
| `R`    | respawn agent window                |
| `p`    | toggle live pane preview            |
| `b`    | open Jira board                     |
| `?`    | full help                           |
| `q`    | quit                                |

### Key bindings (StateJiraBoard)

| Key       | Action                          |
|-----------|---------------------------------|
| `j/↓`     | row down (with scroll)          |
| `k/↑`     | row up (with scroll)            |
| `h/←`     | column left                     |
| `l/→`     | column right                    |
| `space`   | open issue description detail   |
| `r`       | refresh board                   |
| `esc/q`   | back to list                    |

### Key bindings (StateJiraDetail)

| Key       | Action       |
|-----------|--------------|
| `j/↓`     | scroll down  |
| `k/↑`     | scroll up    |
| `pgdn`    | page down    |
| `pgup`    | page up      |
| `esc/q`   | back to board|

### Live pane preview

A `tea.Tick` fires every second (started on first agent load) calling
`CapturePane(agent.TmuxTarget)`. The result is stored in `m.paneContent` and
rendered in the detail panel. `clipLines` is used to prevent overflow from pushing
the header off-screen. Toggle with `p`.

### New agent flow

`n` → (if repos configured) `StateRepoSelect` → (if profiles configured)
`StateProfileSelect` → `StateNewOverlay`. Each step fires a load cmd; the
`transitionAfterRepo()` method decides whether to show the profile picker or
go straight to the form.

---

## Jira Integration (`internal/jira/`)

### Client

```go
type Client struct { baseURL, email, apiToken string }

func New(baseURL, email, apiToken string) *Client

// FetchBoard returns issues grouped by status name.
// boardID > 0 → GET /rest/agile/1.0/board/{id}/issue (no status JQL filter; client-side grouping)
// boardID = 0 → GET /rest/api/3/search/jql with project + status JQL
func (c *Client) FetchBoard(boardID int, project string, statuses []string) (map[string][]Issue, error)

// FetchIssueDescription fetches description from /rest/api/3/issue/{key}?fields=description
// and converts Atlassian Document Format (ADF) to Markdown.
func (c *Client) FetchIssueDescription(key string) (string, error)
```

ADF→Markdown handles: paragraphs, headings (H1–H6), bold/italic/code/strike/links,
bullet + ordered lists (nested), code blocks with language, blockquotes, hard breaks,
rules, mentions, inline/block cards.

When `board_id` is set, the Agile board endpoint is used and no status names are sent
to the API (avoiding 400 errors from mismatched status names). Issues are grouped
client-side; only statuses matching the configured columns are displayed.

### Jira board view layout

```
◈  EXOCORTEX                                   3 agent(s)
──────────────────────────────────────────────────────────
 IN PROGRESS (2)    │ READY FOR CR (1)  │ CODE REVIEW (3)
 ───────────────    │ ──────────────    │ ──────────────
▶PROJ-123           │  PROJ-456         │  PROJ-789
  Fix auth bug      │  Rate limiting    │  Review login
  @Alice            │  @Bob             │  @Charlie
──────────────────────────────────────────────────────────
  b/esc back   j/k row   h/l column   space detail   r refresh
```

Each issue renders as 3 lines (key, summary, assignee). Per-column scroll offsets
(`jiraScrollOffs []int`) keep the selected row in view. `jiraMaxVisible()` computes
visible slots: `(contentHeight - 2 + 1) / 4`.

---

## Config (`internal/config`)

User settings live at `~/.config/exocortex/config.json`:

```json
{
  "repos": ["/abs/path/to/repo1", "/abs/path/to/repo2"],
  "profiles": {
    "personal": "~/.claude-personal",
    "work":     "~/.claude-work"
  },
  "jira": {
    "base_url":  "https://yourcompany.atlassian.net",
    "email":     "you@company.com",
    "api_token": "your-token",
    "project":   "PROJ",
    "board_id":  75,
    "statuses":  ["In Progress", "Ready for CR", "Code Review"]
  }
}
```

- `repos`: shown in TUI repo picker (StateRepoSelect). Empty = no picker, default `"."`.
- `profiles`: maps display name → `CLAUDE_CONFIG_DIR` path. Empty = no picker.
  Selected profile is passed as 4th arg to `CreateAgent`; stored as `agent.Profile`.
- `jira.statuses`: optional; defaults to `["In Progress", "Ready for CR", "Code Review"]`
  if omitted (`JiraConfig.ResolvedStatuses()`).
- `jira.board_id`: optional; when set, uses Agile board API instead of project search.

`LoadRepos`/`LoadProfiles` are `nil` in test helpers — pickers are never shown.

---

## ID Generation (`cmd/slug.go`)

```
slugify("Fix auth bug") → "fixaut"   (lowercase, strip non-alphanum, cap at 6 chars)
uniqueID(task, agents) → appends numeric suffix ("fixaut2", "fixaut3"…) on collision
```

---

## Key Invariants

- All commands **require a tmux session** (`$TMUX` must be set) except `nvim`.
- Worktrees live at `<repoPath>/.worktrees/<id>`.
- Registry writes are atomic (temp + rename); concurrent writes are last-write-wins.
- `syscall.Exec` for nvim means no orphaned Go processes.
- `remove` is best-effort for tmux/git cleanup — registry deletion still happens.
- LSP diagnostics are often stale — always verify with `go build ./... && go test ./...`.
- `lipgloss.Height()` sets a **minimum**, not maximum — use `clipLines()` before rendering
  to prevent variable content from pushing the header off-screen.
- The preview tick starts only on the first successful agent load (not in `Init`) to avoid
  breaking the `agentsLoaded` test helper pattern.
