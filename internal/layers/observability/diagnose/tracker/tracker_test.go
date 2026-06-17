package tracker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeLinter 计数器 linter，模拟 linter 调用。
type fakeLinter struct {
	mu    sync.Mutex
	calls map[string]int
	data  map[string][]Diagnostic
}

func newFakeLinter() *fakeLinter {
	return &fakeLinter{
		calls: make(map[string]int),
		data:  make(map[string][]Diagnostic),
	}
}

func (f *fakeLinter) lint(ctx context.Context, file string) ([]Diagnostic, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[file]++
	out := make([]Diagnostic, len(f.data[file]))
	copy(out, f.data[file])
	return out, nil
}

func (f *fakeLinter) set(file string, diags []Diagnostic) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[file] = diags
}

func (f *fakeLinter) callCount(file string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[file]
}

func diag(file string, line int, severity, msg string) Diagnostic {
	return Diagnostic{File: file, Line: line, Severity: severity, Message: msg, Source: "fake"}
}

// TestSnapshotBefore_StoresBaseline — SnapshotBefore 存基线，Diff 后 baseline 被更新。
func TestSnapshotBefore_StoresBaseline(t *testing.T) {
	tr := New(0)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	fl.set("a.fake", []Diagnostic{diag("a.fake", 1, "error", "old")})

	if err := tr.SnapshotBefore(context.Background(), "a.fake"); err != nil {
		t.Fatal(err)
	}
	if got := tr.Len(); got != 1 {
		t.Fatalf("expected 1 snapshot, got %d", got)
	}
}

// TestDiff_ReportsNewError — 编辑后新增的 diagnostic 被报告。
func TestDiff_ReportsNewError(t *testing.T) {
	tr := New(0)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	fl.set("a.fake", []Diagnostic{diag("a.fake", 1, "error", "old")})
	_ = tr.SnapshotBefore(context.Background(), "a.fake")

	fl.set("a.fake", []Diagnostic{
		diag("a.fake", 1, "error", "old"),
		diag("a.fake", 5, "error", "new"),
	})
	added, err := tr.Diff(context.Background(), "a.fake")
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 {
		t.Fatalf("expected 1 new diagnostic, got %d: %+v", len(added), added)
	}
	if added[0].Line != 5 || added[0].Message != "new" {
		t.Fatalf("unexpected diagnostic: %+v", added[0])
	}
}

// TestDiff_NoChangeNoReport — 编辑前后基线一致 → Diff 返回空。
func TestDiff_NoChangeNoReport(t *testing.T) {
	tr := New(0)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	fl.set("a.fake", []Diagnostic{diag("a.fake", 1, "warning", "w")})
	_ = tr.SnapshotBefore(context.Background(), "a.fake")

	added, _ := tr.Diff(context.Background(), "a.fake")
	if len(added) != 0 {
		t.Fatalf("expected no new diagnostics, got %+v", added)
	}
}

// TestDiff_DoesNotReportRemoved — R2 风险缓解：消失的 diagnostic 不报。
func TestDiff_DoesNotReportRemoved(t *testing.T) {
	tr := New(0)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	fl.set("a.fake", []Diagnostic{
		diag("a.fake", 1, "error", "e1"),
		diag("a.fake", 2, "error", "e2"),
	})
	_ = tr.SnapshotBefore(context.Background(), "a.fake")

	// 编辑后 e2 消失
	fl.set("a.fake", []Diagnostic{diag("a.fake", 1, "error", "e1")})
	added, _ := tr.Diff(context.Background(), "a.fake")
	if len(added) != 0 {
		t.Fatalf("expected no new diagnostics (e2 disappeared), got %+v", added)
	}
}

