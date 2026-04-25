package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

// executeAddNeuron adds a new Neuron of the given type to an existing Nucleus.
// createWorktree=true creates an isolated git worktree at repoPath/.worktrees/<nucleusID>-<neuronID>;
// false (the default) opens the neuron directly in the repo directory on the selected branch.
// createBranch=true creates a new branch (optionally from baseBranch); false checks out an existing one.
// For claude neurons, CLAUDE_CONFIG_DIR is read from the nucleus's Profile field.
func executeAddNeuron(nucleusID, neuronType, repoPath, branch, baseBranch string, createWorktree, createBranch bool, reg nucleusSvc, gt gitSvc, tm tmuxSvc) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	n, err := r.FindByID(nucleusID)
	if err != nil {
		return err
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
			if err := gt.AddWorktree(repoPath, worktreePath, branch, createBranch, baseBranch); err != nil {
				return fmt.Errorf("add worktree: %w", err)
			}
			workdir = worktreePath
		} else if createBranch {
			if err := gt.CheckoutNewBranch(repoPath, branch, baseBranch); err != nil {
				return fmt.Errorf("git checkout -b %s: %w", branch, err)
			}
		} else {
			if err := gt.Checkout(repoPath, branch); err != nil {
				return fmt.Errorf("git checkout %s: %w", branch, err)
			}
		}
	}

	target, err := tm.NewWindow(workdir, neuronType+"-"+nucleusID)
	if err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}

	var launchCmd string
	switch registry.NeuronType(neuronType) {
	case registry.NeuronClaude:
		launchCmd = "claude"
		if n.Profile != "" {
			launchCmd = "CLAUDE_CONFIG_DIR=" + n.Profile + " claude"
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

	return reg.AddNeuron(nucleusID, neuron)
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

// executeAppendPRNeuron adds an nvim neuron to an existing nucleus and links a PR.
// It checks out the existing branch (createBranch=false) into a dedicated worktree.
func executeAppendPRNeuron(nucleusID, repoPath, branch string, pr registry.PullRequest, reg nucleusSvc, gt gitSvc, tm tmuxSvc, _ io.Writer) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	n, err := r.FindByID(nucleusID)
	if err != nil {
		return err
	}

	neuronID := nextNeuronID(n.Neurons, string(registry.NeuronNvim))

	worktreePath := filepath.Join(repoPath, ".worktrees", nucleusID+"-pr"+strconv.Itoa(pr.Number))
	if err := gt.AddWorktree(repoPath, worktreePath, branch, false, ""); err != nil {
		return fmt.Errorf("add worktree: %w", err)
	}

	windowName := "PR#" + strconv.Itoa(pr.Number)
	target, err := tm.NewWindow(worktreePath, windowName)
	if err != nil {
		return fmt.Errorf("tmux new-window: %w", err)
	}

	if err := tm.SendKeys(target, "nvim ."); err != nil {
		fmt.Printf("warning: could not launch nvim: %v\n", err)
	}

	neuron := registry.Neuron{
		ID:           neuronID,
		Type:         registry.NeuronNvim,
		TmuxTarget:   target,
		Status:       "idle",
		RepoPath:     repoPath,
		WorktreePath: worktreePath,
		Branch:       branch,
	}
	if err := reg.AddNeuron(nucleusID, neuron); err != nil {
		return err
	}
	return reg.AddPullRequest(nucleusID, pr)
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
