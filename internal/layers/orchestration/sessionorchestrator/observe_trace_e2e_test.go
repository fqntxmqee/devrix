// Package: sessionorchestrator
//
// File: observe_trace_e2e_test.go
//
// 用途：Observe 节点端到端 trace 验证脚本 (DM-20260708-003)
//
// 每个 test case 打印：
//   1. LLM system prompt (D2 base + i18n observation appendix)
//   2. LLM user prompt (经过 observeLLMFieldMap 过滤的 6 字段 key:value frame)
//   3. LLM raw response (canned JSON, 用于模拟 LLM 真实输出)
//   4. 解析后 → 验证后 → 落 D7 UncertaintyReport 的全链路
//   5. 最终 Partition: BusinessObservations vs Anomalies
//
// 用法：
//   go test -v -run TestObserveTraceE2E \
//     ./internal/layers/orchestration/sessionorchestrator/...
//
// 关键事实（修正我之前的错误表述）：
//   - ObserveSignalInput 共 11 字段
//   - 但 observeLLMFieldMap 只 emit 6 字段到 LLM user prompt
//   - 另外 5 字段是 control-only: work_item_id, prior_mean, incremental_only,
//     prior_artifact_summary, known_gaps
//   - 这 5 字段在 D7 内部消费（work_item_id 进 evidence 兜底、prior_mean 给
//     confidence 校准、prior_artifact_summary 进 Plan frame delta），但不让
//     LLM 看见 — 防 LLM 误用 reputation 自我归因 / 防 LLM 看到"自己上轮 ID"
//     写出循环引用
//
// 每个 test 的注释同时充当字段意图文档 — 跑 -v 输出 + 阅读源码 = 完整理解
// D7 Observe 协议契约。
package sessionorchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// traceLLM captures every raw LLM request and returns a canned response.
// Mirrors stubObsLLM in llm_observation_proposer_test.go but exposes the
// full LLMInvokeRequest so trace tests can print system + messages verbatim.
type traceLLM struct {
	raw        string
	lastSystem string
	lastMsgs   []types.Message
	callCount  int
}

func (l *traceLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	l.callCount++
	l.lastSystem = req.SystemPrompt
	l.lastMsgs = append([]types.Message(nil), req.Messages...)
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: l.raw}
	close(ch)
	return ch, nil
}

// traceMUPS lets the test inject a deterministic D2 system base. The
// observation appendix is auto-appended by LLMObservationProposer.
type traceMUPS struct {
	systemBase string
}

func (m *traceMUPS) MaterializeForMUPS(_ context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	return contracts.MUPSPreparedContext{
		SystemPrompt:  m.systemBase,
		PhaseAppendix: i18n.ObservationTaskAppendix(i18n.ParseLanguage(req.Policy.Locale)),
	}, nil
}

// newTraceProposer wires a trace LLM + trace MUPS into a real
// LLMObservationProposer. Returns the proposer + the LLM (so the test can
// inspect lastSystem/lastMsgs after the call).
func newTraceProposer(t *testing.T, llmRaw, systemBase string, loc i18n.Locale) (*LLMObservationProposer, *traceLLM) {
	t.Helper()
	llm := &traceLLM{raw: llmRaw}
	mups := &traceMUPS{systemBase: systemBase}
	return NewLLMObservationProposer(llm, mups, loc), llm
}

// printBanner prints a single section banner so -v output is scannable.
func printBanner(t *testing.T, title string) {
	t.Helper()
	t.Logf("\n%s\n%s", strings.Repeat("=", 80), title)
	t.Logf("%s\n", strings.Repeat("=", 80))
}

// printSystemAndUser dumps what the LLM actually received. Comments
// inline are intentional — they double as field-intent documentation.
func printSystemAndUser(t *testing.T, llm *traceLLM) {
	t.Helper()
	t.Logf("─── LLM system prompt (SystemPrompt = D2 base + \\n\\n + i18n observation appendix) ───")
	t.Logf("%s", llm.lastSystem)
	t.Logf("\n─── LLM user prompt (after observeLLMFieldMap filter; only 6/11 fields shown to LLM) ───")
	for i, msg := range llm.lastMsgs {
		t.Logf("[message #%d role=%s]", i, msg.Role)
		t.Logf("%s", msg.Content)
	}
}

// printProposals dumps the post-validation Observation list with full
// Payload dump so the test reader sees the Go-side transformation.
func printProposals(t *testing.T, proposals []ObservationProposal) {
	t.Helper()
	if len(proposals) == 0 {
		t.Logf("─── LLM response → validated []ObservationProposal: <empty> ───")
		return
	}
	t.Logf("─── LLM response → validated []ObservationProposal (%d items) ───", len(proposals))
	for i, p := range proposals {
		t.Logf("  [%d] kind=%s category=%s strength=%.2f statement=%q question=%q evidence=%v",
			i, p.Kind, p.Category, p.Strength, truncForLog(p.Statement, 60), p.Question, p.Evidence)
	}
}

