package prompt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxGitStatusChars = 2000

// computeGitStatus returns a git snapshot for the dynamic system prompt section.
// Returns ("", false) when workDir is not a git repository or git is unavailable.
func computeGitStatus(workDir string) (string, bool) {
	if strings.TrimSpace(workDir) == "" {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if !gitInsideWorkTree(ctx, workDir) {
		return "", false
	}

	branch := gitOutput(ctx, workDir, "rev-parse", "--abbrev-ref", "HEAD")
	status := gitOutput(ctx, workDir, "status", "--short")
	logLines := gitOutput(ctx, workDir, "log", "--oneline", "-n", "5")

	if branch == "" && status == "" && logLines == "" {
		return "", false
	}

	var b strings.Builder
	b.WriteString("This is the git status at the start of the conversation. ")
	b.WriteString("Note that this status is a snapshot in time, and will not update during the conversation.\n\n")
	if branch != "" {
		fmt.Fprintf(&b, "Current branch: %s\n\n", branch)
	}
	if status == "" {
		b.WriteString("Status:\n(clean)\n\n")
	} else {
		truncated := status
		if len(truncated) > maxGitStatusChars {
			truncated = truncated[:maxGitStatusChars] + "\n... (truncated — run git status for full output)"
		}
		fmt.Fprintf(&b, "Status:\n%s\n\n", truncated)
	}
	if logLines != "" {
		fmt.Fprintf(&b, "Recent commits:\n%s", logLines)
	}
	return strings.TrimSpace(b.String()), true
}

func gitInsideWorkTree(ctx context.Context, workDir string) bool {
	out := gitOutput(ctx, workDir, "rev-parse", "--is-inside-work-tree")
	return strings.TrimSpace(out) == "true"
}

func gitOutput(ctx context.Context, workDir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Clean(workDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}
