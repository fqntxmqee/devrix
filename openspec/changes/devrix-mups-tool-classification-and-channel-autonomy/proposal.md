# Proposal: MUPS 5 节点 × Tool 元数据 Control Plane + ToolChannel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Status:** S2_Clarified → S3_Design（博弈论双 review 共识已并入 demand/proposal/design/tasks）
**Created:** 2026-07-01
**Parent Demand:** `demand.md`
**Game Theory Reviews:** `game-theory-review.md` · `game-theory-review-composer.md`

---

## 0. Synthesis Lineage

本文档是 6 轮对话的合成，源文档：
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/54-tools-metadata-ideal-state-and-channel-autonomy.md`（doc 54）
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/53-clawcode-tools-design.md`（doc 53，Clawcode 35 字段参照）
- `/Users/fukai/brain/01知识探索/项目/20260620-certain-architecture/core-concepts/38-mature-uncertainty-methodology.md`（doc 38，MUPS v3 5 节点 × 4 类正交）

每文件 source_anchors 含 devrix 当前 file:line 锚点。

---

## 1. Background

PR #373（hotfix 2026-07-01）修了 D1 feishu 表现层 task_incomplete 透传，但**根因未动**：
- LLM 反复 `read_file` × 9 次（累计 50K tokens）
- 每次被 D2 `TruncateToTokens(cfg.ToolResultBudget=8K)` 截断
- LLM 看不到完整内容 → 自我循环探索
- D7 Verify 节点接受探索性 finalText → 失败标 task_incomplete
- D1 渲染红卡（PR #373），但 LLM **实际产出 = 0 review**

之前 4 个相关 change 都只**治标**：
- **DM-20260701-005**（devrix-d7-verify-synthesize-enforce）— 治标：加 verify 节点 "synthesize or fail" 强校验
- **DM-20260701-006**（devrix-d2-tool-result-budget-for-review）— 治标：按 task kind 调 budget 阈值
- **DM-20260630-012**（devrix-d7-deliverable-convergence）— 治标：task_incomplete 标识修
- **DM-20260629-001**（devrix-d7-dsaft-restructuring）— 治本的前置：Span Evidence 100%

**本文档做治本**：Tool 元数据从 declaration 升级为 control plane，4 类正交分解从 MUPS 节点级下沉到 tool metadata 级，让每个 tool 自收敛。

### 1.1 博弈论共识摘要（H7–H12，2026-07-01）

| ID | 设计承诺 |
|----|----------|
| H7 | 单 session **SPE**（`Bounded(n)` hard reject）；跨 session **reputation**（Learn 最小接入） |
| H8 | LTL L4–L6 与 L0–L3 安全 invariant **cross-check**（≥3 条，见 `specs/execute-channels.md`） |
| H9 | PlanKind × EmissionClass **交叉一致性** + `OnResult` 行为重分类（call_count>3 → Probe） |
| H10 | `emission_class` cheap talk → Phase C 写 Learn FeedbackMemory；完整 drift 表 Phase E |
| H11 | Phase B **shadow mode**（`would_reject` log-only，FP<5% 再 enforce） |
| H12 | PR-A **禁止 silent default**；`read_file`/`grep`/`glob` 显式 `Probe+Bounded(15)` |

---

## 2. Problem Statement

| ID | 现象 | 根因 | 严重度 |
|----|------|------|--------|
| P01 | 现有 ToolSpec v2 仅 9 字段，LLM 自我循环无自收敛信号 | 9 字段是 init-time 1 次查表 declaration，runtime 每条命令不再查；无 iteration_bound / emission_class / truncate_marker | **P0** |
| P02 | Execute 节点不分类所有 tool_call 等价处理 | 单 channel 不分流 Fact/Action/Probe/Experiment，Probe 类工具无强终止 | **P0** |
| P03 | Verify 节点接受探索性 finalText | 无 input contract 强校验 deliverable mandatory | **P0** |
| P04 | 截断对 LLM 不透明 | D2 ToolResultBudget 截断无 marker text，LLM 不知文件不完整 | **P0** |
| P05 | verify_exit_reason 在 buildSessionCompleteEvent 局部变量丢失 | verdict.Reason 不写 meta，D1 渲染拿不到 verify 层语义 | **P1** |
| P06 | Filter 维度仅 agent+risk 2 维 | 缺 emission_class / task_kind 路由，review/edit/test/observe 任务一视同仁 | **P1** |
| P07 | 元数据是 declaration 不是 control plane | Clawcode TS 35 字段 runtime 每条命令查 7 fn，devrix 9 字段 init 时查一次就丢 | **P1** |

---

## 3. Proposed Solution

### 3.1 核心决策

**把 MUPS 4 类正交分解从节点级下沉到 tool metadata 级**，让每个 tool 自带 `emission_class` + `convergence_contract` + `iteration_bound` + `source_uncertainty` 4 个新字段。

Execute 节点按 emission_class 路由 4 Channel，每个 channel 在 **register time** 就强制挂 LTL-Lite invariant（`L4` Bounded、`L5` Quotient、`L6` Synthesize）。

Verify 节点用 input contract 4 元组（`deliverable_text / evidence / source_uncertainty / emission_class`）强校验，verdict 透传全链路。

### 3.2 ToolSpec v3 字段定义（15 字段）

