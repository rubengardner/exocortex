package cmd

import (
	"fmt"
	"io"

	iconfig "github.com/ruben_gardner/exocortex/internal/config"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var respawnCmd = &cobra.Command{
	Use:   "respawn <id>",
	Short: "Restart an agent's tmux window after tmux restart or accidental close",
	Args:  cobra.ExactArgs(1),
	RunE:  runRespawn,
}

func runRespawn(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	tm := itmux.New(itmux.ExecRunner{})
	return executeRespawn(args[0], reg, tm, cmd.OutOrStdout())
}

func executeRespawn(id string, reg registrySvc, tm tmuxSvc, out io.Writer) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	agent, err := r.FindByID(id)
	if err != nil {
		return err
	}

	// If the window is still alive, nothing to do.
	exists, err := tm.WindowExists(agent.TmuxTarget)
	if err != nil {
		return fmt.Errorf("check window: %w", err)
	}
	if exists {
		fmt.Fprintf(out, "agent %s is already running at %s\n", id, agent.TmuxTarget)
		return nil
	}

	// Open a new window in the worktree and restart Claude Code.
	target, err := tm.NewWindow(agent.WorktreePath, agent.TaskDescription)
	if err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}

	// Resolve the profile stored on the agent to a CLAUDE_CONFIG_DIR path.
	claudeCmd := "claude"
	if agent.Profile != "" {
		cfg, cfgErr := iconfig.Load(iconfig.DefaultPath())
		if cfgErr == nil {
			if path, ok := cfg.Profiles[agent.Profile]; ok && path != "" {
				claudeCmd = "CLAUDE_CONFIG_DIR=" + path + " claude"
			}
		}
	}
	if err := tm.SendKeys(target, claudeCmd); err != nil {
		return fmt.Errorf("send keys: %w", err)
	}

	// Persist the new target.
	if err := reg.UpdateTmuxTarget(id, target); err != nil {
		return fmt.Errorf("update tmux target: %w", err)
	}

	// Clear stale nvim target — that window is also gone.
	if agent.NvimTarget != "" {
		if err := reg.UpdateNvimTarget(id, ""); err != nil {
			fmt.Fprintf(out, "warning: could not clear nvim target: %v\n", err)
		}
	}

	fmt.Fprintf(out, "respawned agent %s at %s\n", id, target)
	return nil
}
