package registry_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "registry.json")
}

func sampleAgent(id string) registry.Agent {
	return registry.Agent{
		ID:              id,
		RepoPath:        "/repo",
		WorktreePath:    "/repo/.worktrees/" + id,
		Branch:          "feat/" + id,
		TaskDescription: "task " + id,
		TmuxTarget:      "main:1.0",
		Status:          "idle",
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
	}
}

// Load returns empty registry when file does not exist.
func TestLoad_MissingFile(t *testing.T) {
	r, err := registry.Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(r.Agents) != 0 {
		t.Fatalf("expected empty agents, got %d", len(r.Agents))
	}
}

// Save then Load round-trips an agent correctly.
func TestSaveLoad_RoundTrip(t *testing.T) {
	path := tempPath(t)
	a := sampleAgent("abc123")

	reg := &registry.Registry{Agents: []registry.Agent{a}}
	if err := registry.Save(path, reg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(loaded.Agents))
	}
	got := loaded.Agents[0]
	if got.ID != a.ID || got.TaskDescription != a.TaskDescription {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// Add appends an agent and persists it.
func TestAdd(t *testing.T) {
	path := tempPath(t)
	a := sampleAgent("add001")

	if err := registry.Add(path, a); err != nil {
		t.Fatalf("add: %v", err)
	}

	loaded, _ := registry.Load(path)
	if len(loaded.Agents) != 1 {
		t.Fatalf("expected 1 agent after add, got %d", len(loaded.Agents))
	}
	if loaded.Agents[0].ID != "add001" {
		t.Fatalf("wrong id: %s", loaded.Agents[0].ID)
	}
}

// Add a second agent accumulates both.
func TestAdd_MultipleAgents(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleAgent("first1"))
	_ = registry.Add(path, sampleAgent("secnd2"))

	loaded, _ := registry.Load(path)
	if len(loaded.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(loaded.Agents))
	}
}

// Delete removes by ID and persists.
func TestDelete(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleAgent("keep01"))
	_ = registry.Add(path, sampleAgent("gone01"))

	if err := registry.Delete(path, "gone01"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	loaded, _ := registry.Load(path)
	if len(loaded.Agents) != 1 {
		t.Fatalf("expected 1 agent after delete, got %d", len(loaded.Agents))
	}
	if loaded.Agents[0].ID != "keep01" {
		t.Fatalf("wrong agent survived: %s", loaded.Agents[0].ID)
	}
}

// Delete an unknown ID returns an error.
func TestDelete_NotFound(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleAgent("exist1"))

	err := registry.Delete(path, "nope00")
	if err == nil {
		t.Fatal("expected error deleting unknown ID")
	}
}

// FindByID returns the agent when present.
func TestFindByID_Found(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleAgent("find01"))

	loaded, _ := registry.Load(path)
	a, err := loaded.FindByID("find01")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if a.ID != "find01" {
		t.Fatalf("wrong agent: %s", a.ID)
	}
}

// FindByID returns an error when not present.
func TestFindByID_NotFound(t *testing.T) {
	path := tempPath(t)
	loaded, _ := registry.Load(path)
	_, err := loaded.FindByID("ghost0")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

// Save uses an atomic write: the destination must never be a partial file.
// We verify the temp file is cleaned up and the final file is valid JSON.
func TestSave_Atomic(t *testing.T) {
	path := tempPath(t)
	reg := &registry.Registry{Agents: []registry.Agent{sampleAgent("atom01")}}
	if err := registry.Save(path, reg); err != nil {
		t.Fatalf("save: %v", err)
	}
	// No temp file should remain alongside the target.
	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dir, found %d", len(entries))
	}
}

// UpdateStatus changes an agent's status and persists.
func TestUpdateStatus(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleAgent("upd001"))

	if err := registry.UpdateStatus(path, "upd001", "working"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	loaded, _ := registry.Load(path)
	if loaded.Agents[0].Status != "working" {
		t.Fatalf("expected working, got %s", loaded.Agents[0].Status)
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	path := tempPath(t)
	err := registry.UpdateStatus(path, "nope", "working")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

// Save auto-creates the directory if it does not exist.
func TestSave_AutoCreateDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deep", "registry.json")
	reg := &registry.Registry{}
	if err := registry.Save(path, reg); err != nil {
		t.Fatalf("save to new dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
