package cmd

import (
	"fmt"

	igit "github.com/ruben_gardner/exocortex/internal/git"
	"github.com/ruben_gardner/exocortex/internal/registry"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a nucleus, its tmux panes, and its git worktree",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	gt := igit.New(igit.ExecRunner{})
	tm := itmux.New(itmux.ExecRunner{})
	return executeRemove(args[0], reg, gt, tm)
}

func executeRemove(id string, reg nucleusSvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	nucleus, err := r.FindByID(id)
	if err != nil {
		return err
	}

	// Kill all neuron panes — warn on failure but continue cleanup.
	for _, neuron := range nucleus.Neurons {
		if neuron.TmuxTarget == "" {
			continue
		}
		if err := tm.KillPane(neuron.TmuxTarget); err != nil {
			fmt.Printf("warning: could not kill tmux pane %s (%s): %v\n", neuron.TmuxTarget, neuron.ID, err)
		}
	}

	// Remove the git worktree — warn on failure but continue cleanup.
	if err := gt.RemoveWorktree(nucleus.RepoPath, nucleus.WorktreePath); err != nil {
		fmt.Printf("warning: could not remove worktree %s: %v\n", nucleus.WorktreePath, err)
	}

	return reg.Delete(id)
}
