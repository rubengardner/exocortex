package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// summaryNuclei returns a nucleus list designed to exercise all summary sections.
func summaryNuclei() []registry.Nucleus {
	return []registry.Nucleus{
		{
			ID:              "fixaut",
			TaskDescription: "Fix authentication bug",
			Status:          "working",
			JiraKeys:        []string{"PROJ-123"},
			PullRequests: []registry.PullRequest{
				{Number: 456, Repo: "org/repo1"},
			},
			CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, Status: "working", Branch: "agent/fixaut", RepoPath: "/home/user/projects/repo1"},
				{ID: "c2", Type: registry.NeuronClaude, Status: "idle", Branch: "agent/fixaut2", RepoPath: "/home/user/projects/repo2"},
				{ID: "nv", Type: registry.NeuronNvim, Status: "idle", Branch: "agent/fixaut", RepoPath: "/home/user/projects/repo1"},
			},
		},
	}
}

func newSummaryModel(nuclei []registry.Nucleus) ui.Model {
	svc := ui.Services{
		LoadNuclei:     func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:  func(task, jiraKey, profile string) error { return nil },
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

func TestNucleusSummary_DoesNotPanic(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked in StateList: %v", r)
		}
	}()
	_ = m.View()
}

func TestNucleusSummary_EmptyNuclei_DoesNotPanic(t *testing.T) {
	m := newSummaryModel(nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked with empty nuclei: %v", r)
		}
	}()
	_ = m.View()
}

func TestNucleusSummary_ShowsAgentCount(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "2 agent(s)") {
		t.Errorf("expected '2 agent(s)' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_ShowsNvimCount(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "nvim  1") {
		t.Errorf("expected 'nvim  1' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_ShowsBranch(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "agent/fixaut") {
		t.Errorf("expected branch 'agent/fixaut' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_ShowsRepoBasename(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "repo1") {
		t.Errorf("expected repo basename 'repo1' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_ShowsJiraKey(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "PROJ-123") {
		t.Errorf("expected 'PROJ-123' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_ShowsPR(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if !strings.Contains(view, "#456") {
		t.Errorf("expected '#456' in view, got:\n%s", view)
	}
}

func TestNucleusSummary_NoLinksSection_WhenEmpty(t *testing.T) {
	nuclei := []registry.Nucleus{
		{
			ID: "bare", TaskDescription: "Bare nucleus", Status: "idle",
			CreatedAt: time.Now(),
			Neurons: []registry.Neuron{
				{ID: "c1", Type: registry.NeuronClaude, Status: "idle", Branch: "main", RepoPath: "/repo"},
			},
		},
	}
	m := newSummaryModel(nuclei)
	view := m.View()
	if strings.Contains(view, "Links") {
		t.Errorf("expected no 'Links' section when no jira/pr, got:\n%s", view)
	}
}

func TestNucleusSummary_NoPreviewSection(t *testing.T) {
	m := newSummaryModel(summaryNuclei())
	view := m.View()
	if strings.Contains(view, "Preview") {
		t.Errorf("expected no 'Preview' in list view, got:\n%s", view)
	}
}

func TestNucleusSummary_ZeroAgentsShowsDash(t *testing.T) {
	nuclei := []registry.Nucleus{
		{
			ID: "empty", TaskDescription: "No agents", Status: "idle",
			CreatedAt: time.Now(),
		},
	}
	m := newSummaryModel(nuclei)
	view := m.View()
	if !strings.Contains(view, "0 agent(s)") {
		t.Errorf("expected '0 agent(s)' for empty nucleus, got:\n%s", view)
	}
}
