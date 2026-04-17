package git_test

import (
	"errors"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/git"
)

// captureRunner records the last command + args passed to it.
type captureRunner struct {
	cmd    string
	args   []string
	output string
	err    error
}

func (r *captureRunner) Run(cmd string, args ...string) (string, error) {
	r.cmd = cmd
	r.args = args
	return r.output, r.err
}

func TestAddWorktree_Args(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.AddWorktree("/repo", "/repo/.worktrees/abc123", "feat/abc123", true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.cmd != "git" {
		t.Fatalf("expected git, got %s", r.cmd)
	}
	// When creating a new branch: git -C <repo> worktree add -b <branch> <path>
	want := []string{"-C", "/repo", "worktree", "add", "-b", "feat/abc123", "/repo/.worktrees/abc123"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestAddWorktree_WithBaseBranch_Args(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.AddWorktree("/repo", "/repo/.worktrees/abc123", "feat/new", true, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When creating with a base branch: git -C <repo> worktree add -b <branch> <path> <base>
	want := []string{"-C", "/repo", "worktree", "add", "-b", "feat/new", "/repo/.worktrees/abc123", "main"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestAddWorktree_ExistingBranch_Args(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.AddWorktree("/repo", "/repo/.worktrees/abc123", "feat/abc123", false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When using an existing branch: git -C <repo> worktree add <path> <branch>
	want := []string{"-C", "/repo", "worktree", "add", "/repo/.worktrees/abc123", "feat/abc123"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestAddWorktree_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("exit 128")}
	g := git.New(r)

	err := g.AddWorktree("/repo", "/repo/.worktrees/x", "feat/x", true, "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveWorktree_Args(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.RemoveWorktree("/repo", "/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"-C", "/repo", "worktree", "remove", "--force", "/repo/.worktrees/abc123"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestModifiedFiles_ParsesLines(t *testing.T) {
	r := &captureRunner{output: "file_a.go\nfile_b.go\n"}
	g := git.New(r)

	files, err := g.ModifiedFiles("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 || files[0] != "file_a.go" || files[1] != "file_b.go" {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestModifiedFiles_EmptyOutput(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	files, err := g.ModifiedFiles("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty slice, got %v", files)
	}
}

func TestModifiedFiles_Args(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	_, _ = g.ModifiedFiles("/repo/.worktrees/abc123")
	want := []string{"-C", "/repo/.worktrees/abc123", "ls-files", "-m"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestBranchExists_True(t *testing.T) {
	r := &captureRunner{output: "feat/abc123\n"}
	g := git.New(r)

	exists, err := g.BranchExists("/repo", "feat/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected branch to exist")
	}
	want := []string{"-C", "/repo", "branch", "--list", "feat/abc123"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestBranchExists_False(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	exists, err := g.BranchExists("/repo", "feat/nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected branch not to exist")
	}
}

func TestPushBranch_Args(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.PushBranch("/repo", "agent/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"-C", "/repo", "push", "-u", "origin", "agent/abc123"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestHasUncommittedChanges_Clean(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	dirty, err := g.HasUncommittedChanges("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dirty {
		t.Fatal("expected clean worktree")
	}
}

func TestHasUncommittedChanges_Dirty(t *testing.T) {
	r := &captureRunner{output: " M main.go\n"}
	g := git.New(r)

	dirty, err := g.HasUncommittedChanges("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dirty {
		t.Fatal("expected dirty worktree")
	}
	want := []string{"-C", "/repo/.worktrees/abc123", "status", "--porcelain"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestAheadCommits_Args(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	_, _ = g.AheadCommits("/repo/.worktrees/abc123")
	want := []string{"-C", "/repo/.worktrees/abc123", "log", "--oneline", "@{u}..HEAD"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestAheadCommits_ParsesLines(t *testing.T) {
	r := &captureRunner{output: "abc1234 fix auth\ndef5678 add tests\n"}
	g := git.New(r)

	commits, err := g.AheadCommits("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 || commits[0] != "abc1234 fix auth" || commits[1] != "def5678 add tests" {
		t.Fatalf("unexpected commits: %v", commits)
	}
}

func TestAheadCommits_Empty(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	commits, err := g.AheadCommits("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected empty, got %v", commits)
	}
}

func TestAheadCommits_ErrorReturnsSilently(t *testing.T) {
	// When no upstream is configured, git errors — we return empty slice, not error.
	r := &captureRunner{err: errors.New("no upstream")}
	g := git.New(r)

	commits, err := g.AheadCommits("/repo/.worktrees/abc123")
	if err != nil {
		t.Fatalf("expected nil error for missing upstream, got: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected empty slice, got %v", commits)
	}
}

func TestListBranches_Args(t *testing.T) {
	r := &captureRunner{output: "main\nfeat/foo\n"}
	g := git.New(r)

	branches, err := g.ListBranches("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"-C", "/repo", "branch", "--format=%(refname:short)"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
	if len(branches) != 2 || branches[0] != "main" || branches[1] != "feat/foo" {
		t.Fatalf("unexpected branches: %v", branches)
	}
}

func TestListBranches_Empty(t *testing.T) {
	r := &captureRunner{output: ""}
	g := git.New(r)

	branches, err := g.ListBranches("/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 0 {
		t.Fatalf("expected empty slice, got %v", branches)
	}
}

func TestCheckoutExisting_NoCreateBranch(t *testing.T) {
	r := &captureRunner{}
	g := git.New(r)

	err := g.CheckoutExisting("/repo", "/repo/.worktrees/review1", "feat/oauth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must use no -b flag: git -C <repo> worktree add <path> <branch>
	want := []string{"-C", "/repo", "worktree", "add", "/repo/.worktrees/review1", "feat/oauth"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
