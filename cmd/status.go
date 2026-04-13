package cmd

import (
	"fmt"

	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Update an agent's status (hook target)",
	Long:  "Update an agent's status. Called automatically by Claude Code hooks.\nAccepts: idle | working | waiting | blocked",
	Args:  cobra.ExactArgs(2),
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	return executeStatus(args[0], args[1], reg)
}

func executeStatus(id, status string, reg registrySvc) error {
	switch status {
	case "idle", "working", "waiting", "blocked":
	default:
		return fmt.Errorf("unknown status %q: must be idle, working, waiting, or blocked", status)
	}
	return reg.UpdateStatus(id, status)
}
