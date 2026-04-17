package ui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// newDetailModel builds a Model loaded with sampleNuclei and the AddNeuron + GotoNeuron services.
func newDetailModel(addNeuronFn func(nucleusID, neuronType, profile string) error, gotoNeuronFn func(nucleusID, neuronID string) error) ui.Model {
	nuclei := sampleNuclei()
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, repo, branch, profile, jiraKey string, createWorktree bool) error { return nil },
		RemoveNucleus:  func(id string) error { return nil },
		GotoNucleus:    func(id string) error { return nil },
		OpenNvim:       func(id string) error { return nil },
		CloseNvim:      func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
		AddNeuron:      addNeuronFn,
		GotoNeuron:     gotoNeuronFn,
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	return m3.(ui.Model)
}

// ── open / close ──────────────────────────────────────────────────────────────

func TestNucleusDetail_OpenOnEnter(t *testing.T) {
	m := newDetailModel(nil, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	if m2.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail after enter, got %v", m2.(ui.Model).State())
	}
}

func TestNucleusDetail_OpenOnRightArrow(t *testing.T) {
	m := newDetailModel(nil, nil)
	m2, _ := pressSpecial(m, tea.KeyRight)
	if m2.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail after →, got %v", m2.(ui.Model).State())
	}
}

func TestNucleusDetail_QReturnsToList(t *testing.T) {
	m := newDetailModel(nil, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter) // open detail
	m3, _ := press(m2, "q")
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected StateList after q, got %v", m3.(ui.Model).State())
	}
}

func TestNucleusDetail_EscReturnsToList(t *testing.T) {
	m := newDetailModel(nil, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	m3, _ := pressSpecial(m2, tea.KeyEsc)
	if m3.(ui.Model).State() != ui.StateList {
		t.Fatalf("expected StateList after esc, got %v", m3.(ui.Model).State())
	}
}

// ── neuron cursor ─────────────────────────────────────────────────────────────

func TestNucleusDetail_NeuronCursorDown(t *testing.T) {
	// Build a nucleus with 2 neurons so cursor can move.
	nuclei := []registry.Nucleus{
		{
			ID: "nucl1", TaskDescription: "Test",
			Status: "idle",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "idle"},
				{ID: "nvim", Type: registry.NeuronNvim, TmuxTarget: "main:1.1", Status: "idle"},
			},
		},
	}
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus: func(task, repo, branch, profile, jiraKey string, createWorktree bool) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		CloseNvim:     func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))

	m4, _ := pressSpecial(m3, tea.KeyEnter) // open detail (neuronIdx=0)
	m5, _ := press(m4, "j")                 // move to neuron 1
	if m5.(ui.Model).DetailNeuronIdx() != 1 {
		t.Fatalf("expected DetailNeuronIdx=1, got %d", m5.(ui.Model).DetailNeuronIdx())
	}
}

func TestNucleusDetail_NeuronCursorClamped(t *testing.T) {
	m := newDetailModel(nil, nil) // nucl1 has 1 neuron
	m2, _ := pressSpecial(m, tea.KeyEnter)
	m3, _ := press(m2, "j") // already at bottom
	if m3.(ui.Model).DetailNeuronIdx() != 0 {
		t.Fatalf("expected clamped at 0, got %d", m3.(ui.Model).DetailNeuronIdx())
	}
}

func TestNucleusDetail_NeuronCursorUp(t *testing.T) {
	nuclei := []registry.Nucleus{
		{
			ID: "nucl1", TaskDescription: "Test",
			Status: "idle",
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, TmuxTarget: "main:1.0", Status: "idle"},
				{ID: "nvim", Type: registry.NeuronNvim, TmuxTarget: "main:1.1", Status: "idle"},
			},
		},
	}
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus: func(task, repo, branch, profile, jiraKey string, createWorktree bool) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		CloseNvim:     func(id string) error { return nil },
		RespawnNucleus: func(id string) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	m4, _ := pressSpecial(m3, tea.KeyEnter)
	m5, _ := press(m4, "j") // down to 1
	m6, _ := press(m5, "k") // back to 0
	if m6.(ui.Model).DetailNeuronIdx() != 0 {
		t.Fatalf("expected 0 after k, got %d", m6.(ui.Model).DetailNeuronIdx())
	}
}

