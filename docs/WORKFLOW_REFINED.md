# Workflow Refined — Nucleus + Neuron UX Design

This document captures design decisions made in the April 2026 design session.
It supersedes conflicting sections of `ARCHITECTURE_PLAN.md`.
It is self-contained and sufficient to resume implementation at any point.

---

## 1. Mental Model

**Nucleus** — a lightweight workspace container. Tracks a task, an optional Jira key,
a Claude profile, and the neurons it owns. It does NOT own a repo, branch, or worktree.
Those belong exclusively to neurons.

**Neuron** — the atomic unit of work. A single running process (Claude, nvim, or shell)
with its own repo, worktree, and branch. A nucleus can own neurons across multiple repos
simultaneously.

One nucleus = one identity (profile). One neuron = one repo + branch + process.

---

## 2. Settled Design Decisions

### 2.1 Nucleus Creation

- Creates an **empty nucleus** — no neurons spawned automatically.
- Fields: task description (required), Jira key (optional), profile (required, picked
  from configured profiles).
- No repo or branch selection at this stage.
- After creation: user lands back on the nucleus list. Neurons are added manually from
  the detail view.

### 2.2 Neuron Creation (from Detail View)

Multi-step form, each step is its own phase:

```
Step 1 — Type
  ▶ claude
    nvim
    shell

Step 2 — Repo
  ▶ /abs/path/repo-a
    /abs/path/repo-b

Step 3 — Branch mode
  ▶ new branch
    existing branch

Step 4a — New branch (if "new branch" selected)
  Base branch:
    (auto-selected if only one configured; otherwise pick from list)
    ▶ development
      main

  Branch name:
  > feature/my-thing

Step 4b — Existing branch (if "existing branch" selected)
  Filter: > fix
  ▶ fix/auth-bug
    fix/rate-limit
    fix/session-timeout
```

Profile is inherited from the nucleus — not asked at neuron add time.

### 2.3 Profile at Nucleus Level

Profile (i.e. `CLAUDE_CONFIG_DIR` path) is stored on the **Nucleus**, not on individual
neurons. When a Claude neuron is spawned, it reads the profile from its parent nucleus.
nvim and shell neurons ignore it.

**Registry change**: remove `Neuron.Profile`; add `Nucleus.Profile string`.

### 2.4 Status Tracking

- Only **Claude neurons** carry a status (`idle` | `working` | `blocked`).
- nvim and shell neurons have no status.
- The **nucleus list view** shows one coloured dot per Claude neuron — not a single
  nucleus-level status. A nucleus with 3 Claude sessions shows 3 dots.
- Nucleus-level status is derived (most active of its Claude neurons) and used only
  for sorting/filtering, not displayed directly.

### 2.5 Navigation

- **Nucleus list**: overview only. Dots indicate activity. `enter` → detail view.
- **Nucleus detail**: select a neuron with `j/k`. Press `g` to jump to that neuron's
  tmux pane. No direct jumping from the list.
- Reason: keeps the list simple; detail view is the action surface.

### 2.6 Config — Repos with Base Branches

Repos change from a flat string list to structured objects:

```json
"repos": [
  { "path": "/abs/path/repo-a", "base_branches": ["development", "main"] },
  { "path": "/abs/path/repo-b", "base_branches": ["main"] }
]
```

Behaviour:
- If `base_branches` has one entry → auto-selected silently, step is skipped.
- If `base_branches` has multiple entries → user picks from list.
- If `base_branches` is empty → text input (user types the base branch name).

**Migration**: on config load, detect `[]string` format and convert each string `s`
to `RepoConfig{Path: s, BaseBranches: []}`. Same atomic write pattern as registry
migration.

### 2.7 GitHub PR → New Nucleus

From the GitHub PR list or PR detail view, pressing `n` creates:

1. A **nucleus** with task pre-filled from the PR title, PR metadata linked
   (`PullRequests[0]`), and profile selected by the user (quick picker).
2. An **nvim neuron** automatically — branch is checked out into a worktree, nvim
   opens for code review. No Claude session.

User lands in nucleus detail view with nvim already open.

Rationale: A PR is fully specified (repo + branch are known). No ambiguity to resolve,
so the neuron can be created without a form.

### 2.8 GitHub PR → Existing Nucleus (PR-first Append)

From the GitHub PR list or PR detail view, pressing `a` (append):

1. Show a nucleus picker — filterable list of existing nuclei.
2. User selects a nucleus.
3. An **nvim neuron** is created with the PR branch checked out into a worktree.
4. The PR is added to `nucleus.PullRequests[]`.

User lands back in the GitHub view. They can navigate to the nucleus detail separately.

Rationale: multi-repo task — a related PR arrives and needs to be pulled into ongoing
work.

---

## 3. Data Model (Target State)