// printReportPartition runs NewUncertaintyReport + Partition so the test
// reader sees BusinessObservations / Anomalies routing.
func printReportPartition(t *testing.T, sessionID string, observations []orchtypes.Observation) {
	t.Helper()
	if len(observations) == 0 {
		t.Logf("─── UncertaintyReport partition: <no observations> ───")
		return
	}
	rep, err := orchtypes.NewUncertaintyReport(sessionID, observations)
	if err != nil {
		t.Fatalf("NewUncertaintyReport: %v", err)
	}
	if err := rep.Partition(); err != nil {
		t.Logf("Partition err: %v (non-fatal for trace)", err)
		return
	}
	t.Logf("─── UncertaintyReport partition ───")
	t.Logf("  Overall (mean of business strengths) = %.3f", rep.Overall)
	t.Logf("  BusinessObservations (%d):", len(rep.BusinessObservations))
	for i, o := range rep.BusinessObservations {
		t.Logf("    [%d] %s str=%.2f cat=business payload=%s",
			i, o.Kind, o.Strength, payloadSummary(o))
	}
	t.Logf("  Anomalies (%d) [CatSystem+ObsDeviation OR CatSystem+ObsUncertainty≥0.7]:", len(rep.Anomalies))
	for i, o := range rep.Anomalies {
		t.Logf("    [%d] %s str=%.2f cat=system payload=%s",
			i, o.Kind, o.Strength, payloadSummary(o))
	}
}

func payloadSummary(o orchtypes.Observation) string {
	switch p := o.Payload.(type) {
	case orchtypes.FactPayload:
		return fmt.Sprintf("FactPayload{Statement:%q Evidence:%v}", truncForLog(p.Statement, 40), p.Evidence)
	case orchtypes.SignalPayload:
		return fmt.Sprintf("SignalPayload{Name:%q Value:%.2f Threshold:%.2f}", p.Name, p.Value, p.Threshold)
	case orchtypes.DeviationPayload:
		return fmt.Sprintf("DeviationPayload{Metric:%q Observed:%.2f Delta:%.2f}", p.Metric, p.Observed, p.Delta)
	case orchtypes.UncertaintyPayload:
		return fmt.Sprintf("UncertaintyPayload{Question:%q Confidence:%.2f RequiresMore:%v}",
			truncForLog(p.Question, 40), p.Confidence, p.RequiresMore)
	default:
		return fmt.Sprintf("<unknown %T>", p)
	}
}

