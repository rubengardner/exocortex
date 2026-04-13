package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookCommand is one entry in the inner "hooks" array.
type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookMatcher is one element in the event array — a matcher + hooks pair.
// Claude Code format:
//
//	"UserPromptSubmit": [{"matcher": "", "hooks": [{...}]}]
type hookMatcher struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookCommand `json:"hooks"`
}

// Write writes Claude Code hook configuration into <worktreePath>/<dirName>/settings.json.
// Use dirName ".claude" for standard Claude Code layout or ".claude-work" for a custom one.
// If the file already exists its non-hook content is preserved.
// Failure is non-fatal — callers should warn, not abort.
func Write(worktreePath, agentID, dirName string) error {
	claudeDir := filepath.Join(worktreePath, dirName)
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("hooks: mkdir %s: %w", dirName, err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Load existing settings if present — preserve unknown keys.
	existing := map[string]json.RawMessage{}
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Build event arrays using the matcher+hooks structure Claude Code expects.
	userPromptEntry, _ := json.Marshal([]hookMatcher{
		{Matcher: "", Hooks: []hookCommand{
			{Type: "command", Command: "exocortex status " + agentID + " working"},
		}},
	})
	stopEntry, _ := json.Marshal([]hookMatcher{
		{Matcher: "", Hooks: []hookCommand{
			{Type: "command", Command: "exocortex status " + agentID + " waiting; printf '\\a'"},
		}},
	})

	// Merge into existing hooks map, preserving other hook events.
	hooksMap := map[string]json.RawMessage{}
	if raw, ok := existing["hooks"]; ok {
		_ = json.Unmarshal(raw, &hooksMap)
	}
	hooksMap["UserPromptSubmit"] = userPromptEntry
	hooksMap["Stop"] = stopEntry

	hooksRaw, err := json.Marshal(hooksMap)
	if err != nil {
		return fmt.Errorf("hooks: marshal hooks: %w", err)
	}
	existing["hooks"] = hooksRaw

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("hooks: marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("hooks: write settings.json: %w", err)
	}
	return nil
}
