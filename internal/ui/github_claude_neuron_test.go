package ui_test

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/github"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func claudeNeuronNuclei() []registry.Nucleus {
	return []registry.Nucleus{
		{ID: "alpha", TaskDescription: "Alpha task", Status: "working", CreatedAt: time.Now()},
		{ID: "beta", TaskDescription: "Beta task", Status: "idle", CreatedAt: time.Now()},
	}
}

func claudeNeuronPRs() []github.PR {
	return []github.PR{
		{Number: 99, Repo: "org/repo", Title: "My PR", Branch: "feature/branch", State: "open"},
	}
}

// newClaudeNeuronGitHubModel returns a GitHub list model with AddClaudeNeuronFromPR wired.
func newClaudeNeuronGitHubModel(nuclei []registry.Nucleus, addClaude func(nucleusID, repo, branch, profile string, createWorktree bool) error, profileNames []string) ui.Model {
	prs := claudeNeuronPRs()
	svc := ui.Services{
		LoadNuclei:            func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:         func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:         func(id string) error { return nil },
		GotoNucleus:           func(id string) error { return nil },
		OpenNvim:              func(id string) error { return nil },
		LoadGitHubPRs:         func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		AddClaudeNeuronFromPR: addClaude,
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	if len(profileNames) > 0 {
		m3, _ = m3.Update(ui.ProfilesLoadedMsg(profileNames))
	}
	m4 := enterGitHubViewWithPRs(m3)
	return m4.(ui.Model)
}

// pressToWorktreePicker opens the "c" nucleus picker, selects first nucleus,
// and returns the model with the worktree picker active (no profiles configured).
func pressToWorktreePicker(m ui.Model) ui.Model {
	m2, _ := press(m, "c")                           // open nucleus picker
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select first nucleus → worktree picker
	return m3.(ui.Model)
}

// ── "c" key opens nucleus picker ─────────────────────────────────────────────

func TestGitHub_CKey_OpensPicker(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, nil)
	m2, _ := press(m, "c")
	got := m2.(ui.Model)
	if !got.GitHubNucleusPick() {
		t.Fatal("expected githubNucleusPick true after 'c'")
	}
	if got.GitHubNucleusPickMode() != "add_claude" {
		t.Fatalf("expected mode 'add_claude', got %q", got.GitHubNucleusPickMode())
	}
}

func TestGitHub_CKey_EscCancels(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, nil)
	m2, _ := press(m, "c")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m3.(ui.Model).GitHubNucleusPick() {
		t.Fatal("expected githubNucleusPick false after esc")
	}
}

func TestGitHub_CKey_NoService_Noop(t *testing.T) {
	prs := claudeNeuronPRs()
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return claudeNeuronNuclei(), nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadGitHubPRs: func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		// AddClaudeNeuronFromPR intentionally nil
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(claudeNeuronNuclei()))
	m4 := enterGitHubViewWithPRs(m3)
	m5, _ := press(m4, "c")
	if m5.(ui.Model).GitHubNucleusPick() {
		t.Fatal("expected no picker when AddClaudeNeuronFromPR is nil")
	}
}

// ── nucleus picker → worktree picker ─────────────────────────────────────────

func TestGitHub_CKey_NucleusEnter_OpensWorktreePicker(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, nil)
	got := pressToWorktreePicker(m)
	if !got.GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker after nucleus selection")
	}
	if got.GitHubClaudeWorktreeCursor() != 0 {
		t.Fatalf("expected default cursor 0 (no worktree), got %d", got.GitHubClaudeWorktreeCursor())
	}
}

func TestGitHub_CKey_WorktreePickerEscCancels(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, nil)
	m2 := pressToWorktreePicker(m)
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m3.(ui.Model).GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker closed after esc")
	}
}

func TestGitHub_CKey_WorktreePicker_DefaultIsNoWorktree(t *testing.T) {
	var capturedCreateWorktree bool
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, createWorktree bool) error {
		capturedCreateWorktree = createWorktree
		return nil
	}, nil)
	m2 := pressToWorktreePicker(m)
	// Press Enter at default position (0 = no worktree).
	m3, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m3.(ui.Model).GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker closed after Enter")
	}
	if cmd == nil {
		t.Fatal("expected cmd after Enter")
	}
	cmd()
	if capturedCreateWorktree {
		t.Error("expected createWorktree=false at default position 0")
	}
}

