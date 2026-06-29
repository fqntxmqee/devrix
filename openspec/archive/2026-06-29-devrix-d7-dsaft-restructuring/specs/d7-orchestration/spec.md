# D7 Orchestration Spec Delta — 深度 DSAFT 重构 + 清理 (DM-20260629-001)

**Change ID:** devrix-d7-dsaft-restructuring
**Demand ID:** DM-20260629-001
**Delta Type:** MODIFIED (v2.5.1 → v2.6.0)
**SoT:** `openspec/specs/d7-orchestration/{d7-domain,design,a,f,t,span}-registry.md` + `internal/layers/orchestration/`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. 删 ~126 LOC 死代码 + 老链路 | `internal/layers/orchestration/{orchtypes,decisionplanning,delegatetools,escape,sessionorchestrator,workmodel}/` | MODIFIED | 0 业务逻辑改动 |
| 2. 重写 4 处 doc drift | `sessionorchestrator/turn_doc.go` + `orchtypes/{config,routing}.go` + `delegatetools/subquery_fallback.go` | MODIFIED | 0 代码改动 |
| 3. 拆 `turn_orchestrator.go` 1551 行 → 4 文件 | `sessionorchestrator/{turn_orchestrator,turn_loop,turn_invoke,turn_recovery}.go` | MODIFIED | 0 函数签名变化 |
| 4. 36 T 重映射到 D7-S2-A06/A07/A08/A09 4 子活动 | `openspec/specs/d7-orchestration/{a,t}-registry.md` | MODIFIED | 0 代码改动 |
| 5. F 路径修正 6 处 + 命名修正 2 处 | `f-registry.md` | MODIFIED | 0 代码改动 |
| 6. t-registry Statistics 补完（S12/S13/S15/S16）| `t-registry.md` | MODIFIED | 0 代码改动 |
| 7. pipeline-architecture.md v1.1.0 → v1.2.0 | `pipeline-architecture.md` | MODIFIED | 0 代码改动 |
| 8. Legacy 41 ghost → 实际 deprecated 2 + canonical 66 | `f-registry.md` + `a-registry.md` | MODIFIED | 0 代码改动 |
| 9. 6 S 配 ValueFlow Alias 列 | `d7-domain.md` §North Star | MODIFIED | 0 代码改动 |
| 10. a/f/t-registry 加 ValueFlow Semantic 列 | `a-registry.md` + `f-registry.md` + `t-registry.md` | MODIFIED | 0 代码改动 |
| 11. t-registry 加 Span Evidence 列（覆盖率 ≥80%）| `t-registry.md` | MODIFIED | 0 代码改动 |
| 12. observability-guide.md §"T-Without-Span Tracker" | `observability-guide.md` | NEW | - |
| 13. 5 个新 ops span 注册 | `telemetry/names.go` + `sessionorchestrator/spans.go` + `coverage/registry_test.go` | MODIFIED | +5 Span |
| 14. 4 个 acceptance test（LP-1/LP-2/LP-5/Resume）| `tests/integration/d7/d7_acceptance_*.go` | NEW | - |
| 15. 3 项 boundary debt Decision 表 | `d7-domain.md` §Out of Scope + `design.md` §Cross-Domain Boundary | MODIFIED | 0 代码改动 |
| 16. orchtypes/boundary_decision.go governance 文件 | `orchtypes/` | NEW | - |

总计 **10 PR / 55 任务 / ~21.2 工作日 / 60 AC**（含 WorkTree 上行反馈治理纳入 #1）。

---

## 2. 关键约束

### 2.1 不删除 S 旧编号（DSAFT 原则 3）

S1-S6 名称保留为包名引用，ValueFlow Alias 作为"语义层"叠加。**任何 T 注释、Go 文件命名、a/f/t-registry.md 表中 S 编号保持不变**。

### 2.2 不下沉 3 项能力

3 项 boundary debt 在 v2.6.0 显式标注"boundary-borrowed"，**保留当前 D7 归属**，由 v7.0 重新评估。

### 2.3 0 函数签名变化（pure physical migration）

turn_orchestrator.go 拆分、~126 LOC 死代码删除、跨包 F 升格全部 0 函数签名变化。**v6.0.0 已闭合的 4 轮物理迁移成果保留**。

