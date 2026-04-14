package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// ── shared fake registry (implements nucleusSvc) ─────────────────────────────

type fakeRegistry struct {
	nuclei  []registry.Nucleus
	added   *registry.Nucleus
	addErr  error

	deletedID     string
	updatedStatus string

	addedNeuronNucleusID string
	addedNeuron          *registry.Neuron

	removedNeuronNucleusID string
	removedNeuronID        string

	updatedNeuronNucleusID string
	updatedNeuronID        string
	updatedNeuronTarget    string
}

func (f *fakeRegistry) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistry) Add(n registry.Nucleus) error {
	f.added = &n
	return f.addErr
}
func (f *fakeRegistry) Delete(id string) error {
	f.deletedID = id
	return nil
}
func (f *fakeRegistry) UpdateStatus(id, status string) error {
	f.updatedStatus = status
	return nil
}
func (f *fakeRegistry) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	f.addedNeuronNucleusID = nucleusID
	f.addedNeuron = &neuron
	return nil
}
func (f *fakeRegistry) RemoveNeuron(nucleusID, neuronID string) error {
	f.removedNeuronNucleusID = nucleusID
	f.removedNeuronID = neuronID
	return nil
}
func (f *fakeRegistry) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	f.updatedNeuronNucleusID = nucleusID
	f.updatedNeuronID = neuronID
	f.updatedNeuronTarget = target
	return nil
}

// ── shared fake git ────────────────────────────────────────────────────────────

type fakeGit struct {
	addErr       error
	addCalled    bool
	createBranch bool
	branchExists bool
}

func (g *fakeGit) AddWorktree(repoPath, worktreePath, branch string, createBranch bool) error {
	g.addCalled = true
	g.createBranch = createBranch
	return g.addErr
}
func (g *fakeGit) RemoveWorktree(repoPath, worktreePath string) error        { return nil }
func (g *fakeGit) ModifiedFiles(worktreePath string) ([]string, error)       { return nil, nil }
func (g *fakeGit) BranchExists(repoPath, branch string) (bool, error)        { return g.branchExists, nil }

// ── shared fake tmux ──────────────────────────────────────────────────────────

type fakeTmux struct {
	target   string
	splitErr error
}

func (t *fakeTmux) NewWindow(workdir, name string) (string, error)  { return t.target, t.splitErr }
func (t *fakeTmux) SelectPane(target string) error                  { return nil }
func (t *fakeTmux) KillPane(target string) error                    { return nil }
func (t *fakeTmux) SendKeys(target, keys string) error              { return nil }
func (t *fakeTmux) WindowExists(target string) (bool, error)        { return false, nil }
func (t *fakeTmux) CurrentTarget() (string, error)                  { return "", nil }
func (t *fakeTmux) CapturePane(target string) (string, error)       { return "", nil }

// ── executeNew tests ──────────────────────────────────────────────────────────

func TestRunNew_SavesNucleus(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}

	out := &strings.Builder{}
	err := executeNew("Fix auth bug", ".", "", "", reg, gt, tm, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.added == nil {
		t.Fatal("expected registry.Add to be called")
	}
	if reg.added.TaskDescription != "Fix auth bug" {
		t.Fatalf("wrong task: %s", reg.added.TaskDescription)
	}
	primary := reg.added.PrimaryNeuron()
	if primary == nil {
		t.Fatal("expected a primary neuron")
	}
	if primary.TmuxTarget != "main:1.0" {
		t.Fatalf("wrong tmux target: %s", primary.TmuxTarget)
	}
	if reg.added.Status != "idle" {
		t.Fatalf("wrong status: %s", reg.added.Status)
	}
}

func TestRunNew_CreatesWorktree(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if !gt.addCalled {
		t.Fatal("expected git.AddWorktree to be called")
	}
}

func TestRunNew_AutoGeneratesBranch(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if reg.added == nil {
		t.Fatal("nothing saved")
	}
	if reg.added.Branch == "" {
		t.Fatal("branch should be auto-generated")
	}
}

