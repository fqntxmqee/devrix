package notify

import (
	"sync"
	"testing"
	"time"
)

// TestPublish_DeliversToChannel — Publish 写入后 Subscribe 立即能收到。
func TestPublish_DeliversToChannel(t *testing.T) {
	bus := NewInMemoryBus(8)
	bus.Publish("s1", CompletionEvent{TaskID: "t1", Kind: "bash", Summary: "ok"})

	select {
	case e := <-bus.Subscribe("s1"):
		if e.TaskID != "t1" || e.Summary != "ok" {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected event within 100ms")
	}
}

// TestPublish_FillsTimestamp — 缺省 Time 字段自动填 time.Now。
func TestPublish_FillsTimestamp(t *testing.T) {
	bus := NewInMemoryBus(8)
	bus.Publish("s", CompletionEvent{TaskID: "t"})

	e := <-bus.Subscribe("s")
	if e.Time.IsZero() {
		t.Fatal("expected Time to be auto-filled")
	}
}

// TestChannelFull_OverflowsToPending — channel 缓冲写满后,溢出到 pending list。
func TestChannelFull_OverflowsToPending(t *testing.T) {
	bus := NewInMemoryBus(2) // 小 buffer
	bus.Publish("s", CompletionEvent{TaskID: "t1"})
	bus.Publish("s", CompletionEvent{TaskID: "t2"})
	bus.Publish("s", CompletionEvent{TaskID: "t3"}) // 溢出
	bus.Publish("s", CompletionEvent{TaskID: "t4"}) // 溢出

	if got := bus.Len("s"); got != 4 {
		t.Fatalf("expected Len=4, got %d", got)
	}
}

// TestDrain_ReturnsAllEvents — Drain 一次性读出 channel + pending 全部。
func TestDrain_ReturnsAllEvents(t *testing.T) {
	bus := NewInMemoryBus(2)
	bus.Publish("s", CompletionEvent{TaskID: "t1"})
	bus.Publish("s", CompletionEvent{TaskID: "t2"})
	bus.Publish("s", CompletionEvent{TaskID: "t3"})
	bus.Publish("s", CompletionEvent{TaskID: "t4"})

	got := bus.Drain("s")
	if len(got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got))
	}
	// Drain 后应清空
	if bus.Len("s") != 0 {
		t.Errorf("expected Len=0 after Drain, got %d", bus.Len("s"))
	}
}

// TestDrain_SkipsUnknownSession — 未知 session 返回空。
func TestDrain_SkipsUnknownSession(t *testing.T) {
	bus := NewInMemoryBus(8)
	got := bus.Drain("nope")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

// TestSessionIsolation — 不同 session 的 event 互不可见。
func TestSessionIsolation(t *testing.T) {
	bus := NewInMemoryBus(8)
	bus.Publish("alpha", CompletionEvent{TaskID: "a1"})
	bus.Publish("beta", CompletionEvent{TaskID: "b1"})

	a := bus.Drain("alpha")
	b := bus.Drain("beta")
	if len(a) != 1 || a[0].TaskID != "a1" {
		t.Errorf("alpha drain wrong: %+v", a)
	}
	if len(b) != 1 || b[0].TaskID != "b1" {
		t.Errorf("beta drain wrong: %+v", b)
	}
}

// TestFormatReminder_RendersXMLBlock — 渲染 <task_notifications> 块。
func TestFormatReminder_RendersXMLBlock(t *testing.T) {
	events := []CompletionEvent{
		{TaskID: "t1", Kind: "bash", Summary: "ls done", Time: time.Date(2026, 6, 17, 12, 30, 45, 0, time.UTC)},
		{TaskID: "t2", Kind: "agent", Error: "timeout", Time: time.Date(2026, 6, 17, 12, 31, 0, 0, time.UTC)},
	}
	out := FormatReminder(events)
	want := []string{
		"<task_notifications>",
		"</task_notifications>",
		"bash t1: ls done",
		`agent t2 ERROR: timeout`,
		"12:30:45",
		"12:31:00",
	}
	for _, w := range want {
		if !contains(out, w) {
			t.Errorf("output missing %q\nfull:\n%s", w, out)
		}
	}
}

// TestFormatReminder_EmptyReturnsEmpty — 空 events 返回空字符串。
func TestFormatReminder_EmptyReturnsEmpty(t *testing.T) {
	if got := FormatReminder(nil); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	if got := FormatReminder([]CompletionEvent{}); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// TestClose_RemovesChannel — Close 后 Len 不再统计 channel。
func TestClose_RemovesChannel(t *testing.T) {
	bus := NewInMemoryBus(8)
	bus.Publish("s", CompletionEvent{TaskID: "t"})
	bus.Close("s")

	if bus.Len("s") != 0 {
		t.Errorf("expected Len=0 after Close, got %d", bus.Len("s"))
	}
	// 关闭后 channel 仍可重新创建（惰性）
	bus.Publish("s", CompletionEvent{TaskID: "t2"})
	if bus.Len("s") != 1 {
		t.Errorf("expected Len=1 after republish, got %d", bus.Len("s"))
	}
}

// TestConcurrentPublish — 多 producer 并发写入,无数据竞争。
func TestConcurrentPublish(t *testing.T) {
	bus := NewInMemoryBus(64)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			bus.Publish("s", CompletionEvent{TaskID: "t", Summary: "x"})
		}(i)
	}
	wg.Wait()
	if got := bus.Len("s"); got != 50 {
		t.Errorf("expected 50 events, got %d", got)
	}
}

// TestBusInterfaceConformance — InMemoryBus 实现 Bus 接口。
func TestBusInterfaceConformance(t *testing.T) {
	var _ Bus = NewInMemoryBus(8)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
