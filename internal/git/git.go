package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts shell execution so callers can inject a fake in tests.
type Runner interface {
	Run(cmd string, args ...string) (string, error)
}

// ExecRunner is the real Runner that calls os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", fmt.Errorf("git: %w", err)
	}
	return string(out), nil
}

// Git wraps git operations via an injectable Runner.
type Git struct {
	runner Runner
}

// New returns a Git using the provided runner.
// Pass git.ExecRunner{} for real execution.
func New(r Runner) *Git {
	return &Git{runner: r}
}

// AddWorktree creates a git worktree at worktreePath on branch.
// If createBranch is true, the branch is created with -b; otherwise it is checked out directly.
func (g *Git) AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error {
	var args []string
	if createBranch {
		args = []string{"-C", repoPath, "worktree", "add", "-b", branch, worktreePath}
	} else {
		args = []string{"-C", repoPath, "worktree", "add", worktreePath, branch}
	}
	_, err := g.runner.Run("git", args...)
	return err
}

// RemoveWorktree removes the worktree at worktreePath, force-removing it even if dirty.
func (g *Git) RemoveWorktree(repoPath, worktreePath string) error {
	_, err := g.runner.Run("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath)
	return err
}

// BranchExists returns true if the named branch exists in the repo.
func (g *Git) BranchExists(repoPath, branch string) (bool, error) {
	out, err := g.runner.Run("git", "-C", repoPath, "branch", "--list", branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// PushBranch pushes the branch to origin, setting upstream tracking.
func (g *Git) PushBranch(repoPath, branch string) error {
	_, err := g.runner.Run("git", "-C", repoPath, "push", "-u", "origin", branch)
	return err
}

// HasUncommittedChanges returns true if the worktree has uncommitted changes.
func (g *Git) HasUncommittedChanges(worktreePath string) (bool, error) {
	out, err := g.runner.Run("git", "-C", worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// ModifiedFiles returns the list of modified (unstaged) files in the worktree.
// Returns an empty slice if there are none.
func (g *Git) ModifiedFiles(worktreePath string) ([]string, error) {
	out, err := g.runner.Run("git", "-C", worktreePath, "ls-files", "-m")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(out, "\n"), nil
}
