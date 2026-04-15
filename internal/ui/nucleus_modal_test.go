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
	if m.mode != ModeDevelop {
		t.Fatalf("expected ModeDevelop, got %v", m.mode)
	}
	if !m.createWorktree {
		t.Fatal("expected createWorktree=true by default")
	}
	if m.focused != ModalFieldMode {
		t.Fatalf("expected focus on ModalFieldMode, got %v", m.focused)
	}
}

// ── Open ──────────────────────────────────────────────────────────────────────

func TestNucleusModal_Open_DevelopMode(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	if m.mode != ModeDevelop {
		t.Fatalf("expected ModeDevelop, got %v", m.mode)
	}
}

func TestNucleusModal_Open_ReviewMode(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{Mode: ModeReview})
	if m.mode != ModeReview {
		t.Fatalf("expected ModeReview, got %v", m.mode)
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

func TestNucleusModal_Open_JiraContext_PreFillsBranch(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		JiraKey:     "PROJ-42",
		JiraSummary: "Fix auth bug",
	})
	if m.branchInput.Value() != "task/PROJ-42/" {
		t.Fatalf("expected branch='task/PROJ-42/', got %q", m.branchInput.Value())
	}
}

func TestNucleusModal_Open_PRContext_SetsReviewMode(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 7,
		PRRepo:   "org/repo",
		PRBranch: "feat/oauth",
	})
	if m.mode != ModeReview {
		t.Fatalf("expected ModeReview, got %v", m.mode)
	}
	if m.prNumber != 7 {
		t.Fatalf("expected prNumber=7, got %d", m.prNumber)
	}
	if m.branchFilter != "feat/oauth" {
		t.Fatalf("expected branchFilter='feat/oauth', got %q", m.branchFilter)
	}
}

func TestNucleusModal_Open_PRTitle_SetsTaskToReviewPrefix(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 12,
		PRRepo:   "org/repo",
		PRTitle:  "Fix the login bug",
	})
	want := "Review: Fix the login bug"
	if m.taskInput.Value() != want {
		t.Fatalf("expected task=%q, got %q", want, m.taskInput.Value())
	}
}

func TestNucleusModal_Open_PRTitle_Empty_FallsBackToNumber(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 42,
		PRRepo:   "org/repo",
		PRTitle:  "",
	})
	want := "Review PR #42"
	if m.taskInput.Value() != want {
		t.Fatalf("expected task=%q, got %q", want, m.taskInput.Value())
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

// ── SetRepos / SetProfiles / SetBranches ──────────────────────────────────────

func TestNucleusModal_SetRepos(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a/project", "/b/other"})
	if !m.reposReady {
		t.Fatal("expected reposReady after SetRepos")
	}
	if len(m.repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(m.repos))
	}
}

func TestNucleusModal_SetRepos_ClampsCursor(t *testing.T) {
	m := NewNucleusModal(80)
	m.repoCursor = 99
	m = m.SetRepos([]string{"/only"})
	if m.repoCursor != 0 {
		t.Fatalf("expected cursor clamped to 0, got %d", m.repoCursor)
	}
}

func TestNucleusModal_SetRepos_AutoSelectsMatchingPRRepo(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:   ModeReview,
		PRRepo: "org/beta",
	})
	m = m.SetRepos([]string{"/home/user/alpha", "/home/user/beta", "/home/user/gamma"})
	if m.repoCursor != 1 {
		t.Fatalf("expected repoCursor=1 (beta), got %d", m.repoCursor)
	}
}

func TestNucleusModal_SetRepos_NoMatchLeavesFirstSelected(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:   ModeReview,
		PRRepo: "org/unknown",
	})
	m = m.SetRepos([]string{"/home/user/alpha", "/home/user/beta"})
	if m.repoCursor != 0 {
		t.Fatalf("expected repoCursor=0 when no match, got %d", m.repoCursor)
	}
}

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

