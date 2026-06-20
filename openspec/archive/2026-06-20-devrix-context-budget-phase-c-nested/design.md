# Design: Context Budget & Isolation — Phase C (Sub-Agent Nested 分支 Budget 治理)

**Change ID:** 2026-06-20-devrix-context-budget-phase-c-nested
**Demand ID:** DM-20260620-002
**Created:** 2026-06-20
**Status:** S3_Design

---

## 1. 目标

修复 sub-agent nested 路径下 Phase A 全部 budget 控制 bypass 的 bug,让 4 路并行 deep review 类场景正常完成。

## 2. 现状 (Phase A/B 已落地)

### 2.1 `runLoop` 双分支

```go
// internal/layers/orchestration/turn/orchestrator.go:221-326
func (o *DefaultOrchestrator) runLoop(ctx context.Context, req TurnRequest, out chan<- *contracts.EngineEvent) {
    nested := isNestedScope(req.Scope) || len(req.PreloadedMessages) > 0

    if nested {
        // [nested 分支] Phase C 修复目标
        systemPrompt = strings.TrimSpace(req.SystemPrompt)
        messages = append([]types.Message{}, req.PreloadedMessages...)
        messages = append(messages, req.UserMessage)
        if len(req.OverrideTools) > 0 {
            tools = req.OverrideTools
        }
        // ❌ 不调用 o.context.Prepare
        // ❌ 不设置 maxContextTokens (默认 0)
        model = req.Model
        // ❌ maxContextTokens = 0 (zero value)
        // ❌ maxContextTokens 不从 o.maxContextTokens fallback
    } else {
        // [主 scope 分支] Phase A 已落地, 保持不变
        prepared, err = o.context.Prepare(ctx, PrepareRequest{...})
        // ✓ maxContextTokens = prepared.MaxContextTokens
    }
}
```

### 2.2 `runTokenAudit` 守卫

```go
// orchestrator.go:894-904
func (o *DefaultOrchestrator) runTokenAudit(ctx, systemPrompt, messages, maxContextTokens, turnNum, turnSpan) {
    if o.toolResultStore == nil || o.maxAssistantCh <= 0 || maxContextTokens <= 0 {
        return  // ❌ nested 路径 maxContextTokens=0 触发 no-op
    }
    counter := token.NewCounter()
    res := audit.AuditMessages(counter, systemPrompt, messages, maxContextTokens)
    proactive := audit.ShouldFoldProactively(res, o.maxAssistantCh, audit.DefaultProactiveFoldPercent)
    // ... span attr + slog + apply fold
}
```

### 2.3 4 个 budget 控制点 (nested 全部 no-op)

| 控制点 | 实现位置 | nested 触发条件 | Phase C 修复 |
|--------|---------|----------------|--------------|
| `runTokenAudit` | orchestrator.go:383 | maxContextTokens > 0 | 显式注入 TurnRequest |
| `ShouldFoldProactively` | orchestrator.go:386 (调) | maxContextTokens > 0 | 同上 |
| ToolResultStore cap | orchestrator.go:478+ (tool_round) | 不依赖 budget, 但需要 audit 主动 fold | audit 触发后, tool_round 自动 cap |
| `budgetTracker.shouldStopDiminishing` | orchestrator.go:599 | maxContextTokens > 0 | 同上 |

## 3. 修复设计

### 3.1 TurnRequest 新增字段

```go
// internal/layers/orchestration/turn/contracts.go
type TurnRequest struct {
    // ... 既有字段 ...
    // MaxContextTokens DM-20260620-002 (AC1) — 显式注入 nested 路径
    // budget, 让 nested 分支的 runTokenAudit / ShouldFoldProactively /
    // budgetTracker 全部生效. 0 = fallback 到 o.maxContextTokens.
    // 主 scope 分支忽略此字段 (用 prepared.MaxContextTokens).
    MaxContextTokens int
}
```

### 3.2 nested 分支读取 + fallback

