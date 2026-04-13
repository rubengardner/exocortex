package ui_test

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// --- helpers -----------------------------------------------------------------

func newTestModel(agents []registry.Agent) ui.Model {
	svc := ui.Services{
		LoadAgents:   func() ([]registry.Agent, error) { return agents, nil },
		CreateAgent:  func(task, repo, branch, profile string) error { return nil },
		RemoveAgent:  func(id string) error { return nil },
		GotoAgent:    func(id string) error { return nil },
		OpenNvim:     func(id string) error { return nil },
		CloseNvim:    func(id string) error { return nil },
		RespawnAgent: func(id string) error { return nil },
	}
	m := ui.New(svc)
	// Inject agents and a terminal size directly via the message path.
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(agentsLoaded(agents))
	return m3.(ui.Model)
}

// agentsLoaded simulates the async load completing by running Init and draining the Cmd.
// For tests we inject directly via a fake message.
func agentsLoaded(agents []registry.Agent) tea.Msg {
	// We cannot import the private agentsLoadedMsg, so we call Init+Run.
	// Instead, we bypass this by calling Update with a WindowSizeMsg and
	// trusting that newTestModel already ran Init.  We use a thin approach:
	// create the model, call Init to get the cmd, run it to get the msg, feed back.
	svc := ui.Services{
		LoadAgents:   func() ([]registry.Agent, error) { return agents, nil },
		CreateAgent:  func(task, repo, branch, profile string) error { return nil },
		RemoveAgent:  func(id string) error { return nil },
		GotoAgent:    func(id string) error { return nil },
		OpenNvim:     func(id string) error { return nil },
		CloseNvim:    func(id string) error { return nil },
		RespawnAgent: func(id string) error { return nil },
	}
	m := ui.New(svc)
	cmd := m.Init()
	return cmd() // executes LoadAgents and returns agentsLoadedMsg
}

func press(m tea.Model, key string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func pressSpecial(m tea.Model, t tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: t})
}

func sampleAgents() []registry.Agent {
	return []registry.Agent{
		{ID: "agent1", Branch: "feat/agent1", TaskDescription: "First task", Status: "idle", TmuxTarget: "main:1.0", CreatedAt: time.Now()},
		{ID: "agent2", Branch: "feat/agent2", TaskDescription: "Second task", Status: "working", TmuxTarget: "main:1.1", CreatedAt: time.Now()},
	}
}

// --- navigation tests --------------------------------------------------------

func TestCursorMovesDown(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "j")
	if m2.(ui.Model).Cursor() != 1 {
		t.Fatalf("expected cursor=1 after j, got %d", m2.(ui.Model).Cursor())
	}
}

func TestCursorDoesNotGoBelowBottom(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "j")
	m3, _ := press(m2, "j") // already at bottom
	if m3.(ui.Model).Cursor() != 1 {
		t.Fatalf("expected cursor clamped at 1, got %d", m3.(ui.Model).Cursor())
	}
}

func TestCursorMovesUp(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "j")  // cursor=1
	m3, _ := press(m2, "k") // back to 0
	if m3.(ui.Model).Cursor() != 0 {
		t.Fatalf("expected cursor=0 after k, got %d", m3.(ui.Model).Cursor())
	}
}

func TestCursorDoesNotGoAboveTop(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "k") // already at top
	if m2.(ui.Model).Cursor() != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", m2.(ui.Model).Cursor())
	}
}

// --- quit --------------------------------------------------------------------