```go
// /Users/fukai/workspace/devrix/internal/shared/contracts/tool_surface_v3.go
type ToolSpec struct {
    // ---- 现有 9 字段（不变，行为兼容）----
    Name        string
    Description string
    Parameters  string
    Risk        types.RiskLevel
    ReadOnly        bool
    Destructive     bool
    OpenWorld       bool
    ConcurrencySafe bool
    DeferLoading    bool
    // ---- 新增 6 字段（R3 cycle 0 默认值，保证向后兼容）----
    EmissionClass       EmissionClass       // {Fact, Action, Probe, Experiment}; 默认 Action
    ConvergenceContract ConvergenceContract // {StateChangeRequired, EvidenceRequired, QuotientThreshold, None}; 默认 None
    IterationBound      IterationBound      // {Bounded(n), OpenEnded, Quotient(t)}; 默认 OpenEnded (R3) → R4 strict
    SourceUncertainty   SourceUncertainty   // {Source enum, Value float64}; 默认 LLM(0.5)
    MaxResultSizeChars  int                 // 0 表示无限; 默认 0 (R3) → R4 按 8K 默认
    TruncateMarkerText  string              // 截断必附加 marker; 默认 "[TRUNCATED]"
}
```

**3 个 type 定义**：

```go
type EmissionClass int
const (
    EC_Fact EmissionClass = iota        // read_file, grep, glob, lsp_*
    EC_Action                           // bash, write_file, edit_file
    EC_Probe                            // call_agent, delegate_*, MCP, lsp (重读时)
    EC_Experiment                       // free_fork, sub-process, worktree
)

type ConvergenceContract struct {
    Kind         int    // enum: StateChangeRequired / EvidenceRequired / QuotientThreshold / None
    Threshold    float64 // 仅 QuotientThreshold 用
    RequiredText int    // EvidenceRequired 时 evidence 数量下限
}

type IterationBound struct {
    Kind       int     // Bounded / OpenEnded / Quotient
    MaxN       int     // Bounded(n) 的 n
    Quotient   float64 // Quotient 阈值
}

type SourceUncertainty struct {
    Source int     // LLM(0.4) / User(0.85) / Deterministic(1.0) / Memory(0.3)
    Value  float64 // 直接给值
}
```

### 3.3 Execute 节点 4 ToolChannel

**命名澄清**：本 change 引入的新抽象叫 `ToolChannel`（per-tool-EmissionClass 终止约束），与现有 mups/execute/ 中的 4 个 `Channel`（per-PlanKind 执行策略 — DM-20260625-001-PRC2 已归档）是平行不冲突的两套接口。两套都叫 "channel" 但语义层级不同：

| 抽象 | 文件 | 路由依据 | 职责 |
|------|------|----------|------|
| **PlanChannel** (现有 `Channel` rename) | `internal/layers/orchestration/mups/execute/channel.go` | `plan.PlanKind` | Plan 的执行策略（commit/protocol/scenario/exploration） |
| **ToolChannel** (本 change 新增) | `internal/layers/orchestration/mups/execute/toolchannel/channel.go` | `tool.EmissionClass` | 单 tool_call 的终止不变量（fact/action/probe/experiment） |

**P0 门禁（H12 共识）**：PR-B 前将现有 `Channel` interface **rename 为 `PlanChannel`**，允许 1-release `type Channel = PlanChannel` alias，消除 Schelling focal point 冲突。

调用顺序：Orchestrator 按 PlanKind 选 PlanChannel → PlanChannel 内部按 tool.EmissionClass 选 ToolChannel → ToolChannel 应用终止不变量。

```
internal/layers/orchestration/mups/execute/toolchannel/
├── channel.go              # ToolChannel interface + registry + router
├── fact_channel.go         # emission_class=Fact  (L7-FACT-SAME-Q-5x)
├── action_channel.go       # emission_class=Action (L7-ACTION-POSTSNAPSHOT)
├── probe_channel.go        # emission_class=Probe ⚡ (L4-BOUNDED-ITERATIONS 治本核心)
└── experiment_channel.go   # emission_class=Experiment (L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE)
```

每 ToolChannel 接口：

```go
type ToolChannel interface {
    Name() string
    EmissionClass() EmissionClass
    Route(tool ToolSpec) bool
    Accept(ctx context.Context, call *ToolCall) error
    OnResult(ctx context.Context, result *ToolResult) error
    InjectPromptPressure(ctx context.Context, remaining int) error
    Invariants() []ltl.Invariant
    Finalize(ctx context.Context) (*ChannelOutcome, error)
}
```

**ToolChannel 路由规则**：

| ToolChannel | emission_class | LTL-Lite Invariant | PromptPressure 时机 (task_kind override) |
|---|---|---|---|
| FactToolChannel | Fact | `L7-FACT-SAME-Q-5x` (同 query 重复 5 次 → 强制 synthesize) | 剩 2 次 |
| ActionToolChannel | Action | `L7-ACTION-POSTSNAPSHOT` (PostSnapshot ≠ PreSnapshot → Verifiable) | 剩 2 次 |
| **ProbeToolChannel** ⚡ | Probe | `L4-BOUNDED-ITERATIONS` (iter ≥ B(n) → InjectSynthesize) | **剩 5 次软警告，剩 2 次硬警告，0 次强制 synthesize-only** |
| ExperimentToolChannel | Experiment | `L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE` (ConcludedAt < Deadline) | 剩 2 次 |

