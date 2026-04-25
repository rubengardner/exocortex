package github_test

import (
	"strings"
	"testing"
	"time"

	igithub "github.com/ruben_gardner/exocortex/internal/github"
)

// ── BuildQuery ────────────────────────────────────────────────────────────────

func TestBuildQuery_ZeroFilter(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{})
	if !strings.Contains(q, "involves:@me") {
		t.Errorf("zero filter: want 'involves:@me', got %q", q)
	}
	if strings.Contains(q, "author:") {
		t.Errorf("zero filter: unexpected 'author:' in %q", q)
	}
}

func TestBuildQuery_ZeroFilterWithOrg(t *testing.T) {
	q := igithub.BuildQuery("ruben", "BadgerMaps", igithub.PRFilter{})
	if !strings.Contains(q, "involves:@me") {
		t.Errorf("want 'involves:@me', got %q", q)
	}
	if !strings.Contains(q, "org:BadgerMaps") {
		t.Errorf("want 'org:BadgerMaps', got %q", q)
	}
}

func TestBuildQuery_SingleAuthor(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{Authors: []string{"alice"}})
	if !strings.Contains(q, "author:alice") {
		t.Errorf("want 'author:alice', got %q", q)
	}
	if strings.Contains(q, "involves:") {
		t.Errorf("unexpected 'involves:' when author is set, got %q", q)
	}
}

func TestBuildQuery_MultipleAuthors(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{Authors: []string{"alice", "bob"}})
	if !strings.Contains(q, "author:alice") {
		t.Errorf("want 'author:alice', got %q", q)
	}
	if !strings.Contains(q, "author:bob") {
		t.Errorf("want 'author:bob', got %q", q)
	}
}

func TestBuildQuery_OthersOnly(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{Authors: []string{"!me"}})
	if !strings.Contains(q, "-author:ruben") {
		t.Errorf("want '-author:ruben', got %q", q)
	}
	if strings.Contains(q, "involves:") {
		t.Errorf("unexpected 'involves:' when !me is set, got %q", q)
	}
}

func TestBuildQuery_OthersSentinelWithNoLogin(t *testing.T) {
	// When my_login is absent the sentinel is silently ignored.
	q := igithub.BuildQuery("", "", igithub.PRFilter{Authors: []string{"!me"}})
	if strings.Contains(q, "-author:") {
		t.Errorf("want no -author: when login is empty, got %q", q)
	}
}

func TestBuildQuery_OthersPlusTeammate(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{Authors: []string{"!me", "alice"}})
	if !strings.Contains(q, "-author:ruben") {
		t.Errorf("want '-author:ruben', got %q", q)
	}
	if !strings.Contains(q, "author:alice") {
		t.Errorf("want 'author:alice', got %q", q)
	}
}

func TestBuildQuery_RepoFilterDoesNotChangeQuery(t *testing.T) {
	// Repos are filtered client-side; the query stays unchanged.
	withRepos := igithub.BuildQuery("ruben", "", igithub.PRFilter{Repos: []string{"org/a", "org/b"}})
	withoutRepos := igithub.BuildQuery("ruben", "", igithub.PRFilter{})
	if withRepos != withoutRepos {
		t.Errorf("repo filter should not change query:\n  with=%q\n  without=%q", withRepos, withoutRepos)
	}
}

func TestBuildQuery_AuthorAndOrg(t *testing.T) {
	q := igithub.BuildQuery("ruben", "BadgerMaps", igithub.PRFilter{Authors: []string{"alice"}})
	if !strings.Contains(q, "author:alice") {
		t.Errorf("want 'author:alice', got %q", q)
	}
	if !strings.Contains(q, "org:BadgerMaps") {
		t.Errorf("want 'org:BadgerMaps', got %q", q)
	}
}

func TestBuildQuery_BaseFragments(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{})
	if !strings.HasPrefix(q, "is:pr is:open") {
		t.Errorf("query should start with 'is:pr is:open', got %q", q)
	}
}

func TestBuildQuery_HeadBranch(t *testing.T) {
	q := igithub.BuildQuery("ruben", "", igithub.PRFilter{HeadBranch: "agent/fixaut"})
	if !strings.Contains(q, "head:agent/fixaut") {
		t.Errorf("want 'head:agent/fixaut', got %q", q)
	}
}

func TestBuildQuery_HeadBranch_EmptyIsIgnored(t *testing.T) {
	with := igithub.BuildQuery("ruben", "", igithub.PRFilter{HeadBranch: ""})
	without := igithub.BuildQuery("ruben", "", igithub.PRFilter{})
	if with != without {
		t.Errorf("empty HeadBranch should not change query:\n  with=%q\n  without=%q", with, without)
	}
}

// ── ApplyRepoFilter ───────────────────────────────────────────────────────────

func makePR(repo string) igithub.PR {
	return igithub.PR{Number: 1, Repo: repo, Title: "t", UpdatedAt: time.Now()}
}

func TestApplyRepoFilter_EmptyRepos_ReturnsAll(t *testing.T) {
	prs := []igithub.PR{makePR("org/a"), makePR("org/b")}
	got := igithub.ApplyRepoFilter(prs, nil)
	if len(got) != 2 {
		t.Errorf("want 2 PRs, got %d", len(got))
	}
}

func TestApplyRepoFilter_MatchingRepos(t *testing.T) {
	prs := []igithub.PR{makePR("org/a"), makePR("org/b"), makePR("org/c")}
	got := igithub.ApplyRepoFilter(prs, []string{"org/a", "org/c"})
	if len(got) != 2 {
		t.Fatalf("want 2 PRs, got %d", len(got))
	}
	if got[0].Repo != "org/a" {
		t.Errorf("got[0].Repo: want 'org/a', got %q", got[0].Repo)
	}
	if got[1].Repo != "org/c" {
		t.Errorf("got[1].Repo: want 'org/c', got %q", got[1].Repo)
	}
}

func TestApplyRepoFilter_NoMatch_EmptyResult(t *testing.T) {
	prs := []igithub.PR{makePR("org/a")}
	got := igithub.ApplyRepoFilter(prs, []string{"org/other"})
	if len(got) != 0 {
		t.Errorf("want 0 PRs, got %d", len(got))
	}
}

func TestApplyRepoFilter_CaseSensitive(t *testing.T) {
	prs := []igithub.PR{makePR("Org/Repo")}
	got := igithub.ApplyRepoFilter(prs, []string{"org/repo"}) // different case
	if len(got) != 0 {
		t.Errorf("filter should be case-sensitive, want 0 PRs, got %d", len(got))
	}
}

func TestApplyRepoFilter_EmptyInput(t *testing.T) {
	got := igithub.ApplyRepoFilter(nil, []string{"org/a"})
	if len(got) != 0 {
		t.Errorf("want 0 PRs from nil input, got %d", len(got))
	}
}
