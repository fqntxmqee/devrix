# Proposal: Context Budget & Isolation — Phase C (Sub-Agent Nested 分支 Budget 治理)

**Change ID:** 2026-06-20-devrix-context-budget-phase-c-nested
**Demand ID:** DM-20260620-002
**Created:** 2026-06-20
**Status:** S7_Archived (S1_Demand → S2_Proposal → S3_Design → S4_Implementation → S5_Acceptance → S6_Archive)
**Parent:** DM-20260620-001 (Phase A) + DM-20260620-001-B (Phase B)

---

## 澄清记录 (S2)

### Q1: maxContextTokens 是 dep-level fallback 还是显式注入?

**A**: **两者并存** (推荐方案 2):
- **显式注入**: SubTurnRunner.RunSubTurn 在调用 Orchestrator.RunTurn 前把 MaxContextTokens 写入 TurnRequest (Phase C 改动)
- **dep-level fallback**: OrchestratorDeps.MaxContextTokens 保留,emitComplete 时 (orchestrator.go:693-694) 作 fallback 用 — Phase A 已落地,不动

**理由**:
- nested 分支直接读 `req.MaxContextTokens`,不再调 Prepare (避免性能开销)
- 主 scope 分支不调 req.MaxContextTokens (用 prepared.MaxContextTokens),保留 Phase A 路径
- 两者互不干扰

### Q2: SubQueryParams 是否加 MaxContextTokens 字段?

**A**: **加**。链路 `enforce.Run → SubTurnRunner.RunSubTurn`,如果不加则 SubTurnRunner 只能从 deps (SubTurnRunner.Cfg.MaxContextTokens) 拿,丢失 "每次 sub-query 可独立配置" 的灵活性。

caller 链:
- `delegatetools/builtin_agents.go:42-89` (RunExplore/RunPlan/RunImplement)
- `enforce/subquery.go:60-139` (enforce.Run)
- `bootstrap/wire_wave.go:114-126` (SubAgentDeps.Start → enforce.RunBackground → enforce.Run)

**所有 caller 都传 0 即可** (default), bootstrap 在 SubTurnRunner 注入时填上; 或显式 override (后续 D5 等场景需要时可调)。

### Q3: 4 路并行 fixture 怎么写才能触发 audit?

**A**: 模拟真实 deep review 场景:
- 4 个 SubQuery 同时 spawn
- 每个 SubQuery 走 10 步 tool call
- 其中 2 步是大 read_file (50K+ 字符)
- 2 步是 bash output (10K+ 字符)
- 2 步是 assistant 长 summary
- 累计 prompt_tokens ~80K (无 budget 控制) → ~40K (有 audit + fold)

**fixture 文件**: `tests/fixtures/nested-4parallel-deep-review.jsonl`,每行一个 tool call 模拟事件。

### Q4: AC3 D5 spans 22 步复跑是否独立 PR?

**A**: **沿用 Phase B AC12 fixture** (`tests/fixtures/d5-spans-replay.jsonl`),不新建独立 PR。在 C.3 (单独 PR 或 C.2 同 PR) 跑一遍 acceptance test 验证 P95 ≤ 40K 不退化。

### Q5: 4 路并行的集成测试是 D2 还是 D7?

**A**: **D7** (`tests/integration/d7/`),因为走 SubTurnRunner → Orchestrator (D7 turn layer) → LLM stub 链路。D2 仅验证 SubQueryParams 透传,不重复测端到端。

### Q6: nested 分支 tool result cap 怎么生效?

**A**: **现状已生效**。`buildToolResultMsgWithCap` (Phase A AC1 落地) 在 runLoop 的 tool_round 阶段调用 (orchestrator.go:478+ 附近),不看 scope,只看 messages 长度。
- 但 **Phase A AC1 的 cap 触发依赖 messages 累计**,没有预算跟踪
- Phase C 修复 audit 后,audit 会主动 fold largest assistant message + 触发 proactive fold,间接让 tool result 不需要 cap (因为 fold 后 messages 体积下降)

**验证方式**: integration test 中验证 audit span attr `audit.largest_msg_tokens` 不超过阈值。

### Q7: docs 放在 `docs/context-budget.md` 还是新文件?

**A**: **追加到 `docs/context-budget.md`**,新增 §"Nested branch budget injection (Phase C)" 一节。理由:
- Phase A/B 已有该文件
- Phase C 是同一系列延续
- 不分散阅读路径

---

## Problem Statement (Phase C 增量)

Phase A 治理"同 turn 体积",Phase B 治理"子 agent 入口隔离",但 **sub-agent 多轮工具调用累积**这条路径上 budget 控制全部 bypass。