#### 2.3.1 唯一例外：`ReevaluateParentAfterChild` 返回类型变化（向后兼容）

**位置**：`internal/layers/orchestration/workmodel/resolve.go::ReevaluateParentAfterChild`

**变更**：返回类型 `(struct{}, error) → (*RollupReport, error)`

**向后兼容机制**：
- 3 调用点（`session_turn_loop.go:194` + `run_spawn.go:51` + `cli_commands.go:329`）可继续丢弃返回值（Go 允许）
- **0 业务代码改动**：调用方现有调用形式 `workmodel.ReevaluateParentAfterChild(...)` 保持不变
- RollupReport 仅 process 内传递，**不参与** D7-S15 TurnState 序列化（DM-20260628-003 闭环成果保留）
- 验证：`go vet ./internal/layers/orchestration/workmodel/...` PASS + `go test -race ./...` PASS

**Why 必要**：DM-20260629-001 暴露 `child.LastRound` 作隐式 envelope 的 5 处自由取字段债，强类型 RollupReport 取代隐式 envelope 必须经 `ReevaluateParentAfterChild` 返回——这是本 Change 唯一函数签名变化。

**影响范围**：仅 `workmodel/resolve.go` + 3 调用方文件（全部向后兼容）；其他 ~50 文件 0 函数签名变化约束保留。

### 2.4 5 节点管道数据契约不变

UncertaintyReport → Plan → Artifact → Verdict → ReputationEvidence 5 节点数据契约在 v2.6.0 完全不变；本次只在 spec/observability/governance 层 + 死代码清理层修改。

### 2.5 4 轮物理迁移成果保留

v6.0.0 已闭合的 4 轮迁移（hardening / turn→sessionorchestrator / verify-promote / bootstrap-slim）的 0 函数签名变化成果在 v2.6.0 完全保留。

---

## 3. 54 AC 全表

### 3.1 总体 AC（AC-Total，4 项）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-T1** | 22/22 orchestration packages -race PASS | ✅ |
| **AC-T2** | verify-archive.sh 12/12 PASS | ✅ |
| **AC-T3** | 193 P0 T 100% PASS | ✅ |
| **AC-T4** | 10 PR 全部 squash merge + auto-merge | ✅ |

### 3.2 清理 AC（AC-Clean，14 项，子 Change #0）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-C1** | `rg "ErrInvalidVerdictKind" internal/layers/orchestration/` = 0 | ✅ |
| **AC-C2** | `rg "LLMFallback\|ShadowLLMClassify\|FastPathThreshold\|RuleOrchestrateConfig" internal/` = 0 | ✅ |
| **AC-C3** | `rg "rule_orchestrate\|RuleOrchestrate" internal/` = 0 | ✅ |
| **AC-C4** | `rg "orchtypes\.ArtifactStateChangeCert\|orchtypes\.ArtifactProbeReport" internal/` = 0 | ✅ |
| **AC-C5** | `rg "TurnSystemPrompt\|loopFirstSystemPrompt" internal/` = 0 | ✅ |
| **AC-C6** | `rg "BuildSubQueryRunner" internal/` = 0 | ✅ |
| **AC-C7** | `rg "time\.Now" internal/layers/orchestration/escape/engine.go` = 0 | ✅ |
| **AC-C8** | `rg "DM-20260620-003\|Legacy explicit-code" internal/` = 0 | ✅ |
| **AC-C9** | `rg "AggregateVerdicts\|AggregationStrategy" internal/layers/orchestration/` = 0 | ✅ |
| **AC-C10** | `rg "v2.0 Slice\|FastPath calls" internal/layers/orchestration/sessionorchestrator/turn_doc.go` = 0 | ✅ |
| **AC-C11** | orchtypes/config.go header 不含 v1.0 dead fields 描述 | ✅ |
| **AC-C12** | orchtypes/routing.go docstring 不含 rule_orchestrate arm 描述 | ✅ |
| **AC-C13** | delegatetools/subquery_fallback.go docstring 不含 AC6/AC10 引用 | ✅ |
| **AC-C14** | `go test ./...` 0 失败 | ✅ |

