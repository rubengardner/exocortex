package cmd

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	iconfig "github.com/ruben_gardner/exocortex/internal/config"
	igit "github.com/ruben_gardner/exocortex/internal/git"
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
			return cfg.Repos, nil
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
		CreateNucleus: func(task, repo, branch, profileName string) error {
			claudeConfigDir := ""
			if profileName != "" {
				cfg, err := iconfig.Load(iconfig.DefaultPath())
				if err == nil {
					claudeConfigDir = cfg.Profiles[profileName]
				}
			}
			return executeNew(task, repo, branch, claudeConfigDir, reg, gt, tm, io.Discard)
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
	}
}
