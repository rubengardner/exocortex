package cmd

import (
	"github.com/ruben_gardner/exocortex/internal/registry"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/spf13/cobra"
)

var gotoCmd = &cobra.Command{
	Use:   "goto [id]",
	Short: "Switch tmux focus to an agent's pane",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGoto,
}

func runGoto(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	tm := itmux.New(itmux.ExecRunner{})
	id, err := resolveID(args, reg, tm)
	if err != nil {
		return err
	}
	return executeGoto(id, reg, tm)
}

// resolveID returns the agent ID from args[0] if provided, or detects it from
// the current tmux pane when called with no arguments.
func resolveID(args []string, reg registrySvc, tm tmuxSvc) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	return detectAgentID(reg, tm)
}

func executeGoto(id string, reg registrySvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	agent, err := r.FindByID(id)
	if err != nil {
		return err
	}
	return tm.SelectPane(agent.TmuxTarget)
}
