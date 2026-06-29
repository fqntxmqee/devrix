# Proposal: devrix-d7-dsaft-restructuring (DM-20260629-001)

**Change ID:** `devrix-d7-dsaft-restructuring`
**Demand ID:** DM-20260629-001
**Priority:** P0
**Sprint:** d7-v6.x 维护阶段 → v7.0 演进起点
**PR Count:** 8-10 (6 子 Change 联动 + WorkTree 治理纳入 #1)
**Status:** S2_Proposal → S3_Design → S4_Implemented → S5_Accepted → S7_Archived
**DSAFT 阶段:** §阶段 3 North Star 重整 + §阶段 4 v1.1 Traceability + §阶段 6 双锚点对齐 + 原则 1/3/4/5 修复

---

## 1. Background

D7 v6.0.0 (2026-06-26 S7_Archived) 已完成 4 轮物理迁移实现 0 函数签名变化。但 2026-06-29 双 Agent 全量盘点暴露 **6 类深度架构债**：

1. **S 层语义偏离**（6 S 全是包名，违反原则 1）
2. **god function**（turn_orchestrator.go 1551 行超 800 行硬上限近 2 倍）
3. **Registry 路径漂移**（6 F 路径错误）
4. **~126 LOC 死代码 + 老链路**（双 Agent grep 验证 0 外部调用者）
5. **文档漂移**（4 处 stale doc 描述已删字段）
6. **T↔Span 覆盖率仅 ~30%** + 3 项能力跨域越界

详见 `demand.md` §1。

---

## 2. Problem Statement

D7 在 DSAFT 方法论 4 轴评估下处于 **"v2.0 结构已闭合、v1.1 追溯未闭合、S 语义偏离未修复、god function 与死代码累积"** 的复杂中间状态。

| 维度 | 当前 | 目标 |
|---|---|---|
| S 层合规 | ⭐⭐ | ⭐⭐⭐⭐ |
| A 层合规 | ⭐⭐⭐ (god function 拖分) | ⭐⭐⭐⭐ |
| F 层合规 | ⭐⭐⭐ (6 路径错 + 41 ghost) | ⭐⭐⭐⭐⭐ |
| T↔Span 追溯 | ⭐⭐ (~30%) | ⭐⭐⭐⭐ (≥80%) |
| 跨域边界 | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| 死代码债 | ⭐⭐ (~126 LOC) | ⭐⭐⭐⭐⭐ (0) |
| God function 债 | ⭐ (1551 行) | ⭐⭐⭐⭐⭐ (<800 行) |

需要 **6 个子 Change 联动** + **7-9 个 PR** + **~50 个 AC** 全闭环。

---

## 3. Goals / Non-Goals

### 3.1 Goals（12 项量化指标）

| Goal | Metric | Target |
|---|---|---|
| **G1**：~126 LOC 死代码 + 老链路全删 | grep + 22/22 -race | 0 死符号 |
| **G2**：turn_orchestrator.go 拆完 | wc -l | 每个文件 <800 行；3-4 个文件 |
| **G3**：Registry 路径全对 | f-registry | 6/6 修正 |
| **G4**：6 S 配 ValueFlow Alias | d7-domain.md §North Star | 6/6 |
| **G5**：T↔Span 覆盖率 ≥80% | t-registry Span Evidence 列 | ≥80% |
| **G6**：跨域越界 Decision 表 | d7-domain.md §Out of Scope | 3/3 |
| **G7**：Legacy 段 + ghost count 全删 | a/f-registry 头部 | 0 |
| **G8**：pipeline-architecture.md v1.2.0 | 同步 6 S + 1 横切 | ✅ |
| **G9**：orchestration packages -race PASS | regression | 22/22 |
| **G10**：verify-archive.sh | acceptance | 12/12 PASS |
| **G11**：P0 T 100% PASS | acceptance | 193/193 |
| **G12**：god function 36 T 重映射 | t-registry 更新 | 36/36 归属正确 |
| **G13**：WorkTree 上行反馈 typed RollupReport | 5 处 child.LastRound 调用 → 1 处 RollupReport struct | 5/5 |
| **G14**：`sessionRootGoal` 确定性 | 多 root 场景下 unit test | 1/1 |
| **G15**：D7-S0-A07 3 T 点登记 | t-registry Statistics | 3/3 |

### 3.2 Non-Goals

- 不删 S 旧编号（DSAFT 原则 3）
- 不下沉 3 项 boundary debt 能力（保留实现，仅 Decision 标注）
- 不动 5 节点管道数据契约
- 不动 D7 bootstrap wire 拓扑
- 不重构 WorkTree v2 升级（TD-WT-01..06 单独 change）
- 不下沉 ReputationEvidence / SystemAnomaly / AdaptivePrior

---

## 4. Solution（6 子 Change 拆分）

### 4.1 子 Change #0：dead-code-cleanup（先行 PR，0 业务改动）

**PR-1 范围**：~126 LOC 死代码 + 老链路删除 + 4 处 doc drift 重写

**修改清单**：

| # | 文件 | 改动 | LOC 变化 |
|---|---|---|---|
| 0.1 | `internal/layers/orchestration/orchtypes/uncertainty_coord.go` | 删 `ErrInvalidVerdictKind` alias | -4 |
| 0.2 | `internal/layers/orchestration/orchtypes/config.go` | 删 `LLMFallback` / `ShadowLLMClassify` / `ShadowLLMTimeoutMs` / `FastPathThreshold` + `RuleOrchestrateConfig()` | -13 |
| 0.3 | `internal/layers/orchestration/orchtypes/routing.go` | 删 `RoutingModeRuleOrchestrate` enum + 死 switch arm | -6 |
| 0.4 | `internal/layers/orchestration/orchtypes/artifact_kind_alias.go` | 删 4 alias constants（保留 type aliases） | -6 |
| 0.5 | `internal/layers/orchestration/decisionplanning/prompts.go` | 删整文件 | -26 |
| 0.6 | `internal/layers/orchestration/delegatetools/subquery_fallback.go` | 删 `BuildSubQueryRunner` | -4 |
| 0.7 | `internal/layers/orchestration/escape/engine.go` | 删 `var _ = time.Now` + 删 time import | -2 |
| 0.8 | `internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` | 删 legacy explicit-code path | -6 |
| 0.9 | `internal/layers/orchestration/workmodel/aggregate_verdicts.go` | 删 `AggregateVerdicts` + 4 serialization helpers | -60 |
| 0.10 | `internal/layers/orchestration/sessionorchestrator/turn_doc.go` | 重写 v6.0 6S 状态描述 | 0 |
| 0.11 | `internal/layers/orchestration/orchtypes/config.go` | header 重写（去 v1.0 dead fields 描述） | 0 |
| 0.12 | `internal/layers/orchestration/orchtypes/routing.go` | docstring 去 `rule_orchestrate` arm 描述 | 0 |
| 0.13 | `internal/layers/orchestration/delegatetools/subquery_fallback.go` | docstring 去 AC6/AC10 引用 | 0 |
| 0.14 | 5 处调用方迁移（test 文件） | 替换 RuleOrchestrateConfig → DefaultConfig | -10 |

**总 LOC 变化**：约 -135 LOC（126 死代码 + 9 doc drift 重写）

**AC**：grep 验证 0 外部调用者；22/22 orchestration packages -race PASS；0 test 失败；verify-archive.sh 12/12 PASS。

**PR-1 提交并合入 master**（PR 链路起点）。

### 4.2 子 Change #1：turn-fn-split（god function 拆分 + WorkTree 上行反馈治理）

**PR-2 + PR-3 + PR-3-extended 范围**：
1. `turn_orchestrator.go` 1551 行按职责拆 4 文件
2. **WorkTree 上行反馈 3 项治理**（用户 2026-06-29 复盘追问触发；推荐项"纳入 #1 turn-fn-split"）

**拆分方案**（按"职责子模块"切面）：

```
turn_orchestrator.go (1551 行, god)
   ↓ 拆
   ├─ turn_orchestrator.go (<300 行, 主入口 RunTurn + 路由)
   ├─ turn_loop.go (<500 行, SessionTurnLoop + 迭代 + escape 检查)
   ├─ turn_invoke.go (<500 行, LLMInvoke + ToolRound + ReAct)
   └─ turn_recovery.go (<500 行, EscapeEngine 接入 + ResumeSession + Error Recovery)
```

**WorkTree 上行反馈 3 项治理**（用户已批准纳入本子 Change）：

| 改进 | 触发问题 | 文件 | 工作量 |
|---|---|---|---|
| **A typed RollupReport** | `child.LastRound` 隐式 envelope 5 处取字段 | 新建 `workmodel/rollup_report.go` + 改 5 处调用方 | 1 PR / 1 天 |
| **B deterministic sessionRootGoal** | `rollup_gate.go:122-129` map 遍历非确定 | `workmodel/rollup_gate.go` 改为 `sort.Slice` | 0.5 天 |
| **C 3 T 登记** | ApplyPipelineDecide 顺序 + ReevaluateParentAfterChild 幂等 + PathA/B trigger 缺 T | `t-registry.md` 加 D7-S0-A07-T01..T03（含 Path A `ShouldRollupAfterChildren` + Path B `MaybeRootRollupFallback` 10 组合 unit test）| 1 天 |

**A 改进详细方案**（RollupReport struct）：

```go
// workmodel/rollup_report.go (NEW, DM-20260629-001-A)
// RollupReport 强类型聚合 child.LastRound → 上行反馈 envelope。
// 替代 context_bubble_apply.go / rollup_gate.go / rollup_gate.go:154 多处自由取字段。
type RollupReport struct {
    ChildID          string                  `json:"child_id"`
    VerdictKind      string                  `json:"verdict_kind"`
    ArtifactSummary  string                  `json:"artifact_summary"`
    UncertaintyMean  float64                 `json:"uncertainty_mean"`
    SpawnPolicy      string                  `json:"spawn_policy"`
    BubbleKind       string                  `json:"bubble_kind"`
    GeneratedAt      time.Time               `json:"generated_at"`
}

// NewRollupReportFromRound 构造器（保持 child.LastRound 不变，RollupReport 仅作为读取侧聚合）。
func NewRollupReportFromRound(childID string, round *WorkItemPipelineRound) *RollupReport { ... }

// ReevaluateParentAfterChild 返回 *RollupReport 而非 (struct{}{})，3 调用点统一返回类型。
func ReevaluateParentAfterChild(sessionID, childID string, tm *TaskManager) (*RollupReport, error) { ... }
```

**约束**：
- **0 函数签名变化**（pure physical split）；**唯一例外**：`ReevaluateParentAfterChild` 返回类型 `(struct{}, error) → (*RollupReport, error)`，但**调用方 0 改动**（丢弃返回值即可，向后兼容）
  - 例外条款详细文档：`specs/d7-orchestration/spec.md §2.3.1`
- **D7-S15 TurnState 接口不变**（D7-S15 DM-20260628-003 刚闭环的 turn 串行化；RollupReport 不参与 TurnState 序列化）
- **0 跨文件 import cycle 引入**（turn_*.go 同 package；RollupReport 在 workmodel 包内）
- **child.LastRound 不变**（RollupReport 是读取侧聚合，不修改 source-of-truth）

**AC**：每个新文件 wc -l <800；36 T 全部归属正确；5 处 child.LastRound 调用全部替换为 RollupReport；sessionRootGoal 多 root unit test PASS；D7-S0-A07-T01..T03 登记；22/22 -race PASS；v6.0.0 已闭合的 4 轮迁移 0 函数签名变化保留。

### 4.3 子 Change #2：registry-sync（F 路径 + 文档同步）

**PR-4 范围**：F 路径 + t-registry Statistics + pipeline-architecture v1.2.0 + Legacy ghost count + d7-domain.md 错配

**修改清单**：

| # | 文件 | 改动 |
|---|---|---|
| 2.1 | `openspec/specs/d7-orchestration/f-registry.md` D7-S1-A02-F01..F06 | 路径 `contextengine/tasks/task_manager.go` → `workmodel/task_manager.go` |
| 2.2 | `openspec/specs/d7-orchestration/f-registry.md` D7-S2-A01-F03 | `orchestrate` → `orchestratePath` |
| 2.3 | `openspec/specs/d7-orchestration/f-registry.md` D7-S2-A01-F04 | 路径修正为 `turn_orchestrator.go` |
| 2.4 | `openspec/specs/d7-orchestration/f-registry.md` D7-S5-A01-F01..F03 | 路径修正为 `classifier_fallback.go` |
| 2.5 | `openspec/specs/d7-orchestration/t-registry.md` D7-S4-T01..T09 | 路径加 `executionflow/` 前缀 |
| 2.6 | `openspec/specs/d7-orchestration/t-registry.md` Statistics | 补 S12/S13/S15/S16 + 验证总数 230 |
| 2.7 | `openspec/specs/d7-orchestration/pipeline-architecture.md` v1.1.0 → v1.2.0 | §2.1 13 S → 6 S + 1 横切；§1.1 ASCII 图同步；§6.3 路径加 `executionflow/` 前缀 |
| 2.8 | `openspec/specs/d7-orchestration/f-registry.md` 头部 | Legacy 41 ghost → "deprecated 2 + canonical 66 = 68" |
| 2.9 | `openspec/specs/d7-orchestration/a-registry.md` line 32-50 | 删 Legacy 双轨段（已无意义） |
| 2.10 | `openspec/specs/d7-orchestration/d7-domain.md` line 147 | "S6-A14 PLANNED" → IMPLEMENTED |
| 2.11 | `openspec/specs/d7-orchestration/span-registry.md` | 路径加 `executionflow/` 前缀（与 t-registry 一致） |

**AC**：f-registry 路径 100% 正确（grep 验证）；t-registry Statistics 14 S 全列（数字 = 230）；pipeline-architecture v1.2.0 与 d7-domain.md v2.5.1 完全一致。

### 4.4 子 Change #3：value-flow-rename（S 层语义升级）

**PR-5 范围**：纯文档（与上次 proposal 一致）

**修改清单**：d7-domain.md §North Star 6 S 加 ValueFlow Alias 列；a/f/t-registry 加 ValueFlow Semantic 列；terminal-state-guide.md §3 加"用户感知层"段。

| S 名 | ValueFlow Alias | 用户故事 |
|---|---|---|
| S1 WorkModel | **Multi-Step Task Coordination** | 用户想看到/管理/审阅多步任务进度 |
| S2 SessionOrchestrator | **Turn-Based Conversation** | 用户在多轮对话中获得确定性响应 |
| S3 WaveScheduler | **Parallel Worktree Execution** | 用户期望"快"——并行执行不冲突 |
| S4 ExecutionFlow + Verify | **Trustworthy Conclusion Delivery** | 用户收到的结论是可信、可验证的 |
| S5 DecisionPlanning + Observe | **Intent + Uncertainty Quantization** | 系统能识别意图并量化不确定性 |
| S6 MUPS Pipeline | **Learn from Outcome** | 系统能基于历史优化未来决策 |

**AC**：6/6 S 配 ValueFlow Alias；a/f/t-registry ValueFlow Semantic 列完整；不删旧 S 编号（DSAFT 原则 3）。

### 4.5 子 Change #4：t-span-coverage（A + F Span 增补）

**PR-6 + PR-7 + PR-8 范围**：5 ops span + 4 acceptance test + Span Evidence 列填充

**5 个新 ops span**：

| Span 名 | 触发点 | 覆盖 T |
|---|---|---|
| `D7_Orchestration_LongTerm_Reputation_Update` | `mups/learn/reputation/store.go::BayesianUpdate` | LP-1 长期信誉 4 T |
| `D7_Orchestration_Anomaly_Trigger` | `executionflow/verify/anomaly.go::SystemAnomalyDetector.Trigger` | SystemAnomaly 生产触发 8 T |
| `D7_Orchestration_AdaptivePrior_Inject` | `sessionorchestrator/observe.go::buildObserveRequest` | 跨 S5/S6 契约 6 T |
| `D7_Orchestration_Resume_Decision_Path` | `orchestrator.go::ApplyResumeSession` | ResumeSession 3 决策 6 T |
| `D7_Orchestration_Feishu_Card_Render` | `communication/feishu/progress.go::finalizeReplyCardStreaming` | D7→D1 决策可观测 |

**4 个 acceptance test**（tests/integration/d7_acceptance_*.go）：
- `d7_acceptance_lp1_test.go` — LP-1 长期 Bayesian 信誉
- `d7_acceptance_lp2_test.go` — LP-2 Pending 隔离
- `d7_acceptance_lp5_test.go` — LP-5 反向追溯
- `d7_acceptance_resume_test.go` — ResumeSession 3 决策

**AC**：5 ops span 注册到 coverage/registry_test.go；4 acceptance test PASS；t-registry Span Evidence 列覆盖率 ≥80%；observability-guide.md 新增 §"T-Without-Span Tracker"。

### 4.6 子 Change #5：boundary-decision（S + 横切）

**PR-9 范围**：纯文档 + 1 governance 文件（与上次 proposal 一致）

**修改清单**：d7-domain.md §Out of Scope 加 Pending Boundary Decision 列；design.md §Cross-Domain Boundary 加 Decision 表（3 项）；orchtypes/boundary_decision.go + test.go governance 文件。

**3 项 Decision**：
- **Decision 1**：ReputationEvidence（D7-S6 vs D6）— 保留当前归属，标注 boundary-borrowed，v7.0 重新评估
- **Decision 2**：SystemAnomaly（D7-S4 vs D5）— 保留当前归属，标注 boundary-borrowed，v7.0 重新评估
- **Decision 3**：AdaptivePrior（跨 S5+S6）— 保留当前共享，标注 boundary-borrowed，v7.0 重新评估

**AC**：Decision 表 3/3 完整；orchtypes/boundary_decision.go + test PASS；不删旧实现（DSAFT 原则 3）。

---

## 5. PR 序列与依赖图

```
PR-1 (#0 dead-code-cleanup, 纯删除 + doc 重写)
   ↓ no blocker
   ├─ PR-2 (#1 turn-fn-split 第一批: 拆 turn_loop.go)
   │   ↓
   │  PR-3 (#1 turn-fn-split 第二批: 拆 turn_invoke.go + turn_recovery.go)
   │   ↓
   │  PR-3-extended (#1 turn-fn-split 第三批: WorkTree 治理 — RollupReport + deterministic root + 3 T 点)
   │
   ├─ PR-4 (#2 registry-sync, 文档同步)
   │   ↓ no blocker
   │  PR-5 (#3 value-flow-rename, 纯文档)
   │   ↓
   │  PR-6 (#4 t-span-coverage 第一批: 5 ops span + coverage registry)
   │   ↓
   │  PR-7 (#4 t-span-coverage 第二批: 4 acceptance test)
   │   ↓
   │  PR-8 (#4 t-span-coverage 第三批: t-registry Span Evidence 列填充)
   │
   └─ PR-9 (#5 boundary-decision, 文档 + governance)

verify-archive.sh + S7_Archive
```

**PR 顺序依赖**：
- **PR-1 必须先**（删死代码后 turn_orchestrator.go 1551 → ~1500 行，更易拆分）
- **PR-2/3/3-extended 顺序**（先拆 turn_loop.go，再拆 invoke + recovery，最后 WorkTree 上行反馈治理）
- **PR-4/5/9 顺序**（先 registry-sync → value-flow-rename → boundary-decision）
- **PR-6/7/8 顺序**（先增 span → 后写 acceptance test → 最后填 registry 列）
- **PR-1 与 PR-4 可并行**（互不依赖）

---

## 6. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| 死代码删除破坏 hidden 引用 | Low | High | 双 Agent grep 0 外部调用者 + 22/22 -race PASS 守门 |
| turn_orchestrator.go 拆分破坏 turn 序列化（D7-S15） | Mid | High | D7-S15 TurnState 接口不变；按"职责切面"拆分；逐 PR 后 -race PASS |
| Registry 路径修复导致 spec drift | Low | Mid | 同步更新 t-registry + pipeline-architecture + span-registry |
| ValueFlow Alias 引入后文档阅读混乱 | Low | Mid | Alias 作为语义层叠加；不改 S 编号 |
| 5 ops span 与 telemetry/names.go 命名冲突 | Low | High | 前缀 `D7_Orchestration_*` + coverage registry 测试守门 |
| 4 acceptance test 触发 D5 路径异常 | Low | Low | 仅在 D7 域内 -race 测试 |
| 跨域 Decision 表引发其他域争议 | Mid | Mid | Decision 表显式"保留当前归属"；不推翻 v6.0.0 共识 |
| 10 PR 联动回归测试成本 | Mid | Mid | 每 PR 后 22/22 -race PASS 守门；最终 verify-archive.sh 12/12 |
| WorkTree RollupReport 改动引入 race | Mid | Mid | RollupReport 仅 process 内传递，不参与 TurnState 序列化；调用方丢弃返回值即可 |
| `sessionRootGoal` 排序改动破坏现有 rollup 行为 | Low | Mid | unit test 覆盖多 root 场景 + 排序前/后行为对比 |
| 3 T 点登记与现有 t-registry 编排冲突 | Low | Low | D7-S0-A07 新号段不污染现有 230 T；仅文档同步 |

---

## 7. Acceptance Criteria（~50 AC 全表）

### 7.1 AC 分类

| 类别 | AC 数 | 子 Change |
|---|---|---|
| **清理 AC**（AC-Clean） | 14 | #0 |
| **拆分 AC**（AC-Split） | 10 | #1 |
| **WorkTree AC**（AC-WT） | 6 | #1（含 RollupReport + deterministic root + 3 T 点） |
| **Registry AC**（AC-Reg） | 11 | #2 |
| **ValueFlow AC**（AC-VF） | 4 | #3 |
| **Span AC**（AC-Span） | 8 | #4 |
| **Boundary AC**（AC-Bound） | 3 | #5 |
| **总体 AC**（AC-Total） | 4 | 全部 |
| **总计** | **~60 AC** | - |

### 7.2 总体 AC（AC-Total）

| AC | 验收方式 | 必过 |
|---|---|---|
| AC-T1 | 22/22 orchestration packages -race PASS | ✅ |
| AC-T2 | verify-archive.sh 12/12 PASS | ✅ |
| AC-T3 | 193 P0 T 100% PASS（acceptance + unit + integration）| ✅ |
| AC-T4 | 10 PR 全部 squash merge + auto-merge | ✅ |

### 7.3 WorkTree AC（AC-WT，6 项，子 Change #1 扩展）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-WT1** | `rg "child\.LastRound" internal/layers/orchestration/workmodel/context_bubble_apply.go internal/layers/orchestration/workmodel/rollup_gate.go` 仅出现在 `NewRollupReportFromRound` 构造器内 | ✅ |
| **AC-WT2** | `rg "RollupReport" internal/layers/orchestration/workmodel/` ≥ 5 处（struct + 构造器 + 3 调用点）| ✅ |
| **AC-WT3** | `ReevaluateParentAfterChild` 函数签名 `(*RollupReport, error)`，3 调用点 `session_turn_loop.go:194` + `run_spawn.go:51` + `cli_commands.go:329` 全部丢弃返回值（兼容调用）| ✅ |
| **AC-WT4** | `sessionRootGoal` 函数体含 `sort.Slice` 调用，多 root unit test PASS（`workmodel/rollup_gate_test.go::TestSessionRootGoal_DeterministicOrder`）| ✅ |
| **AC-WT5** | `t-registry.md` 含 D7-S0-A07-T01（ApplyPipelineDecide 4 步顺序不变式，对应 `context_decide.go:4`）+ T02（ReevaluateParentAfterChild 3 调用点幂等性，对应 `resolve.go:7`）+ T03（Path A `ShouldRollupAfterChildren` + Path B `MaybeRootRollupFallback` 10 组合 unit test）| ✅ |
| **AC-WT6** | D7-S15 TurnState 接口不变 + 22/22 -race PASS | ✅ |

详细 6 AC 见 `specs/d7-orchestration/spec.md` §3.3 WorkTree AC 段。

---

## 8. Decision 记录

### Decision 1: 死代码删除 vs 保留

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 全部删除 | 0 LOC + 0 认知负担 | 未来可能需要（v1.1 计划未启） |
| **B: 全部删除（推荐）** | 同 A + git history 保留 | - |
| C: 全部保留 | 向后兼容 | 累计 ~126 LOC 永久债 |

**选择:** B
**理由:** 双 Agent grep 验证 0 外部调用者；DSAFT 原则 6 "分阶段终态"——v1.1 已 defer 到 v7.0，未启用则删；git history 完整保留可回溯。
**影响:** ~126 LOC 删除；0 业务逻辑改动；orchtypes/ -13 LOC + decisionplanning/ -26 LOC + workmodel/ -60 LOC + 其余 -27 LOC。

### Decision 2: turn_orchestrator.go 拆分粒度（4 文件）

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 拆 2 文件（主+invoke） | 改动小 | turn_loop + recovery 仍耦合 |
| B: 拆 3 文件（主+loop+invoke） | 中等粒度 | recovery 仍耦合 |
| **C: 拆 4 文件（主+loop+invoke+recovery，推荐）** | 4 个职责清晰独立 | 改动较大 |
| D: 拆 5 文件（主+loop+invoke+recovery+observe） | 更细粒度 | observe 跨 S5/S6 边界模糊 |

**选择:** C
**理由:** turn_orchestrator.go 1551 行的 4 个核心职责（路由 / 主循环 / LLM 调用 / 错误恢复）天然分离；recovery 单文件便于 v5 EscapeEngine 演进；observe 已在 observe_request.go + observe.go 独立。
**影响:** turn_orchestrator.go 1551 → <300 + 3 个新文件 <500 各；36 T 重映射到 4 子活动；0 函数签名变化。

### Decision 3: S6 是否拆 S6a + S6b（Execute + Learn）

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 拆 S6a + S6b | 一域一角色 | 7 个 S 太分散；编号空间再扩 |
| **B: 保留 S6，加内部 sub-header（推荐）** | 0 改动 + 显式分组 | 治标不治本 |
| C: 维持现状 | 0 改动 | 不暴露角色混合 |

**选择:** B
**理由:** v6.0.0 已 S7_Archived（14 S → 6 S），再次重组风险大；内部 sub-header 满足"显式分组"诉求；v7.0 重新评估。
**影响:** d7-domain.md §North Star 表 S6 行加 sub-header 区分 Pipeline Coordinator vs Memory Curator；0 编号变化。

### Decision 4: 5 ops span + 4 acceptance test 范围

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 5 span + 4 acceptance | 覆盖关键缺口 | 工作量大 |
| **B: 5 span + 4 acceptance（推荐）** | 同 A | - |
| C: 仅 3 span + 2 acceptance | 工作量小 | 覆盖不完整 |

**选择:** B
**理由:** 5 span 覆盖 LP-1 / SystemAnomaly / AdaptivePrior / Resume / D7→D1 5 类关键缺口；4 acceptance test 覆盖 LP-1/LP-2/LP-5/Resume 4 类闭环验证；其他缺口由 T-Without-Span Tracker 标注，渐进闭合。
**影响:** 3 PR 工作量；覆盖率 ~30% → ≥80%。

### Decision 5: 3 项 boundary debt 处置

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 全部下沉到 D5/D6 | 严格遵循原则 4 | 跨域工作量极大 + 推翻 v6.0.0 |
| **B: 全部保留 + Decision 表（推荐）** | 0 代码改动 + 显式决策 | 留"未来可能下沉"的债 |
| C: 仅 1 项下沉试点 | 渐进 | 试点项选择难 |

**选择:** B
**理由:** v6.0.0 已 S7_Archived，重新跨域下沉 = 推翻 v6.0.0 共识；Decision 表显式标注 = 给 v7.0 留 boundary 演进空间。
**影响:** 1 governance 文件 orchtypes/boundary_decision.go；decision 标注 "Future: re-evaluate at v7.0"。

### Decision 6: WorkTree 上行反馈治理（用户 2026-06-29 复盘触发）

| 方案 | 优点 | 缺点 |
|---|---|---|
| A: 不动 WorkTree | 0 改动 | 留下游消费侧不一致 + root 选择非确定债 |
| B: 单独立项 #7 worktree-rollup-clarity | 1 PR / 3 天，scope 清晰 | PR 总数 9 → 10，scope 边界硬 |
| **C: 纳入 #1 turn-fn-split（推荐）** | god function 拆分时同步治理 rollup 上行路径，1 个 PR 关闭 AC-Split + AC-WT | #1 scope 略扩（5.3 → 7.5 天） |

**选择:** C
**理由:** god function 拆分时正好同步治理 rollup 上行路径；3 处 `ReevaluateParentAfterChild` 调用点都在 turn loop 上下文；用户在 2026-06-29 会话明确推荐"纳入 #1 turn-fn-split"。`ReevaluateParentAfterChild` 函数签名变化（`struct{} → *RollupReport`）向后兼容（调用方丢弃返回值即可，0 业务代码改动）。
**影响:** #1 从 5.3 天 → 7.5 天；新增 1 个文件 `workmodel/rollup_report.go`；3 调用点签名同步；D7-S0-A07-T01..T03 登记；6 个新 AC（AC-WT1..WT6）。

---

## 9. 相关文档

- `demand.md` — 背景 + 7 类架构债 + 范围（含 WorkTree 传播/反馈债 §1.8）
- `tasks.md` — 55 任务清单（按 6 子 Change + WorkTree 4 任务）
- `design.md` — S→A→F 逐层详细设计 + §2.4 WorkTree 治理
- `specs/d7-orchestration/spec.md` — ~60 AC 全表 + Gherkin
- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.1 → v2.6.0