func truncForLog(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// =============================================================================
// 测试 1: render-only — 展示 11 字段在 observeLLMFieldMap 过滤后
//   实际只有 6 字段进入 LLM user prompt,另外 5 是 Go-only control fields
// =============================================================================

func TestObserveTraceE2E_OnlyFieldsVisibleToLLM(t *testing.T) {
	// 准备一个"全字段都填"的 ObserveSignalInput
	full := ObserveSignalInput{
		SessionID:           "sess_trace_allfields",
		WorkItemID:          "wi_trace_001",
		Directive:           "review d7 plan 目录",
		PriorParseReject:    "上一轮 strength 越界 1.0",
		PriorMean:           0.62,
		ScopeGoal:           "review d7 编排层",
		ScopeOpenQuestions:  []string{"是否包括 plan 子包?", "test 覆盖到 plan/ 吗?"},
		InboundSignalLines:  []string{"artifact_summary: 之前的 attempt 失败", "child_downlink_scope_in: d7/plan/"},
		PriorObservationIDs: []string{"obs_1", "obs_2"},
		IncrementalOnly:     true,
		PriorArtifactSummary: "上轮 Execute 收敛于 root rollup 合成",
		KnownGaps:           []string{"gap.d7.plan_sub_coverage"},
	}

	// 直接调 buildLLMObservationUserPrompt (bypass LLM)
	rendered := buildLLMObservationUserPrompt(full, i18n.LocaleZH)

	printBanner(t, "TestObserveTraceE2E_OnlyFieldsVisibleToLLM — 11→6 字段过滤 trace")
	t.Logf("─── 输入: ObserveSignalInput (11 字段全填) ───")
	t.Logf("  WorkItemID=%q Directive=%q", full.WorkItemID, full.Directive)
	t.Logf("  PriorParseReject=%q PriorMean=%.2f", full.PriorParseReject, full.PriorMean)
	t.Logf("  ScopeGoal=%q ScopeOpenQuestions=%v", full.ScopeGoal, full.ScopeOpenQuestions)
	t.Logf("  InboundSignalLines=%v", full.InboundSignalLines)
	t.Logf("  PriorObservationIDs=%v IncrementalOnly=%v", full.PriorObservationIDs, full.IncrementalOnly)
	t.Logf("  PriorArtifactSummary=%q KnownGaps=%v", full.PriorArtifactSummary, full.KnownGaps)

	t.Logf("\n─── LLM user prompt (实际渲染) ───")
	t.Logf("%s", rendered)

	// ── 断言 LLM 实际看到的 6 字段 ──
	expectedInPrompt := []string{
		"directive", "prior_parse_reject", "scope_goal",
		"scope_open_question", "signal", "prior_observation_ids",
	}
	for _, k := range expectedInPrompt {
		if !strings.Contains(rendered, k) {
			t.Errorf("❌ 字段 %q 应该出现在 LLM user prompt 中,但缺失", k)
		} else {
			t.Logf("  ✓ 字段 %q 出现在 prompt", k)
		}
	}

	// ── 断言 LLM 看不到的 5 control 字段 ──
	notInPrompt := []string{
		"work_item_id",        // Go 端用做 evidence 兜底
		"prior_mean",          // Go 端用做 confidence 校准
		"incremental_only",    // bool, structbind 跳过
		"prior_artifact_summary", // Phase 2 T8, 喂 Plan 不喂 Obs
		"known_gaps",          // Phase 2 stub = [], 永远不 emit
	}
	for _, k := range notInPrompt {
		if strings.Contains(rendered, k) {
			t.Errorf("❌ control 字段 %q 不应出现在 LLM user prompt,但出现了", k)
		} else {
			t.Logf("  ✓ control 字段 %q 被正确过滤", k)
		}
	}

	t.Logf("\n─── 字段意图速查 ───")
	t.Logf("  ┌──────────────────────┬────────┬─────────────────────────────────────────┐")
	t.Logf("  │ 字段                  │ 给 LLM │ 用途                                     │")
	t.Logf("  ├──────────────────────┼────────┼─────────────────────────────────────────┤")
	t.Logf("  │ work_item_id         │  ✗    │ Go 用: 进 evidence 兜底                  │")
	t.Logf("  │ directive            │  ✓    │ 主信号, 用户原话                          │")
	t.Logf("  │ prior_parse_reject   │  ✓    │ 上一轮 parse 失败原因 (LLM 自纠)         │")
	t.Logf("  │ prior_mean           │  ✗    │ Bayesian 信誉, Go 算 confidence 校准     │")
	t.Logf("  │ scope_goal           │  ✓    │ scope 收缩目标                            │")
	t.Logf("  │ scope_open_question  │  ✓    │ 待闭合的 OpenQuestion, LLM 看是否还能答  │")
	t.Logf("  │ signal               │  ✓    │ 多行结构化输入 (artifact_summary, etc)  │")
	t.Logf("  │ prior_observation_ids│  ✓    │ 跨轮去重, 避免 LLM 重复提相同 obs       │")
	t.Logf("  │ incremental_only     │  ✗    │ bool 标志, structbind 跳过                │")
	t.Logf("  │ prior_artifact_summary│ ✗    │ Phase 2 T8, 喂 Plan frame delta          │")
	t.Logf("  │ known_gaps           │  ✗    │ Phase 2 stub = [], 永远不 emit           │")
	t.Logf("  └──────────────────────┴────────┴─────────────────────────────────────────┘")
}

// =============================================================================
// 测试 2: ObsFact — 确定性问答 → fast-path 命中
// =============================================================================

func TestObserveTraceE2E_ObsFact_FastPathTrigger(t *testing.T) {
	// LLM canned response: 高 strength CatBusiness ObsFact
	llmRaw := `[{"kind":"obs_fact","strength":0.95,"statement":"在标准算术下，2×3=6。","question":"","evidence":[]}]`
	// 故意把 strength 写成 0.95 — 验证 Go 端 cap 到 0.85

	proposer, llm := newTraceProposer(t, llmRaw, "你是 Devrix 助手。", i18n.LocaleZH)
	in := ObserveSignalInput{
		SessionID:  "sess_trace_fact",
		WorkItemID: "wi_fact_001",
		Directive:  "2×3=几?",
		PriorMean:  0.5,
	}

	printBanner(t, "TestObserveTraceE2E_ObsFact_FastPathTrigger — deterministic Q&A fast-path")
	printSystemAndUser(t, llm)
	t.Logf("\n─── LLM raw response (canned) ───")
	t.Logf("%s", llmRaw)

	proposals, err := proposer.ProposeObservations(context.Background(), in)
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	printProposals(t, proposals)

	// Convert to []orchtypes.Observation so we can run Partition
	obs, _ := ValidateObservationProposals(proposals, in.SessionID, in.WorkItemID)
	printReportPartition(t, in.SessionID, obs)

	// ── 关键断言 ──
	if len(proposals) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(proposals))
	}
	p := proposals[0]
	if p.Kind != orchtypes.ObsFact {
		t.Errorf("proposal kind = %s, want ObsFact", p.Kind)
	}
	if p.Category != orchtypes.CatBusiness {
		t.Errorf("proposal category = %s, want CatBusiness (LLM mock proposals forced to business)", p.Category)
	}
	// 关键: validateOneProposal 把 LLM 0.95 cap 到 0.85 + 注入 evidence
	if len(obs) != 1 {
		t.Fatalf("expected 1 obs, got %d", len(obs))
	}
	o := obs[0]
	if o.Strength != 0.85 {
		t.Errorf("obs strength = %.2f, want 0.85 (Go cap from LLM-claimed 0.95)", o.Strength)
	}
	// Evidence 应包含 work_item_id + sessionID (Go 兜底) — evidence 在 FactPayload 上
	fp, ok := o.Payload.(orchtypes.FactPayload)
	if !ok {
		t.Fatalf("payload type = %T, want FactPayload", o.Payload)
	}
	wantEvidence := map[string]bool{in.WorkItemID: false, in.SessionID: false}
	for _, e := range fp.Evidence {
		if _, ok := wantEvidence[e]; ok {
			wantEvidence[e] = true
		}
	}
	for k, found := range wantEvidence {
		if !found {
			t.Errorf("evidence missing required ID %q (Go auto-append)", k)
		}
	}

	// fast-path gate 验证: pickHighStrengthBusinessFact(0.85) 应命中
	rep, _ := orchtypes.NewUncertaintyReport(in.SessionID, obs)
	_, stmt, ok := pickHighStrengthBusinessFact(rep, 0.85)
	if !ok {
		t.Errorf("❌ fast-path gate FAIL: pickHighStrengthBusinessFact(rep, 0.85) = (_, _, false)")
	} else {
		t.Logf("  ✓ fast-path gate HIT: pickHighStrengthBusinessFact(rep, 0.85) → stmt=%q", stmt)
	}
}

