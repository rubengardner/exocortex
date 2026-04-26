package ui_test

// Tests for Phase 6: Jira ↔ Nucleus integration.
//
// Covers:
//   - N key on Jira board transitions to new-nucleus form
//   - Form is pre-filled with the issue summary and task/<key>/ branch prefix
//   - Submitting the form calls CreateNucleus with the correct JiraKey
//   - Cancelling the form returns to the Jira board (not the main list)
//   - Nucleus detail view fires metadata load and renders Jira section

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

// sampleJiraColumns and sampleJiraIssues define the canned board used in
// several tests.
var sampleJiraColumns = []string{"In Progress", "Code Review"}

func sampleJiraIssues() map[string][]jira.Issue {
	return map[string][]jira.Issue{
		"In Progress": {
			{Key: "PROJ-42", Summary: "Fix authentication bug", Status: "In Progress", Assignee: "Alice Smith", URL: "https://jira.example.com/browse/PROJ-42"},
		},
		"Code Review": {},
	}
}

// newJiraBoardModel returns a Model that is in StateJiraBoard with the canned
// board loaded, using no repo picker and no profile picker.
func newJiraBoardModel(t *testing.T, extraSvc func(*ui.Services)) ui.Model {
	t.Helper()

	columns := sampleJiraColumns
	issues := sampleJiraIssues()

	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nil, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadJiraBoard: func() ([]string, map[string][]jira.Issue, error) {
			return columns, issues, nil
		},
	}
	if extraSvc != nil {
		extraSvc(&svc)
	}

	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open board (triggers async load).
	m3, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if cmd == nil {
		t.Fatal("expected loadJiraBoardCmd from 'b' key")
	}
	// Drain the async load.
	m4, _ := m3.Update(cmd())
	return m4.(ui.Model)
}

// ── N key: board → form ───────────────────────────────────────────────────────

func TestJiraBoard_NKey_OpensNewNucleusForm(t *testing.T) {
	m := newJiraBoardModel(t, nil)

	if m.State() != ui.StateJiraBoard {
		t.Fatalf("prerequisite: expected StateJiraBoard, got %v", m.State())
	}

	// Press N — no repo picker, no profile picker → should land on form.
	m2, _ := press(m, "N")
	if m2.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("expected StateNewOverlay after N, got %v", m2.(ui.Model).State())
	}
}

func TestJiraBoard_NKey_ViewShowsJiraKey(t *testing.T) {
	m := newJiraBoardModel(t, nil)
	m2, _ := press(m, "N")

	view := m2.(ui.Model).View()
	if !strings.Contains(view, "PROJ-42") {
		t.Fatalf("expected 'PROJ-42' in form view, got:\n%s", view)
	}
}

func TestJiraBoard_NKey_FormPrefillsTaskWithSummary(t *testing.T) {
	m := newJiraBoardModel(t, nil)
	m2, _ := press(m, "N")

	view := m2.(ui.Model).View()
	if !strings.Contains(view, "Fix authentication bug") {
		t.Fatalf("expected issue summary in form view, got:\n%s", view)
	}
}

func TestJiraBoard_NKey_FormPrefillsTaskFromSummary(t *testing.T) {
	// Branch prefix is no longer shown in develop mode (no neuron fields).
	// Task description is pre-filled from the Jira summary.
	m := newJiraBoardModel(t, nil)
	m2, _ := press(m, "N")

	view := m2.(ui.Model).View()
	if !strings.Contains(view, "Fix authentication bug") {
		t.Fatalf("expected issue summary pre-filled in task field, got:\n%s", view)
	}
}

// ── cancel from Jira form returns to board ────────────────────────────────────

func TestJiraForm_EscReturnsToJiraBoard(t *testing.T) {
	m := newJiraBoardModel(t, nil)
	m2, _ := press(m, "N") // enter form
	if m2.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("prerequisite: expected StateNewOverlay, got %v", m2.(ui.Model).State())
	}
	m3, _ := pressSpecial(m2, tea.KeyEsc)
	if m3.(ui.Model).State() != ui.StateJiraBoard {
		t.Fatalf("expected StateJiraBoard after esc from Jira form, got %v", m3.(ui.Model).State())
	}
}

// ── submit passes jiraKey to CreateNucleus ─────────────────────────────────────