**task_kind override**（PerTaskKindFilter 推 iteration_bound 时）：
- `review` → `Bounded(15)` + 软警告@5, 硬警告@2, 强制@16
- `edit` → `Bounded(10)` + 软警告@3, 硬警告@1, 强制@11
- `test` → `Bounded(12)` + 软警告@4, 硬警告@1, 强制@13
- `observe` → `OpenEnded` (不强制)

### 3.4 Verify Input Contract

```go
// /Users/fukai/workspace/devrix/internal/layers/orchestration/executionflow/verify/verify_contract.go
type VerifyContract struct {
    ExpectedClass       EmissionClass
    DeliverableRequired bool
    DeliverableMinChars int               // task_kind-dependent: review=20, edit=10, test=30, observe=10
    EvidenceRequired    bool
    MinEvidenceCount    int
    MinSourceQuality    float64           // calibrated_confidence 下限
}

// NewVerifyContract 显式构造器（防 Go 零值陷阱：MinSourceQuality=0 永远过）
func NewVerifyContract(taskKind string, expected EmissionClass) VerifyContract {
    return VerifyContract{
        ExpectedClass:       expected,
        DeliverableRequired: true,
        DeliverableMinChars: defaultMinCharsForTaskKind(taskKind),
        EvidenceRequired:    expected != EC_Fact,
        MinEvidenceCount:    1,
        MinSourceQuality:    0.5,
    }
}

func Verify(ctx, contract VerifyContract, turnOut *TurnOutput) (*Verdict, error) {
    // 1. 校验 deliverable_text 非空（若 Required）
    if contract.DeliverableRequired && len(strings.TrimSpace(turnOut.FinalText)) < contract.DeliverableMinChars {
        return &Verdict{
            Kind: VerdictFail,
            Reason: "deliverable_missing",
            CalibratedConfidence: 0.0,
        }, nil
    }
    // 2. 校验 evidence 数量（若 Required）
    if contract.EvidenceRequired && len(turnOut.ToolCalls) < contract.MinEvidenceCount {
        return &Verdict{
            Kind: VerdictFail,
            Reason: "evidence_insufficient",
        }, nil
    }
    // 3. 计算 calibrated_confidence
    cc := computeCalibratedConfidence(turnOut)
    if cc < contract.MinSourceQuality {
        return &Verdict{Kind: VerdictPartial, CalibratedConfidence: cc, Reason: "source_uncertainty_high"}, nil
    }
    return &Verdict{Kind: VerdictPass, CalibratedConfidence: cc}, nil
}
```

**`calibrated_confidence` 公式修正**（Codex Critical review 修复）：

```yaml
# 单 EmissionClass 会话：CC = source_uncertainty（直觉一致）
# 多 EmissionClass 混合：CC = Σ(su × w) / Σ(w) （按权重和归一化）
calibrated_confidence = Σ(source_uncertainty × emission_class_weight) / Σ(emission_class_weight)

emission_class_weight（Codex Critical #6 修正：EC_Fact 确定性读 > EC_Action 可观察状态变更）：
  EC_Fact:        0.50  # 确定性读，要么返内容要么 error
  EC_Action:      0.35  # 状态变更可观察但可失败/回滚
  EC_Probe:       0.20  # 探索性，依赖 LLM 判断
  EC_Experiment:  0.10  # 最不确定
```

**验算**：
- 单 `EC_Fact + User(0.85)` 会话：CC = 0.85 × 0.50 / 0.50 = **0.85**（intuitive ✓）
- 单 `EC_Action + User(0.85)` 会话：CC = 0.85 × 0.35 / 0.35 = **0.85**（intuitive ✓）
- 单 `EC_Action + LLM(0.4)` 会话：CC = 0.4 × 0.35 / 0.35 = **0.4**（intuitive ✓）
- 多类混合：权重归一化保证结果在 [0, 1]

### 3.5 Reason 透传链路

```
D7 verifyRollupArtifact(contract, output) → Verdict{Kind, Reason, CC}
       ↓
D7 buildSessionCompleteEvent:
       meta["verify_exit_reason"]   = verdict.Reason       ← NEW
       meta["emission_class"]       = primary_class        ← NEW
       meta["source_uncertainty"]   = verdict.CC           ← NEW
       ↓
D7 Emit EngineEvent(complete, ...)
       ↓
D1 EmitComplete (透传 meta)
       ↓
D1 feishu.go OnMessage(complete)
       msg.Metadata["verify_exit_reason"]
       msg.Metadata["emission_class"]
       msg.Metadata["source_uncertainty"]
       ↓
D1 feishu render:
       title: "❌ 任务失败 (ProbeToolChannel: exploration_loop_aborted @ iter 12/15, source_uncertainty=0.62)"
       footer: "❌ 任务未完成 (reason: deliverable_missing)"
```

### 3.6 Filter v2 路由

