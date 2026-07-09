// Package: mups/execute
//
// File: execute_trace_e2e_test.go
//
// 用途: Execute 节点 4 Channel 端到端 trace 验证脚本 (DM-20260708-005)
//
// 每个 test case 打印:
//   1. ChannelRequest (3 字段) + ToolRequest (5 字段) — 输入协议
//   2. ToolRunner 模拟行为 (canned handler, 记录 call 顺序)
//   3. 4 Channel 内部行为 (commit: 1 step sync; protocol: rollback; scenario: majority;
//      exploration: priority sort; mixed: timeout+ctx cancel)
//   4. *wavescheduler.Artifact 完整字段 (TaskID/Kind/SideEffectStatus/ExitCode 等) — 输出协议
//   5. SentinelError + EXEC_CHANNEL_9001-9007 错误码 (如适用)
//
// 用法:
//   go test -v -run TestExecuteTraceE2E \
//     ./internal/layers/orchestration/mups/execute/...
//
// 关键事实 (与 Observe/Plan 节点的根本区别):
//   - Execute↔ToolRunner 协议 ≠ Execute↔LLM 协议
//   - Execute 节点不直接调 LLM, 通过 4 Channel 派发 plan.Step 到 pluggable ToolRunner
//   - 4 Channel I/O 契约: ChannelRequest (3 字段) + ToolRequest (5 字段) → Artifact
//   - 4 Channel 输出差异: ArtifactKind (4 态) + SideEffectStatus (5 态) + WorkerType (3 态)
//
// 每个 test 的注释同时充当字段意图文档 — 跑 -v 输出 + 阅读源码 = 完整理解
// D7 Execute 节点 4 Channel 协议契约.
package execute

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// =============================================================================
// 测试 fixture: 扩展 fakeRunner 增 trace 打印
// =============================================================================

// traceRunner extends fakeRunner with structured call logging. Each Invoke
// appends a call record (tool, args snapshot, idempotency key, rollback flag)
// to calls. Used by every trace test to verify the Channel's per-step behavior.
type traceRunner struct {
	*fakeRunner
	mu    sync.Mutex
	calls []traceCall
}

type traceCall struct {
	tool         string
	args         map[string]any
	idemKey      string
	stepID       string
	rollbackHint bool
}

func newTraceRunner() *traceRunner {
	return &traceRunner{fakeRunner: newFakeRunner()}
}

func (r *traceRunner) record(req ToolRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, traceCall{
		tool:         req.ToolName,
		args:         copyArgs(req.Args),
		idemKey:      req.IdempotencyKey,
		stepID:       req.StepID,
		rollbackHint: req.Args["__rollback"] == true,
	})
}

// callSnapshot returns a copy of the calls slice (safe for iteration after
// the Channel returns). The copy is shallow on the slice but each traceCall
// already has its own copy of args (see copyArgs).
func (r *traceRunner) callSnapshot() []traceCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]traceCall, len(r.calls))
	copy(out, r.calls)
	return out
}

func copyArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Override Invoke to record before delegating to the underlying handler.
func (r *traceRunner) Invoke(ctx context.Context, req ToolRequest) (ToolResult, error) {
	r.record(req)
	return r.fakeRunner.Invoke(ctx, req)
}

// traceValidPlan builds a plan.Plan with the given kind + steps. Mirrors
// validPlan() in execute_test.go but with explicit Strength / SessionID
// fields (used by trace tests for assertion clarity).
func traceValidPlan(kind plan.PlanKind, sessionID string, persistScope plan.PersistScope, steps ...plan.Step) *plan.Plan {
	if steps == nil {
		steps = []plan.Step{}
	}
	p := plan.NewPlan("plan_trace_"+kind.String(), sessionID, kind,
		[]string{"obs_1"}, steps, 0.85).
		WithBlastRadius(plan.BlastRadius{
			FileCount: 1, APICallCount: len(steps), TokenCost: 100,
			PersistScope: persistScope,
		}).
		WithAnomaliesCount(0)
	return &p
}

// printExecuteBanner prints a single section banner so -v output is scannable.
func printExecuteBanner(t *testing.T, title string) {
	t.Helper()
	t.Logf("\n%s\n%s\n%s\n", strings.Repeat("=", 80), title, strings.Repeat("=", 80))
}