// =============================================================================
// 测试 3: ObsUncertainty — 模糊 directive → 走 Plan 追问
// =============================================================================

func TestObserveTraceE2E_ObsUncertainty_PlanDecompose(t *testing.T) {
	llmRaw := `[{"kind":"obs_uncertainty","strength":0.7,"statement":"","question":"优化什么指标? (性能/UX/代码结构?)","evidence":[]}]`

	proposer, llm := newTraceProposer(t, llmRaw, "你是 Devrix 助手。", i18n.LocaleZH)
	in := ObserveSignalInput{
		SessionID:          "sess_trace_uncertainty",
		WorkItemID:         "wi_u_001",
		Directive:          "帮我优化一下",
		ScopeOpenQuestions: []string{"优化哪个模块?", "优化什么指标?"},
	}

	printBanner(t, "TestObserveTraceE2E_ObsUncertainty_PlanDecompose — scope unclear")
	printSystemAndUser(t, llm)
	t.Logf("\n─── LLM raw response (canned) ───")
	t.Logf("%s", llmRaw)

	proposals, err := proposer.ProposeObservations(context.Background(), in)
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	printProposals(t, proposals)

	obs, _ := ValidateObservationProposals(proposals, in.SessionID, in.WorkItemID)
	printReportPartition(t, in.SessionID, obs)

	// ── 关键断言 ──
	if len(proposals) != 1 || proposals[0].Kind != orchtypes.ObsUncertainty {
		t.Fatalf("expected 1 ObsUncertainty, got %+v", proposals)
	}
	p := proposals[0]
	// UncertaintyPayload.Confidence = 1 - strength = 0.3
	if p.Statement != "" && p.Question == "" {
		t.Errorf("ObsUncertainty: question should fall back to statement, but both empty")
	}

	// hasObsUncertainty gate 验证: should return true (blocks fast-path)
	rep, _ := orchtypes.NewUncertaintyReport(in.SessionID, obs)
	if !hasObsUncertainty(rep) {
		t.Errorf("❌ hasObsUncertainty FAIL: returned false, but ObsUncertainty present")
	} else {
		t.Logf("  ✓ hasObsUncertainty HIT → fast-path BLOCKED, Plan 节点会 decompose")
	}

	// pickHighStrengthBusinessFact 不应命中 (no ObsFact)
	if _, _, ok := pickHighStrengthBusinessFact(rep, 0.85); ok {
		t.Errorf("❌ fast-path gate HIT (unexpected): no ObsFact, gate should be closed")
	}
}

// =============================================================================
// 测试 4: ObsSignal — 结构化 signal 摘要
// =============================================================================

func TestObserveTraceE2E_ObsSignal_StructuredMetric(t *testing.T) {
	llmRaw := `[{"kind":"obs_signal","strength":0.6,"statement":"重复 attempt","question":"","evidence":[]}]`

	proposer, llm := newTraceProposer(t, llmRaw, "你是 Devrix 助手。", i18n.LocaleZH)
	in := ObserveSignalInput{
		SessionID:          "sess_trace_signal",
		WorkItemID:         "wi_sig_001",
		Directive:          "再试一次",
		InboundSignalLines: []string{"artifact_summary: connection refused (3rd retry)"},
	}

	printBanner(t, "TestObserveTraceE2E_ObsSignal_StructuredMetric — repeat attempt signal")
	printSystemAndUser(t, llm)
	t.Logf("\n─── LLM raw response (canned) ───")
	t.Logf("%s", llmRaw)

	proposals, err := proposer.ProposeObservations(context.Background(), in)
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	printProposals(t, proposals)

	obs, _ := ValidateObservationProposals(proposals, in.SessionID, in.WorkItemID)
	printReportPartition(t, in.SessionID, obs)

	// ── 关键断言 ──
	if proposals[0].Kind != orchtypes.ObsSignal {
		t.Fatalf("kind = %s, want ObsSignal", proposals[0].Kind)
	}
	// SignalPayload: Name=Statement, Value=strength
	if sig, ok := obs[0].Payload.(orchtypes.SignalPayload); ok {
		if sig.Name != "重复 attempt" {
			t.Errorf("SignalPayload.Name = %q, want %q", sig.Name, "重复 attempt")
		}
		if sig.Value != 0.6 {
			t.Errorf("SignalPayload.Value = %.2f, want 0.6", sig.Value)
		}
		if sig.Threshold != 0.5 {
			t.Errorf("SignalPayload.Threshold = %.2f, want 0.5 (hardcoded default)", sig.Threshold)
		}
		t.Logf("  ✓ SignalPayload{Name, Value=strength, Threshold=0.5} confirmed")
	} else {
		t.Errorf("payload type = %T, want SignalPayload", obs[0].Payload)
	}
}

// =============================================================================
// 测试 5: ObsDeviation + CatSystem → Anomalies 触发
// =============================================================================

