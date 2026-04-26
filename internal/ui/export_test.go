package ui

import (
	"github.com/ruben_gardner/exocortex/internal/jira"
)

// ProfilesLoadedMsg returns a profilesLoadedMsg for use in external test files.
func ProfilesLoadedMsg(names []string) interface{} {
	paths := make(map[string]string)
	for _, n := range names {
		paths[n] = "/path/" + n
	}
	return profilesLoadedMsg{names: names, paths: paths}
}

// JiraBoardLoadedMsg returns a jiraBoardLoadedMsg for use in external test files.
func JiraBoardLoadedMsg(columns []string, issues map[string][]jira.Issue) interface{} {
	return jiraBoardLoadedMsg{columns: columns, issues: issues}
}

// JiraNucleusPick returns whether the Jira nucleus picker overlay is active.
func (m Model) JiraNucleusPick() bool { return m.jiraNucleusPick }

// JiraPendingKey returns the Jira key pending attachment.
func (m Model) JiraPendingKey() string { return m.jiraPendingKey }

// JiraKeyPickActive returns whether the Jira key picker overlay is active.
func (m Model) JiraKeyPickActive() bool { return m.jiraKeyPickActive }

// GitHubNucleusPick returns whether the GitHub nucleus picker overlay is active.
func (m Model) GitHubNucleusPick() bool { return m.githubNucleusPick }

// GitHubNucleusPickMode returns the current GitHub nucleus picker mode.
func (m Model) GitHubNucleusPickMode() string { return m.githubNucleusPickMode }

// GitHubProfilePick returns whether the GitHub profile picker overlay is active.
func (m Model) GitHubProfilePick() bool { return m.githubProfilePick }

// GitHubProfilePickMode returns the current GitHub profile picker mode.
func (m Model) GitHubProfilePickMode() string { return m.githubProfilePickMode }

// GitHubClaudeWorktreePick returns whether the Claude worktree picker overlay is active.
func (m Model) GitHubClaudeWorktreePick() bool { return m.githubClaudeWorktreePick }

// GitHubClaudeWorktreeCursor returns the current cursor in the worktree picker.
func (m Model) GitHubClaudeWorktreeCursor() int { return m.githubClaudeWorktreeCursor }

// GitHubWorktreeMode returns the current worktree picker mode ("add_claude" or "add_nvim").
func (m Model) GitHubWorktreeMode() string { return m.githubWorktreeMode }
