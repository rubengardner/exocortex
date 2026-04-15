package registry

import (
	"fmt"
	"time"
)

// NeuronType identifies what kind of process a Neuron is.
type NeuronType string

const (
	NeuronClaude NeuronType = "claude"
	NeuronNvim   NeuronType = "nvim"
	NeuronShell  NeuronType = "shell"
)

// Neuron is a running process (tmux pane) inside a Nucleus.
type Neuron struct {
	ID         string     `json:"id"`
	Type       NeuronType `json:"type"`
	TmuxTarget string     `json:"tmux_target"`
	Profile    string     `json:"profile,omitempty"` // CLAUDE_CONFIG_DIR path (claude neurons only)
	Status     string     `json:"status"`
}

// Nucleus is the top-level unit of work. It owns one git worktree and branch
// and may contain multiple Neurons (processes) working within it.
type Nucleus struct {
	ID              string    `json:"id"`
	RepoPath        string    `json:"repo_path"`
	WorktreePath    string    `json:"worktree_path"`
	Branch          string    `json:"branch"`
	TaskDescription string    `json:"task_description"`

	// Optional external linkage.
	JiraKey  string `json:"jira_key,omitempty"`
	PRNumber int    `json:"pr_number,omitempty"`
	PRRepo   string `json:"pr_repo,omitempty"`

	Neurons []Neuron `json:"neurons"`

	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Workdir returns the directory that new tmux windows for this nucleus should
// open in. When a worktree exists it is preferred; otherwise the repo root is
// used. This covers the case where a nucleus was created without a worktree
// (e.g. a review nucleus with createWorktree=false).
func (n *Nucleus) Workdir() string {
	if n.WorktreePath != "" {
		return n.WorktreePath
	}
	return n.RepoPath
}

// FindNeuronByID returns a pointer to the Neuron with the given ID.
// Returns an error if the Neuron is not found.
func (n *Nucleus) FindNeuronByID(id string) (*Neuron, error) {
	for i := range n.Neurons {
		if n.Neurons[i].ID == id {
			return &n.Neurons[i], nil
		}
	}
	return nil, fmt.Errorf("registry: neuron %q not found in nucleus %q", id, n.ID)
}

// PrimaryNeuron returns the first Claude Neuron, or the first Neuron of any type
// if none are Claude. Returns nil if there are no Neurons.
func (n *Nucleus) PrimaryNeuron() *Neuron {
	for i := range n.Neurons {
		if n.Neurons[i].Type == NeuronClaude {
			return &n.Neurons[i]
		}
	}
	if len(n.Neurons) > 0 {
		return &n.Neurons[0]
	}
	return nil
}

// NvimNeuron returns the first Neuron of type NeuronNvim, or nil if none exists.
func (n *Nucleus) NvimNeuron() *Neuron {
	for i := range n.Neurons {
		if n.Neurons[i].Type == NeuronNvim {
			return &n.Neurons[i]
		}
	}
	return nil
}
