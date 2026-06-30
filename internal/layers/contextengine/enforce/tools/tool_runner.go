package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/sandboxast"
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
	policy := NewCommandPolicy(enabled, toolCfg.Sandbox.AllowlistExtra, toolCfg.Sandbox.DenyPatternsExtra)
	// DM-20260617-002 W4 (AC10): 注入 G2 Bash AST analyzer，仅当 ASTEnabled=true。
	if enabled && toolCfg.ASTEnabled() {
		policy.ASTAnalyzer = sandboxast.NewPolicyAnalyzer()
	}
	return &toolExecConfig{
		timeout:        defaultToolTimeout,
		maxOutputBytes: defaultMaxToolOutput,
		policy:         policy,
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
func NewBuiltinToolRunner() (IToolRunner, error) {
	return NewBuiltinToolRegistry(config.DefaultToolConfig())
}

// NewBuiltinToolRunnerFromConfig builds the built-in registry from tool config.
func NewBuiltinToolRunnerFromConfig(toolCfg *config.ToolConfig) (IToolRunner, error) {
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
		Description: "Execute a shell command in the session WorkDir (sandboxed). Use relative paths; prefer read_file/glob/list_dir for file reads.",
		// Tool-arg schema for Bash (2026-06-20): the previous implementation
		// omitted Parameters, which made the LLM emit `arguments: "{}"` for
		// every bash call. The tool then rejected with "invalid command —
		// pass a shell command string" and the LLM re-issued the same empty
		// call, producing the 4-call loop visible in
		// sess_1781908264924_6000.json. With Parameters declared, the model
		// knows to send {"command": "..."} and bashWrongToolHint stops
		// firing on the first turn.
		Parameters: `{"type":"object","required":["command"],"properties":{"command":{"type":"string","description":"Shell command to execute. Prefer relative paths; use read_file/glob/grep for file reads."}}}`,
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
	return ToolSchema{
		Name:        "read_file",
		Description: "Read a file from the workspace. Args: {\"path\":\"relative/or/abs/path\"}",
		Parameters:  `{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"file_path":{"type":"string"}}}`,
	}
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
	return ToolSchema{
		Name:        "write_file",
		Description: "Write content to a file in the workspace. Args: {\"path\":\"...\",\"content\":\"...\"}",
		Parameters:  `{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"file_path":{"type":"string"},"content":{"type":"string"}}}`,
	}
}

func (r *writeFileRunner) RiskLevel() types.RiskLevel { return types.RiskLevelMedium }

func (r *writeFileRunner) Execute(ctx context.Context, workDir, input string) (*ToolResult, error) {
	fields := ParseToolInput(input)
	path := firstNonEmpty(fields, "path", "file", "file_path")
	if path != "" {
		target := path
		if workDir != "" && !filepath.IsAbs(path) {
			target = filepath.Join(workDir, path)
		}
		if denied := EnforcePlanModeWrite(ctx, target); denied != nil {
			return denied, nil
		}
	}
	return runWriteFile(workDir, input, r.cfg)
}

func runBash(ctx context.Context, workDir, input string, cfg *toolExecConfig) (*ToolResult, error) {
	command := ToolInputString(input, "command", "cmd")
	if command == "" {
		command = strings.TrimSpace(input)
	}
	if command == "" {
		return &ToolResult{Error: "bash: command is required"}, nil
	}
	if hint := bashWrongToolHint(input, command); hint != "" {
		return &ToolResult{Error: hint}, nil
	}

	if workDir != "" {
		command = NormalizeWorkspacePaths(workDir, command)
	}

	if denied := EnforcePlanModeBash(ctx); denied != nil {
		return denied, nil
	}

	if cfg.policy != nil {
		if err := cfg.policy.Validate(command); err != nil {
			return &ToolResult{Error: err.Error()}, nil
		}
	}

	if cfg.auditEnabled {
		slog.Info("tool.bash.audit", "command", redactCommandForAudit(command), "work_dir", workDir)
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
	path := ToolInputString(input, "path", "file", "file_path")
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
	fields := ParseToolInput(input)
	path := firstNonEmpty(fields, "path", "file", "file_path")
	content := fields["content"]
	if path == "" {
		return &ToolResult{Error: "write_file: path is required (use \"path\" or \"file_path\")"}, nil
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
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes workspace: %s", relPath)
		}
		// RH-D2-02 (DM-20260630-013): symlink containment. A symlink inside
		// the workspace pointing outside it would otherwise let a malicious
		// or stale link escape. Reject when the resolved realpath falls
		// outside the (also-resolved) workDir.
		if escaped, evalErr := pathEscapesViaSymlink(workDir, target); evalErr != nil {
			return "", fmt.Errorf("path containment check failed: %w", evalErr)
		} else if escaped {
			return "", fmt.Errorf("path escapes workspace via symlink: %s", relPath)
		}
	}
	return target, nil
}

// pathEscapesViaSymlink returns true when target's resolved realpath falls
// outside workDir's realpath. For paths that do not exist yet, it resolves the
// closest existing ancestor first; this blocks writes like
// "workspace/link_to_outside/new.txt", where the final file is missing but an
// intermediate symlink directory escapes the workspace.
func pathEscapesViaSymlink(workDir, target string) (bool, error) {
	realWork, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return false, err
	}
	existing := target
	for {
		realTarget, err := filepath.EvalSymlinks(existing)
		if err == nil {
			rel, relErr := filepath.Rel(realWork, realTarget)
			if relErr != nil {
				return true, nil
			}
			return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
		}
		if !os.IsNotExist(err) {
			return false, err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return false, err
		}
		existing = parent
	}
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

func redactCommandForAudit(command string) string {
	redacted := command
	patterns := []string{
		`(?i)(authorization:\s*bearer\s+)[^\s"'\\]+`,
		`(?i)(api[_-]?key\s*=\s*)[^\s"'\\]+`,
		`(?i)(token\s*=\s*)[^\s"'\\]+`,
		`(?i)(password\s*=\s*)[^\s"'\\]+`,
		`(?i)(cookie:\s*)[^\n]+`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		redacted = re.ReplaceAllString(redacted, `${1}<redacted>`)
	}
	return redacted
}

// bashWrongToolHint detects when the model invoked bash with another tool's JSON args.
func bashWrongToolHint(rawInput, command string) string {
	if !strings.HasPrefix(strings.TrimSpace(rawInput), "{") {
		return ""
	}
	if ToolInputString(rawInput, "command", "cmd") != "" {
		return ""
	}
	fields := ParseToolInput(rawInput)
	switch {
	case fields["pattern"] != "":
		return "bash: use the glob tool for file pattern search, not bash"
	case fields["path"] != "" || fields["file_path"] != "":
		return "bash: use read_file or glob for file access, not bash"
	case strings.HasPrefix(command, "{"):
		return "bash: invalid command — pass a shell command string, or use read_file/glob/grep tools"
	default:
		return ""
	}
}