func TestQuitReturnsQuitCmd(t *testing.T) {
	m := newTestModel(sampleAgents())
	_, cmd := press(m, "q")
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	// tea.Quit returns a tea.QuitMsg when invoked.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// --- nvim --------------------------------------------------------------------

func TestNvim_CallsOpenNvim(t *testing.T) {
	var calledID string
	agents := sampleAgents()
	svc := ui.Services{
		LoadAgents:   func() ([]registry.Agent, error) { return agents, nil },
		CreateAgent:  func(task, repo, branch, profile string) error { return nil },
		RemoveAgent:  func(id string) error { return nil },
		GotoAgent:    func(id string) error { return nil },
		OpenNvim:     func(id string) error { calledID = id; return nil },
		RespawnAgent: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(agentsLoaded(agents))

	_, cmd := press(m3, "e")
	if cmd == nil {
		t.Fatal("expected cmd from e key")
	}
	cmd() // executes OpenNvim
	if calledID != "agent1" {
		t.Fatalf("expected OpenNvim called with agent1, got %q", calledID)
	}
}

func TestNvimError_SetsLastErr(t *testing.T) {
	agents := sampleAgents()
	svc := ui.Services{
		LoadAgents:   func() ([]registry.Agent, error) { return agents, nil },
		CreateAgent:  func(task, repo, branch, profile string) error { return nil },
		RemoveAgent:  func(id string) error { return nil },
		GotoAgent:    func(id string) error { return nil },
		OpenNvim:     func(id string) error { return errors.New("nvim gone") },
		RespawnAgent: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(agentsLoaded(agents))

	m4, cmd := press(m3, "e")
	if cmd == nil {
		t.Fatal("expected cmd from e key")
	}
	msg := cmd() // executes OpenNvim, returns actionDoneMsg{err: ...}
	m5, _ := m4.Update(msg)
	if m5.(ui.Model).LastErr() != "nvim gone" {
		t.Fatalf("expected lastErr 'nvim gone', got %q", m5.(ui.Model).LastErr())
	}
}

// --- new overlay -------------------------------------------------------------

func TestNewOverlay_OpensOnN(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "n")
	if m2.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("expected stateNewOverlay, got %v", m2.(ui.Model).State())
	}
}

func TestNewOverlay_EscCancels(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "n")
	m3, _ := pressSpecial(m2, tea.KeyEsc)
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected stateList after esc, got %v", m3.(ui.Model).State())
	}
}

// --- confirm delete ----------------------------------------------------------

func TestDelete_OpensConfirmOnD(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "d")
	if m2.(ui.Model).State() != ui.StateConfirmDelete {
		t.Fatalf("expected stateConfirmDelete, got %v", m2.(ui.Model).State())
	}
}

func TestDelete_CancelOnAnyOtherKey(t *testing.T) {
	m := newTestModel(sampleAgents())
	m2, _ := press(m, "d")
	m3, _ := press(m2, "n") // 'n' cancels
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected stateList after cancel, got %v", m3.(ui.Model).State())
	}
}

func TestDelete_ConfirmCallsRemove(t *testing.T) {
	removed := ""
	agents := sampleAgents()
	svc := ui.Services{
		LoadAgents:   func() ([]registry.Agent, error) { return agents, nil },
		CreateAgent:  func(task, repo, branch, profile string) error { return nil },
		RemoveAgent:  func(id string) error { removed = id; return nil },
		GotoAgent:    func(id string) error { return nil },
		OpenNvim:     func(id string) error { return nil },
		CloseNvim:    func(id string) error { return nil },
		RespawnAgent: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(agentsLoaded(agents))

	m4, _ := press(m3, "d")      // open confirm
	_, cmd := press(m4, "y")     // confirm
	if cmd == nil {
		t.Fatal("expected a cmd from confirm")
	}
	msg := cmd() // execute the remove
	if msg == nil {
		t.Fatal("cmd returned nil msg")
	}
	if removed != "agent1" {
		t.Fatalf("expected agent1 removed, got %q", removed)
	}
}

// --- view smoke test ---------------------------------------------------------

func TestView_DoesNotPanic(t *testing.T) {
	m := newTestModel(sampleAgents())
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}

func TestView_EmptyAgents_DoesNotPanic(t *testing.T) {
	m := newTestModel(nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked with empty agents: %v", r)
		}
	}()
	_ = m.View()
}
