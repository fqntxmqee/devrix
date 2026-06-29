# Tasks: D7 TaskContract 统一 (v6.0.x → v7.0.0)

**Change ID:** `devrix-d7-taskcontract-unification`
**Demand ID:** DM-20260629-006
**Status:** S6_Archived (2026-06-29, DESIGN ONLY — implementation deferred to v7.0)
**PR Count:** 3
**Sprint:** d7-v7 演进起点（v6.0.x 维护阶段收官后的第一枪）
**前置:** `devrix-d7-dsaft-restructuring`（DM-20260629-001）S7_Archived，v6.0.x 维护阶段收官
**模板:** `devrix-d7-dsaft-restructuring`（DM-20260629-001 S7_Archived 2026-06-29）+ `devrix-d4-dsaft-restructuring`（DM-20260629-004 S7_Archived 2026-06-29）tasks.md 模板

---

## §1 T 总览

| PR | 主题 | 覆盖 AC | T 范围 | 工作量 | 验收 |
|----|------|--------|--------|--------|------|
| **PR-A** | L1 接口 + L2 字段语义 + spec 同步 | AC1, AC2, AC3, AC4, AC5, AC17 | 6 AC / 6+ T | 1 PR / 1 周 | 22/22 orchestration packages -race PASS + `interfaces_task_spec_coverage{call_site} == 1.0` |
| **PR-B** | L3 防御低风险 + L4 治理基础 | AC11, AC15, AC9, AC10, AC16, AC21, AC22, AC23 | 8 AC / 8+ T | 1 PR / 2 周 | feature flag 默认 disabled + ORCH_* SentinelError 闭合 + 3 boundary test PASS |
| **PR-C** | L3 防御高风险 + L4 治理收口 | AC13, AC12, AC14, AC6, AC7, AC8, AC18, AC19, AC20 | 9 AC / 9+ T | 1 PR / 1.5 周 | Coverage ≥ 80% + Performance P99 < 1ms + CoW 灰度 1% → 100% |
| **总计** | — | **23 AC** | **24+ T** | **3 PR / 4.5 周** | — |

---

## §2 PR-A #0 L1 接口 + L2 字段语义（AC1-AC5, AC17）

**目标**：建立 `internal/layers/orchestration/interfaces/` 治理包 + TaskSpec/TaskReport 双契约 + 3 处创建点统一迁移 + spec 文档同步。

### T01 (D7-S16-A01-T01) `interfaces/task_spec.go` (NEW, ~80 lines)

**目标**：TaskSpec struct + 4+2 字段 + builder。

```go
package interfaces

type TaskSpec struct {
    Goal              string
    HardConstraints   []Constraint
    SoftPreferences   []Preference
    ConvergenceBudget ConvergenceBudget
    TraceID           string
    CostBudget        CostQuota
}

// 子类型（独立文件）
type Constraint struct { ... }       // interfaces/constraint.go
type Preference struct { ... }        // interfaces/preference.go
type ConvergenceBudget struct {       // interfaces/convergence_budget.go
    MaxRounds   int
    MaxTokens   int
    MaxSteps    int
    MaxSubagentDepth int
}
type CostQuota struct { ... }         // interfaces/cost.go

// 不可变 API
func NewTaskSpec(goal string) (*TaskSpec, error)        // Fail-fast: goal 空
func (s *TaskSpec) WithConstraint(c Constraint) *TaskSpec
func (s *TaskSpec) WithPreference(p Preference) *TaskSpec
func (s *TaskSpec) WithConvergenceBudget(b ConvergenceBudget) *TaskSpec
func (s *TaskSpec) WithTraceID(id string) *TaskSpec
func (s *TaskSpec) WithCostBudget(q CostQuota) *TaskSpec
func (s *TaskSpec) Validate() error
```

**约束**：
- `With*` 全部返回新副本（不可变）
- `interfaces` 包自身 0 import D7 任何子包（Pure types，AC21 防循环依赖）
- `NewTaskSpec("")` → `ErrInterfacesTaskSpecInvalid`（AC23）

**关键文件**：
- `internal/layers/orchestration/interfaces/task_spec.go` (NEW, ~80 lines)
- `internal/layers/orchestration/interfaces/{constraint,preference,convergence_budget,cost}.go` (NEW, ~30 lines each)

### T02 (D7-S16-A02-T01) `interfaces/task_report.go` (NEW, ~100 lines)