func TestNucleusModal_SetBranches(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetBranches([]string{"main", "feat/oauth", "fix/login"})
	if !m.branchesReady {
		t.Fatal("expected branchesReady after SetBranches")
	}
	if len(m.branchList) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(m.branchList))
	}
}

func TestNucleusModal_SetBranches_PreservesFilter(t *testing.T) {
	m := NewNucleusModal(80)
	m.branchFilter = "feat"
	m = m.SetBranches([]string{"feat/oauth", "main"})
	// filter text is preserved
	if m.branchFilter != "feat" {
		t.Fatalf("expected branchFilter preserved as 'feat', got %q", m.branchFilter)
	}
	// cursor is reset
	if m.branchCursor != 0 {
		t.Fatalf("expected branchCursor reset to 0, got %d", m.branchCursor)
	}
}

// ── SelectedRepo ─────────────────────────────────────────────────────────────

func TestNucleusModal_SelectedRepo_NoRepos_ReturnsDot(t *testing.T) {
	m := NewNucleusModal(80)
	if m.SelectedRepo() != "." {
		t.Fatalf("expected '.' when no repos, got %q", m.SelectedRepo())
	}
}

func TestNucleusModal_SelectedRepo_SingleRepo(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/home/user/project"})
	if m.SelectedRepo() != "/home/user/project" {
		t.Fatalf("expected '/home/user/project', got %q", m.SelectedRepo())
	}
}

func TestNucleusModal_SelectedRepo_FollowsCursor(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a", "/b", "/c"})
	m.repoCursor = 2
	if m.SelectedRepo() != "/c" {
		t.Fatalf("expected '/c', got %q", m.SelectedRepo())
	}
}

// ── visibleFields ─────────────────────────────────────────────────────────────

func TestNucleusModal_VisibleFields_NoReposNoProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	fields := m.visibleFields()
	// Should be: Mode, Task, Branch, Worktree
	want := []ModalField{ModalFieldMode, ModalFieldTask, ModalFieldBranch, ModalFieldWorktree}
	if len(fields) != len(want) {
		t.Fatalf("expected %d fields, got %d: %v", len(want), len(fields), fields)
	}
	for i, f := range want {
		if fields[i] != f {
			t.Fatalf("field[%d]: expected %v, got %v", i, f, fields[i])
		}
	}
}

func TestNucleusModal_VisibleFields_MultipleRepos(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a", "/b"})
	fields := m.visibleFields()
	if !containsField(fields, ModalFieldRepo) {
		t.Fatal("expected ModalFieldRepo when multiple repos")
	}
}

func TestNucleusModal_VisibleFields_SingleRepo_NoRepoField(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/only"})
	fields := m.visibleFields()
	if containsField(fields, ModalFieldRepo) {
		t.Fatal("ModalFieldRepo should be hidden when only one repo")
	}
}

func TestNucleusModal_VisibleFields_WithProfiles(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetProfiles([]string{"work"}, map[string]string{"work": "~/.claude-work"})
	fields := m.visibleFields()
	if !containsField(fields, ModalFieldProfile) {
		t.Fatal("expected ModalFieldProfile when profiles are configured")
	}
}

// ── Tab navigation ────────────────────────────────────────────────────────────

func TestNucleusModal_Tab_AdvancesField(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldMode
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	// Should advance to Task (no repos, no profiles)
	if m.focused != ModalFieldTask {
		t.Fatalf("expected ModalFieldTask after Tab, got %v", m.focused)
	}
}

func TestNucleusModal_Tab_WrapsFromWorktree(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldWorktree
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != ModalFieldMode {
		t.Fatalf("expected ModalFieldMode after Tab wrap, got %v", m.focused)
	}
}

func TestNucleusModal_Tab_SkipsRepoWhenSingleRepo(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/only"})
	m.focused = ModalFieldMode
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused == ModalFieldRepo {
		t.Fatal("should skip ModalFieldRepo when only one repo")
	}
}

