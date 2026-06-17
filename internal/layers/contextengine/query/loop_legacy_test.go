package query_test

// T: D7-S2-A06-T10 — LoopFirst=true keeps D2.QueryLoop.Run off the hot path.
// The counter is the operational signal that loopFirst routing actually
// avoids the legacy D2 entry point. When a unit test exercises
// query.Loop.Run directly (loopFirst=false simulation) the counter must
// move; production wiring keeps it at zero.
import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: D7-S2-A06-T10
// Scenario: loopFirst=false simulation — every direct Run() invocation
// bumps d2_query_loop_legacy_invocations_total.
func TestLoopRun_legacy_counter_bumps_on_every_invocation(t *testing.T) {
	llm := &query.SequentialLLM{Responses: []query.LLMScript{
		{Content: "ok"},
	}}
	counter := metrics.NewCounter("d2_query_loop_legacy_invocations_total", nil)

	loop := &query.Loop{
		LLM:           llm,
		Tools:         &query.RecordingToolExecutor{},
		Permission:    query.AllowPermission{},
		LegacyCounter: counter,
	}

	for i := 0; i < 3; i++ {
		sc := &types.SessionContext{SessionID: "s" + itoa(i), Model: "test"}
		if _, err := loop.Run(context.Background(), sc, query.Params{
			SystemPrompt: "sys",
			Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "go"}},
			MaxTurns:     1,
		}, nil); err != nil {
			t.Fatalf("Run #%d: %v", i, err)
		}
	}

	if v := counter.Value(); v != 3 {
		t.Errorf("legacy counter = %d, want 3 (one per Run())", v)
	}
}

// Covers: D7-S2-A06-T10
// Scenario: LegacyCounter nil → Run() must not panic, must keep working.
func TestLoopRun_legacy_counter_nil_safe(t *testing.T) {
	llm := &query.SequentialLLM{Responses: []query.LLMScript{{Content: "ok"}}}
	loop := &query.Loop{
		LLM:        llm,
		Tools:      &query.RecordingToolExecutor{},
		Permission: query.AllowPermission{},
		// LegacyCounter intentionally nil
	}
	sc := &types.SessionContext{SessionID: "s", Model: "test"}
	if _, err := loop.Run(context.Background(), sc, query.Params{
		SystemPrompt: "sys",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "go"}},
		MaxTurns:     1,
	}, nil); err != nil {
		t.Fatalf("Run with nil counter should not error: %v", err)
	}
}

// itoa is a minimal integer-to-string helper to avoid pulling strconv
// into a 30-line test file. Tests only need a few session IDs.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := "0123456789"
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	return string(buf[pos:])
}
