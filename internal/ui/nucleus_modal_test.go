package ui

// Internal (white-box) tests for NucleusModal.
// These live in package ui so they can inspect unexported fields directly.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ── construction ──────────────────────────────────────────────────────────────

func TestNucleusModal_Defaults(t *testing.T) {
	m := NewNucleusModal(80)
	if m.focused != ModalFieldTask {
		t.Fatalf("expected focus on ModalFieldTask, got %v", m.focused)
	}
}

// ── Open ──────────────────────────────────────────────────────────────────────

func TestNucleusModal_Open_FocusesTask(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	if m.focused != ModalFieldTask {
		t.Fatalf("expected focus on ModalFieldTask after Open, got %v", m.focused)
	}
}

func TestNucleusModal_Open_JiraContext_PreFillsTask(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		JiraKey:     "PROJ-42",
		JiraSummary: "Fix auth bug",
	})
	if m.taskInput.Value() != "Fix auth bug" {
		t.Fatalf("expected task='Fix auth bug', got %q", m.taskInput.Value())
	}
}

func TestNucleusModal_Open_ResetsError(t *testing.T) {
	m := NewNucleusModal(80)
	m.err = "previous error"
	m, _ = m.Open(NucleusModalContext{})
	if m.err != "" {
		t.Fatalf("expected err cleared, got %q", m.err)
	}
}

func TestNucleusModal_Open_ReturnsBlinkCmd(t *testing.T) {
	m := NewNucleusModal(80)
	_, cmd := m.Open(NucleusModalContext{})
	if cmd == nil {
		t.Fatal("expected blink cmd from Open")
	}
}

// ── SetProfiles ───────────────────────────────────────────────────────────────

func TestNucleusModal_SetProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles(
		[]string{"work", "personal"},
		map[string]string{"work": "~/.claude-work", "personal": "~/.claude-personal"},
	)
	if !m.profilesReady {
		t.Fatal("expected profilesReady after SetProfiles")
	}
	if len(m.profileNames) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(m.profileNames))
	}
}

// ── visibleFields ─────────────────────────────────────────────────────────────

func TestNucleusModal_VisibleFields_NoProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	fields := m.visibleFields()
	want := []ModalField{ModalFieldTask}
	if len(fields) != len(want) {
		t.Fatalf("expected %d fields, got %d: %v", len(want), len(fields), fields)
	}
	for i, f := range want {
		if fields[i] != f {
			t.Fatalf("field[%d]: expected %v, got %v", i, f, fields[i])
		}
	}
}

func TestNucleusModal_VisibleFields_WithProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work"}, map[string]string{"work": "~/.claude-work"})
	fields := m.visibleFields()
	if !containsField(fields, ModalFieldProfile) {
		t.Fatal("expected ModalFieldProfile when profiles are configured")
	}
	if !containsField(fields, ModalFieldTask) {
		t.Fatal("expected ModalFieldTask always present")
	}
}

// ── Tab navigation ────────────────────────────────────────────────────────────

func TestNucleusModal_Tab_WrapsFromTask_NoProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldTask
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Only field is Task; wraps back to itself.
	if m.focused != ModalFieldTask {
		t.Fatalf("expected ModalFieldTask after Tab wrap, got %v", m.focused)
	}
}

func TestNucleusModal_Tab_AdvancesFromProfile(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work"}, map[string]string{})
	m.focused = ModalFieldProfile
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != ModalFieldTask {
		t.Fatalf("expected ModalFieldTask after Tab from Profile, got %v", m.focused)
	}
}

func TestNucleusModal_Tab_WrapsFromTaskToProfile(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work"}, map[string]string{})
	m.focused = ModalFieldTask
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != ModalFieldProfile {
		t.Fatalf("expected ModalFieldProfile after Tab wrap from Task, got %v", m.focused)
	}
}

// ── Profile navigation ────────────────────────────────────────────────────────

func TestNucleusModal_Profile_JMovesDown(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work", "personal"}, map[string]string{})
	m.focused = ModalFieldProfile
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.profileCursor != 1 {
		t.Fatalf("expected profileCursor=1, got %d", m.profileCursor)
	}
}

// ── Task input ────────────────────────────────────────────────────────────────

func TestNucleusModal_Task_TypingUpdatesInput(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldTask
	cmd := m.taskInput.Focus()
	_ = cmd
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	if !strings.Contains(m.taskInput.Value(), "hello") {
		t.Fatalf("expected 'hello' in task input, got %q", m.taskInput.Value())
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func TestNucleusModal_Submit_EmptyTask_SetsErr(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit != nil {
		t.Fatal("expected no Submit on empty task")
	}
	if m.err == "" {
		t.Fatal("expected error message on empty task")
	}
}

// ── Submit ─────────────────────────────────────────────────────────────────────

func TestNucleusModal_Submit_ReturnsTask(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("fix the bug")

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit on valid form")
	}
	if req.Submit.Task != "fix the bug" {
		t.Fatalf("expected task='fix the bug', got %q", req.Submit.Task)
	}
}

func TestNucleusModal_Submit_WithJiraKey(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		JiraKey:     "PROJ-42",
		JiraSummary: "Fix auth",
	})
	// task is pre-filled
	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit when task is pre-filled from Jira")
	}
	if req.Submit.JiraKey != "PROJ-42" {
		t.Fatalf("expected JiraKey='PROJ-42', got %q", req.Submit.JiraKey)
	}
}

func TestNucleusModal_Submit_PassesProfile(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work"}, map[string]string{"work": "~/.claude-work"})
	m.profileCursor = 0
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("some task")

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit")
	}
	if req.Submit.Profile != "work" {
		t.Fatalf("expected Profile='work', got %q", req.Submit.Profile)
	}
}

// ── Cancel ────────────────────────────────────────────────────────────────────

func TestNucleusModal_Cancel_Esc(t *testing.T) {
	m := NewNucleusModal(80)
	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !req.Cancel {
		t.Fatal("expected Cancel request from Esc")
	}
}

func TestNucleusModal_Cancel_CtrlC(t *testing.T) {
	m := NewNucleusModal(80)
	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !req.Cancel {
		t.Fatal("expected Cancel request from Ctrl+C")
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func TestNucleusModal_View_DoesNotPanic(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}

func TestNucleusModal_View_ShowsTitle(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	if !strings.Contains(m.View(), "New Nucleus") {
		t.Fatal("expected 'New Nucleus' in view")
	}
}

func TestNucleusModal_View_ShowsJiraKey(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{JiraKey: "PROJ-42"})
	if !strings.Contains(m.View(), "PROJ-42") {
		t.Fatal("expected 'PROJ-42' in view")
	}
}

func TestNucleusModal_View_ShowsError(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m.err = "task is required"
	if !strings.Contains(m.View(), "task is required") {
		t.Fatal("expected error in view")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsField(fields []ModalField, target ModalField) bool {
	for _, f := range fields {
		if f == target {
			return true
		}
	}
	return false
}
