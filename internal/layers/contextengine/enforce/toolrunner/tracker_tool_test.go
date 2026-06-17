package toolrunner_test

// W8 — D5-S23-A02 (alias G6) query_diagnostics LLM tool 单元测试。
//
// AC6:
//   - tracker tick 后 query_diagnostics 返回累积的 diagnostic
//   - 无 tick 时 query_diagnostics 返回空
//   - tracker 未注入 → tool 拒绝

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
)

// setGlobalTrackerForTest 注入 stub tracker,测试结束时清理。
func setGlobalTrackerForTest(t *testing.T, tr *tracker.Tracker) {
	t.Helper()
	prev := tracker.GlobalTracker()
	tracker.SetGlobalTracker(tr)
	t.Cleanup(func() { tracker.SetGlobalTracker(prev) })
}

// fakeGoLinter 返回固定 diagnostic 列表,验证 TickOnce + query 流程。
func fakeGoLinter(_ context.Context, file string) ([]tracker.Diagnostic, error) {
	return []tracker.Diagnostic{
		{File: file, Line: 1, Column: 1, Severity: "error", Message: "fake err 1", Source: "fake"},
		{File: file, Line: 2, Column: 5, Severity: "error", Message: "fake err 2", Source: "fake"},
		{File: file, Line: 3, Column: 1, Severity: "warning", Message: "fake warn", Source: "fake"},
	}, nil
}

// T: D5-S23-A02-T01
// tracker tick 后 query_diagnostics 返回累积的 diagnostic 列表。
func TestQueryDiagnostics_TickAccumulates(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/foo.go")
	setGlobalTrackerForTest(t, tr)

	// 第一次 tick:baseline 之前,所有 diagnostic 视为新增 (3 条)。
	added := tr.TickOnce(context.Background())
	if added != 3 {
		t.Fatalf("TickOnce added=%d, want 3", added)
	}

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		Count       int                   `json:"count"`
		TotalInBuf  int                   `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic  `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, res.Output)
	}
	if out.Count != 3 {
		t.Errorf("count = %d, want 3", out.Count)
	}
	if out.TotalInBuf != 3 {
		t.Errorf("total_in_buffer = %d, want 3", out.TotalInBuf)
	}
	if len(out.Diagnostics) != 3 {
		t.Errorf("diagnostics len = %d, want 3", len(out.Diagnostics))
	}
}

// T: D5-S23-A02-T02
// 无 tick 时 query_diagnostics 返回 count=0。
func TestQueryDiagnostics_EmptyWithoutTick(t *testing.T) {
	tr := tracker.New(0)
	setGlobalTrackerForTest(t, tr)

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		Count       int                  `json:"count"`
		TotalInBuf  int                  `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Count != 0 {
		t.Errorf("count = %d, want 0", out.Count)
	}
	if out.TotalInBuf != 0 {
		t.Errorf("total_in_buffer = %d, want 0", out.TotalInBuf)
	}
	if len(out.Diagnostics) != 0 {
		t.Errorf("diagnostics len = %d, want 0", len(out.Diagnostics))
	}
}

// T: global tracker 未注入 → tool 拒绝。
func TestQueryDiagnostics_GlobalTrackerNil(t *testing.T) {
	setGlobalTrackerForTest(t, nil)
	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{}`,
	})
	if !strings.Contains(res.Error, "global tracker not initialized") {
		t.Errorf("expected not initialized error, got %q", res.Error)
	}
}

// T: severity 过滤。
func TestQueryDiagnostics_SeverityFilter(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/bar.go")
	tr.TickOnce(context.Background())
	setGlobalTrackerForTest(t, tr)

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{"severity": "error"}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		Count       int                  `json:"count"`
		TotalInBuf  int                  `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Count != 2 {
		t.Errorf("count = %d, want 2 (only errors)", out.Count)
	}
	for _, d := range out.Diagnostics {
		if d.Severity != "error" {
			t.Errorf("severity filter leaked: got %s", d.Severity)
		}
	}
}

// T: file 过滤。
func TestQueryDiagnostics_FileFilter(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/a.go")
	tr.WatchFile("/tmp/b.go")
	// 第一次 tick:各自 baseline 累积 (a=3, b=3)。
	tr.TickOnce(context.Background())
	// 第二次 tick:Diff 返回空 (基线已对齐)。
	tr.TickOnce(context.Background())
	// 手动注入 b 的新 diagnostic 来测试过滤。
	tr.RecordDiags([]tracker.Diagnostic{
		{File: "/tmp/b.go", Line: 9, Column: 1, Severity: "error", Message: "b-only", Source: "fake"},
	})
	setGlobalTrackerForTest(t, tr)

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{"file": "/tmp/b.go"}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		Count       int                  `json:"count"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, d := range out.Diagnostics {
		if d.File != "/tmp/b.go" {
			t.Errorf("file filter leaked: got %s", d.File)
		}
	}
}

// T: limit 截断。
func TestQueryDiagnostics_LimitCap(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/limit.go")
	// 累积 5 轮,每轮 baseline 对齐,再 RecordDiags 注入 5 条独立 diagnostic。
	for i := 0; i < 5; i++ {
		tr.TickOnce(context.Background())
		tr.RecordDiags([]tracker.Diagnostic{
			{File: "/tmp/limit.go", Line: i + 1, Severity: "info", Message: "m", Source: "fake"},
		})
	}
	setGlobalTrackerForTest(t, tr)

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{"limit": 2}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
	var out struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Output), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Count != 2 {
		t.Errorf("count = %d, want 2 (limit applied)", out.Count)
	}
}

// T: 并发 tick 安全 (smoke test).
func TestQueryDiagnostics_ConcurrentTick(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/conc.go")
	setGlobalTrackerForTest(t, tr)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.TickOnce(context.Background())
		}()
	}
	wg.Wait()

	reg := toolrunner.NewToolRegistry()
	if err := toolrunner.RegisterTrackerTool(reg); err != nil {
		t.Fatalf("RegisterTrackerTool: %v", err)
	}
	res, _ := reg.Execute(context.Background(), toolrunner.ToolCall{
		Name:  "query_diagnostics",
		Input: `{}`,
	})
	if res.Error != "" {
		t.Fatalf("tool error: %s", res.Error)
	}
}