// ── GotoNeuron ────────────────────────────────────────────────────────────────

func TestNucleusDetail_GotoCallsGotoNeuron(t *testing.T) {
	var calledNucleusID, calledNeuronID string
	m := newDetailModel(
		nil,
		func(nucleusID, neuronID string) error {
			calledNucleusID = nucleusID
			calledNeuronID = neuronID
			return nil
		},
	)
	m2, _ := pressSpecial(m, tea.KeyEnter) // open detail
	_, cmd := press(m2, "g")
	if cmd == nil {
		t.Fatal("expected cmd from g key in detail")
	}
	cmd()
	if calledNucleusID != "nucl1" {
		t.Fatalf("expected nucleusID=nucl1, got %q", calledNucleusID)
	}
	if calledNeuronID != "c1" {
		t.Fatalf("expected neuronID=c1, got %q", calledNeuronID)
	}
}

// ── NeuronAdd overlay ─────────────────────────────────────────────────────────

func TestNeuronAdd_OpenOnA(t *testing.T) {
	m := newDetailModel(func(nucleusID, neuronType, profile string) error { return nil }, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	m3, _ := press(m2, "a")
	if m3.(ui.Model).State() != ui.StateNeuronAdd {
		t.Fatalf("expected StateNeuronAdd after a, got %v", m3.(ui.Model).State())
	}
}

func TestNeuronAdd_EscReturnsToDetail(t *testing.T) {
	m := newDetailModel(func(nucleusID, neuronType, profile string) error { return nil }, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	m3, _ := press(m2, "a")
	m4, _ := pressSpecial(m3, tea.KeyEsc)
	if m4.(ui.Model).State() != ui.StateNucleusDetail {
		t.Fatalf("expected StateNucleusDetail after esc, got %v", m4.(ui.Model).State())
	}
}

func TestNeuronAdd_SelectTypeAndSubmit(t *testing.T) {
	var calledWith struct{ nucleusID, neuronType, profile string }
	m := newDetailModel(
		func(nucleusID, neuronType, profile string) error {
			calledWith.nucleusID = nucleusID
			calledWith.neuronType = neuronType
			calledWith.profile = profile
			return nil
		},
		nil,
	)
	m2, _ := pressSpecial(m, tea.KeyEnter) // open detail
	m3, _ := press(m2, "a")               // open neuron add
	m4, _ := press(m3, "j")               // cursor to "nvim"
	_, cmd := pressSpecial(m4, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("expected cmd after enter in neuron add")
	}
	cmd()
	if calledWith.nucleusID != "nucl1" {
		t.Fatalf("expected nucleusID=nucl1, got %q", calledWith.nucleusID)
	}
	if calledWith.neuronType != "nvim" {
		t.Fatalf("expected neuronType=nvim, got %q", calledWith.neuronType)
	}
}

// ── view smoke tests ──────────────────────────────────────────────────────────

func TestNucleusDetailDashboard_DoesNotPanic(t *testing.T) {
	m := newDetailModel(nil, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked in StateNucleusDetail: %v", r)
		}
	}()
	_ = m2.(ui.Model).View()
}

func TestNeuronAdd_ViewDoesNotPanic(t *testing.T) {
	m := newDetailModel(func(nucleusID, neuronType, profile string) error { return nil }, nil)
	m2, _ := pressSpecial(m, tea.KeyEnter)
	m3, _ := press(m2, "a")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked in StateNeuronAdd: %v", r)
		}
	}()
	_ = m3.(ui.Model).View()
}