// printExecuteInput dumps the ChannelRequest + Plan (input protocol).
func printExecuteInput(t *testing.T, chName string, req ChannelRequest, p *plan.Plan) {
	t.Helper()
	t.Logf("─── 输入协议: ChannelRequest → %s ───", chName)
	t.Logf("  ChannelRequest.SessionID=%q", req.SessionID)
	t.Logf("  ChannelRequest.PriorVerdictKinds=%v", req.PriorVerdictKinds)
	t.Logf("  ChannelRequest.Spec=%v (nil = legacy)", req.Spec)
	t.Logf("  Plan.Kind=%q (4 PlanKind enum, 1:1 → Channel)", p.Kind)
	t.Logf("  Plan.Steps=%d BlastRadius.PersistScope=%q Strength=%.2f",
		len(p.Steps), p.BlastRadius.PersistScope, p.Strength)
	for i, s := range p.Steps {
		t.Logf("    Step[%d] id=%q tool=%q idem=%q estimated_tokens=%d",
			i, s.ID, s.ToolName, s.IdempotencyKey, s.EstimatedTokens)
	}
}

// printExecuteOutput dumps the *wavescheduler.Artifact + error (output protocol).
func printExecuteOutput(t *testing.T, art *wavescheduler.Artifact, err error) {
	t.Helper()
	t.Logf("─── 输出协议: *wavescheduler.Artifact ───")
	if art == nil {
		t.Logf("  Artifact: <nil>")
	} else {
		t.Logf("  TaskID=%q Kind=%q", art.TaskID, art.Kind)
		t.Logf("  SessionID=%q SourcePlanID=%q WorkerType=%q",
			art.SessionID, art.SourcePlanID, art.WorkerType)
		t.Logf("  ExitCode=%d Duration=%v", art.ExitCode, art.Duration)
		t.Logf("  SideEffectStatus=%q", art.SideEffectStatus)
		t.Logf("  SideEffectDetail.IdempotencyKey=%q SentAt=%d ConfirmedAt=%d",
			sideEffectKey(art), sideEffectSentAt(art), sideEffectConfirmedAt(art))
		if art.Summary != "" {
			t.Logf("  Summary=%q", truncForLog(art.Summary, 120))
		}
		if art.Error != "" {
			t.Logf("  Error=%q", truncForLog(art.Error, 120))
		}
	}
	if err != nil {
		t.Logf("  err=%v (sentinel=%T)", err, unwrapSentinel(err))
	}
}

func sideEffectKey(art *wavescheduler.Artifact) string {
	if art == nil || art.SideEffectDetail == nil {
		return ""
	}
	return art.SideEffectDetail.IdempotencyKey
}
func sideEffectSentAt(art *wavescheduler.Artifact) int64 {
	if art == nil || art.SideEffectDetail == nil {
		return 0
	}
	return art.SideEffectDetail.SentAt
}
func sideEffectConfirmedAt(art *wavescheduler.Artifact) int64 {
	if art == nil || art.SideEffectDetail == nil {
		return 0
	}
	return art.SideEffectDetail.ConfirmedAt
}
func unwrapSentinel(err error) error {
	// sharederrors.SentinelError wraps with errors.Join; for log we just print the type.
	var se interface{ Code() string }
	if errors.As(err, &se) {
		return err
	}
	return err
}