**目标**：TaskReport struct + 5+2 字段 + builder + 7 子类型。

```go
type TaskReport struct {
    Result            Result
    Evidence          Evidence
    Dissent           []DissentEntry
    Blockage          Blockage
    Resource          CostActual
    TraceID           string
    CostActual        CostActual
    MVPArtifact       *MVPArtifact      // AC11 后续 PR-B 填充
    HardEvidence      *HardEvidence     // AC15 后续 PR-B 填充
    FallbackUsed      bool              // AC12 后续 PR-C 填充
    VersionChainHash  Hash              // AC13 后续 PR-C 填充
}

// 子类型
type Result struct { Kind ResultKind; Confidence float64; Reason string }
type Evidence struct { Items []EvidenceItem; TotalCount int }
type DissentEntry struct { PlanID, PlanKind, Reason, Rejecter, Classification string }
type Blockage struct { Kind BlockageKind; Detail string }
type CostActual struct { TokensUsed, TimeMs, StepsTaken int; SubagentDepth int }
type Hash [32]byte

func NewTaskReport(result Result, evidence Evidence) (*TaskReport, error)  // Fail-fast
func (r *TaskReport) WithDissent(d DissentEntry) *TaskReport
func (r *TaskReport) WithBlockage(b Blockage) *TaskReport
func (r *TaskReport) WithResource(c CostActual) *TaskReport
func (r *TaskReport) WithTraceID(id string) *TaskReport
```

**关键文件**：
- `internal/layers/orchestration/interfaces/task_report.go` (NEW, ~100 lines)
- `internal/layers/orchestration/interfaces/{dissent,blockage,result,evidence,hash}.go` (NEW, ~30 lines each)

### T03 (D7-S17-A01-T01) `interfaces/dissent.go` 填充逻辑 (NEW, ~50 lines)

**目标**：Dissent 字段填充规则（INDETERMINATE 触发）+ top-3 截断。

```go
// FillDissent 从 exploration.go 全量结果抽取 minority_plan
func FillDissent(allResults []Result, verdict ResultKind, fallbackUsed bool) []DissentEntry {
    if verdict != ResultIndeterminate && !fallbackUsed {
        return nil
    }
    entries := []DissentEntry{}
    for _, r := range allResults {
        if r.PlanID != bestPlanID {  // 排除胜出方案
            entries = append(entries, DissentEntry{
                PlanID: r.PlanID, PlanKind: r.PlanKind,
                Reason: r.Reason, Rejecter: r.Rejecter,
                Classification: "internal",
            })
        }
    }
    return TruncateTop3(entries)  // AC3 性能保护
}
```

**关键文件**：
- `internal/layers/orchestration/interfaces/dissent.go` (NEW, ~50 lines)
- `internal/layers/orchestration/interfaces/dissent_test.go` (NEW, ~80 lines)

### T04 (D7-S17-A02-T01) `interfaces/blockage.go` 填充逻辑 (NEW, ~40 lines)

**目标**：Blockage 字段填充规则（FAIL 触发）+ 3 类 kind 分类。

```go
type BlockageKind string
const (
    BlockageMissingInfo      BlockageKind = "missing_info"
    BlockageInfeasiblePath   BlockageKind = "infeasible_path"
    BlockageRequiredExternal BlockageKind = "required_external"
)

func ClassifyBlockage(err error) Blockage {
    kind := BlockageInfeasiblePath  // fail-safe 兜底
    switch {
    case errors.Is(err, ErrMissingInput):       kind = BlockageMissingInfo
    case errors.Is(err, ErrExternalRequired):   kind = BlockageRequiredExternal
    }
    return Blockage{Kind: kind, Detail: err.Error()}  // 不 sanitize
}
```

**关键文件**：
- `internal/layers/orchestration/interfaces/blockage.go` (NEW, ~40 lines)
- `internal/layers/orchestration/interfaces/blockage_test.go` (NEW, ~60 lines)

### T05 (D7-S17-A03-T01) `interfaces/cost.go` Resource 字段填充 (NEW, ~40 lines)

**目标**：从 ContextBudget Phase B 抽取 token/time/step 消耗。

