package cmd

import (
	"fmt"
	"io"

	iconfig "github.com/ruben_gardner/exocortex/internal/config"
	"github.com/ruben_gardner/exocortex/internal/registry"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/spf13/cobra"
)

var respawnCmd = &cobra.Command{
	Use:   "respawn <id>",
	Short: "Restart a nucleus's Claude window after tmux restart or accidental close",
	Args:  cobra.ExactArgs(1),
	RunE:  runRespawn,
}

func runRespawn(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	tm := itmux.New(itmux.ExecRunner{})
	return executeRespawn(args[0], reg, tm, cmd.OutOrStdout())
}

func executeRespawn(id string, reg nucleusSvc, tm tmuxSvc, out io.Writer) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	nucleus, err := r.FindByID(id)
	if err != nil {
		return err
	}

	primary := nucleus.PrimaryNeuron()
	if primary == nil {
		return fmt.Errorf("nucleus %q has no neurons to respawn", id)
	}

	// If the window is still alive, nothing to do.
	exists, err := tm.WindowExists(primary.TmuxTarget)
	if err != nil {
		return fmt.Errorf("check window: %w", err)
	}
	if exists {
		fmt.Fprintf(out, "nucleus %s is already running at %s\n", id, primary.TmuxTarget)
		return nil
	}

	// Open a new window in the worktree and restart Claude Code.
	target, err := tm.NewWindow(nucleus.Workdir(), nucleus.TaskDescription)
	if err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}

	// Resolve the profile stored on the nucleus to a CLAUDE_CONFIG_DIR path.
	claudeCmd := "claude"
	if nucleus.Profile != "" {
		cfg, cfgErr := iconfig.Load(iconfig.DefaultPath())
		if cfgErr == nil {
			if path, ok := cfg.Profiles[nucleus.Profile]; ok && path != "" {
				claudeCmd = "CLAUDE_CONFIG_DIR=" + path + " claude"
			}
		}
	}
	if err := tm.SendKeys(target, claudeCmd); err != nil {
		return fmt.Errorf("send keys: %w", err)
	}

	// Persist the new target on the primary neuron.
	if err := reg.UpdateNeuronTarget(id, primary.ID, target); err != nil {
		return fmt.Errorf("update neuron target: %w", err)
	}

	// Remove stale nvim neuron — that window is also gone.
	if nvim := nucleus.NvimNeuron(); nvim != nil {
		if err := reg.RemoveNeuron(id, nvim.ID); err != nil {
			fmt.Fprintf(out, "warning: could not remove nvim neuron: %v\n", err)
		}
	}

	fmt.Fprintf(out, "respawned nucleus %s at %s\n", id, target)
	return nil
}
