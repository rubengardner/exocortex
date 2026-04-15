package cmd

import (
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// ── nextNeuronID ──────────────────────────────────────────────────────────────

func TestNextNeuronID_ClaudeNoExisting(t *testing.T) {
	id := nextNeuronID(nil, "claude")
	if id != "c1" {
		t.Fatalf("expected c1, got %s", id)
	}
}

func TestNextNeuronID_ClaudeOneExisting(t *testing.T) {
	neurons := []registry.Neuron{{ID: "c1", Type: registry.NeuronClaude}}
	id := nextNeuronID(neurons, "claude")
	if id != "c2" {
		t.Fatalf("expected c2, got %s", id)
	}
}

func TestNextNeuronID_ShellNoExisting(t *testing.T) {
	id := nextNeuronID(nil, "shell")
	if id != "sh1" {
		t.Fatalf("expected sh1, got %s", id)
	}
}

func TestNextNeuronID_NvimExistingNoNumber(t *testing.T) {
	// Old nvim neuron has ID "nvim" (no number suffix from migration).
	neurons := []registry.Neuron{{ID: "nvim", Type: registry.NeuronNvim}}
	id := nextNeuronID(neurons, "nvim")
	if id != "nvim2" {
		t.Fatalf("expected nvim2, got %s", id)
	}
}

func TestNextNeuronID_MultipleShell(t *testing.T) {
	neurons := []registry.Neuron{
		{ID: "sh1", Type: registry.NeuronShell},
		{ID: "sh2", Type: registry.NeuronShell},
	}
	id := nextNeuronID(neurons, "shell")
	if id != "sh3" {
		t.Fatalf("expected sh3, got %s", id)
	}
}

// ── executeAddNeuron ──────────────────────────────────────────────────────────

type fakeRegistryNeuronAdd struct {
	nuclei      []registry.Nucleus
	addedNeuron registry.Neuron
	addedTo     string
}

func (f *fakeRegistryNeuronAdd) Load() (*registry.Registry, error) {
	return &registry.Registry{Version: 2, Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryNeuronAdd) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryNeuronAdd) Delete(id string) error        { return nil }
func (f *fakeRegistryNeuronAdd) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryNeuronAdd) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	f.addedTo = nucleusID
	f.addedNeuron = neuron
	return nil
}
func (f *fakeRegistryNeuronAdd) RemoveNeuron(nucleusID, neuronID string) error  { return nil }
func (f *fakeRegistryNeuronAdd) UpdateNeuronTarget(nID, neuID, target string) error { return nil }

type fakeTmuxNeuronAdd struct {
	newWindowTarget string
	newWindowDir    string
	sentKeys        string
}

func (f *fakeTmuxNeuronAdd) NewWindow(workdir, name string) (string, error) {
	f.newWindowDir = workdir
	f.newWindowTarget = "main:99.0"
	return f.newWindowTarget, nil
}
func (f *fakeTmuxNeuronAdd) SelectPane(target string) error                { return nil }
func (f *fakeTmuxNeuronAdd) KillPane(target string) error                  { return nil }
func (f *fakeTmuxNeuronAdd) SendKeys(target, keys string) error            { f.sentKeys = keys; return nil }
func (f *fakeTmuxNeuronAdd) WindowExists(target string) (bool, error)      { return true, nil }
func (f *fakeTmuxNeuronAdd) CurrentTarget() (string, error)                { return "main:1.0", nil }
func (f *fakeTmuxNeuronAdd) CapturePane(target string) (string, error)     { return "", nil }