```go
func ExtractResource(sessionSpan trace.Span) CostActual {
    return CostActual{
        TokensUsed:     getSpanAttrInt(sessionSpan, "d7.context.budget.phase_b.tokens"),
        TimeMs:         getSpanAttrInt(sessionSpan, "d7.context.budget.phase_b.time_ms"),
        StepsTaken:     getSpanAttrInt(sessionSpan, "d7.context.budget.phase_b.steps"),
        SubagentDepth:  getSpanAttrInt(sessionSpan, "d7.context.budget.phase_b.subagent_depth"),
    }
}
```

**关键文件**：
- `internal/layers/orchestration/interfaces/cost.go` (NEW, ~40 lines)
- `internal/layers/orchestration/interfaces/cost_test.go` (NEW, ~50 lines)

### T06 (D7-S19-A02-T01) Spec 文档同步 (MODIFIED)

**目标**：`d7-domain.md` v2.6.0 → v3.0.0 + `spec.md` v4.10.0 → v7.0.0 + `t-registry.md` 加 6+ T 行。

- `openspec/specs/d7-orchestration/d7-domain.md` v2.6.0 → v3.0.0：新增 §TaskContract 章节（含 D7-S16/S17/S18/S19 Layer 说明）
- `openspec/specs/d7-orchestration/spec.md` v4.10.0 → v7.0.0：新增 4 个 Requirement（D7-S16/S17/S18/S19 各 1 Requirement）
- `openspec/specs/d7-orchestration/t-registry.md` v3.18.0 → v4.0.0：新增 D7-S16-A01-T01..S17-A03-T01 共 6 T 行（PR-A 子集）
- `openspec/t-registry.md` (root) v4.9.0 → v5.0.0：新增 DM-20260629-006 增量条目

**关键文件**：
- `openspec/specs/d7-orchestration/{d7-domain,spec,t-registry}.md` (MODIFIED)
- `openspec/t-registry.md` (MODIFIED)

### T07 (PR-A 集成验证) `interfaces/*_test.go` 6 个测试文件 (NEW, ~400 lines)

- `task_spec_test.go` (NEW, ~80 lines): TestNewTaskSpec / TestWith* 不可变 / TestValidate / TestError
- `task_report_test.go` (NEW, ~80 lines): TestNewTaskReport / TestWith* 不可变 / TestMVPNil / TestError
- `dissent_test.go` (NEW, ~80 lines): TestFillDissent_INDETERMINATE / TestFillDissent_Nil / TestTruncateTop3
- `blockage_test.go` (NEW, ~60 lines): TestClassifyBlockage_3Kind / TestClassifyBlockage_Default
- `cost_test.go` (NEW, ~50 lines): TestExtractResource / TestSubagentDepthLimit
- `interfaces_test.go` (NEW, ~50 lines): TestSecurityClassification / TestNoD7SubPackageImport

**验证**：
- `go test ./internal/layers/orchestration/interfaces/... -race -count=1` 6/6 PASS
- `wc -l internal/layers/orchestration/interfaces/*.go` ≤ 600 LOC（11 文件）
- `grep -rE "^import" internal/layers/orchestration/interfaces/ | wc -l` 必须为 0（Pure types）
- 22/22 orchestration packages `go test -race -count=1` PASS

**关键文件**：
- 6 NEW test files + 11 NEW impl files

### PR-A 提交检查清单

- [ ] 11 NEW impl files + 6 NEW test files
- [ ] 4 MODIFIED docs files（d7-domain.md + spec.md + t-registry.md + root t-registry.md）
- [ ] `go test -race` 全绿
- [ ] CI gate: `grep -rE "import" internal/layers/orchestration/interfaces/` = 0
- [ ] `interfaces_task_spec_coverage{call_site}` gauge = 1.0（创建点已就位，但 channel.New/plan.New/workitem.New 的迁移在 PR-A 后续步骤；本 PR 仅完成 11 NEW 文件）

---

## §3 PR-B #1 L3 防御低风险 + L4 治理基础（AC11, AC15, AC9, AC10, AC16, AC21, AC22, AC23）

**目标**：Pessimistic Commit + Hard Evidence + 治理基础（type alias / 边界测试 / feature flag / error code）。

### T08 (D7-S18-A01-T01) Pessimistic Commit 触发逻辑 (NEW, ~80 lines)

**目标**：`escape/pessimistic_commit.go` 实现 Evaluate 函数 + MVPArtifact 生成。

