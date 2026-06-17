package prompt_test

// W12 — D4-S12-A03 (alias G3) Task Notify consume via prompt assembler 单元测试。
//
// AC11:
//   - notify bus 有 1+ event → <task_notifications> block 注入到 session context
//   - notify bus 空 → block 不注入
//   - 第二次 drain (同一 event) → block 不再注入 (drain 是消费性的)

import (
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel/notify"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// resetNotifyBusForTest 用新的 in-memory bus 替换 global, 测试结束时恢复.
func resetNotifyBusForTest(t *testing.T) {
	t.Helper()
	prev := notify.GlobalBus()
	notify.SetGlobalBus(notify.NewInMemoryBus(8))
	t.Cleanup(func() { notify.SetGlobalBus(prev) })
}

func newAssembler(t *testing.T) *prompt.SystemPromptAssembler {
	t.Helper()
	return prompt.NewSystemPromptAssembler(config.WorkspacePromptConfig{
		EmbedCoreTemplate: false,
	})
}

// T: D4-S12-A03-T01
// notify bus 有 1 event → block 注入。
func TestAssemble_NotifyInjectedWhenEventPresent(t *testing.T) {
	resetNotifyBusForTest(t)
	bus := notify.GlobalBus()
	bus.Publish("sess_notify_1", notify.CompletionEvent{
		TaskID:  "task-A",
		Kind:    "bash",
		Summary: "ran ls",
		Time:    time.Now(),
	})

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
// notify bus 空 → block 不注入。
func TestAssemble_NotifyAbsentWhenBusEmpty(t *testing.T) {
	resetNotifyBusForTest(t)
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
	resetNotifyBusForTest(t)
	bus := notify.GlobalBus()
	bus.Publish("sess_notify_3", notify.CompletionEvent{
		TaskID:  "task-B",
		Kind:    "agent",
		Summary: "did something",
		Time:    time.Now(),
	})

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
	resetNotifyBusForTest(t)
	bus := notify.GlobalBus()
	for i := 0; i < 3; i++ {
		bus.Publish("sess_notify_4", notify.CompletionEvent{
			TaskID:  "t-" + string(rune('A'+i)),
			Kind:    "bash",
			Summary: "job",
			Time:    time.Now(),
		})
	}
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
	resetNotifyBusForTest(t)
	a := newAssembler(t)
	out, _ := a.Build(prompt.SystemPromptBuildInput{
		WorkDir: "/tmp",
		Runtime: prompt.ProcessRuntimeContext{SessionID: ""},
	})
	if strings.Contains(out, "<task_notifications>") {
		t.Errorf("did not expect <task_notifications> for empty sessionID, got:\n%s", out)
	}
}