### 3.3 拆分 AC（AC-Split，10 项，子 Change #1）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-S1** | `wc -l internal/layers/orchestration/sessionorchestrator/turn_orchestrator.go` < 300 | ✅ |
| **AC-S2** | `wc -l internal/layers/orchestration/sessionorchestrator/turn_loop.go` < 500 | ✅ |
| **AC-S3** | `wc -l internal/layers/orchestration/sessionorchestrator/turn_invoke.go` < 500 | ✅ |
| **AC-S4** | `wc -l internal/layers/orchestration/sessionorchestrator/turn_recovery.go` < 500 | ✅ |
| **AC-S5** | 0 函数签名变化（git diff --stat 验证）| ✅ |
| **AC-S6** | D7-S15 TurnState 接口不变（grep 验证）| ✅ |
| **AC-S7** | D7-S2-A06/A07/A08/A09 4 子活动在 a-registry 登记 | ✅ |
| **AC-S8** | 36 T 全部归属正确（t-registry 验证）| ✅ |
| **AC-S9** | a-registry A 总数 49 → 52 | ✅ |
| **AC-S10** | 22/22 orchestration packages -race PASS | ✅ |

### 3.3.1 WorkTree AC（AC-WT，6 项，子 Change #1 扩展，用户 2026-06-29 复盘触发）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-WT1** | `rg "child\.LastRound\.(ContextBubbleKind\|ArtifactSummary\|SpawnPolicy)" internal/layers/orchestration/workmodel/` 仅出现在 `NewRollupReportFromRound` 构造器内 | ✅ |
| **AC-WT2** | `rg "RollupReport" internal/layers/orchestration/workmodel/rollup_report.go` ≥ 8 处（struct 7 字段 + 构造器 + receiver）| ✅ |
| **AC-WT3** | `ReevaluateParentAfterChild` 函数签名 `(*RollupReport, error)`，3 调用点 `session_turn_loop.go:194` + `run_spawn.go:51` + `cli_commands.go:329` 全部丢弃返回值（兼容调用，0 业务代码改动）| ✅ |
| **AC-WT4** | `sessionRootGoal` 函数体含 `sort.Slice` 调用；多 root unit test PASS（`workmodel/rollup_gate_test.go::TestSessionRootGoal_DeterministicOrder`：构造 3 root 顺序打乱 → 始终返回 ID 最小）| ✅ |
| **AC-WT5** | `t-registry.md` 含 D7-S0-A07-T01（ApplyPipelineDecide 4 步顺序不变式，对应 `context_decide.go:4`）+ T02（ReevaluateParentAfterChild 3 调用点幂等性，对应 `resolve.go:7`）+ T03（Path A `ShouldRollupAfterChildren` + Path B `MaybeRootRollupFallback` 10 组合 unit test）；t-registry Statistics 总数 276 → 279 | ✅ |
| **AC-WT6** | D7-S15 TurnState 接口不变 + RollupReport 不参与 TurnState 序列化 + 22/22 -race PASS | ✅ |

### 3.4 Registry AC（AC-Reg，11 项，子 Change #2）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-R1** | `rg "contextengine/tasks/task_manager" openspec/` = 0 | ✅ |
| **AC-R2** | `rg "orchestrator\.go.*EventPublisher.*orchestrate[^P]" openspec/` = 0 | ✅ |
| **AC-R3** | `rg "orchestration/workplan/\|orchestration/imsink/" openspec/specs/d7-orchestration/t-registry.md` = 0 | ✅ |
| **AC-R4** | t-registry Statistics 总数与 Scenario 求和一致 = 230 | ✅ |
| **AC-R5** | t-registry 14 S 全列（S1-S6 + S8-S16）| ✅ |
| **AC-R6** | pipeline-architecture.md 版本 v1.2.0 + 6 S + 1 横切 | ✅ |
| **AC-R7** | `rg "Legacy 41" openspec/` = 0 | ✅ |
| **AC-R8** | f-registry Statistics: "deprecated 2 + canonical 66 = 68" | ✅ |
| **AC-R9** | a-registry Legacy 双轨段删除 | ✅ |
| **AC-R10** | d7-domain.md line 147 "S6-A14 PLANNED" → IMPLEMENTED | ✅ |
| **AC-R11** | span-registry 路径加 `executionflow/` 前缀 | ✅ |

