package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "registry.json")
}

func sampleNucleus(id string) registry.Nucleus {
	return registry.Nucleus{
		ID:              id,
		TaskDescription: "task " + id,
		Neurons: []registry.Neuron{
			{
				ID:           "c1",
				Type:         registry.NeuronClaude,
				TmuxTarget:   "main:1.0",
				Status:       "idle",
				RepoPath:     "/repo",
				WorktreePath: "/repo/.worktrees/" + id,
				Branch:       "task/" + id,
			},
		},
		Status:    "idle",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

// ── Load / Save ───────────────────────────────────────────────────────────────

func TestLoad_MissingFile(t *testing.T) {
	r, err := registry.Load(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(r.Nuclei) != 0 {
		t.Fatalf("expected empty nuclei, got %d", len(r.Nuclei))
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := tempPath(t)
	n := sampleNucleus("abc123")

	reg := &registry.Registry{Version: 3, Nuclei: []registry.Nucleus{n}}
	if err := registry.Save(path, reg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Nuclei) != 1 {
		t.Fatalf("expected 1 nucleus, got %d", len(loaded.Nuclei))
	}
	got := loaded.Nuclei[0]
	if got.ID != n.ID || got.TaskDescription != n.TaskDescription {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(got.Neurons) != 1 || got.Neurons[0].ID != "c1" {
		t.Fatalf("neuron not round-tripped: %+v", got.Neurons)
	}
}

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

func TestSave_Atomic(t *testing.T) {
	path := tempPath(t)
	reg := &registry.Registry{Nuclei: []registry.Nucleus{sampleNucleus("atom01")}}
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

// ── Add / Delete ──────────────────────────────────────────────────────────────

func TestAdd(t *testing.T) {
	path := tempPath(t)
	if err := registry.Add(path, sampleNucleus("add001")); err != nil {
		t.Fatalf("add: %v", err)
	}
	loaded, _ := registry.Load(path)
	if len(loaded.Nuclei) != 1 {
		t.Fatalf("expected 1 nucleus after add, got %d", len(loaded.Nuclei))
	}
	if loaded.Nuclei[0].ID != "add001" {
		t.Fatalf("wrong id: %s", loaded.Nuclei[0].ID)
	}
}

func TestAdd_MultipleNuclei(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("first1"))
	_ = registry.Add(path, sampleNucleus("secnd2"))

	loaded, _ := registry.Load(path)
	if len(loaded.Nuclei) != 2 {
		t.Fatalf("expected 2 nuclei, got %d", len(loaded.Nuclei))
	}
}

func TestDelete(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("keep01"))
	_ = registry.Add(path, sampleNucleus("gone01"))

	if err := registry.Delete(path, "gone01"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	loaded, _ := registry.Load(path)
	if len(loaded.Nuclei) != 1 {
		t.Fatalf("expected 1 nucleus after delete, got %d", len(loaded.Nuclei))
	}
	if loaded.Nuclei[0].ID != "keep01" {
		t.Fatalf("wrong nucleus survived: %s", loaded.Nuclei[0].ID)
	}
}

func TestDelete_NotFound(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("exist1"))
	if err := registry.Delete(path, "nope00"); err == nil {
		t.Fatal("expected error deleting unknown ID")
	}
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func TestFindByID_Found(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("find01"))

	loaded, _ := registry.Load(path)
	n, err := loaded.FindByID("find01")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if n.ID != "find01" {
		t.Fatalf("wrong nucleus: %s", n.ID)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	path := tempPath(t)
	loaded, _ := registry.Load(path)
	_, err := loaded.FindByID("ghost0")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

// ── UpdateStatus ──────────────────────────────────────────────────────────────

func TestUpdateStatus(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("upd001"))

	if err := registry.UpdateStatus(path, "upd001", "working"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	loaded, _ := registry.Load(path)
	if loaded.Nuclei[0].Status != "working" {
		t.Fatalf("expected working, got %s", loaded.Nuclei[0].Status)
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	path := tempPath(t)
	if err := registry.UpdateStatus(path, "nope", "working"); err == nil {
		t.Fatal("expected error for unknown id")
	}
}

// ── AddNeuron / RemoveNeuron ──────────────────────────────────────────────────

func TestAddNeuron(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("nuclx"))

	nvim := registry.Neuron{
		ID:         "nvim",
		Type:       registry.NeuronNvim,
		TmuxTarget: "main:2.0",
		Status:     "idle",
	}
	if err := registry.AddNeuron(path, "nuclx", nvim); err != nil {
		t.Fatalf("add neuron: %v", err)
	}

	loaded, _ := registry.Load(path)
	n, _ := loaded.FindByID("nuclx")
	if len(n.Neurons) != 2 {
		t.Fatalf("expected 2 neurons, got %d", len(n.Neurons))
	}
	if n.Neurons[1].ID != "nvim" {
		t.Fatalf("wrong neuron id: %s", n.Neurons[1].ID)
	}
}

func TestAddNeuron_NucleusNotFound(t *testing.T) {
	path := tempPath(t)
	nvim := registry.Neuron{ID: "nvim", Type: registry.NeuronNvim}
	if err := registry.AddNeuron(path, "nope", nvim); err == nil {
		t.Fatal("expected error for unknown nucleus id")
	}
}

func TestRemoveNeuron(t *testing.T) {
	path := tempPath(t)
	n := sampleNucleus("nuclr")
	n.Neurons = append(n.Neurons, registry.Neuron{
		ID:     "nvim",
		Type:   registry.NeuronNvim,
		Status: "idle",
	})
	_ = registry.Add(path, n)

	if err := registry.RemoveNeuron(path, "nuclr", "nvim"); err != nil {
		t.Fatalf("remove neuron: %v", err)
	}

	loaded, _ := registry.Load(path)
	found, _ := loaded.FindByID("nuclr")
	if len(found.Neurons) != 1 {
		t.Fatalf("expected 1 neuron after remove, got %d", len(found.Neurons))
	}
	if found.Neurons[0].ID == "nvim" {
		t.Fatal("nvim neuron was not removed")
	}
}

func TestRemoveNeuron_NotFound(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("nuclq"))
	if err := registry.RemoveNeuron(path, "nuclq", "ghost"); err == nil {
		t.Fatal("expected error for unknown neuron id")
	}
}

// ── UpdateNeuronTarget ────────────────────────────────────────────────────────

func TestUpdateNeuronTarget(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("targ1"))

	if err := registry.UpdateNeuronTarget(path, "targ1", "c1", "main:3.1"); err != nil {
		t.Fatalf("update neuron target: %v", err)
	}

	loaded, _ := registry.Load(path)
	n, _ := loaded.FindByID("targ1")
	neuron, _ := n.FindNeuronByID("c1")
	if neuron.TmuxTarget != "main:3.1" {
		t.Fatalf("expected main:3.1, got %s", neuron.TmuxTarget)
	}
}

func TestUpdateNeuronTarget_NucleusNotFound(t *testing.T) {
	path := tempPath(t)
	if err := registry.UpdateNeuronTarget(path, "nope", "c1", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateNeuronTarget_NeuronNotFound(t *testing.T) {
	path := tempPath(t)
	_ = registry.Add(path, sampleNucleus("targ2"))
	if err := registry.UpdateNeuronTarget(path, "targ2", "ghost", "x"); err == nil {
		t.Fatal("expected error")
	}
}

// ── Nucleus helpers ───────────────────────────────────────────────────────────

func TestNucleus_FindNeuronByID(t *testing.T) {
	n := sampleNucleus("test1")
	neuron, err := n.FindNeuronByID("c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if neuron.Type != registry.NeuronClaude {
		t.Fatalf("expected claude type, got %s", neuron.Type)
	}
}

func TestNucleus_FindNeuronByID_NotFound(t *testing.T) {
	n := sampleNucleus("test2")
	if _, err := n.FindNeuronByID("ghost"); err == nil {
		t.Fatal("expected error for missing neuron")
	}
}

func TestNucleus_PrimaryNeuron_Claude(t *testing.T) {
	n := sampleNucleus("prim1")
	primary := n.PrimaryNeuron()
	if primary == nil {
		t.Fatal("expected a primary neuron, got nil")
	}
	if primary.Type != registry.NeuronClaude {
		t.Fatalf("expected claude, got %s", primary.Type)
	}
}

func TestNucleus_PrimaryNeuron_FallsBackToFirst(t *testing.T) {
	n := registry.Nucleus{
		ID: "prim2",
		Neurons: []registry.Neuron{
			{ID: "sh1", Type: registry.NeuronShell, Status: "idle"},
		},
	}
	primary := n.PrimaryNeuron()
	if primary == nil || primary.ID != "sh1" {
		t.Fatalf("expected sh1, got %v", primary)
	}
}

func TestNucleus_PrimaryNeuron_NoNeurons(t *testing.T) {
	n := registry.Nucleus{ID: "empty"}
	if n.PrimaryNeuron() != nil {
		t.Fatal("expected nil for nucleus with no neurons")
	}
}

func TestNeuron_Workdir_PrefersWorktree(t *testing.T) {
	neu := registry.Neuron{
		RepoPath:     "/repo",
		WorktreePath: "/repo/.worktrees/abc",
	}
	if got := neu.Workdir(); got != "/repo/.worktrees/abc" {
		t.Fatalf("expected worktree path, got %q", got)
	}
}

func TestNeuron_Workdir_FallsBackToRepo(t *testing.T) {
	neu := registry.Neuron{
		RepoPath:     "/repo",
		WorktreePath: "",
	}
	if got := neu.Workdir(); got != "/repo" {
		t.Fatalf("expected repo path when worktree is empty, got %q", got)
	}
}

func TestNucleus_Workdir_DelegatesToPrimaryNeuron(t *testing.T) {
	n := sampleNucleus("wkdir")
	if got := n.Workdir(); got != "/repo/.worktrees/wkdir" {
		t.Fatalf("expected neuron worktree path, got %q", got)
	}
}

func TestNucleus_NvimNeuron(t *testing.T) {
	n := sampleNucleus("nvm1")
	if n.NvimNeuron() != nil {
		t.Fatal("expected nil when no nvim neuron")
	}
	n.Neurons = append(n.Neurons, registry.Neuron{ID: "nvim", Type: registry.NeuronNvim})
	if n.NvimNeuron() == nil {
		t.Fatal("expected nvim neuron after append")
	}
}

// ── v1 → v3 migration ─────────────────────────────────────────────────────────

func TestMigrateV1toV3_Basic(t *testing.T) {
	// Write a v1-format registry file (no "version" key, uses "agents").
	v1JSON := `{
		"agents": [
			{
				"id":               "fixaut",
				"repo_path":        "/repo",
				"worktree_path":    "/repo/.worktrees/fixaut",
				"branch":           "agent/fixaut",
				"task_description": "Fix auth",
				"tmux_target":      "main:1.0",
				"status":           "idle",
				"created_at":       "2024-01-01T00:00:00Z"
			}
		]
	}`
	path := tempPath(t)
	if err := os.WriteFile(path, []byte(v1JSON), 0o644); err != nil {
		t.Fatalf("write v1 file: %v", err)
	}

	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load v1: %v", err)
	}
	if r.Version != 4 {
		t.Fatalf("expected version 4 after migration, got %d", r.Version)
	}
	if len(r.Nuclei) != 1 {
		t.Fatalf("expected 1 nucleus, got %d", len(r.Nuclei))
	}
	n := r.Nuclei[0]
	if n.ID != "fixaut" {
		t.Fatalf("wrong id: %s", n.ID)
	}
	if len(n.Neurons) != 1 {
		t.Fatalf("expected 1 neuron, got %d", len(n.Neurons))
	}
	neuron := n.Neurons[0]
	if neuron.Type != registry.NeuronClaude {
		t.Fatalf("expected claude neuron, got %s", neuron.Type)
	}
	if neuron.TmuxTarget != "main:1.0" {
		t.Fatalf("wrong tmux_target: %s", neuron.TmuxTarget)
	}
	if neuron.Branch != "agent/fixaut" {
		t.Fatalf("branch not migrated to neuron: %s", neuron.Branch)
	}
	if neuron.RepoPath != "/repo" {
		t.Fatalf("repo_path not migrated to neuron: %s", neuron.RepoPath)
	}
}

func TestMigrateV1toV3_WithNvimTarget(t *testing.T) {
	// An agent with an nvim_target gets two neurons after migration.
	v1JSON := `{
		"agents": [
			{
				"id":               "myagnt",
				"repo_path":        "/repo",
				"worktree_path":    "/repo/.worktrees/myagnt",
				"branch":           "agent/myagnt",
				"task_description": "Do stuff",
				"tmux_target":      "main:1.0",
				"nvim_target":      "main:2.0",
				"status":           "working",
				"created_at":       "2024-01-01T00:00:00Z"
			}
		]
	}`
	path := tempPath(t)
	_ = os.WriteFile(path, []byte(v1JSON), 0o644)

	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	n := r.Nuclei[0]
	if len(n.Neurons) != 2 {
		t.Fatalf("expected 2 neurons (claude + nvim), got %d", len(n.Neurons))
	}
	nvim := n.NvimNeuron()
	if nvim == nil {
		t.Fatal("expected an nvim neuron")
	}
	if nvim.TmuxTarget != "main:2.0" {
		t.Fatalf("wrong nvim target: %s", nvim.TmuxTarget)
	}
}

func TestMigrateV1toV3_WithProfile(t *testing.T) {
	v1JSON := `{
		"agents": [
			{
				"id": "profag",
				"repo_path": "/repo",
				"worktree_path": "/repo/.worktrees/profag",
				"branch": "agent/profag",
				"task_description": "Work task",
				"tmux_target": "main:1.0",
				"profile": "~/.claude-work",
				"status": "idle",
				"created_at": "2024-01-01T00:00:00Z"
			}
		]
	}`
	path := tempPath(t)
	_ = os.WriteFile(path, []byte(v1JSON), 0o644)

	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	n := r.Nuclei[0]
	// Profile is now on the Nucleus, not the Neuron.
	if n.Profile != "~/.claude-work" {
		t.Fatalf("profile not migrated to nucleus: %s", n.Profile)
	}
}

func TestMigrateV1toV3_EmptyAgents(t *testing.T) {
	v1JSON := `{"agents": []}`
	path := tempPath(t)
	_ = os.WriteFile(path, []byte(v1JSON), 0o644)

	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(r.Nuclei) != 0 {
		t.Fatalf("expected 0 nuclei, got %d", len(r.Nuclei))
	}
}

func TestMigrateV1toV3_SavePersistsAsV3(t *testing.T) {
	// After loading a v1 file, saving it should produce a v3 file.
	v1JSON := `{"agents": [{"id":"ag1","repo_path":"/r","worktree_path":"/r/.worktrees/ag1","branch":"b","task_description":"t","tmux_target":"m:1.0","status":"idle","created_at":"2024-01-01T00:00:00Z"}]}`
	path := tempPath(t)
	_ = os.WriteFile(path, []byte(v1JSON), 0o644)

	r, _ := registry.Load(path)
	if err := registry.Save(path, r); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Re-load and verify v2 format.
	data, _ := os.ReadFile(path)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	if _, hasAgents := raw["agents"]; hasAgents {
		t.Fatal("saved file should not contain 'agents' key")
	}
	if _, hasNuclei := raw["nuclei"]; !hasNuclei {
		t.Fatal("saved file should contain 'nuclei' key")
	}
}

// ── v2 → v3 migration ─────────────────────────────────────────────────────────

func TestMigrateV2toV3_MovesFieldsToNeuron(t *testing.T) {
	// A v2 registry has repo_path/worktree_path/branch on the Nucleus and a
	// single pr_number/pr_repo field. After migration these should appear on
	// the primary Claude Neuron and in PullRequests respectively.
	v2JSON := `{
		"version": 2,
		"nuclei": [
			{
				"id": "rev01",
				"repo_path": "/repo",
				"worktree_path": "/repo/.worktrees/rev01",
				"branch": "feat/oauth",
				"task_description": "Review PR",
				"pr_number": 42,
				"pr_repo": "owner/repo",
				"neurons": [
					{"id": "c1", "type": "claude", "tmux_target": "main:1.0", "status": "idle"}
				],
				"status": "idle",
				"created_at": "2024-01-01T00:00:00Z"
			}
		]
	}`
	path := tempPath(t)
	_ = os.WriteFile(path, []byte(v2JSON), 0o644)

	r, err := registry.Load(path)
	if err != nil {
		t.Fatalf("load v2: %v", err)
	}
	if r.Version != 4 {
		t.Fatalf("expected version 4, got %d", r.Version)
	}
	n := r.Nuclei[0]
	primary := n.PrimaryNeuron()
	if primary == nil {
		t.Fatal("expected primary neuron")
	}
	if primary.Branch != "feat/oauth" {
		t.Fatalf("branch not moved to neuron: %q", primary.Branch)
	}
	if primary.RepoPath != "/repo" {
		t.Fatalf("repo_path not moved to neuron: %q", primary.RepoPath)
	}
	if primary.WorktreePath != "/repo/.worktrees/rev01" {
		t.Fatalf("worktree_path not moved to neuron: %q", primary.WorktreePath)
	}
	if len(n.PullRequests) != 1 || n.PullRequests[0].Number != 42 {
		t.Fatalf("pr_number not moved to PullRequests: %v", n.PullRequests)
	}
	if n.PullRequests[0].Repo != "owner/repo" {
		t.Fatalf("pr_repo not moved to PullRequests: %v", n.PullRequests)
	}
}