// extractSentinelCode returns the EXEC_CHANNEL_9001-style wire code from a
// sharederrors.SentinelError, or "" if not a SentinelError. The code lives
// in the .Code field (not in .Error() / .Message) so a string-match against
// err.Error() would always fail.
func extractSentinelCode(err error) string {
	var se *sharederrors.SentinelError
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// =============================================================================
// 测试 1: 场景 1 — CommitChannel 1 step 成功 → ArtifactStateChangeCert
//   完整链路: ChannelRouter.Route → CommitChannel.Execute → ToolRunner.Invoke
//   → SideEffectCommitted + SideEffectDetail(IdempotencyKey, SentAt, ConfirmedAt)
// =============================================================================

func TestExecuteTraceE2E_Commit_Success(t *testing.T) {
	runner := newTraceRunner()
	runner.OnInvoke("shell", func(req ToolRequest) (ToolResult, error) {
		return okResult("shell"), nil
	})
	ch, err := NewCommitChannel(runner, CommitChannelConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewCommitChannel: %v", err)
	}

	step := plan.Step{
		ID: "step_deploy_001", Directive: "deploy build v2.3.0",
		ToolName: "shell", ToolArgs: map[string]any{"cmd": "kubectl apply -f deploy.yaml"},
		IdempotencyKey: "deploy:v2.3.0:prod", EstimatedTokens: 100,
	}
	p := traceValidPlan(plan.CommitmentPlan, "sess_trace_commit", plan.PersistSession, step)
	req := ChannelRequest{SessionID: "sess_trace_commit"}

	printExecuteBanner(t, "TestExecuteTraceE2E_Commit_Success — 场景 1 (1 step ok → ArtifactStateChangeCert + SideEffectCommitted)")
	printExecuteInput(t, "CommitChannel", req, p)
	t.Logf("\n─── ToolRunner.Invoke 调用链 ───")
	t.Logf("  预期: 1 次 shell 调用, IdempotencyKey=%q, 成功 → ExitCode=0", step.IdempotencyKey)

	art, err := ch.Execute(context.Background(), p, req)
	if err != nil {
		t.Fatalf("CommitChannel.Execute: %v", err)
	}
	calls := runner.callSnapshot()
	printExecuteOutput(t, art, err)
	t.Logf("\n─── traceRunner.calls (实际 Invoke 顺序) ───")
	for i, c := range calls {
		t.Logf("  [%d] tool=%q idem=%q rollback=%v", i, c.tool, c.idemKey, c.rollbackHint)
	}

	// ── 关键断言 ──
	if len(calls) != 1 {
		t.Errorf("❌ ToolRunner.Invoke 调用次数=%d, want 1", len(calls))
	} else if calls[0].idemKey != step.IdempotencyKey {
		t.Errorf("❌ IdempotencyKey=%q, want %q", calls[0].idemKey, step.IdempotencyKey)
	} else {
		t.Logf("  ✓ ToolRunner.Invoke 1 次, IdempotencyKey 透传正确")
	}
	if art.Kind != types.ArtifactStateChangeCert {
		t.Errorf("❌ Kind=%q, want ArtifactStateChangeCert", art.Kind)
	} else {
		t.Logf("  ✓ Kind=ArtifactStateChangeCert (1:1 mapping CommitmentPlan → CommitChannel)")
	}
	if art.SideEffectStatus != types.SideEffectCommitted {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectCommitted", art.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectCommitted (terminal=true, NeedsAttention=false)")
	}
	if art.WorkerType != wavescheduler.WorkerCursor {
		t.Errorf("❌ WorkerType=%q, want cursor", art.WorkerType)
	} else {
		t.Logf("  ✓ WorkerType=cursor (CommitChannel 硬编码)")
	}
	if art.SideEffectDetail == nil {
		t.Error("❌ SideEffectDetail 应非 nil (IdempotencyKey + 时间戳)")
	} else if art.SideEffectDetail.IdempotencyKey != step.IdempotencyKey {
		t.Errorf("❌ SideEffectDetail.IdempotencyKey=%q, want %q",
			art.SideEffectDetail.IdempotencyKey, step.IdempotencyKey)
	} else if art.SideEffectDetail.ConfirmedAt == 0 {
		t.Error("❌ SideEffectDetail.ConfirmedAt 应 > 0 (commit 成功时已确认)")
	} else {
		t.Logf("  ✓ SideEffectDetail{IdempotencyKey=%q, ConfirmedAt=%d} 完整",
			art.SideEffectDetail.IdempotencyKey, art.SideEffectDetail.ConfirmedAt)
	}
}

// =============================================================================
// 测试 2: 场景 2 — ProtocolChannel 3 steps + step 2 失败 → rollback
//   完整链路: 顺序执行 step_1 + step_2(fail) → rollback step_1 → SideEffectRolledBack
//   关键: rollback 用 context.Background() + __rollback=true 提示 + :rollback 派生 key
// =============================================================================

