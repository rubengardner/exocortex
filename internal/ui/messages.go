package ui

import (
	"github.com/ruben_gardner/exocortex/internal/github"
	"github.com/ruben_gardner/exocortex/internal/jira"
	"github.com/ruben_gardner/exocortex/internal/registry"
)

// nucleiLoadedMsg is emitted when the async nucleus load completes.
type nucleiLoadedMsg struct {
	nuclei []registry.Nucleus
	err    error
}

// reposLoadedMsg is emitted when the repo list load completes.
type reposLoadedMsg struct {
	repos []string
	err   error
}

// profilesLoadedMsg is emitted when the profile list load completes.
type profilesLoadedMsg struct {
	names []string          // sorted display names
	paths map[string]string // name → CLAUDE_CONFIG_DIR path
	err   error
}

// actionDoneMsg is emitted after any nucleus-mutating side effect completes.
type actionDoneMsg struct {
	err       error
	quitAfter bool // quit the TUI on success (used by GotoNucleus)
}

// tickMsg fires on the periodic preview timer.
type tickMsg struct{}

// paneCapturedMsg carries the latest capture-pane output.
type paneCapturedMsg struct {
	content string
	err     error
}

// jiraBoardLoadedMsg is emitted when the Jira board fetch completes.
type jiraBoardLoadedMsg struct {
	columns []string
	issues  map[string][]jira.Issue
	err     error
}

// jiraIssueLoadedMsg is emitted when a single Jira issue description is fetched.
type jiraIssueLoadedMsg struct {
	key      string
	title    string // "KEY — Summary"
	markdown string
	err      error
}

// branchInfoLoadedMsg is emitted when git status/log data is fetched for a nucleus.
type branchInfoLoadedMsg struct {
	modified     []string // relative paths of modified files
	aheadCommits []string // one-line log entries ahead of upstream
	err          error
}

// githubPRsLoadedMsg is emitted when the GitHub PR list fetch completes.
type githubPRsLoadedMsg struct {
	prs []github.PR
	err error
}

// githubPRDetailLoadedMsg is emitted when a single PR detail fetch completes.
type githubPRDetailLoadedMsg struct {
	detail *github.PRDetail
	err    error
}
