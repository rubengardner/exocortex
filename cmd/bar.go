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

// barOutput returns the tmux status fragment showing active nuclei with their
// neuron counts and a waiting indicator when relevant.
// Returns empty string when there is nothing worth displaying.
// Errors are swallowed — a broken bar is worse than a silent one.
func barOutput(reg nucleusSvc) (string, error) {
	r, err := reg.Load()
	if err != nil {
		return "", err
	}

	var nucleiParts []string
	waiting := 0
	for _, n := range r.Nuclei {
		if n.Status == "waiting" {
			waiting++
		}
		if len(n.Neurons) > 0 {
			nucleiParts = append(nucleiParts, fmt.Sprintf("%s(%d)", n.ID, len(n.Neurons)))
		}
	}

	var out string
	if len(nucleiParts) > 0 {
		out += "#[fg=cyan] " + joinStrings(nucleiParts, " ") + " #[default]"
	}
	if waiting > 0 {
		out += fmt.Sprintf("#[fg=yellow] %d waiting #[default]", waiting)
	}
	return out, nil
}

// joinStrings concatenates ss with sep between each element.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}
