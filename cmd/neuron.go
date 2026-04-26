package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// executeAddNeuron adds a new Neuron of the given type to an existing Nucleus.
// createWorktree=true creates an isolated git worktree at repoPath/.worktrees/<nucleusID>-<neuronID>;
// false (the default) opens the neuron directly in the repo directory on the selected branch.
// createBranch=true creates a new branch (optionally from baseBranch); false checks out an existing one.
// For claude neurons, CLAUDE_CONFIG_DIR is read from the nucleus's Profile field.
func executeAddNeuron(nucleusID, neuronType, repoPath, branch, baseBranch string, createWorktree, createBranch bool, reg nucleusSvc, gt gitSvc, tm tmuxSvc) error {
	_, err := executeAddNeuronWithProfile(nucleusID, neuronType, repoPath, branch, baseBranch, "", createWorktree, createBranch, reg, gt, tm)
	return err
}

// executeAddNeuronWithProfile is like executeAddNeuron but accepts an explicit claudeConfigDir
// that overrides the nucleus-level Profile for this neuron's launch command. Pass "" to fall
// back to the nucleus Profile as usual.
func executeAddNeuronWithProfile(nucleusID, neuronType, repoPath, branch, baseBranch, claudeConfigDir string, createWorktree, createBranch bool, reg nucleusSvc, gt gitSvc, tm tmuxSvc) (string, error) {
	r, err := reg.Load()
	if err != nil {
		return "", err
	}
	n, err := r.FindByID(nucleusID)
	if err != nil {
		return "", err
	}

	neuronID := nextNeuronID(n.Neurons, neuronType)

	workdir := n.Workdir()
	if repoPath != "" {
		workdir = repoPath
	}

	var worktreePath string
	if branch != "" && repoPath != "" {
		if createWorktree {
			worktreePath = filepath.Join(repoPath, ".worktrees", nucleusID+"-"+neuronID)
			err := gt.AddWorktree(repoPath, worktreePath, branch, createBranch, baseBranch)
			if err != nil && !createBranch {
				// Branch may only exist remotely; create a local tracking branch from origin.
				err = gt.AddWorktree(repoPath, worktreePath, branch, true, "origin/"+branch)
			}
			if err != nil {
				return "", fmt.Errorf("add worktree: %w", err)
			}
			workdir = worktreePath
		} else if createBranch {
			if err := gt.CheckoutNewBranch(repoPath, branch, baseBranch); err != nil {
				return "", fmt.Errorf("git checkout -b %s: %w", branch, err)
			}
		} else {
			// No new worktree, no new branch: reuse a sibling neuron's worktree if one
			// already has this branch checked out (avoids "already checked out" errors),
			// otherwise open in the repo directory without switching branches.
			for _, existing := range n.Neurons {
				if existing.Branch == branch && existing.WorktreePath != "" {
					workdir = existing.WorktreePath
					break
				}
			}
		}
	}

	target, err := tm.NewWindow(workdir, neuronType+"-"+nucleusID)
	if err != nil {
		return "", fmt.Errorf("tmux new-window: %w", err)
	}

	var launchCmd string
	switch registry.NeuronType(neuronType) {
	case registry.NeuronClaude:
		profile := claudeConfigDir
		if profile == "" {
			profile = n.Profile
		}
		launchCmd = "claude"
		if profile != "" {
			launchCmd = "CLAUDE_CONFIG_DIR=" + profile + " claude"
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
		ID:           neuronID,
		Type:         registry.NeuronType(neuronType),
		TmuxTarget:   target,
		Status:       "idle",
		RepoPath:     repoPath,
		WorktreePath: worktreePath,
		Branch:       branch,
	}

	return target, reg.AddNeuron(nucleusID, neuron)
}

// executeRemoveNeuron kills the neuron's tmux pane, removes its worktree (if any),
// and deletes it from the registry. Tmux and git failures are best-effort (warned, not fatal).
func executeRemoveNeuron(nucleusID, neuronID string, reg nucleusSvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	n, err := r.FindByID(nucleusID)
	if err != nil {
		return err
	}
	neu, err := n.FindNeuronByID(neuronID)
	if err != nil {
		return err
	}

	if neu.TmuxTarget != "" {
		if err := tm.KillPane(neu.TmuxTarget); err != nil {
			fmt.Printf("warning: could not kill tmux pane %s: %v\n", neu.TmuxTarget, err)
		}
	}
	if neu.WorktreePath != "" {
		if err := gt.RemoveWorktree(neu.RepoPath, neu.WorktreePath); err != nil {
			fmt.Printf("warning: could not remove worktree %s: %v\n", neu.WorktreePath, err)
		}
	}
	return reg.RemoveNeuron(nucleusID, neuronID)
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
