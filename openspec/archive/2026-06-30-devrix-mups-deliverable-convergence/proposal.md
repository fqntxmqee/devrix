# Proposal: MUPS 交付收敛 — LLM 战略提案 + Deliverable Gate

**Change ID:** `devrix-mups-deliverable-convergence`  
**Demand ID:** DM-20260630-012  
**Status:** S7_Archived  
**Created:** 2026-06-30

---

## 1. Problem Statement

Devrix MUPS + WorkTree 对 **review 类任务** 存在「能向下分解、不能向上合成」的结构性失败：

1. **战略层代码化**：Goal 强制 ExplorationPlan；无 scope 时 `DefaultDecomposeProposer` 固定拆 2 子任务；子任务 `max_iters=3` — LLM 未参与「单 WorkItem 即可完成」的判断。
2. **Execute 终止态≠交付态**：ReAct 在 `max_iters` 退出时，最后一轮常为 *"Let me continue exploring..."* + tool_calls，该文本进入 `ArtifactSummary`。
3. **向上反馈不诚实**：`VerdictPartial → TaskStatusCompleted`；Structured Bubble 不传 findings；父 Goal 可能标 Completed 而无合成结论。
4. **Session 出口选错**：`RunSessionTurnLoop` 优先 `lastArtifactSummary`（最后一轮 WorkItem），而非 `ExtractSessionDeliverable`（Goal rollup）。
5. **质量门路径分裂**：DM-20260630-011 的 LastTextQualityGate 覆盖 Turn finalize，**未覆盖** ItemPipeline complete。

用户明确要求：**战略由大模型提案，代码只守不变量** — 与现有 Observe G3（LLM 提案 Obs* → 规则校验）一致，应扩展到 Plan / Verify / Complete。

## 2. Proposed Solution

### 2.1 控制面 / 数据面分离

```
┌─────────────────────────────────────────────────────────┐
│ 控制面（LLM 提案，G3）                                   │
│  Observe: Obs* 提案 (已有 A75)                          │
│  Plan:    StrategicPlan JSON (NEW A76)                  │
│  Rollup:  合成报告 (Execute + schema 约束)               │
└─────────────────────────────────────────────────────────┘
                          ↓ 规则门控
┌─────────────────────────────────────────────────────────┐
│ 数据面（代码不变量）                                     │
│  scope 路径合法 / 深度≤MaxDepth / 子节点≤MaxChildren    │
│  blast radius / token budget / iter cap 上限             │
│  deliverable_schema 程序校验                           │
│  Session complete 必须合格 deliverable 或 TaskIncomplete │
└─────────────────────────────────────────────────────────┘
```

### 2.2 新增：StrategicPlanProposer @ Plan

**输入**：`UncertaintyReport` + directive + WorkItem kind/depth + optional ScopeContract  
**输出 JSON**（示例）：

```json
{
  "execution_mode": "single|decompose|parallel_probe",
  "scope_in": ["internal/layers/contextengine/kernel/"],
  "child_specs": [],
  "deliverable_schema": "p0_p1_file_line",
  "react_iters_hint": 5,
  "rationale": "14 files under one directory; single pass sufficient"
}
```

**门控（代码）**：

- `scope_in` 路径存在且在 workspace 内
- `decompose` → `child_specs` 非空且每 spec 含 `expected_return`
- `react_iters_hint` clamp 到 `[1, DefaultWorkItemMaxIters]`
- LLM 失败 → **fallback** `DefaultPlanner` + 现有 `DefaultDecomposeProposer`（行为不回归）

**Wire**：`bootstrap/wire_item_pipeline.go` 注入 `NewLLMStrategicPlanProposer(llm, ctx, locale)`，与 `LLMObservationProposer` 对称。

### 2.3 新增：DeliverableVerifier @ Verify

对 `ExpectedReturn` / `deliverable_schema` 做 **程序校验**（首期 `p0_p1_file_line`）：

- 必须匹配 `file:line` citation 正则（复用 `item_verify.fileLineCitationRE`）
- 必须含 P0 或 P1 结构标记（heading 或 JSON field）
- `metadata.stop_reason == max_iters` **且** 无 citation → Verdict **Incomplete**（新 kind 或 Fail+reason）

**Status 映射变更**：

- `StatusAfterSpawnNone(Partial)` ** alone** 不再一律 Completed
- 新增 `DeliverableStatus` on round：`complete | incomplete | not_applicable`
- 仅 `DeliverableStatus=complete` + Pass/Partial(with deliverable) → Completed

### 2.4 向上：StructuredDeliverable Bubble

扩展 `WorkItemPipelineRound`：

```go
StructuredDeliverable *DeliverablePayload // parsed findings JSON or nil
```

