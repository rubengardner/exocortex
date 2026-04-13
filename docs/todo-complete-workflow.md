# Complete Workflow — Implementation Plan

## Vision

One command creates an agent: a ClaudeCode terminal starts automatically, hooks
report status back in real time, the tmux status bar shows which agents need
attention, and a popup TUI lets you jump between them from anywhere. `g` goes
to ClaudeCode, `e` opens nvim. When you're done, `finish` pushes the branch.

---

## Phase 1 — Status infrastructure

Everything else depends on `waiting` existing and being settable from a hook.

### `internal/registry/registry.go`

- [ ] No data model change needed — status is a free string

### `cmd/status.go` (new command)

- [ ] `exocortex status <id> <status>` — thin wrapper around `registry.UpdateStatus`
- [ ] Accepts `idle | working | waiting | blocked`
- [ ] This is the hook target — must be fast, no TUI, no prompts
- [ ] Wire into `rootCmd` in `root.go`

### `internal/ui/styles.go`

- [ ] Add `waiting` colour to `StatusDot` — distinct from working (yellow/amber)

### `internal/ui/model.go`

- [ ] Add `waiting` to `statusCycle` between `working` and `blocked`

### Tests

- [ ] `cmd/status_test.go` — `TestStatusCmd_UpdatesRegistry`, `TestStatusCmd_UnknownID`

---

## Phase 2 — ClaudeCode hooks auto-wired on `new`

### `internal/hooks/hooks.go` (new package)

- [ ] `Write(worktreePath, agentID string) error`
- [ ] Creates `.claude/` dir inside worktree if absent
- [ ] Merges into existing `.claude/settings.json` if present (unmarshal → set hooks → marshal)
- [ ] Hook payload:
  ```json
  {
    "hooks": {
      "UserPromptSubmit": [
        { "type": "command", "command": "exocortex status <id> working" }
      ],
      "Stop": [
        {
          "type": "command",
          "command": "exocortex status <id> waiting; printf '\\a'"
        }
      ]
    }
  }
  ```
  Replace `<id>` with the real agent ID at write time.

### `cmd/new.go`

- [ ] After worktree is created, call `hooks.Write(worktreePath, id)`
- [ ] Failure is warn-not-abort (same pattern as tmux/git cleanup)

### `cmd/new.go` — auto-start ClaudeCode

- [ ] After `tm.NewWindow(...)`, call `tm.SendKeys(target, "claude")` to start ClaudeCode immediately
- [ ] Agent goes live the moment it is created

### Tests

- [ ] `internal/hooks/hooks_test.go`
  - [ ] `TestWrite_CreatesFile` — file written with correct hook commands
  - [ ] `TestWrite_MergesExisting` — does not clobber unrelated keys in existing settings.json
  - [ ] `TestWrite_CreatesDir` — `.claude/` created when absent
- [ ] `cmd/new_test.go`
  - [ ] `TestRunNew_SendsClaudeKeys` — verify `SendKeys` called with `"claude"` after window creation

---

## Phase 3 — Passive monitoring

### `cmd/bar.go` (new command)

- [ ] `exocortex bar` — reads registry, prints tmux-formatted status fragment
- [ ] Output when agents waiting: `#[fg=yellow] 2 waiting #[default]`
- [ ] Output when none waiting: empty string (disappears from bar cleanly)
- [ ] Must be fast — tmux polls this on its refresh interval (default 15s)

### `cmd/init.go` (new command)

- [ ] `exocortex init` — prints recommended tmux.conf additions to stdout
- [ ] Snippet includes:
  - Popup binding: `bind-key e display-popup -w 80% -h 80% -E "exocortex"`
  - Status-right entry: `#(exocortex bar)`
- [ ] User pipes to their config or copies manually
- [ ] Add note: run `tmux source ~/.tmux.conf` to apply

### Tests

- [ ] `cmd/bar_test.go`
  - [ ] `TestBar_NoWaiting` — empty output
  - [ ] `TestBar_OneWaiting` — contains "1 waiting"
  - [ ] `TestBar_MultipleWaiting` — correct count

---

## Phase 4 — Auto-detection of current agent

Lets `nvim` and `goto` work with no ID argument when called from inside an agent's pane.

### `cmd/detect.go` (new internal helper, not a command)

- [ ] `detectAgentID(reg registrySvc) (string, error)`
- [ ] Reads `$TMUX_PANE` from environment
- [ ] Loads registry, scans `TmuxTarget` for a match
- [ ] Returns agent ID or error if not found / not in tmux

### `cmd/nvim.go`

- [ ] Change `cobra.ExactArgs(1)` → `cobra.MaximumNArgs(1)`
- [ ] If no arg, call `detectAgentID` to resolve current pane
- [ ] Error clearly if neither arg nor detectable pane

### `cmd/goto.go`

- [ ] Same: `cobra.MaximumNArgs(1)`, fallback to `detectAgentID`

### Tests

- [ ] `cmd/detect_test.go`
  - [ ] `TestDetectAgentID_Match` — env set to matching pane target
  - [ ] `TestDetectAgentID_NoMatch` — env set to unknown pane
  - [ ] `TestDetectAgentID_NoEnv` — `$TMUX_PANE` not set
- [ ] `cmd/nvim_test.go` — `TestRunNvim_NoArg_UsesCurrentPane`
- [ ] `cmd/goto_test.go` — `TestRunGoto_NoArg_UsesCurrentPane`

---

## Phase 5 — Respawn

Recovers agents after tmux restart or accidental window close.