func TestExecuteAddNeuron_Claude(t *testing.T) {
	reg := &fakeRegistryNeuronAdd{
		nuclei: []registry.Nucleus{
			{
				ID:           "nucl1",
				WorktreePath: "/repo/.worktrees/nucl1",
				Neurons: []registry.Neuron{
					{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0"},
				},
			},
		},
	}
	tm := &fakeTmuxNeuronAdd{}

	err := executeAddNeuron("nucl1", "claude", "", reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.addedTo != "nucl1" {
		t.Fatalf("expected neuron added to nucl1, got %q", reg.addedTo)
	}
	if reg.addedNeuron.ID != "c2" {
		t.Fatalf("expected ID c2, got %q", reg.addedNeuron.ID)
	}
	if reg.addedNeuron.Type != registry.NeuronClaude {
		t.Fatalf("expected claude type, got %q", reg.addedNeuron.Type)
	}
	if tm.sentKeys != "claude" {
		t.Fatalf("expected 'claude' sent to pane, got %q", tm.sentKeys)
	}
}

func TestExecuteAddNeuron_Shell(t *testing.T) {
	reg := &fakeRegistryNeuronAdd{
		nuclei: []registry.Nucleus{
			{
				ID:           "nucl1",
				WorktreePath: "/repo/.worktrees/nucl1",
				Neurons:      []registry.Neuron{{ID: "c1", Type: registry.NeuronClaude}},
			},
		},
	}
	tm := &fakeTmuxNeuronAdd{}

	err := executeAddNeuron("nucl1", "shell", "", reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.addedNeuron.ID != "sh1" {
		t.Fatalf("expected sh1, got %q", reg.addedNeuron.ID)
	}
	if reg.addedNeuron.Type != registry.NeuronShell {
		t.Fatalf("expected shell type, got %q", reg.addedNeuron.Type)
	}
	// No command sent for shell.
	if tm.sentKeys != "" {
		t.Fatalf("expected no keys sent for shell, got %q", tm.sentKeys)
	}
}

func TestExecuteAddNeuron_WithClaudeConfigDir(t *testing.T) {
	reg := &fakeRegistryNeuronAdd{
		nuclei: []registry.Nucleus{
			{
				ID:           "nucl1",
				WorktreePath: "/repo/.worktrees/nucl1",
				Neurons:      []registry.Neuron{{ID: "c1", Type: registry.NeuronClaude}},
			},
		},
	}
	tm := &fakeTmuxNeuronAdd{}

	err := executeAddNeuron("nucl1", "claude", "~/.claude-work", reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "CLAUDE_CONFIG_DIR=~/.claude-work claude"
	if tm.sentKeys != want {
		t.Fatalf("expected %q, got %q", want, tm.sentKeys)
	}
	if reg.addedNeuron.Profile != "~/.claude-work" {
		t.Fatalf("expected profile stored, got %q", reg.addedNeuron.Profile)
	}
}

func TestExecuteAddNeuron_UsesWorktreePathWhenSet(t *testing.T) {
	reg := &fakeRegistryNeuronAdd{
		nuclei: []registry.Nucleus{
			{
				ID:           "nucl1",
				RepoPath:     "/repo",
				WorktreePath: "/repo/.worktrees/nucl1",
				Neurons:      []registry.Neuron{{ID: "c1", Type: registry.NeuronClaude}},
			},
		},
	}
	tm := &fakeTmuxNeuronAdd{}

	if err := executeAddNeuron("nucl1", "shell", "", reg, tm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.newWindowDir != "/repo/.worktrees/nucl1" {
		t.Fatalf("expected worktree path, got %q", tm.newWindowDir)
	}
}

func TestExecuteAddNeuron_FallsBackToRepoPathWhenNoWorktree(t *testing.T) {
	// Nuclei created without a worktree (e.g. review with createWorktree=false)
	// must open new neurons in RepoPath, not in an empty string.
	reg := &fakeRegistryNeuronAdd{
		nuclei: []registry.Nucleus{
			{
				ID:           "nucl1",
				RepoPath:     "/repo",
				WorktreePath: "", // no worktree
				Neurons:      []registry.Neuron{{ID: "c1", Type: registry.NeuronClaude}},
			},
		},
	}
	tm := &fakeTmuxNeuronAdd{}

	if err := executeAddNeuron("nucl1", "shell", "", reg, tm); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.newWindowDir != "/repo" {
		t.Fatalf("expected repo path when WorktreePath is empty, got %q", tm.newWindowDir)
	}
}