func TestExecuteTraceE2E_Protocol_RollbackSuccess(t *testing.T) {
	runner := newTraceRunner()
	// step_1 login: forward + rollback 都成功
	runner.OnInvoke("login", func(req ToolRequest) (ToolResult, error) {
		if req.Args["__rollback"] == true {
			return okResult("login:rollback"), nil
		}
		return okResult("login"), nil
	})
	// step_2 fetch: forward 失败, 不需要 rollback handler
	runner.OnInvoke("fetch", func(req ToolRequest) (ToolResult, error) {
		return ToolResult{}, fmt.Errorf("network error: fetch failed")
	})
	// step_3 parse: 永远走不到, 但 handler 必填
	runner.OnInvoke("parse", func(req ToolRequest) (ToolResult, error) {
		return okResult("parse"), nil
	})

	ch, err := NewProtocolChannel(runner, ProtocolChannelConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewProtocolChannel: %v", err)
	}

	steps := []plan.Step{
		{ID: "step_login", Directive: "login", ToolName: "login", IdempotencyKey: "k_login", EstimatedTokens: 50},
		{ID: "step_fetch", Directive: "fetch data", ToolName: "fetch", IdempotencyKey: "k_fetch", EstimatedTokens: 100},
		{ID: "step_parse", Directive: "parse", ToolName: "parse", IdempotencyKey: "k_parse", EstimatedTokens: 50},
	}
	p := traceValidPlan(plan.ProtocolPlan, "sess_trace_proto", plan.PersistSession, steps...)
	req := ChannelRequest{SessionID: "sess_trace_proto"}

	printExecuteBanner(t, "TestExecuteTraceE2E_Protocol_RollbackSuccess — 场景 2 (3 steps + step 2 fail → rollback → SideEffectRolledBack)")
	printExecuteInput(t, "ProtocolChannel", req, p)
	t.Logf("\n─── ToolRunner.Invoke 预期调用链 (3 次) ───")
	t.Logf("  [1] login      (forward)  → ok")
	t.Logf("  [2] fetch      (forward)  → fail: network error")
	t.Logf("  [3] login      (rollback) → ok, __rollback=true, IdempotencyKey=k_login:rollback")

	art, err := ch.Execute(context.Background(), p, req)
	if err == nil {
		t.Fatal("期望 step 2 失败导致 error, 但 Execute 返回 nil")
	}
	calls := runner.callSnapshot()
	printExecuteOutput(t, art, err)
	t.Logf("\n─── traceRunner.calls (实际 Invoke 顺序) ───")
	for i, c := range calls {
		t.Logf("  [%d] tool=%q idem=%q step_id=%q rollback=%v",
			i, c.tool, c.idemKey, c.stepID, c.rollbackHint)
	}

	// ── 关键断言 ──
	if len(calls) != 3 {
		t.Errorf("❌ ToolRunner.Invoke 调用次数=%d, want 3 (login + fetch-fail + login-rollback)",
			len(calls))
	} else {
		t.Logf("  ✓ ToolRunner.Invoke 3 次 (forward login + fetch 失败 + login rollback)")
	}
	if calls[0].tool != "login" {
		t.Errorf("❌ calls[0]=%q, want 'login'", calls[0].tool)
	}
	if calls[1].tool != "fetch" {
		t.Errorf("❌ calls[1]=%q, want 'fetch'", calls[1].tool)
	}
	if calls[2].tool != "login" || !calls[2].rollbackHint {
		t.Errorf("❌ calls[2]=%q rollback=%v, want 'login' with rollback=true",
			calls[2].tool, calls[2].rollbackHint)
	} else {
		t.Logf("  ✓ calls[2] = login rollback (rollback hint 正确传递)")
	}
	if calls[2].idemKey != "k_login:rollback" {
		t.Errorf("❌ rollback IdempotencyKey=%q, want 'k_login:rollback'",
			calls[2].idemKey)
	} else {
		t.Logf("  ✓ rollback IdempotencyKey=%q (派生规则原 key + ':rollback')", calls[2].idemKey)
	}
	if art.Kind != types.ArtifactResponseRecord {
		t.Errorf("❌ Kind=%q, want ArtifactResponseRecord", art.Kind)
	} else {
		t.Logf("  ✓ Kind=ArtifactResponseRecord (1:1 mapping ProtocolPlan → ProtocolChannel)")
	}
	if art.SideEffectStatus != types.SideEffectRolledBack {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectRolledBack", art.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectRolledBack (terminal=true, rollback 成功)")
	}
	if art.WorkerType != wavescheduler.WorkerClaudeCode {
		t.Errorf("❌ WorkerType=%q, want claude_code", art.WorkerType)
	} else {
		t.Logf("  ✓ WorkerType=claude_code (ProtocolChannel 硬编码)")
	}
}

// =============================================================================
// 测试 3: 场景 3 — ScenarioChannel 5 并行探测, 3 通过 → majority vote
//   完整链路: 并行启动 5 probes → 3 success + 2 fail → success_count > 2 → pass
//   关键: SideEffect 永远是 None (read-only), ExitCode 表达成败
// =============================================================================