func TestNucleusModal_Tab_IncludesRepoWhenMultiple(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a", "/b"})
	m.focused = ModalFieldMode
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focused != ModalFieldRepo {
		t.Fatalf("expected ModalFieldRepo after Tab from Mode, got %v", m.focused)
	}
}

// ── Mode toggle ───────────────────────────────────────────────────────────────

func TestNucleusModal_Mode_SpaceTogglesToReview(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldMode
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.mode != ModeReview {
		t.Fatalf("expected ModeReview after Space, got %v", m.mode)
	}
}

func TestNucleusModal_Mode_SpaceTogglesToDevelop(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldMode
	m.mode = ModeReview
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.mode != ModeDevelop {
		t.Fatalf("expected ModeDevelop after second Space, got %v", m.mode)
	}
}

func TestNucleusModal_Mode_ToggleToReview_RequestsLoadBranches(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldMode
	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !req.LoadBranches {
		t.Fatal("expected LoadBranches request when switching to Review mode")
	}
}

func TestNucleusModal_Mode_ToggleToDevelop_NoLoadBranchesRequest(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldMode
	m.mode = ModeReview
	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if req.LoadBranches {
		t.Fatal("should not request LoadBranches when switching back to Develop mode")
	}
}

// ── Repo navigation ────────────────────────────────────────────────────────────

func TestNucleusModal_Repo_JMovesDown(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a", "/b", "/c"})
	m.focused = ModalFieldRepo
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.repoCursor != 1 {
		t.Fatalf("expected repoCursor=1, got %d", m.repoCursor)
	}
}

func TestNucleusModal_Repo_KMovesUp(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a", "/b"})
	m.focused = ModalFieldRepo
	m.repoCursor = 1
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.repoCursor != 0 {
		t.Fatalf("expected repoCursor=0, got %d", m.repoCursor)
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
	// Focus the task input first
	cmd := m.taskInput.Focus()
	_ = cmd
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	if !strings.Contains(m.taskInput.Value(), "hello") {
		t.Fatalf("expected 'hello' in task input, got %q", m.taskInput.Value())
	}
}

// ── Branch input (develop mode) ───────────────────────────────────────────────

func TestNucleusModal_Branch_DevelopMode_TypingUpdatesInput(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeDevelop
	m.focused = ModalFieldBranch
	cmd := m.branchInput.Focus()
	_ = cmd
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("my-branch")})
	if !strings.Contains(m.branchInput.Value(), "my-branch") {
		t.Fatalf("expected 'my-branch' in branch input, got %q", m.branchInput.Value())
	}
}

// ── Branch search (review mode) ───────────────────────────────────────────────

func TestNucleusModal_Branch_ReviewMode_TypingFilters(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeReview
	m.focused = ModalFieldBranch
	m = m.SetBranches([]string{"feat/oauth", "feat/login", "main"})
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("feat")})
	filtered := m.filteredBranches()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 branches matching 'feat', got %d", len(filtered))
	}
}

func TestNucleusModal_Branch_ReviewMode_JMovesDown(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeReview
	m.focused = ModalFieldBranch
	m = m.SetBranches([]string{"a", "b", "c"})
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.branchCursor != 1 {
		t.Fatalf("expected branchCursor=1, got %d", m.branchCursor)
	}
}

func TestNucleusModal_Branch_ReviewMode_KMovesUp(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeReview
	m.focused = ModalFieldBranch
	m = m.SetBranches([]string{"a", "b"})
	m.branchCursor = 1
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.branchCursor != 0 {
		t.Fatalf("expected branchCursor=0, got %d", m.branchCursor)
	}
}

func TestNucleusModal_Branch_ReviewMode_BackspaceErasesFilter(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeReview
	m.focused = ModalFieldBranch
	m.branchFilter = "feat"
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.branchFilter != "fea" {
		t.Fatalf("expected filter='fea' after backspace, got %q", m.branchFilter)
	}
}