**根因** (`internal/layers/orchestration/turn/orchestrator.go:221-268`):

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

`maxContextTokens = 0` 触发 4 个 no-op:
1. `runTokenAudit` (line 894-904) → 不 audit
2. `ShouldFoldProactively` → 不 fold
3. ToolResultStore cap → 不 cap (实际是 tool result 在 tool_round 阶段 cap,不依赖 budget,但缺 audit 后无主动 fold)
4. `budgetTracker.shouldStopDiminishing` → 永不停止

4 路并行 deep review 失败链路:
```
Leader (D7 IntentOrchestrate)
  → TaskDecomposer.buildNodes (4 路真并行需 LLM Decomp)
  → WaveScheduler.Start (SubAgent pool=3, 第 4 个 queue)
  → ContextResolver.Resolve (ContextFresh) → 1 条 user(directive)
  → SubQuery.Run → SubTurnRunner.RunSubTurn
  → Orchestrator.RunTurn (nested branch) ← maxContextTokens=0
  → runLoop 多轮工具调用 → messages 累积 → LLM reject
```

## Proposed Solution

按 **C.1 → C.4** 四阶段推进,每阶段独立 PR:

| AC | 描述 | 度量 | PR |
|----|------|------|----|
| **AC1** | `TurnRequest` + `SubTurnRequest` + `SubQueryParams` 各加 `MaxContextTokens int` 字段; nested 分支读 `req.MaxContextTokens`, fallback 到 `o.maxContextTokens`; `SubTurnRunner.Cfg.MaxContextTokens` 注入; bootstrap `NewSubTurnRunner` 注入 `maxContextTokens` (沿用 wire_coordinator.go:78 的变量) | 单测覆盖 nested 分支 4 个 no-op 反转; span attr `audit.total_tokens`/`budget_percent`/`proactive_fold_triggered` 不再 0 | C.1 |
| **AC2** | 4 路并行 deep review fixture + integration test PASS | spawn 4 个 SubQuery 同时跑, 10 步 tool call, prompt_tokens ≤ 40K, 0 LLM reject, audit.* span 出现, proactive_fold_triggered ≥ 1 | C.2 |
| **AC3** | Phase B AC12 不退化: D5 spans 22 步复跑 P95 ≤ 40K | 沿用 Phase B fixture | C.3 |
| **AC4** | t-registry 登记 ~8 T 点 (D7-S2-A06 T18-T23 + D2-S15-A08 T09-T10) + docs §Nested branch budget injection | docs + t-registry | C.4 |

### Sub-PR 拆分

| PR | 范围 | 风险 | 依赖 |
|----|------|------|------|
| **C.1** | AC1 + AC4 — TurnRequest/SubTurnRequest/SubQueryParams 透传 MaxContextTokens + nested 分支读取 + 单测覆盖 | Low (新增字段,向后兼容) | — |
| **C.2** | AC2 — 4 路并行 deep review fixture + integration test | Med (fixture 写实才能触发 audit) | C.1 |
| **C.3** | AC3 — Phase B AC12 回归 (同 C.2 PR 或独立) | Low | C.1 |
| **C.4** | docs + t-registry + S6 归档 | Low | C.1-C.3 |

### 跨 AC

| AC | 描述 |
|----|------|
| **AC12** | 所有 sub-PR 合入后, 回归 D5 spans 设计任务原 prompt 走一遍: 22 步后 prompt_tokens P95 ≤ 40K (Phase B baseline), feishu 0 ERROR |
| **AC11b (DEFERRED)** | 沿用 Phase B defer, 等 Anthropic provider 接入 |
| **AC3 (DEFERRED)** | 沿用 Phase A defer |

## Backward Compatibility

- `TurnRequest.MaxContextTokens` 新增字段,空值时 nested 分支 fallback 到 `o.maxContextTokens` (Phase A 已注入,默认 128000)
- `SubTurnRequest.MaxContextTokens` 新增字段,空值时 SubTurnRunner 用 `Cfg.MaxContextTokens` (Phase C 注入)
- `SubQueryParams.MaxContextTokens` 新增字段,空值时透传 0,SubTurnRunner fallback 到 Cfg
- 所有现有 caller 不传新字段即可走 fallback,无 breaking change

## Scope

### In Scope