```go
// escape/pessimistic_commit.go
package escape

func Evaluate(ctx context.Context, state CircuitState) (*interfaces.MVPArtifact, error) {
    if !state.BudgetExhausted && !state.EscapeForceExit {
        return nil, nil  // 不触发
    }
    if state.TokensRemaining > state.CostBudget.MinReserve {
        return nil, nil  // 资源尚有富余，**不**误降级（风险缓解 #6）
    }
    artifact := &interfaces.MVPArtifact{
        CodePath:     state.LastCommitHash,
        TestStatus:   state.LastTestStatus,
        RiskWarning:  "资源耗尽，产物可能不完整（PR-B 灰度 1% 验证中）",
        TriggerReason: classifyTrigger(state),
    }
    return artifact, ErrPessimisticCommitTriggered
}
```

**关键文件**：
- `internal/layers/orchestration/escape/pessimistic_commit.go` (NEW, ~80 lines)
- `internal/layers/orchestration/escape/pessimistic_commit_test.go` (NEW, ~100 lines)

### T09 (D7-S18-A02-T01) Hard Evidence 拒绝"空证 Pass" (NEW, ~60 lines)

**目标**：`executionflow/verify/verifier.go` 集成硬证据校验。

```go
// executionflow/verify/verifier.go
func (v *Verifier) evaluate(evidence *interfaces.HardEvidence) (Result, error) {
    if evidence.IsEmpty() {  // 3 项全为零值
        if v.kind == "code" {
            return Result{Kind: ResultIndeterminate, Reason: "insufficient_evidence"},
                ErrHardEvidenceInsufficient
        }
        // chat 任务允许 entity_hash 单项通过
    }
    // ... 正常 Pass/Fail 逻辑
}
```

**关键文件**：
- `internal/layers/orchestration/executionflow/verify/verifier.go` (MODIFIED, +40 lines)
- `internal/layers/orchestration/executionflow/verify/verifier_test.go` (MODIFIED, +60 lines)

### T10 (D7-S19-A01-T01) Migration Plan 类型别名 (MODIFIED, ~20 lines)

**目标**：`mups/execute/{plan,channel}.go` 加 v6.0.x 类型别名 + Deprecation warning。

```go
// mups/execute/plan.go
type Plan = interfaces.TaskSpecV1  // v6.0.x 兼容

//mups/execute/channel.go
type ChannelRequest = interfaces.TaskSpecV1
```

**关键文件**：
- `internal/layers/orchestration/mups/execute/{plan,channel}.go` (MODIFIED, +10 lines each)
- `internal/layers/orchestration/mups/execute/migration_test.go` (NEW, ~40 lines)

### T11 (D7-S19-A06-T01) Cross-Domain Boundary tests (NEW, ~150 lines)

**目标**：`interfaces/boundary_test.go` 3 个 boundary test。

- `TestBoundary_D2_ConsumeTaskSpec`：D2 读取 `TaskSpec.Goal` + `ConvergenceBudget` 写入 context budget
- `TestBoundary_D4_ConsumeTaskSpec`：D4 worker 读取 `TaskSpec.HardConstraints` 阻塞违反约束
- `TestBoundary_D6_ConsumeTaskSpec`：D6 observer 读取 `TaskReport.Result` + `Dissent` advisory 校验

**关键文件**：
- `internal/layers/orchestration/interfaces/boundary_test.go` (NEW, ~150 lines)

### T12 (D7-S19-A07-T01) Feature Flag env-gated (NEW, ~60 lines)

**目标**：`hardening/feature_flag.go` 暴露 2 个 flag + 默认 disabled。

```go
// hardening/feature_flag.go
func IsPessimisticCommitEnabled() bool {
    return os.Getenv("D7_FEATURE_PESSIMISTIC_COMMIT") == "1"
}
func IsCoWVersionChainEnabled() bool {
    return os.Getenv("D7_FEATURE_COW_VERSION_CHAIN") == "1"
}
```

**关键文件**：
- `internal/layers/orchestration/hardening/feature_flag.go` (NEW, ~60 lines)
- `internal/layers/orchestration/hardening/feature_flag_test.go` (NEW, ~80 lines)

### T13 (D7-S19-A08-T01) ORCH_* SentinelError 闭合 (NEW, ~80 lines)

**目标**：`internal/shared/errors/orch_*.go` 7 个文件 + 三元组。