func TestObserveTraceE2E_ObsDeviation_AnomalyTrigger(t *testing.T) {
	// 关键: LLM JSON 不指定 category,Go 强制设 CatBusiness
	// 这里我们手改 category 为 CatSystem 来演示 anomaly 路由
	llmRaw := `[{"kind":"obs_deviation","strength":0.9,"statement":"P99 latency","question":"","evidence":[]}]`

	proposer, llm := newTraceProposer(t, llmRaw, "你是 Devrix 助手。", i18n.LocaleZH)
	in := ObserveSignalInput{
		SessionID:          "sess_trace_deviation",
		WorkItemID:         "wi_dev_001",
		Directive:          "检查 API 性能",
		InboundSignalLines: []string{"artifact_summary: P99 latency 850ms (baseline 200ms)"},
	}

	printBanner(t, "TestObserveTraceE2E_ObsDeviation_AnomalyTrigger — metric delta → anomaly")
	printSystemAndUser(t, llm)
	t.Logf("\n─── LLM raw response (canned) ───")
	t.Logf("%s", llmRaw)

	proposals, err := proposer.ProposeObservations(context.Background(), in)
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	// 关键 hack: 模拟"调用方后续手动改 Category 为 CatSystem"
	// (真实生产路径是某个上游 detector 改的,例如 DetectAnomalies)
	for i := range proposals {
		proposals[i].Category = orchtypes.CatSystem
	}
	printProposals(t, proposals)

	obs, _ := ValidateObservationProposals(proposals, in.SessionID, in.WorkItemID)
	printReportPartition(t, in.SessionID, obs)

	// ── 关键断言 ──
	if len(obs) == 0 {
		t.Fatal("expected 1 obs")
	}
	if obs[0].Category != orchtypes.CatSystem {
		t.Errorf("category = %s, want CatSystem", obs[0].Category)
	}
	// 跑 Partition, 确认 ObsDeviation + CatSystem → Anomalies
	rep, _ := orchtypes.NewUncertaintyReport(in.SessionID, obs)
	if err := rep.Partition(); err != nil {
		t.Fatalf("Partition: %v", err)
	}
	if len(rep.Anomalies) != 1 {
		t.Errorf("Anomalies count = %d, want 1 (CatSystem+ObsDeviation always routes here)", len(rep.Anomalies))
	} else {
		t.Logf("  ✓ CatSystem+ObsDeviation → Anomalies (%d entry)", len(rep.Anomalies))
	}
	if len(rep.BusinessObservations) != 0 {
		t.Errorf("BusinessObservations count = %d, want 0 (CatSystem excluded from business path)", len(rep.BusinessObservations))
	}
}

// =============================================================================
// 测试 6: Strength Clamping — LLM 自评 0.99 / 0.5 / 0.0 三种边界
// =============================================================================

func TestObserveTraceE2E_StrengthClamping(t *testing.T) {
	cases := []struct {
		name       string
		rawJSON    string
		wantCap    float64
		wantFloor  bool // true = should be lifted to 0.5
	}{
		{
			name:    "ObsFact 0.99 → cap 0.85",
			rawJSON: `[{"kind":"obs_fact","strength":0.99,"statement":"x","question":"","evidence":[]}]`,
			wantCap: 0.85,
		},
		{
			name:    "ObsFact 0.85 → no cap (already at limit)",
			rawJSON: `[{"kind":"obs_fact","strength":0.85,"statement":"x","question":"","evidence":[]}]`,
			wantCap: 0.85,
		},
		{
			name:    "ObsFact 0.50 → no cap",
			rawJSON: `[{"kind":"obs_fact","strength":0.5,"statement":"x","question":"","evidence":[]}]`,
			wantCap: 0.50,
		},
		{
			name:    "ObsFact 0.0 → lifted to 0.5 (zero protection)",
			rawJSON: `[{"kind":"obs_fact","strength":0.0,"statement":"x","question":"","evidence":[]}]`,
			wantCap: 0.50, wantFloor: true,
		},
		{
			name:    "ObsUncertainty 0.99 → NO cap (only ObsFact has cap)",
			rawJSON: `[{"kind":"obs_uncertainty","strength":0.99,"question":"q","evidence":[]}]`,
			wantCap: 0.99,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposer, _ := newTraceProposer(t, tc.rawJSON, "base", i18n.LocaleZH)
			proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
				SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
			})
			if err != nil {
				t.Fatal(err)
			}
			obs, _ := ValidateObservationProposals(proposals, "s1", "wi_1")
			if len(obs) == 0 {
				t.Fatal("no obs after validation")
			}
			got := obs[0].Strength
			if got != tc.wantCap {
				t.Errorf("strength = %.4f, want %.4f", got, tc.wantCap)
			} else {
				t.Logf("  ✓ %s → strength=%.4f", tc.name, got)
			}
		})
	}
}

// =============================================================================
// 测试 7: Max Proposals — 4 条提案 → 截到 3 条
// =============================================================================

func TestObserveTraceE2E_MaxProposalsTruncated(t *testing.T) {
	llmRaw := `[
		{"kind":"obs_signal","strength":0.5,"statement":"a","question":"","evidence":[]},
		{"kind":"obs_signal","strength":0.5,"statement":"b","question":"","evidence":[]},
		{"kind":"obs_signal","strength":0.5,"statement":"c","question":"","evidence":[]},
		{"kind":"obs_signal","strength":0.5,"statement":"d","question":"","evidence":[]}
	]`

	proposer, llm := newTraceProposer(t, llmRaw, "base", i18n.LocaleZH)
	proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
	})
	if err != nil {
		t.Fatal(err)
	}

	printBanner(t, "TestObserveTraceE2E_MaxProposalsTruncated — 4→3 截断")
	t.Logf("─── LLM 返回 4 条 obs ───")
	for i, p := range proposals {
		t.Logf("  [%d] %s statement=%q", i, p.Kind, p.Statement)
	}

	// ValidateObservationProposals 截到 3
	obs, _ := ValidateObservationProposals(proposals, "s1", "wi_1")
	t.Logf("\n─── Validate 后保留 %d 条 (max=3) ───", len(obs))
	if len(obs) != 3 {
		t.Errorf("kept %d obs, want 3 (maxObservationProposals)", len(obs))
	}
	gotStmts := payloadStmts(obs)
	if len(gotStmts) >= 3 && (gotStmts[0] != "a" || gotStmts[1] != "b" || gotStmts[2] != "c") {
		t.Errorf("truncation should keep first 3 in order, got %v", gotStmts)
	}
	if llm.callCount != 1 {
		t.Errorf("LLM called %d times, want 1", llm.callCount)
	}
}

