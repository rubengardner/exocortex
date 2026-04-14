package cmd

import (
	"fmt"

	igit "github.com/ruben_gardner/exocortex/internal/git"
	"github.com/ruben_gardner/exocortex/internal/registry"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/spf13/cobra"
)

var nvimCloseCmd = &cobra.Command{
	Use:   "nvim-close [id]",
	Short: "Kill the nvim window for a nucleus",
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

func executeCloseNvim(id string, reg nucleusSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	nucleus, err := r.FindByID(id)
	if err != nil {
		return err
	}
	nvimNeuron := nucleus.NvimNeuron()
	if nvimNeuron == nil {
		return nil // nothing to close
	}
	// Best-effort kill — the window may already be gone.
	_ = tm.KillPane(nvimNeuron.TmuxTarget)
	return reg.RemoveNeuron(id, nvimNeuron.ID)
}

var nvimCmd = &cobra.Command{
	Use:   "nvim [id]",
	Short: "Open-or-focus a nucleus's nvim window (<id>-DEV)",
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

func executeNvim(id string, reg nucleusSvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	nucleus, err := r.FindByID(id)
	if err != nil {
		return err
	}

	// If an nvim neuron exists and its window is still alive, switch to it.
	nvimNeuron := nucleus.NvimNeuron()
	if nvimNeuron != nil {
		exists, err := tm.WindowExists(nvimNeuron.TmuxTarget)
		if err == nil && exists {
			return tm.SelectPane(nvimNeuron.TmuxTarget)
		}
	}

	// Open a new window and launch nvim.
	files, err := gt.ModifiedFiles(nucleus.WorktreePath)
	if err != nil {
		return fmt.Errorf("git ls-files: %w", err)
	}
	file := "."
	if len(files) > 0 {
		file = files[0]
	}

	target, err := tm.NewWindow(nucleus.WorktreePath, id+"-DEV")
	if err != nil {
		return err
	}
	if err := tm.SendKeys(target, "nvim "+file); err != nil {
		return err
	}

	// Persist the nvim neuron: update if it already existed (but window was dead),
	// or add a new one if this is the first time.
	if nvimNeuron != nil {
		return reg.UpdateNeuronTarget(id, nvimNeuron.ID, target)
	}
	return reg.AddNeuron(id, registry.Neuron{
		ID:         "nvim",
		Type:       registry.NeuronNvim,
		TmuxTarget: target,
		Status:     "idle",
	})
}