func TestExecuteTraceE2E_Scenario_MajorityPass(t *testing.T) {
	runner := newTraceRunner()
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	// 3 个成功 + 2 个失败
	successTools := []string{"probe_a", "probe_b", "probe_c"}
	for _, tool := range successTools {
		tool := tool
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			concurrent.Add(1)
			defer concurrent.Add(-1)
			if c := concurrent.Load(); c > maxConcurrent.Load() {
				maxConcurrent.Store(c)
			}
			time.Sleep(15 * time.Millisecond)
			return okResult(tool), nil
		})
	}
	failTools := []string{"probe_d", "probe_e"}
	for _, tool := range failTools {
		tool := tool
		runner.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			concurrent.Add(1)
			defer concurrent.Add(-1)
			if c := concurrent.Load(); c > maxConcurrent.Load() {
				maxConcurrent.Store(c)
			}
			time.Sleep(15 * time.Millisecond)
			return ToolResult{}, fmt.Errorf("probe %s failed", tool)
		})
	}

	ch, err := NewScenarioChannel(runner, ScenarioChannelConfig{MaxParallel: 5, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewScenarioChannel: %v", err)
	}

	steps := []plan.Step{
		{ID: "p_0", Directive: "probe_a", ToolName: "probe_a", IdempotencyKey: "i0"},
		{ID: "p_1", Directive: "probe_b", ToolName: "probe_b", IdempotencyKey: "i1"},
		{ID: "p_2", Directive: "probe_c", ToolName: "probe_c", IdempotencyKey: "i2"},
		{ID: "p_3", Directive: "probe_d", ToolName: "probe_d", IdempotencyKey: "i3"},
		{ID: "p_4", Directive: "probe_e", ToolName: "probe_e", IdempotencyKey: "i4"},
	}
	p := traceValidPlan(plan.ScenarioPlan, "sess_trace_probe", plan.PersistTransient, steps...)
	req := ChannelRequest{SessionID: "sess_trace_probe"}

	printExecuteBanner(t, "TestExecuteTraceE2E_Scenario_MajorityPass — 场景 3 (5 probes + 3 pass → majority → SideEffectNone)")
	printExecuteInput(t, "ScenarioChannel", req, p)
	t.Logf("\n─── ToolRunner.Invoke 预期 (5 probes 并行, MaxParallel=5) ───")
	t.Logf("  probe_a/b/c → ok; probe_d/e → fail")
	t.Logf("  majority: 3 > 5/2=2 → pass")

	art, err := ch.Execute(context.Background(), p, req)
	if err != nil {
		t.Fatalf("期望 3/5 pass → 无 error, 但 Execute 返回: %v", err)
	}
	calls := runner.callSnapshot()
	printExecuteOutput(t, art, err)
	t.Logf("\n─── traceRunner.calls (实际 Invoke 顺序) ───")
	for i, c := range calls {
		t.Logf("  [%d] tool=%q idem=%q", i, c.tool, c.idemKey)
	}
	t.Logf("\n─── 并发度指标 ───")
	t.Logf("  maxConcurrent=%d (期望 ≥ 2, 证明并行执行)", maxConcurrent.Load())

	// ── 关键断言 ──
	if len(calls) != 5 {
		t.Errorf("❌ ToolRunner.Invoke 调用次数=%d, want 5", len(calls))
	} else {
		t.Logf("  ✓ ToolRunner.Invoke 5 次 (5 probes 全跑)")
	}
	if maxConcurrent.Load() < 2 {
		t.Errorf("❌ maxConcurrent=%d, 期望 ≥ 2 (并行未生效)", maxConcurrent.Load())
	} else {
		t.Logf("  ✓ maxConcurrent=%d (并行执行, > 1 即通过)", maxConcurrent.Load())
	}
	if art.Kind != types.ArtifactProbeReport {
		t.Errorf("❌ Kind=%q, want ArtifactProbeReport", art.Kind)
	} else {
		t.Logf("  ✓ Kind=ArtifactProbeReport (1:1 mapping ScenarioPlan → ScenarioChannel)")
	}
	if art.SideEffectStatus != types.SideEffectNone {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectNone (read-only 不变)", art.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectNone (探测只读, 永远为 None, 与结果无关)")
	}
	if art.ExitCode != 0 {
		t.Errorf("❌ ExitCode=%d, want 0 (majority 3/5 pass)", art.ExitCode)
	} else {
		t.Logf("  ✓ ExitCode=0 (majority pass: 3 > 5/2=2)")
	}
	if !strings.Contains(art.Summary, "3/5 probes succeeded") {
		t.Errorf("❌ Summary=%q, 应包含 '3/5 probes succeeded'", art.Summary)
	} else {
		t.Logf("  ✓ Summary 包含 '3/5 probes succeeded' (vote count 正确)")
	}
	if art.WorkerType != wavescheduler.WorkerSubAgent {
		t.Errorf("❌ WorkerType=%q, want subagent", art.WorkerType)
	} else {
		t.Logf("  ✓ WorkerType=subagent (ScenarioChannel 硬编码)")
	}
}

// =============================================================================
// 测试 4: 场景 4 — ExplorationChannel 3 实验 + 优先级排序
//   完整链路: 3 experiments 并行 + 2 成功 + 1 失败 → sort by (success first, duration, EstimatedTokens)
//   关键: sideEffectForScope(PersistTransient) → SideEffectNone
// =============================================================================