```go
// internal/shared/errors/orch_task_spec.go
var ErrInterfacesTaskSpecInvalid = &SentinelError{
    Code:        "ORCH_TASK_SPEC_INVALID",
    Message:     "interfaces.TaskSpec validation failed",
    Remediation: "set non-empty goal and provide hard constraints",
}
// 类似 orch_task_report.go / orch_pessimistic_commit.go / orch_similarity_collapse.go /
//      orch_hard_evidence.go / orch_version_chain.go / orch_rule_fallback.go
```

**关键文件**：
- `internal/shared/errors/orch_{task_spec,task_report,pessimistic_commit,similarity_collapse,hard_evidence,version_chain,rule_fallback}.go` (NEW, ~12 lines each)
- `internal/shared/errors/orch_registry_test.go` (NEW, ~50 lines)

### T14 (PR-B 集成验证) race test + LP 回归 (AC9, AC10)

**目标**：22/22 orchestration packages -race PASS + LP-1/LP-2/LP-5 100% 兼容。

- `go test ./internal/layers/orchestration/... -race -count=1` 22/22 PASS
- `go test -run TestAutoClose_FullLP1Loop ./tests/integration/d7/...` PASS
- `go test -run TestIntegration_5NodePipeline_End2End ./tests/integration/d7/...` PASS
- `go test -run TestLPReverseTraceability ./tests/integration/d7/...` PASS

**关键文件**：
- 无 NEW/MODIFIED（仅验证）

### PR-B 提交检查清单

- [ ] 8 NEW impl files (escape + hardening + errors) + 6 NEW test files
- [ ] 2 MODIFIED 跨包 (verifier.go + plan.go + channel.go)
- [ ] CI gate: `grep -rE "errors.New|fmt.Errorf" --include="*.go" internal/layers/orchestration/{interfaces,escape}/ | grep -v "ORCH_"` = 0
- [ ] `D7_FEATURE_PESSIMISTIC_COMMIT=0` 测试 PASS（默认 disabled）
- [ ] `D7_FEATURE_PESSIMISTIC_COMMIT=1` 测试 PASS（PR-B 内部测试，prod 灰度走脚本）
- [ ] 22/22 orchestration packages -race PASS
- [ ] LP-1/LP-2/LP-5 集成测试 3/3 PASS

---

## §4 PR-C #2 L3 防御高风险 + L4 治理收口（AC13, AC12, AC14, AC6, AC7, AC8, AC18, AC19, AC20）

**目标**：CoW VersionChain + Rule-based Fallback + Similarity Check + 治理收口（convergence span / AdaptiveThreshold / Coverage / Performance / Security）。

### T15 (D7-S18-A03-T01) CoW VersionChain (NEW, ~120 lines)

**目标**：`workmodel/version_chain.go` 实现 VersionChain 追加 + 父 Archive 只读 + GC + Rollback。

```go
// workmodel/version_chain.go
type VersionChain struct {
    mu       sync.RWMutex
    snapshots map[Hash]*Snapshot  // O(1) 索引
    order    []Hash                // 保序
    maxLen   int                   // 默认 10
}

func (vc *VersionChain) Append(delta []byte) Hash { ... }     // 追加 + 父只读
func (vc *VersionChain) RollbackTo(h Hash) error  { ... }    // O(1) 查找
func (vc *VersionChain) GC() int { ... }                     // 周期任务
```

**关键文件**：
- `internal/layers/orchestration/workmodel/version_chain.go` (NEW, ~120 lines)
- `internal/layers/orchestration/workmodel/version_chain_test.go` (NEW, ~150 lines)
- `internal/layers/orchestration/workmodel/workitem.go` (MODIFIED, +20 lines: `VersionChain []Hash` 字段)

### T16 (D7-S18-A04-T01) Rule-based Fallback 候选规则 (NEW, ~100 lines)

**目标**：`escape/rule_fallback.go` 4 候选规则 + env 切换 + A/B test。

```go
// escape/rule_fallback.go
type FallbackStrategy string
const (
    FallbackMostTestsPassed  FallbackStrategy = "most_tests_passed"
    FallbackCompiledClean    FallbackStrategy = "compiled_clean"
    FallbackMinCost          FallbackStrategy = "min_cost"
    FallbackMinUncertainty   FallbackStrategy = "min_uncertainty"  // 默认
)

func Evaluate(ctx context.Context, results []Result, round int) (Result, error) {
    if round < 3 || !shouldFallback(results) {
        return Result{}, nil
    }
    strategy := os.Getenv("D7_RULE_FALLBACK_STRATEGY")
    if strategy == "" { strategy = string(FallbackMinUncertainty) }
    return selectByStrategy(strategy, results), ErrRuleFallbackSelected
}
```

