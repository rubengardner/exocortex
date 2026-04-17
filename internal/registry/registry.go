package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Registry is the top-level container persisted to disk.
// Version 3 uses Nuclei with per-Neuron repo/worktree/branch.
// Version 2 (legacy) stores those fields on the Nucleus and is auto-migrated on load.
// Version 0/1 (legacy) used Agents and is also auto-migrated on load.
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
// does not exist. Legacy v1/v2 files are migrated to v3 on load but not written
// back — the caller must Save to persist the migration.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Registry{Version: 3}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}

	var probe struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(data, &probe)

	switch probe.Version {
	case 0:
		return migrateV1(data)
	case 2:
		return migrateV2(data)
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
	r.Version = 3 // always write as v3
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

// AddPullRequest appends a PullRequest to the named Nucleus.
func AddPullRequest(path, nucleusID string, pr PullRequest) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Nuclei {
		if r.Nuclei[i].ID == nucleusID {
			r.Nuclei[i].PullRequests = append(r.Nuclei[i].PullRequests, pr)
			return Save(path, r)
		}
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

// migrateV1 converts a v1 JSON blob directly to a v3 Registry.
// Each Agent becomes a Nucleus; repo/worktree/branch land on the Claude Neuron.
func migrateV1(data []byte) (*Registry, error) {
	var v1 registryV1
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, fmt.Errorf("registry: parse v1: %w", err)
	}

	r := &Registry{Version: 3, Nuclei: make([]Nucleus, 0, len(v1.Agents))}
	for _, a := range v1.Agents {
		n := Nucleus{
			ID:              a.ID,
			TaskDescription: a.TaskDescription,
			Status:          a.Status,
			CreatedAt:       a.CreatedAt,
		}

		claudeNeuron := Neuron{
			ID:           "c1",
			Type:         NeuronClaude,
			TmuxTarget:   a.TmuxTarget,
			Profile:      a.Profile,
			Status:       a.Status,
			RepoPath:     a.RepoPath,
			WorktreePath: a.WorktreePath,
			Branch:       a.Branch,
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

// ── v2 migration ──────────────────────────────────────────────────────────────

// nucleusV2 matches the legacy Nucleus struct (version 2) for JSON unmarshalling.
type nucleusV2 struct {
	ID              string    `json:"id"`
	RepoPath        string    `json:"repo_path"`
	WorktreePath    string    `json:"worktree_path"`
	Branch          string    `json:"branch"`
	TaskDescription string    `json:"task_description"`
	JiraKey         string    `json:"jira_key,omitempty"`
	PRNumber        int       `json:"pr_number,omitempty"`
	PRRepo          string    `json:"pr_repo,omitempty"`
	Neurons         []Neuron  `json:"neurons"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type registryV2 struct {
	Version int        `json:"version"`
	Nuclei  []nucleusV2 `json:"nuclei"`
}

// migrateV2 converts a v2 JSON blob to a v3 Registry.
// Repo/worktree/branch are moved from each Nucleus onto its primary Claude Neuron.
// A single PRNumber/PRRepo becomes PullRequests[0].
func migrateV2(data []byte) (*Registry, error) {
	var v2 registryV2
	if err := json.Unmarshal(data, &v2); err != nil {
		return nil, fmt.Errorf("registry: parse v2: %w", err)
	}

	r := &Registry{Version: 3, Nuclei: make([]Nucleus, 0, len(v2.Nuclei))}
	for _, n2 := range v2.Nuclei {
		n := Nucleus{
			ID:              n2.ID,
			TaskDescription: n2.TaskDescription,
			JiraKey:         n2.JiraKey,
			Status:          n2.Status,
			CreatedAt:       n2.CreatedAt,
		}

		neurons := make([]Neuron, len(n2.Neurons))
		copy(neurons, n2.Neurons)

		// Find the primary Claude neuron to attach repo/worktree/branch to.
		primaryIdx := -1
		for i, neu := range neurons {
			if neu.Type == NeuronClaude {
				primaryIdx = i
				break
			}
		}
		if primaryIdx == -1 && len(neurons) > 0 {
			primaryIdx = 0
		}
		if primaryIdx >= 0 {
			neurons[primaryIdx].RepoPath = n2.RepoPath
			neurons[primaryIdx].WorktreePath = n2.WorktreePath
			neurons[primaryIdx].Branch = n2.Branch
		}
		n.Neurons = neurons

		if n2.PRNumber != 0 {
			n.PullRequests = []PullRequest{{Number: n2.PRNumber, Repo: n2.PRRepo}}
		}

		r.Nuclei = append(r.Nuclei, n)
	}
	return r, nil
}