func TestNucleusModal_Branch_ReviewMode_FilterResetsCursor(t *testing.T) {
	m := NewNucleusModal(80)
	m.mode = ModeReview
	m.focused = ModalFieldBranch
	m = m.SetBranches([]string{"a", "b", "c"})
	m.branchCursor = 2
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.branchCursor != 0 {
		t.Fatalf("expected branchCursor reset to 0 on filter change, got %d", m.branchCursor)
	}
}

// ── Worktree toggle ───────────────────────────────────────────────────────────

func TestNucleusModal_Worktree_SpaceToggles(t *testing.T) {
	m := NewNucleusModal(80)
	m.focused = ModalFieldWorktree
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.createWorktree {
		t.Fatal("expected createWorktree=false after toggle")
	}
	m, _, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.createWorktree {
		t.Fatal("expected createWorktree=true after second toggle")
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

func TestNucleusModal_Submit_ReviewMode_NoBranch_SetsErr(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{Mode: ModeReview})
	m.taskInput.SetValue("review task")
	// no branches loaded and no filter → filtered list is empty
	m, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit != nil {
		t.Fatal("expected no Submit when no branch selected in review mode")
	}
	if m.err == "" {
		t.Fatal("expected error message when no branch selected")
	}
}

func TestNucleusModal_Open_ReviewMode_WorktreeDefaultsOff(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 5,
		PRRepo:   "org/repo",
		PRBranch: "feat/thing",
	})
	if m.createWorktree {
		t.Fatal("expected createWorktree=false by default for review mode")
	}
}

func TestNucleusModal_Submit_ReviewMode_UsesBranchFilterWhenBranchesNotLoaded(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 3,
		PRRepo:   "org/repo",
		PRBranch: "feat/oauth",
	})
	m.taskInput.SetValue("Review: Add OAuth")
	// branches not yet loaded (branchesReady=false, branchList=nil)

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit even when branches haven't loaded")
	}
	if req.Submit.Branch != "feat/oauth" {
		t.Fatalf("expected branch='feat/oauth', got %q", req.Submit.Branch)
	}
}

func TestNucleusModal_Submit_ReviewMode_PrefersFilteredListOverRawFilter(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 3,
		PRRepo:   "org/repo",
		PRBranch: "feat/oauth",
	})
	m.taskInput.SetValue("Review: Add OAuth")
	m = m.SetBranches([]string{"feat/oauth", "feat/oauth-v2", "main"})
	// cursor at 0 → "feat/oauth" (exact match first in filtered list)

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit with branches loaded")
	}
	if req.Submit.Branch != "feat/oauth" {
		t.Fatalf("expected branch='feat/oauth', got %q", req.Submit.Branch)
	}
}

// ── Submit ─────────────────────────────────────────────────────────────────────

func TestNucleusModal_Submit_DevelopMode(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("fix the bug")
	m.branchInput.SetValue("fix/bug")

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit on valid develop form")
	}
	sub := req.Submit
	if sub.Mode != ModeDevelop {
		t.Fatalf("expected ModeDevelop, got %v", sub.Mode)
	}
	if sub.Task != "fix the bug" {
		t.Fatalf("expected task='fix the bug', got %q", sub.Task)
	}
	if sub.Branch != "fix/bug" {
		t.Fatalf("expected branch='fix/bug', got %q", sub.Branch)
	}
	if !sub.CreateWorktree {
		t.Fatal("expected CreateWorktree=true by default")
	}
}

func TestNucleusModal_Submit_DevelopMode_EmptyBranchAllowed(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("some task")
	// branch left empty — auto-generated by executeNew

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit when branch is empty (auto-generate)")
	}
	if req.Submit.Branch != "" {
		t.Fatalf("expected empty branch, got %q", req.Submit.Branch)
	}
}

