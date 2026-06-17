package bootstrap

// W5 — D5-S24-A02 (alias A2) DebugFilter 通过 CLI flag 启动时过滤日志 wiring 测试。
//
// AC12:
//   - categories 非空 → filter 启用
//   - categories 为空 → 跳过 (no-op)
//   - DEBUG + 匹配 component → 通过
//   - DEBUG + 不匹配 component → 过滤
//   - 非 DEBUG + 任意 component → 通过 (passthroughNonDebug)

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/observability/instrument/logger/debugfilter"
)

func TestParseDebugFlag_Empty(t *testing.T) {
	got := ParseDebugFlag([]string{"foo", "bar"})
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseDebugFlag_EmptyValue(t *testing.T) {
	got := ParseDebugFlag([]string{"--debug="})
	if got != nil {
		t.Errorf("expected nil for empty value, got %v", got)
	}
}

func TestParseDebugFlag_Single(t *testing.T) {
	got := ParseDebugFlag([]string{"--debug=api"})
	if len(got) != 1 || got[0] != "api" {
		t.Errorf("got %v, want [api]", got)
	}
}

func TestParseDebugFlag_Multiple(t *testing.T) {
	got := ParseDebugFlag([]string{"--debug=api,hooks,telemetry"})
	want := []string{"api", "hooks", "telemetry"}
	if len(got) != 3 {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("[%d] got %q, want %q", i, got[i], w)
		}
	}
}

func TestParseDebugFlag_TrimsWhitespace(t *testing.T) {
	got := ParseDebugFlag([]string{"--debug= api , hooks "})
	want := []string{"api", "hooks"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestInstallDebugFilter_NoCategories_NoOp(t *testing.T) {
	// categories 为空 → 跳过
	prev := slog.Default()
	prevHandler := prev.Handler()

	InstallDebugFilter(nil)

	after := slog.Default()
	if after.Handler() != prevHandler {
		t.Errorf("handler should not change when categories empty")
	}
}

// TestSlogFilterAdapter_AllowedComponent 验证 DEBUG + 匹配 component → 写入 buffer。
func TestSlogFilterAdapter_AllowedComponent(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     map[string]bool{"api": true},
		passthru: true,
	}
	logger := slog.New(adapter)
	logger.Debug("test", "component", "api")
	if !strings.Contains(buf.String(), "test") {
		t.Errorf("expected 'test' in output, got %q", buf.String())
	}
}

// TestSlogFilterAdapter_FilteredComponent 验证 DEBUG + 不匹配 component → 丢弃。
func TestSlogFilterAdapter_FilteredComponent(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     map[string]bool{"api": true},
		passthru: true,
	}
	logger := slog.New(adapter)
	logger.Debug("test", "component", "telemetry")
	if strings.Contains(buf.String(), "test") {
		t.Errorf("expected 'test' to be filtered out, got %q", buf.String())
	}
}

// TestSlogFilterAdapter_PassthroughNonDebug 验证非 DEBUG 级别不受 filter 影响。
func TestSlogFilterAdapter_PassthroughNonDebug(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     map[string]bool{"api": true},
		passthru: true,
	}
	logger := slog.New(adapter)
	logger.Info("info msg", "component", "telemetry")
	if !strings.Contains(buf.String(), "info msg") {
		t.Errorf("expected info passthrough, got %q", buf.String())
	}
	logger.Warn("warn msg", "component", "telemetry")
	if !strings.Contains(buf.String(), "warn msg") {
		t.Errorf("expected warn passthrough, got %q", buf.String())
	}
}

// TestSlogFilterAdapter_NoComponentAttr 验证 DEBUG 但无 component attr → 丢弃。
func TestSlogFilterAdapter_NoComponentAttr(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     map[string]bool{"api": true},
		passthru: true,
	}
	logger := slog.New(adapter)
	logger.Debug("debug no component")
	if strings.Contains(buf.String(), "debug no component") {
		t.Errorf("expected debug w/o component to be filtered, got %q", buf.String())
	}
}

// TestSlogLevelConversion 验证 Debug 级别走 filter、Info/Warn 走 passthrough。
func TestSlogLevelConversion(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	adapter := &slogFilterAdapter{
		inner:    inner,
		cats:     map[string]bool{"api": true},
		passthru: true,
	}
	logger := slog.New(adapter)
	// DEBUG + 不匹配 component → 过滤
	logger.Debug("debug msg", "component", "telemetry")
	if strings.Contains(buf.String(), "debug msg") {
		t.Errorf("expected debug filtered, got %q", buf.String())
	}
	buf.Reset()
	// Info + 不匹配 component → passthrough
	logger.Info("info msg", "component", "telemetry")
	if !strings.Contains(buf.String(), "info msg") {
		t.Errorf("expected info passthrough, got %q", buf.String())
	}
}

// TestCategoriesOf 验证从 Filter 提取 categories 集合。
func TestCategoriesOf(t *testing.T) {
	filter := debugfilter.New(noopHandler{}, []string{"api", "hooks"})
	cats := categoriesOf(filter)
	if len(cats) != 2 {
		t.Fatalf("got %d categories, want 2", len(cats))
	}
	if !cats["api"] || !cats["hooks"] {
		t.Errorf("missing categories: got %v", cats)
	}
}