func TestExecuteTraceE2E_Exploration_PartialSuccess(t *testing.T) {
	runner := newTraceRunner()
	// v1: 成功, slow (50ms), EstimatedTokens=5000
	runner.OnInvoke("exp_v1", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(50 * time.Millisecond)
		return okResult("exp_v1"), nil
	})
	// v2: 成功, fast (10ms), EstimatedTokens=3000 (应排第 1: success + 短 + token 少)
	runner.OnInvoke("exp_v2", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(10 * time.Millisecond)
		return okResult("exp_v2"), nil
	})
	// v3: 失败, EstimatedTokens=8000
	runner.OnInvoke("exp_v3", func(req ToolRequest) (ToolResult, error) {
		time.Sleep(20 * time.Millisecond)
		return ToolResult{}, fmt.Errorf("v3 impl has bug")
	})

	ch, err := NewExplorationChannel(runner, ExplorationChannelConfig{MaxParallel: 3, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewExplorationChannel: %v", err)
	}

	steps := []plan.Step{
		{ID: "e_0", Directive: "impl_v1 (LRU)", ToolName: "exp_v1", IdempotencyKey: "exp:v1", EstimatedTokens: 5000},
		{ID: "e_1", Directive: "impl_v2 (LFU)", ToolName: "exp_v2", IdempotencyKey: "exp:v2", EstimatedTokens: 3000},
		{ID: "e_2", Directive: "impl_v3 (ARC)", ToolName: "exp_v3", IdempotencyKey: "exp:v3", EstimatedTokens: 8000},
	}
	// 关键: PersistTransient (沙箱只读, 无副作用)
	p := traceValidPlan(plan.ExplorationPlan, "sess_trace_explore", plan.PersistTransient, steps...)
	req := ChannelRequest{SessionID: "sess_trace_explore"}

	printExecuteBanner(t, "TestExecuteTraceE2E_Exploration_PartialSuccess — 场景 4 (3 exp + 2 成功 + 1 失败 + 优先级排序 → sideEffectForScope)")
	printExecuteInput(t, "ExplorationChannel", req, p)
	t.Logf("\n─── ToolRunner.Invoke 预期 (3 exp 并行, MaxParallel=3) ───")
	t.Logf("  exp_v1: success, slow 50ms,  tokens=5000")
	t.Logf("  exp_v2: success, fast 10ms,  tokens=3000 (→ top_result)")
	t.Logf("  exp_v3: fail,    20ms,      tokens=8000")
	t.Logf("\n─── sideEffectForScope 映射 ───")
	t.Logf("  p.BlastRadius.PersistScope=%q → Artifact.SideEffectStatus=SideEffectNone",
		p.BlastRadius.PersistScope)

	art, err := ch.Execute(context.Background(), p, req)
	if err != nil {
		t.Fatalf("ExplorationChannel.Execute: %v", err)
	}
	calls := runner.callSnapshot()
	printExecuteOutput(t, art, err)
	t.Logf("\n─── traceRunner.calls (实际 Invoke 顺序) ───")
	for i, c := range calls {
		t.Logf("  [%d] tool=%q idem=%q", i, c.tool, c.idemKey)
	}

	// ── 关键断言 ──
	if len(calls) != 3 {
		t.Errorf("❌ ToolRunner.Invoke 调用次数=%d, want 3", len(calls))
	} else {
		t.Logf("  ✓ ToolRunner.Invoke 3 次 (3 experiments 全跑, 不短路)")
	}
	if art.Kind != types.ArtifactExperimentData {
		t.Errorf("❌ Kind=%q, want ArtifactExperimentData", art.Kind)
	} else {
		t.Logf("  ✓ Kind=ArtifactExperimentData (1:1 mapping ExplorationPlan → ExplorationChannel)")
	}
	if art.SideEffectStatus != types.SideEffectNone {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectNone (sideEffectForScope(PersistTransient))",
			art.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectNone (sideEffectForScope(PersistTransient) 正确映射)")
	}
	if !strings.Contains(art.Summary, "2/3 succeeded") {
		t.Errorf("❌ Summary=%q, 应包含 '2/3 succeeded'", art.Summary)
	} else {
		t.Logf("  ✓ Summary 包含 '2/3 succeeded' (统计正确)")
	}
	if !strings.Contains(art.Summary, "exp_v2") {
		t.Errorf("❌ Summary=%q, 应包含 'exp_v2' (top_result, 优先级排序第 1)",
			art.Summary)
	} else {
		t.Logf("  ✓ Summary 包含 'exp_v2' (top_result: 成功 + 短 + token 少 → 排第 1)")
	}
	if art.WorkerType != wavescheduler.WorkerSubAgent {
		t.Errorf("❌ WorkerType=%q, want subagent", art.WorkerType)
	} else {
		t.Logf("  ✓ WorkerType=subagent (ExplorationChannel 硬编码)")
	}
}

