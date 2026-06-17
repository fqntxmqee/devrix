package capture_test

// W11 — D1-S2-A02 (alias A3) Transcript OnSessionClose 持久化 单元测试。
//
// AC9:
//   - session close (ExpireSession) 后 .jsonl 写入 session_close event
//   - ListSessions 按 mtime 倒序

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/communication/capture/transcript"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D1-S2-A02-T01
// ExpireSession 后, transcript .jsonl 文件写入 session_close event.
func TestExpireSession_WritesTranscript(t *testing.T) {
	tmp := t.TempDir()
	tw, err := transcript.NewWriter(filepath.Join(tmp, "transcripts"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	prevW := transcript.GlobalWriter()
	transcript.SetGlobalWriter(tw)
	t.Cleanup(func() { transcript.SetGlobalWriter(prevW) })

	store, err := capture.NewFileSessionStore(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("NewFileSessionStore: %v", err)
	}
	sess := types.NewSession("sess_close_1", "cli", "/tmp/work")
	if err := store.Create(sess); err != nil {
		t.Fatalf("store.Create: %v", err)
	}

	gw := capture.NewCommunicationGateway(store, nil, nil, nil)
	if err := gw.ExpireSession("sess_close_1"); err != nil {
		t.Fatalf("ExpireSession: %v", err)
	}

	// 验证 .jsonl 文件存在 + 至少一条 session_close event.
	events, err := tw.LoadReader("sess_close_1")
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("no events written to transcript")
	}
	last := events[len(events)-1]
	if last.Kind != "session_close" {
		t.Errorf("last event kind = %q, want session_close", last.Kind)
	}
	if last.Body != "expired" {
		t.Errorf("last event body = %q, want expired", last.Body)
	}
}

// T: ExpireSession 后 session 在 store 中被删除.
func TestExpireSession_RemovesFromStore(t *testing.T) {
	tmp := t.TempDir()
	store, err := capture.NewFileSessionStore(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("NewFileSessionStore: %v", err)
	}
	sess := types.NewSession("sess_close_2", "cli", "/tmp/work")
	if err := store.Create(sess); err != nil {
		t.Fatalf("store.Create: %v", err)
	}

	gw := capture.NewCommunicationGateway(store, nil, nil, nil)
	if err := gw.ExpireSession("sess_close_2"); err != nil {
		t.Fatalf("ExpireSession: %v", err)
	}
	got, err := store.Get("sess_close_2")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if got != nil {
		t.Errorf("expected session to be deleted, got %+v", got)
	}
}

// T: ListSessions 按 mtime 倒序 — 写入 2 个 session jsonl, 验证排序.
func TestListSessions_OrderedByMTime(t *testing.T) {
	tmp := t.TempDir()
	tw, err := transcript.NewWriter(tmp)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	// 写入 2 个 session, 间隔 > 10ms 确保 mtime 不同.
	if err := tw.Append("alpha", transcript.Event{Kind: "user", Body: "hi"}); err != nil {
		t.Fatalf("Append alpha: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := tw.Append("beta", transcript.Event{Kind: "user", Body: "hi"}); err != nil {
		t.Fatalf("Append beta: %v", err)
	}
	sessions, err := tw.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	// beta 应该排第一（最近写入）
	if sessions[0] != "beta" {
		t.Errorf("sessions[0] = %q, want beta (most recent)", sessions[0])
	}
	if sessions[1] != "alpha" {
		t.Errorf("sessions[1] = %q, want alpha (older)", sessions[1])
	}
}

// T: 无 GlobalWriter 时 ExpireSession 不报错（transcript 是 best-effort）.
func TestExpireSession_NoGlobalWriterNoError(t *testing.T) {
	prevW := transcript.GlobalWriter()
	transcript.SetGlobalWriter(nil)
	t.Cleanup(func() { transcript.SetGlobalWriter(prevW) })

	tmp := t.TempDir()
	store, err := capture.NewFileSessionStore(filepath.Join(tmp, "sessions"))
	if err != nil {
		t.Fatalf("NewFileSessionStore: %v", err)
	}
	sess := types.NewSession("sess_no_tw", "cli", "/tmp")
	if err := store.Create(sess); err != nil {
		t.Fatalf("store.Create: %v", err)
	}
	gw := capture.NewCommunicationGateway(store, nil, nil, nil)
	if err := gw.ExpireSession("sess_no_tw"); err != nil {
		t.Errorf("ExpireSession should not error when transcript is nil, got: %v", err)
	}
	// Ensure context import is referenced.
	_ = context.Background
	_ = os.Getenv
}