```go
// /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/filter/v2/per_emission_class.go
type PerEmissionClassFilter struct{ AllowClasses []EmissionClass }
func (f *PerEmissionClassFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    var out []ToolSpec
    for _, s := range specs {
        for _, ac := range f.AllowClasses {
            if s.EmissionClass == ac { out = append(out, s); break }
        }
    }
    return out
}

// /Users/fukai/workspace/devrix/internal/layers/contextengine/enforce/tools/filter/v2/per_task_kind.go
type PerTaskKindFilter struct{ TaskKind string }
func (f *PerTaskKindFilter) Apply(specs []ToolSpec, ctx FilterCtx) []ToolSpec {
    bound := taskKindBound(f.TaskKind) // review=Bounded(15), edit=Bounded(10), test=Bounded(12), observe=OpenEnded
    var out []ToolSpec
    for _, s := range specs {
        // 6-轴兼容性矩阵（Codex W1 扩展 — 含 Quotient case）:
        //   OpenEnded(K=0) 在任何 task 都允许（无 bound 不越界）
        //   任何 tool 在 OpenEnded task 都允许（任务无上限）
        //   Quotient(K=2) 在 Bounded task 允许（soft bound 不强制 hard cap）
        //   Bounded(K=1) 在 Quotient task 允许（Quotient task 无 MaxN 上限）
        //   Bounded+Quotient 任意方向允许（基于 metric 自然收敛）
        //   Bounded+Bounded → tool.MaxN ≤ task.MaxN
        if s.IterationBound.Kind == 0 || bound.Kind == 0 ||
           s.IterationBound.Kind == 2 || bound.Kind == 2 ||
           s.IterationBound.MaxN <= bound.MaxN {
            out = append(out, s)
        }
    }
    return out
}
```

### 3.7 阶段拆解（4 Phase）

| Phase | 范围 | T 点 | PR |
|-------|------|------|-----|
| **A** | ToolSpec v3 schema + 19 工具默认 metadata + silent default gate | 8 | PR-A |
| **B-pre** | `Channel` → `PlanChannel` rename（PR-B 前 P0 门禁） | 1 | PR-B 首 commit |
| **B** ⭐ | Execute 4 ToolChannel + shadow mode + ProbeToolChannel Bounded(15) + LTL-Lite L4-L6 + L0–L3 cross-check | 12 | PR-B |
| **C** | VerifyContract 4 元组 + burden of proof + verdict 透传 D1 + **Learn FeedbackMemory** | 7 | PR-C |
| **D** | Filter v2 三维（agent + emission_class + task_kind；**无 workspace**）+ cross-consistency | 5 | PR-D |
| **Phase count** | **4 PR 联动**（见 tasks.md 估时） | **33 T** | |

**PR-A 治本叙事关键**：PR-A 包含 **19 工具默认 metadata 迁移**（read_file = Probe + Bounded(15) + MaxResultSizeChars=8K + TruncateMarker 等），不依赖后续 PR。Codex Critical #4 修复：原方案 OOS-6 "19 工具元数据迁移" 拆到 Phase E → 治本叙事破产（PR-A 零行为变更）；修正后 19 工具迁移作为 PR-A 完成项。

### 3.8 关键约束

- **不重编号** 历史 T/A/F/S ID
- **不破坏** 现有 9 字段向后兼容（新增 6 字段有 R3 cycle 0 默认值）
- **不引入** 新外部依赖
- **不迁移** 任何物理目录（mups/execute/ 现有 4 PlanKind Channel 保留）
- **不回迁** 现有测试（行为兼容，新增 6 字段放 struct 末尾 + 实证 grep `contracts.ToolSpec{` 全部用 named field 语法，0 position literal break 风险）
- **新增 ltl/ 子包** parallel 不冲突（observability/instrument/ltl/invariants/termination/）
- **PlanChannel 与 ToolChannel 命名解耦**：PR-B 前 rename（P0-AC-8）；新抽象 ToolChannel，旧抽象 PlanChannel
- **禁止 silent metadata default**：缺 EmissionClass → CI fail（P0-AC-10）；禁止 runtime fallback `Action+OpenEnded`
- **Shadow mode 再 enforce**：`EnableMupsChannels` 先 log-only `would_reject`（H11）
- **Learn 最小接入**：Phase C 写 `verify_exit_reason` → FeedbackMemory（H6/H10 最小子集）
- **可独立回滚** 每个 Phase（feature flag `registry.RuntimeConfig.EnableMupsChannels`）

---

## 4. Success Metrics

### 4.1 主目标