// =============================================================================
// 测试 5: 场景 5 (混合) — Commit timeout (EXEC_CHANNEL_9006) + Scenario ctx cancel (EXEC_CHANNEL_9007)
//   关键: 两个错误码 wire format 不混淆, StrategyDecider 据此走不同路径
//   - 9006: StrategyAskNow (侧效应状态不确定, 询问用户)
//   - 9007: StrategyCancel (turn abort, 取消)
// =============================================================================

func TestExecuteTraceE2E_CommitTimeout_Inflight(t *testing.T) {
	// ── 子场景 5a: CommitChannel timeout → SideEffectInflight (EXEC_CHANNEL_9006) ──
	runner := newTraceRunner()
	runner.OnInvoke("slow_tool", func(req ToolRequest) (ToolResult, error) {
		// 模拟 ToolRunner 在 Channel timeout 之前一直挂着, 直接返回
		// context.DeadlineExceeded 让 CommitChannel 走 timeout 分支。
		// (与 TestCommitChannel_Timeout_InflightSideEffect 一致)
		return ToolResult{ToolName: "slow_tool"}, context.DeadlineExceeded
	})
	ch, err := NewCommitChannel(runner, CommitChannelConfig{Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewCommitChannel: %v", err)
	}

	step := plan.Step{
		ID: "step_slow", Directive: "call slow tool", ToolName: "slow_tool",
		IdempotencyKey: "slow:1", EstimatedTokens: 100,
	}
	p := traceValidPlan(plan.CommitmentPlan, "sess_trace_timeout", plan.PersistSession, step)
	req := ChannelRequest{SessionID: "sess_trace_timeout"}

	printExecuteBanner(t, "TestExecuteTraceE2E_CommitTimeout_Inflight — 场景 5a (Commit 50ms timeout → SideEffectInflight + EXEC_CHANNEL_9006)")
	printExecuteInput(t, "CommitChannel", req, p)
	t.Logf("\n─── 预期 ───")
	t.Logf("  ToolRunner 阻塞 50ms, 触发 ctx.DeadlineExceeded")
	t.Logf("  → Artifact.SideEffectStatus=SideEffectInflight (terminal=false, NeedsAttention=true)")
	t.Logf("  → err = NewChannelToolCallTimedOutError = EXEC_CHANNEL_9006")
	t.Logf("  → StrategyDecider 应走 StrategyAskNow (询问用户)")

	art, err := ch.Execute(context.Background(), p, req)
	if err == nil {
		t.Fatal("期望 timeout error, 但 Execute 返回 nil")
	}
	printExecuteOutput(t, art, err)

	// ── 关键断言 ──
	if art.SideEffectStatus != types.SideEffectInflight {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectInflight", art.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectInflight (terminal=false, NeedsAttention=true)")
	}
	if !errors.Is(err, ErrChannelToolCallTimedOut) {
		t.Errorf("❌ err=%v, want errors.Is(ErrChannelToolCallTimedOut)", err)
	} else {
		t.Logf("  ✓ err=ErrChannelToolCallTimedOut (SentinelError)")
	}
	if code := extractSentinelCode(err); code != "EXEC_CHANNEL_9006" {
		t.Errorf("❌ err code=%q, want EXEC_CHANNEL_9006", code)
	} else {
		t.Logf("  ✓ wire format EXEC_CHANNEL_9006 (StrategyDecider 据此路由 StrategyAskNow)")
	}
	if art.SideEffectDetail == nil {
		t.Error("❌ SideEffectDetail 应非 nil (IdempotencyKey + SentAt 透传)")
	} else if art.SideEffectDetail.ConfirmedAt != 0 {
		t.Errorf("❌ SideEffectDetail.ConfirmedAt=%d, want 0 (inflight 未确认)",
			art.SideEffectDetail.ConfirmedAt)
	} else {
		t.Logf("  ✓ SideEffectDetail{IdempotencyKey=%q, SentAt=%d, ConfirmedAt=0} (inflight 状态)",
			art.SideEffectDetail.IdempotencyKey, art.SideEffectDetail.SentAt)
	}

	// ── 子场景 5b: ScenarioChannel ctx cancel → ErrChannelCtxCancelled (EXEC_CHANNEL_9007) ──
	t.Logf("\n%s\n", strings.Repeat("─", 80))
	t.Logf("子场景 5b: ScenarioChannel ctx cancel → EXEC_CHANNEL_9007 (StrategyCancel)")
	t.Logf("%s\n", strings.Repeat("─", 80))

	runner2 := newTraceRunner()
	for i := 0; i < 5; i++ {
		tool := fmt.Sprintf("ok_%d", i)
		runner2.OnInvoke(tool, func(req ToolRequest) (ToolResult, error) {
			// handler 立即返回, 但外层 ctx 在调用前 cancel
			return okResult(tool), nil
		})
	}
	sch, err := NewScenarioChannel(runner2, ScenarioChannelConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("NewScenarioChannel: %v", err)
	}

	steps := make([]plan.Step, 5)
	for i := range steps {
		steps[i] = plan.Step{
			ID: fmt.Sprintf("s%d", i+1), Directive: fmt.Sprintf("probe_%d", i),
			ToolName: fmt.Sprintf("ok_%d", i), IdempotencyKey: fmt.Sprintf("i%d", i),
		}
	}
	p2 := traceValidPlan(plan.ScenarioPlan, "sess_trace_cancel", plan.PersistTransient, steps...)
	req2 := ChannelRequest{SessionID: "sess_trace_cancel"}

	t.Logf("\n─── 输入协议: ChannelRequest → ScenarioChannel ───")
	t.Logf("  ctx=cancel() (调用前 cancel, 模拟 turn abort)")
	t.Logf("\n─── 预期 ───")
	t.Logf("  → err = NewChannelCtxCancelledError = EXEC_CHANNEL_9007")
	t.Logf("  → Artifact.SideEffectStatus=SideEffectUnknown (不是 None!)")
	t.Logf("  → 跳过 majority vote (RH-D7-09 fix: cancel 优先检查)")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 调用前 cancel
	art2, err2 := sch.Execute(ctx, p2, req2)
	if err2 == nil {
		t.Fatal("期望 ctx-cancelled error, 但 Execute 返回 nil")
	}
	printExecuteOutput(t, art2, err2)

	// ── 关键断言 ──
	if !errors.Is(err2, ErrChannelCtxCancelled) {
		t.Errorf("❌ err2=%v, want errors.Is(ErrChannelCtxCancelled)", err2)
	} else {
		t.Logf("  ✓ err2=ErrChannelCtxCancelled (SentinelError, RH-D7-09 fix)")
	}
	if code := extractSentinelCode(err2); code != "EXEC_CHANNEL_9007" {
		t.Errorf("❌ err2 code=%q, want EXEC_CHANNEL_9007", code)
	} else {
		t.Logf("  ✓ wire format EXEC_CHANNEL_9007 (StrategyDecider 据此路由 StrategyCancel)")
	}
	if art2.SideEffectStatus != types.SideEffectUnknown {
		t.Errorf("❌ SideEffectStatus=%q, want SideEffectUnknown (cancel 优先, 不是 None)",
			art2.SideEffectStatus)
	} else {
		t.Logf("  ✓ SideEffectUnknown (RH-D7-09: cancel 优先, 跳过 majority vote)")
	}
	if art2.Kind != types.ArtifactProbeReport {
		t.Errorf("❌ Kind=%q, want ArtifactProbeReport (即使 cancel 仍构造 Artifact)",
			art2.Kind)
	} else {
		t.Logf("  ✓ Kind=ArtifactProbeReport (cancel 也构造 Artifact 给 StrategyDecider)")
	}

	// ── 场景 5 综合断言: 两个错误码绝不混淆 ──
	if code := extractSentinelCode(err); code == "EXEC_CHANNEL_9007" {
		t.Error("❌ CommitChannel timeout 错误不应有 EXEC_CHANNEL_9007 (应只有 9006)")
	}
	if code := extractSentinelCode(err2); code == "EXEC_CHANNEL_9006" {
		t.Error("❌ Scenario ctx cancel 错误不应有 EXEC_CHANNEL_9006 (应只有 9007)")
	}
	t.Logf("\n─── 综合断言 ───")
	t.Logf("  ✓ EXEC_CHANNEL_9006 (timeout) 和 EXEC_CHANNEL_9007 (cancel) wire format 不混淆")
	t.Logf("  ✓ StrategyDecider 可据此区分: 9006 → AskNow, 9007 → Cancel")
}

// 防止 unused import wavescheduler (供 future 扩展使用)
var _ = wavescheduler.WorkerCursor