**关键文件**：
- `internal/layers/orchestration/escape/rule_fallback.go` (NEW, ~100 lines)
- `internal/layers/orchestration/escape/rule_fallback_test.go` (NEW, ~120 lines)

### T17 (D7-S18-A05-T01) Similarity Check 防递归塌陷 (NEW, ~100 lines)

**目标**：`mups/execute/similarity_check.go` embedding 哈希 + cosine 阈值 + LLM 二次校验。

```go
// mups/execute/similarity_check.go
const SimilarityThreshold = 0.80

func Validate(parent, child *interfaces.TaskSpec) error {
    h1 := embedHash(parent.Goal)
    h2 := embedHash(child.Goal)
    if h1 == h2 { return ErrSimilarityCollapseDetected }  // 哈希精确匹配
    sim := cosineSimilarity(h1, h2)
    if sim > SimilarityThreshold { return ErrSimilarityCollapseDetected }
    if sim > 0.70 { return llmSecondCheck(parent, child) }  // 边界升级
    return nil
}
```

**关键文件**：
- `internal/layers/orchestration/mups/execute/similarity_check.go` (NEW, ~100 lines)
- `internal/layers/orchestration/mups/execute/similarity_check_test.go` (NEW, ~120 lines)
- `internal/layers/orchestration/mups/execute/child_downlink.go` (MODIFIED, +10 lines: 调用 Validate)

### T18 (D7-S19-A09-T01) convergence.feasible_space_width span (NEW, ~40 lines)

**目标**：`mups/observe/convergence.go` 每次聚合后采样 W_up/W_down 比值。

```go
// mups/observe/convergence.go
func RecordConvergence(ctx context.Context, upstream, downstream int) {
    ratio := float64(downstream) / float64(upstream)
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.Int("feasible_space_width_upstream", upstream),
        attribute.Int("feasible_space_width_downstream", downstream),
        attribute.Float64("ratio", ratio),
    )
    if ratio > 1.0 {  // 异常发散
        slog.Warn("convergence anomaly: ratio > 1.0", "ratio", ratio)
    }
}
```

**关键文件**：
- `internal/layers/orchestration/mups/observe/convergence.go` (NEW, ~40 lines)
- `internal/layers/orchestration/mups/observe/convergence_test.go` (NEW, ~50 lines)

### T19 (D7-S19-A10-T01) AdaptiveThreshold 接入 RunTurn (MODIFIED, ~30 lines)

**目标**：`sessionorchestrator/run_turn.go` 类型安全读取 TaskSpec（解 TD-WT-01）。

```go
// sessionorchestrator/run_turn.go
func (o *SessionOrchestrator) runTurn(spec *interfaces.TaskSpec) error {
    threshold := o.adaptiveThreshold.Adjust(spec.ConvergenceBudget)  // 类型安全
    // ... 后续逻辑
}
```

**关键文件**：
- `internal/layers/orchestration/sessionorchestrator/run_turn.go` (MODIFIED, +30 lines)
- `internal/layers/orchestration/sessionorchestrator/run_turn_test.go` (MODIFIED, +40 lines)

### T20 (D7-S19-A11-T01) Layout guard interfaces 包 (NEW, ~60 lines)

**目标**：`hardening/layout_guard.go` 跨包 import 白名单 + 创建点合规。

```go
// hardening/layout_guard.go
var allowedInterfacesImporters = []string{
    "mups/execute", "mups/learn", "workmodel", "decisionplanning",
    "escape", "hardening", "sessionorchestrator", "executionflow", "d7-bootstrap",
}

func CheckInterfacesPackage() error { ... }  // CI gate
```

**关键文件**：
- `internal/layers/orchestration/hardening/layout_guard.go` (NEW, ~60 lines)
- `internal/layers/orchestration/hardening/layout_guard_test.go` (NEW, ~80 lines)

### T21 (D7-S19-A03-T01) Coverage ≥ 80% (AC18)

**目标**：`scripts/d7-coverage-report.sh` CI gate + 4 子包 ≥ 80% 覆盖率。

