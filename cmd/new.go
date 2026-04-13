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
	Short: "Create a new agent in a git worktree and tmux pane",
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

// executeNew creates a new agent. claudeConfigDir, when non-empty, is the path
// passed as CLAUDE_CONFIG_DIR when launching Claude Code (selects which profile to use).
func executeNew(task, repoArg, branch, claudeConfigDir string, reg registrySvc, gt gitSvc, tm tmuxSvc, out io.Writer) error {
	repoPath, err := filepath.Abs(repoArg)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	existing, err := reg.Load()
	if err != nil {
		return err
	}

	id := uniqueID(task, existing.Agents)

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

	// Write Claude Code hooks into the worktree — warn on failure, do not abort.
	// Uses .claude-work/ as the settings directory; change to ".claude" if needed.
	if err := hooks.Write(worktreePath, id, ".claude-work"); err != nil {
		fmt.Fprintf(out, "warning: could not write Claude Code hooks: %v\n", err)
	}

	target, err := tm.NewWindow(worktreePath, task)
	if err != nil {
		// Best-effort cleanup of the worktree we just created.
		_ = gt.RemoveWorktree(repoPath, worktreePath)
		return fmt.Errorf("tmux new-window: %w", err)
	}

	// Auto-start Claude Code in the new window, using the selected profile if set.
	claudeCmd := "claude"
	if claudeConfigDir != "" {
		claudeCmd = "CLAUDE_CONFIG_DIR=" + claudeConfigDir + " claude"
	}
	if err := tm.SendKeys(target, claudeCmd); err != nil {
		fmt.Fprintf(out, "warning: could not start claude: %v\n", err)
	}

	agent := registry.Agent{
		ID:              id,
		RepoPath:        repoPath,
		WorktreePath:    worktreePath,
		Branch:          branch,
		TaskDescription: task,
		TmuxTarget:      target,
		Profile:         claudeConfigDir,
		Status:          "idle",
		CreatedAt:       time.Now().UTC(),
	}

	if err := reg.Add(agent); err != nil {
		return fmt.Errorf("registry add: %w", err)
	}

	fmt.Fprintf(out, "created agent %s\n", id)
	return nil
}
