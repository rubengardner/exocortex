package cmd

import (
	"errors"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryRemove struct {
	agents    []registry.Agent
	deletedID string
	deleteErr error
}

func (f *fakeRegistryRemove) Load() (*registry.Registry, error) {
	return &registry.Registry{Agents: f.agents}, nil
}
func (f *fakeRegistryRemove) Add(a registry.Agent) error { return nil }
func (f *fakeRegistryRemove) Delete(id string) error {
	f.deletedID = id
	return f.deleteErr
}
func (f *fakeRegistryRemove) UpdateStatus(id, status string) error     { return nil }
func (f *fakeRegistryRemove) UpdateNvimTarget(id, target string) error { return nil }
func (f *fakeRegistryRemove) UpdateTmuxTarget(id, target string) error { return nil }

type fakeGitRemove struct {
	removeCalled bool
	removeErr    error
}

func (g *fakeGitRemove) AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error {
	return nil
}
func (g *fakeGitRemove) RemoveWorktree(repoPath, worktreePath string) error {
	g.removeCalled = true
	return g.removeErr
}
func (g *fakeGitRemove) ModifiedFiles(worktreePath string) ([]string, error) { return nil, nil }
func (g *fakeGitRemove) BranchExists(repoPath, branch string) (bool, error)  { return false, nil }

type fakeTmuxRemove struct {
	killCalled   bool
	killErr      error
	killedTargets []string
}

func (t *fakeTmuxRemove) NewWindow(workdir, name string) (string, error) { return "", nil }
func (t *fakeTmuxRemove) SelectPane(target string) error                 { return nil }
func (t *fakeTmuxRemove) KillPane(target string) error {
	t.killCalled = true
	t.killedTargets = append(t.killedTargets, target)
	return t.killErr
}
func (t *fakeTmuxRemove) SendKeys(target, keys string) error             { return nil }
func (t *fakeTmuxRemove) WindowExists(target string) (bool, error)       { return false, nil }
func (t *fakeTmuxRemove) CurrentTarget() (string, error)                 { return "", nil }
func (t *fakeTmuxRemove) CapturePane(target string) (string, error)      { return "", nil }

func TestRunRemove_KillsPane(t *testing.T) {
	reg := &fakeRegistryRemove{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.2", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	err := executeRemove("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tm.killCalled {
		t.Fatal("expected tmux.KillPane to be called")
	}
}

func TestRunRemove_RemovesWorktree(t *testing.T) {
	reg := &fakeRegistryRemove{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.2", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	_ = executeRemove("abc123", reg, gt, tm)
	if !gt.removeCalled {
		t.Fatal("expected git.RemoveWorktree to be called")
	}
}

func TestRunRemove_DeletesFromRegistry(t *testing.T) {
	reg := &fakeRegistryRemove{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.2", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	_ = executeRemove("abc123", reg, gt, tm)
	if reg.deletedID != "abc123" {
		t.Fatalf("expected abc123 to be deleted from registry, got %q", reg.deletedID)
	}
}

func TestRunRemove_UnknownID(t *testing.T) {
	reg := &fakeRegistryRemove{}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	err := executeRemove("nope", reg, gt, tm)
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestRunRemove_KillsNvimWindow(t *testing.T) {
	reg := &fakeRegistryRemove{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.2", NvimTarget: "main:2.0", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	err := executeRemove("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, target := range tm.killedTargets {
		if target == "main:2.0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected main:2.0 (NvimTarget) to be killed, killed: %v", tm.killedTargets)
	}
}

func TestRunRemove_TmuxError_StillRemovesWorktreeAndRegistry(t *testing.T) {
	// tmux pane may already be gone; we should still clean up git and registry.
	reg := &fakeRegistryRemove{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.2", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{killErr: errors.New("no such pane")}

	err := executeRemove("abc123", reg, gt, tm)
	// Should succeed with a warning, not a hard failure.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gt.removeCalled {
		t.Fatal("git worktree remove should still be called")
	}
	if reg.deletedID != "abc123" {
		t.Fatal("registry delete should still be called")
	}
}
