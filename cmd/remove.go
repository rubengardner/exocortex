package cmd

import (
	"fmt"

	igit "github.com/ruben_gardner/exocortex/internal/git"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an agent, its tmux pane, and its git worktree",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	gt := igit.New(igit.ExecRunner{})
	tm := itmux.New(itmux.ExecRunner{})
	return executeRemove(args[0], reg, gt, tm)
}

func executeRemove(id string, reg registrySvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	agent, err := r.FindByID(id)
	if err != nil {
		return err
	}

	// Kill tmux pane — warn on failure but continue cleanup.
	if err := tm.KillPane(agent.TmuxTarget); err != nil {
		fmt.Printf("warning: could not kill tmux pane %s: %v\n", agent.TmuxTarget, err)
	}

	// Kill nvim window if one was recorded — best-effort.
	if agent.NvimTarget != "" {
		if err := tm.KillPane(agent.NvimTarget); err != nil {
			fmt.Printf("warning: could not kill nvim window %s: %v\n", agent.NvimTarget, err)
		}
	}

	// Remove the git worktree — warn on failure but continue cleanup.
	if err := gt.RemoveWorktree(agent.RepoPath, agent.WorktreePath); err != nil {
		fmt.Printf("warning: could not remove worktree %s: %v\n", agent.WorktreePath, err)
	}

	return reg.Delete(id)
}
