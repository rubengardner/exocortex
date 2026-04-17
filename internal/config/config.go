package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// JiraConfig holds credentials and settings for the Jira board view.
type JiraConfig struct {
	BaseURL  string   `json:"base_url"`
	Email    string   `json:"email"`
	APIToken string   `json:"api_token"`
	Project  string   `json:"project"`
	BoardID  int      `json:"board_id,omitempty"` // Agile board ID; when set, scopes results to that board
	Statuses []string `json:"statuses,omitempty"`
	TeamID   string   `json:"team_id,omitempty"` // Jira team UUID; when set, filters board to that team only
}

// ResolvedStatuses returns the configured statuses, or the default three if none are set.
func (j *JiraConfig) ResolvedStatuses() []string {
	if len(j.Statuses) > 0 {
		return j.Statuses
	}
	return []string{"In Progress", "Ready for CR", "Code Review"}
}

// GitHubConfig holds credentials and settings for the GitHub PR view.
type GitHubConfig struct {
	Token     string   `json:"token"`
	Org       string   `json:"org,omitempty"`
	MyLogin   string   `json:"my_login,omitempty"`   // authenticated user's GitHub login
	Teammates []string `json:"teammates,omitempty"`  // teammates' GitHub logins for filter modal
}

// RepoConfig holds per-repository settings shown in the TUI repo picker.
type RepoConfig struct {
	Path         string   `json:"path"`
	BaseBranches []string `json:"base_branches,omitempty"`
}

// GitHubRepoNames returns "Org/dirname" for every path in Config.Repos.
// Used to populate the REPOSITORIES section of the GitHub filter modal.
// Returns nil when GitHub is unconfigured, Org is empty, or Repos is empty.
func (c *Config) GitHubRepoNames() []string {
	if c.GitHub == nil || c.GitHub.Org == "" || len(c.Repos) == 0 {
		return nil
	}
	names := make([]string, 0, len(c.Repos))
	for _, r := range c.Repos {
		p := strings.TrimRight(r.Path, "/")
		names = append(names, c.GitHub.Org+"/"+filepath.Base(p))
	}
	return names
}

// Config holds user-level settings for exocortex.
type Config struct {
	// Repos is the list of repository configurations shown in the TUI repo picker.
	// Edit ~/.config/exocortex/config.json to add or remove entries.
	// Legacy format (plain string paths) is auto-migrated on load.
	Repos []RepoConfig `json:"repos,omitempty"`

	// Profiles maps a profile name to the CLAUDE_CONFIG_DIR path for that profile.
	// Example: {"work": "~/.claude-work", "personal": "~/.claude-personal"}
	// When non-empty, the TUI shows a picker before the new-agent form.
	Profiles map[string]string `json:"profiles,omitempty"`

	// Jira holds credentials and settings for the Jira board view.
	// When nil, the board view shows a "not configured" message.
	Jira *JiraConfig `json:"jira,omitempty"`

	// GitHub holds credentials for the GitHub PR view.
	// When nil, the GitHub view shows a "not configured" message.
	GitHub *GitHubConfig `json:"github,omitempty"`
}

// DefaultPath returns the canonical config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "exocortex", "config.json")
}

// Load reads the config from path. Returns an empty Config if the file does
// not exist; any other error is returned to the caller.
// Legacy configs with repos as []string are silently migrated to []RepoConfig.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	// Probe the repos field independently to handle legacy []string format.
	var reposProbe struct {
		Repos json.RawMessage `json:"repos"`
	}
	_ = json.Unmarshal(data, &reposProbe)

	var repos []RepoConfig
	if len(reposProbe.Repos) > 0 && string(reposProbe.Repos) != "null" {
		if err := json.Unmarshal(reposProbe.Repos, &repos); err != nil {
			// Legacy format: clear any partial zero-value entries from the failed
			// decode attempt, then try []string migration.
			repos = nil
			var paths []string
			if err2 := json.Unmarshal(reposProbe.Repos, &paths); err2 == nil {
				for _, p := range paths {
					repos = append(repos, RepoConfig{Path: p})
				}
			}
		}
	}

	// Unmarshal remaining fields using a struct that excludes repos to avoid
	// type mismatch when the on-disk format is legacy []string.
	type configRest struct {
		Profiles map[string]string `json:"profiles,omitempty"`
		Jira     *JiraConfig       `json:"jira,omitempty"`
		GitHub   *GitHubConfig     `json:"github,omitempty"`
	}
	var rest configRest
	if err := json.Unmarshal(data, &rest); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	return &Config{
		Repos:    repos,
		Profiles: rest.Profiles,
		Jira:     rest.Jira,
		GitHub:   rest.GitHub,
	}, nil
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
