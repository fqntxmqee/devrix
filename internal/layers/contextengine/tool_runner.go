package contextengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
)

const (
	defaultToolTimeout   = 60 * time.Second
	defaultMaxToolOutput = 64 * 1024
)

// BuiltinToolRunner executes built-in tools (bash, read_file, write_file) in the session workspace.
type BuiltinToolRunner struct {
	Timeout        time.Duration
	MaxOutputBytes int
	Policy         *CommandPolicy
	AuditEnabled   bool
}

// NewBuiltinToolRunner creates a production tool runner with safe defaults.
func NewBuiltinToolRunner() *BuiltinToolRunner {
	return NewBuiltinToolRunnerFromConfig(config.DefaultToolConfig())
}

// NewBuiltinToolRunnerFromConfig builds a runner from tool configuration.
func NewBuiltinToolRunnerFromConfig(toolCfg *config.ToolConfig) *BuiltinToolRunner {
	if toolCfg == nil {
		toolCfg = config.DefaultToolConfig()
	}
	enabled := toolCfg.SandboxEnabled()
	policy := NewCommandPolicy(
		enabled,
		toolCfg.Sandbox.AllowlistExtra,
		toolCfg.Sandbox.DenyPatternsExtra,
	)
	return &BuiltinToolRunner{
		Timeout:        defaultToolTimeout,
		MaxOutputBytes: defaultMaxToolOutput,
		Policy:         policy,
		AuditEnabled:   enabled,
	}
}

// Execute runs a tool call in the workspace from context.
func (r *BuiltinToolRunner) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	workDir, err := ResolveToolWorkDir(ctx)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}

	switch call.Name {
	case "bash":
		return r.runBash(ctx, workDir, call.Input)
	case "read_file":
		return r.runReadFile(workDir, call.Input)
	case "write_file":
		return r.runWriteFile(workDir, call.Input)
	default:
		return &ToolResult{Error: fmt.Sprintf("unknown tool: %s", call.Name)}, nil
	}
}

func (r *BuiltinToolRunner) runBash(ctx context.Context, workDir, input string) (*ToolResult, error) {
	command := toolInputString(input, "command", "cmd")
	if command == "" {
		command = strings.TrimSpace(input)
	}
	if command == "" {
		return &ToolResult{Error: "bash: command is required"}, nil
	}

	if r.Policy != nil {
		if err := r.Policy.Validate(command); err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
	}

	if r.AuditEnabled {
		slog.Info("tool.bash.audit",
			"command", command,
			"work_dir", workDir,
		)
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultToolTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	cmd.Dir = workDir
	if r.Policy != nil && r.Policy.Enabled {
		cmd.Env = []string{
			"HOME=" + workDir,
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"PWD=" + workDir,
			"USER=devrix",
		}
	}
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := formatCommandOutput(stdout.String(), stderr.String(), r.maxOutput())
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return &ToolResult{Error: fmt.Sprintf("bash: timeout after %s", timeout), Output: out}, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := fmt.Sprintf("exit code %d", exitErr.ExitCode())
			if out != "" {
				return &ToolResult{Error: msg, Output: out}, nil
			}
			return &ToolResult{Error: msg}, nil
		}
		return &ToolResult{Error: err.Error(), Output: out}, nil
	}
	return &ToolResult{Output: out}, nil
}

func (r *BuiltinToolRunner) runReadFile(workDir, input string) (*ToolResult, error) {
	path := toolInputString(input, "path", "file")
	if path == "" {
		path = strings.TrimSpace(input)
	}
	target, err := resolveWorkspacePath(workDir, path)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	return &ToolResult{Output: truncateToolOutput(string(data), r.maxOutput())}, nil
}

func (r *BuiltinToolRunner) runWriteFile(workDir, input string) (*ToolResult, error) {
	fields := parseToolInput(input)
	path := firstNonEmpty(fields, "path", "file")
	content := fields["content"]
	if path == "" {
		return &ToolResult{Error: "write_file: path is required"}, nil
	}

	target, err := resolveWorkspacePath(workDir, path)
	if err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return &ToolResult{Error: err.Error()}, nil
	}
	return &ToolResult{Output: fmt.Sprintf("wrote %d bytes to %s", len(content), path)}, nil
}

func (r *BuiltinToolRunner) maxOutput() int {
	if r.MaxOutputBytes > 0 {
		return r.MaxOutputBytes
	}
	return defaultMaxToolOutput
}

func parseToolInput(input string) map[string]string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	if !strings.HasPrefix(input, "{") {
		return map[string]string{"command": input, "path": input}
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &generic); err != nil {
		return map[string]string{"command": input, "path": input}
	}

	out := make(map[string]string, len(generic))
	for k, raw := range generic {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			out[k] = s
			continue
		}
		out[k] = strings.TrimSpace(string(raw))
	}
	return out
}

func toolInputString(input string, keys ...string) string {
	fields := parseToolInput(input)
	return firstNonEmpty(fields, keys...)
}

func firstNonEmpty(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(fields[key]); v != "" {
			return v
		}
	}
	return ""
}

func resolveWorkspacePath(workDir, relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}

	workDir = filepath.Clean(workDir)
	target := relPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(workDir, target)
	}
	target = filepath.Clean(target)

	if workDir != "" {
		rel, err := filepath.Rel(workDir, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path escapes workspace: %s", relPath)
		}
	}
	return target, nil
}

func formatCommandOutput(stdout, stderr string, maxBytes int) string {
	var b strings.Builder
	if stdout != "" {
		b.WriteString(stdout)
	}
	if stderr != "" {
		if b.Len() > 0 && !strings.HasSuffix(stdout, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString(stderr)
	}
	return truncateToolOutput(strings.TrimRight(b.String(), "\n"), maxBytes)
}

func truncateToolOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...(output truncated)"
}
