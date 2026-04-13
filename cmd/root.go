package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "exocortex",
	Short: "Manage parallel AI coding agents across git worktrees",
	// RunE launches the TUI when called with no subcommand.
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("TMUX") == "" {
			return fmt.Errorf("exocortex must be run inside a tmux session")
		}
		return runTUI()
	},
	// PersistentPreRunE guards all subcommands that need tmux.
	// Subcommands that must run outside tmux (bar, init) override this
	// by defining their own PersistentPreRunE.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("TMUX") == "" {
			return fmt.Errorf("exocortex must be run inside a tmux session")
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(gotoCmd)
	rootCmd.AddCommand(nvimCmd)
	rootCmd.AddCommand(nvimCloseCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(barCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(respawnCmd)
}