```go
// orchestrator.go:230 (改 1 行)
var (
    prepared         PreparedContext
    err              error
    systemPrompt     string
    messages         []types.Message
    tools            []ToolSchema
    model            string
    maxContextTokens int  // ← 既有
    persister        SessionPersister = o.persist
)

if nested {
    // ... 既有逻辑 ...
    // ↓ Phase C: 显式读取 + fallback
    maxContextTokens = req.MaxContextTokens
    if maxContextTokens <= 0 {
        maxContextTokens = o.maxContextTokens
    }
} else {
    // 主 scope 不变
    maxContextTokens = prepared.MaxContextTokens
}
```

### 3.3 SubTurnRequest + SubTurnRunner.Cfg 新增字段

```go
// internal/shared/contracts/subturn.go
type SubTurnRequest struct {
    // ... 既有字段 ...
    // MaxContextTokens DM-20260620-002 (AC1) — 透传到 TurnRequest.
    // 0 = SubTurnRunner 用 Cfg.MaxContextTokens.
    MaxContextTokens int
}

// internal/layers/orchestration/turn/subturn.go
type SubTurnConfig struct {
    DefaultMode      string
    LegacyMode       string
    MaxDepth         int
    // ↓ Phase C 新增
    MaxContextTokens int  // 0 = 无 budget 控制 (旧行为, Phase B)
}

// RunSubTurn 注入到 TurnRequest:
turnReq := TurnRequest{
    // ...
    MaxContextTokens: req.MaxContextTokens,  // ← 优先用 req
}
if turnReq.MaxContextTokens <= 0 {
    turnReq.MaxContextTokens = r.Cfg.MaxContextTokens  // ← fallback
}
```

### 3.4 SubQueryParams + enforce.Run 透传

```go
// internal/layers/contextengine/enforce/subquery.go
type SubQueryParams struct {
    // ... 既有字段 ...
    // ↓ Phase C 新增
    MaxContextTokens int  // 0 = SubTurnRunner fallback
}

// Run 中透传:
res, err := deps.SubTurn.RunSubTurn(ctx, contracts.SubTurnRequest{
    // ...
    MaxContextTokens: params.MaxContextTokens,  // ← 透传
})
```

### 3.5 bootstrap 注入

```go
// internal/bootstrap/wire_coordinator.go:79 (maxContextTokens 已有)
// ...
subTurn := turn.NewSubTurnRunner(orch, turn.SubTurnConfig{
    DefaultMode:      subagentCfg.DefaultMode,
    LegacyMode:       subagentCfg.LegacyMode,
    MaxDepth:         subagentCfg.MaxDepth,
    MaxContextTokens: maxContextTokens,  // ← Phase C 新增,沿用 line 78 变量
})
```

**所有 caller 链** (沿用 fallback, 0 = 用 bootstrap 注入):

| Caller | 调用位置 | 是否需要传 MaxContextTokens |
|--------|---------|---------------------------|
| `delegatetools/builtin_agents.go:RunExplore/Plan/Implement` | 内置 agent | 0 (fallback) |
| `bootstrap/wire_wave.go:114 (SubAgentDeps.Start → enforce.RunBackground → enforce.Run)` | Wave subagent | 0 (fallback) |
| `enforce/subquery.go:enforce.Run` | D2 fallback | 透传 (字段新增,旧 caller 传 0) |

### 3.6 4 路并行 fixture 设计

`tests/fixtures/nested-4parallel-deep-review.jsonl` 结构:

```jsonl
{"worker": 1, "step": 1, "type": "read_file", "path": "/Users/fukai/workspace/devrix/internal/layers/orchestration/turn/orchestrator.go", "output_chars": 60000}
{"worker": 1, "step": 2, "type": "bash", "command": "find . -name '*.go' | xargs wc -l", "output_chars": 12000}
{"worker": 1, "step": 3, "type": "read_file", "path": "/Users/fukai/workspace/devrix/internal/layers/orchestration/turn/subturn.go", "output_chars": 30000}
{"worker": 1, "step": 4, "type": "assistant_summary", "chars": 15000}
{"worker": 1, "step": 5, "type": "read_file", "path": "/Users/fukai/workspace/devrix/internal/bootstrap/wire_coordinator.go", "output_chars": 20000}
{"worker": 1, "step": 6, "type": "bash", "command": "git log --oneline -20", "output_chars": 5000}
... (4 路 × 10 步 = 40 事件)
```

