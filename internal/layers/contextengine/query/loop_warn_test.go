package query_test

// T: D5-S24-A02-T05 — legacy path logs a deprecation warning exactly
// once per process. The warnLegacyOnce sync.Once must guarantee
// that the first Run() emits the slog.Warn line and that all
// subsequent Run() calls stay silent (the deprecation message is
// not chatty under load).
import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/shared/types"
)

// recordingHandler is a minimal slog.Handler that buffers the JSON
// payload of every record it receives. Tests assert on the
// buffered slice after the production code has run.
type recordingHandler struct {
	mu      sync.Mutex
	records []map[string]any
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		rec[a.Key] = a.Value.Any()
		return true
	})
	h.records = append(h.records, rec)
	return nil
}
func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *recordingHandler) countDeprecation() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.records {
		if msg, _ := r["msg"].(string); strings.Contains(msg, "D2.QueryLoop.Run is DEPRECATED") {
			n++
		}
	}
	return n
}

// Covers: D5-S24-A02-T05
// Scenario: legacy path is invoked 5 times → exactly 1 deprecation
// log line is emitted, regardless of how many subsequent Run() calls
// happen on the same Loop instance.
func TestLoopRun_warnLegacyOnce_emits_exactly_one_warning(t *testing.T) {
	handler := &recordingHandler{}
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	llm := &query.SequentialLLM{Responses: []query.LLMScript{
		{Content: "ok"},
	}}
	loop := &query.Loop{
		LLM:        llm,
		Tools:      &query.RecordingToolExecutor{},
		Permission: query.AllowPermission{},
	}

	for i := 0; i < 5; i++ {
		sc := &types.SessionContext{SessionID: itoa(i), Model: "test"}
		if _, err := loop.Run(context.Background(), sc, query.Params{
			SystemPrompt: "sys",
			Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "go"}},
			MaxTurns:     1,
		}, nil); err != nil {
			t.Fatalf("Run #%d: %v", i, err)
		}
	}

	if n := handler.countDeprecation(); n != 1 {
		t.Errorf("deprecation warnings = %d, want 1 (sync.Once must dedupe)", n)
	}
}

// Covers: D5-S24-A02-T05
// Scenario: an unrelated log line is left untouched; only the
// deprecation message gets emitted from Run().
func TestLoopRun_warnLegacyOnce_payload_is_well_formed(t *testing.T) {
	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	llm := &query.SequentialLLM{Responses: []query.LLMScript{{Content: "ok"}}}
	loop := &query.Loop{LLM: llm, Tools: &query.RecordingToolExecutor{}, Permission: query.AllowPermission{}}
	sc := &types.SessionContext{SessionID: "s1", Model: "test"}
	if _, err := loop.Run(context.Background(), sc, query.Params{
		SystemPrompt: "sys",
		Messages:     []types.Message{{Role: types.MessageRoleUser, Content: "go"}},
		MaxTurns:     1,
	}, nil); err != nil {
		t.Fatal(err)
	}

	line := buf.String()
	if !strings.Contains(line, "D2.QueryLoop.Run is DEPRECATED") {
		t.Fatalf("log line missing deprecation message: %q", line)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &rec); err != nil {
		t.Fatalf("log line is not valid JSON: %v (%q)", err, line)
	}
	if rec["dm"] != "DM-20260617-001" {
		t.Errorf("dm = %v, want DM-20260617-001", rec["dm"])
	}
	if rec["change"] != "devrix-queryloop-legacy-decommission" {
		t.Errorf("change = %v", rec["change"])
	}
	if rec["canonical_path"] != "D7-S2-A06 RunTurnLoop" {
		t.Errorf("canonical_path = %v", rec["canonical_path"])
	}
}