func TestRunNew_UsesBranchFlag(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "my-explicit-branch", "", reg, gt, tm, &strings.Builder{})
	if reg.added.Branch != "my-explicit-branch" {
		t.Fatalf("expected explicit branch, got %s", reg.added.Branch)
	}
}

func TestRunNew_PrintsID(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}
	out := &strings.Builder{}

	_ = executeNew("my task", ".", "", "", reg, gt, tm, out)
	if !strings.Contains(out.String(), reg.added.ID) {
		t.Fatalf("output should contain nucleus ID, got: %s", out.String())
	}
}

func TestRunNew_GitError_Propagates(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{addErr: errors.New("git exploded")}
	tm := &fakeTmux{target: "main:1.0"}

	err := executeNew("task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error from git")
	}
}

func TestRunNew_TmuxError_Propagates(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{splitErr: errors.New("no session")}

	err := executeNew("task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if err == nil {
		t.Fatal("expected error from tmux")
	}
}

func TestRunNew_ExistingBranch_DoesNotCreateBranch(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{branchExists: true}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "existing-branch", "", reg, gt, tm, &strings.Builder{})
	if gt.createBranch {
		t.Fatal("should not create branch when it already exists")
	}
}

func TestRunNew_NewBranch_CreatesBranch(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{branchExists: false}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if !gt.createBranch {
		t.Fatal("should create branch when it does not exist")
	}
}

func TestRunNew_SendsClaudeKeys(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	sentTarget, sentKeys := "", ""
	tm := &fakeTmuxSpy{target: "main:1.0", onSendKeys: func(target, keys string) {
		sentTarget = target
		sentKeys = keys
	}}

	err := executeNew("my task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentKeys != "claude" {
		t.Fatalf("expected 'claude' keys sent, got %q", sentKeys)
	}
	if sentTarget != "main:1.0" {
		t.Fatalf("expected keys sent to main:1.0, got %q", sentTarget)
	}
}

func TestRunNew_HasClaudeNeuron(t *testing.T) {
	reg := &fakeRegistry{}
	gt := &fakeGit{}
	tm := &fakeTmux{target: "main:1.0"}

	_ = executeNew("my task", ".", "", "", reg, gt, tm, &strings.Builder{})
	if reg.added == nil {
		t.Fatal("nothing saved")
	}
	primary := reg.added.PrimaryNeuron()
	if primary == nil {
		t.Fatal("expected primary neuron")
	}
	if primary.Type != registry.NeuronClaude {
		t.Fatalf("expected claude neuron type, got %s", primary.Type)
	}
}

// fakeTmuxSpy captures SendKeys calls for inspection.
type fakeTmuxSpy struct {
	target     string
	splitErr   error
	onSendKeys func(target, keys string)
}

func (t *fakeTmuxSpy) NewWindow(workdir, name string) (string, error) { return t.target, t.splitErr }
func (t *fakeTmuxSpy) SelectPane(target string) error                 { return nil }
func (t *fakeTmuxSpy) KillPane(target string) error                   { return nil }
func (t *fakeTmuxSpy) SendKeys(target, keys string) error {
	if t.onSendKeys != nil {
		t.onSendKeys(target, keys)
	}
	return nil
}
func (t *fakeTmuxSpy) WindowExists(target string) (bool, error)  { return false, nil }
func (t *fakeTmuxSpy) CurrentTarget() (string, error)            { return "", nil }
func (t *fakeTmuxSpy) CapturePane(target string) (string, error) { return "", nil }

// ── slug tests ────────────────────────────────────────────────────────────────

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Fix auth bug", "fixaut"},
		{"hello world", "hellow"},
		{"A", "a"},
		{"!!!", ""},
		{"abcdefghij", "abcdef"},
	}
	for _, c := range cases {
		got := slugify(c.in)
		if got != c.want {
			t.Errorf("slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueID_NoCollision(t *testing.T) {
	id := uniqueID("my task", nil)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestUniqueID_Collision(t *testing.T) {
	nuclei := []registry.Nucleus{{ID: "mytask"}}
	id := uniqueID("my task", nuclei)
	if id == "mytask" {
		t.Fatal("expected a different id on collision")
	}
}
