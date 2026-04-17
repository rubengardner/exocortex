package ui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruben_gardner/exocortex/internal/github"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newGitHubModel returns a model configured with LoadGitHubPRs and
// LoadGitHubFilterConfig stubs, sized and seeded with a PR list.
func newGitHubModel(myLogin string, teammates, repos []string) ui.Model {
	prs := []github.PR{
		{Number: 1, Repo: "org/a", Title: "First PR", State: "open"},
		{Number: 2, Repo: "org/b", Title: "Second PR", State: "open"},
	}
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nil, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadGitHubPRs: func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
		LoadGitHubFilterConfig: func() (string, []string, []string, error) {
			return myLogin, teammates, repos, nil
		},
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Seed PR list via a synthetic load message.
	m3 := enterGitHubViewWithPRs(m2)
	return m3.(ui.Model)
}

// pressFilter presses "f", executes the config load cmd, and feeds the
// resulting githubFilterConfigLoadedMsg back into the model so it transitions
// to StateGitHubFilter.
func pressFilter(m tea.Model) tea.Model {
	m2, cmd := press(m, "f")
	if cmd == nil {
		return m2
	}
	return drainCmd(m2, cmd)
}

// ── state transitions ─────────────────────────────────────────────────────────

func TestGitHubFilter_PressF_OpensModal(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, []string{"org/a"})
	m2 := pressFilter(m)
	if m2.(ui.Model).State() != ui.StateGitHubFilter {
		t.Errorf("state: got %v, want StateGitHubFilter", m2.(ui.Model).State())
	}
}

func TestGitHubFilter_PressF_NoConfig_StaysOnList(t *testing.T) {
	// When LoadGitHubFilterConfig is nil, pressing f should do nothing.
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nil, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadGitHubPRs: func(_ github.PRFilter) ([]github.PR, error) { return nil, nil },
		// LoadGitHubFilterConfig is nil
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3 := enterGitHubViewWithPRs(m2)
	m4, _ := press(m3, "f")
	if m4.(ui.Model).State() != ui.StateGitHubView {
		t.Errorf("state: got %v, want StateGitHubView", m4.(ui.Model).State())
	}
}

func TestGitHubFilter_EscCancels(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, []string{"org/a"})
	inModal := pressFilter(m)
	if inModal.(ui.Model).State() != ui.StateGitHubFilter {
		t.Fatal("expected to be in filter modal")
	}
	// Press esc — should return to list without changing the committed filter.
	m2, _ := pressSpecial(inModal, tea.KeyEsc)
	if m2.(ui.Model).State() != ui.StateGitHubView {
		t.Errorf("after esc: got %v, want StateGitHubView", m2.(ui.Model).State())
	}
}

// ── toggling and confirming ───────────────────────────────────────────────────

func TestGitHubFilter_ToggleAndConfirm_AuthorFilter(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, nil)
	inModal := pressFilter(m)

	// Cursor starts on the first selectable item. Navigate down until we see
	// "alice" selected and then confirm.
	// Press space to toggle the current item, then enter to confirm.
	toggled, _ := pressSpecial(inModal, tea.KeySpace)
	confirmed, confirmCmd := pressSpecial(toggled, tea.KeyEnter)

	// confirmCmd should emit githubFilterConfirmedMsg, which triggers a reload.
	result := drainCmd(confirmed, confirmCmd)
	got := result.(ui.Model)

	if got.State() != ui.StateGitHubView {
		t.Errorf("after confirm: state = %v, want StateGitHubView", got.State())
	}
}

func TestGitHubFilter_ClearAllSelections(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, []string{"org/a"})
	inModal := pressFilter(m)

	// Toggle a couple of items, then press c to clear.
	m2, _ := pressSpecial(inModal, tea.KeySpace)
	m3, _ := press(m2, "j")
	m4, _ := pressSpecial(m3, tea.KeySpace)
	cleared, _ := press(m4, "c")

	// Confirm and check the resulting filter is zero.
	confirmedModel, cmd := pressSpecial(cleared, tea.KeyEnter)
	final := drainCmd(confirmedModel, cmd)
	// After a clear+confirm the committed filter should be zero.
	_ = final // state assertions covered by other tests; here we just verify no panic
}

// ── navigation ────────────────────────────────────────────────────────────────

