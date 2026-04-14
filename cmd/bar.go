package cmd

import (
	"fmt"

	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var barCmd = &cobra.Command{
	Use:   "bar",
	Short: "Print tmux status-bar fragment for waiting nuclei",
	Long: "Reads the registry and prints a tmux-formatted fragment.\n" +
		"Output when nuclei are waiting: #[fg=yellow] N waiting #[default]\n" +
		"Output when none waiting: empty (disappears from status bar cleanly).\n" +
		"Designed for use in tmux status-right: set -g status-right \"#(exocortex bar)\"",
	Args: cobra.NoArgs,
	RunE: runBar,
	// Exempt from the tmux guard — tmux itself calls this via #(exocortex bar).
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
}

func runBar(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	out, err := barOutput(reg)
	if err == nil && out != "" {
		fmt.Print(out)
	}
	return nil
}

// barOutput returns the tmux status fragment, or empty string when nothing is waiting.
// Errors are swallowed — a broken bar is worse than a silent one.
func barOutput(reg nucleusSvc) (string, error) {
	r, err := reg.Load()
	if err != nil {
		return "", err
	}
	count := 0
	for _, n := range r.Nuclei {
		if n.Status == "waiting" {
			count++
		}
	}
	if count == 0 {
		return "", nil
	}
	return fmt.Sprintf("#[fg=yellow] %d waiting #[default]", count), nil
}