func TestNucleusModal_Submit_ReviewMode(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{
		Mode:     ModeReview,
		PRNumber: 7,
		PRRepo:   "org/repo",
	})
	m.taskInput.SetValue("review pr 7")
	m = m.SetBranches([]string{"feat/oauth", "main"})
	// branchCursor=0 → selects "feat/oauth"

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit on valid review form")
	}
	sub := req.Submit
	if sub.Mode != ModeReview {
		t.Fatalf("expected ModeReview, got %v", sub.Mode)
	}
	if sub.Branch != "feat/oauth" {
		t.Fatalf("expected branch='feat/oauth', got %q", sub.Branch)
	}
	if sub.PRNumber != 7 {
		t.Fatalf("expected PRNumber=7, got %d", sub.PRNumber)
	}
	if sub.PRRepo != "org/repo" {
		t.Fatalf("expected PRRepo='org/repo', got %q", sub.PRRepo)
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

func TestNucleusModal_Submit_CreateWorktree_False(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("some task")
	m.createWorktree = false

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit")
	}
	if req.Submit.CreateWorktree {
		t.Fatal("expected CreateWorktree=false")
	}
}

func TestNucleusModal_Submit_PassesProfileAndRepo(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a/project", "/b/other"})
	m = m.SetProfiles([]string{"work"}, map[string]string{"work": "~/.claude-work"})
	m.repoCursor = 1
	m.profileCursor = 0
	m, _ = m.Open(NucleusModalContext{})
	m.taskInput.SetValue("some task")
	m.repoCursor = 1 // /b/other

	_, req, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if req.Submit == nil {
		t.Fatal("expected Submit")
	}
	if req.Submit.Repo != "/b/other" {
		t.Fatalf("expected Repo='/b/other', got %q", req.Submit.Repo)
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

// ── filteredBranches ──────────────────────────────────────────────────────────

func TestNucleusModal_FilteredBranches_EmptyFilter_ReturnsAll(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetBranches([]string{"main", "feat/x", "feat/y"})
	if len(m.filteredBranches()) != 3 {
		t.Fatalf("expected 3 branches with no filter, got %d", len(m.filteredBranches()))
	}
}

func TestNucleusModal_FilteredBranches_CaseInsensitive(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetBranches([]string{"MAIN", "Feat/X"})
	m.branchFilter = "feat"
	filtered := m.filteredBranches()
	if len(filtered) != 1 || filtered[0] != "Feat/X" {
		t.Fatalf("expected case-insensitive match, got %v", filtered)
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

func TestNucleusModal_View_ShowsPRContext(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{Mode: ModeReview, PRNumber: 9, PRRepo: "x/y"})
	view := m.View()
	if !strings.Contains(view, "#9") {
		t.Fatalf("expected '#9' in view, got:\n%s", view)
	}
}

func TestNucleusModal_View_ShowsRepoWhenMultiple(t *testing.T) {
	m := NewNucleusModal(80)
	m = m.SetRepos([]string{"/a/project", "/b/other"})
	m, _ = m.Open(NucleusModalContext{})
	m.repos = []string{"/a/project", "/b/other"} // re-set after Open resets
	view := m.View()
	if !strings.Contains(view, "project") {
		t.Fatalf("expected repo name in view when multiple repos, got:\n%s", view)
	}
}

func TestNucleusModal_View_ShowsWorktreeCheck(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{})
	if !strings.Contains(m.View(), "✓") && !strings.Contains(m.View(), "worktree") {
		t.Fatal("expected worktree toggle in view")
	}
}

func TestNucleusModal_View_ReviewMode_ShowsFilterPrompt(t *testing.T) {
	m := NewNucleusModal(80)
	m, _ = m.Open(NucleusModalContext{Mode: ModeReview})
	m.focused = ModalFieldBranch
	view := m.View()
	if !strings.Contains(view, "Filter") {
		t.Fatalf("expected 'Filter' in review branch view, got:\n%s", view)
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