```go
// internal/registry/nucleus.go

type Nucleus struct {
    ID              string
    TaskDescription string
    JiraKey         string          // optional
    Profile         string          // CLAUDE_CONFIG_DIR path; inherited by Claude neurons
    PullRequests    []PullRequest
    Neurons         []Neuron
    Status          string          // derived; "idle" | "working" | "blocked"
    CreatedAt       time.Time
}

type Neuron struct {
    ID           string     // "c1", "c2", "nvim", "sh1"
    Type         NeuronType // "claude" | "nvim" | "shell"
    TmuxTarget   string     // "session:window.pane"
    Status       string     // non-empty only for claude neurons
    RepoPath     string
    WorktreePath string     // empty = use RepoPath
    Branch       string
}

// Profile removed from Neuron — now on Nucleus.
// NeuronType, PullRequest unchanged.
```

---

## 4. Config (Target State)

```go
// internal/config/config.go

type RepoConfig struct {
    Path         string   `json:"path"`
    BaseBranches []string `json:"base_branches"`
}

type Config struct {
    Repos    []RepoConfig      `json:"repos"`
    Profiles map[string]string `json:"profiles"`
    Jira     JiraConfig        `json:"jira"`
    GitHub   GitHubConfig      `json:"github"`
}
```

Migration (in `Load()`): if raw JSON `repos` is `[]string`, convert to `[]RepoConfig`
with empty `BaseBranches`. Log nothing — silent upgrade.

---

## 5. UI State Changes

### Nucleus List (`stateList`)

| Key     | Action                                      |
|---------|---------------------------------------------|
| `enter` | Open nucleus detail                         |
| `n`     | New nucleus (task + jira + profile form)    |
| `d`     | Delete (confirmation)                       |
| `b`     | Jira board                                  |
| `G`     | GitHub PR list                              |
| `r`     | Refresh                                     |
| `?`     | Help                                        |
| `q`     | Quit                                        |

Row format (per nucleus):
```
fixaut   PROJ-42   Fix auth bug   ●● (2 Claude dots)
```

### Nucleus Detail (`stateNucleusDetail`)

| Key     | Action                                      |
|---------|---------------------------------------------|
| `j/k`   | Select neuron in cluster                    |
| `g`     | Jump to selected neuron's tmux pane         |
| `a`     | Add neuron (opens neuron add form)          |
| `d`     | Remove selected neuron                      |
| `p`     | Toggle live pane preview                    |
| `esc/q` | Back to nucleus list                        |

### Neuron Add (`stateNeuronAdd`)

Steps: type → repo → new/existing → (base branch if new) → branch name or filter.
Each step renders as a focused overlay. `esc` goes back one step; `esc` on step 1
cancels and returns to detail.

### GitHub PR List (`stateGitHubView`)

| Key     | Action                                      |
|---------|---------------------------------------------|
| `j/k`   | Navigate PRs                                |
| `enter` | Open PR detail                              |
| `n`     | New nucleus from PR (auto nvim neuron)      |
| `a`     | Append PR to existing nucleus               |
| `r`     | Refresh                                     |
| `esc/q` | Back to nucleus list                        |

### GitHub PR Detail (`stateGitHubPRDetail`)

| Key     | Action                                      |
|---------|---------------------------------------------|
| `j/k`   | Scroll                                      |
| `n`     | New nucleus from PR                         |
| `a`     | Append PR to existing nucleus               |
| `esc/q` | Back to PR list                             |

---

## 6. Services (Additions / Changes)

```go
// internal/ui/model.go — Services struct additions

type Services struct {
    // ... existing fields ...

    // Changed: profile no longer passed per-neuron; nucleus owns it
    CreateNucleus func(task, jiraKey, profile string) error

    // Changed: no profile arg; inherited from nucleus
    AddNeuron func(nucleusID, neuronType, repoPath, branch string, createWorktree bool) error

    // New: create nucleus + nvim neuron from a PR in one call
    CreateReviewNucleus func(task, profile string, pr registry.PullRequest, repo, branch string) error

    // New: append nvim neuron + PR metadata to existing nucleus
    AppendPRToNucleus func(nucleusID string, pr registry.PullRequest, repo, branch string) error

    // New: base branches for a given repo path (from config)
    BaseBranchesForRepo func(repoPath string) []string

    // New: nucleus picker (for PR-append flow)
    LoadNucleusPicker func() ([]registry.Nucleus, error)
}
```

---

## 7. Implementation Phases

Each phase leaves the codebase in a working, buildable state.

### Phase A — Config Migration

**Goal**: Change `repos` from `[]string` to `[]RepoConfig` with base branches.

Files:
- `internal/config/config.go`: add `RepoConfig` struct; update `Config.Repos` type;
  add migration in `Load()`.
- `cmd/ui.go`: update `LoadRepos` to return `[]RepoConfig` (or adapt callers).
- `internal/ui/nucleus_modal.go` (or form file): thread base branches into neuron add
  form step.

Tests:
- `internal/config`: migration from `[]string` → `[]RepoConfig`; round-trip load/save.

**Acceptance**: existing config with `"repos": ["/path"]` silently upgrades; base branch
list appears in neuron add form.

---

### Phase B — Profile to Nucleus Level

**Goal**: Move `Profile` from `Neuron` to `Nucleus` in the registry and all callers.

