package contextengine

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

)

// VerifyCommand describes a whitelisted verify command.
type VerifyCommand struct {
	Name       string
	Executable string
	Args       []string
	Timeout    time.Duration
	WorkDir    string
}

// VerifyCommandResult holds command execution output.
type VerifyCommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// IVerifyCommandRunner executes verify commands safely.
type IVerifyCommandRunner interface {
	Run(ctx context.Context, cmd VerifyCommand) (VerifyCommandResult, error)
}

// BuiltinVerifyRunner runs commands via exec.CommandContext without shell.
type BuiltinVerifyRunner struct {
	TrustedWorkDir string
}

// NewBuiltinVerifyRunner creates a runner bound to a trusted work directory.
func NewBuiltinVerifyRunner(trustedWorkDir string) *BuiltinVerifyRunner {
	return &BuiltinVerifyRunner{TrustedWorkDir: filepath.Clean(trustedWorkDir)}
}

// Run executes a verify command.
func (r *BuiltinVerifyRunner) Run(ctx context.Context, cmd VerifyCommand) (VerifyCommandResult, error) {
	if err := validateVerifyCommand(cmd); err != nil {
		return VerifyCommandResult{}, err
	}
	workDir := filepath.Clean(cmd.WorkDir)
	if r.TrustedWorkDir != "" && workDir != r.TrustedWorkDir {
		return VerifyCommandResult{}, fmt.Errorf("workdir mismatch: got %q want %q", workDir, r.TrustedWorkDir)
	}
	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	c := exec.CommandContext(runCtx, cmd.Executable, cmd.Args...)
	c.Dir = workDir
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	result := VerifyCommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, err
}

func validateVerifyCommand(cmd VerifyCommand) error {
	if err := validateNoShellMeta(cmd.Executable); err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	for i, arg := range cmd.Args {
		if err := validateNoShellMeta(arg); err != nil {
			return fmt.Errorf("args[%d]: %w", i, err)
		}
	}
	return nil
}

func validateNoShellMeta(s string) error {
	if strings.ContainsAny(s, ";|&$`") {
		return fmt.Errorf("shell metacharacters not allowed")
	}
	return nil
}