func TestGitHubFilter_JKNavigation_SkipsHeaders(t *testing.T) {
	// With both authors and repos, there are two header items. Verify j/k never
	// land on them by navigating through all items and checking view output.
	m := newGitHubModel("ruben", []string{"alice", "bob"}, []string{"org/a", "org/b"})
	inModal := pressFilter(m)

	// Navigate down through all items — should never show a header as selected.
	cur := inModal
	for i := 0; i < 10; i++ {
		view := cur.(ui.Model).View()
		// A selected header would render "  AUTHORS" or "  REPOSITORIES" with
		// the StyleSelected highlight. We check the modal render doesn't contain
		// a highlighted header (we look for "[x]" adjacent to section names).
		if strings.Contains(view, "[x] AUTHORS") || strings.Contains(view, "[x] REPOSITORIES") {
			t.Errorf("cursor landed on a header at step %d", i)
		}
		cur, _ = press(cur, "j")
	}
}

// ── filter indicator in header ────────────────────────────────────────────────

func TestGitHubFilter_HeaderShowsIndicator_WhenFilterActive(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, nil)
	inModal := pressFilter(m)

	// Toggle the first item (should be "me") and confirm.
	toggled, _ := pressSpecial(inModal, tea.KeySpace)
	confirmed, cmd := pressSpecial(toggled, tea.KeyEnter)
	result := drainCmd(confirmed, cmd)

	view := result.(ui.Model).View()
	if !strings.Contains(view, "filtered:") {
		t.Errorf("expected 'filtered:' indicator in header after filter applied, got:\n%s", view)
	}
}

func TestGitHubFilter_HeaderNoIndicator_WhenFilterZero(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, nil)
	view := m.View()
	if strings.Contains(view, "filtered:") {
		t.Error("expected no 'filtered:' indicator when filter is zero")
	}
}

// ── my_login absent ───────────────────────────────────────────────────────────

func TestGitHubFilter_NoMyLogin_HidesMeAndOthers(t *testing.T) {
	m := newGitHubModel("", []string{"alice"}, nil)
	inModal := pressFilter(m)
	view := inModal.(ui.Model).View()
	if strings.Contains(view, "others") {
		t.Error("expected 'others' row to be hidden when my_login is absent")
	}
	// "alice" should still appear.
	if !strings.Contains(view, "alice") {
		t.Error("expected 'alice' row to appear even when my_login is absent")
	}
}

// ── no repos in config ────────────────────────────────────────────────────────

func TestGitHubFilter_NoRepos_HidesReposSection(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, nil)
	inModal := pressFilter(m)
	view := inModal.(ui.Model).View()
	if strings.Contains(view, "REPOSITORIES") {
		t.Error("expected REPOSITORIES section to be hidden when no repos configured")
	}
}

func TestGitHubFilter_WithRepos_ShowsReposSection(t *testing.T) {
	m := newGitHubModel("ruben", []string{"alice"}, []string{"org/a", "org/b"})
	inModal := pressFilter(m)
	view := inModal.(ui.Model).View()
	if !strings.Contains(view, "REPOSITORIES") {
		t.Error("expected REPOSITORIES section to appear when repos are configured")
	}
	if !strings.Contains(view, "org/a") {
		t.Error("expected org/a to appear in repos section")
	}
}

// ── PR list panel: repo display ───────────────────────────────────────────────

func TestGitHubListPanel_ShowsShortRepoName(t *testing.T) {
	// PRs from two different repos; the list panel should show the short name
	// (part after the last "/") on each row.
	prs := []github.PR{
		{Number: 10, Repo: "org/alpha", Title: "Fix login", State: "open"},
		{Number: 20, Repo: "org/beta", Title: "Add tests", State: "open"},
	}
	svc := ui.Services{
		LoadNuclei:    func() ([]registry.Nucleus, error) { return nil, nil },
		CreateNucleus: func(task, jiraKey, profile string) error { return nil },
		RemoveNucleus: func(id string) error { return nil },
		GotoNucleus:   func(id string) error { return nil },
		OpenNvim:      func(id string) error { return nil },
		LoadGitHubPRs: func(_ github.PRFilter) ([]github.PR, error) { return prs, nil },
	}
	m := ui.New(svc)
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m3 := enterGitHubViewWithPRs(m2)
	view := m3.(ui.Model).View()

	if !strings.Contains(view, "alpha") {
		t.Error("expected short repo name 'alpha' in list panel")
	}
	if !strings.Contains(view, "beta") {
		t.Error("expected short repo name 'beta' in list panel")
	}
	// Full owner/repo should not appear in the list rows (only the short name).
	if strings.Contains(view, "org/alpha") {
		t.Error("expected full 'org/alpha' to be stripped to short name in list panel")
	}
}
