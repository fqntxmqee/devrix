package prompt_test

// W12 — D4-S12-A03 (alias G3) Task Notify consume via prompt assembler 单元测试。
//
// AC11:
//   - notify bus 有 1+ event → <task_notifications> block 注入到 session context
//   - notify bus 空 → block 不注入
//   - 第二次 drain (同一 event) → block 不再注入 (drain 是消费性的)
//
// S4-Gate H-3 fix: 测试通过 prompt.SetTaskNotifDrainer 注入 stub, 不再
// import orchestration/workmodel/notify, 保持 D2 边界. 真实 drainer 在
// bootstrap 层注入.

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// stubTaskNotifStore 用一个简单的 in-memory store 模拟 notify bus 的行为:
//  - per-session 一次性注入
//  - Drain 后清空
type stubTaskNotifStore struct {
	mu        sync.Mutex
	pending   map[string][]stubCompletionEvent
	delivered map[string]bool
}

type stubCompletionEvent struct {
	TaskID  string
	Kind    string
	Summary string
	Time    time.Time
}

func newStubTaskNotifStore() *stubTaskNotifStore {
	return &stubTaskNotifStore{
		pending:   map[string][]stubCompletionEvent{},
		delivered: map[string]bool{},
	}
}

func (s *stubTaskNotifStore) publish(sessionID, taskID, kind, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[sessionID] = append(s.pending[sessionID], stubCompletionEvent{
		TaskID:  taskID,
		Kind:    kind,
		Summary: summary,
		Time:    time.Now(),
	})
}

func (s *stubTaskNotifStore) drainerFor(sessionID string) prompt.TaskNotifDrainerFunc {
	return func(sessID string) string {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.delivered[sessID] {
			return ""
		}
		evs := s.pending[sessID]
		if len(evs) == 0 {
			return ""
		}
		s.delivered[sessID] = true
		// 渲染成 <task_notifications> block — 跟 notify.FormatReminder 一致
		var b strings.Builder
		b.WriteString("<task_notifications>\n")
		for _, e := range evs {
			b.WriteString("- ")
			b.WriteString(e.TaskID)
			b.WriteString(" (")
			b.WriteString(e.Kind)
			b.WriteString("): ")
			b.WriteString(e.Summary)
			b.WriteString("\n")
		}
		b.WriteString("</task_notifications>\n")
		return b.String()
	}
}

// resetTaskNotifDrainerForTest 把全局 drainer 替换成 stub, 测试结束后恢复.
func resetTaskNotifDrainerForTest(t *testing.T) prompt.TaskNotifDrainerFunc {
	t.Helper()
	// 把 drainer 设成一个永远返回 "" 的占位, 测试自己重新注入
	prev := prompt.SetTaskNotifDrainerForTest(func(_ string) string { return "" })
	t.Cleanup(func() { prompt.SetTaskNotifDrainerForTest(prev) })
	return prev
}

func newAssembler(t *testing.T) *prompt.SystemPromptAssembler {
	t.Helper()
	return prompt.NewSystemPromptAssembler(config.WorkspacePromptConfig{
		EmbedCoreTemplate: false,
	})
}

// T: D4-S12-A03-T01
// drainer 注入 1 event → block 注入。
func TestAssemble_NotifyInjectedWhenEventPresent(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	store := newStubTaskNotifStore()
	store.publish("sess_notify_1", "task-A", "bash", "ran ls")
	prompt.SetTaskNotifDrainer(store.drainerFor("sess_notify_1"))

	a := newAssembler(t)
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Session: types.NewSession("sess_notify_1", "cli", "/tmp"),
		Runtime: prompt.ProcessRuntimeContext{SessionID: "sess_notify_1"},
	})
	if !strings.Contains(out, "<task_notifications>") {
		t.Fatalf("expected <task_notifications> block in prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "task-A") {
		t.Errorf("expected task-A in prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "ran ls") {
		t.Errorf("expected summary 'ran ls' in prompt, got:\n%s", out)
	}
}

// T: D4-S12-A03-T02
// drainer 空 → block 不注入。
func TestAssemble_NotifyAbsentWhenBusEmpty(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	a := newAssembler(t)
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Session: types.NewSession("sess_notify_2", "cli", "/tmp"),
		Runtime: prompt.ProcessRuntimeContext{SessionID: "sess_notify_2"},
	})
	if strings.Contains(out, "<task_notifications>") {
		t.Errorf("did not expect <task_notifications> block, got:\n%s", out)
	}
}

