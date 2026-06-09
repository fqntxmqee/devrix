package contextengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func isSingleRoundVerifyMode(mode string) bool {
	mode = strings.TrimSpace(mode)
	return mode == "" || mode == config.VerifyModeBasic || mode == config.VerifyModeNone
}

func (e *PEVEngine) verify(ctx context.Context, sc *types.SessionContext, toolResults []ToolResult) types.VerifyResult {
	switch strings.TrimSpace(e.cfg.VerifyMode) {
	case config.VerifyModeNone:
		return types.VerifyResult{Passed: true}
	case config.VerifyModeCommands:
		if !verifyBasic(toolResults).Passed {
			return verifyBasic(toolResults)
		}
		return e.verifyCommands(ctx, sc, toolResults)
	case config.VerifyModeBasic, "":
		return verifyBasic(toolResults)
	default:
		return verifyBasic(toolResults)
	}
}

func verifyBasic(results []ToolResult) types.VerifyResult {
	if len(results) == 0 {
		return types.VerifyResult{Passed: true}
	}
	for _, r := range results {
		if r.Error == "" && strings.TrimSpace(r.Output) != "" {
			return types.VerifyResult{Passed: true}
		}
	}
	return types.VerifyResult{Passed: false, Deviation: 1}
}

func (e *PEVEngine) verifyCommands(ctx context.Context, sc *types.SessionContext, toolResults []ToolResult) types.VerifyResult {
	if e.verifyRunner == nil || len(e.cfg.VerifyCommands) == 0 {
		return verifyBasic(toolResults)
	}
	policy := e.cfg.VerifyPolicy
	if policy == "" {
		policy = config.VerifyPolicyAllPass
	}

	passedCount := 0
	failCount := 0
	for _, cfgCmd := range e.cfg.VerifyCommands {
		cmd := VerifyCommand{
			Name:       cfgCmd.Name,
			Executable: cfgCmd.Executable,
			Args:       append([]string(nil), cfgCmd.Args...),
			Timeout:    cfgCmd.Timeout,
			WorkDir:    sc.WorkDir,
		}
		_, verifySpan := e.startSpan(ctx, telemetry.OpContextVerifyCommand, tracer.SpanKindInternal,
			tracer.Attribute{Key: "command.name", Value: cmd.Name},
			tracer.Attribute{Key: "work_dir", Value: cmd.WorkDir},
		)
		result, err := e.verifyRunner.Run(ctx, cmd)
		if verifySpan != nil {
			verifySpan.SetAttributes(
				tracer.Attribute{Key: "verify.exit_code", Value: fmt.Sprintf("%d", result.ExitCode)},
				tracer.Attribute{Key: "verify.duration_ms", Value: fmt.Sprintf("%d", result.Duration.Milliseconds())},
			)
			if err != nil {
				verifySpan.RecordError(err)
			}
			verifySpan.End()
		}
		if e.pevObserver != nil {
			e.pevObserver.EmitVerifyCommand(sc.SessionID, cmd.Name, result)
		}
		if err != nil {
			failCount++
			continue
		}
		if result.ExitCode == 0 {
			passedCount++
		} else {
			failCount++
			e.recordVerifyFailure(cmd.Name)
		}
	}

	switch policy {
	case config.VerifyPolicyAnyPass:
		if passedCount > 0 {
			return types.VerifyResult{Passed: true}
		}
	case config.VerifyPolicyAllPass:
		if failCount == 0 && passedCount > 0 {
			return types.VerifyResult{Passed: true}
		}
	}
	deviation := float64(failCount) / float64(max(1, len(e.cfg.VerifyCommands)))
	return types.VerifyResult{Passed: false, Deviation: deviation}
}

func (e *PEVEngine) recordVerifyFailure(command string) {
	if e.obsBridge == nil || e.obsBridge.Meter() == nil {
		return
	}
	c, _ := e.obsBridge.Meter().Int64Counter("devrix_ctx_verify_command_failures_total")
	if c != nil {
		c.Add(1)
	}
	_ = command
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
