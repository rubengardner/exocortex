package hooks_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/hooks"
)

const testDir = ".claude-work"

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := hooks.Write(dir, "abc123", testDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	path := filepath.Join(dir, testDir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	hooksRaw, ok := got["hooks"]
	if !ok {
		t.Fatal("expected 'hooks' key in settings.json")
	}
	hooksMap := hooksRaw.(map[string]any)
	if _, ok := hooksMap["UserPromptSubmit"]; !ok {
		t.Fatal("expected UserPromptSubmit hook")
	}
	if _, ok := hooksMap["Stop"]; !ok {
		t.Fatal("expected Stop hook")
	}
}

func TestWrite_CorrectHookFormat(t *testing.T) {
	// Claude Code requires: [{"matcher": "...", "hooks": [{...}]}]
	// NOT bare: [{"type": "...", "command": "..."}]
	dir := t.TempDir()
	if err := hooks.Write(dir, "abc123", testDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, testDir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(data, &got)

	hooksMap := got["hooks"].(map[string]any)
	eventArr := hooksMap["UserPromptSubmit"].([]any)
	if len(eventArr) == 0 {
		t.Fatal("expected at least one event entry")
	}
	entry := eventArr[0].(map[string]any)
	if _, ok := entry["matcher"]; !ok {
		t.Fatal("expected 'matcher' key in hook entry")
	}
	if _, ok := entry["hooks"]; !ok {
		t.Fatal("expected 'hooks' array in hook entry")
	}
	innerHooks := entry["hooks"].([]any)
	if len(innerHooks) == 0 {
		t.Fatal("expected at least one inner hook command")
	}
	cmd := innerHooks[0].(map[string]any)
	if cmd["type"] != "command" {
		t.Fatalf("expected type=command, got %v", cmd["type"])
	}
}

func TestWrite_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	if err := hooks.Write(dir, "abc123", testDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	claudeDir := filepath.Join(dir, testDir)
	info, err := os.Stat(claudeDir)
	if err != nil {
		t.Fatalf("%s dir not created: %v", testDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", testDir)
	}
}

func TestWrite_MergesExisting(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, testDir)
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write existing settings.json with an unrelated key.
	existing := map[string]any{
		"permissions": map[string]any{"allow": []string{"*"}},
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := hooks.Write(dir, "abc123", testDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var got map[string]any
	_ = json.Unmarshal(result, &got)

	if _, ok := got["permissions"]; !ok {
		t.Fatal("existing 'permissions' key was clobbered")
	}
	if _, ok := got["hooks"]; !ok {
		t.Fatal("'hooks' key was not written")
	}
}

func TestWrite_HookCommandsContainAgentID(t *testing.T) {
	dir := t.TempDir()
	if err := hooks.Write(dir, "myagent", testDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, testDir, "settings.json"))
	if !strings.Contains(string(data), "myagent") {
		t.Fatal("expected agent ID in hook commands")
	}
}

func TestWrite_CustomDirName(t *testing.T) {
	dir := t.TempDir()
	if err := hooks.Write(dir, "abc123", ".claude"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err != nil {
		t.Fatal("expected .claude/settings.json to exist")
	}
}