- `internal/layers/orchestration/turn/contracts.go` — `TurnRequest.MaxContextTokens` 字段
- `internal/layers/orchestration/turn/orchestrator.go` — nested 分支读 `req.MaxContextTokens`,fallback `o.maxContextTokens` (line 230 改 1 行)
- `internal/shared/contracts/subturn.go` — `SubTurnRequest.MaxContextTokens` 字段
- `internal/layers/orchestration/turn/subturn.go` — `SubTurnRunner.Cfg.MaxContextTokens` 字段 + 注入 TurnRequest
- `internal/bootstrap/wire_coordinator.go` — `NewSubTurnRunner` 调用加 `MaxContextTokens: maxContextTokens`
- `internal/layers/contextengine/enforce/subquery.go` — `SubQueryParams.MaxContextTokens` 字段 + Run 中透传
- `internal/layers/orchestration/turn/orchestrator_test.go` — `TestNestedBranch_BudgetBypass_Reversed`
- `internal/layers/orchestration/turn/subturn_test.go` — `TestSubTurnRunner_MaxContextTokens_Propagated`
- `tests/integration/d7/nested_budget_test.go` (新) — `TestIntegration_NestedBudget_4ParallelDeepReview`
- `tests/fixtures/nested-4parallel-deep-review.jsonl` (新)
- `docs/context-budget.md` — §"Nested branch budget injection (Phase C)"
- `openspec/specs/d7-orchestration/t-registry.md` — D7-S2-A06 T18-T23 (6 T 点)
- `openspec/specs/d2-context-engine/t-registry.md` — D2-S15-A08 T09-T10 (2 T 点)

### Out of Scope

- AC11b (Anthropic cache 锚点) — 单独 OpenSpec
- AC3 (per-iter Prepare) — Defer
- Sub-agent recursion depth 调整 — Phase B 已 MaxSubagentDepth=3
- 长上下文模型适配
- 跨 session 上下文共享
- LLM provider 选型 / 模型切换
- RunRegistry disk output 治理 (DM-011)

## Impact Analysis

| Component | Change Required | Phase | Details |
|-----------|-----------------|-------|---------|
| `internal/layers/orchestration/turn/contracts.go` | Yes | C.1 | `TurnRequest.MaxContextTokens` 新字段 |
| `internal/layers/orchestration/turn/orchestrator.go` | Yes | C.1 | nested 分支 line 230 读 `req.MaxContextTokens` + fallback `o.maxContextTokens` |
| `internal/shared/contracts/subturn.go` | Yes | C.1 | `SubTurnRequest.MaxContextTokens` 新字段 |
| `internal/layers/orchestration/turn/subturn.go` | Yes | C.1 | `SubTurnRunner.Cfg.MaxContextTokens` + 注入 TurnRequest |
| `internal/bootstrap/wire_coordinator.go` | Yes | C.1 | `NewSubTurnRunner` 调用加 MaxContextTokens |
| `internal/layers/contextengine/enforce/subquery.go` | Yes | C.1 | `SubQueryParams.MaxContextTokens` 字段 + 透传 |
| `internal/layers/orchestration/turn/orchestrator_test.go` | Yes | C.1 | `TestNestedBranch_BudgetBypass_Reversed` |
| `internal/layers/orchestration/turn/subturn_test.go` | Yes | C.1 | `TestSubTurnRunner_MaxContextTokens_Propagated` |
| `tests/integration/d7/nested_budget_test.go` | Yes (新) | C.2 | `TestIntegration_NestedBudget_4ParallelDeepReview` |
| `tests/fixtures/nested-4parallel-deep-review.jsonl` | Yes (新) | C.2 | 4-路并行 fixture |
| `docs/context-budget.md` | Yes | C.4 | 新增 §"Nested branch budget injection (Phase C)" |
| `openspec/specs/d7-orchestration/t-registry.md` | Yes | C.4 | D7-S2-A06 T18-T23 |
| `openspec/specs/d2-context-engine/t-registry.md` | Yes | C.4 | D2-S15-A08 T09-T10 |
| `LayerViolationProbe` | No | - | 接口契约扩展 (新增字段) 非破坏性 |
| Phase A/B 已落地代码 | No | - | `runTokenAudit`/`ShouldFoldProactively`/`ToolResultStore`/`budgetTracker` 已就位 |

## Architecture Considerations

### 1. nested 分支不调 Prepare 的设计权衡

Phase A 设计 `runLoop` 时, nested 分支不调 Prepare 是**有意为之**:
- 性能: 每次 turn 跑 Prepare 太贵 (Phase A AC3 已 defer per-iter Prepare)
- 语义: nested 路径的消息由调用方 (SubTurnRunner) 显式构造,Prepare 不应再次重写

但 **Phase A 漏了 budget 透传**。修复方案是显式把 budget 作为 TurnRequest 字段,而不是让 nested 分支调 Prepare 拿 (会破坏 nested 路径性能/语义)。

### 2. fallback 链: req → deps → zero

