package cmd

import (
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// registryAdapter binds the path-based registry functions to the registrySvc interface.
type registryAdapter struct {
	path string
}

func (a *registryAdapter) Load() (*registry.Registry, error) {
	return registry.Load(a.path)
}

func (a *registryAdapter) Add(agent registry.Agent) error {
	return registry.Add(a.path, agent)
}

func (a *registryAdapter) Delete(id string) error {
	return registry.Delete(a.path, id)
}

func (a *registryAdapter) UpdateStatus(id, status string) error {
	return registry.UpdateStatus(a.path, id, status)
}

func (a *registryAdapter) UpdateNvimTarget(id, target string) error {
	return registry.UpdateNvimTarget(a.path, id, target)
}

func (a *registryAdapter) UpdateTmuxTarget(id, target string) error {
	return registry.UpdateTmuxTarget(a.path, id, target)
}