func TestJiraForm_Submit_PassesJiraKey(t *testing.T) {
	var capturedJiraKey string

	m := newJiraBoardModel(t, func(svc *ui.Services) {
		svc.CreateNucleus = func(task, jiraKey, profile string) error {
			capturedJiraKey = jiraKey
			return nil
		}
	})

	m2, _ := press(m, "N")
	if m2.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("prerequisite: expected StateNewOverlay, got %v", m2.(ui.Model).State())
	}

	// The task field is pre-filled with the issue summary; pressing Enter should submit.
	_, cmd := pressSpecial(m2, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected a cmd from Enter on the form")
	}
	cmd() // executes CreateNucleus

	if capturedJiraKey != "PROJ-42" {
		t.Fatalf("expected JiraKey 'PROJ-42' passed to CreateNucleus, got %q", capturedJiraKey)
	}
}

func TestJiraForm_Submit_PassesEmptyKeyForAdHoc(t *testing.T) {
	var capturedJiraKey string
	nuclei := sampleNuclei()

	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, jiraKey, profile string) error { capturedJiraKey = jiraKey; return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	// Open form via n (ad-hoc, no Jira).
	m4, _ := press(m3, "n")
	if m4.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("prerequisite: expected StateNewOverlay, got %v", m4.(ui.Model).State())
	}

	// Type a task and submit.
	m5, _ := m4.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my task")})
	_, cmd := pressSpecial(m5, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	cmd()

	if capturedJiraKey != "" {
		t.Fatalf("expected empty JiraKey for ad-hoc nucleus, got %q", capturedJiraKey)
	}
}

// ── nucleus detail: Jira metadata loading ─────────────────────────────────────

func TestNucleusDetail_WithJiraKey_LoadsMetadata(t *testing.T) {
	var loadedKey string
	nuclei := []registry.Nucleus{
		{
			ID: "auth1", TaskDescription: "Fix auth bug",
			JiraKeys: []string{"PROJ-42"}, Status: "working", CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "working"},
			},
		},
	}

	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		LoadJiraIssueMeta: func(key string) (*jira.Issue, error) {
			loadedKey = key
			return &jira.Issue{Key: key, Summary: "Fix authentication bug", Status: "In Progress"}, nil
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	// Enter detail view.
	m4, cmds := pressSpecial(m3, tea.KeyEnter)
	if m4.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail, got %v", m4.(ui.Model).State())
	}

	// Execute all commands and drain metadata load.
	if cmds != nil {
		msg := cmds()
		if msg != nil {
			m4, _ = m4.Update(msg)
		}
	}

	if loadedKey != "PROJ-42" {
		t.Fatalf("expected LoadJiraIssueMeta called with 'PROJ-42', got %q", loadedKey)
	}
}

func TestNucleusDetail_JiraMetadata_RendersInView(t *testing.T) {
	issue := &jira.Issue{
		Key: "PROJ-42", Summary: "Fix authentication bug",
		Status: "In Progress", Assignee: "Alice Smith",
		URL: "https://jira.example.com/browse/PROJ-42",
	}
	nuclei := []registry.Nucleus{
		{
			ID: "auth1", TaskDescription: "Fix auth bug",
			JiraKeys: []string{"PROJ-42"}, Status: "working", CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "working"},
			},
		},
	}

	metaCmd := make(chan tea.Cmd, 1)
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		LoadJiraIssueMeta: func(key string) (*jira.Issue, error) {
			return issue, nil
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	close(metaCmd)

	// Enter detail view — this fires loadBranchInfoCmd + loadJiraIssueMetaCmd.
	m4, cmd := pressSpecial(m3, tea.KeyEnter)
	if m4.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail, got %v", m4.(ui.Model).State())
	}

	// Drain the batched commands by executing until we get the meta message.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			m4, _ = m4.Update(msg)
		}
	}

	view := m4.(ui.Model).View()
	if !strings.Contains(view, "JIRA PROJ-42") {
		t.Fatalf("expected 'JIRA PROJ-42' in detail view, got:\n%s", view)
	}
}

func TestNucleusDetail_NoJiraKey_NoBranchInfoJiraSection(t *testing.T) {
	nuclei := []registry.Nucleus{
		{
			ID: "task1", TaskDescription: "Ad-hoc task",
			Status: "idle", CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "idle"},
			},
		},
	}

	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		LoadJiraIssueMeta: func(key string) (*jira.Issue, error) {
			return nil, nil // should not be called for a nucleus without JiraKey
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	m4, _ := pressSpecial(m3, tea.KeyEnter)
	if m4.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail, got %v", m4.(ui.Model).State())
	}

	view := m4.(ui.Model).View()
	if strings.Contains(view, "JIRA ") {
		t.Fatalf("expected no 'JIRA' section for nucleus without JiraKey, got:\n%s", view)
	}
}
