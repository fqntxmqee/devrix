# Demand: Context Budget & Isolation — Phase C (Sub-Agent Nested 分支 Budget 治理)

**Demand ID:** DM-20260620-002
**Change ID:** 2026-06-20-devrix-context-budget-phase-c-nested
**Created:** 2026-06-20
**Status:** S1_Demand
**Priority:** P0
**DSAFT Domain:** context-engine, multi-agent (D2/D7 联合)
**Owner:** devrix core
**Parent:** DM-20260620-001 (Phase A — S7_Archived, PR #128 + #129)
       + DM-20260620-001-B (Phase B — S7_Archived, PR #130-#132)

---

## Background

Phase A（PR #128）治理**同 turn 体积**：tool result cap (AC1)、assistant fold (AC2)、per-iter audit (AC4)、feishu precheck (AC5)、TruncateToTokens (AC13)。

Phase B（PR #130-#132）治理**子 agent 入口隔离**：3-mode dispatch (AC6/AC8/AC11a)、depth cap (AC9)、tool schema 暴露 mode (AC10)、D5 spans 22 步回归 ≤ 40K (AC12)。

但 2026-06-20 用户发"深度 review 项目"指令，devrix 走 4 路并行（TaskDecomposer 拆解 + LLM Decomposer 或 N 个 delegate_* 触发），**全部失败，LLM 调用报 "messages too long"**。Phase A/B 都无法解决此问题，因为**根因不在入口 messages 数量，也不在单 turn 体积**。

## Problem Statement

`internal/layers/orchestration/turn/orchestrator.go:221-268` `runLoop` 的 nested 分支：

```go
nested := isNestedScope(req.Scope) || len(req.PreloadedMessages) > 0
if nested {
    systemPrompt = strings.TrimSpace(req.SystemPrompt)
    messages = append([]types.Message{}, req.PreloadedMessages...)
    messages = append(messages, req.UserMessage)
    // ↑ 关键：这里 **不调用 o.context.Prepare**
    // ↑ 所以 prepared 是 zero value → maxContextTokens = 0
}
```

**`maxContextTokens = 0` 触发 4 个 no-op**（Phase A 的全部 budget 控制失效）：

1. `runTokenAudit`（line 894-904）守卫 `if ... || maxContextTokens <= 0 { return }` → 不做 audit
2. `ShouldFoldProactively` 同理不触发 → assistant 长输出不 head/tail fold
3. ToolResultStore cap 仅在 Prepare 路径生效 → nested 路径 tool result **不截断**
4. `budgetTracker.shouldStopDiminishing(maxContextTokens)` 永远 false → 永不停止

而 Phase B 只解决了 **入口 messages 隔离**（brief/fork/full + depth 限制），**没解决 nested 多轮工具调用累积**——这是两个独立维度。

### 4 路并行 deep review 失败复现链路

```
Leader session (D7 → IntentOrchestrate)
  ↓ TaskDecomposer.buildNodes (默认串行 DependsOn, 4 路真并行需 LLM Decomp 或 N 个 delegate_*)
  ↓ WaveScheduler.Start (DefaultPoolCapacity.SubAgent=3, 第 4 个 queue 等)
  ↓ ContextResolver.Resolve (ContextFresh) → 1 条 user(directive)
  ↓ SubAgentRunner.Run → enforce.RunBackground → SubQuery.Run
  ↓ SubTurnRunner.RunSubTurn (brief mode) → PreloadedMessages=nil, UserMessage=directive
  ↓ Orchestrator.RunTurn (nested branch, TurnScopeWaveWorker)
  ↓ runLoop 多轮工具调用 → messages 累积无 audit → LLM reject (>100K tokens)
```

## Phase A → Phase B → Phase C 衔接

| 维度 | Phase A (PR #128) | Phase B (PR #130-#132) | **Phase C (本文)** |
|------|-------------------|------------------------|---------------------|
| Tool result 大小 | AC1 cap + 落盘 ✓ | — | — |
| Assistant 输出 | AC2 fold ✓ | — | **nested 路径也 fold ✓ (AC1)** |
| Turn 边界 | AC4 audit + proactive fold ✓ | — | **nested 路径也 audit ✓ (AC1)** |
| 子 agent 入口 | 全量继承 | 3 mode + depth ✓ | — |
| **子 agent 多轮 audit** | — | — | **✓ (AC1)** |
| **子 agent proactive fold** | — | — | **✓ (AC1)** |
| **子 agent tool result cap** | — | — | **✓ (AC1)** |
| 4 路并行 deep review | — | — | **fixture + integration ✓ (AC2)** |
| D5 spans 22 步 | 51K (baseline) | P95=21707 ≤ 40K ✓ | **不退化 (AC3)** |

## Goal

**Sub-agent nested 路径 budget 控制全部生效**：

- 单测覆盖 nested 分支 4 个 no-op 反转（AC1）
- 4 路并行 deep review fixture PASS，0 LLM reject，prompt_tokens ≤ 40K（AC2）
- Phase B D5 spans 22 步复跑不退化，P95 ≤ 40K（AC3）
- t-registry 登记 ~8 个 T 点（AC4）

## Non-Goals

- AC11b (Anthropic `cache_control: ephemeral` 锚点) — 沿用 Phase B defer，等 Anthropic provider 接入
- AC3 (per-iter Prepare cadence) — 沿用 Phase A defer
- Sub-agent recursion depth 调整 — Phase B 已 MaxSubagentDepth=3
- 长上下文模型 (Claude Sonnet 4.6 1M context) 适配
- 跨 session 上下文共享
- LLM provider 选型 / 模型切换

## Success Criteria (Phase C)

- [ ] **C.1** AC1 + AC4 — TurnRequest/SubTurnRequest/SubQueryParams 透传 MaxContextTokens + nested 分支读取 + 单测覆盖 4 个 no-op 反转
- [ ] **C.2** AC2 — 4 路并行 deep review fixture + integration test PASS（prompt_tokens ≤ 40K, 0 LLM reject, audit.* span 出现, proactive_fold_triggered ≥ 1）
- [ ] **C.3** AC3 — Phase B D5 spans 22 步复跑 P95 ≤ 40K（不退化）
- [ ] **C.4** docs + t-registry + S6 归档
- [ ] 全量 `go test ./...` 绿
- [ ] 全量 `go vet ./...` 通过
- [ ] `tools/layer-lint` 通过
- [ ] integration test 覆盖 AC1 + AC2
- [ ] D7-S2-A06 t-registry 新增 ~6 T 点
- [ ] D2-S15-A08 t-registry 新增 ~2 T 点

## Scope

### In Scope

- `internal/layers/orchestration/turn/contracts.go` — `TurnRequest.MaxContextTokens` 字段
- `internal/layers/orchestration/turn/orchestrator.go` — nested 分支读 `req.MaxContextTokens`，fallback `o.maxContextTokens`
- `internal/shared/contracts/subturn.go` — `SubTurnRequest.MaxContextTokens` 字段
- `internal/layers/orchestration/turn/subturn.go` — `SubTurnRunner.Cfg.MaxContextTokens` 字段 + 注入 TurnRequest
- `internal/bootstrap/wire_coordinator.go` — `NewSubTurnRunner` 调用加 MaxContextTokens 注入
- `internal/layers/contextengine/enforce/subquery.go` — `SubQueryParams.MaxContextTokens` 字段 + Run 中透传
- `internal/layers/orchestration/turn/orchestrator_test.go` — `TestNestedBranch_BudgetBypass_Reversed`
- `internal/layers/orchestration/turn/subturn_test.go` — `TestSubTurnRunner_MaxContextTokens_Propagated`
- `tests/integration/d7/nested_budget_test.go` (新) — `TestIntegration_NestedBudget_4ParallelDeepReview`
- `tests/fixtures/nested-4parallel-deep-review.jsonl` (新)
- `docs/context-budget.md` — 新增 §"Nested branch budget injection (Phase C)"
- `openspec/specs/d7-orchestration/t-registry.md` — 新增 ~6 T 点
- `openspec/specs/d2-context-engine/t-registry.md` — 新增 ~2 T 点

### Out of Scope

- AC11b (Anthropic cache 锚点) — 单独 OpenSpec
- AC3 (per-iter Prepare) — Defer
- Sub-agent recursion depth 调整 — Phase B 已闭环
- 长上下文模型适配
- 跨 session 上下文共享

## 验证度量

- 单测：`go test -race ./internal/layers/orchestration/turn/...`
- Integration：`go test -tags integration -race ./tests/integration/d7/nested_budget_test.go`
- 回归：`go test -tags acceptance -race ./tests/acceptance/p0/d5_spans_replay_test.go`
- 全量：`go test -race ./...` + `go vet ./...` + `tools/layer-lint`
- 手工：发"深度 review devrix 项目"，期望 4 路 sub-task summary 卡片，audit.* span 非 0

## 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| SubQueryParams.MaxContextTokens 漏一个 caller | Med | C.4 docs 列全所有 caller；integration test 覆盖完整链路 |
| 4 路并行 fixture 写得太轻无法触发 audit | High | fixture 含 2 个 read_file (50K+) + 2 个 bash output (10K+) + assistant 长 summary |
| nested fallback `o.maxContextTokens` 与主 scope 不一致 | Low | SubTurnRunner 显式注入，deps fallback 仅 emitComplete 用 |
| AC3 D5 spans 复跑 P95 > 40K | Med | C.1 已修路径，C.3 验证不退化即可 |

## 关联

- Memory: `devrix-subagent-nested-budget-bypass.md`（根因分析，2026-06-20）
- Memory: `devrix-context-budget-phase-a-pr128.md`（Phase A）
- Memory: `devrix-context-budget-phase-b-s7-archived.md`（Phase B）
- 归档目录（待创建）：`openspec/archive/2026-06-20-devrix-context-budget-phase-c-nested/`