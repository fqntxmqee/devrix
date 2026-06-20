# D7 Orchestration Spec — Sub-Agent Nested Budget Injection (Phase C)

**Spec ID:** D7-S2-A06-PhaseC
**Change ID:** 2026-06-20-devrix-context-budget-phase-c-nested
**Demand ID:** DM-20260620-002
**Status:** S3_Design
**Parent:** DM-20260620-001 (Phase A) + DM-20260620-001-B (Phase B)

---

## Feature: Sub-Agent Nested Branch Budget Injection

### Background

Phase A (DM-20260620-001) 落地了同 turn 体积治理 (tool result cap, assistant fold, per-iter audit, feishu precheck). Phase B (DM-20260620-001-B) 落地了子 agent 入口隔离 (3-mode dispatch, depth cap).

但 sub-agent **多轮工具调用累积**这条路径上, Phase A 全部 budget 控制因 `maxContextTokens = 0` 而 no-op, 导致 4 路并行 deep review 类场景 LLM reject (>100K tokens).

### Scenario: 4 路并行 deep review

```gherkin
Given 用户发"深度 review devrix 项目"
And D7 TaskDecomposer 拆 4 个并行子任务
And 每个子任务 spawn 1 个 SubAgent (SubQuery)
And 每个 SubAgent 走 10 步 tool call (read_file 50K+, bash 10K+, assistant summary)
When 4 路 SubQuery 并发执行
Then 每个 SubAgent prompt_tokens ≤ 40K (无 Phase C 修复时 ~80K+)
And 4 个 SubAgent 全部 0 LLM reject (无 Phase C 修复时全部失败)
And audit.* span 属性非 0 (无 Phase C 修复时 0)
And proactive_fold_triggered ≥ 1 (无 Phase C 修复时 0)
```

### Scenario: Single SubAgent nested branch

```gherkin
Given SubTurnRunner.RunSubTurn 收到 SubTurnRequest{MaxContextTokens: 128000}
When SubTurnRunner 调用 Orchestrator.RunTurn 注入 TurnRequest
Then TurnRequest.MaxContextTokens == 128000
And nested 分支读取 req.MaxContextTokens = 128000
And fallback o.maxContextTokens 不被使用
And runTokenAudit 触发 (audit.total_tokens > 0)
And ShouldFoldProactively 触发 (largest message 被 fold)
And budgetTracker.shouldStopDiminishing 在 budget >= 90% 时停止
```

### Scenario: Backward compatibility (MaxContextTokens=0)

```gherkin
Given 旧 caller 不传 MaxContextTokens (传 0)
When SubTurnRunner.RunSubTurn 处理
Then SubTurnRequest.MaxContextTokens == 0
And SubTurnRunner 注入 TurnRequest.MaxContextTokens = Cfg.MaxContextTokens
And Cfg.MaxContextTokens 来自 bootstrap.NewSubTurnRunner(orch, cfg.MaxContextTokens)
And bootstrap 沿用 wire_coordinator.go:78 变量 (默认 128000, 文件可覆盖)
```

### Scenario: 主 scope 分支不受影响

```gherkin
Given TurnRequest{Scope: TurnScopeMain} (主 scope)
When runLoop 处理
Then nested == false
And prepared, err = o.context.Prepare(...) 调用
And maxContextTokens = prepared.MaxContextTokens (来自 Prepare)
And req.MaxContextTokens 字段被忽略
```

---

## Acceptance Criteria

### AC1 (P0): nested 分支 maxContextTokens 注入路径打通

| T ID | 描述 | 度量 |
|------|------|------|
| D7-S2-A06-T18 | nested 分支读 `req.MaxContextTokens` + fallback `o.maxContextTokens` | 单测覆盖, o.maxContextTokens fallback chain |
| D7-S2-A06-T19 | `runTokenAudit` nested 路径触发 (audit.total_tokens 非 0) | span attr 断言 |
| D7-S2-A06-T20 | `ShouldFoldProactively` nested 路径触发 (proactive_fold_triggered=true) | largest message 被 fold |
| D7-S2-A06-T21 | `SubTurnRequest.MaxContextTokens` 字段透传到 `TurnRequest` | 单测覆盖 3-mode × MaxContextTokens |
| D7-S2-A06-T22 | `SubTurnRunner.Cfg.MaxContextTokens` fallback 链 (req → Cfg) | 单测覆盖 0 / 128000 / 0 fallback 3 case |

### AC2 (P0): 4 路并行 deep review fixture 闭环

| T ID | 描述 | 度量 |
|------|------|------|
| D7-S2-A06-T23 | 4 路并行 deep review integration test PASS (prompt_tokens ≤ 40K, 0 LLM reject, audit.* span 出现, proactive_fold_triggered ≥ 1) | `tests/fixtures/nested-4parallel-deep-review.jsonl` + `tests/integration/d7/nested_budget_test.go` |

### AC3 (P1): Phase B AC12 不退化

- D5 spans 22 步 fixture 复跑 P95 ≤ 40K (Phase B baseline)
- 0 feishu ERROR

### AC4 (P0): t-registry 登记

- D7-S2-A06 T18-T23 (6 T 点)
- D2-S15-A08 T09-T10 (2 T 点)
- 根索引 `openspec/t-registry.md` 同步

---

## Out of Scope

- AC11b (Anthropic cache 锚点) — 沿用 Phase B defer
- AC3 (per-iter Prepare) — 沿用 Phase A defer
- Sub-agent recursion depth 调整 — Phase B 已闭环
- 长上下文模型适配
- 跨 session 上下文共享

---

## Reference

- 根因分析: `~/.claude/projects/-Users-fukai-workspace/memory/devrix-subagent-nested-budget-bypass.md`
- 根因代码: `internal/layers/orchestration/turn/orchestrator.go:221-268` (nested 分支)
- Phase A 实现: `internal/layers/contextengine/prepare/{audit,persist}/`
- Phase B 实现: `internal/layers/orchestration/turn/subturn.go` (3-mode dispatch)
- maxContextTokens flow: `internal/bootstrap/wire_coordinator.go:78-164`