**集成测试目标**:

1. 4 个 SubQuery 同时 spawn, 各走 10 步
2. 每个 SubQuery 内 prompt_tokens ~80K (无 Phase C 修复)
3. 修复后 prompt_tokens ~40K (audit + fold 后)
4. 验证 `audit.*` span 属性非 0
5. 验证 `proactive_fold_triggered=true` 至少 1 次
6. 验证 LLM 0 reject

### 3.7 t-registry 登记

**D7-S2-A06** (D7 orchestration turn loop) — 新增 6 T 点:

| T ID | 描述 | 优先级 |
|------|------|--------|
| D7-S2-A06-T18 | nested 分支读 `req.MaxContextTokens` + fallback `o.maxContextTokens` | P0 |
| D7-S2-A06-T19 | `runTokenAudit` nested 路径触发 (audit.total_tokens 非 0) | P0 |
| D7-S2-A06-T20 | `ShouldFoldProactively` nested 路径触发 (proactive_fold_triggered=true) | P0 |
| D7-S2-A06-T21 | `SubTurnRequest.MaxContextTokens` 字段透传到 `TurnRequest` | P0 |
| D7-S2-A06-T22 | `SubTurnRunner.Cfg.MaxContextTokens` 字段 fallback 链 | P0 |
| D7-S2-A06-T23 | 4 路并行 deep review integration test PASS | P0 |

**D2-S15-A08** (D2 context engine — audit + fold) — 新增 2 T 点:

| T ID | 描述 | 优先级 |
|------|------|--------|
| D2-S15-A08-T09 | `SubQueryParams.MaxContextTokens` 字段透传到 `SubTurnRequest` | P0 |
| D2-S15-A08-T10 | enforce.Run 透传链: `SubQueryParams.MaxContextTokens` → `SubTurnRequest.MaxContextTokens` → `TurnRequest.MaxContextTokens` | P0 |

## 4. 改动文件清单 (final)

### 4.1 生产代码 (5 文件)

| 文件 | 改动 | 行数 |
|------|------|------|
| `internal/layers/orchestration/turn/contracts.go` | `TurnRequest.MaxContextTokens` 字段 | +5 |
| `internal/layers/orchestration/turn/orchestrator.go` | nested 分支读 `req.MaxContextTokens` + fallback `o.maxContextTokens` | +4 |
| `internal/shared/contracts/subturn.go` | `SubTurnRequest.MaxContextTokens` 字段 | +5 |
| `internal/layers/orchestration/turn/subturn.go` | `SubTurnRunner.Cfg.MaxContextTokens` + 注入 TurnRequest | +10 |
| `internal/layers/contextengine/enforce/subquery.go` | `SubQueryParams.MaxContextTokens` + 透传 | +6 |
| `internal/bootstrap/wire_coordinator.go` | `NewSubTurnRunner` 加 `MaxContextTokens: maxContextTokens` | +1 |

**总计**: +31 行 (生产代码)

### 4.2 测试 (3 文件)

| 文件 | 改动 | 覆盖 AC |
|------|------|---------|
| `internal/layers/orchestration/turn/orchestrator_test.go` | `TestNestedBranch_BudgetBypass_Reversed` (新增 ~80 行) | AC1 |
| `internal/layers/orchestration/turn/subturn_test.go` | `TestSubTurnRunner_MaxContextTokens_Propagated` (新增 ~50 行) | AC1 |
| `tests/integration/d7/nested_budget_test.go` (新) | `TestIntegration_NestedBudget_4ParallelDeepReview` (~150 行) | AC2 |

### 4.3 文档 + t-registry (4 文件)

| 文件 | 改动 |
|------|------|
| `docs/context-budget.md` | 新增 §"Nested branch budget injection (Phase C)" |
| `openspec/specs/d7-orchestration/t-registry.md` | D7-S2-A06 T18-T23 |
| `openspec/specs/d2-context-engine/t-registry.md` | D2-S15-A08 T09-T10 |
| `openspec/t-registry.md` | 根索引加 8 T 点 |

## 5. 验证策略

### 5.1 单元测试

```bash
go test -race ./internal/layers/orchestration/turn/...
```