- `internal/layers/orchestration/interfaces/` ≥ 80%
- `internal/layers/orchestration/workmodel/version_chain.go` ≥ 80%
- `internal/layers/orchestration/escape/pessimistic_commit.go` ≥ 80%
- `internal/layers/orchestration/escape/{rule_fallback,similarity_check}.go` ≥ 80%
- 总体覆盖率不下降（delta ≥ -2% 可接受，<-2% 触发 S4-Gate 拒绝）

**关键文件**：
- `scripts/d7-coverage-report.sh` (NEW, ~30 lines)
- `.github/workflows/d7-coverage.yml` (NEW, ~20 lines)

### T22 (D7-S19-A04-T01) Performance Budget benchstat (AC19)

**目标**：3 个 benchmark + benchstat CI gate。

- `BenchmarkTaskSpecNew` P99 < 1ms
- `BenchmarkVersionChainLookup` O(1)
- `BenchmarkSimilarityCheck` O(1) embedding 命中

**关键文件**：
- `internal/layers/orchestration/interfaces/task_spec_bench_test.go` (NEW, ~40 lines)
- `internal/layers/orchestration/workmodel/version_chain_bench_test.go` (NEW, ~40 lines)
- `internal/layers/orchestration/mups/execute/similarity_check_bench_test.go` (NEW, ~40 lines)

### T23 (D7-S19-A05-T01) Security Classification (AC20)

**目标**：`interfaces/security_classification.go` 标签化 Dissent.Reason + LogExcerpt + 过滤逻辑。

- Learn 节点沉淀时按 `Classification` 标签过滤
- `secret` 不写入 SkillMemory（仅 ScheduledMemory 暂存）

**关键文件**：
- `internal/layers/orchestration/interfaces/security_classification.go` (NEW, ~50 lines)
- `internal/layers/orchestration/interfaces/security_classification_test.go` (NEW, ~60 lines)

### T24 (PR-C 集成验证) Spec 同步 + 归档

**目标**：spec.md / d7-domain.md / t-registry.md 最终同步 + 灰度脚本 + verify-archive。

- `openspec/specs/d7-orchestration/d7-domain.md` v3.0.0 → v3.1.0（PR-B/PR-C 累积 delta）
- `openspec/specs/d7-orchestration/spec.md` v7.0.0（PR-B/PR-C 累积 13 个 ADDED Requirements）
- `openspec/specs/d7-orchestration/t-registry.md` v4.0.0 → v4.1.0（PR-B/PR-C 累积 18+ T 行）
- `openspec/t-registry.md` (root) v5.0.0 → v5.1.0
- `scripts/devrix.sh` 加 `rollout-flag` 子命令

**关键文件**：
- 4 MODIFIED docs files + 1 MODIFIED shell script

### PR-C 提交检查清单

- [ ] 13 NEW impl files + 11 NEW test files + 3 NEW bench files + 2 NEW scripts
- [ ] 3 MODIFIED 跨包 (workitem.go + child_downlink.go + run_turn.go)
- [ ] 4 MODIFIED docs files + 1 MODIFIED shell script
- [ ] CI gate: `d7-coverage-report.sh` 4 子包 ≥ 80% PASS
- [ ] CI gate: `benchstat` P99 < 1ms + VersionChain/Similarity O(1) PASS
- [ ] CI gate: `grep -rE "interfaces" internal/layers/orchestration/` 仅在白名单包
- [ ] 22/22 orchestration packages -race PASS
- [ ] LP-1/LP-2/LP-5 集成测试 3/3 PASS
- [ ] `interfaces_task_spec_coverage{call_site: "plan.New|channel.New|workitem.New"}` = 1.0

---

## §5 后续归档（S6 — PR-D follow-up）

PR-D（不在本 Change 范围）做 S6 归档：

- 移动 `openspec/changes/devrix-d7-taskcontract-unification/` → `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/`
- 更新 `openspec/demand-archive-index.md` 新增 DM-20260629-006 行
- 运行 `./scripts/verify-archive.sh devrix-d7-taskcontract-unification` 12/12 PASS
- `d7-domain.md` v3.1.0 → v3.2.0（S7_Archived 标记）
- `t-registry.md` v4.1.0 → v4.2.0（DM-20260629-006 归档条目）

---

## §6 F-T 映射表

