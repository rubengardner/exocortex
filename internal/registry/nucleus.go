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
// Each Neuron owns its own repo, worktree, and branch so a single Nucleus can
// span multiple repositories simultaneously.
type Neuron struct {
	ID           string     `json:"id"`
	Type         NeuronType `json:"type"`
	TmuxTarget   string     `json:"tmux_target"`
	Status       string     `json:"status"`
	RepoPath     string     `json:"repo_path,omitempty"`     // absolute path to git repo root
	WorktreePath string     `json:"worktree_path,omitempty"` // absolute path to git worktree (empty = repo root)
	Branch       string     `json:"branch,omitempty"`        // branch this neuron is working on
}

// Workdir returns the working directory for this neuron: the worktree when one
// exists, otherwise the repo root.
func (neu *Neuron) Workdir() string {
	if neu.WorktreePath != "" {
		return neu.WorktreePath
	}
	return neu.RepoPath
}

// PullRequest records an external pull request linked to a Nucleus.
type PullRequest struct {
	Number int    `json:"number"`
	Repo   string `json:"repo"`
	URL    string `json:"url,omitempty"`
}

// Nucleus is the top-level unit of work. It is a logical grouping of neurons
// (one per repo/task slice) and may be linked to any number of Jira tickets and
// pull requests.
type Nucleus struct {
	ID              string        `json:"id"`
	TaskDescription string        `json:"task_description"`
	JiraKeys        []string      `json:"jira_keys,omitempty"`
	Profile         string        `json:"profile,omitempty"` // CLAUDE_CONFIG_DIR path; inherited by all Claude neurons
	PullRequests    []PullRequest `json:"pull_requests,omitempty"`
	Neurons         []Neuron      `json:"neurons"`
	Status          string        `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
}

// HasJiraKey reports whether key is already linked to this nucleus.
func (n *Nucleus) HasJiraKey(key string) bool {
	for _, k := range n.JiraKeys {
		if k == key {
			return true
		}
	}
	return false
}

// Workdir returns the working directory for this nucleus by delegating to the
// primary neuron. Returns "" when there are no neurons.
func (n *Nucleus) Workdir() string {
	if neu := n.PrimaryNeuron(); neu != nil {
		return neu.Workdir()
	}
	return ""
}

// PrimaryBranch returns the branch of the primary neuron, or "" if there are none.
func (n *Nucleus) PrimaryBranch() string {
	if neu := n.PrimaryNeuron(); neu != nil {
		return neu.Branch
	}
	return ""
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
