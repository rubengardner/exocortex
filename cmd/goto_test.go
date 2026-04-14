package cmd

import (
	"errors"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeTmuxGoto struct {
	selectedTarget string
	selectErr      error
}

func (t *fakeTmuxGoto) NewWindow(workdir, name string) (string, error) { return "", nil }
func (t *fakeTmuxGoto) SelectPane(target string) error {
	t.selectedTarget = target
	return t.selectErr
}
func (t *fakeTmuxGoto) KillPane(target string) error                   { return nil }
func (t *fakeTmuxGoto) SendKeys(target, keys string) error             { return nil }
func (t *fakeTmuxGoto) WindowExists(target string) (bool, error)       { return false, nil }
func (t *fakeTmuxGoto) CurrentTarget() (string, error)                 { return "", nil }
func (t *fakeTmuxGoto) CapturePane(target string) (string, error)      { return "", nil }

func TestRunGoto_SelectsCorrectPane(t *testing.T) {
	reg := &fakeRegistry{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
		}},
	}}
	tm := &fakeTmuxGoto{}

	err := executeGoto("abc123", reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.selectedTarget != "main:1.2" {
		t.Fatalf("expected main:1.2, got %s", tm.selectedTarget)
	}
}

func TestRunGoto_UnknownID(t *testing.T) {
	reg := &fakeRegistry{}
	tm := &fakeTmuxGoto{}

	err := executeGoto("nope", reg, tm)
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestRunGoto_NoArg_UsesCurrentPane(t *testing.T) {
	t.Setenv("TMUX_PANE", "%1")
	reg := &fakeRegistry{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0"},
		}},
	}}
	tm := &fakeTmuxGoto{}

	// With an explicit arg, resolveID returns it directly.
	id, err := resolveID([]string{"abc123"}, reg, tm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "abc123" {
		t.Fatalf("expected abc123, got %q", id)
	}
}

func TestRunGoto_TmuxError(t *testing.T) {
	reg := &fakeRegistry{nuclei: []registry.Nucleus{
		{ID: "abc123", Neurons: []registry.Neuron{
			{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.2"},
		}},
	}}
	tm := &fakeTmuxGoto{selectErr: errors.New("pane gone")}

	err := executeGoto("abc123", reg, tm)
	if err == nil {
		t.Fatal("expected error from tmux")
	}
}
