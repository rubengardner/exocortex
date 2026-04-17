package cmd

import (
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// registryAdapter binds the path-based registry functions to the nucleusSvc interface.
type registryAdapter struct {
	path string
}

func (a *registryAdapter) Load() (*registry.Registry, error) {
	return registry.Load(a.path)
}

func (a *registryAdapter) Add(n registry.Nucleus) error {
	return registry.Add(a.path, n)
}

func (a *registryAdapter) Delete(id string) error {
	return registry.Delete(a.path, id)
}

func (a *registryAdapter) UpdateStatus(id, status string) error {
	return registry.UpdateStatus(a.path, id, status)
}

func (a *registryAdapter) AddNeuron(nucleusID string, neuron registry.Neuron) error {
	return registry.AddNeuron(a.path, nucleusID, neuron)
}

func (a *registryAdapter) RemoveNeuron(nucleusID, neuronID string) error {
	return registry.RemoveNeuron(a.path, nucleusID, neuronID)
}

func (a *registryAdapter) UpdateNeuronTarget(nucleusID, neuronID, target string) error {
	return registry.UpdateNeuronTarget(a.path, nucleusID, neuronID, target)
}

func (a *registryAdapter) AddPullRequest(nucleusID string, pr registry.PullRequest) error {
	return registry.AddPullRequest(a.path, nucleusID, pr)
}
