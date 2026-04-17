package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/config"
)

// ── GitHubConfig fields ───────────────────────────────────────────────────────

func TestGitHubConfig_NewFieldsRoundTrip(t *testing.T) {
	original := &config.Config{
		GitHub: &config.GitHubConfig{
			Token:     "tok",
			Org:       "BadgerMaps",
			MyLogin:   "ruben",
			Teammates: []string{"alice", "bob"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got config.Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.GitHub == nil {
		t.Fatal("GitHub is nil after round-trip")
	}
	if got.GitHub.MyLogin != "ruben" {
		t.Errorf("MyLogin: got %q, want 'ruben'", got.GitHub.MyLogin)
	}
	if len(got.GitHub.Teammates) != 2 {
		t.Fatalf("Teammates: got %d items, want 2", len(got.GitHub.Teammates))
	}
	if got.GitHub.Teammates[0] != "alice" || got.GitHub.Teammates[1] != "bob" {
		t.Errorf("Teammates: got %v, want [alice bob]", got.GitHub.Teammates)
	}
}

func TestGitHubConfig_EmptyTeammates_OmittedFromJSON(t *testing.T) {
	cfg := &config.Config{
		GitHub: &config.GitHubConfig{Token: "tok"},
	}
	data, _ := json.Marshal(cfg)
	if string(data) == "" {
		t.Fatal("empty marshal")
	}
	// Teammates and MyLogin should be absent (omitempty).
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	gh, _ := raw["github"].(map[string]interface{})
	if _, ok := gh["teammates"]; ok {
		t.Error("teammates key should be absent when empty")
	}
	if _, ok := gh["my_login"]; ok {
		t.Error("my_login key should be absent when empty")
	}
}

// ── GitHubRepoNames ───────────────────────────────────────────────────────────

func TestGitHubRepoNames_NilGitHub(t *testing.T) {
	cfg := &config.Config{Repos: []config.RepoConfig{{Path: "/path/to/repo"}}}
	if cfg.GitHubRepoNames() != nil {
		t.Error("want nil when GitHub is nil")
	}
}

func TestGitHubRepoNames_EmptyOrg(t *testing.T) {
	cfg := &config.Config{
		Repos:  []config.RepoConfig{{Path: "/path/to/badger-go"}},
		GitHub: &config.GitHubConfig{Token: "tok"},
	}
	if cfg.GitHubRepoNames() != nil {
		t.Error("want nil when Org is empty")
	}
}

func TestGitHubRepoNames_EmptyRepos(t *testing.T) {
	cfg := &config.Config{
		GitHub: &config.GitHubConfig{Token: "tok", Org: "BadgerMaps"},
	}
	if cfg.GitHubRepoNames() != nil {
		t.Error("want nil when Repos is empty")
	}
}

func TestGitHubRepoNames_DerivedFromPaths(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/Users/ruben/projects/badger-go"},
			{Path: "/Users/ruben/projects/badger-messenger"},
			{Path: "/Users/ruben/projects/there_geocoding"},
		},
		GitHub: &config.GitHubConfig{Token: "tok", Org: "BadgerMaps"},
	}
	got := cfg.GitHubRepoNames()
	want := []string{
		"BadgerMaps/badger-go",
		"BadgerMaps/badger-messenger",
		"BadgerMaps/there_geocoding",
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGitHubRepoNames_TrailingSlash(t *testing.T) {
	cfg := &config.Config{
		Repos:  []config.RepoConfig{{Path: "/path/to/badger-go/"}},
		GitHub: &config.GitHubConfig{Token: "tok", Org: "BadgerMaps"},
	}
	got := cfg.GitHubRepoNames()
	if len(got) != 1 || got[0] != "BadgerMaps/badger-go" {
		t.Errorf("got %v, want [BadgerMaps/badger-go]", got)
	}
}

// ── Load / Save ───────────────────────────────────────────────────────────────

func TestLoad_MissingFile_ReturnsEmpty(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil Config")
	}
}

func TestLoad_Save_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/path/to/repo", BaseBranches: []string{"main", "development"}},
		},
		GitHub: &config.GitHubConfig{
			Token:     "ghp_test",
			Org:       "BadgerMaps",
			MyLogin:   "ruben",
			Teammates: []string{"alice"},
		},
	}

	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.GitHub == nil {
		t.Fatal("loaded GitHub is nil")
	}
	if loaded.GitHub.MyLogin != "ruben" {
		t.Errorf("MyLogin: got %q, want 'ruben'", loaded.GitHub.MyLogin)
	}
	if len(loaded.GitHub.Teammates) != 1 || loaded.GitHub.Teammates[0] != "alice" {
		t.Errorf("Teammates: got %v, want [alice]", loaded.GitHub.Teammates)
	}
	if len(loaded.Repos) != 1 {
		t.Fatalf("Repos len: got %d, want 1", len(loaded.Repos))
	}
	if loaded.Repos[0].Path != "/path/to/repo" {
		t.Errorf("Repos[0].Path: got %q, want '/path/to/repo'", loaded.Repos[0].Path)
	}
	if len(loaded.Repos[0].BaseBranches) != 2 {
		t.Errorf("Repos[0].BaseBranches: got %v, want [main development]", loaded.Repos[0].BaseBranches)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "config.json")

	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

// ── Migration ─────────────────────────────────────────────────────────────────

func TestLoad_StringReposMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write legacy format with repos as plain strings.
	legacy := `{"repos":["/path/to/repo-a","/path/to/repo-b"],"profiles":{"work":"~/.claude-work"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Repos) != 2 {
		t.Fatalf("Repos len: got %d, want 2", len(cfg.Repos))
	}
	if cfg.Repos[0].Path != "/path/to/repo-a" {
		t.Errorf("Repos[0].Path: got %q, want '/path/to/repo-a'", cfg.Repos[0].Path)
	}
	if cfg.Repos[1].Path != "/path/to/repo-b" {
		t.Errorf("Repos[1].Path: got %q, want '/path/to/repo-b'", cfg.Repos[1].Path)
	}
	if cfg.Repos[0].BaseBranches != nil {
		t.Errorf("Repos[0].BaseBranches: got %v, want nil", cfg.Repos[0].BaseBranches)
	}
	// Other fields should still be parsed correctly.
	if cfg.Profiles["work"] != "~/.claude-work" {
		t.Errorf("Profiles[work]: got %q, want '~/.claude-work'", cfg.Profiles["work"])
	}
}

func TestLoad_RepoConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/abs/repo", BaseBranches: []string{"main", "hotfix"}},
		},
	}
	if err := config.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Repos) != 1 {
		t.Fatalf("Repos len: got %d, want 1", len(loaded.Repos))
	}
	if loaded.Repos[0].Path != "/abs/repo" {
		t.Errorf("Path: got %q", loaded.Repos[0].Path)
	}
	if len(loaded.Repos[0].BaseBranches) != 2 ||
		loaded.Repos[0].BaseBranches[0] != "main" ||
		loaded.Repos[0].BaseBranches[1] != "hotfix" {
		t.Errorf("BaseBranches: got %v, want [main hotfix]", loaded.Repos[0].BaseBranches)
	}
}
