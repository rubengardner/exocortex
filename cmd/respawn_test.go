package cmd

import (
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryRespawn struct {
	nuclei              []registry.Nucleus
	updatedNeuronTarget string
	removedNeuronID     string
}

func (f *fakeRegistryRespawn) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryRespawn) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryRespawn) Delete(id string) error       { return nil }
func (f *fakeRegistryRespawn) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryRespawn) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	return nil
}
func (f *fakeRegistryRespawn) RemoveNeuron(nucleusID, neuronID string) error {
	f.removedNeuronID = neuronID
	return nil
}
func (f *fakeRegistryRespawn) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	f.updatedNeuronTarget = target
	return nil
}
func (f *fakeRegistryRespawn) AddPullRequest(nucleusID string, pr registry.PullRequest) error { return nil }


type fakeTmuxRespawn struct {
	windowExistsResult bool
	newWindowTarget    string
	sentTarget         string
	sentKeys           string
	newWindowCalled    bool
}

func (t *fakeTmuxRespawn) NewWindow(workdir, name string) (string, error) {
	t.newWindowCalled = true
	return t.newWindowTarget, nil
}
func (t *fakeTmuxRespawn) SelectPane(target string) error { return nil }
func (t *fakeTmuxRespawn) KillPane(target string) error   { return nil }
func (t *fakeTmuxRespawn) SendKeys(target, keys string) error {
	t.sentTarget = target
	t.sentKeys = keys
	return nil
}
func (t *fakeTmuxRespawn) WindowExists(target string) (bool, error) {
	return t.windowExistsResult, nil
}
func (t *fakeTmuxRespawn) CurrentTarget() (string, error)            { return "", nil }
func (t *fakeTmuxRespawn) CapturePane(target string) (string, error) { return "", nil }

func sampleNucleusRespawn(id, tmuxTarget string) registry.Nucleus {
	return registry.Nucleus{
		ID:              id,
		TaskDescription: "fix bug",
		Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: tmuxTarget, Status: "idle", WorktreePath: "/repo/.worktrees/" + id},
		},
	}
}

func TestRespawn_WindowGone_CreatesNew(t *testing.T) {
	reg := &fakeRegistryRespawn{nuclei: []registry.Nucleus{
		sampleNucleusRespawn("abc123", "main:1.0"),
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:2.0"}

	out := &strings.Builder{}
	err := executeRespawn("abc123", reg, tm, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tm.newWindowCalled {
		t.Fatal("expected new window to be created")
	}
}

func TestRespawn_WindowAlive_DoesNothing(t *testing.T) {
	reg := &fakeRegistryRespawn{nuclei: []registry.Nucleus{
		sampleNucleusRespawn("abc123", "main:1.0"),
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: true}

	out := &strings.Builder{}
	err := executeRespawn("abc123", reg, tm, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.newWindowCalled {
		t.Fatal("expected no new window when existing window is alive")
	}
}

func TestRespawn_UpdatesNeuronTarget(t *testing.T) {
	reg := &fakeRegistryRespawn{nuclei: []registry.Nucleus{
		sampleNucleusRespawn("abc123", "main:1.0"),
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:3.0"}

	_ = executeRespawn("abc123", reg, tm, &strings.Builder{})
	if reg.updatedNeuronTarget != "main:3.0" {
		t.Fatalf("expected updated target main:3.0, got %q", reg.updatedNeuronTarget)
	}
}

func TestRespawn_RemovesNvimNeuron(t *testing.T) {
	n := sampleNucleusRespawn("abc123", "main:1.0")
	n.Neurons = append(n.Neurons, registry.Neuron{
		ID: "nvim", Type: registry.NeuronNvim, TmuxTarget: "main:2.0",
	})
	reg := &fakeRegistryRespawn{nuclei: []registry.Nucleus{n}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:3.0"}

	_ = executeRespawn("abc123", reg, tm, &strings.Builder{})
	if reg.removedNeuronID != "nvim" {
		t.Fatalf("expected nvim neuron removed, got %q", reg.removedNeuronID)
	}
}

func TestRespawn_SendsClaudeKeys(t *testing.T) {
	reg := &fakeRegistryRespawn{nuclei: []registry.Nucleus{
		sampleNucleusRespawn("abc123", "main:1.0"),
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:3.0"}

	_ = executeRespawn("abc123", reg, tm, &strings.Builder{})
	if tm.sentKeys != "claude" {
		t.Fatalf("expected 'claude' keys sent, got %q", tm.sentKeys)
	}
}

func TestRespawn_UnknownID(t *testing.T) {
	reg := &fakeRegistryRespawn{}
	tm := &fakeTmuxRespawn{}

	err := executeRespawn("nope", reg, tm, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}
