package cmd

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	iconfig "github.com/ruben_gardner/exocortex/internal/config"
	igit "github.com/ruben_gardner/exocortex/internal/git"
	igithub "github.com/ruben_gardner/exocortex/internal/github"
	ijira "github.com/ruben_gardner/exocortex/internal/jira"
	"github.com/ruben_gardner/exocortex/internal/registry"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/ruben_gardner/exocortex/internal/ui"
)

// runTUI launches the full-screen Bubble Tea interface.
func runTUI() error {
	svc := buildServices()
	m := ui.New(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

// buildServices wires real infrastructure into the ui.Services function fields.
func buildServices() ui.Services {
	regPath := registry.DefaultPath()
	reg := &registryAdapter{path: regPath}
	gt := igit.New(igit.ExecRunner{})
	tm := itmux.New(itmux.ExecRunner{})

	return ui.Services{
		LoadRepos: func() ([]string, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil {
				return nil, err
			}
			paths := make([]string, len(cfg.Repos))
			for i, r := range cfg.Repos {
				paths[i] = r.Path
			}
			return paths, nil
		},
		BaseBranchesForRepo: func(repoPath string) []string {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil {
				return nil
			}
			for _, r := range cfg.Repos {
				if r.Path == repoPath {
					return r.BaseBranches
				}
			}
			return nil
		},
		LoadProfiles: func() (map[string]string, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil {
				return nil, err
			}
			if len(cfg.Profiles) == 0 {
				return nil, nil
			}
			return cfg.Profiles, nil
		},
		LoadNuclei: func() ([]registry.Nucleus, error) {
			r, err := reg.Load()
			if err != nil {
				return nil, err
			}
			return r.Nuclei, nil
		},
		CreateNucleus: func(task, jiraKey, profile string) error {
			return executeCreateNucleusOnly(task, jiraKey, profile, reg)
		},
		RemoveNucleus: func(id string) error {
			return executeRemove(id, reg, gt, tm)
		},
		CapturePane: func(target string) (string, error) {
			return tm.CapturePane(target)
		},
		GotoNucleus: func(id string) error {
			return executeGoto(id, reg, tm)
		},
		GotoNeuron: func(nucleusID, neuronID string) error {
			r, err := reg.Load()
			if err != nil {
				return err
			}
			n, err := r.FindByID(nucleusID)
			if err != nil {
				return err
			}
			neu, err := n.FindNeuronByID(neuronID)
			if err != nil {
				return err
			}
			return tm.SelectPane(neu.TmuxTarget)
		},
		AddNeuron: func(nucleusID, neuronType, repoPath, branch, baseBranch string, createWorktree, createBranch bool) error {
			if err := executeAddNeuron(nucleusID, neuronType, repoPath, branch, baseBranch, createWorktree, createBranch, reg, gt, tm); err != nil {
				return err
			}
			if neuronType == string(registry.NeuronClaude) {
				autoLinkPRForBranch(nucleusID, repoPath, branch)
			}
			return nil
		},
		RemoveNeuron: func(nucleusID, neuronID string) error {
			return executeRemoveNeuron(nucleusID, neuronID, reg, gt, tm)
		},
		AddPullRequest: func(nucleusID string, pr registry.PullRequest) error {
			return registry.AddPullRequest(registry.DefaultPath(), nucleusID, pr)
		},
		LoadBranchInfo: func(worktreePath string) ([]string, []string, error) {
			modified, err := gt.ModifiedFiles(worktreePath)
			if err != nil {
				return nil, nil, err
			}
			ahead, err := gt.AheadCommits(worktreePath)
			if err != nil {
				return nil, nil, err
			}
			return modified, ahead, nil
		},
		OpenNvim: func(id string) error {
			return executeNvim(id, reg, gt, tm)
		},
		CloseNvim: func(id string) error {
			return executeCloseNvim(id, reg, tm)
		},
		RespawnNucleus: func(id string) error {
			return executeRespawn(id, reg, tm, io.Discard)
		},
		LoadJiraBoard: func() ([]string, map[string][]ijira.Issue, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.Jira == nil {
				return nil, nil, err
			}
			statuses := cfg.Jira.ResolvedStatuses()
			client := ijira.New(cfg.Jira.BaseURL, cfg.Jira.Email, cfg.Jira.APIToken)
			issues, err := client.FetchBoard(cfg.Jira.BoardID, cfg.Jira.Project, statuses, cfg.Jira.TeamID)
			if err != nil {
				return nil, nil, err
			}
			return statuses, issues, nil
		},
		LoadJiraIssue: func(key string) (string, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.Jira == nil {
				return "", err
			}
			client := ijira.New(cfg.Jira.BaseURL, cfg.Jira.Email, cfg.Jira.APIToken)
			return client.FetchIssueDescription(key)
		},
		LoadJiraIssueMeta: func(key string) (*ijira.Issue, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.Jira == nil {
				return nil, err
			}
			client := ijira.New(cfg.Jira.BaseURL, cfg.Jira.Email, cfg.Jira.APIToken)
			return client.FetchIssue(key)
		},
		LoadGitHubPRs: func(f igithub.PRFilter) ([]igithub.PR, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.GitHub == nil {
				return nil, err
			}
			client := igithub.New("https://api.github.com", cfg.GitHub.Token, cfg.GitHub.Org)
			query := igithub.BuildQuery(cfg.GitHub.MyLogin, cfg.GitHub.Org, f)
			prs, err := client.ListPRs(query)
			if err != nil {
				return nil, err
			}
			return igithub.ApplyRepoFilter(prs, f.Repos), nil
		},
		LoadGitHubFilterConfig: func() (string, []string, []string, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.GitHub == nil {
				return "", nil, nil, err
			}
			return cfg.GitHub.MyLogin, cfg.GitHub.Teammates, cfg.GitHubRepoNames(), nil
		},
		LoadGitHubPR: func(repo string, number int) (*igithub.PRDetail, error) {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.GitHub == nil {
				return nil, err
			}
			client := igithub.New("https://api.github.com", cfg.GitHub.Token, cfg.GitHub.Org)
			return client.FetchPRDetail(repo, number)
		},
		ListBranches: func(repoPath string) ([]string, error) {
			return gt.ListBranches(repoPath)
		},
		AddNvimNeuronFromPR: func(nucleusID, repo, branch string, createWorktree bool) error {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil {
				return err
			}
			repoPath := resolveRepoPath(cfg, repo)
			if repoPath == "." {
				if r, err2 := reg.Load(); err2 == nil {
					if n, err3 := r.FindByID(nucleusID); err3 == nil {
						for _, neu := range n.Neurons {
							if neu.RepoPath != "" {
								repoPath = neu.RepoPath
								break
							}
						}
					}
				}
			}
			target, err := executeAddNeuronWithProfile(nucleusID, "nvim", repoPath, branch, "", "", createWorktree, false, reg, gt, tm)
			if err != nil {
				return err
			}
			autoLinkPRForBranch(nucleusID, repoPath, branch)
			if target != "" {
				_ = tm.SelectPane(target)
			}
			return nil
		},
		AddClaudeNeuronFromPR: func(nucleusID, repo, branch, profile string, createWorktree bool) error {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil {
				return err
			}
			repoPath := resolveRepoPath(cfg, repo)
			if repoPath == "." {
				// Config doesn't contain this repo; fall back to an existing neuron's RepoPath.
				if r, err2 := reg.Load(); err2 == nil {
					if n, err3 := r.FindByID(nucleusID); err3 == nil {
						for _, neu := range n.Neurons {
							if neu.RepoPath != "" {
								repoPath = neu.RepoPath
								break
							}
						}
					}
				}
			}
			claudeConfigDir := ""
			if profile != "" {
				claudeConfigDir = cfg.Profiles[profile]
			}
			target, err := executeAddNeuronWithProfile(nucleusID, "claude", repoPath, branch, "", claudeConfigDir, createWorktree, false, reg, gt, tm)
			if err != nil {
				return err
			}
			autoLinkPRForBranch(nucleusID, repoPath, branch)
			if target != "" {
				_ = tm.SelectPane(target)
			}
			return nil
		},
		OpenNvimFile: func(nucleusID, filePath string, line int) error {
			return executeNvimFile(nucleusID, filePath, line, reg, gt, tm)
		},
		BrowserOpen: func(url string) error {
			return exec.Command("open", url).Start()
		},
		AddJiraKey: func(nucleusID, key string) error {
			return registry.AddJiraKey(registry.DefaultPath(), nucleusID, key)
		},
		OpenJiraKey: func(key string) error {
			cfg, err := iconfig.Load(iconfig.DefaultPath())
			if err != nil || cfg.Jira == nil {
				return nil
			}
			url := strings.TrimRight(cfg.Jira.BaseURL, "/") + "/browse/" + key
			return exec.Command("open", url).Start()
		},
	}
}

// autoLinkPRForBranch looks up an open GitHub PR for the given branch and
// attaches it to the nucleus registry entry. Best-effort: errors are silently
// ignored so neuron creation is never blocked by a failed PR lookup.
func autoLinkPRForBranch(nucleusID, repoPath, branch string) {
	if branch == "" {
		return
	}
	cfg, err := iconfig.Load(iconfig.DefaultPath())
	if err != nil || cfg.GitHub == nil {
		return
	}
	repoName := filepath.Base(repoPath)
	repoFull := cfg.GitHub.Org + "/" + repoName
	client := igithub.New("https://api.github.com", cfg.GitHub.Token, cfg.GitHub.Org)
	query := igithub.BuildQuery(cfg.GitHub.MyLogin, cfg.GitHub.Org, igithub.PRFilter{
		Repos:      []string{repoFull},
		HeadBranch: branch,
	})
	prs, err := client.ListPRs(query)
	if err != nil || len(prs) == 0 {
		return
	}
	pr := prs[0]
	_ = registry.AddPullRequest(registry.DefaultPath(), nucleusID, registry.PullRequest{
		Number: pr.Number,
		Repo:   pr.Repo,
		URL:    pr.URL,
	})
}

// resolveRepoPath maps a "org/repo" short name to the matching local repo path
// from config. Falls back to "." when no match is found.
func resolveRepoPath(cfg *iconfig.Config, prRepo string) string {
	repoName := prRepo
	if idx := strings.LastIndex(prRepo, "/"); idx >= 0 {
		repoName = prRepo[idx+1:]
	}
	for _, r := range cfg.Repos {
		if filepath.Base(r.Path) == repoName {
			return r.Path
		}
	}
	return "."
}
