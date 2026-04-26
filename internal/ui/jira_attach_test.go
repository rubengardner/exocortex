package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/jira"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func boardNuclei() []registry.Nucleus {
	return []registry.Nucleus{
		{ID: "alpha", TaskDescription: "Alpha task", Status: "working", CreatedAt: time.Now()},
		{ID: "beta", TaskDescription: "Beta task", Status: "idle", CreatedAt: time.Now()},
	}
}

func boardIssues() ([]string, map[string][]jira.Issue) {
	cols := []string{"In Progress"}
	issues := map[string][]jira.Issue{
		"In Progress": {
			{Key: "PROJ-99", Summary: "Fix the thing", Assignee: "Alice"},
		},
	}
	return cols, issues
}

func newBoardModel(nuclei []registry.Nucleus, addJiraKey func(string, string) error) ui.Model {
	cols, issues := boardIssues()
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		CloseNvim:     func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		AddJiraKey:    addJiraKey,
		LoadJiraBoard: func() ([]string, map[string][]jira.Issue, error) {
			return cols, issues, nil
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	// Enter Jira board state.
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	// Simulate the board loading.
	m5, _ := m4.Update(ui.JiraBoardLoadedMsg(cols, issues))
	return m5.(ui.Model)
}

// ── Jira board attach flow ────────────────────────────────────────────────────

func TestJiraBoard_AKey_OpensPicker(t *testing.T) {
	m := newBoardModel(boardNuclei(), func(_, _ string) error { return nil })
	m2, _ := press(m, "a")
	got := m2.(ui.Model)
	if !got.JiraNucleusPick() {
		t.Fatal("expected jiraNucleusPick to be true after pressing 'a'")
	}
	if got.JiraPendingKey() != "PROJ-99" {
		t.Fatalf("expected pending key 'PROJ-99', got %q", got.JiraPendingKey())
	}
}

func TestJiraBoard_AKey_EscCancels(t *testing.T) {
	m := newBoardModel(boardNuclei(), func(_, _ string) error { return nil })
	m2, _ := press(m, "a")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := m3.(ui.Model)
	if got.JiraNucleusPick() {
		t.Fatal("expected jiraNucleusPick to be false after esc")
	}
}

func TestJiraBoard_AKey_EnterAttaches(t *testing.T) {
	var capturedNucleusID, capturedKey string
	m := newBoardModel(boardNuclei(), func(nucleusID, key string) error {
		capturedNucleusID = nucleusID
		capturedKey = key
		return nil
	})
	m2, _ := press(m, "a")
	// Press Enter to select the first nucleus in the picker.
	m3, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m3.(ui.Model).JiraNucleusPick() {
		t.Fatal("expected picker to close after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a cmd to be returned after Enter")
	}
	// Execute the cmd to trigger the AddJiraKey call.
	msg := cmd()
	m4, _ := m3.Update(msg)
	_ = m4
	if capturedKey != "PROJ-99" {
		t.Errorf("expected key 'PROJ-99', got %q", capturedKey)
	}
	if capturedNucleusID != "alpha" {
		t.Errorf("expected nucleus 'alpha' (first in list), got %q", capturedNucleusID)
	}
}

func TestJiraBoard_AKey_PickerRendersOverlay(t *testing.T) {
	m := newBoardModel(boardNuclei(), func(_, _ string) error { return nil })
	m2, _ := press(m, "a")
	view := m2.(ui.Model).View()
	if !strings.Contains(view, "PROJ-99") {
		t.Errorf("expected PROJ-99 in picker overlay, got:\n%s", view)
	}
	if !strings.Contains(view, "alpha") {
		t.Errorf("expected nucleus 'alpha' in picker overlay, got:\n%s", view)
	}
}

func TestJiraBoard_AKey_NoService_Noop(t *testing.T) {
	cols, issues := boardIssues()
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return boardNuclei(), nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		CloseNvim:     func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		// AddJiraKey intentionally nil
		LoadJiraBoard: func() ([]string, map[string][]jira.Issue, error) {
			return cols, issues, nil
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(boardNuclei()))
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m5, _ := m4.Update(ui.JiraBoardLoadedMsg(cols, issues))
	m6, _ := press(m5, "a")
	if m6.(ui.Model).JiraNucleusPick() {
		t.Fatal("expected no picker when AddJiraKey service is nil")
	}
}

// ── Nucleus list open Jira flow ───────────────────────────────────────────────

func newOpenJiraModel(nuclei []registry.Nucleus, openJiraKey func(string) error) ui.Model {
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		CloseNvim:     func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		OpenJiraKey:   openJiraKey,
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	return m3.(ui.Model)
}

func TestNucleusList_OKey_NoKeys_Noop(t *testing.T) {
	nuclei := []registry.Nucleus{
		{ID: "bare", TaskDescription: "No Jira", Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(_ string) error {
		t.Fatal("OpenJiraKey should not be called when no keys are linked")
		return nil
	})
	m2, _ := press(m, "o")
	if m2.(ui.Model).JiraKeyPickActive() {
		t.Fatal("expected no picker when nucleus has no JiraKeys")
	}
}

func TestNucleusList_OKey_SingleKey_DoesNotOpenPicker(t *testing.T) {
	var called string
	nuclei := []registry.Nucleus{
		{ID: "n1", TaskDescription: "One key", JiraKeys: []string{"PROJ-1"}, Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(key string) error { called = key; return nil })
	m2, cmd := press(m, "o")
	if m2.(ui.Model).JiraKeyPickActive() {
		t.Fatal("expected no picker for single Jira key")
	}
	if cmd == nil {
		t.Fatal("expected a cmd to open browser")
	}
	cmd() // execute to trigger OpenJiraKey
	if called != "PROJ-1" {
		t.Errorf("expected OpenJiraKey called with 'PROJ-1', got %q", called)
	}
}

func TestNucleusList_OKey_MultiKey_Picker(t *testing.T) {
	nuclei := []registry.Nucleus{
		{ID: "n1", TaskDescription: "Multi", JiraKeys: []string{"PROJ-1", "PROJ-2"}, Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(_ string) error { return nil })
	m2, _ := press(m, "o")
	if !m2.(ui.Model).JiraKeyPickActive() {
		t.Fatal("expected picker to open for multiple Jira keys")
	}
}

func TestNucleusList_OKey_PickerRendersKeys(t *testing.T) {
	nuclei := []registry.Nucleus{
		{ID: "n1", TaskDescription: "Multi", JiraKeys: []string{"PROJ-1", "PROJ-2"}, Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(_ string) error { return nil })
	m2, _ := press(m, "o")
	view := m2.(ui.Model).View()
	if !strings.Contains(view, "PROJ-1") {
		t.Errorf("expected PROJ-1 in picker view, got:\n%s", view)
	}
	if !strings.Contains(view, "PROJ-2") {
		t.Errorf("expected PROJ-2 in picker view, got:\n%s", view)
	}
}

func TestNucleusList_OKey_PickerEscCancels(t *testing.T) {
	nuclei := []registry.Nucleus{
		{ID: "n1", TaskDescription: "Multi", JiraKeys: []string{"PROJ-1", "PROJ-2"}, Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(_ string) error { return nil })
	m2, _ := press(m, "o")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m3.(ui.Model).JiraKeyPickActive() {
		t.Fatal("expected picker to close after esc")
	}
}

func TestNucleusList_OKey_PickerSelectOpens(t *testing.T) {
	var called string
	nuclei := []registry.Nucleus{
		{ID: "n1", TaskDescription: "Multi", JiraKeys: []string{"PROJ-1", "PROJ-2"}, Status: "idle", CreatedAt: time.Now()},
	}
	m := newOpenJiraModel(nuclei, func(key string) error { called = key; return nil })
	// Open picker, move down to second key, press enter.
	m2, _ := press(m, "o")
	m3, _ := press(m2, "j")
	m4, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m4.(ui.Model).JiraKeyPickActive() {
		t.Fatal("expected picker to close after Enter")
	}
	if cmd != nil {
		cmd()
	}
	if called != "PROJ-2" {
		t.Errorf("expected OpenJiraKey called with 'PROJ-2' (second key), got %q", called)
	}
}
