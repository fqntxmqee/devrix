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
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	defaultToolTimeout   = 60 * time.Second
	defaultMaxToolOutput = 64 * 1024
)

type toolExecConfig struct {
	timeout        time.Duration
	maxOutputBytes int
	policy         *CommandPolicy
	auditEnabled   bool
}

func newToolExecConfig(toolCfg *config.ToolConfig) *toolExecConfig {
	if toolCfg == nil {
		toolCfg = config.DefaultToolConfig()
	}
	enabled := toolCfg.SandboxEnabled()
	return &toolExecConfig{
		timeout:        defaultToolTimeout,
		maxOutputBytes: defaultMaxToolOutput,
		policy:         NewCommandPolicy(enabled, toolCfg.Sandbox.AllowlistExtra, toolCfg.Sandbox.DenyPatternsExtra),
		auditEnabled:   enabled,
	}
}

func (c *toolExecConfig) maxOutput() int {
	if c.maxOutputBytes > 0 {
		return c.maxOutputBytes
	}
	return defaultMaxToolOutput
}

// NewBuiltinToolRunner creates the default built-in tool registry as IToolRunner.
func NewBuiltinToolRunner() IToolRunner {
	return NewBuiltinToolRegistry(config.DefaultToolConfig())
}

// NewBuiltinToolRunnerFromConfig builds the built-in registry from tool config.
func NewBuiltinToolRunnerFromConfig(toolCfg *config.ToolConfig) IToolRunner {
	return NewBuiltinToolRegistry(toolCfg)
}

type bashRunner struct {
	cfg *toolExecConfig
}

func newBashRunner(cfg *toolExecConfig) *bashRunner {
	return &bashRunner{cfg: cfg}
}

func (r *bashRunner) Name() string { return "bash" }

func (r *bashRunner) Schema() ToolSchema {
	return ToolSchema{
		Name:        "bash",
		Description: "Execute a shell command (sandboxed)",
	}
}

func (r *bashRunner) RiskLevel() types.RiskLevel { return types.RiskLevelHigh }

func (r *bashRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	return runBash(ctx, workDir, input, r.cfg)
}

type readFileRunner struct {
	cfg *toolExecConfig
}

func newReadFileRunner(cfg *toolExecConfig) *readFileRunner {
	return &readFileRunner{cfg: cfg}
}

func (r *readFileRunner) Name() string { return "read_file" }

func (r *readFileRunner) Schema() ToolSchema {
	return ToolSchema{Name: "read_file", Description: "Read a file from the workspace"}
}

func (r *readFileRunner) RiskLevel() types.RiskLevel { return types.RiskLevelLow }

func (r *readFileRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	_ = ctx
	return runReadFile(workDir, input, r.cfg)
}

type writeFileRunner struct {
	cfg *toolExecConfig
}

func newWriteFileRunner(cfg *toolExecConfig) *writeFileRunner {
	return &writeFileRunner{cfg: cfg}
}

func (r *writeFileRunner) Name() string { return "write_file" }

func (r *writeFileRunner) Schema() ToolSchema {
	return ToolSchema{Name: "write_file", Description: "Write content to a file"}
}

func (r *writeFileRunner) RiskLevel() types.RiskLevel { return types.RiskLevelMedium }

func (r *writeFileRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	_ = ctx
	return runWriteFile(workDir, input, r.cfg)
}

func runBash(ctx context.Context, workDir, input string, cfg *toolExecConfig) (*ToolResult, error) {
	command := toolInputString(input, "command", "cmd")
	if command == "" {
		command = strings.TrimSpace(input)
	}
	if command == "" {
		return &ToolResult{Error: "bash: command is required"}, nil
	}

	if cfg.policy != nil {
		if err := cfg.policy.Validate(command); err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
	}

	if cfg.auditEnabled {
		slog.Info("tool.bash.audit", "command", command, "work_dir", workDir)
	}

	timeout := cfg.timeout
	if timeout <= 0 {
		timeout = defaultToolTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	cmd.Dir = workDir
	if cfg.policy != nil && cfg.policy.Enabled {
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
	out := formatCommandOutput(stdout.String(), stderr.String(), cfg.maxOutput())
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

func runReadFile(workDir, input string, cfg *toolExecConfig) (*ToolResult, error) {
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
	return &ToolResult{Output: truncateToolOutput(string(data), cfg.maxOutput())}, nil
}

func runWriteFile(workDir, input string, cfg *toolExecConfig) (*ToolResult, error) {
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
