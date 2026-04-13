package cmd

import (
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeGitNvim struct {
	files []string
}

func (g *fakeGitNvim) AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error {
	return nil
}
func (g *fakeGitNvim) RemoveWorktree(repoPath, worktreePath string) error { return nil }
func (g *fakeGitNvim) ModifiedFiles(worktreePath string) ([]string, error) {
	return g.files, nil
}
func (g *fakeGitNvim) BranchExists(repoPath, branch string) (bool, error)        { return false, nil }

type fakeRegistryNvim struct {
	agents         []registry.Agent
	nvimTargetID   string
	nvimTargetVal  string
}

func (f *fakeRegistryNvim) Load() (*registry.Registry, error) {
	return &registry.Registry{Agents: f.agents}, nil
}
func (f *fakeRegistryNvim) Add(a registry.Agent) error { return nil }
func (f *fakeRegistryNvim) Delete(id string) error     { return nil }
func (f *fakeRegistryNvim) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryNvim) UpdateNvimTarget(id, target string) error {
	f.nvimTargetID = id
	f.nvimTargetVal = target
	return nil
}
func (f *fakeRegistryNvim) UpdateTmuxTarget(id, target string) error { return nil }

type fakeTmuxNvim struct {
	newWindowName  string
	newWindowErr   error
	sentTarget     string
	sentKeys       string
	selectedTarget string
	windowExists   bool
	killCalled     bool
}

func (t *fakeTmuxNvim) NewWindow(workdir, name string) (string, error) {
	t.newWindowName = name
	return "main:2.0", t.newWindowErr
}
func (t *fakeTmuxNvim) SelectPane(target string) error {
	t.selectedTarget = target
	return nil
}
func (t *fakeTmuxNvim) KillPane(target string) error {
	t.killCalled = true
	return nil
}
func (t *fakeTmuxNvim) SendKeys(target, keys string) error {
	t.sentTarget = target
	t.sentKeys = keys
	return nil
}
func (t *fakeTmuxNvim) WindowExists(target string) (bool, error) {
	return t.windowExists, nil
}
func (t *fakeTmuxNvim) CurrentTarget() (string, error)            { return "", nil }
func (t *fakeTmuxNvim) CapturePane(target string) (string, error) { return "", nil }

func TestRunNvim_UsesModifiedFile(t *testing.T) {
	reg := &fakeRegistryNvim{agents: []registry.Agent{
		{ID: "abc123", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitNvim{files: []string{"main.go", "other.go"}}
	tm := &fakeTmuxNvim{}

	err := executeNvim("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.newWindowName != "abc123-DEV" {
		t.Fatalf("expected window name abc123-DEV, got %q", tm.newWindowName)
	}
	if tm.sentKeys != "nvim main.go" {
		t.Fatalf("expected keys 'nvim main.go', got %q", tm.sentKeys)
	}
}

func TestRunNvim_FallsBackToWorktreeRoot(t *testing.T) {
	reg := &fakeRegistryNvim{agents: []registry.Agent{
		{ID: "abc123", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitNvim{files: []string{}} // no modified files
	tm := &fakeTmuxNvim{}

	err := executeNvim("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.sentKeys != "nvim ." {
		t.Fatalf("expected keys 'nvim .', got %q", tm.sentKeys)
	}
}

func TestRunNvim_ExistingWindow_SelectsInsteadOfCreating(t *testing.T) {
	reg := &fakeRegistryNvim{agents: []registry.Agent{
		{ID: "abc123", WorktreePath: "/repo/.worktrees/abc123", NvimTarget: "main:3.0"},
	}}
	gt := &fakeGitNvim{files: []string{"main.go"}}
	tm := &fakeTmuxNvim{windowExists: true}

	err := executeNvim("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.selectedTarget != "main:3.0" {
		t.Fatalf("expected SelectPane called with main:3.0, got %q", tm.selectedTarget)
	}
	if tm.newWindowName != "" {
		t.Fatal("expected no new window to be created when existing window is alive")
	}
	if tm.sentKeys != "" {
		t.Fatal("expected no SendKeys when reusing existing window")
	}
}

func TestRunNvim_SavesNvimTarget(t *testing.T) {
	reg := &fakeRegistryNvim{agents: []registry.Agent{
		{ID: "abc123", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	gt := &fakeGitNvim{files: []string{"main.go"}}
	tm := &fakeTmuxNvim{}

	err := executeNvim("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.nvimTargetID != "abc123" {
		t.Fatalf("expected UpdateNvimTarget called for abc123, got %q", reg.nvimTargetID)
	}
	if reg.nvimTargetVal != "main:2.0" {
		t.Fatalf("expected nvim target main:2.0, got %q", reg.nvimTargetVal)
	}
}

func TestRunNvim_UnknownID(t *testing.T) {
	reg := &fakeRegistryNvim{}
	gt := &fakeGitNvim{}
	tm := &fakeTmuxNvim{}
	err := executeNvim("nope", reg, gt, tm)
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}