### `cmd/respawn.go` (new command)

- [ ] `exocortex respawn <id>`
- [ ] Loads agent from registry
- [ ] Checks `tm.WindowExists(agent.TmuxTarget)` — if alive, prints "already running" and exits
- [ ] `tm.NewWindow(agent.WorktreePath, id)` → new pane target
- [ ] `tm.SendKeys(target, "claude")` — restart ClaudeCode
- [ ] `reg.UpdateTmuxTarget(id, target)` — persist new target
- [ ] If `agent.NvimTarget != ""`, clear it (the nvim window is also gone)

### `internal/registry/registry.go`

- [ ] Add `UpdateTmuxTarget(path, id, target string) error` — same pattern as `UpdateNvimTarget`

### `cmd/registry_adapter.go`

- [ ] Bind `registry.UpdateTmuxTarget`

### `cmd/services.go`

- [ ] Add `UpdateTmuxTarget(id, target string) error` to `registrySvc`

### Tests

- [ ] `cmd/respawn_test.go`
  - [ ] `TestRespawn_WindowGone_CreatesNew` — WindowExists false → new window created
  - [ ] `TestRespawn_WindowAlive_DoesNothing` — WindowExists true → no new window
  - [ ] `TestRespawn_UpdatesTmuxTarget` — registry updated with new target
  - [ ] `TestRespawn_ClearsNvimTarget` — NvimTarget cleared when respawning
  - [ ] `TestRespawn_UnknownID`

---

## Phase 6 — Finish workflow

Closes the loop: code written → branch pushed → PR opened → agent removed.

### `cmd/finish.go` (new command)

- [ ] `exocortex finish <id> [--no-pr]`
- [ ] Loads agent from registry
- [ ] Checks for uncommitted changes via `git status --porcelain` — warns but continues
- [ ] `git.PushBranch(repoPath, branch)` — push to origin
- [ ] Unless `--no-pr`: runs `gh pr create --fill` (uses branch name + task description)
- [ ] Prints PR URL
- [ ] Calls `executeRemove` to clean up windows, worktree, registry

### `internal/git/git.go`

- [ ] Add `PushBranch(repoPath, branch string) error` — runs `git push -u origin <branch>`
- [ ] Add `HasUncommittedChanges(worktreePath string) (bool, error)` — runs `git status --porcelain`

### `gitSvc` interface (`cmd/services.go`)

- [ ] Add `PushBranch(repoPath, branch string) error`
- [ ] Add `HasUncommittedChanges(worktreePath string) (bool, error)`

### Tests

- [ ] `internal/git/git_test.go`
  - [ ] `TestPushBranch_Args`
  - [ ] `TestHasUncommittedChanges_Clean`
  - [ ] `TestHasUncommittedChanges_Dirty`
- [ ] `cmd/finish_test.go`
  - [ ] `TestFinish_PushesBranch`
  - [ ] `TestFinish_RemovesAgent`
  - [ ] `TestFinish_WarnsDirtyTree` — succeeds with warning, does not abort
  - [ ] `TestFinish_NoPR_SkipsPR` — `--no-pr` flag skips `gh`
  - [ ] `TestFinish_UnknownID`

---

## Phase 7 — TUI polish

### `internal/ui/keys.go`

- [ ] `Goto` help: `"goto ClaudeCode"`
- [ ] `Nvim` help: `"open nvim"`

### `internal/ui/model.go`

- [ ] Add `Finish` binding (e.g. `f`) → confirm dialog → calls `FinishAgent`
- [ ] Add `Respawn` binding (e.g. `R`) → calls `RespawnAgent`
- [ ] `Services` struct: add `FinishAgent func(id string) error`, `RespawnAgent func(id string) error`

### `cmd/ui.go`

- [ ] Wire `FinishAgent` and `RespawnAgent` into `buildServices`

### `internal/ui/model_test.go`

- [ ] Tests for `f` key (confirm dialog), `R` key (immediate action)

---

## Fake/stub updates (all test files)

Every time a new method lands on `tmuxSvc`, `registrySvc`, or `gitSvc`, add a
no-op stub to every fake in the test suite. Checklist:

- [ ] `fakeTmux` (`cmd/new_test.go`) — `SendKeys` ✓, `WindowExists` ✓
- [ ] `fakeTmuxRemove` (`cmd/remove_test.go`) — `SendKeys` ✓, `WindowExists` ✓
- [ ] `fakeTmuxGoto` (`cmd/goto_test.go`) — `SendKeys` ✓, `WindowExists` ✓
- [ ] `fakeTmuxNvim` (`cmd/nvim_test.go`) — full ✓
- [ ] All `registrySvc` fakes — `UpdateNvimTarget` ✓, `UpdateTmuxTarget` pending
- [ ] All `gitSvc` fakes — `PushBranch` pending, `HasUncommittedChanges` pending

---

## Dependency order for implementation

```
Phase 1 (status cmd)
  └── Phase 2 (hooks in new, auto-start)
        └── Phase 3 (bar, init)
Phase 4 (auto-detect) — independent
Phase 5 (respawn)     — needs UpdateTmuxTarget (Phase 1 pattern)
Phase 6 (finish)      — needs PushBranch, calls executeRemove
Phase 7 (TUI polish)  — needs Finish + Respawn wired
```

Phases 1–3 are the highest-value path: status becomes automatic, the bar lights
up, and the core loop (create → work → wait → reply → done) is fully visible.

Phases 4–5 reduce friction. Phase 6 closes the loop. Phase 7 is polish.
