# exocortex

A CLI tool for managing multiple AI coding agents running in parallel across a repository. Built around a simple philosophy: **agents are just background processes — exocortex routes your attention, not their logic.**

Sub-second context switching between parallel agent tasks using tmux and Neovim.

---

## Requirements

- [Go](https://go.dev) 1.24+
- [tmux](https://github.com/tmux/tmux) — exocortex must be run inside a tmux session
- [git](https://git-scm.com) 2.5+ — uses git worktrees
- [Neovim](https://neovim.io) — for the `nvim` command and `e` key in the TUI

---

## Installation

```sh
git clone https://github.com/ruben_gardner/exocortex
cd exocortex
go build -o ~/bin/exocortex .
```

Make sure `~/bin` is on your `$PATH`.

---

## Quick start

All commands must be run inside a tmux session.

```sh
# Launch the full-screen TUI (recommended)
exocortex

# Or use individual CLI commands
exocortex new --task "refactor auth middleware"
exocortex list
exocortex goto <id>
exocortex nvim <id>
exocortex remove <id>
```

---

## The TUI

Running `exocortex` with no arguments opens a full-screen interface — the recommended way to use the tool day-to-day.

```
◈  EXOCORTEX                                        2 agent(s)
│ ● refact   idle  │  refact
│   Refactor auth  │  ──────────────────────────────────────
│   3h             │  Task        Refactor auth middleware
│                  │  Branch      agent/refact
│ ● fixbug  working│  Status      ● idle
│   Fix login bug  │  Tmux        main:1.2
│   12m            │  Worktree    /repo/.worktrees/refact
│                  │  Created     3h ago
│                  │
│                  │  ── Actions ─────────────────────────
│                  │  g  jump to tmux pane
│                  │  e  open in neovim
│                  │  s  cycle status
│                  │  d  remove agent
──────────────────────────────────────────────────────────────
↑/k ↓/j  g goto  e nvim  n new  d remove  s status  q quit
```

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down the agent list |
| `k` / `↑` | Move up the agent list |
| `g` | Jump to the selected agent's tmux pane |
| `e` | Open the selected agent's worktree in Neovim |
| `n` | Create a new agent (opens form overlay) |
| `d` | Remove the selected agent (asks for confirmation) |
| `s` | Cycle agent status: idle → working → blocked → idle |
| `r` | Refresh the agent list from disk |
| `?` | Show full keybindings help |
| `q` | Quit |

### Creating a new agent

Press `n` to open the new agent form. Fill in:

- **Task** (required) — a short description, e.g. `fix login redirect bug`
- **Branch** (optional) — defaults to `agent/<id>` if left blank

Press `tab` to switch between fields, `enter` to create, `esc` to cancel.

### Opening in Neovim

Press `e` to open the agent's worktree in Neovim. The TUI exits cleanly and Neovim replaces the process — no leftover processes in the background. Neovim opens on the first modified file in the worktree, or the worktree root if there are no modifications.

When you quit Neovim you return to your shell. Run `exocortex` again to reopen the TUI.

### Removing an agent

Press `d` to remove the selected agent. A confirmation dialog appears — press `y` to confirm or any other key to cancel. Removal:

1. Kills the tmux pane
2. Removes the git worktree (`git worktree remove --force`)
3. Deletes the agent from the registry

If the tmux pane or worktree is already gone, removal continues with a warning rather than failing.

---

## CLI commands

The individual commands exist for scripting and shell workflows. All require an active tmux session (except `nvim`).

### `exocortex new`

Creates a new agent: a git worktree, a tmux pane, and a registry entry.

```sh
exocortex new --task "fix login redirect bug"
exocortex new --task "refactor auth" --branch feat/auth-refactor
exocortex new --task "add tests" --repo /path/to/other/repo
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--task` | *(required)* | Task description |
| `--branch` | `agent/<id>` | Branch name. Created if it doesn't exist; checked out if it does |
| `--repo` | current directory | Path to the git repository |

The agent ID is derived from the task description (lowercased, alphanumeric, 6 characters). A numeric suffix is appended if there is a collision.

### `exocortex list`

Prints a table of all active agents.

```sh
exocortex list
```

```
ID      BRANCH         TASK                    STATUS   TMUX TARGET
--      ------         ----                    ------   -----------
refact  agent/refact   Refactor auth middleware idle     main:1.2
fixbug  feat/fixbug    Fix login redirect bug  working  main:1.3
```

### `exocortex goto <id>`

Switches tmux focus to the agent's pane.

```sh
exocortex goto refact
```

### `exocortex nvim <id>`

Opens the agent's worktree in Neovim, replacing the current process. Neovim opens on the first modified file, or `.` if there are none.

```sh
exocortex nvim refact
```

### `exocortex remove <id>`

Removes an agent: kills its tmux pane, removes its git worktree, and deletes it from the registry.

```sh
exocortex remove refact
```

---

## How it works

### State

All state lives in `~/.config/exocortex/registry.json`. The file is written atomically (temp file + rename) so a crash never corrupts it. The directory is created automatically on first use.

### Git worktrees

Each agent gets its own [git worktree](https://git-scm.com/docs/git-worktree) at `<repo>/.worktrees/<id>`. This means multiple agents can work on different branches of the same repository simultaneously without interfering with each other. The worktrees directory is inside the repo — add `.worktrees/` to your `.gitignore`.

```sh
echo '.worktrees/' >> .gitignore
```

### Tmux panes

Each agent gets a dedicated tmux pane (horizontal split in the current window). The pane target (`session:window.pane`) is stored in the registry so exocortex can find and switch to it later.

### Agent status

Status is a manual label — `idle`, `working`, or `blocked`. exocortex does not poll agents or auto-detect status. Update it with `s` in the TUI to give yourself a quick visual signal of where things stand.

---

## Project structure

```
exocortex/
├── main.go
├── cmd/                    # Cobra CLI commands + TUI launcher
│   ├── root.go             # Root command, tmux guard, TUI entry point
│   ├── new.go
│   ├── list.go
│   ├── goto.go
│   ├── nvim.go
│   ├── remove.go
│   ├── ui.go               # Wires real services into the TUI
│   └── services.go         # Internal service interfaces
├── internal/
│   ├── registry/           # JSON state: load, save, add, delete, update
│   ├── git/                # git worktree wrappers
│   ├── tmux/               # tmux pane wrappers
│   └── ui/                 # Bubble Tea TUI (model, styles, keys)
└── docs/
    ├── architecture.md
    └── backlog.md
```
