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

# ── Status bar ────────────────────────────────────────────────────────────────
# Show nucleus IDs with neuron counts and waiting indicator.
# Example: cyan "fixaut(2) review(1)" + yellow "1 waiting"
set -g status-right "#(exocortex bar)"
set -g status-interval 5

# ── Popup TUI ─────────────────────────────────────────────────────────────────
# Toggle the exocortex TUI in a centered popup with prefix+t.
bind-key t display-popup -w 80% -h 80% -E "exocortex"

# ── Multi-neuron window names ─────────────────────────────────────────────────
# Each nucleus opens panes in a dedicated window named after its ID.
# Disable auto-rename so window names set by exocortex are preserved.
set-window-option -g automatic-rename off

# Show pane index and title in the pane border when multiple panes are open.
# Neuron type is sent as the pane title by exocortex (e.g. "claude", "nvim").
set-option -g pane-border-status top
set-option -g pane-border-format " #{pane_index}: #{pane_title} "

# Optional: give each nucleus window a distinct colour in the status bar.
# Exocortex names windows <id>-CLAUDE, <id>-DEV, <id>-SH for each neuron type,
# so you can match on the suffix with a format string if desired.
# set-window-option -g window-status-current-format " #W #[fg=cyan]● "
`)
	return nil
}