| ID | 标准 | 验证方式 | Phase |
|----|------|----------|-------|
| AC1 | ToolSpec 有 15 字段（含 6 新字段，位置在末尾） | `awk '/^type ToolSpec struct/,/^}/' shared/contracts/tool_surface.go \| grep -cE '^\s+(EmissionClass\|ConvergenceContract\|IterationBound\|SourceUncertainty\|MaxResultSizeChars\|TruncateMarkerText)\b'` = 6 | A |
| AC2 | 19 工具全部有 6 新字段（治本前置：含 PR-A 默认 metadata 迁移） | `grep -L "EmissionClass:" internal/layers/contextengine/enforce/tools/surface/*.go` = empty | A |
| AC3 | Execute 节点 4 ToolChannel 路由生效 | `mups/execute/toolchannel/{fact,action,probe,experiment}_channel_test.go` 全 PASS | B |
| AC4 | ProbeToolChannel Bounded(15) 硬校验生效（iter 16 → SynthesizeNowSignal） | `probe_channel_test.go::TestBoundedIterationHardStops` PASS | B |
| AC5 | PromptPressure 软警告@剩5/硬警告@剩2/强制@0 注入 LLM（task_kind override） | `probe_channel_test.go::TestPromptPressureInjectReview` + `TestPromptPressureInjectEdit` PASS | B |
| AC6 | LTL-Lite L4-L6 invariant 3 个新类可用 | `observability/instrument/ltl/invariants/termination/{bounded,quotient,synthesize}.go` + 9 unit tests | B |
| AC7 | VerifyContract 校验 deliverable 缺失/过短 → FAIL（task_kind-dependent MinChars） | `verify_contract_test.go::TestDeliverableMissing` + `TestMinCharsByTaskKind` PASS | C |
| AC8 | verdict.Reason 写入 meta["verify_exit_reason"] | `session_complete_test.go::TestReasonInMeta` PASS | C |
| AC9 | D1 feishu render reason 标签（用 RenderArgs struct param 避免签名 break） | `feishu_progress_test.go::TestRenderVerifyExitReason` PASS | C |
| AC10 | TruncateMarker 必附加 marker，LLM 可见 | `truncate_marker_test.go::TestMarkerAlwaysAppended` + `TestShortOutputNoMarker` PASS | C |
| AC11 | PerEmissionClassFilter 路由 | `per_emission_class_test.go::TestFilterByEmissionClass` PASS | D |
| AC12 | PerTaskKindFilter 路由 + task_kind 驱动 iteration_bound（review=Bounded(15) etc.） | `per_task_kind_test.go::TestReviewGetsBounded15` + `TestObserveOpenEnded` + `TestEditBounded10` PASS | D |
| AC13 | D2 PrepareOrchestrator 从 user_intent 推 task_kind | `prepare_orchestrator_test.go::TestTaskKindInference` PASS | D |
| AC14 | `calibrated_confidence` 数学正确（单类 CC = su，混合类 Σ(su×w)/Σ(w) 归一化） | `verify_contract_test.go::TestCalibratedConfidenceFormula` + 8 子用例 PASS | C |
| AC15 | `NewVerifyContract(taskKind, expected)` 显式构造器（防 Go 零值陷阱） | `verify_contract_test.go::TestZeroValueUsesSafeDefaults` PASS | C |
| AC16 | PlanChannel rename 完成（P0-AC-8） | 无裸 `type Channel interface` 与 ToolChannel 同名 | B-pre |
| AC17 | read_file/grep/glob = Probe+Bounded(15)（P0-AC-9） | surface spec 单测 PASS | A |
| AC18 | 禁止 silent metadata default（P0-AC-10） | 缺 EmissionClass 的 surface CI fail | A |
| AC19 | Equilibrium Concept 写入 design.md §2.4（P1-AC-1） | Spec review | design |
| AC20 | L4–L6 vs L0–L3 cross-check ≥3 条（P1-AC-2） | `specs/execute-channels.md` + 单测 | B |
| AC21 | VerifyContract burden of proof by class（P1-AC-3） | `TestBurdenOfProofByClass` PASS | C |
| AC22 | verify_exit_reason → Learn FeedbackMemory（P1-AC-4） | `TestReasonInFeedbackMemory` PASS | C |
| AC23 | Shadow mode FP<5% 再 hard enforce（P1-AC-5） | shadow 指标 + sign-off | B |
| AC24 | review 时 Probe 不得 OpenEnded（P1-AC-7） | `TestPerTaskKindFilterCrossConsistency` PASS (D2-S15-A02-T15) | D |
| AC25 | Filter v2 不含 workspace 维（P1-AC-8） | 代码 + spec 审查 | D |

### 4.2 范围外

| ID | 不做 | 理由 |
|----|------|------|
| OOS-1 | 历史 T/A/F 路径重编号 | 跨 30+ 历史 change |
| OOS-2 | 物理目录迁移（mups/execute/ 现有 4 PlanKind Channel 保留） | 各域自治 |
| OOS-3 | WaveScheduler / D4 delegate 行为变更 | 无关 |
| OOS-4 | Clawcode TS Tool 接口 goalong | 我们是 Go 实现 |
| OOS-5 | 现有 LTL-Lite L0-L3 安全 invariant 改造 | observability/instrument/ 新建 ltl/ 子包，平行不干扰 |
| OOS-6 | ~~现有 19 工具元数据默认值迁移~~ | **已 INCLUDE in PR-A**（Codex Critical #4 修复：避免治本叙事破产） |
| OOS-7 | Bash 22 zsh rules 改造 | 与本次无关 |
| OOS-8 | D1 / D3 / D4 / D6 域元数据 | 域自治 |
| OOS-9 | 新建外部依赖 | Pure Go |
| OOS-10 | Filter v2 **workspace 维** | 双 review 共识 defer（P1-AC-8） |
| OOS-11 | 完整 drift audit + 半自动 emission_class 分类器 | Phase E；本 change 仅 Learn 最小接入 |

### 4.3 AC ↔ T 点映射

| AC | T 点 (DSAFT) | Phase |
|----|--------------|-------|
| AC1 | D2-S15-A02-T06..T07 (ToolSpec v3 struct + 4 type 定义) | A |
| AC2 | D2-S15-A02-T08..T12 (per-surface spec 重标 + 19 工具默认 metadata) | A |
| AC3 | D7-S9-A50-T01..T03 (ToolChannel interface + router + 4 实现) | B |
| AC4 | D7-S9-A50-T04 (ProbeToolChannel Bounded(15) hard stop) | B |
| AC5 | D7-S9-A50-T05 (PromptPressure 注入，task_kind override) | B |
| AC6 | D5-S25-A01-T01 + A02-T01 + A03-T01 (L4 Bounded + L5 Quotient + L6 Synthesize) | B |
| AC7 | D7-S10-A50-T01 (VerifyContract 校验 deliverable/evidence/CC) | C |
| AC8 | D7-S10-A50-T02 (Reason 写入 meta) + D7-S2-A50-T07 (session_complete 透传) | C |
| AC9 | D7-S10-A50-T03 (D1 feishu render reason，RenderArgs struct param) | C |
| AC10 | D2-S15-A02-T13 (TruncateWithMarker 实现) | C |
| AC11 | D2-S15-A02-T02 (PerEmissionClassFilter) | D |
| AC12 | D2-S15-A02-T03..T04 (PerTaskKindFilter + iteration_bound 推) | D |
| AC13 | D2-S15-A02-T05 (D2 PrepareOrchestrator task_kind 推) | D |
| AC14 | D7-S10-A50-T01 (calibrated_confidence 数学 + 单测 8 子用例) | C |
| AC15 | D7-S10-A50-T01 (NewVerifyContract 显式构造器) | C |