func payloadStmts(obs []orchtypes.Observation) []string {
	out := make([]string, len(obs))
	for i, o := range obs {
		if p, ok := o.Payload.(orchtypes.SignalPayload); ok {
			out[i] = p.Name
		} else if p, ok := o.Payload.(orchtypes.FactPayload); ok {
			out[i] = p.Statement
		}
	}
	return out
}

// =============================================================================
// 测试 8: Empty Proposals → parse reject
// =============================================================================

func TestObserveTraceE2E_EmptyProposalsRejected(t *testing.T) {
	llmRaw := `[]`
	proposer, _ := newTraceProposer(t, llmRaw, "base", i18n.LocaleZH)
	proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	// parseObservationProposalsJSON 对 "[]" 返回 (nil, nil)
	// 跑到 ValidateObservationProposals → obs=[], proposals=[]
	// → mergeProposedObservations 走 line 282-283: 返回 RejectValidateEmpty
	if len(proposals) != 0 {
		t.Errorf("empty LLM should give 0 proposals, got %d", len(proposals))
	}
	obs, _ := ValidateObservationProposals(proposals, "s1", "wi_1")
	if len(obs) != 0 {
		t.Errorf("empty LLM should give 0 obs, got %d", len(obs))
	}
	t.Logf("  ✓ empty [] → 0 proposals, 0 obs (D7 走 fallback / Plan)")
}

// =============================================================================
// 测试 9: ObsFact statement 必填 — 空 statement 拒绝
// =============================================================================

func TestObserveTraceE2E_FactEmptyStatementRejected(t *testing.T) {
	llmRaw := `[{"kind":"obs_fact","strength":0.85,"statement":"","question":"","evidence":[]}]`
	proposer, _ := newTraceProposer(t, llmRaw, "base", i18n.LocaleZH)
	proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := ValidateObservationProposals(proposals, "s1", "wi_1")
	if len(obs) != 0 {
		t.Errorf("ObsFact with empty statement should be rejected, got %d obs", len(obs))
	}
	t.Logf("  ✓ ObsFact empty statement → 0 obs (validateOneProposal 拒绝)")
}

// =============================================================================
// 测试 10: ObsUncertainty question 必填, fallback 到 statement, 再空就拒
// =============================================================================

func TestObserveTraceE2E_UncertaintyQuestionFallback(t *testing.T) {
	cases := []struct {
		name           string
		rawJSON        string
		wantQuestion   string
		wantObsCount   int
	}{
		{
			name:         "question 填, statement 空",
			rawJSON:      `[{"kind":"obs_uncertainty","strength":0.7,"statement":"","question":"q?","evidence":[]}]`,
			wantQuestion: "q?",
			wantObsCount: 1,
		},
		{
			name:         "question 空, statement 填 → fallback",
			rawJSON:      `[{"kind":"obs_uncertainty","strength":0.7,"statement":"s?","question":"","evidence":[]}]`,
			wantQuestion: "s?",
			wantObsCount: 1,
		},
		{
			name:         "question 和 statement 都空 → 拒",
			rawJSON:      `[{"kind":"obs_uncertainty","strength":0.7,"statement":"","question":"","evidence":[]}]`,
			wantObsCount: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposer, _ := newTraceProposer(t, tc.rawJSON, "base", i18n.LocaleZH)
			proposals, _ := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
				SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
			})
			obs, _ := ValidateObservationProposals(proposals, "s1", "wi_1")
			if len(obs) != tc.wantObsCount {
				t.Fatalf("obs count = %d, want %d", len(obs), tc.wantObsCount)
			}
			if tc.wantObsCount == 1 {
				if u, ok := obs[0].Payload.(orchtypes.UncertaintyPayload); ok {
					if u.Question != tc.wantQuestion {
						t.Errorf("Question = %q, want %q", u.Question, tc.wantQuestion)
					}
					// Confidence = 1 - strength = 0.3
					if abs(u.Confidence-0.3) > 1e-9 {
						t.Errorf("Confidence = %.4f, want 0.3 (= 1 - 0.7)", u.Confidence)
					}
					if !u.RequiresMore {
						t.Errorf("RequiresMore = false, want true")
					}
				}
			}
		})
	}
}

// =============================================================================
// 测试 11: Kind alias 容错 — obs_fact / fact 都接受
// =============================================================================

