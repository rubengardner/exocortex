package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JiraConfig holds credentials and settings for the Jira board view.
type JiraConfig struct {
	BaseURL  string   `json:"base_url"`
	Email    string   `json:"email"`
	APIToken string   `json:"api_token"`
	Project  string   `json:"project"`
	BoardID  int      `json:"board_id,omitempty"` // Agile board ID; when set, scopes results to that board
	Statuses []string `json:"statuses,omitempty"`
}

// ResolvedStatuses returns the configured statuses, or the default three if none are set.
func (j *JiraConfig) ResolvedStatuses() []string {
	if len(j.Statuses) > 0 {
		return j.Statuses
	}
	return []string{"In Progress", "Ready for CR", "Code Review"}
}

// Config holds user-level settings for exocortex.
type Config struct {
	// Repos is the list of absolute repository paths shown in the TUI repo picker.
	// Edit ~/.config/exocortex/config.json to add or remove entries.
	Repos []string `json:"repos"`

	// Profiles maps a profile name to the CLAUDE_CONFIG_DIR path for that profile.
	// Example: {"work": "~/.claude-work", "personal": "~/.claude-personal"}
	// When non-empty, the TUI shows a picker before the new-agent form.
	Profiles map[string]string `json:"profiles,omitempty"`

	// Jira holds credentials and settings for the Jira board view.
	// When nil, the board view shows a "not configured" message.
	Jira *JiraConfig `json:"jira,omitempty"`
}

// DefaultPath returns the canonical config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "exocortex", "config.json")
}

// Load reads the config from path. Returns an empty Config if the file does
// not exist; any other error is returned to the caller.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the config to path atomically (temp file + rename).
// It creates the parent directory if it does not exist.
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("config: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("config: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("config: rename: %w", err)
	}
	return nil
}
