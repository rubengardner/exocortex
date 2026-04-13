package cmd

import (
	"fmt"
	"os"
)

// detectAgentID identifies the agent whose tmux pane is currently focused.
// It checks that TMUX_PANE is set (ensuring we are inside a tmux pane), then
// asks tmux for the current pane's session:window.pane target and matches it
// against the registry.
func detectAgentID(reg registrySvc, tm tmuxSvc) (string, error) {
	if os.Getenv("TMUX_PANE") == "" {
		return "", fmt.Errorf("not running inside a tmux pane ($TMUX_PANE not set)")
	}
	currentTarget, err := tm.CurrentTarget()
	if err != nil {
		return "", fmt.Errorf("detect agent: %w", err)
	}
	r, err := reg.Load()
	if err != nil {
		return "", err
	}
	for _, a := range r.Agents {
		if a.TmuxTarget == currentTarget {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("current pane %q is not a known agent pane", currentTarget)
}