---

## 5. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| ToolSpec v3 是 breaking change | 6 新字段有 R3 cycle 0 默认值，旧调用方 0 修改通过 |
| Execute 4 Channel 行为变更影响现有 22 zsh rules | emit 不变，只加 prompt pressure；bash 仍走 ActionChannel |
| LTL-Lite L4-L6 是新概念 | 用 read_file + edit_file 作 MVP fixture；PR-B 内含 5+ tests |
| PR-B 与 DM-20260701-005 治本目标重叠 | DM-005 是 Verify synthesize enforce（治标），DM-007 是 Execute + tool metadata（治本），DM-005 留作 Phase C 协同实现 |
| PR-A 与 DM-20260618-001 (ToolSpec v2) 兼容 | DM-007 加字段不删字段，纯 additive |
| PerTaskKindFilter 需要 IntentClassifier | DM-20260618 已实现 IntentClassifier（per D7 Phase 5 验证集 90%+），复用 |
| PlanChannel vs ToolChannel 命名冲突 | PR-B 前 rename + 1-release type alias（P0-AC-8） |
| PromptPressure 被 LLM 忽略 | hard reject @n+1；shadow + baseline（P1-AC-5/6） |
| Phase B 硬切 false positive | shadow mode 1 周（H11） |
| emission_class cheap talk | Learn FeedbackMemory 最小接入 + Phase E drift audit |
| Filter 维度过载 | workspace 维 defer（OOS-10） |

---

## 6. Implementation Plan

### Phase A: ToolSpec v3 Schema（治本前置 — 含 19 工具默认 metadata）

**PR-A：** `feat/devrix-mups-tool-classification-and-channel-autonomy/pr-a-tool-spec-v3`

