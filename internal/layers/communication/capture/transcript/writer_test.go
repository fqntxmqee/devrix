package transcript

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestWriter_AppendCreatesFile — Append 后文件存在。
func TestWriter_AppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Append("sess-1", Event{Kind: "user", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.LoadReader("sess-1"); err != nil {
		t.Fatalf("read: %v", err)
	}
}

// TestWriter_FillsTimestamp — Time 零值时自动填。
func TestWriter_FillsTimestamp(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	_ = w.Append("s", Event{Kind: "user", Body: "x"})
	got, _ := w.LoadReader("s")
	if len(got) != 1 || got[0].Time.IsZero() {
		t.Errorf("Time not auto-filled: %+v", got)
	}
}

// TestWriter_AppendMultipleLines — 多条 event 都按序追加。
func TestWriter_AppendMultipleLines(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	for i := 0; i < 5; i++ {
		_ = w.Append("s", Event{Kind: "user", Body: string(rune('a' + i)), Time: time.Now().Add(time.Duration(i) * time.Millisecond)})
	}
	got, _ := w.LoadReader("s")
	if len(got) != 5 {
		t.Fatalf("expected 5, got %d", len(got))
	}
	for i, ev := range got {
		want := string(rune('a' + i))
		if ev.Body != want {
			t.Errorf("line %d: want %q, got %q", i, want, ev.Body)
		}
	}
}

// TestWriter_LoadReader_UnknownSession — 未知 session → empty, no error。
func TestWriter_LoadReader_UnknownSession(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	got, err := w.LoadReader("never-existed")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

// TestWriter_RequiresKind — Kind 为空 → error。
func TestWriter_RequiresKind(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	if err := w.Append("s", Event{Body: "x"}); err == nil {
		t.Fatal("expected error for empty kind")
	}
}

// TestWriter_RequiresSessionID — sessionID 为空 → error。
func TestWriter_RequiresSessionID(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	if err := w.Append("", Event{Kind: "user"}); err == nil {
		t.Fatal("expected error for empty sessionID")
	}
}

// TestWriter_NilSafe — nil writer 不 panic。
func TestWriter_NilSafe(t *testing.T) {
	var w *Writer
	if err := w.Append("s", Event{Kind: "user"}); err == nil {
		t.Fatal("expected error from nil writer")
	}
	if w.Dir() != "" {
		t.Error("nil writer Dir should be empty")
	}
}

// TestWriter_PathTraversal — sessionID 含 ../ 路径分隔符会被 sanitize。
func TestWriter_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	_ = w.Append("../../etc/passwd", Event{Kind: "user", Body: "x"})
	// 文件应落到 dir 内,而不是 dir 外
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if len(files) != 1 {
		t.Errorf("expected 1 file in dir, got %d", len(files))
	}
	for _, f := range files {
		if !strings.HasPrefix(f, dir) {
			t.Errorf("file escaped dir: %s", f)
		}
	}
}

// TestWriter_ConcurrentAppend — 100 个 goroutine 并发追加,无丢行。
func TestWriter_ConcurrentAppend(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = w.Append("s", Event{Kind: "user", Body: "x"})
		}(i)
	}
	wg.Wait()
	got, _ := w.LoadReader("s")
	if len(got) != 100 {
		t.Errorf("expected 100 events, got %d", len(got))
	}
}

// TestWriter_ListSessions — 多 session 时按 mtime 倒序。
func TestWriter_ListSessions(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	_ = w.Append("alpha", Event{Kind: "user", Body: "x"})
	time.Sleep(20 * time.Millisecond)
	_ = w.Append("beta", Event{Kind: "user", Body: "x"})

	got, err := w.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("expected >=2 sessions, got %d", len(got))
	}
	if got[0] != "beta" {
		t.Errorf("expected beta first (most recent), got %v", got)
	}
}

// TestWriter_Dir_ReportsPath — Dir() 返回构造时的 dir。
func TestWriter_Dir_ReportsPath(t *testing.T) {
	dir := t.TempDir()
	w, _ := NewWriter(dir)
	if w.Dir() != dir {
		t.Errorf("Dir() = %q, want %q", w.Dir(), dir)
	}
}

// TestNewWriter_EmptyDirRejected — 空 dir → error。
func TestNewWriter_EmptyDirRejected(t *testing.T) {
	_, err := NewWriter("")
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}
