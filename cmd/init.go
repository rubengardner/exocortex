package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Print recommended tmux.conf additions to stdout",
	Long: "Print the tmux configuration snippet that wires up the status bar\n" +
		"and the popup keybinding. Pipe to your config or copy manually:\n\n" +
		"  exocortex init >> ~/.tmux.conf\n" +
		"  tmux source ~/.tmux.conf",
	Args: cobra.NoArgs,
	RunE: runInit,
	// Exempt from the tmux guard — users run this outside tmux to set up their config.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Print(`# exocortex tmux integration
# Add to ~/.tmux.conf then run: tmux source ~/.tmux.conf

# Show waiting-agent count in status bar
set -g status-right "#(exocortex bar)"
set -g status-interval 5

# Toggle agent popup with prefix+t
bind-key t display-popup -w 80% -h 80% -E "exocortex"
`)
	return nil
}
