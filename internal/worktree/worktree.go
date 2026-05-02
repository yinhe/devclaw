package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worktree manages git worktree isolation for Drone tasks
type Worktree struct {
	RepoDir     string // original repo root
	WorktreeDir string // isolated worktree path
	BranchName  string // branch created for this task
}

// Create sets up an isolated git worktree for a task.
// Creates a new branch and worktree directory.
func Create(repoDir, taskID string) (*Worktree, error) {
	branch := fmt.Sprintf("drone/%s-%d", taskID, time.Now().Unix())
	wtDir := filepath.Join(os.TempDir(), "drone-worktree", taskID)

	// Clean up if exists from previous run
	os.RemoveAll(wtDir)

	// Create the worktree with a new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git worktree add: %s: %w", string(out), err)
	}

	return &Worktree{
		RepoDir:     repoDir,
		WorktreeDir: wtDir,
		BranchName:  branch,
	}, nil
}

// Cleanup removes the worktree and optionally the branch
func (w *Worktree) Cleanup(deleteBranch bool) error {
	// Remove worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", w.WorktreeDir)
	cmd.Dir = w.RepoDir
	cmd.CombinedOutput() // ignore error if already removed

	os.RemoveAll(w.WorktreeDir)

	// Optionally delete branch
	if deleteBranch {
		cmd = exec.Command("git", "branch", "-D", w.BranchName)
		cmd.Dir = w.RepoDir
		cmd.CombinedOutput()
	}
	return nil
}

// CommitAll stages and commits all changes in the worktree
func (w *Worktree) CommitAll(message string) error {
	// Stage all
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = w.WorktreeDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", string(out), err)
	}

	// Check if there are changes to commit
	cmd = exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = w.WorktreeDir
	if cmd.Run() == nil {
		return nil // nothing to commit
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = w.WorktreeDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", string(out), err)
	}
	return nil
}

// DiffSummary returns a summary of changes in the worktree
func (w *Worktree) DiffSummary() string {
	cmd := exec.Command("git", "diff", "--stat", "HEAD")
	cmd.Dir = w.WorktreeDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
