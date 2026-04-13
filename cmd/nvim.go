package cmd

import (
	"fmt"

	igit "github.com/ruben_gardner/exocortex/internal/git"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var nvimCloseCmd = &cobra.Command{
	Use:   "nvim-close [id]",
	Short: "Kill the nvim window for an agent",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNvimClose,
}

func runNvimClose(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	tm := itmux.New(itmux.ExecRunner{})
	id, err := resolveID(args, reg, tm)
	if err != nil {
		return err
	}
	return executeCloseNvim(id, reg, tm)
}

func executeCloseNvim(id string, reg registrySvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	agent, err := r.FindByID(id)
	if err != nil {
		return err
	}
	if agent.NvimTarget == "" {
		return nil // nothing to close
	}
	// Best-effort kill — the window may already be gone.
	_ = tm.KillPane(agent.NvimTarget)
	return reg.UpdateNvimTarget(id, "")
}

var nvimCmd = &cobra.Command{
	Use:   "nvim [id]",
	Short: "Open-or-focus an agent's nvim window (<id>-DEV)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNvim,
}

func runNvim(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	gt := igit.New(igit.ExecRunner{})
	tm := itmux.New(itmux.ExecRunner{})
	id, err := resolveID(args, reg, tm)
	if err != nil {
		return err
	}
	return executeNvim(id, reg, gt, tm)
}

func executeNvim(id string, reg registrySvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	agent, err := r.FindByID(id)
	if err != nil {
		return err
	}

	// If a nvim window was previously opened and is still alive, switch to it.
	if agent.NvimTarget != "" {
		exists, err := tm.WindowExists(agent.NvimTarget)
		if err == nil && exists {
			return tm.SelectPane(agent.NvimTarget)
		}
	}

	// Open a new window and launch nvim.
	files, err := gt.ModifiedFiles(agent.WorktreePath)
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	file := "."
	if len(files) > 0 {
		file = files[0]
	}

	target, err := tm.NewWindow(agent.WorktreePath, id+"-DEV")
	if err != nil {
		return err
	}
	if err := tm.SendKeys(target, "nvim "+file); err != nil {
		return err
	}
	return reg.UpdateNvimTarget(id, target)
}
