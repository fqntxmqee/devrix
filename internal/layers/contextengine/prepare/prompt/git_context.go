package prompt

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
)

const maxGitStatusChars = 2000

// computeGitStatus returns a git snapshot for the dynamic system prompt section.
// Returns ("", false) when workDir is not a git repository or git is unavailable.
func computeGitStatus(workDir string, loc i18n.Locale) (string, bool) {
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

	statusOut := status
	truncated := false
	if len(statusOut) > maxGitStatusChars {
		statusOut = statusOut[:maxGitStatusChars]
		truncated = true
	}
	out := i18n.FormatGitStatus(loc, branch, statusOut, logLines, truncated)
	if out == "" {
		return "", false
	}
	return out, true
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
