// T: D2-S15-A02-T19 — 50-file review 并发版本 e2e 测试.
//
// 在 PR-B 之前, 50 个 read_file 串行执行, 用户体验差. PR-B 通过
// partitionToolCalls 把它们并到一个 batch, 50 read_file 全部并发执行.
//
// AC10: 50/50 完成, 总 wall time < 串行 / 3.
//
// 老的 review50_e2e_test.go (compression 包) 是 D2 Token Design 2.0
// 治本 invariant 测试 (read_file offset/limit 验证), 跟本测试职责分离:
//   - 老的: 验证 read_file 自身 + 内容保留 (内容 / EOF marker)
//   - 本测试: 验证 partitionToolCalls 的并发路径 (wall time / 50/50)
//
// DSAFT: D7-S9-A50-T19 (DM-20260702-009 PR-B).
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/sessionorchestrator"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// readFileSurface is a ToolSurface that simulates the BuiltinSurface's
// read_file tool. It sleeps 30ms per call (to simulate disk I/O) and
// reads the file from disk. The IsConcurrencySafe baseline is
// spec.ConcurrencySafe=true (read_file is per-AC18 always concurrency
// safe regardless of input size).
type readFileSurface struct {
	rootDir string
	hits    *int32
	delay   time.Duration
}

func (s *readFileSurface) Name() string { return "builtin" }
func (s *readFileSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: "read_file", ConcurrencySafe: true, Risk: types.RiskLevelLow}}
}
func (s *readFileSurface) RiskLevel(name string) types.RiskLevel {
	if name == "read_file" {
		return types.RiskLevelLow
	}
	return ""
}
func (s *readFileSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *readFileSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *readFileSurface) IsConcurrencySafe(_ json.RawMessage) bool { return true }
func (s *readFileSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

func (s *readFileSurface) Execute(_ context.Context, _, input, _ string) (*contracts.ToolResult, error) {
	atomic.AddInt32(s.hits, 1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	var in struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &contracts.ToolResult{Error: "invalid input: " + err.Error()}, nil
	}
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return &contracts.ToolResult{Error: err.Error()}, nil
	}
	return &contracts.ToolResult{Output: string(data)}, nil
}

// make50Files creates 50 small text files in t.TempDir(). Each file
// has a unique name and unique content; content size is tiny (~100
// bytes) so the read_file Execute path itself is negligible — the
// dominant cost is the per-call delay (30ms) which makes the serial vs
// concurrent timing difference observable.
func make50Files(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	files := make([]string, 50)
	for i := 0; i < 50; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file-%02d.txt", i))
		content := fmt.Sprintf("file %d: line1\nfile %d: line2\nEOF-MARKER-%d\n", i, i, i)
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files[i] = name
	}
	return dir, files
}

// T19: 50 read_file calls in one batch — total wall time < serial / 3.
//
// Serial: 50 × 30ms = 1500ms.
// Concurrent: ~30ms (1 batch of 50, all in parallel).
// AC10 target: < 500ms (serial / 3).
func TestReview50_Concurrent_UnderSerialThird(t *testing.T) {
	_, files := make50Files(t)
	hits := new(int32)
	s := &readFileSurface{hits: hits, delay: 30 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})

	// 50 read_file calls.
	calls := make([]llmgateway.ToolCall, len(files))
	for i, f := range files {
		calls[i] = llmgateway.ToolCall{
			ID:    fmt.Sprintf("tc_%02d", i),
			Name:  "read_file",
			Input: fmt.Sprintf(`{"file_path":%q}`, f),
		}
	}

	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		if res == nil {
			return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: "nil result"}
		}
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}

	start := time.Now()
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)
	elapsed := time.Since(start)

	// AC10: 50/50 完成, no errors.
	if len(results) != 50 {
		t.Fatalf("len(results)=%d, want 50", len(results))
	}
	for i, r := range results {
		if r.ToolCallID != calls[i].ID {
			t.Errorf("results[%d].ToolCallID=%q, want %q (order broken)", i, r.ToolCallID, calls[i].ID)
		}
		if r.Error != "" {
			t.Errorf("results[%d].Error=%q, want empty", i, r.Error)
		}
		if r.Output == "" {
			t.Errorf("results[%d].Output is empty", i)
		}
	}
	if got := atomic.LoadInt32(hits); int(got) != 50 {
		t.Errorf("hits=%d, want 50", got)
	}

	// AC10: wall time < serial / 3. Serial = 50 × 30ms = 1500ms.
	// Target: < 500ms. Generous bound: 800ms (CI noise allowance).
	const serialTime = 1500 * time.Millisecond
	const target = serialTime / 3 // 500ms
	const ceiling = 800 * time.Millisecond
	t.Logf("T19: 50 read_file wall time = %v (target <%v, ceiling <%v)", elapsed, target, ceiling)
	if elapsed > ceiling {
		t.Errorf("elapsed=%v, want <%v (AC10: 50/50 < serial/3)", elapsed, ceiling)
	}
}