```go
// nested 分支 line 230 改:
maxContextTokens := req.MaxContextTokens
if maxContextTokens <= 0 {
    maxContextTokens = o.maxContextTokens  // deps-level fallback (Phase A 已注入)
}
// 主 scope 不变 (line 265):
maxContextTokens = prepared.MaxContextTokens  // 来自 Prepare
```

**3 层优先级**:
1. `req.MaxContextTokens` (Phase C 新增, SubTurnRunner 显式注入)
2. `o.maxContextTokens` (OrchestratorDeps, Phase A 已落地, 默认 128000)
3. `0` (兜底, 触发 runTokenAudit no-op, 保持旧行为兼容)

### 3. 与 Phase A/B 的关系

- Phase A 落地 `runTokenAudit` / `ShouldFoldProactively` / `ToolResultStore` / `budgetTracker` —— Phase C 修复 nested 路径触发条件
- Phase B 落地 3-mode dispatch / depth cap —— Phase C 修复 nested 路径 budget 治理
- 三者**互补**, 无功能重叠

### 4. 与 clawcode 的对齐

clawcode `src/query.ts:365-468` 5 层 pipeline 中,**每层 turn loop 都独立调用 Prepare**,因此 clawcode 不存在 nested 分支 budget bypass 问题 (因为 nested 路径不存在,所有路径都走 Prepare)。

devrix 选 nested 分支**不调 Prepare** 是性能/语义优化,Phase C 用 explicit 注入修复 budget 治理,**保留 nested 路径优势**。

## Success Criteria

| Milestone | AC 范围 | 核心交付 | 验证 |
|-----------|---------|---------|------|
| **C.1** | AC1 + AC4 | 透传 MaxContextTokens + nested 分支读取 + 单测覆盖 4 no-op 反转 | unit test -race PASS |
| **C.2** | AC2 | 4-路并行 deep review fixture + integration test | integration -race PASS |
| **C.3** | AC3 | Phase B D5 spans 22 步复跑不退化 | acceptance test PASS |
| **C.4** | docs + t-registry + S6 归档 | docs + t-registry | docs sync + verify-archive.sh |

- [ ] 全量 `go test ./...` 每版绿
- [ ] 全量 `go vet ./...` 通过
- [ ] `tools/layer-lint` 通过
- [ ] integration test 覆盖 AC1 + AC2
- [ ] 4 路并行 fixture prompt_tokens ≤ 40K, 0 LLM reject
- [ ] D5 spans 22 步复跑 P95 ≤ 40K (Phase B baseline 不退化)
- [ ] feishu 0 ERROR 命中"4 路并行 deep review 场景"

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| nested 分支 fallback `o.maxContextTokens` 与主 scope 不一致 | Low | Low | SubTurnRunner 显式注入, deps fallback 仅 emitComplete 用 |
| SubQueryParams.MaxContextTokens 漏一个 caller | Med | Med | C.4 docs 列全所有 caller; integration test 覆盖完整链路 |
| 4 路并行 fixture 写得太轻无法触发 audit | Med | High | fixture 含 2 个 read_file (50K+) + 2 个 bash output (10K+) + assistant 长 summary |
| AC3 D5 spans 复跑 P95 > 40K | Low | Med | C.1 已修路径, C.3 验证不退化即可 |
| nested 分支 tool result cap 路径未验证 | Low | Low | Phase A AC1 测试已覆盖 tool_round, Phase C 仅补 audit/fold 触发, 不重复 |
| LLM stub 4 路并发不稳定 | Low | Med | integration test 用 sync.WaitGroup 串行收集 span 属性, 不依赖 LLM 真实并发 |

## Implementation Order

1. **C.1**: AC1 + AC4 — TurnRequest/SubTurnRequest/SubQueryParams 透传 MaxContextTokens + nested 分支读取 + 单测覆盖, 1 PR, 1 squash auto-merge
2. **C.2**: AC2 — 4 路并行 deep review fixture + integration test, 1 PR, 1 squash auto-merge
3. **C.3**: AC3 — D5 spans 22 步复跑回归 (同 C.2 PR 或独立 1 PR), 1 squash auto-merge
4. **C.4**: docs + t-registry + S6 归档, 1 PR, 1 squash auto-merge

## Reference

- 根因分析 memory: `~/.claude/projects/-Users-fukai-workspace/memory/devrix-subagent-nested-budget-bypass.md`
- Phase A 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/`
- Phase B 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation-phase-b/`
- Phase B AC12 fixture: `tests/fixtures/d5-spans-replay.jsonl`
- Phase A 实现 (audit/fold/tool result cap): `internal/layers/contextengine/prepare/{audit,persist}/`
- Phase B 实现 (3-mode dispatch): `internal/layers/orchestration/turn/subturn.go`
- maxContextTokens flow: `internal/bootstrap/wire_coordinator.go:78-164`