**`TestNestedBranch_BudgetBypass_Reversed`** 验证:
1. 构造 `TurnRequest{Scope: TurnScopeSubQuery, MaxContextTokens: 128000}` 不传 PreloadedMessages
2. mock `o.context.Prepare` 不被调用
3. mock `o.toolResultStore` + `o.maxAssistantCh` 已配置
4. mock LLM 返回超长 assistant message (15K chars)
5. mock span recorder 收集 span attr
6. 断言:
   - `audit.total_tokens > 0`
   - `audit.budget_percent > 0`
   - `proactive_fold_triggered == true`
   - largest assistant message 被 fold (chars 减少)

**`TestSubTurnRunner_MaxContextTokens_Propagated`** 验证:
1. 构造 `SubTurnRequest{MaxContextTokens: 128000}`
2. mock Orchestrator 接收 TurnRequest
3. 断言 `TurnRequest.MaxContextTokens == 128000`

### 5.2 集成测试

```bash
go test -tags integration -race ./tests/integration/d7/nested_budget_test.go
```

**`TestIntegration_NestedBudget_4ParallelDeepReview`** 验证:
1. spawn 4 个 SubQuery (`RunSubQuery` × 4)
2. 每个 SubQuery 走 fixture 中 10 步 tool call
3. LLM stub 按 fixture 返回 tool result / assistant summary
4. 收集所有 span attr
5. 断言:
   - 4 个 SubQuery 全部 0 LLM reject
   - 4 个 SubQuery 中至少 3 个有 `audit.total_tokens > 0` span
   - 4 个 SubQuery 中至少 2 个有 `proactive_fold_triggered=true` span
   - 单 turn prompt_tokens P95 ≤ 40K (vs Phase A baseline 80K+)

### 5.3 D5 spans 回归

```bash
go test -tags acceptance -race ./tests/acceptance/p0/d5_spans_replay_test.go
```

**`TestD5Spans_PhaseB_AC12_NoRegression`** 验证:
1. 沿用 Phase B D5 spans 22 步 fixture
2. 跑一遍
3. 断言 prompt_tokens P95 ≤ 40K (Phase B baseline)
4. 断言 feishu 0 ERROR

### 5.4 全量

```bash
go test -race ./...
go vet ./...
tools/layer-lint
```

### 5.5 手工验证

```bash
devrix.sh build && devrix.sh restart
# 飞书发"深度 review devrix 项目"
# 期望: 4 路 sub-task summary 卡片, audit.* span 非 0
```

## 6. 风险与回滚

### 6.1 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| nested fallback `o.maxContextTokens` 不一致 | Low | Low | SubTurnRunner 显式注入, deps fallback 仅 emitComplete |
| SubQueryParams 漏 caller | Med | Med | C.4 docs 列全 caller; integration test 覆盖 |
| 4 路 fixture 写轻 | Med | High | fixture 含 2 read_file (50K+) + 2 bash (10K+) |
| D5 spans 退化 | Low | Med | C.3 验证不退化 |

### 6.2 回滚

- C.1 改动最小,回滚 `git revert <C.1 commit>` 即恢复 nested bypass
- C.2/C.3 仅加测试 + fixture,回滚删文件
- C.4 仅 docs + t-registry,回滚删 commit

## 7. 关联

- Phase A 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation/`
- Phase B 归档: `openspec/archive/2026-06-20-devrix-context-budget-and-isolation-phase-b/`
- Phase B AC12 fixture: `tests/fixtures/d5-spans-replay.jsonl`
- 根因 memory: `~/.claude/projects/-Users-fukai-workspace/memory/devrix-subagent-nested-budget-bypass.md`

## 8. 设计签收

| 维度 | 结论 |
|------|------|
| 与 Phase A/B 兼容性 | ✓ 兼容 (新增字段, fallback chain) |
| nested 路径性能 | ✓ 不变 (仍不调 Prepare) |
| 主 scope 路径 | ✓ 不变 (仍用 prepared.MaxContextTokens) |
| clawcode 对齐 | ✓ devrix nested + 显式注入 ≈ clawcode Prepare-per-layer |
| 测试覆盖 | ✓ AC1 (单测) + AC2 (集成) + AC3 (回归) |