| F (Activity 子活动) | 关联 T 点 | 关联 AC | Phase |
|---------------------|----------|--------|-------|
| F01: TaskSpec struct + builder | T01 | AC1 | PR-A |
| F02: TaskReport struct + builder | T02 | AC2 | PR-A |
| F03: Dissent 填充逻辑 | T03 | AC3 | PR-A |
| F04: Blockage 填充逻辑 | T04 | AC4 | PR-A |
| F05: Resource 填充逻辑 | T05 | AC5 | PR-A |
| F06: Spec 文档同步 | T06 | AC17 | PR-A |
| F07: interfaces/*_test.go (6 文件) | T07 | AC9, AC18 | PR-A + PR-C |
| F08: Pessimistic Commit 触发 | T08 | AC11 | PR-B |
| F09: Hard Evidence 拒绝 | T09 | AC15 | PR-B |
| F10: Migration type alias | T10 | AC16 | PR-B |
| F11: Cross-Domain boundary_test | T11 | AC21 | PR-B |
| F12: Feature Flag env-gated | T12 | AC22 | PR-B |
| F13: ORCH_* SentinelError | T13 | AC23 | PR-B |
| F14: race test + LP 回归 | T14 | AC9, AC10 | PR-B |
| F15: CoW VersionChain | T15 | AC13 | PR-C |
| F16: Rule-based Fallback 候选 | T16 | AC12 | PR-C |
| F17: Similarity Check 拦截 | T17 | AC14 | PR-C |
| F18: convergence span | T18 | AC6 | PR-C |
| F19: AdaptiveThreshold 接入 | T19 | AC7 | PR-C |
| F20: Layout guard | T20 | AC8 | PR-C |
| F21: Coverage 验证 | T21 | AC18 | PR-C |
| F22: Performance benchstat | T22 | AC19 | PR-C |
| F23: Security Classification | T23 | AC20 | PR-C |
| F24: Spec 同步 + 归档 | T24 | AC17 | PR-C + PR-D |

---

## §7 风险与缓解（与 demand.md §6 对齐）

| 风险 | T 点 | 缓解 |
|------|------|------|
| TaskSpec/TaskReport 引入后老调用方断裂 | T10, T15 | 类型别名 + Layout guard 渐进迁移 |
| Dissent 字段数据量大 | T03 | top-3 截断 + summary 哈希引用 |
| Resource 字段需重新埋点 | T05 | 复用 ContextBudget Phase B 现有 metric |
| AC7 接入 RunTurn 触发未识别 bug | T19 | 分两阶段（PR-C 内部测试，prod 灰度走 flag） |
| 跨包 import cycle | T01, T02, T20 | interfaces 包只放 type，0 import D7 子包；Layout guard 白名单 |
| AC11 Pessimistic Commit 误触发 | T08 | 仅 EscapeForceExit/budget 归零时触发，资源富余直接 nil |
| AC12 Fallback 规则选择错误 | T16 | env A/B test，灰度脚本 |
| AC13 CoW 版本链膨胀 | T15 | 24h GC 周期 + hash-only 归档 |
| AC14 Similarity Check LLM 开销 | T17 | embedding 哈希 + 边界 [0.70, 0.80] 才升级 LLM |
| AC15 Hard Evidence 误伤 | T09 | Verifier kind-specific（code vs chat） |
| AC22 Feature Flag 灰度失败 | T12 | `RolloutDisable()` 自动回滚 |
| AC23 Error Code 不闭合 | T13 | CI grep 0 命中 gate |
| 全部 23 AC 节奏失控 | 全部 | 每个 PR 配 S3/S4 Gate；AC9/AC10 跨 PR 验证；AC17 spec 同步前置 |

---

## §8 关联引用

- `openspec/changes/devrix-d7-taskcontract-unification/demand.md` §3（23 AC 表）
- `openspec/changes/devrix-d7-taskcontract-unification/design.md` §5（File Manifest）
- `openspec/changes/devrix-d7-taskcontract-unification/specs/d7-orchestration/spec.md`（4 ADDED Requirements）
- `openspec/changes/devrix-d7-taskcontract-unification/.openspec.yaml`（24+ T 点）
- 前置归档：`openspec/archive/2026-06-29-devrix-d7-dsaft-restructuring/`（v6.0.x 收官）
- 指南原文（用户提供）：《多层递归循环的向下传播与向上反馈》
- Gemini 工程实践 review（2026-06-29）：AC11-AC15 映射
