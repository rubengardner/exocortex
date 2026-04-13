package cmd

import (
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryBar struct {
	agents []registry.Agent
}

func (f *fakeRegistryBar) Load() (*registry.Registry, error) {
	return &registry.Registry{Agents: f.agents}, nil
}
func (f *fakeRegistryBar) Add(a registry.Agent) error                  { return nil }
func (f *fakeRegistryBar) Delete(id string) error                      { return nil }
func (f *fakeRegistryBar) UpdateStatus(id, status string) error        { return nil }
func (f *fakeRegistryBar) UpdateNvimTarget(id, target string) error    { return nil }
func (f *fakeRegistryBar) UpdateTmuxTarget(id, target string) error    { return nil }

func TestBar_NoWaiting(t *testing.T) {
	reg := &fakeRegistryBar{agents: []registry.Agent{
		{ID: "a", Status: "idle"},
		{ID: "b", Status: "working"},
	}}
	out, err := barOutput(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output when no waiting agents, got %q", out)
	}
}

func TestBar_OneWaiting(t *testing.T) {
	reg := &fakeRegistryBar{agents: []registry.Agent{
		{ID: "a", Status: "waiting"},
		{ID: "b", Status: "idle"},
	}}
	out, err := barOutput(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1 waiting") {
		t.Fatalf("expected '1 waiting' in output, got %q", out)
	}
}

func TestBar_MultipleWaiting(t *testing.T) {
	reg := &fakeRegistryBar{agents: []registry.Agent{
		{ID: "a", Status: "waiting"},
		{ID: "b", Status: "waiting"},
		{ID: "c", Status: "idle"},
	}}
	out, err := barOutput(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "2 waiting") {
		t.Fatalf("expected '2 waiting' in output, got %q", out)
	}
}
