package cmd

// These interfaces let cmd/ stay decoupled from internal packages
// and make every command testable without real git/tmux/disk I/O.

import "github.com/ruben_gardner/exocortex/internal/registry"

type gitSvc interface {
	AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error
	RemoveWorktree(repoPath, worktreePath string) error
	ModifiedFiles(worktreePath string) ([]string, error)
	BranchExists(repoPath, branch string) (bool, error)
}

type tmuxSvc interface {
	NewWindow(workdir, name string) (string, error)
	SelectPane(target string) error
	KillPane(target string) error
	SendKeys(target, keys string) error
	WindowExists(target string) (bool, error)
	CurrentTarget() (string, error)
	CapturePane(target string) (string, error)
}

type registrySvc interface {
	Load() (*registry.Registry, error)
	Add(a registry.Agent) error
	Delete(id string) error
	UpdateStatus(id, status string) error
	UpdateNvimTarget(id, target string) error
	UpdateTmuxTarget(id, target string) error
}
