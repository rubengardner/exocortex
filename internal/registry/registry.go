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
		return &Registry{Version: 4}, nil
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
	case 3:
		return migrateV3(data)
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
	r.Version = 4 // always write as v4
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

// migrateV1 converts a v1 JSON blob directly to a v4 Registry.
// Each Agent becomes a Nucleus; repo/worktree/branch land on the Claude Neuron.
// Profile moves from the agent-level field to Nucleus.Profile.
func migrateV1(data []byte) (*Registry, error) {
	var v1 registryV1
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, fmt.Errorf("registry: parse v1: %w", err)
	}

	r := &Registry{Version: 4, Nuclei: make([]Nucleus, 0, len(v1.Agents))}
	for _, a := range v1.Agents {
		n := Nucleus{
			ID:              a.ID,
			TaskDescription: a.TaskDescription,
			Profile:         a.Profile, // moved from Neuron to Nucleus
			Status:          a.Status,
			CreatedAt:       a.CreatedAt,
		}

		claudeNeuron := Neuron{
			ID:           "c1",
			Type:         NeuronClaude,
			TmuxTarget:   a.TmuxTarget,
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

// neuronV2 preserves the Profile field that existed on Neuron in registry v2.
type neuronV2 struct {
	ID           string     `json:"id"`
	Type         NeuronType `json:"type"`
	TmuxTarget   string     `json:"tmux_target"`
	Profile      string     `json:"profile,omitempty"`
	Status       string     `json:"status"`
	RepoPath     string     `json:"repo_path,omitempty"`
	WorktreePath string     `json:"worktree_path,omitempty"`
	Branch       string     `json:"branch,omitempty"`
}

// nucleusV2 matches the legacy Nucleus struct (version 2) for JSON unmarshalling.
type nucleusV2 struct {
	ID              string      `json:"id"`
	RepoPath        string      `json:"repo_path"`
	WorktreePath    string      `json:"worktree_path"`
	Branch          string      `json:"branch"`
	TaskDescription string      `json:"task_description"`
	JiraKey         string      `json:"jira_key,omitempty"`
	PRNumber        int         `json:"pr_number,omitempty"`
	PRRepo          string      `json:"pr_repo,omitempty"`
	Neurons         []neuronV2  `json:"neurons"`
	Status          string      `json:"status"`
	CreatedAt       time.Time   `json:"created_at"`
}

type registryV2 struct {
	Version int         `json:"version"`
	Nuclei  []nucleusV2 `json:"nuclei"`
}

// migrateV2 converts a v2 JSON blob to a v4 Registry.
// Repo/worktree/branch are moved from each Nucleus onto its primary Claude Neuron.
// A single PRNumber/PRRepo becomes PullRequests[0].
// Profile is moved from the primary Claude neuron to Nucleus.Profile.
func migrateV2(data []byte) (*Registry, error) {
	var v2 registryV2
	if err := json.Unmarshal(data, &v2); err != nil {
		return nil, fmt.Errorf("registry: parse v2: %w", err)
	}

	r := &Registry{Version: 4, Nuclei: make([]Nucleus, 0, len(v2.Nuclei))}
	for _, n2 := range v2.Nuclei {
		n := Nucleus{
			ID:              n2.ID,
			TaskDescription: n2.TaskDescription,
			JiraKey:         n2.JiraKey,
			Status:          n2.Status,
			CreatedAt:       n2.CreatedAt,
		}

		// Convert shadow neurons to current Neuron type.
		neurons := make([]Neuron, len(n2.Neurons))
		for i, sn := range n2.Neurons {
			neurons[i] = Neuron{
				ID:           sn.ID,
				Type:         sn.Type,
				TmuxTarget:   sn.TmuxTarget,
				Status:       sn.Status,
				RepoPath:     sn.RepoPath,
				WorktreePath: sn.WorktreePath,
				Branch:       sn.Branch,
			}
		}

		// Find the primary Claude neuron to attach repo/worktree/branch to.
		primaryIdx := -1
		for i, neu := range n2.Neurons {
			if neu.Type == NeuronClaude {
				primaryIdx = i
				break
			}
		}
		if primaryIdx == -1 && len(n2.Neurons) > 0 {
			primaryIdx = 0
		}
		if primaryIdx >= 0 {
			neurons[primaryIdx].RepoPath = n2.RepoPath
			neurons[primaryIdx].WorktreePath = n2.WorktreePath
			neurons[primaryIdx].Branch = n2.Branch
			// Move Profile from primary neuron to Nucleus.
			n.Profile = n2.Neurons[primaryIdx].Profile
		}
		n.Neurons = neurons

		if n2.PRNumber != 0 {
			n.PullRequests = []PullRequest{{Number: n2.PRNumber, Repo: n2.PRRepo}}
		}

		r.Nuclei = append(r.Nuclei, n)
	}
	return r, nil
}

// ── v3 migration ──────────────────────────────────────────────────────────────

// neuronV3 preserves the Profile field that existed on Neuron in registry v3.
type neuronV3 struct {
	ID           string     `json:"id"`
	Type         NeuronType `json:"type"`
	TmuxTarget   string     `json:"tmux_target"`
	Profile      string     `json:"profile,omitempty"`
	Status       string     `json:"status"`
	RepoPath     string     `json:"repo_path,omitempty"`
	WorktreePath string     `json:"worktree_path,omitempty"`
	Branch       string     `json:"branch,omitempty"`
}

// nucleusV3 matches the v3 Nucleus struct for JSON unmarshalling.
type nucleusV3 struct {
	ID              string        `json:"id"`
	TaskDescription string        `json:"task_description"`
	JiraKey         string        `json:"jira_key,omitempty"`
	Profile         string        `json:"profile,omitempty"` // may already be set in late v3 files
	PullRequests    []PullRequest `json:"pull_requests,omitempty"`
	Neurons         []neuronV3    `json:"neurons"`
	Status          string        `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
}

type registryV3 struct {
	Version int        `json:"version"`
	Nuclei  []nucleusV3 `json:"nuclei"`
}

// migrateV3 converts a v3 JSON blob to a v4 Registry.
// Profile is moved from the primary Claude neuron to Nucleus.Profile when not
// already set at the nucleus level.
func migrateV3(data []byte) (*Registry, error) {
	var v3 registryV3
	if err := json.Unmarshal(data, &v3); err != nil {
		return nil, fmt.Errorf("registry: parse v3: %w", err)
	}

	r := &Registry{Version: 4, Nuclei: make([]Nucleus, 0, len(v3.Nuclei))}
	for _, n3 := range v3.Nuclei {
		n := Nucleus{
			ID:              n3.ID,
			TaskDescription: n3.TaskDescription,
			JiraKey:         n3.JiraKey,
			Profile:         n3.Profile,
			PullRequests:    n3.PullRequests,
			Status:          n3.Status,
			CreatedAt:       n3.CreatedAt,
		}

		neurons := make([]Neuron, len(n3.Neurons))
		for i, sn := range n3.Neurons {
			neurons[i] = Neuron{
				ID:           sn.ID,
				Type:         sn.Type,
				TmuxTarget:   sn.TmuxTarget,
				Status:       sn.Status,
				RepoPath:     sn.RepoPath,
				WorktreePath: sn.WorktreePath,
				Branch:       sn.Branch,
			}
			// Promote Profile from primary Claude neuron to nucleus if not set.
			if n.Profile == "" && sn.Type == NeuronClaude && sn.Profile != "" {
				n.Profile = sn.Profile
			}
		}
		n.Neurons = neurons
		r.Nuclei = append(r.Nuclei, n)
	}
	return r, nil
}
