package cmd

import (
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryBar struct {
	nuclei []registry.Nucleus
}

func (f *fakeRegistryBar) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryBar) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryBar) Delete(id string) error       { return nil }
func (f *fakeRegistryBar) UpdateStatus(id, status string) error { return nil }
func (f *fakeRegistryBar) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	return nil
}
func (f *fakeRegistryBar) RemoveNeuron(nucleusID, neuronID string) error { return nil }
func (f *fakeRegistryBar) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	return nil
}

func TestBar_NoWaiting(t *testing.T) {
	reg := &fakeRegistryBar{nuclei: []registry.Nucleus{
		{ID: "a", Status: "idle"},
		{ID: "b", Status: "working"},
	}}
	out, err := barOutput(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output when no waiting nuclei, got %q", out)
	}
}

func TestBar_OneWaiting(t *testing.T) {
	reg := &fakeRegistryBar{nuclei: []registry.Nucleus{
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
	reg := &fakeRegistryBar{nuclei: []registry.Nucleus{
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
