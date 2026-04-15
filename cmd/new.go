package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	igit "github.com/ruben_gardner/exocortex/internal/git"
	"github.com/ruben_gardner/exocortex/internal/hooks"
	itmux "github.com/ruben_gardner/exocortex/internal/tmux"
	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new nucleus in a git worktree and tmux pane",
	RunE:  runNew,
}

var (
	newRepo   string
	newTask   string
	newBranch string
)

func init() {
	newCmd.Flags().StringVar(&newRepo, "repo", ".", "Path to the git repository (default: current directory)")
	newCmd.Flags().StringVar(&newTask, "task", "", "Task description (required)")
	newCmd.Flags().StringVar(&newBranch, "branch", "", "Branch name (auto-generated from task if omitted)")
	_ = newCmd.MarkFlagRequired("task")
}

func runNew(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	gt := igit.New(igit.ExecRunner{})
	tm := itmux.New(itmux.ExecRunner{})
	return executeNew(newTask, newRepo, newBranch, "", "", true, reg, gt, tm, cmd.OutOrStdout())
}

// executeNew creates a new nucleus with a single Claude neuron. claudeConfigDir,
// when non-empty, is passed as CLAUDE_CONFIG_DIR when launching Claude Code.
// jiraKey, when non-empty, is stored on the Nucleus for Jira linkage.
// When createWorktree is false the nucleus is opened directly in the repo
// directory without creating an isolated git worktree.
func executeNew(task, repoArg, branch, claudeConfigDir, jiraKey string, createWorktree bool, reg nucleusSvc, gt gitSvc, tm tmuxSvc, out io.Writer) error {
	repoPath, err := filepath.Abs(repoArg)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	existing, err := reg.Load()
	if err != nil {
		return err
	}

	id := uniqueID(task, existing.Nuclei)

	if branch == "" {
		branch = "agent/" + id
	}

	windowDir := repoPath
	var worktreePath string

	if createWorktree {
		worktreePath = filepath.Join(repoPath, ".worktrees", id)

		exists, err := gt.BranchExists(repoPath, branch)
		if err != nil {
			return fmt.Errorf("check branch: %w", err)
		}
		createBranch := !exists

		if err := gt.AddWorktree(repoPath, worktreePath, branch, createBranch); err != nil {
			return fmt.Errorf("git worktree add: %w", err)
		}

		// Write Claude Code hooks — warn on failure, do not abort.
		if err := hooks.Write(worktreePath, id, ".claude-work"); err != nil {
			fmt.Fprintf(out, "warning: could not write Claude Code hooks: %v\n", err)
		}

		windowDir = worktreePath
	}

	target, err := tm.NewWindow(windowDir, task)
	if err != nil {
		if createWorktree && worktreePath != "" {
			_ = gt.RemoveWorktree(repoPath, worktreePath)
		}
		return fmt.Errorf("tmux new-window: %w", err)
	}

	claudeCmd := "claude"
	if claudeConfigDir != "" {
		claudeCmd = "CLAUDE_CONFIG_DIR=" + claudeConfigDir + " claude"
	}
	if err := tm.SendKeys(target, claudeCmd); err != nil {
		fmt.Fprintf(out, "warning: could not start claude: %v\n", err)
	}

	nucleus := registry.Nucleus{
		ID:              id,
		RepoPath:        repoPath,
		WorktreePath:    worktreePath,
		Branch:          branch,
		TaskDescription: task,
		JiraKey:         jiraKey,
		Neurons: []registry.Neuron{
			{
				ID:         "c1",
				Type:       registry.NeuronClaude,
				TmuxTarget: target,
				Profile:    claudeConfigDir,
				Status:     "idle",
			},
		},
		Status:    "idle",
		CreatedAt: time.Now().UTC(),
	}

	if err := reg.Add(nucleus); err != nil {
		return fmt.Errorf("registry add: %w", err)
	}

	fmt.Fprintf(out, "created nucleus %s\n", id)
	return nil
}

// executeReview creates a Nucleus on an existing branch for PR review.
// Unlike executeNew, it checks out an existing branch without -b.
// When createWorktree is false the nucleus opens in the repo root without
// creating an isolated worktree.
func executeReview(task, repoArg, branch, claudeConfigDir string, prNumber int, prRepo string, createWorktree bool, reg nucleusSvc, gt gitSvc, tm tmuxSvc, out io.Writer) error {
	repoPath, err := filepath.Abs(repoArg)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	existing, err := reg.Load()
	if err != nil {
		return err
	}

	id := uniqueID(task, existing.Nuclei)

	windowDir := repoPath
	var worktreePath string

	if createWorktree {
		worktreePath = filepath.Join(repoPath, ".worktrees", id)

		// Check out existing branch — no -b flag.
		if err := gt.AddWorktree(repoPath, worktreePath, branch, false); err != nil {
			return fmt.Errorf("git worktree add: %w", err)
		}

		// Write Claude Code hooks — warn on failure, do not abort.
		if err := hooks.Write(worktreePath, id, ".claude-work"); err != nil {
			fmt.Fprintf(out, "warning: could not write Claude Code hooks: %v\n", err)
		}

		windowDir = worktreePath
	} else {
		// No worktree — switch the repo itself to the PR branch.
		if err := gt.Checkout(repoPath, branch); err != nil {
			return fmt.Errorf("git checkout %s: %w", branch, err)
		}
	}

	target, err := tm.NewWindow(windowDir, task)
	if err != nil {
		if createWorktree && worktreePath != "" {
			_ = gt.RemoveWorktree(repoPath, worktreePath)
		}
		return fmt.Errorf("tmux new-window: %w", err)
	}

	claudeCmd := "claude"
	if claudeConfigDir != "" {
		claudeCmd = "CLAUDE_CONFIG_DIR=" + claudeConfigDir + " claude"
	}
	if err := tm.SendKeys(target, claudeCmd); err != nil {
		fmt.Fprintf(out, "warning: could not start claude: %v\n", err)
	}

	nucleus := registry.Nucleus{
		ID:              id,
		RepoPath:        repoPath,
		WorktreePath:    worktreePath,
		Branch:          branch,
		TaskDescription: task,
		PRNumber:        prNumber,
		PRRepo:          prRepo,
		Neurons: []registry.Neuron{
			{
				ID:         "c1",
				Type:       registry.NeuronClaude,
				TmuxTarget: target,
				Profile:    claudeConfigDir,
				Status:     "idle",
			},
		},
		Status:    "idle",
		CreatedAt: time.Now().UTC(),
	}

	if err := reg.Add(nucleus); err != nil {
		return fmt.Errorf("registry add: %w", err)
	}

	fmt.Fprintf(out, "created nucleus %s\n", id)
	return nil
}