Files:
- `internal/registry/nucleus.go`: add `Nucleus.Profile`; remove `Neuron.Profile`.
- `internal/registry/registry.go`: add migration step (copy `neurons[0].Profile` →
  `nucleus.Profile` on load if `nucleus.Profile` is empty and neuron has one).
- `cmd/new.go`, `cmd/neuron.go`: update construction; pass profile from nucleus to
  Claude send-keys command.
- `internal/ui/nucleus_modal.go`: profile picker now submits to nucleus, not neuron.
- `cmd/ui.go`: `AddNeuron` service reads profile from nucleus registry entry.

Tests:
- Registry migration: nucleus gains profile from primary neuron.
- `AddNeuron`: Claude launch command includes `CLAUDE_CONFIG_DIR` from nucleus profile.

**Acceptance**: `go build ./... && go test ./...` passes; profile still applied when
launching Claude.

---

### Phase C — Neuron Add Form (Repo + Branch)

**Goal**: Rich neuron-add UX with new/existing branch toggle and base branch picker.

Files:
- `internal/ui/neuron_add.go`: extend to multi-step:
  - Step 0: type picker (existing)
  - Step 1: repo picker
  - Step 2: branch mode (new / existing)
  - Step 3a (new): base branch list (from `BaseBranchesForRepo`) + branch name input
  - Step 3b (existing): filterable branch list (via `ListBranches`)
- `cmd/ui.go`: wire `BaseBranchesForRepo` from config; wire `ListBranches` from git.

Tests:
- `neuron_add.go` unit tests: step transitions, auto-skip base branch if single entry,
  filterable list behaviour.

**Acceptance**: adding a Claude neuron from detail view shows all steps correctly;
worktree is created on the right branch.

---

### Phase D — Nucleus List Multi-Dot Status

**Goal**: Nucleus list shows one dot per Claude neuron instead of one nucleus status.

Files:
- `internal/ui/nucleus_list.go`: change row render to iterate Claude neurons and
  render a dot per one.
- `internal/ui/styles.go`: ensure `StatusDot()` is reusable per-neuron.

Tests:
- View render test: nucleus with 2 Claude neurons shows 2 dots; nucleus with 0 shows
  none; nvim-only nucleus shows none.

**Acceptance**: list view dots reflect per-Claude-neuron status.

---

### Phase E — GitHub PR → New Nucleus (Auto nvim Neuron)

**Goal**: `n` from GitHub PR view creates nucleus + checks out branch in nvim.

Files:
- `cmd/new.go`: add `executeCreateReviewNucleus()` — creates nucleus with PR metadata
  + one nvim neuron on the PR branch.
- `cmd/ui.go`: wire `CreateReviewNucleus` service.
- `internal/ui/github.go`: `n` key → profile picker overlay → call
  `CreateReviewNucleus` → navigate to nucleus detail.

Tests:
- `executeCreateReviewNucleus`: fake git + fake tmux; assert worktree on correct branch,
  nvim neuron stored, PR in `PullRequests[]`.

**Acceptance**: pressing `n` on a PR creates a nucleus with one nvim neuron; detail view
shows the neuron and PR metadata.

---

### Phase F — GitHub PR → Existing Nucleus (PR-first Append)

**Goal**: `a` from GitHub PR view appends an nvim neuron to a picked nucleus.

Files:
- `cmd/neuron.go`: add `executeAppendPRNeuron(nucleusID, pr, repo, branch)`.
- `cmd/ui.go`: wire `AppendPRToNucleus` service.
- `internal/ui/github.go`: `a` key → nucleus picker overlay (filterable list of
  existing nuclei) → call `AppendPRToNucleus` → stay in GitHub view.
- `internal/ui/`: add nucleus picker overlay (reusable, used here first).

Tests:
- `executeAppendPRNeuron`: assert neuron added to correct nucleus; PR appended to
  `PullRequests[]`; worktree on PR branch.
- UI: picker shows nuclei; selecting one fires the service.

**Acceptance**: pressing `a` on a PR, picking an existing nucleus, appends an nvim
neuron; nucleus detail shows both the new neuron and the PR.

---

## 8. What Does Not Change

- `cmd/slug.go` — ID generation unchanged.
- `internal/tmux/tmux.go` — unchanged.
- `internal/jira/` — unchanged.
- `internal/github/` — client implementation unchanged.
- `internal/hooks/` — unchanged.
- Go version, module path, binary name — unchanged.
- Registry atomic write pattern — unchanged.
- `clipLines()` invariant in TUI — unchanged.

---

## 9. Open Questions (Resolved)

| Question | Decision |
|----------|----------|
| Nucleus creation starts empty? | Yes — always empty |
| Profile at nucleus or neuron level? | Nucleus |
| New branch base: single config or list? | List per repo; auto-select if one |
| Worktree always or optional? | Optional toggle (kept) |
| Status on which neurons? | Claude only |
| Nucleus list status display? | One dot per Claude neuron |
| Jump to neuron from list? | No — go to detail first, then `g` |
| GitHub PR → new nucleus: Claude or nvim? | nvim (for code review) |
| GitHub PR append: PR-first or nucleus-first? | PR-first (`a` key in GitHub view) |

---

*Session date: 2026-04-17*
