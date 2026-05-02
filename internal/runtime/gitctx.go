package runtime

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitContext holds rich git information for system prompt injection
type GitContext struct {
	Branch     string
	Status     string
	Diff       string   // staged + unstaged diff summary
	RecentLogs []string // last 5 commit messages
	RemoteURL  string
}

// CollectGitContext gathers comprehensive git context from the working directory
func CollectGitContext(cwd string) *GitContext {
	branch := gitExec(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "" {
		return nil // not a git repo
	}

	gc := &GitContext{Branch: branch}

	// Status (short)
	gc.Status = gitExec(cwd, "status", "--short")

	// Diff stat (staged + unstaged, compact)
	diffStat := gitExec(cwd, "diff", "--stat", "--no-color")
	stagedStat := gitExec(cwd, "diff", "--cached", "--stat", "--no-color")
	var parts []string
	if stagedStat != "" {
		parts = append(parts, "Staged:\n"+truncLines(stagedStat, 8))
	}
	if diffStat != "" {
		parts = append(parts, "Unstaged:\n"+truncLines(diffStat, 8))
	}
	gc.Diff = strings.Join(parts, "\n")

	// Recent log (last 5 oneline)
	logOut := gitExec(cwd, "log", "--oneline", "-5", "--no-decorate")
	if logOut != "" {
		gc.RecentLogs = strings.Split(strings.TrimSpace(logOut), "\n")
	}

	// Remote URL
	gc.RemoteURL = gitExec(cwd, "remote", "get-url", "origin")

	return gc
}

// Format returns a formatted string for system prompt injection
func (gc *GitContext) Format() string {
	if gc == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nGit branch: %s\n", gc.Branch))

	if gc.RemoteURL != "" {
		sb.WriteString(fmt.Sprintf("Git remote: %s\n", gc.RemoteURL))
	}

	if gc.Status != "" {
		lines := strings.Split(strings.TrimSpace(gc.Status), "\n")
		sb.WriteString(fmt.Sprintf("Git status (%d files changed):\n", len(lines)))
		sb.WriteString(truncLines(gc.Status, 10) + "\n")
	}

	if gc.Diff != "" {
		sb.WriteString("Git diff summary:\n" + gc.Diff + "\n")
	}

	if len(gc.RecentLogs) > 0 {
		sb.WriteString("Recent commits:\n")
		for _, l := range gc.RecentLogs {
			sb.WriteString("  " + l + "\n")
		}
	}

	return sb.String()
}

func gitExec(cwd string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func truncLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.TrimSpace(s)
	}
	result := strings.Join(lines[:max], "\n")
	return result + fmt.Sprintf("\n...+%d more lines", len(lines)-max)
}
