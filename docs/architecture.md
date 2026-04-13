# exocortex — Architecture

## Philosophy

Agents are background processes. exocortex does not orchestrate their logic; it routes your attention.
The binary is a thin shell: parse → delegate → side-effect → report. No magic, no hidden state beyond the registry file.

---

## Layer Map

```
┌─────────────────────────────────────────┐
│  cmd/          (Cobra — input & routing) │
│  Parse flags · validate args · call svc  │
├─────────────────────────────────────────┤
│  internal/registry   (state)            │
│  Read / write ~/.config/exocortex/      │
│  registry.json atomically               │
├──────────────────┬──────────────────────┤
│  internal/tmux   │  internal/git        │
│  os/exec wrappers│  os/exec wrappers    │
└──────────────────┴──────────────────────┘
```

**Rule:** cmd/ never touches os/exec directly. internal/ packages never import each other.

---

## Directory Structure

```
exocortex/
├── main.go
├── cmd/
│   ├── root.go       # global flags, pre-run tmux guard
│   ├── new.go
│   ├── list.go
│   ├── goto.go
│   ├── nvim.go
│   └── remove.go
├── internal/
│   ├── registry/
│   │   ├── registry.go       # Load / Save / CRUD
│   │   └── registry_test.go
│   ├── tmux/
│   │   ├── tmux.go           # SplitWindow, SelectPane, KillPane
│   │   └── tmux_test.go
│   └── git/
│       ├── git.go            # AddWorktree, RemoveWorktree, ModifiedFiles
│       └── git_test.go
├── go.mod
└── docs/
```

---

## Data Model

Source of truth: `~/.config/exocortex/registry.json`

```go
type Registry struct {
    Agents []Agent `json:"agents"`
}

type Agent struct {
    ID              string    `json:"id"`               // 6-char slug from task name
    RepoPath        string    `json:"repo_path"`        // abs path to root repo
    WorktreePath    string    `json:"worktree_path"`    // abs path to worktree
    Branch          string    `json:"branch"`           // git branch name
    TaskDescription string    `json:"task_description"`
    TmuxTarget      string    `json:"tmux_target"`      // "session:window.pane"
    Status          string    `json:"status"`           // "working" | "idle" | "blocked"
    CreatedAt       time.Time `json:"created_at"`
    LastFile        string    `json:"last_file,omitempty"`
}
```

**Write strategy:** load → mutate in memory → write to temp file → `os.Rename` (atomic swap). Never write to the registry file directly to avoid corruption on crash.

---

## Command Contracts

| Command | Input | Side effects | Output |
|---|---|---|---|
| `new` | `--repo`, `--task`, `--branch?` | git worktree, tmux pane, registry append | prints agent ID |
| `list` | — | none | tabwriter table |
| `goto <id>` | agent ID | tmux select-pane | — |
| `nvim <id>` | agent ID | chdir, syscall.Exec → nvim | replaces process |
| `remove <id>` | agent ID | tmux kill-pane, git worktree remove, registry delete | — |

---

## Key Constraints

- **`nvim` must use `syscall.Exec`** — replaces the process so no orphaned Go process sits behind Neovim.
- **`new` must check for an existing tmux session** before splitting (`root.go` pre-run guard). Error early if not in tmux.
- **IDs are generated from the task name** (lowercase, strip non-alpha, truncate to 6 chars + collision suffix if needed), not UUIDs — they need to be typeable.
- **No global state** in `internal/` packages; functions are pure wrappers that accept paths/targets as arguments. This is what makes them testable.

---

## Testing Strategy

| Layer | What to test | How |
|---|---|---|
| `internal/registry` | Load/Save round-trip, Add/Delete, atomic write, missing dir auto-create | `os.TempDir()` + real file I/O |
| `internal/git` | Correct shell command construction | capture `exec.Cmd` args; stub out execution via injectable runner |
| `internal/tmux` | Correct shell command construction | same injectable runner pattern |
| `cmd/` | Flag parsing, error paths, correct delegation | `cobra.Command.Execute()` with mocked internal packages via interfaces |

Use interfaces at the boundary between cmd/ and internal/ so commands can be tested without real tmux or git:

```go
// internal/tmux/tmux.go
type Tmux interface {
    SplitWindow(workdir string) (target string, err error)
    SelectPane(target string) error
    KillPane(target string) error
}
```

Same pattern for `Git` and `Registry`.
