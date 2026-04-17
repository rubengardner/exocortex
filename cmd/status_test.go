package cmd

import (
	"testing"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

type fakeRegistryStatus struct {
	nuclei        []registry.Nucleus
	updatedID     string
	updatedStatus string
	updateErr     error
}

func (f *fakeRegistryStatus) Load() (*registry.Registry, error) {
	return &registry.Registry{Nuclei: f.nuclei}, nil
}
func (f *fakeRegistryStatus) Add(n registry.Nucleus) error { return nil }
func (f *fakeRegistryStatus) Delete(id string) error       { return nil }
func (f *fakeRegistryStatus) UpdateStatus(id, status string) error {
	f.updatedID = id
	f.updatedStatus = status
	return f.updateErr
}
func (f *fakeRegistryStatus) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	return nil
}
func (f *fakeRegistryStatus) RemoveNeuron(nucleusID, neuronID string) error { return nil }
func (f *fakeRegistryStatus) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	return nil
}
func (f *fakeRegistryStatus) AddPullRequest(nucleusID string, pr registry.PullRequest) error { return nil }


func TestStatusCmd_UpdatesRegistry(t *testing.T) {
	reg := &fakeRegistryStatus{}
	if err := executeStatus("abc123", "waiting", reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reg.updatedID != "abc123" {
		t.Fatalf("expected id abc123, got %q", reg.updatedID)
	}
	if reg.updatedStatus != "waiting" {
		t.Fatalf("expected status waiting, got %q", reg.updatedStatus)
	}
}

func TestStatusCmd_AllValidStatuses(t *testing.T) {
	for _, status := range []string{"idle", "working", "waiting", "blocked"} {
		reg := &fakeRegistryStatus{}
		if err := executeStatus("abc", status, reg); err != nil {
			t.Fatalf("status %q should be valid, got error: %v", status, err)
		}
	}
}

func TestStatusCmd_InvalidStatus(t *testing.T) {
	reg := &fakeRegistryStatus{}
	err := executeStatus("abc", "unknown", reg)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}