func TestGitHub_CKey_WorktreePicker_SelectWorktree(t *testing.T) {
	var capturedCreateWorktree bool
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, createWorktree bool) error {
		capturedCreateWorktree = createWorktree
		return nil
	}, nil)
	m2 := pressToWorktreePicker(m)
	// Move down to position 1 (new worktree).
	m3, _ := press(m2, "j")
	m4, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m4.(ui.Model).GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker closed after Enter")
	}
	if cmd == nil {
		t.Fatal("expected cmd after Enter")
	}
	cmd()
	if !capturedCreateWorktree {
		t.Error("expected createWorktree=true when option 1 selected")
	}
}

func TestGitHub_CKey_WorktreePicker_CallsServiceWithCorrectArgs(t *testing.T) {
	var capturedNucleus, capturedRepo, capturedBranch, capturedProfile string
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(nucleusID, repo, branch, profile string, _ bool) error {
		capturedNucleus = nucleusID
		capturedRepo = repo
		capturedBranch = branch
		capturedProfile = profile
		return nil
	}, nil)
	m2 := pressToWorktreePicker(m)
	_, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	cmd()
	if capturedNucleus != "alpha" {
		t.Errorf("expected nucleusID 'alpha', got %q", capturedNucleus)
	}
	if capturedRepo != "org/repo" {
		t.Errorf("expected repo 'org/repo', got %q", capturedRepo)
	}
	if capturedBranch != "feature/branch" {
		t.Errorf("expected branch 'feature/branch', got %q", capturedBranch)
	}
	if capturedProfile != "" {
		t.Errorf("expected empty profile, got %q", capturedProfile)
	}
}

// ── profile picker integration ────────────────────────────────────────────────

func TestGitHub_CKey_ProfilePick_Single_GoesToWorktree(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, []string{"work"})
	m2, _ := press(m, "c")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select nucleus → single profile auto-selected → worktree picker
	if !m3.(ui.Model).GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker after nucleus selection with single profile")
	}
	if m3.(ui.Model).GitHubProfilePick() {
		t.Fatal("expected no profile picker for single profile")
	}
}

func TestGitHub_CKey_ProfilePick_Multi_ShowsProfileFirst(t *testing.T) {
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, _ string, _ bool) error { return nil }, []string{"personal", "work"})
	m2, _ := press(m, "c")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select nucleus → profile picker
	if !m3.(ui.Model).GitHubProfilePick() {
		t.Fatal("expected profile picker for multiple profiles")
	}
	if m3.(ui.Model).GitHubProfilePickMode() != "add_claude" {
		t.Fatalf("expected mode 'add_claude', got %q", m3.(ui.Model).GitHubProfilePickMode())
	}
}

func TestGitHub_CKey_ProfilePick_Multi_ProfileEnterGoesToWorktree(t *testing.T) {
	var capturedProfile string
	m := newClaudeNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _, profile string, _ bool) error {
		capturedProfile = profile
		return nil
	}, []string{"personal", "work"})
	m2, _ := press(m, "c")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // nucleus picker → profile picker
	m4, _ := press(m3, "j")                            // move to "work"
	m5, _ := m4.Update(tea.KeyMsg{Type: tea.KeyEnter}) // profile picker → worktree picker
	if !m5.(ui.Model).GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker after profile selection")
	}
	// Confirm with Enter → calls service.
	m6, cmd := m5.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m6
	if cmd == nil {
		t.Fatal("expected cmd from worktree picker Enter")
	}
	cmd()
	if capturedProfile != "work" {
		t.Errorf("expected profile 'work', got %q", capturedProfile)
	}
}

// ── PR detail view ────────────────────────────────────────────────────────────

