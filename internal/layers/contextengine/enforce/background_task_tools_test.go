package enforce

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
)

func TestTaskStopRunner_cancels_running_task(t *testing.T) {
	reg := NewBackgroundRegistry()
	handle, _ := reg.RegisterWithCancel("sess_ts", "explore", "Explore", "explore_ts")
	defer handle.Cancel()

	SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{Registry: reg, Waiter: NewBackgroundWaiter(reg)})
	defer SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{})

	reg2 := tools.NewToolRegistry()
	if err := RegisterBackgroundTaskTools(reg2); err != nil {
		t.Fatal(err)
	}

	ctx := tools.WithToolSessionID(context.Background(), "sess_ts")
	res, err := reg2.Execute(ctx, tools.ToolCall{Name: "task_stop", Input: `{"task_id":"` + handle.ID + `"}`})
	if err != nil {
		t.Fatal(err)
	}
	if res.Error != "" {
		t.Fatalf("expected no error, got %q", res.Error)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("decode output: %v (raw=%s)", err, res.Output)
	}
	if out["cancelled"] != true {
		t.Fatalf("expected cancelled=true, got %v", out["cancelled"])
	}
	if out["new_status"] != "cancelled" {
		t.Fatalf("expected new_status=cancelled, got %v", out["new_status"])
	}
}

func TestTaskStopRunner_rejects_cross_session(t *testing.T) {
	reg := NewBackgroundRegistry()
	handle, _ := reg.RegisterWithCancel("sess_a", "explore", "Explore", "explore_a")
	defer handle.Cancel()

	SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{Registry: reg, Waiter: NewBackgroundWaiter(reg)})
	defer SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{})

	reg2 := tools.NewToolRegistry()
	_ = RegisterBackgroundTaskTools(reg2)

	ctx := tools.WithToolSessionID(context.Background(), "sess_b")
	res, _ := reg2.Execute(ctx, tools.ToolCall{Name: "task_stop", Input: `{"task_id":"` + handle.ID + `"}`})
	if res.Error == "" {
		t.Fatal("expected cross-session error")
	}
}

func TestTaskOutputRunner_block_false_returns_running(t *testing.T) {
	reg := NewBackgroundRegistry()
	handle, _ := reg.RegisterWithCancel("sess_out", "explore", "Explore", "explore_out")
	defer handle.Cancel()

	SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{Registry: reg, Waiter: NewBackgroundWaiter(reg)})
	defer SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{})

	reg2 := tools.NewToolRegistry()
	_ = RegisterBackgroundTaskTools(reg2)

	ctx := tools.WithToolSessionID(context.Background(), "sess_out")
	res, _ := reg2.Execute(ctx, tools.ToolCall{Name: "task_output", Input: `{"task_id":"` + handle.ID + `","block":false}`})
	if res.Error != "" {
		t.Fatalf("expected no error, got %q", res.Error)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(res.Output), &out)
	if out["status"] != "running" {
		t.Fatalf("expected status=running, got %v", out["status"])
	}
}

func TestTaskOutputRunner_block_true_waits_until_terminal(t *testing.T) {
	reg := NewBackgroundRegistry()
	handle, _ := reg.RegisterWithCancel("sess_wait", "explore", "Explore", "explore_wait")

	SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{Registry: reg, Waiter: NewBackgroundWaiter(reg)})
	defer SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{})

	reg2 := tools.NewToolRegistry()
	_ = RegisterBackgroundTaskTools(reg2)

	go func() {
		time.Sleep(50 * time.Millisecond)
		reg.Complete(handle.ID, "result text", "", nil)
	}()

	start := time.Now()
	ctx := tools.WithToolSessionID(context.Background(), "sess_wait")
	res, _ := reg2.Execute(ctx, tools.ToolCall{Name: "task_output", Input: `{"task_id":"` + handle.ID + `","block":true,"timeout_ms":2000}`})
	elapsed := time.Since(start)
	if res.Error != "" {
		t.Fatalf("expected no error, got %q", res.Error)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("block=true should not exceed timeout (took %v)", elapsed)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(res.Output), &out)
	if out["status"] != "completed" {
		t.Fatalf("expected status=completed, got %v (raw=%s)", out["status"], res.Output)
	}
	if out["output"] != "result text" {
		t.Fatalf("expected output='result text', got %v", out["output"])
	}
}

func TestTaskListBackgroundRunner_returns_session_tasks(t *testing.T) {
	reg := NewBackgroundRegistry()
	c1, _ := reg.RegisterWithCancel("sess_l", "explore", "Explore", "e1")
	c2, _ := reg.RegisterWithCancel("sess_l", "implement", "Implement", "i1")
	c3, _ := reg.RegisterWithCancel("sess_other", "explore", "Explore", "e2")
	defer c1.Cancel()
	defer c2.Cancel()
	defer c3.Cancel()

	SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{Registry: reg, Waiter: NewBackgroundWaiter(reg)})
	defer SetBackgroundTaskToolsDeps(BackgroundTaskToolsDeps{})

	reg2 := tools.NewToolRegistry()
	_ = RegisterBackgroundTaskTools(reg2)

	ctx := tools.WithToolSessionID(context.Background(), "sess_l")
	res, _ := reg2.Execute(ctx, tools.ToolCall{Name: "task_list_background", Input: "{}"})
	var out struct {
		Count int `json:"count"`
		Tasks []struct {
			TaskID string `json:"task_id"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 2 {
		t.Fatalf("expected count=2, got %d", out.Count)
	}
}
