package bootstrap

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine"
	"github.com/devrix/devrix/internal/layers/llmgateway"
)

// timedTool is a context-aware IToolRunner mock that blocks for `delay` or
// until ctx is canceled, whichever comes first. Mirrors the real runner's
// ctx-respecting behavior so the test exercises the timeout fail-closed path
// instead of accidentally racing a goroutine.
type timedTool struct {
	delay time.Duration
}

func (s *timedTool) Execute(ctx context.Context, _ contextengine.ToolCall) (*contextengine.ToolResult, error) {
	select {
	case <-time.After(s.delay):
		return &contextengine.ToolResult{Output: "ok"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func newTimeoutTestAdapter(tool contextengine.IToolRunner) *contextEngineAdapter {
	// Only `tools` is needed for the executeOne timeout path. a.perm is nil
	// so the gate stays open. a.surfaces is nil so the surface branch is
	// skipped and we go straight to a.tools.Execute.
	return &contextEngineAdapter{tools: tool}
}

func withToolTimeoutEnv(t *testing.T, seconds int) {
	t.Helper()
	if err := os.Setenv("DEVRIX_TOOL_TIMEOUT_SECONDS", itoa(seconds)); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVRIX_TOOL_TIMEOUT_SECONDS") })
}

func itoa(n int) string {
	// avoid importing strconv for a single test helper
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// DM-20260704-003 / D7-S2-A50-T09 (case 1): a tool that finishes well within
// the timeout must return success and not be aborted.
func TestExecuteOne_Timeout_FastTool(t *testing.T) {
	withToolTimeoutEnv(t, 5)
	a := newTimeoutTestAdapter(&timedTool{delay: 50 * time.Millisecond})

	start := time.Now()
	res := a.executeOne(context.Background(), "s1", llmgateway.ToolCall{ID: "tc1", Name: "fast"})
	elapsed := time.Since(start)

	if res.Error != "" {
		t.Fatalf("expected success, got error: %s", res.Error)
	}
	if res.Output != "ok" {
		t.Errorf("Output = %q, want %q", res.Output, "ok")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("fast tool elapsed %v; expected < 500ms", elapsed)
	}
}

// DM-20260704-003 / D7-S2-A50-T09+T10 (case 2): a tool that exceeds the
// timeout must be fail-closed — return an "timeout" error within ~timeout
// duration (not the tool's natural delay).
func TestExecuteOne_Timeout_SlowToolReturnsErr(t *testing.T) {
	withToolTimeoutEnv(t, 1)
	a := newTimeoutTestAdapter(&timedTool{delay: 3 * time.Second})

	start := time.Now()
	res := a.executeOne(context.Background(), "s1", llmgateway.ToolCall{ID: "tc2", Name: "slow"})
	elapsed := time.Since(start)

	if !strings.Contains(res.Error, "timeout") {
		t.Fatalf("expected timeout error, got: %q", res.Error)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("timeout elapsed %v; expected ~1s (fail-closed)", elapsed)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("timeout elapsed %v; expected >= ~0.9s (timeout 1s)", elapsed)
	}
	if res.Output != "" {
		t.Errorf("Output = %q, want empty on timeout", res.Output)
	}
}

// DM-20260704-003 / D7-S2-A50-T09 (case 3): env override must take effect
// even if a future config field defaults to 0. This guards against
// regressions where loadToolTimeoutSeconds() silently falls back to 60.
func TestExecuteOne_Timeout_EnvOverrideShorterThanDefault(t *testing.T) {
	withToolTimeoutEnv(t, 1) // shorter than default 60
	a := newTimeoutTestAdapter(&timedTool{delay: 3 * time.Second})

	start := time.Now()
	res := a.executeOne(context.Background(), "s1", llmgateway.ToolCall{ID: "tc3", Name: "slow"})
	elapsed := time.Since(start)

	if !strings.Contains(res.Error, "timeout") {
		t.Fatalf("env=1s should fail-closed slow tool, got: %q", res.Error)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("env=1s elapsed %v; expected ~1s", elapsed)
	}
}

// DM-20260704-003 / D7-S2-A50-T09 (case 4): a malformed/empty env value
// must fall back to the 60s default rather than crashing or using 0
// (which would mean every tool hangs forever).
func TestExecuteOne_Timeout_InvalidEnvFallsBackToDefault(t *testing.T) {
	if err := os.Setenv("DEVRIX_TOOL_TIMEOUT_SECONDS", "not-a-number"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVRIX_TOOL_TIMEOUT_SECONDS") })

	got := loadToolTimeoutSeconds()
	want := time.Duration(defaultToolTimeoutSeconds) * time.Second
	if got != want {
		t.Errorf("loadToolTimeoutSeconds() with invalid env = %v, want %v (default 60s)", got, want)
	}
}

// DM-20260704-003 / D7-S2-A50-T09 (case 5): zero/negative env must fall
// back to default (defends against DEVRIX_TOOL_TIMEOUT_SECONDS=0 disabling
// the fail-closed guard).
func TestExecuteOne_Timeout_ZeroEnvFallsBackToDefault(t *testing.T) {
	if err := os.Setenv("DEVRIX_TOOL_TIMEOUT_SECONDS", "0"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("DEVRIX_TOOL_TIMEOUT_SECONDS") })

	got := loadToolTimeoutSeconds()
	want := time.Duration(defaultToolTimeoutSeconds) * time.Second
	if got != want {
		t.Errorf("loadToolTimeoutSeconds() with env=0 = %v, want %v (default 60s)", got, want)
	}
}