func TestObserveTraceE2E_KindAliasCaseInsensitive(t *testing.T) {
	cases := []string{
		`[{"kind":"obs_fact","strength":0.85,"statement":"a","evidence":[]}]`,
		`[{"kind":"FACT","strength":0.85,"statement":"b","evidence":[]}]`,
		`[{"kind":"fact","strength":0.85,"statement":"c","evidence":[]}]`,
		`[{"kind":" obs_fact ","strength":0.85,"statement":"d","evidence":[]}]`,
	}
	for i, raw := range cases {
		t.Run(fmt.Sprintf("alias_%d", i), func(t *testing.T) {
			proposer, _ := newTraceProposer(t, raw, "base", i18n.LocaleZH)
			proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
				SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(proposals) != 1 || proposals[0].Kind != orchtypes.ObsFact {
				t.Errorf("alias parse fail: got %+v", proposals)
			}
		})
	}
}

// =============================================================================
// 测试 12: JSON 解析容错 — LLM 偶尔吐 markdown 包裹 / 解释
// =============================================================================

func TestObserveTraceE2E_JSONParseLeniency(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
	}{
		{"plain JSON", `[{"kind":"obs_fact","strength":0.85,"statement":"x","evidence":[]}]`, 1},
		{"with leading prose", `Here is my answer: [{"kind":"obs_fact","strength":0.85,"statement":"x","evidence":[]}]`, 1},
		{"with trailing note", `[{"kind":"obs_fact","strength":0.85,"statement":"x","evidence":[]}] (end)`, 1},
		{"garbage", `not json at all`, 0},
		{"empty", ``, 0},
		{"just brackets", `[]`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposer, _ := newTraceProposer(t, tc.raw, "base", i18n.LocaleZH)
			proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
				SessionID: "s1", WorkItemID: "wi_1", Directive: "x",
			})
			if err != nil {
				t.Logf("  err: %v", err)
			}
			t.Logf("  proposals count = %d (want %d)", len(proposals), tc.want)
			if len(proposals) != tc.want {
				t.Errorf("got %d, want %d", len(proposals), tc.want)
			}
		})
	}
}

// =============================================================================
// 测试 13: Bayesian Prior trace — PriorMean 是否被 Go 算 confidence 校准
// =============================================================================

func TestObserveTraceE2E_BayesianPrior_GoSideOnly(t *testing.T) {
	// 这个测试要 trace 一件事: prior_mean 不在 LLM prompt 里, 但
	// mergeProposedObservations 会读 in.PriorMean 并... 等等,让我重看代码
	//
	// 实际: mergeProposedObservations (observation_proposer.go:268-270) 读
	//   prior.PriorBeta.Mean() 写入 in.PriorMean
	// 但 in.PriorMean 进了 buildLLMObservationUserPrompt 后被 observeLLMFieldMap
	// 过滤掉 — 也就是说 PriorMean *不直接* 出现在 LLM 输入
	//
	// 那 PriorMean 真正的作用是? 答: 喂给 Plan / Verify 节点的 downstream
	// 决策 (例如 adaptive threshold),不是 Observe LLM 本身
	//
	// 这个测试要 trace 这个区分

	printBanner(t, "TestObserveTraceE2E_BayesianPrior_GoSideOnly — prior_mean 给 Go 不给 LLM")

	llmRaw := `[{"kind":"obs_fact","strength":0.85,"statement":"x","evidence":[]}]`
	proposer, llm := newTraceProposer(t, llmRaw, "base", i18n.LocaleZH)
	in := ObserveSignalInput{
		SessionID:  "sess_prior",
		WorkItemID: "wi_p_001",
		Directive:  "x",
		PriorMean:  0.7, // Bayesian prior from learner
	}

	_, err := proposer.ProposeObservations(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}

	// 断言: LLM user prompt 不含 prior_mean
	userPrompt := llm.lastMsgs[0].Content
	if strings.Contains(userPrompt, "prior_mean") {
		t.Errorf("❌ prior_mean 不应出现在 LLM user prompt (Go-only field)")
	} else {
		t.Logf("  ✓ prior_mean 不在 LLM prompt (Go 内部消费)")
	}
	if strings.Contains(userPrompt, "0.70") || strings.Contains(userPrompt, "0.7") {
		t.Errorf("❌ PriorMean=0.7 不应出现在 LLM prompt")
	} else {
		t.Logf("  ✓ PriorMean=0.7 没泄漏到 LLM")
	}

	// 说明 PriorMean 实际消费点
	t.Logf("\n─── PriorMean 实际消费点 (Go 端, 不进 LLM) ───")
	t.Logf("  1. observation_proposer.go:268-270: 读 prior.PriorBeta.Mean() → in.PriorMean")
	t.Logf("  2. (这里被过滤掉, 不到 LLM)")
	t.Logf("  3. 下游: Plan.QuantizeWithPrior / AnomalyDetector.DetectWithPrior 读 in.PriorMean")
	t.Logf("  4. adaptive_prior_overload.go: 高 strength ObsUncertainty ≥0.7 → α--, β++ penalty")
}

// =============================================================================
// 测试 14: 全 4 kind 一起跑, 验证 partition 路由
// =============================================================================