范围：
- `internal/shared/contracts/tool_surface.go` **EXTEND** (加 6 字段在末尾 + 4 type 定义 + JSON tags 一致性)
- `internal/shared/contracts/tool_surface_test.go` EXTEND (struct literal 兼容性单测)
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go` 扩展 19 工具**默认 metadata 迁移**
- `internal/layers/contextengine/enforce/tools/surface/{builtin,lsptool,tracker,freefork,verify,askuser,backgroundtask,tool_search}_surface.go` 各自 spec 重标
- 0 业务代码改动，metadata additive only
- **关键**：6 字段位置在 struct 末尾，避免 position struct literal break

T 点 (DSAFT)：D2-S15-A02-T06..T12

### Phase B-pre: PlanChannel Rename（P0 门禁，PR-B 首 commit）

**范围：**
- `internal/layers/orchestration/mups/execute/channel.go`：`Channel` → `PlanChannel` + optional `type Channel = PlanChannel` alias 1-release
- 更新 callers/tests/bootstrap wire

**T 点：** D7-S9-A26-T06

### Phase B: Execute 4 ToolChannel ⭐

> ⚠️ **S/A Registry 前置门禁**（Codex Critical #1 修复）：PR-B 引入的新 S/A（D5-S25 / D7-S9-A50）**必须先在对应域的 `spec.md` + `a-registry.md` 注册**，再开 PR-B 实现分支。S/A 未注册 → PR-B review 自动 FAIL。
> - `openspec/specs/d5-observability/spec.md` 加 S25 (LTL-Lite termination) + `a-registry.md` 加 A01/A02/A03
> - `openspec/specs/d7-orchestration/spec.md` 加 A50 (ToolChannel) + `a-registry.md` 加 A50
> - PR-C 的 D7-S10-A50 复用 PR-B 的 A50 namespace，同一 spec.md 注册即可

**PR-B：** `feat/devrix-mups-tool-classification-and-channel-autonomy/pr-b-execute-channels`

范围：
- `internal/layers/orchestration/mups/execute/toolchannel/` NEW 子包
  - `channel.go` ToolChannel interface + registry + router + telemetry
  - `fact_channel.go` / `action_channel.go` / `probe_channel.go` (核心) / `experiment_channel.go`
- `internal/layers/observability/instrument/ltl/invariants/termination/` NEW 子包（observability/instrument/ltl/ 不存在需新建）
  - `bounded.go` (L4)
  - `quotient.go` (L5)
  - `synthesize.go` (L6)
- `internal/layers/orchestration/mups/execute/channel.go` EXTEND (PlanKind Channel 内部调 ToolChannelRouter)
- `internal/layers/contextengine/queryloop/turn_adapter.go` 接 toolchannel.emit
- `internal/bootstrap/surfaces.go` Wire ToolChannel
- **Shadow mode**：`ToolChannelRouter` log `would_reject` 不 block；`EnableMupsChannelsEnforce` 单独 flag
- **`OnResult` 行为重分类**：call_count>3 + 同 query → 升级 Probe（H9）
- `specs/execute-channels.md` 增 L4–L6 vs L0–L3 cross-check（H8）

T 点 (DSAFT)：D7-S9-A26-T06 + D7-S9-A50-T01..T08 + D5-S25-A01-T01..T02 + A02-T01 + A03-T01

### Phase C: VerifyContract + Reason 透传

**PR-C：** `feat/devrix-mups-tool-classification-and-channel-autonomy/pr-c-verify-contract-reason`

范围：
- `internal/layers/orchestration/executionflow/verify/verify_contract.go` NEW VerifyContract + NewVerifyContract() 构造器
- `internal/layers/orchestration/executionflow/verify/verify_contract_test.go` NEW (含 8 子用例 CC 公式 + MinChars by task_kind)
- `internal/layers/orchestration/sessionorchestrator/session_complete.go` 改 meta 透传 5 元 verdict
- `internal/layers/communication/channel/conclusion/conclusion.go` 改 EmitComplete 透传
- `internal/layers/communication/channel/adapters/feishu.go` 改 OnMessage 读 meta + 改用 RenderArgs struct param（避免 break 5-param finalizeStructuredSession 签名）
- `internal/layers/communication/channel/adapters/feishu_progress.go` 改 render 加 reason 标签
- `internal/layers/contextengine/compression/truncate_marker.go` NEW
- **VerifyContract burden of proof** by EmissionClass（demand §3.3）
- **`mups/learn/` FeedbackMemory**：session_complete 写 `verify_exit_reason`（H6/H10 最小接入）

T 点 (DSAFT)：D7-S10-A50-T01..T04 + D7-S2-A50-T07..T08 + D7-S11-A40-T01 + D2-S15-A02-T13

### Phase D: Filter v2 + Task Kind 路由

**PR-D：** `feat/devrix-mups-tool-classification-and-channel-autonomy/pr-d-filter-v2-task-kind`

范围：
- `internal/layers/contextengine/enforce/tools/filter/v2/` NEW 子包
  - `per_emission_class.go` PerEmissionClassFilter
  - `per_task_kind.go` PerTaskKindFilter (含 4 task_kind → IterationBound 映射)
- `internal/layers/contextengine/enforce/tools/filter/per_agent.go` 改 v2 (加 emission_class 二级过滤，explore→Fact+Probe, worker→Fact+Action+Probe, delegate→Probe+Action)
- `internal/layers/contextengine/prepare/orchestrator.go` 改 PrepareOrchestrator 加 task_kind 推
- `internal/bootstrap/surfaces.go` DefaultFilters v2（**三维**：agent + emission_class + task_kind；**无 workspace**）
- **Cross-consistency**：review 时 Probe 工具 IterationBound 不得 OpenEnded（H9）

T 点 (DSAFT)：D2-S15-A02-T02..T06

---

## 7. Out-of-band References

- Obsidian doc 54 (本文档详细说明 source)
- `devrix/internal/shared/contracts/tool_surface.go:19-152` (v2 schema 基线 — PR-A 在末尾 additive 加 6 字段)
- `devrix/internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go:44-142` (19 工具 metadata 现状 — PR-A 改造)
- `devrix/internal/layers/contextengine/enforce/tools/surface/tool_search_surface.go:43-221` (deferred discovery)
- `devrix/internal/layers/contextengine/enforce/tools/sandbox.go:14-204` (现有 bashAST)
- `devrix/internal/layers/contextengine/enforce/tools/bash/zsh_rules.go:29-96` (22 zsh rules)
- `devrix/internal/layers/contextengine/enforce/tools/filter/per_agent.go:27-125` (PerAgentFilter — PR-D 改造为 v2)
- `devrix/internal/bootstrap/surfaces.go:47-117` (BuildSurfaces + DefaultFilters — PR-D 改造)
- `devrix/internal/layers/orchestration/mups/execute/channel.go:55-258` (现有 4 PlanKind Channel + ChannelRegistry + ChannelRouter — DM-20260625-001-PRC2 已归档；PR-B 内部扩展，**不破坏现有接口**)
- `devrix/internal/layers/orchestration/mups/execute/{commit,protocol,scenario,exploration}.go` (4 PlanKind Channel 实现)
- `devrix/internal/layers/orchestration/mups/execute/toolchannel/` NEW (PR-B 新建 — ToolChannel per EmissionClass)
- `devrix/internal/layers/orchestration/executionflow/verify/` (现有 verify 子包 — PR-C 加 verify_contract.go)
- `devrix/internal/layers/orchestration/executionflow/verify/contracts.go` (现有 Verify contracts — PR-C 不破坏)
- `devrix/internal/layers/orchestration/sessionorchestrator/session_complete.go:17-44` (现有 buildSessionCompleteEvent — PR-C 改 meta 透传)
- `devrix/internal/layers/orchestration/sessionorchestrator/verdict_to_exit_reason.go` (现有 verdict→exit_reason 映射)
- `devrix/internal/layers/observability/instrument/{logger,metrics,telemetry,tracer}/` (现有 observability 子包 — PR-B 新建 ltl/)
- `devrix/internal/layers/communication/channel/conclusion/conclusion.go:91-154` (现有 EmitComplete — PR-C 改透传)
- `devrix/internal/layers/communication/channel/adapters/feishu.go:138-148` (现有 OnMessage "complete" — PR-C 改读 meta)
- `devrix/internal/layers/communication/channel/adapters/feishu_progress.go:138-148` (PR #373 task_incomplete 落地 — PR-C 改 render)
- Clawcode TS Tool.ts:1-792 / tools.ts:1-389 (35 字段参照)
- Clawcode BashTool bashSecurity.ts:1-2592 (23 bash 安全 check)
- Clawcode ClawTool.ts ToolSearchTool.ts:1-471 (deferred discovery 模式)

---

## 更新历史

- 2026-07-01：v1 创建 (6 轮对话综合 → 4 Phase + 23 T 点)
- 2026-07-01：v1.1 S3-Gate Review 反馈 — Critical #1-#9 修复
  - Channel→ToolChannel 重命名 + 与现有 mups/execute/ 4 PlanKind Channel 命名解耦
  - 文件路径全改 (sessionorchestrator/executionflow/ 不存在 → mups/execute/ + executionflow/verify/)
  - PR-A INCLUDE 19 工具默认 metadata 迁移 (OOS-6 移除)
  - calibrated_confidence 数学 Σ(su×w)/Σ(w) 归一化 + EC_Fact>EC_Action 权重排序
  - PromptPressure 软警告@5/硬警告@2 + task_kind override
  - DeliverableMinChars 按 task_kind 区分 + NewVerifyContract() 构造器
  - T 点全部改为 DSAFT 格式 D{X}-S{X}-A{XX}-T{XX}
  - dsaft_scenarios 扩展到 9 个 (D2-S1, D2-S2, D2-S15, D5-S3, D7-S2, D7-S5, D7-S9, D7-S10, D7-S11)
  - status enum s1_requirement → s3_design + 新建 demand.md
  - "9 天"估时从 proposal 移到 tasks.md
  - feishu render 改 RenderArgs struct param (避免 break 5-param finalizeStructuredSession 签名)
  - lsp_* EC_Fact/Probe 拆分 (deterministic read-only 归 Fact)
- 2026-07-01：v1.2 S3-Gate Re-Review 反馈 — 6 Critical T-ID 冲突修复
  - D2-S1 RETIRED → Phase A 改用 **D2-S15-A02-T06..T12**（跳 T01 RepairToolChain + T02..T05 Phase D Filter）
  - D2-S2 RETIRED → TruncateWithMarker 改用 **D2-S15-A02-T13**
  - D7-S9-A26 T01..T05 已被 4 PlanKind Channel 占 → Phase B ToolChannel 改用 **D7-S9-A50-T01..T06**（新 A）
  - D5-S3 RETIRED (legacy Logger S) → LTL-Lite 改用 **D5-S25-A01-T01..A03-T01**（新 S25）
  - D7-S2-A50-T01..T06 已被 turn merge + API error class 占 → session_complete meta 透传改用 **D7-S2-A50-T07**
  - D2-S15-A02-T01 已被 RepairToolChain 占 → Phase D Filter 改用 **D2-S15-A02-T02..T05**
  - dsaft_scenarios 缩到 6 个（D2-S1/D2-S2/D5-S3/D7-S11 RETIRED — D7-S5 保留因 IntentClassifier 仍被 D2-S15-A02-T03/T05 复用, D5-S25 新增）
  - 25 T 点（v1.1 23 → v1.2 25，因为 Phase B 拆 ToolChannel + LTL-Lite 子点）
  - 加 **S/A Registry 前置门禁**（PR-B 之前先在 d5/d7 spec.md + a-registry.md 注册新 S/A）
  - PerTaskKindFilter 6-轴扩展（OpenEnded + Bounded+Quotient 任意方向）
  - read_file ConvergenceContract = **None**（不是 Quotient(0.7)）
  - scope narrative 治本关键 → 治本前置（PR-A 是治本前置，不是治本关键）
- 2026-07-01：v1.2.1 S3-Gate Re-Review patch — 5 Critical 修复
  - ⚠️ **S/A Registry 前置门禁**（tasks.md L293-303 + proposal.md §6 Phase B preamble）— D5-S25/D7-S9-A50/D7-S10-A50 必须在域 spec.md + a-registry.md 先注册
  - verify-contract.md:23 stale T07（D7-S2-A50-T01 → T07）
  - proposal.md:332 AC1 grep gate 用 awk 限定 ToolSpec struct 范围
  - design.md §4.3 Span table 8 stale A-prefix 全部 remap
- 2026-07-01：v1.2.2 v1.2.1 review 4 项 Warning/Info 修复
  - specs/tool-spec-v3.md:26 ConvergenceContract 默认描述 (read=Quotient → read=None)
  - tasks.md:237 Phase B 旧 T-B* 编号列 7 → 9 (T-B01..T-B09)
  - proposal.md:517 update history D7-S5 RETIRED 误标 → 修正为 4 项 retired (D7-S5 保留)
  - design.md 序列图 Channel.OnCall/ProbeChannel → ToolChannel.OnCall/ProbeToolChannel (10 处)
- 2026-07-01：v1.3 博弈论双 review 共识并入（H7–H12、PlanChannel rename、shadow mode、Learn FeedbackMemory、Filter 三维、AC16–AC25）
- 2026-07-01：v1.3.1 S3-Gate Re-Review patch — T-ID 冲突修复（AC24 加 D2-S15-A02-T15 引用）+ .openspec.yaml W1 计数文本修正（11 → 14 T-points）+ v1.3.1 version_scope 注册；codex 复审 PASS