### 3.5 ValueFlow AC（AC-VF，4 项，子 Change #3）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-V1** | d7-domain.md §North Star 表 6 S 配 ValueFlow Alias | ✅ |
| **AC-V2** | a-registry 49 A 全部加 ValueFlow Semantic 列 | ✅ |
| **AC-V3** | f-registry 68 F 全部加 ValueFlow Semantic 列 | ✅ |
| **AC-V4** | t-registry 230 T 全部加 ValueFlow Semantic + Span Evidence 列 | ✅ |

### 3.6 Span AC（AC-Span，8 项，子 Change #4）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-Sp1** | `rg "OpD7_Orchestration_(LongTerm\|Anomaly\|AdaptivePrior\|Resume\|Feishu)" telemetry/names.go` = 5 | ✅ |
| **AC-Sp2** | `rg "SinceVersion.*2\.6\.0.*Instrumented.*true" sessionorchestrator/spans.go` ≥ 5 | ✅ |
| **AC-Sp3** | `rg "D7_LongTerm_Reputation_Update\|D7_Anomaly_Trigger\|D7_AdaptivePrior_Inject\|D7_Resume_Decision_Path\|D7_Feishu_Card_Render" coverage/registry_test.go` = 5 | ✅ |
| **AC-Sp4** | `go test -tags=acceptance ./tests/integration/d7/d7_acceptance_lp1_test.go` PASS | ✅ |
| **AC-Sp5** | `go test -tags=acceptance ./tests/integration/d7/d7_acceptance_lp2_test.go` PASS | ✅ |
| **AC-Sp6** | `go test -tags=acceptance ./tests/integration/d7/d7_acceptance_lp5_test.go` PASS | ✅ |
| **AC-Sp7** | `go test -tags=acceptance ./tests/integration/d7/d7_acceptance_resume_test.go` PASS | ✅ |
| **AC-Sp8** | t-registry Span Evidence 列覆盖率 ≥80% | ✅ |

### 3.7 Boundary AC（AC-Bound，3 项，子 Change #5）

| AC | 验收方式 | 必过 |
|---|---|---|
| **AC-B1** | d7-domain.md §Out of Scope 表 3 项 boundary debt 显式标注 | ✅ |
| **AC-B2** | design.md §Cross-Domain Boundary Decision 表 3 项完整 | ✅ |
| **AC-B3** | `go test ./internal/layers/orchestration/orchtypes/` PASS | ✅ |

### 3.8 总计

| 类别 | AC 数 |
|---|---|
| AC-Total | 4 |
| AC-Clean | 14 |
| AC-Split | 10 |
| **AC-WT**（WorkTree 上行反馈治理） | **6** |
| AC-Reg | 11 |
| AC-VF | 4 |
| AC-Span | 8 |
| AC-Bound | 3 |
| **总计** | **60 AC** |

---

## 4. Gherkin 验收

### Scenario: ~126 LOC 死代码 + 老链路清理

```gherkin
Given: D7 v2.5.1 orchestration 28.9K LOC + ~126 LOC 死代码 + 老链路
When: PR-1 完成（子 Change #0 dead-code-cleanup）
Then:
  - 12 项死符号全删（grep 验证 0 命中）
  - 4 处 doc drift 重写
  - 22/22 orchestration packages -race PASS
  - 0 函数签名变化
  - 5 处 test 调用方迁移完成
```

### Scenario: turn_orchestrator.go God Function 拆分

```gherkin
Given: turn_orchestrator.go 1551 行（超 800 行硬上限近 2 倍）+ 36 T 全部挂 D7-S2-A06 单一函数
When: PR-2 + PR-3 完成（子 Change #1 turn-fn-split）
Then:
  - turn_orchestrator.go <300 行（主入口）
  - turn_loop.go <500 行（SessionTurnLoop + iter loop + escape 检查）
  - turn_invoke.go <500 行（LLMInvoke + ToolRound + ReAct）
  - turn_recovery.go <500 行（EscapeEngine + ResumeSession + Error Recovery）
  - D7-S2-A06 拆 D7-S2-A06/A07/A08/A09 4 子活动
  - 36 T 全部归属正确（不再单挂一函数）
  - a-registry A 总数 49 → 52
  - D7-S15 TurnState 接口不变
  - 22/22 orchestration packages -race PASS
```

### Scenario: WorkTree 上行反馈机制清晰度提升（用户 2026-06-29 复盘触发）

