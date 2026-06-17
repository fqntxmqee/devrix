package debugfilter

import (
	"io"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/logger"
)

// recordingHandler 记录所有 handle 调用,用于断言过滤行为。
type recordingHandler struct {
	mu      sync.Mutex
	entries []logger.LogEntry
}

func (h *recordingHandler) Handle(e *logger.LogEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e != nil {
		h.entries = append(h.entries, *e)
	}
	return nil
}

func (h *recordingHandler) SetLevel(_ logger.LogLevel) {}
func (h *recordingHandler) SetOutput(_ io.Writer)      {}

func (h *recordingHandler) snapshot() []logger.LogEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]logger.LogEntry, len(h.entries))
	copy(out, h.entries)
	return out
}

// TestFilter_NoCategories_PassesAll — 空 categories → 全部放行。
func TestFilter_NoCategories_PassesAll(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, nil)
	for _, comp := range []string{"api", "hooks", ""} {
		_ = f.Handle(&logger.LogEntry{Level: "DEBUG", Component: comp, Message: "x"})
	}
	if got := len(inner.snapshot()); got != 3 {
		t.Errorf("expected 3 entries passed, got %d", got)
	}
}

// TestFilter_OnlyEnabledComponent — enabled={api} → 只 api 放行。
func TestFilter_OnlyEnabledComponent(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, []string{"api"})
	for _, comp := range []string{"api", "hooks", "telemetry"} {
		_ = f.Handle(&logger.LogEntry{Level: "DEBUG", Component: comp, Message: "x"})
	}
	got := inner.snapshot()
	if len(got) != 1 || got[0].Component != "api" {
		t.Errorf("expected 1 'api' entry, got %+v", got)
	}
}

// TestFilter_NonDebugPassThrough — Info/Warn/Error 不受 category 限制。
func TestFilter_NonDebugPassThrough(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, []string{"api"})
	_ = f.Handle(&logger.LogEntry{Level: "INFO", Component: "hooks", Message: "x"})
	_ = f.Handle(&logger.LogEntry{Level: "WARN", Component: "telemetry", Message: "y"})
	_ = f.Handle(&logger.LogEntry{Level: "ERROR", Component: "", Message: "z"})
	got := inner.snapshot()
	if len(got) != 3 {
		t.Errorf("expected 3 non-debug entries passed, got %d", len(got))
	}
}

// TestFilter_NoComponentOnDebug_Blocked — debug level 但无 Component → 阻止。
func TestFilter_NoComponentOnDebug_Blocked(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, []string{"api"})
	_ = f.Handle(&logger.LogEntry{Level: "DEBUG", Component: "", Message: "orphan"})
	if got := len(inner.snapshot()); got != 0 {
		t.Errorf("expected 0 entries (orphan debug blocked), got %d", got)
	}
}

// TestFilter_Enabled_ReportsCorrectly — Enabled() 反映 categories 状态。
func TestFilter_Enabled_ReportsCorrectly(t *testing.T) {
	if (&Filter{}).Enabled() {
		t.Error("zero-value Filter should not be enabled")
	}
	if !New(nil, []string{"x"}).Enabled() {
		t.Error("non-empty categories should enable")
	}
	if New(nil, nil).Enabled() {
		t.Error("nil categories should not enable")
	}
}

// TestFilter_NilInnerSafe — nil inner 不会 panic。
func TestFilter_NilInnerSafe(t *testing.T) {
	f := New(nil, []string{"api"})
	if err := f.Handle(&logger.LogEntry{Level: "DEBUG", Component: "api"}); err != nil {
		t.Errorf("nil inner should not error, got %v", err)
	}
}

// TestFilter_NilEntrySafe — nil entry → no-op。
func TestFilter_NilEntrySafe(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, []string{"api"})
	if err := f.Handle(nil); err != nil {
		t.Errorf("nil entry should not error, got %v", err)
	}
	if got := len(inner.snapshot()); got != 0 {
		t.Errorf("nil entry should not record, got %d", got)
	}
}

// TestFilter_WithPassthroughNonDebug_Off — 关闭 passthrough → 严格白名单。
func TestFilter_WithPassthroughNonDebug_Off(t *testing.T) {
	inner := &recordingHandler{}
	f := New(inner, []string{"api"}).WithPassthroughNonDebug(false)
	_ = f.Handle(&logger.LogEntry{Level: "INFO", Component: "hooks"})
	_ = f.Handle(&logger.LogEntry{Level: "DEBUG", Component: "api"})
	got := inner.snapshot()
	if len(got) != 1 || got[0].Component != "api" {
		t.Errorf("expected only api DEBUG, got %+v", got)
	}
}

// TestCategories_ExportRoundTrip — Categories() 包含全部。
func TestCategories_ExportRoundTrip(t *testing.T) {
	f := New(nil, []string{"api", "hooks", "telemetry"})
	cats := f.Categories()
	if len(cats) != 3 {
		t.Errorf("expected 3 categories, got %d", len(cats))
	}
	want := map[string]bool{"api": true, "hooks": true, "telemetry": true}
	for _, c := range cats {
		if !want[c] {
			t.Errorf("unexpected category: %q", c)
		}
	}
}
