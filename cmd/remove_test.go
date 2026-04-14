package cmd

import (
	"errors"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryRemove struct {
	nuclei    []registry.Nucleus
	deletedID string
	deleteErr error
}

func (f *fakeRegistryRemove) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryRemove) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryRemove) Delete(id string) error {
	f.deletedID = id
	return f.deleteErr
}
func (f *fakeRegistryRemove) UpdateStatus(id, status string) error                    { return nil }
func (f *fakeRegistryRemove) AddNeuron(nucleusID string, neuron registry.Neuron) error { return nil }
func (f *fakeRegistryRemove) RemoveNeuron(nucleusID, neuronID string) error            { return nil }
func (f *fakeRegistryRemove) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	return nil
}

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
func (g *fakeGitRemove) AheadCommits(worktreePath string) ([]string, error)  { return nil, nil }
func (g *fakeGitRemove) ListBranches(repoPath string) ([]string, error)      { return nil, nil }

type fakeTmuxRemove struct {
	killCalled    bool
	killErr       error
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

func TestRunRemove_KillsAllNeuronPanes(t *testing.T) {
	reg := &fakeRegistryRemove{nuclei: []registry.Nucleus{
		{ID: "abc123", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
			}},
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

func TestRunRemove_KillsAllNeurons(t *testing.T) {
	reg := &fakeRegistryRemove{nuclei: []registry.Nucleus{
		{ID: "abc123", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
				{ID: "nvim", Type: registry.NeuronNvim, TmuxTarget: "main:2.0"},
			}},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	err := executeRemove("abc123", reg, gt, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tm.killedTargets) != 2 {
		t.Fatalf("expected 2 panes killed, got %d: %v", len(tm.killedTargets), tm.killedTargets)
	}
}

func TestRunRemove_RemovesWorktree(t *testing.T) {
	reg := &fakeRegistryRemove{nuclei: []registry.Nucleus{
		{ID: "abc123", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
			}},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{}

	_ = executeRemove("abc123", reg, gt, tm)
	if !gt.removeCalled {
		t.Fatal("expected git.RemoveWorktree to be called")
	}
}

func TestRunRemove_DeletesFromRegistry(t *testing.T) {
	reg := &fakeRegistryRemove{nuclei: []registry.Nucleus{
		{ID: "abc123", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
			}},
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

func TestRunRemove_TmuxError_StillRemovesWorktreeAndRegistry(t *testing.T) {
	reg := &fakeRegistryRemove{nuclei: []registry.Nucleus{
		{ID: "abc123", RepoPath: "/repo", WorktreePath: "/repo/.worktrees/abc123",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
			}},
	}}
	gt := &fakeGitRemove{}
	tm := &fakeTmuxRemove{killErr: errors.New("no such pane")}

	err := executeRemove("abc123", reg, gt, tm)
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
