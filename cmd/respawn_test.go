package cmd

import (
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryRespawn struct {
	agents           []registry.Agent
	tmuxTargetID     string
	tmuxTargetVal    string
	nvimTargetCleared bool
}

func (f *fakeRegistryRespawn) Load() (*registry.Registry, error) {
	return &registry.Registry{Agents: f.agents}, nil
}
func (f *fakeRegistryRespawn) Add(a registry.Agent) error { return nil }
func (f *fakeRegistryRespawn) Delete(id string) error     { return nil }
func (f *fakeRegistryRespawn) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryRespawn) UpdateNvimTarget(id, target string) error {
	if target == "" {
		f.nvimTargetCleared = true
	}
	return nil
}
func (f *fakeRegistryRespawn) UpdateTmuxTarget(id, target string) error {
	f.tmuxTargetID = id
	f.tmuxTargetVal = target
	return nil
}

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

func TestRespawn_WindowGone_CreatesNew(t *testing.T) {
	reg := &fakeRegistryRespawn{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.0", WorktreePath: "/repo/.worktrees/abc123", TaskDescription: "fix bug"},
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
	reg := &fakeRegistryRespawn{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.0", WorktreePath: "/repo/.worktrees/abc123"},
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

func TestRespawn_UpdatesTmuxTarget(t *testing.T) {
	reg := &fakeRegistryRespawn{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.0", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:3.0"}

	_ = executeRespawn("abc123", reg, tm, &strings.Builder{})
	if reg.tmuxTargetID != "abc123" {
		t.Fatalf("expected UpdateTmuxTarget called for abc123, got %q", reg.tmuxTargetID)
	}
	if reg.tmuxTargetVal != "main:3.0" {
		t.Fatalf("expected new target main:3.0, got %q", reg.tmuxTargetVal)
	}
}

func TestRespawn_ClearsNvimTarget(t *testing.T) {
	reg := &fakeRegistryRespawn{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.0", NvimTarget: "main:2.0", WorktreePath: "/repo/.worktrees/abc123"},
	}}
	tm := &fakeTmuxRespawn{windowExistsResult: false, newWindowTarget: "main:3.0"}

	_ = executeRespawn("abc123", reg, tm, &strings.Builder{})
	if !reg.nvimTargetCleared {
		t.Fatal("expected NvimTarget to be cleared on respawn")
	}
}

func TestRespawn_SendsClaudeKeys(t *testing.T) {
	reg := &fakeRegistryRespawn{agents: []registry.Agent{
		{ID: "abc123", TmuxTarget: "main:1.0", WorktreePath: "/repo/.worktrees/abc123"},
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