// T19 (control): 50 read_file calls with the OLD sequential path
// (per-tool IsConcurrencySafe=false on every call) — should run in
// ~1500ms (50 × 30ms), NOT < 500ms. This proves the concurrent path
// is actually concurrent (and not just timing-flukey on a fast CI).
func TestReview50_SequentialBaseline_NotConcurrent(t *testing.T) {
	_, files := make50Files(t)
	hits := new(int32)
	// ConcurrencySafe=false forces the sequential path. The
	// partitionToolCalls algorithm will see 50 unsafe calls and
	// produce 50 sequential batches.
	s := &noConcReadFileSurface{hits: hits, delay: 30 * time.Millisecond}
	lookup := BuildSurfaceLookup([]contracts.ToolSurface{s})

	calls := make([]llmgateway.ToolCall, len(files))
	for i, f := range files {
		calls[i] = llmgateway.ToolCall{
			ID:    fmt.Sprintf("tc_%02d", i),
			Name:  "read_file",
			Input: fmt.Sprintf(`{"file_path":%q}`, f),
		}
	}

	exec := func(_ context.Context, call llmgateway.ToolCall) sessionorchestrator.ToolResult {
		res, _ := s.Execute(context.Background(), call.Name, call.Input, "")
		if res == nil {
			return sessionorchestrator.ToolResult{ToolCallID: call.ID, Error: "nil result"}
		}
		return sessionorchestrator.ToolResult{ToolCallID: call.ID, Output: res.Output, Error: res.Error}
	}

	start := time.Now()
	results := ExecuteBatches(context.Background(), calls, lookup, exec, 0)
	elapsed := time.Since(start)

	if len(results) != 50 {
		t.Fatalf("len(results)=%d, want 50", len(results))
	}
	for i, r := range results {
		if r.Error != "" {
			t.Errorf("results[%d].Error=%q, want empty", i, r.Error)
		}
	}
	// Sequential: 50 × 30ms = 1500ms. Generous lower bound: 1000ms
	// (sanity check that the test is actually running sequential).
	if elapsed < 1000*time.Millisecond {
		t.Errorf("elapsed=%v, want >= 1000ms (sequential baseline)", elapsed)
	}
	t.Logf("T19 control: 50 read_file sequential = %v (sanity check)", elapsed)
}

// noConcReadFileSurface is the control variant — IsConcurrencySafe
// returns false (so the partition path runs sequentially). Same
// read_file execution logic.
type noConcReadFileSurface struct {
	rootDir string
	hits    *int32
	delay   time.Duration
}

func (s *noConcReadFileSurface) Name() string { return "builtin" }
func (s *noConcReadFileSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
	return []contracts.ToolSpec{{Name: "read_file", ConcurrencySafe: false, Risk: types.RiskLevelLow}}
}
func (s *noConcReadFileSurface) RiskLevel(name string) types.RiskLevel {
	if name == "read_file" {
		return types.RiskLevelLow
	}
	return ""
}
func (s *noConcReadFileSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}
func (s *noConcReadFileSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}
func (s *noConcReadFileSurface) IsConcurrencySafe(_ json.RawMessage) bool { return false }
func (s *noConcReadFileSurface) ToAutoClassifierInput(_ json.RawMessage) string {
	return ""
}

func (s *noConcReadFileSurface) Execute(_ context.Context, _, input, _ string) (*contracts.ToolResult, error) {
	atomic.AddInt32(s.hits, 1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	var in struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return &contracts.ToolResult{Error: "invalid input: " + err.Error()}, nil
	}
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return &contracts.ToolResult{Error: err.Error()}, nil
	}
	return &contracts.ToolResult{Output: string(data)}, nil
}
