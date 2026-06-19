package runtime

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// D2-S9-A01-T03: 旧路径调用计数基线 = 0；QueryLoop 路径应被记录，
// 且不变量：Snapshot 内部一致性。
func TestPathResolver_Record_AndSnapshot(t *testing.T) {
	Reset()
	if s := Snapshot(); s.D7Turn != 0 || s.LegacyHarness != 0 {
		t.Fatalf("after Reset, want zeros, got %+v", s)
	}
	Record(PathD7Turn)
	Record(PathD7Turn)
	Record(PathLegacyHarness)

	s := Snapshot()
	if s.D7Turn != 2 {
		t.Errorf("D7Turn = %d, want 2", s.D7Turn)
	}
	if s.LegacyHarness != 1 {
		t.Errorf("LegacyHarness = %d, want 1", s.LegacyHarness)
	}
}

// D2-S9-A01-T03: 并发 Record 不丢更新。
func TestPathResolver_ConcurrentRecord(t *testing.T) {
	Reset()
	const goroutines = 16
	const perGoroutine = 500
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				Record(PathD7Turn)
			}
		}()
	}
	wg.Wait()
	if got := Snapshot().D7Turn; got != int64(goroutines*perGoroutine) {
		t.Errorf("D7Turn = %d, want %d", got, goroutines*perGoroutine)
	}
}

// 未知 path 类型不抛 panic。
func TestPathResolver_UnknownKind_Noop(t *testing.T) {
	Reset()
	Record(PathKind("totally_unknown_path"))
	if s := Snapshot(); s.D7Turn != 0 || s.LegacyHarness != 0 {
		t.Errorf("unknown path leaked into counters: %+v", s)
	}
}

// Reset() 行为。
func TestPathResolver_Reset(t *testing.T) {
	Reset()
	Record(PathD7Turn)
	Record(PathLegacyHarness)
	Reset()
	if s := Snapshot(); s.D7Turn != 0 || s.LegacyHarness != 0 {
		t.Errorf("after Reset, want zeros, got %+v", s)
	}
}


func captureSlogResolver(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	return &buf, func() { slog.SetDefault(prev) }
}

// D5 v2.1 Terminal (DM-20260619-006): Record on PathLegacyHarness must emit a
// DEPRECATED slog.Warn so on-call can spot live stragglers.
func TestRecord_LegacyHarness_LogsDeprecation(t *testing.T) {
	buf, restore := captureSlogResolver(t)
	defer restore()
	Reset()
	Record(PathLegacyHarness)
	if !strings.Contains(buf.String(), "DEPRECATED") {
		t.Fatalf("expected DEPRECATED warning, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "legacy_harness") {
		t.Fatalf("expected legacy_harness in warning, got: %s", buf.String())
	}
}

// Record on PathD7Turn must NOT emit the deprecation warning.
func TestRecord_D7Turn_NoDeprecationLog(t *testing.T) {
	buf, restore := captureSlogResolver(t)
	defer restore()
	Reset()
	Record(PathD7Turn)
	if strings.Contains(buf.String(), "DEPRECATED") {
		t.Fatalf("PathD7Turn must not log DEPRECATED, got: %s", buf.String())
	}
}