func TestObserveTraceE2E_AllKinds_PartitionRouting(t *testing.T) {
	// 一次性喂 3 种 kind (max=3 上限), 第 3 条 deviation 用 CatSystem 演示 anomaly 路由
	llmRaw := `[
		{"kind":"obs_fact","strength":0.85,"statement":"在标准算术下 2×3=6","evidence":[]},
		{"kind":"obs_signal","strength":0.6,"statement":"重复 attempt","evidence":[]},
		{"kind":"obs_deviation","strength":0.9,"statement":"P99 latency","evidence":[]}
	]`
	proposer, _ := newTraceProposer(t, llmRaw, "base", i18n.LocaleZH)
	proposals, _ := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "sess_allkinds", WorkItemID: "wi_all_001", Directive: "composite",
	})
	// 关键: 把 obs_deviation 的 Category 改 CatSystem 来演示 anomaly 路由
	// (LLM 一律返回 CatBusiness, 由 Go 端基于信号特征做 system/business 二分)
	for i := range proposals {
		if proposals[i].Kind == orchtypes.ObsDeviation {
			proposals[i].Category = orchtypes.CatSystem
		}
	}
	obs, _ := ValidateObservationProposals(proposals, "sess_allkinds", "wi_all_001")

	printBanner(t, "TestObserveTraceE2E_AllKinds_PartitionRouting — 3 kinds 同时跑")
	t.Logf("─── LLM 返回 3 条提案 (max=3 cap) ───")
	for _, p := range proposals {
		t.Logf("  %s str=%.2f cat=%s", p.Kind, p.Strength, p.Category)
	}
	printReportPartition(t, "sess_allkinds", obs)

	// 断言 routing
	rep, _ := orchtypes.NewUncertaintyReport("sess_allkinds", obs)
	if err := rep.Partition(); err != nil {
		t.Fatal(err)
	}
	// BusinessObservations: fact + signal (LLM 默认 CatBusiness)
	if len(rep.BusinessObservations) != 2 {
		t.Errorf("BusinessObservations = %d, want 2 (fact+signal)", len(rep.BusinessObservations))
	}
	// Anomalies: deviation (CatSystem)
	if len(rep.Anomalies) != 1 {
		t.Errorf("Anomalies = %d, want 1 (deviation→CatSystem)", len(rep.Anomalies))
	}
	// Overall = mean of business strengths = (0.85 + 0.6) / 2 (3rd is CatSystem, excluded)
	wantOverall := (0.85 + 0.6) / 2
	if abs(rep.Overall-wantOverall) > 0.001 {
		t.Errorf("Overall = %.4f, want %.4f (mean of business strengths: fact+signal only)", rep.Overall, wantOverall)
	}
	t.Logf("  ✓ Overall = %.3f (= mean of business strengths only, system excluded)", rep.Overall)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// =============================================================================
// 测试 15: 真实 WorkItem flow — 整条 pipeline 跑, capture round.ArtifactSummary
// =============================================================================

func TestObserveTraceE2E_FullPipeline_FactFastPath(t *testing.T) {
	// 用 newItemPipelineTestRunner + traceLLM, 跑整条 Run(), 看 fast-path
	// 是否真触发, round.ArtifactSummary 是不是 "在标准算术下 2×3=6"
	runner, tm, _ := newItemPipelineTestRunner(t)
	exec := &gateCountingExecutor{}
	runner.Executor = exec
	llm := &traceLLM{
		raw: `[{"kind":"obs_fact","strength":0.95,"statement":"在标准算术下，2×3=6。","question":"","evidence":[]}]`,
	}
	mups := &traceMUPS{systemBase: "你是 Devrix 助手。"}
	runner.ObservationProposer = NewLLMObservationProposer(llm, mups, i18n.LocaleZH)

	goal, err := tm.EnsureGoal("sess_trace_fullpipe", "2×3=几?")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}

	printBanner(t, "TestObserveTraceE2E_FullPipeline_FactFastPath — 端到端跑 Run()")

	round, err := runner.Run(context.Background(), "sess_trace_fullpipe", goal, "u1", ItemPipelineRunOpts{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	printSystemAndUser(t, llm)
	t.Logf("\n─── LLM response ───")
	t.Logf("%s", llm.raw)
	t.Logf("\n─── Run() result ───")
	t.Logf("  round.ArtifactSummary = %q", round.ArtifactSummary)
	t.Logf("  round.ExitReason = %q", round.ExitReason)
	t.Logf("  round.VerdictKind = %s", round.VerdictKind)
	t.Logf("  round.VerdictConfidence = %.2f", round.VerdictConfidence)
	t.Logf("  ExecuteWorkItem called %d times (want 0 — fast-path skipped Plan/Execute/Verify)", exec.callCount())

	if round.ArtifactSummary != "在标准算术下，2×3=6。" {
		t.Errorf("ArtifactSummary = %q, want %q", round.ArtifactSummary, "在标准算术下，2×3=6。")
	}
	if round.ExitReason != "observational_answer" {
		t.Errorf("ExitReason = %q, want observational_answer", round.ExitReason)
	}
	if exec.callCount() != 0 {
		t.Errorf("ExecuteWorkItem called %d times, want 0 (fast-path bypassed)", exec.callCount())
	}

	// item.Status 应是 Completed (PR #469 fix)
	got, _ := tm.GetWorkItem("sess_trace_fullpipe", goal.ID)
	if got.Status != workmodel.TaskStatusCompleted {
		t.Errorf("item.Status = %s, want TaskStatusCompleted", got.Status)
	} else {
		t.Logf("  ✓ item.Status = %s (PR #469 fix — session loop exits)", got.Status)
	}
	if !got.Locked {
		t.Errorf("item.Locked = false, want true")
	}
}

// =============================================================================
// Sanity: 确认 trace 工具的 import 没漏
// =============================================================================
var _ = json.Marshal
