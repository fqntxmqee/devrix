package toolrunner_test

// W11 phase 2a migration: query_diagnostics tool 单元测试 now exercises the
// surface path (TrackerSurface) instead of the deleted legacy
// toolrunner.trackerRunner + tracker.SetGlobalTracker path.
//
// The semantics are identical: pass a *tracker.Tracker into
// surface.NewTrackerSurface, then call surface.Execute. The previous
// "global not initialized" case is reproduced by passing nil.

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce/toolrunner/surface"
	"github.com/devrix/devrix/internal/layers/observability/diagnose/tracker"
)

// fakeGoLinter 返回固定 diagnostic 列表,验证 TickOnce + query 流程。
func fakeGoLinter(_ context.Context, file string) ([]tracker.Diagnostic, error) {
	return []tracker.Diagnostic{
		{File: file, Line: 1, Column: 1, Severity: "error", Message: "fake err 1", Source: "fake"},
		{File: file, Line: 2, Column: 5, Severity: "error", Message: "fake err 2", Source: "fake"},
		{File: file, Line: 3, Column: 1, Severity: "warning", Message: "fake warn", Source: "fake"},
	}, nil
}

func executeQueryDiag(t *testing.T, tr *tracker.Tracker, input string) (string, string) {
	t.Helper()
	s := surface.NewTrackerSurface(tr)
	res, err := s.Execute(context.Background(), "query_diagnostics", input, "")
	if err != nil {
		t.Fatalf("surface.Execute: %v", err)
	}
	return res.Output, res.Error
}

// T: D5-S23-A02-T01
// tracker tick 后 query_diagnostics 返回累积的 diagnostic 列表。
func TestQueryDiagnostics_TickAccumulates(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/foo.go")

	added := tr.TickOnce(context.Background())
	if added != 3 {
		t.Fatalf("TickOnce added=%d, want 3", added)
	}

	out, errStr := executeQueryDiag(t, tr, `{}`)
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		Count       int                  `json:"count"`
		TotalInBuf  int                  `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	if o.Count != 3 {
		t.Errorf("count = %d, want 3", o.Count)
	}
	if o.TotalInBuf != 3 {
		t.Errorf("total_in_buffer = %d, want 3", o.TotalInBuf)
	}
	if len(o.Diagnostics) != 3 {
		t.Errorf("diagnostics len = %d, want 3", len(o.Diagnostics))
	}
}

// T: D5-S23-A02-T02 — 无 tick 时 query_diagnostics 返回 count=0。
func TestQueryDiagnostics_EmptyWithoutTick(t *testing.T) {
	tr := tracker.New(0)
	out, errStr := executeQueryDiag(t, tr, `{}`)
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		Count       int                  `json:"count"`
		TotalInBuf  int                  `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	if o.Count != 0 {
		t.Errorf("count = %d, want 0", o.Count)
	}
	if o.TotalInBuf != 0 {
		t.Errorf("total_in_buffer = %d, want 0", o.TotalInBuf)
	}
	if len(o.Diagnostics) != 0 {
		t.Errorf("diagnostics len = %d, want 0", len(o.Diagnostics))
	}
}

// T: tracker 未注入 → surface 拒绝 (返回 "tracker not initialized")。
func TestQueryDiagnostics_TrackerNil(t *testing.T) {
	_, errStr := executeQueryDiag(t, nil, `{}`)
	if !strings.Contains(errStr, "tracker not initialized") {
		t.Errorf("expected not initialized error, got %q", errStr)
	}
}

// T: severity 过滤。
func TestQueryDiagnostics_SeverityFilter(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/bar.go")
	tr.TickOnce(context.Background())

	out, errStr := executeQueryDiag(t, tr, `{"severity": "error"}`)
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		Count       int                  `json:"count"`
		TotalInBuf  int                  `json:"total_in_buffer"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	if o.Count != 2 {
		t.Errorf("count = %d, want 2 (only errors)", o.Count)
	}
	for _, d := range o.Diagnostics {
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
	tr.TickOnce(context.Background())
	tr.TickOnce(context.Background())
	tr.RecordDiags([]tracker.Diagnostic{
		{File: "/tmp/b.go", Line: 9, Column: 1, Severity: "error", Message: "b-only", Source: "fake"},
	})

	out, errStr := executeQueryDiag(t, tr, `{"file": "/tmp/b.go"}`)
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		Count       int                  `json:"count"`
		Diagnostics []tracker.Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	for _, d := range o.Diagnostics {
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
	for i := 0; i < 5; i++ {
		tr.TickOnce(context.Background())
		tr.RecordDiags([]tracker.Diagnostic{
			{File: "/tmp/limit.go", Line: i + 1, Severity: "info", Message: "m", Source: "fake"},
		})
	}

	out, errStr := executeQueryDiag(t, tr, `{"limit": 2}`)
	if errStr != "" {
		t.Fatalf("tool error: %s", errStr)
	}
	var o struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("unmarshal: %v, output=%s", err, out)
	}
	if o.Count != 2 {
		t.Errorf("count = %d, want 2 (limit applied)", o.Count)
	}
}

// T: 并发 tick 安全 (smoke test).
func TestQueryDiagnostics_ConcurrentTick(t *testing.T) {
	tr := tracker.New(0)
	tr.SetLinter(".go", fakeGoLinter)
	tr.WatchFile("/tmp/conc.go")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.TickOnce(context.Background())
		}()
	}
	wg.Wait()

	// 8 个并发 tick 中至少一个会看到 baseline 之前的 3 个 diagnostic;
	// concurrent execution 不应 crash, surface.Execute 也不应 race.
	out, errStr := executeQueryDiag(t, tr, `{}`)
	if errStr != "" {
		t.Errorf("tool error: %s", errStr)
	}
	if !strings.Contains(out, `"diagnostics"`) {
		t.Errorf("expected diagnostics key in output, got %s", out)
	}
}