func TestGitHub_Detail_CKey_OpensPicker(t *testing.T) {
	nuclei := claudeNeuronNuclei()
	prs := claudeNeuronPRs()
	detail := &github.PRDetail{
		PR: github.PR{Number: 99, Repo: "org/repo", Title: "My PR", Branch: "feature/branch", State: "open"},
	}
	svc := ui.Services{
		LoadNuclei:            func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:         func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:         func(id string) error { return nil },
		GotoNucleus:           func(id string) error { return nil },
		OpenNvim:              func(id string) error { return nil },
		LoadGitHubPRs:         func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		LoadGitHubPR:          func(_ string, _ int) (*github.PRDetail, error) { return detail, nil },
		AddClaudeNeuronFromPR: func(_, _, _, _ string, _ bool) error { return nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	m4 := enterGitHubViewWithPRs(m3)
	m5, cmd := pressSpecial(m4, tea.KeyEnter)
	if cmd != nil {
		m5 = drainCmd(m5, cmd)
	}
	m6, _ := press(m5, "c")
	got := m6.(ui.Model)
	if !got.GitHubNucleusPick() {
		t.Fatal("expected githubNucleusPick true after 'c' in PR detail")
	}
	if got.GitHubNucleusPickMode() != "add_claude" {
		t.Fatalf("expected mode 'add_claude', got %q", got.GitHubNucleusPickMode())
	}
}

// ── "n" key → AddNvimNeuronFromPR flow ───────────────────────────────────────

func newNvimNeuronGitHubModel(nuclei []registry.Nucleus, addNvim func(nucleusID, repo, branch string, createWorktree bool) error) ui.Model {
	prs := claudeNeuronPRs()
	svc := ui.Services{
		LoadNuclei:          func() ([]registry.Nucleus, error) { return nuclei, nil },
		CreateNucleus:       func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus:       func(id string) error { return nil },
		GotoNucleus:         func(id string) error { return nil },
		OpenNvim:            func(id string) error { return nil },
		LoadGitHubPRs:       func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		AddNvimNeuronFromPR: addNvim,
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(nuclei))
	return enterGitHubViewWithPRs(m3).(ui.Model)
}

func TestGitHub_NKey_OpensPicker(t *testing.T) {
	m := newNvimNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _ string, _ bool) error { return nil })
	m2, _ := press(m, "n")
	got := m2.(ui.Model)
	if !got.GitHubNucleusPick() {
		t.Fatal("expected githubNucleusPick true after 'n'")
	}
	if got.GitHubNucleusPickMode() != "add_nvim" {
		t.Fatalf("expected mode 'add_nvim', got %q", got.GitHubNucleusPickMode())
	}
}

func TestGitHub_NKey_NoService_Noop(t *testing.T) {
	prs := claudeNeuronPRs()
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return claudeNeuronNuclei(), nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadGitHubPRs: func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		// AddNvimNeuronFromPR intentionally nil
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3, _ := m2.Update(nucleiLoaded(claudeNeuronNuclei()))
	m4 := enterGitHubViewWithPRs(m3)
	m5, _ := press(m4, "n")
	if m5.(ui.Model).GitHubNucleusPick() {
		t.Fatal("expected no picker when AddNvimNeuronFromPR is nil")
	}
}

func TestGitHub_NKey_EnterGoesToWorktreePicker(t *testing.T) {
	m := newNvimNeuronGitHubModel(claudeNeuronNuclei(), func(_, _, _ string, _ bool) error { return nil })
	m2, _ := press(m, "n")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := m3.(ui.Model)
	if !got.GitHubClaudeWorktreePick() {
		t.Fatal("expected worktree picker after nucleus selection")
	}
	if got.GitHubWorktreeMode() != "add_nvim" {
		t.Fatalf("expected worktree mode 'add_nvim', got %q", got.GitHubWorktreeMode())
	}
	if got.GitHubClaudeWorktreeCursor() != 0 {
		t.Fatalf("expected default cursor 0 (no worktree), got %d", got.GitHubClaudeWorktreeCursor())
	}
}

func TestGitHub_NKey_WorktreePickerCallsService(t *testing.T) {
	var capturedNucleus, capturedRepo, capturedBranch string
	var capturedWorktree bool
	m := newNvimNeuronGitHubModel(claudeNeuronNuclei(), func(nucleusID, repo, branch string, createWorktree bool) error {
		capturedNucleus = nucleusID
		capturedRepo = repo
		capturedBranch = branch
		capturedWorktree = createWorktree
		return nil
	})
	m2, _ := press(m, "n")
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select first nucleus → worktree picker
	m4, cmd := m3.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm at default (no worktree)
	_ = m4
	if cmd == nil {
		t.Fatal("expected cmd from worktree picker Enter")
	}
	cmd()
	if capturedNucleus != "alpha" {
		t.Errorf("expected nucleusID 'alpha', got %q", capturedNucleus)
	}
	if capturedRepo != "org/repo" {
		t.Errorf("expected repo 'org/repo', got %q", capturedRepo)
	}
	if capturedBranch != "feature/branch" {
		t.Errorf("expected branch 'feature/branch', got %q", capturedBranch)
	}
	if capturedWorktree {
		t.Error("expected createWorktree=false at default position 0")
	}
}