- Execute 最后一轮 appendix 要求：无 tool_calls 时输出 schema 合规 JSON
- Verify 解析写入 round；Bubble `StructuredBubbleStatement` 携带 findings digest（截断至 token budget）
- Rollup Execute 输入 = 子节点 `StructuredDeliverable` 合并，非空时才可 Pass rollup verify

### 2.5 Session 出口：Deliverable Gate

`RunSessionTurnLoop` 终止时：

1. `content = ExtractSessionDeliverable(tm, sessionID)` **优先**
2. 若空 → `lastArtifactSummary`（兼容单 WorkItem 无 rollup）
3. `EmitLastTextQualityGate(content)` → meta `summary_quality` / `final_quality`
4. 两者均 bad → `TaskIncompleteMessage`（复用 DM-20260630-011）
5. `complete` EngineEvent 携带 meta（D1 EmitComplete 已有逻辑）

扩展过渡句 marker：`let me continue`, `let me read`, `let me explore`, `i'll examine`（EN）+ 中文等价。

## 3. Scope

### In Scope

- `StrategicPlanProposer` + bootstrap wire + 单测
- `DeliverableVerifier` + `StatusAfterSpawnNone` 条件化
- `StructuredDeliverable` round 字段 + bubble 载荷
- `session_turn_loop.go` complete 优先级 + quality gate
- spec delta + t-registry
- 集成测试：`TestItemPipeline_ReviewKernel_ConvergesOrIncomplete`

### Out of Scope

- LLM SpawnPolicyEvaluator（Phase 2）
- 删除 `DefaultDecomposeProposer`
- 新 devrix.yaml 配置项
- 全目录 batch-read 新 tool

## 4. Impact Analysis

| 组件 | 变更 | 详情 |
|------|------|------|
| D7 Plan | **Yes** | LLM StrategicPlanProposer；DefaultPlanner 降级为 fallback |
| D7 Verify | **Yes** | Deliverable schema gate；Partial 语义收紧 |
| D7 WorkModel | **Yes** | Round 字段；Status 映射；Bubble payload |
| D7 SessionTurnLoop | **Yes** | complete deliverable 优先 + quality gate |
| D1 Communication | **Minor** | 复用 011 EmitComplete；无新 adapter 逻辑 |
| D2 Context | **Minor** | Plan proposer 走 D2 Prepare（同 Observe A75 模式） |

## 5. Success Criteria

- [ ] review kernel 指令：complete 为 P0/P1 报告 **或** TaskIncompleteMessage，**never** 纯探索过渡句
- [ ] LLM 提案 `execution_mode=single` 时，单 WorkItem 完成（Jaeger 无 SpawnDecompose）
- [ ] 子任务 max_iters 无 deliverable → 非 Completed；父 NeedsRollup 触发合成轮
- [ ] Item pipeline complete 事件含 `summary_quality` attribute
- [ ] 单元 + 集成测试绿；t-registry 更新

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| LLM Plan 提案不稳定 | Med | rule fallback；ValidateStrategicPlan 门控 |
| 收紧 Partial→Completed 阻塞旧 E2E | Med | `deliverable_schema=not_applicable` 保留旧行为 |
| Rollup 合成再 hit max_iters | Med | rollup `max_iters=2` 已有；合成 prompt 强制 JSON-only |
| 与 011 重复 | Low | 012 显式引用 011 helper，不 fork 逻辑 |

## 7. Phasing

| Phase | 内容 | 交付 |
|-------|------|------|
| **P0** | DeliverableVerifier + Session complete gate + Item LastTextQualityGate | 用户不见过渡句 |
| **P1** | StrategicPlanProposer + StructuredDeliverable bubble | LLM 决定 single vs decompose |
| **P2** | LLM Spawn/Decide 提案（登记 backlog，本 change 不编码） | — |

---

## Archive Information

**Archived:** 2026-06-30
**Duration:** 1 day
**Outcome:** Successfully implemented (P0 + P1; P2 backlog deferred)
**PR:** [#353](https://github.com/fqntxmqee/devrix/pull/353) squash merged `3288ddb0`

### Specs Updated
- `openspec/specs/d7-orchestration/spec.md` — v4.20.0 lite-mode 契约 + 范式 2 交付收敛
- `openspec/specs/d7-orchestration/t-registry.md` — 8 T points (D7-S5/S9/S15/S16/S2)
- `openspec/specs/d7-orchestration/CHANGELOG.md` — timeline entry

### Key Files
- `sessionorchestrator/{deliverable_verify,strategic_plan_proposer,session_complete,session_turn_loop}.go`
- `workmodel/{pipeline_apply,deliverable,expected_return,context_bubble_apply}.go`
- `bootstrap/wire_item_pipeline.go`
- `.cursor/rules/orchestration-no-tactical-hardcoding.mdc`
