package cmd

import (
	"fmt"
	"strings"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// executeAddNeuron adds a new Neuron of the given type to an existing Nucleus.
// For claude neurons, claudeConfigDir sets the CLAUDE_CONFIG_DIR env var.
func executeAddNeuron(nucleusID, neuronType, claudeConfigDir string, reg nucleusSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	n, err := r.FindByID(nucleusID)
	if err != nil {
		return err
	}

	neuronID := nextNeuronID(n.Neurons, neuronType)

	target, err := tm.NewWindow(n.WorktreePath, neuronType+"-"+nucleusID)
	if err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}

	var launchCmd string
	switch registry.NeuronType(neuronType) {
	case registry.NeuronClaude:
		launchCmd = "claude"
		if claudeConfigDir != "" {
			launchCmd = "CLAUDE_CONFIG_DIR=" + claudeConfigDir + " claude"
		}
	case registry.NeuronNvim:
		launchCmd = "nvim ."
	case registry.NeuronShell:
		// Plain shell — no launch command.
	}
	if launchCmd != "" {
		if err := tm.SendKeys(target, launchCmd); err != nil {
			fmt.Printf("warning: could not start %s: %v\n", neuronType, err)
		}
	}

	neuron := registry.Neuron{
		ID:         neuronID,
		Type:       registry.NeuronType(neuronType),
		TmuxTarget: target,
		Profile:    claudeConfigDir,
		Status:     "idle",
	}

	return reg.AddNeuron(nucleusID, neuron)
}

// nextNeuronID generates the next unique ID for a new neuron of the given type
// within an existing set of neurons.
//
// Prefixes: claude → "c", nvim → "nvim", shell → "sh", other → "n".
// Examples: no existing → "c1"; existing "c1" → "c2"; existing "nvim" → "nvim2".
func nextNeuronID(neurons []registry.Neuron, neuronType string) string {
	var prefix string
	switch registry.NeuronType(neuronType) {
	case registry.NeuronClaude:
		prefix = "c"
	case registry.NeuronNvim:
		prefix = "nvim"
	case registry.NeuronShell:
		prefix = "sh"
	default:
		prefix = "n"
	}

	maxNum := 0
	for _, n := range neurons {
		if n.ID == prefix {
			// Bare prefix with no numeric suffix (e.g. "nvim" from v1 migration).
			if maxNum < 1 {
				maxNum = 1
			}
		} else if strings.HasPrefix(n.ID, prefix) {
			rest := n.ID[len(prefix):]
			var num int
			if _, err := fmt.Sscanf(rest, "%d", &num); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}
	return fmt.Sprintf("%s%d", prefix, maxNum+1)
}
