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
	return executeNew(newTask, newRepo, newBranch, "", reg, gt, tm, cmd.OutOrStdout())
}

// executeNew creates a new nucleus with a single Claude neuron. claudeConfigDir,
// when non-empty, is passed as CLAUDE_CONFIG_DIR when launching Claude Code.
func executeNew(task, repoArg, branch, claudeConfigDir string, reg nucleusSvc, gt gitSvc, tm tmuxSvc, out io.Writer) error {
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

	worktreePath := filepath.Join(repoPath, ".worktrees", id)

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

	target, err := tm.NewWindow(worktreePath, task)
	if err != nil {
		_ = gt.RemoveWorktree(repoPath, worktreePath)
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
