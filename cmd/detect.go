package cmd

import (
	"fmt"
	"os"
)

// detectAgentID identifies the nucleus whose tmux pane is currently focused.
// It checks that TMUX_PANE is set, then asks tmux for the current pane's target
// and matches it against all neurons in the registry.
func detectAgentID(reg nucleusSvc, tm tmuxSvc) (string, error) {
	if os.Getenv("TMUX_PANE") == "" {
		return "", fmt.Errorf("not running inside a tmux pane ($TMUX_PANE not set)")
	}
	currentTarget, err := tm.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("detect nucleus: %w", err)
	}
	r, err := reg.Load()
	if err != nil {
		return "", err
	}
	for _, n := range r.Nuclei {
		for _, neuron := range n.Neurons {
			if neuron.TmuxTarget == currentTarget {
				return n.ID, nil
			}
		}
	}
	return "", fmt.Errorf("current pane %q is not a known nucleus pane", currentTarget)
}
