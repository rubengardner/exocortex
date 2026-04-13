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
	agents []registry.Agent
}

func (f *fakeRegistryDetect) Load() (*registry.Registry, error) {
	return &registry.Registry{Agents: f.agents}, nil
}
func (f *fakeRegistryDetect) Add(a registry.Agent) error                  { return nil }
func (f *fakeRegistryDetect) Delete(id string) error                      { return nil }
func (f *fakeRegistryDetect) UpdateStatus(id, status string) error        { return nil }
func (f *fakeRegistryDetect) UpdateNvimTarget(id, target string) error    { return nil }
func (f *fakeRegistryDetect) UpdateTmuxTarget(id, target string) error    { return nil }

func TestDetectAgentID_Match(t *testing.T) {
	t.Setenv("TMUX_PANE", "%3")
	reg := &fakeRegistryDetect{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:2.0"},
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
	reg := &fakeRegistryDetect{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:2.0"},
	}}
	tm := &fakeTmuxDetect{currentTarget: "main:9.9"}

	_, err := detectAgentID(reg, tm)
	if err == nil {
		t.Fatal("expected error when pane does not match any agent")
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