```gherkin
Given: D7 v2.5.1 WorkTree 向上反馈 3 类债：
  - ReevaluateParentAfterChild 3 调用点缺统一治理
  - child.LastRound 作隐式 envelope（5 处自由取字段）
  - sessionRootGoal map 遍历非确定性
When: PR-3-extended 完成（子 Change #1 扩展 WorkTree 上行反馈治理）
Then:
  - workmodel/rollup_report.go NEW 包含 RollupReport struct（7 字段：5 数据 + ChildID + GeneratedAt + NewRollupReportFromRound 构造器）
  - ReevaluateParentAfterChild 函数签名 (struct{}, error) → (*RollupReport, error)，向后兼容（3 调用点丢弃返回值即可）
  - 5 处 child.LastRound.{ContextBubbleKind|ArtifactSummary|SpawnPolicy} 全部替换为 NewRollupReportFromRound(...)
  - sessionRootGate 改为 sort.Slice 按 item.ID 排序，多 root unit test PASS
  - D7-S0-A07-T01..T03 在 t-registry 登记（ApplyPipelineDecide 4 步顺序 + ReevaluateParentAfterChild 3 调用点幂等 + Path A `ShouldRollupAfterChildren` + Path B `MaybeRootRollupFallback` 10 组合 unit test）
  - t-registry Statistics 总数 276 → 279
  - D7-S15 TurnState 接口不变（RollupReport 不参与 TurnState 序列化）
  - 22/22 orchestration packages -race PASS
  - 6 个新 AC 全部 PASS（AC-WT1..WT6）
```

### Scenario: Registry 路径全对

```gherkin
Given: D7 v2.5.1 f-registry 6 F 路径错误 + t-registry Statistics 过期 + pipeline-architecture v1.1.0 13 S
When: PR-4 完成（子 Change #2 registry-sync）
Then:
  - 6 F 路径全对（contextengine/tasks/task_manager → workmodel/task_manager）
  - 2 F 命名修正（orchestrate → orchestratePath + EventPublisher 路径）
  - t-registry D7-S4-T01..T09 路径加 executionflow/ 前缀
  - t-registry Statistics 补 S12/S13/S15/S16 + 总数 230
  - pipeline-architecture v1.2.0 6 S + 1 横切
  - Legacy 41 ghost → "deprecated 2 + canonical 66 = 68"
  - a-registry Legacy 双轨段删除
  - d7-domain.md line 147 "S6-A14 PLANNED" → IMPLEMENTED
```

### Scenario: S 层语义升级 ValueFlow Alias

```gherkin
Given: D7 v2.5.1 d7-domain.md §North Star 表只有 3 列
When: PR-5 完成（子 Change #3 value-flow-rename）
Then:
  - §North Star 表新增 "ValueFlow Alias" 列
  - 6 S 全部配 ValueFlow Alias（Multi-Step Task Coordination / Turn-Based Conversation / Parallel Worktree Execution / Trustworthy Conclusion Delivery / Intent + Uncertainty Quantization / Learn from Outcome）
  - a-registry 49 A 加 ValueFlow Semantic 列
  - f-registry 68 F 加 ValueFlow Semantic 列
  - t-registry 230 T 加 ValueFlow Semantic + Span Evidence 列
  - 不删旧 S 编号（DSAFT 原则 3）
```

### Scenario: T↔Span 覆盖率 ≥80%

```gherkin
Given: D7 v2.5.1 t-registry 230 T 中 ~30% 有 Span Evidence
When: PR-6 + PR-7 + PR-8 完成（子 Change #4 t-span-coverage）
Then:
  - 5 个新 ops span 注册（LongTerm_Reputation_Update / Anomaly_Trigger / AdaptivePrior_Inject / Resume_Decision_Path / Feishu_Card_Render）
  - 4 个 acceptance test PASS（LP-1 / LP-2 / LP-5 / Resume）
  - t-registry Span Evidence 列覆盖率 ≥80%
  - observability-guide.md §"T-Without-Span Tracker" 列出剩余缺口
  - coverage/registry_test.go 期望列表加 5 个新 Op（CI 守门）
```

### Scenario: 3 项 boundary debt 显式 governance