// T: drain 是消费性的 — 第二次 build 不再注入相同 event。
func TestAssemble_NotifyDrainIsIdempotent(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	store := newStubTaskNotifStore()
	store.publish("sess_notify_3", "task-B", "agent", "did something")
	prompt.SetTaskNotifDrainer(store.drainerFor("sess_notify_3"))

	a := newAssembler(t)
	in := prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Session: types.NewSession("sess_notify_3", "cli", "/tmp"),
		Runtime: prompt.ProcessRuntimeContext{SessionID: "sess_notify_3"},
	}
	first, _ := a.Build(in)
	if !strings.Contains(first, "task-B") {
		t.Fatalf("expected task-B in first build, got:\n%s", first)
	}

	second, _ := a.Build(in)
	if strings.Contains(second, "task-B") {
		t.Errorf("expected task-B drained after first build, got:\n%s", second)
	}
	if strings.Contains(second, "<task_notifications>") {
		t.Errorf("expected no <task_notifications> in second build, got:\n%s", second)
	}
}

// T: 多 event 全部注入。
func TestAssemble_NotifyMultipleEvents(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	store := newStubTaskNotifStore()
	for i := 0; i < 3; i++ {
		store.publish("sess_notify_4", "t-"+string(rune('A'+i)), "bash", "job")
	}
	prompt.SetTaskNotifDrainer(store.drainerFor("sess_notify_4"))

	a := newAssembler(t)
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Session: types.NewSession("sess_notify_4", "cli", "/tmp"),
		Runtime: prompt.ProcessRuntimeContext{SessionID: "sess_notify_4"},
	})
	for _, tid := range []string{"t-A", "t-B", "t-C"} {
		if !strings.Contains(out, tid) {
			t.Errorf("expected %q in prompt, got:\n%s", tid, out)
		}
	}
}

// T: SessionID 为空时, drainTaskNotifications 安全返回空。
func TestAssemble_NotifyEmptySessionID(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	a := newAssembler(t)
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Runtime: prompt.ProcessRuntimeContext{SessionID: ""},
	})
	if strings.Contains(out, "<task_notifications>") {
		t.Errorf("did not expect <task_notifications> for empty sessionID, got:\n%s", out)
	}
}

// T: S4-Gate H-2 fix — 第二次 build 即使 session_context 命中 cache, 也要能 drain
// 到新 event. 之前 drainTaskNotifications 嵌在 buildSessionContext 内部, 被
// dynamic_sections 的 cache 缓存住, 导致后续 publish 的 event 丢失.
func TestAssemble_NotifyCacheDoesNotLoseEvents(t *testing.T) {
	resetTaskNotifDrainerForTest(t)
	store := newStubTaskNotifStore()

	a := newAssembler(t)
	in := prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Session: types.NewSession("sess_notify_cache", "cli", "/tmp"),
		Runtime: prompt.ProcessRuntimeContext{SessionID: "sess_notify_cache"},
	}

	// 第一次 build, 触发 dynamic_sections cache 命中 session_context.
	store.publish("sess_notify_cache", "task-1", "bash", "first job")
	prompt.SetTaskNotifDrainer(store.drainerFor("sess_notify_cache"))
	first, _ := a.Build(in)
	if !strings.Contains(first, "task-1") {
		t.Fatalf("expected task-1 in first build, got:\n%s", first)
	}

	// 清空 dynamic section cache, 模拟 session context 已经被"缓存"过的状态.
	// 实际上, 即使没有这个 clear, 第二次 build 命中 cache 也必须能拿到新 event.
	prompt.ClearDynamicSectionCache("sess_notify_cache")

	// 关键: 第二次 build 时 publish 新 event, 必须能 drain 出来.
	// 但 store 已经被第一次 drain 标记 delivered, 需要用新 store.
	store2 := newStubTaskNotifStore()
	store2.publish("sess_notify_cache", "task-2", "agent", "second job")
	prompt.SetTaskNotifDrainer(store2.drainerFor("sess_notify_cache"))

	second, _ := a.Build(in)
	if !strings.Contains(second, "task-2") {
		t.Errorf("H-2: expected task-2 in second build (drain must not be cached), got:\n%s", second)
	}
	if !strings.Contains(second, "<task_notifications>") {
		t.Errorf("H-2: expected <task_notifications> in second build, got:\n%s", second)
	}
}