// TestLRU_EvictsOldest — 容量 N 满时插入新 file 触发 LRU 淘汰。
func TestLRU_EvictsOldest(t *testing.T) {
	tr := New(2)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	_ = tr.SnapshotBefore(context.Background(), "a.fake")
	_ = tr.SnapshotBefore(context.Background(), "b.fake")
	_ = tr.SnapshotBefore(context.Background(), "c.fake") // 触发淘汰 a

	if got := tr.Len(); got != 2 {
		t.Fatalf("expected len=2, got %d", got)
	}
}

// TestLRU_RefreshOnAccess — 访问过的 file 不会被淘汰。
func TestLRU_RefreshOnAccess(t *testing.T) {
	tr := New(2)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)

	_ = tr.SnapshotBefore(context.Background(), "a.fake")
	_ = tr.SnapshotBefore(context.Background(), "b.fake")
	// 重新访问 a（bump 到 front）
	_ = tr.SnapshotBefore(context.Background(), "a.fake")
	_ = tr.SnapshotBefore(context.Background(), "c.fake") // 触发淘汰 b

	if got := tr.Len(); got != 2 {
		t.Fatalf("expected len=2, got %d", got)
	}
	// a 应仍在：基线不应消失 → 重新 SnapshotBefore 应有数据
	fl.set("a.fake", []Diagnostic{diag("a.fake", 9, "error", "still-here")})
	added, _ := tr.Diff(context.Background(), "a.fake")
	if len(added) != 1 || added[0].Message != "still-here" {
		t.Fatalf("a.fake was evicted, expected still-here diagnostic, got %+v", added)
	}
}

// TestNoLinter_NoOp — 无 linter 路由时 SnapshotBefore/Diff 静默返回空。
func TestNoLinter_NoOp(t *testing.T) {
	tr := New(0)
	_ = tr.SnapshotBefore(context.Background(), "unknown.xyz")
	added, _ := tr.Diff(context.Background(), "unknown.xyz")
	if added != nil {
		t.Fatalf("expected nil for unknown ext, got %+v", added)
	}
	if tr.Len() != 0 {
		t.Fatalf("expected 0 snapshots, got %d", tr.Len())
	}
}

// TestFlush_ClearsAll — Flush 清空所有快照。
func TestFlush_ClearsAll(t *testing.T) {
	tr := New(0)
	fl := newFakeLinter()
	tr.SetLinter(".fake", fl.lint)
	_ = tr.SnapshotBefore(context.Background(), "a.fake")
	_ = tr.SnapshotBefore(context.Background(), "b.fake")
	tr.Flush()
	if tr.Len() != 0 {
		t.Fatalf("expected 0 after Flush, got %d", tr.Len())
	}
}

// TestAppendToReminder_Empty — 空 diagnostic 不修改 reminder。
func TestAppendToReminder_Empty(t *testing.T) {
	got := AppendToReminder("hello", nil)
	if got != "hello" {
		t.Fatalf("expected unchanged reminder, got %q", got)
	}
}

// TestAppendToReminder_WithDiags — diagnostic 列表渲染为 block。
func TestAppendToReminder_WithDiags(t *testing.T) {
	got := AppendToReminder("base", []Diagnostic{
		{File: "a.go", Line: 1, Severity: "error", Source: "go-vet", Message: "boom"},
	})
	for _, want := range []string{"<file_diagnostics>", "go-vet", "boom", "a.go", "base"} {
		if !contains(got, want) {
			t.Fatalf("missing %q in reminder:\n%s", want, got)
		}
	}
}

// TestConcurrent_NoRace — 并发 SnapshotBefore/Diff 无 race。
func TestConcurrent_NoRace(t *testing.T) {
	tr := New(100)
	var counter int64
	tr.SetLinter(".fake", func(ctx context.Context, file string) ([]Diagnostic, error) {
		atomic.AddInt64(&counter, 1)
		return []Diagnostic{diag(file, int(atomic.LoadInt64(&counter)), "info", "x")}, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f := fmt.Sprintf("f%d.fake", i)
			_ = tr.SnapshotBefore(context.Background(), f)
			_, _ = tr.Diff(context.Background(), f)
		}(i)
	}
	wg.Wait()
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
