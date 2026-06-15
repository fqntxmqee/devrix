package runtime

import (
	"sync"
	"testing"
)

// D2-S9-A01-T03: 旧路径调用计数基线 = 0；QueryLoop 路径应被记录，
// 且不变量：Snapshot 内部一致性。
func TestPathResolver_Record_AndSnapshot(t *testing.T) {
	Reset()
	if s := Snapshot(); s.QueryLoop != 0 || s.LegacyHarness != 0 {
		t.Fatalf("after Reset, want zeros, got %+v", s)
	}
	Record(PathQueryLoop)
	Record(PathQueryLoop)
	Record(PathLegacyHarness)

	s := Snapshot()
	if s.QueryLoop != 2 {
		t.Errorf("QueryLoop = %d, want 2", s.QueryLoop)
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
				Record(PathQueryLoop)
			}
		}()
	}
	wg.Wait()
	if got := Snapshot().QueryLoop; got != int64(goroutines*perGoroutine) {
		t.Errorf("QueryLoop = %d, want %d", got, goroutines*perGoroutine)
	}
}

// 未知 path 类型不抛 panic。
func TestPathResolver_UnknownKind_Noop(t *testing.T) {
	Reset()
	Record(PathKind("totally_unknown_path"))
	if s := Snapshot(); s.QueryLoop != 0 || s.LegacyHarness != 0 {
		t.Errorf("unknown path leaked into counters: %+v", s)
	}
}

// Reset() 行为。
func TestPathResolver_Reset(t *testing.T) {
	Reset()
	Record(PathQueryLoop)
	Record(PathLegacyHarness)
	Reset()
	if s := Snapshot(); s.QueryLoop != 0 || s.LegacyHarness != 0 {
		t.Errorf("after Reset, want zeros, got %+v", s)
	}
}
