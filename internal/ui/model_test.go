package ui_test

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestModel(nuclei []registry.Nucleus) ui.Model {
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		CloseNvim:      func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	return m3.(ui.Model)
}

// nucleiLoaded simulates the async load completing by running Init and draining the Cmd.
func nucleiLoaded(nuclei []registry.Nucleus) tea.Msg {
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		CloseNvim:      func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	cmd := m.Init()
	return cmd() // executes LoadNuclei and returns nucleiLoadedMsg
}

func press(m tea.Model, key string) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func pressSpecial(m tea.Model, t tea.KeyType) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyMsg{Type: t})
}

func sampleNuclei() []registry.Nucleus {
	return []registry.Nucleus{
		{
			ID: "nucl1", Branch: "task/nucl1", TaskDescription: "First task",
			Status: "idle", CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "idle"},
			},
		},
		{
			ID: "nucl2", Branch: "task/nucl2", TaskDescription: "Second task",
			Status: "working", CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.1", Status: "working"},
			},
		},
	}
}

// ── navigation tests ──────────────────────────────────────────────────────────

func TestCursorMovesDown(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "j")
	if m2.(ui.Model).Cursor() != 1 {
		t.Fatalf("expected cursor=1 after j, got %d", m2.(ui.Model).Cursor())
	}
}

func TestCursorDoesNotGoBelowBottom(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "j")
	m3, _ := press(m2, "j") // already at bottom
	if m3.(ui.Model).Cursor() != 1 {
		t.Fatalf("expected cursor clamped at 1, got %d", m3.(ui.Model).Cursor())
	}
}

func TestCursorMovesUp(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "j")  // cursor=1
	m3, _ := press(m2, "k") // back to 0
	if m3.(ui.Model).Cursor() != 0 {
		t.Fatalf("expected cursor=0 after k, got %d", m3.(ui.Model).Cursor())
	}
}

func TestCursorDoesNotGoAboveTop(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "k") // already at top
	if m2.(ui.Model).Cursor() != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", m2.(ui.Model).Cursor())
	}
}

// ── quit ──────────────────────────────────────────────────────────────────────

func TestQuitReturnsQuitCmd(t *testing.T) {
	m := newTestModel(sampleNuclei())
	_, cmd := press(m, "q")
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// ── nvim ──────────────────────────────────────────────────────────────────────

func TestNvim_CallsOpenNvim(t *testing.T) {
	var calledID string
	nuclei := sampleNuclei()
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { calledID = id; return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	_, cmd := press(m3, "e")
	if cmd == nil {
		t.Fatal("expected cmd from e key")
	}
	cmd() // executes OpenNvim
	if calledID != "nucl1" {
		t.Fatalf("expected OpenNvim called with nucl1, got %q", calledID)
	}
}

func TestNvimError_SetsLastErr(t *testing.T) {
	nuclei := sampleNuclei()
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return errors.New("nvim gone") },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	m4, cmd := press(m3, "e")
	if cmd == nil {
		t.Fatal("expected cmd from e key")
	}
	msg := cmd()
	m5, _ := m4.Update(msg)
	if m5.(ui.Model).LastErr() != "nvim gone" {
		t.Fatalf("expected lastErr 'nvim gone', got %q", m5.(ui.Model).LastErr())
	}
}

// ── new overlay ───────────────────────────────────────────────────────────────

func TestNewOverlay_OpensOnN(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "n")
	if m2.(ui.Model).State() != ui.StateNewOverlay {
		t.Fatalf("expected stateNewOverlay, got %v", m2.(ui.Model).State())
	}
}

func TestNewOverlay_EscCancels(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "n")
	m3, _ := pressSpecial(m2, tea.KeyEsc)
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected stateList after esc, got %v", m3.(ui.Model).State())
	}
}

// ── confirm delete ────────────────────────────────────────────────────────────

func TestDelete_OpensConfirmOnD(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "d")
	if m2.(ui.Model).State() != ui.StateConfirmDelete {
		t.Fatalf("expected stateConfirmDelete, got %v", m2.(ui.Model).State())
	}
}

func TestDelete_CancelOnAnyOtherKey(t *testing.T) {
	m := newTestModel(sampleNuclei())
	m2, _ := press(m, "d")
	m3, _ := press(m2, "n") // 'n' cancels
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected stateList after cancel, got %v", m3.(ui.Model).State())
	}
}

func TestDelete_ConfirmCallsRemove(t *testing.T) {
	removed := ""
	nuclei := sampleNuclei()
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile string) error { return nil },
		RemoveNucleus:  func(id string) error { removed = id; return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		CloseNvim:      func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	m4, _ := press(m3, "d")
	_, cmd := press(m4, "y")
	if cmd == nil {
		t.Fatal("expected a cmd from confirm")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("cmd returned nil msg")
	}
	if removed != "nucl1" {
		t.Fatalf("expected nucl1 removed, got %q", removed)
	}
}

// ── view smoke tests ──────────────────────────────────────────────────────────

func TestView_DoesNotPanic(t *testing.T) {
	m := newTestModel(sampleNuclei())
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}

func TestView_EmptyNuclei_DoesNotPanic(t *testing.T) {
	m := newTestModel(nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked with empty nuclei: %v", r)
		}
	}()
	_ = m.View()
}