```gherkin
Given: D7 v2.5.1 design.md 无 §Cross-Domain Boundary 章节
When: PR-9 完成（子 Change #5 boundary-decision）
Then:
  - d7-domain.md §Out of Scope 表 3 项 boundary debt 显式标注
  - design.md §Cross-Domain Boundary Decision 表 3 项完整（ReputationEvidence / SystemAnomaly / AdaptivePrior）
  - orchtypes/boundary_decision.go 暴露 3 个常量
  - orchtypes/boundary_decision_test.go PASS
  - 0 函数签名变化（仅 governance 常量 + 测试）
```

---

## 5. 行为不变保证

- **5 节点管道数据契约**（UncertaintyReport / Plan / Artifact / Verdict / ReputationEvidence）完全不变
- **4 轮物理迁移成果**（hardening / turn→sessionorchestrator / verify-promote / bootstrap-slim）0 函数签名变化保留
- **23 ops + 9 sessionSpan attr 既有 span**完全不变（仅增量 +5）
- **230 T 既有 T**完全不变（仅加 Span Evidence + ValueFlow Semantic 列 + 新增 T 在 D7-S0 新号段）
- **bootstrap wire 拓扑**（InitOrchestration ≤ 200 行）完全不变
- **Hard Ban 三连 + Out of Scope 6 项**既有边界完全不变

---

## 6. T 点增量（v2.5.1 → v2.6.0）

### 6.1 D7-S0 Meta-Scenario（治理 + 清理 T 点）

| T ID | 描述 | 归属 | Status |
|------|------|------|--------|
| D7-S0-A00-T01..T14 | dead-code-cleanup 14 项 | Meta | PENDING (S4) |
| D7-S0-A02-T01..T10 | registry-sync 10 项 | Meta | PENDING (S4) |
| D7-S0-A03-T01..T05 | value-flow-rename 5 项 | Meta | PENDING (S4) |
| D7-S0-A04-T01..T10 | t-span-coverage 10 项 | Meta | PENDING (S4) |
| D7-S0-A05-T01..T04 | boundary-decision 4 项 | Meta | PENDING (S4) |
| D7-S0-A06-T01..T03 | verify-archive 3 项 | Meta | PENDING (S4) |
| D7-S0-A07-T01..T03 | WorkTree 上行反馈治理 3 项（PR-3-extended）| Meta | PENDING (S4) |

合计 **49 个新 T 点 P1**（在 D7-S0 新号段 A00-A07，不污染现有 230 T）。

### 6.2 D7-S2 重映射（god activity 拆分）

| T ID | 描述 | 归属 | Status |
|------|------|------|--------|
| D7-S2-A06-T01..T36 (重映射) | 36 T 拆 D7-S2-A06/A07/A08/A09 | S2 | PENDING (S4) |

### 6.3 总计

| 类别 | T 数 |
|---|---|
| 既有 T (v2.5.1) | 230 |
| 新增 D7-S0-A00..A06 | 46 |
| 新增 D7-S0-A07（WorkTree 治理，PR-3-extended）| 3 |
| 重映射 D7-S2-A06 → A06-A09 | 36（保持原 ID） |
| **v2.6.0 总 T** | **279** |

---

## 7. 相关文档

- `openspec/changes/devrix-d7-dsaft-restructuring/demand.md` — 6 类架构债 + 范围 + 风险
- `openspec/changes/devrix-d7-dsaft-restructuring/proposal.md` — 6 子 Change + 5 Decision
- `openspec/changes/devrix-d7-dsaft-restructuring/tasks.md` — 51 任务清单
- `openspec/changes/devrix-d7-dsaft-restructuring/design.md` — S→A→F 逐层详细设计
- `openspec/specs/d7-orchestration/d7-domain.md` v2.5.1 → v2.6.0
- `openspec/specs/d7-orchestration/{a,f,t,span}-registry.md`
- `openspec/specs/d7-orchestration/design.md` §Cross-Domain Boundary (NEW)
- `openspec/specs/d7-orchestration/pipeline-architecture.md` v1.1.0 → v1.2.0
- `openspec/specs/d7-orchestration/observability-guide.md` §T-Without-Span Tracker (NEW)
- `internal/layers/orchestration/orchtypes/boundary_decision.go` (NEW)
- `internal/layers/observability/instrument/telemetry/names.go` (+5 Op)
- `tests/integration/d7/d7_acceptance_{lp1,lp2,lp5,resume}_test.go` (NEW)
- `docs/methodology/dsaft-methodology.md` v4.0.0
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0