package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Registry is the top-level container persisted to disk.
// Version 2 uses Nuclei; version 0/1 (legacy) used Agents and is auto-migrated on load.
type Registry struct {
	Version int       `json:"version"`
	Nuclei  []Nucleus `json:"nuclei"`
}

// DefaultPath returns the canonical registry file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "exocortex", "registry.json")
}

// Load reads the registry from path. Returns an empty registry if the file
// does not exist. A legacy v1 file (with "agents" key) is migrated to v2 on
// load but not written back — the caller must Save to persist the migration.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Registry{Version: 2}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}

	// Probe for the version field without fully parsing the document.
	var probe struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(data, &probe)

	if probe.Version == 0 {
		// Legacy v1 format — migrate.
		return migrateV1(data)
	}

	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}
	return &r, nil
}

// Save writes the registry to path atomically (temp file + rename).
// It creates the parent directory if it does not exist.
func Save(path string, r *Registry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: mkdir: %w", err)
	}
	r.Version = 2 // always write as v2
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".registry-*.json.tmp")
	if err != nil {
		return fmt.Errorf("registry: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("registry: rename: %w", err)
	}
	return nil
}

// Add loads the registry, appends the nucleus, and saves.
func Add(path string, n Nucleus) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	r.Nuclei = append(r.Nuclei, n)
	return Save(path, r)
}

// Delete loads the registry, removes the nucleus with the given ID, and saves.
func Delete(path, id string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	idx := -1
	for i, n := range r.Nuclei {
		if n.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("registry: nucleus %q not found", id)
	}
	r.Nuclei = append(r.Nuclei[:idx], r.Nuclei[idx+1:]...)
	return Save(path, r)
}

// UpdateStatus loads the registry, sets the named nucleus's status, and saves.
func UpdateStatus(path, id, status string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Nuclei {
		if r.Nuclei[i].ID == id {
			r.Nuclei[i].Status = status
			return Save(path, r)
		}
	}
	return fmt.Errorf("registry: nucleus %q not found", id)
}

// AddNeuron appends a Neuron to the named Nucleus.
func AddNeuron(path, nucleusID string, neuron Neuron) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Nuclei {
		if r.Nuclei[i].ID == nucleusID {
			r.Nuclei[i].Neurons = append(r.Nuclei[i].Neurons, neuron)
			return Save(path, r)
		}
	}
	return fmt.Errorf("registry: nucleus %q not found", nucleusID)
}

// RemoveNeuron removes a Neuron by ID from the named Nucleus.
func RemoveNeuron(path, nucleusID, neuronID string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Nuclei {
		if r.Nuclei[i].ID != nucleusID {
			continue
		}
		idx := -1
		for j, neu := range r.Nuclei[i].Neurons {
			if neu.ID == neuronID {
				idx = j
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("registry: neuron %q not found in nucleus %q", neuronID, nucleusID)
		}
		r.Nuclei[i].Neurons = append(r.Nuclei[i].Neurons[:idx], r.Nuclei[i].Neurons[idx+1:]...)
		return Save(path, r)
	}
	return fmt.Errorf("registry: nucleus %q not found", nucleusID)
}

// UpdateNeuronTarget sets TmuxTarget on a specific Neuron within a Nucleus.
func UpdateNeuronTarget(path, nucleusID, neuronID, target string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Nuclei {
		if r.Nuclei[i].ID != nucleusID {
			continue
		}
		for j := range r.Nuclei[i].Neurons {
			if r.Nuclei[i].Neurons[j].ID == neuronID {
				r.Nuclei[i].Neurons[j].TmuxTarget = target
				return Save(path, r)
			}
		}
		return fmt.Errorf("registry: neuron %q not found in nucleus %q", neuronID, nucleusID)
	}
	return fmt.Errorf("registry: nucleus %q not found", nucleusID)
}

// FindByID returns a pointer to the Nucleus with the given ID.
func (r *Registry) FindByID(id string) (*Nucleus, error) {
	for i := range r.Nuclei {
		if r.Nuclei[i].ID == id {
			return &r.Nuclei[i], nil
		}
	}
	return nil, fmt.Errorf("registry: nucleus %q not found", id)
}

// ── v1 migration ──────────────────────────────────────────────────────────────

// agentV1 matches the legacy Agent struct for JSON unmarshalling during migration.
type agentV1 struct {
	ID              string    `json:"id"`
	RepoPath        string    `json:"repo_path"`
	WorktreePath    string    `json:"worktree_path"`
	Branch          string    `json:"branch"`
	TaskDescription string    `json:"task_description"`
	TmuxTarget      string    `json:"tmux_target"`
	NvimTarget      string    `json:"nvim_target,omitempty"`
	Profile         string    `json:"profile,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type registryV1 struct {
	Agents []agentV1 `json:"agents"`
}

// migrateV1 converts a v1 JSON blob to a v2 Registry.
// Each Agent becomes a Nucleus; TmuxTarget → claude Neuron; NvimTarget (if set) → nvim Neuron.
func migrateV1(data []byte) (*Registry, error) {
	var v1 registryV1
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, fmt.Errorf("registry: parse v1: %w", err)
	}

	r := &Registry{Version: 2, Nuclei: make([]Nucleus, 0, len(v1.Agents))}
	for _, a := range v1.Agents {
		n := Nucleus{
			ID:              a.ID,
			RepoPath:        a.RepoPath,
			WorktreePath:    a.WorktreePath,
			Branch:          a.Branch,
			TaskDescription: a.TaskDescription,
			Status:          a.Status,
			CreatedAt:       a.CreatedAt,
		}

		claudeNeuron := Neuron{
			ID:         "c1",
			Type:       NeuronClaude,
			TmuxTarget: a.TmuxTarget,
			Profile:    a.Profile,
			Status:     a.Status,
		}
		n.Neurons = append(n.Neurons, claudeNeuron)

		if a.NvimTarget != "" {
			nvimNeuron := Neuron{
				ID:         "nvim",
				Type:       NeuronNvim,
				TmuxTarget: a.NvimTarget,
				Status:     "idle",
			}
			n.Neurons = append(n.Neurons, nvimNeuron)
		}

		r.Nuclei = append(r.Nuclei, n)
	}
	return r, nil
}
