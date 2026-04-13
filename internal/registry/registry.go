package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Registry struct {
	Agents []Agent `json:"agents"`
}

type Agent struct {
	ID              string    `json:"id"`
	RepoPath        string    `json:"repo_path"`
	WorktreePath    string    `json:"worktree_path"`
	Branch          string    `json:"branch"`
	TaskDescription string    `json:"task_description"`
	TmuxTarget      string    `json:"tmux_target"`
	NvimTarget      string    `json:"nvim_target,omitempty"`
	Profile         string    `json:"profile,omitempty"` // profile name used to launch claude (maps to CLAUDE_CONFIG_DIR)
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	LastFile        string    `json:"last_file,omitempty"`
}

// DefaultPath returns the canonical registry file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "exocortex", "registry.json")
}

// Load reads the registry from path. Returns an empty registry if the file
// does not exist; any other error is returned to the caller.
func Load(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Registry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
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
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".registry-*.json.tmp")
	if err != nil {
		return fmt.Errorf("registry: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Always clean up the temp file on failure.
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

// Add loads the registry, appends the agent, and saves.
func Add(path string, a Agent) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	r.Agents = append(r.Agents, a)
	return Save(path, r)
}

// Delete loads the registry, removes the agent with the given ID, and saves.
// Returns an error if the ID is not found.
func Delete(path string, id string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	idx := -1
	for i, a := range r.Agents {
		if a.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("registry: agent %q not found", id)
	}
	r.Agents = append(r.Agents[:idx], r.Agents[idx+1:]...)
	return Save(path, r)
}

// UpdateStatus loads the registry, sets the named agent's status, and saves.
func UpdateStatus(path, id, status string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Agents {
		if r.Agents[i].ID == id {
			r.Agents[i].Status = status
			return Save(path, r)
		}
	}
	return fmt.Errorf("registry: agent %q not found", id)
}

// UpdateTmuxTarget loads the registry, sets the named agent's tmux_target, and saves.
func UpdateTmuxTarget(path, id, target string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Agents {
		if r.Agents[i].ID == id {
			r.Agents[i].TmuxTarget = target
			return Save(path, r)
		}
	}
	return fmt.Errorf("registry: agent %q not found", id)
}

// UpdateNvimTarget loads the registry, sets the named agent's nvim_target, and saves.
func UpdateNvimTarget(path, id, target string) error {
	r, err := Load(path)
	if err != nil {
		return err
	}
	for i := range r.Agents {
		if r.Agents[i].ID == id {
			r.Agents[i].NvimTarget = target
			return Save(path, r)
		}
	}
	return fmt.Errorf("registry: agent %q not found", id)
}

// FindByID returns a pointer to the agent with the given ID.
// Returns an error if not found.
func (r *Registry) FindByID(id string) (*Agent, error) {
	for i := range r.Agents {
		if r.Agents[i].ID == id {
			return &r.Agents[i], nil
		}
	}
	return nil, fmt.Errorf("registry: agent %q not found", id)
}
