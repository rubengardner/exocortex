package cmd

import (
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeTmuxDetect struct {
	currentTarget string
	currentErr    error
}

func (t *fakeTmuxDetect) NewWindow(workdir, name string) (string, error) { return "", nil }
func (t *fakeTmuxDetect) SelectPane(target string) error                 { return nil }
func (t *fakeTmuxDetect) KillPane(target string) error                   { return nil }
func (t *fakeTmuxDetect) SendKeys(target, keys string) error             { return nil }
func (t *fakeTmuxDetect) WindowExists(target string) (bool, error)       { return false, nil }
func (t *fakeTmuxDetect) CapturePane(target string) (string, error)      { return "", nil }
func (t *fakeTmuxDetect) CurrentTarget() (string, error) {
	return t.currentTarget, t.currentErr
}

type fakeRegistryDetect struct {
	nuclei []registry.Nucleus
}

func (f *fakeRegistryDetect) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryDetect) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryDetect) Delete(id string) error       { return nil }
func (f *fakeRegistryDetect) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryDetect) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	return nil
}
func (f *fakeRegistryDetect) RemoveNeuron(nucleusID, neuronID string) error { return nil }
func (f *fakeRegistryDetect) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	return nil
}
func (f *fakeRegistryDetect) AddPullRequest(nucleusID string, pr registry.PullRequest) error { return nil }


func TestDetectAgentID_Match(t *testing.T) {
	t.Setenv("TMUX_PANE", "%3")
	reg := &fakeRegistryDetect{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:2.0"},
		}},
	}}
	tm := &fakeTmuxDetect{currentTarget: "main:2.0"}

	id, err := detectAgentID(reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc123" {
		t.Fatalf("expected abc123, got %q", id)
	}
}

func TestDetectAgentID_MatchesNvimNeuron(t *testing.T) {
	t.Setenv("TMUX_PANE", "%4")
	reg := &fakeRegistryDetect{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0"},
			{ID: "nvim", Type: registry.NeuronNvim, TmuxTarget: "main:2.0"},
		}},
	}}
	tm := &fakeTmuxDetect{currentTarget: "main:2.0"}

	id, err := detectAgentID(reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc123" {
		t.Fatalf("expected abc123, got %q", id)
	}
}

func TestDetectAgentID_NoMatch(t *testing.T) {
	t.Setenv("TMUX_PANE", "%3")
	reg := &fakeRegistryDetect{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:2.0"},
		}},
	}}
	tm := &fakeTmuxDetect{currentTarget: "main:9.9"}

	_, err := detectAgentID(reg, tm)
	if err == nil {
		t.Fatal("expected error when pane does not match any neuron")
	}
}

func TestDetectAgentID_NoEnv(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	reg := &fakeRegistryDetect{}
	tm := &fakeTmuxDetect{}

	_, err := detectAgentID(reg, tm)
	if err == nil {
		t.Fatal("expected error when TMUX_PANE not set")
	}
}
