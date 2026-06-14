# Design: D7 Turn 编排上移

**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020  
**Status:** S3_Design（草稿）  
**Depends On:** proposal.md R1 决议

---

## 1. 设计决策（Decision Log）

| # | 决策 | 理由 |
|---|------|------|
| D1 | Turn SoT 归 D7-S2-A06 | Leader 拥有循环；与 D4 Follower 对称 |
| D2 | D2-S16 保留 Legacy 冻结 | 38+ T 注释不改；Archive 映射 |
| D3 | 三接口替代 `QueryLoopExecutor` | 可测试、可观测、边界清晰 |
| D4 | `ILLMGateway` 注释/消费方改 D7 | Bridge 路径不变（`bridges/llm/`） |
| D5 | SubQuery = D7 嵌套 `RunTurn` | OQ3；与 Hub-Spoke 包装一致 |
| D6 | Autocompact = D2 信号 + D7 调 D3 | 杜绝 D2 直连 LLM |
| D7 | RouteModel 在 D7 InvokeLLM 前 | OQ4；复用 D3-S1 |

---

## 2. 目标接口契约（v1.0 登记，v2.0 实现）

### 2.1 D7 — `coordinator/turn.go`（v2.0 路径）

```go
// TurnOrchestrator owns the LLM↔Tool turn loop (D7-S2-A06).
type TurnOrchestrator interface {
    RunTurn(ctx context.Context, req TurnRequest) (<-chan *contracts.EngineEvent, error)
}

type TurnRequest struct {
    SessionID    string
    UserMessage  types.Message
    MaxTurns     int
    Scope        TurnScope // main | subquery | compress
}

type TurnScope string
const (
    TurnScopeMain     TurnScope = "main"
    TurnScopeSubQuery TurnScope = "subquery"
    TurnScopeCompress TurnScope = "compress"
)

// LLMInvoker performs one D3 streaming call (D7-S2-A07).
type LLMInvoker interface {
    InvokeStream(ctx context.Context, req LLMInvokeRequest) (<-chan llmgateway.Chunk, error)
}

type LLMInvokeRequest struct {
    SessionID    string
    Tier         string          // resolved via D3-S1 before call
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSpec
}
```

### 2.2 D2 — 拆面契约（替代黑盒 QueryLoopExecutor）

```go
// ContextPreparer assembles legal context for one iteration (D2-S15).
type ContextPreparer interface {
    Prepare(ctx context.Context, req PrepareRequest) (PreparedContext, error)
}

type PreparedContext struct {
    SystemPrompt string
    Messages     []types.Message
    Tools        []ToolSchema
    CompressHint *CompressRequest // non-nil → D7 must call D3 summary
}

type CompressRequest struct {
    MessagesToSummarize []types.Message
    TargetTokenBudget   int
}

// ToolRoundExecutor runs policy-gated tool batch (D2-S18).
type ToolRoundExecutor interface {
    ExecuteRound(ctx context.Context, req ToolRoundRequest) (ToolRoundResult, error)
}

// SessionPersister commits turn outcome (D2-S17).
type SessionPersister interface {
    PersistTurn(ctx context.Context, req PersistRequest) error
}
```

### 2.3 Legacy 兼容（v2.0-f，1 发布周期）

```go
// QueryLoopExecutor — DEPRECATED: delegates to TurnOrchestrator internally.
type QueryLoopExecutor interface {
    RunQueryLoop(ctx context.Context, req QueryRequest) (<-chan *contracts.EngineEvent, error)
}
```

### 2.4 依赖规则（目标态）

```text
✅ D7 → D2（ContextPreparer / ToolRoundExecutor / SessionPersister）
✅ D7 → D3（ILLMGateway via bridges/llm）
✅ D7 → D4 / D1 / D5 / D6（已有）
❌ D2 → D3（禁止：llmgateway, bridges/llm）
❌ D2 → D7（已有 lint）
```

---

## 3. Turn 状态机（D7-S2-A06）

```text
                    ┌─────────────┐
                    │   START     │
                    └──────┬──────┘
                           ▼
                    ┌─────────────┐
              ┌────│  PREPARE    │◀── CompressHint?
              │    │  (D2-S15)   │    └── D7→D3 summary → merge → retry
              │    └──────┬──────┘
              │           ▼
              │    ┌─────────────┐
              │    │ ROUTE+LLM   │
              │    │ D3-S1+S2    │
              │    └──────┬──────┘
              │           ▼
              │     tool_calls?
              │      ┌────┴────┐
              │     no        yes
              │      │         ▼
              │      │  ┌─────────────┐
              │      │  │ TOOL_ROUND  │
              │      │  │ (D2-S18)    │
              │      │  └──────┬──────┘
              │      │         │
              │      │    turns < max?
              │      │         └── yes ──┘
              │      ▼
              │    ┌─────────────┐
              └───▶│  PERSIST    │
                   │  (D2-S17)   │
                   └──────┬──────┘
                          ▼
                    ┌─────────────┐
                    │  COMPLETE   │
                    └─────────────┘
```

---

## 4. Gherkin Scenarios

### 4.1 D7-S2-A06 RunTurnLoop

```gherkin
Feature: D7 Turn Leader orchestrates prepare-llm-tools-persist

  Scenario: FastPath turn calls D2 then D3 in order
    Given a session with query_loop enabled
    And D7 FastPath receives a user message
    When D7 RunTurnLoop executes
    Then D2 PrepareExecutionContext is invoked before any LLM stream
    And D7 InvokeLLM calls D3 ChatStream with PreparedContext
    And D2 does not import llmgateway package

  Scenario: Multi-turn tool_use loops under D7
    Given PreparedContext with tools registered
    And D3 returns tool_use in stream
    When D7 RunTurnLoop continues
    Then D2 ExecuteToolRound runs with permission gate
    And D7 InvokeLLM is called again with updated messages
    Until D3 returns final text without tool_calls

  Scenario: Cancel propagates to D3 stream and D2 tools
    Given an in-flight RunTurnLoop
    When the user sends interrupt
    Then D7 cancels Turn context
    And D3 stream closes without leak
    And D2 tool execution aborts

  Scenario: SubQuery nested turn uses same orchestrator
    Given a SubQuery explore request
    When D7 wraps D2-S19 nested execution
    Then nested RunTurn uses TurnScopeSubQuery
    And LLM is invoked by D7 not D2
```

### 4.2 D7-S2-A07 InvokeLLM

```gherkin
Feature: D7 invokes D3 with route and resilience

  Scenario: RouteModel before StreamChat
    Given session model tier "fast"
    When D7 InvokeLLM runs
    Then D3-S1 RouteModel resolves tier to concrete model
    And D3-S2 StreamChat receives resolved model

  Scenario: Breaker open returns recoverable error to FastPath
    Given D3 circuit breaker is open for provider
    When D7 InvokeLLM runs
    Then FastPath emits recoverable EngineEvent error
    And D7 does not call D2 ExecuteToolRound

  # T: D7-S2-A07-T01 (P0 — 双边共识 G-12)
  Scenario: Breaker open with no fallback provider returns error
    Given D3 circuit breaker is open for ALL providers
    When D7 InvokeLLM runs
    Then an EngineEvent with kind "error" and recoverable=true is emitted
    And the error message indicates "no LLM provider available"
    And D7 does NOT panic or crash
    And D2 PersistTurn is called to save partial state

  # T: D7-S2-A07-T02 (P0)
  Scenario: StreamChat timeout propagates as EngineEvent
    Given D3 StreamChat exceeds timeout
    When D7 InvokeLLM deadline is reached
    Then the stream is closed without goroutine leak
    And a timeout EngineEvent is emitted
```

### 4.3 D2-S15 / S18 / S17（修订边界）

```gherkin
Feature: D2 Context Follower without LLM

  Scenario: Prepare only does not call LLM
    Given messages exceed autocompact threshold
    When D2 Prepare returns CompressHint
    Then D2 does not invoke any LLM adapter
    And D7 is responsible for summary via D3

  Scenario: ExecuteToolRound enforces permission
    Given a CRITICAL risk tool call
    When D2 ExecuteToolRound runs
    Then permission gate blocks before toolrunner execute
```

---

## 5. A / F 注册表增量（v1.0 写入 specs 草案）

### 5.1 D7 新增

| A ID | Name | F 要点 | Code（v1.0） | Code（v2.0） |
|------|------|--------|-------------|-------------|
| D7-S2-A06 | RunTurnLoop | F01 OrchestrateTurn, F02 LoopUntilFinal | `fastpath.go` 黑盒 | `orchestration/turn/orchestrator.go` |
| D7-S2-A07 | InvokeLLM | F01 RouteAndStream, F02 MapChunksToEvents | — | `orchestration/turn/llm.go` |

### 5.2 D2 修订

| A ID | 变更 |
|------|------|
| D2-S16-A01 RunQueryLoop | **Legacy 冻结** → maps D7-S2-A06 |
| D2-S18-A02 ExecuteToolRound | **新增**（自 S16 拆出 tool 面） |
| D2-S15-A01 | 增加 `CompressHint` 输出 F |

### 5.3 D3 消费方注释修订

| 契约 | 旧 | 新 |
|------|-----|-----|
| `ILLMGateway` doc | D2 consumer | **D7 primary consumer** |
| `AdaptToContextEngine` F 名 | 保留 Legacy 别名 | v2.0 增 `AdaptToOrchestrator` |

---

## 6. T 层映射（Legacy Archive 草案）

| Legacy T ID | Canonical T ID | Canonical S | 域 |
|-------------|----------------|-------------|-----|
| D2-S16-A01-T01 | D7-S2-A06-T01 | S2 Turn | D7 |
| D2-S16-A01-T02 | D7-S2-A06-T02 | S2 Turn cancel | D7 |
| D2-S16-A01-T03 | D2-THIN-T01 | import lint | D2 |
| D2-S10-A01-T34 | D7-S2-A06-T03 | multi-turn loop | D7 |
| D2-S10-A01-T35~T42 | D2-S15/S18/S19-T* | 按机制拆分 | D2 |
| （新增） | D7-S2-A07-T01 | RouteModel+Stream | D7 |
| （新增） | D7-S2-A06-T04 | SubQuery nested turn | D7 |
| （新增） | D2-S15-A01-T10 | CompressHint no LLM | D2 |

> v1.0：不改 `// T:` 注释；上表写入 `t-registry.md` §Legacy Archive。

---

## 7. 跨域边界文档增量

### 7.1 新建 `d7-orchestration/d3-boundary.md` 要点

| 方向 | 规则 |
|------|------|
| D7 → D3 | `ILLMGateway.ChatStream`；先 `ITierResolver` |
| D3 → D7 | 仅 Chunk/Error 返回；Breaker 事件经 D5/D7 EngineEvent |
| D2 → D3 | **禁止** |
| Bridge | `internal/bridges/llm/`；SoT 跨域锚点 |

### 7.2 修订 `cross-domain-boundaries.md` §2.1.2

| 概念 | 旧 SoT | 新 SoT |
|------|--------|--------|
| Turn 主循环 | D2-S16 | **D7-S2-A06** |
| LLM 调用编排 | D3 内部 + D2 触发 | **D7 触发** → D3 执行 |
| 上下文组装 | D2 | D2（不变） |
| `ILLMGateway` 消费方 | D2 定义 | D2 **废弃消费**；D7 消费 |

---

## 8. v2.0 迁移 Slice

| Slice | 任务 | 产出 | T |
|-------|------|------|---|
| **a** | `orchestration/turn/` 骨架 + 接口 | `orchestrator.go` `llm.go` | — |
| **b** | `WireContextLLM` bootstrap D2→D7 | `bootstrap/turn_wiring.go` | D7-S2-A07 |
| **c** | FastPath 改 `TurnOrchestrator` | `fastpath.go` | D7-S2-A06 P0 |
| **d** | D2 移除 `ILLMGateway`；拆 `query.Loop` | `engine.go` | D2-THIN-T01 |
| **d** | **import lint D2→D3（P0 博弈论关键）** | CI 硬阻断 | D2-THIN-T01 |

> **D2-THIN-T01 lint 博弈论 rationale（双边共识 G-10）：** import lint 是整个 D7 Turn 编排机制中强制力最高的 commitment device。没有它，D2→D3 禁令在时间不一致性压力下必然退化——开发者因 deadline 压力走捷径时，CI 阻断提供了"停下来重新思考"的硬边界。它的存在不是为了防止"故意违规"，而是为了防止"不经意间的架构退化"。v1.0 即登记 lint 规则文本；v2.0-d CI 启用硬阻断。
| **e** | Autocompact D7→D3 | 删 `llm_summarizer` 直连 | D2-S15-T10 |
| **f** | Legacy `QueryLoopExecutor` adapter + T 全绿 | re-export 1 周期 | 全量 P0 |

---

## 9. Bootstrap 接线（v2.0-b 目标）

```text
bootstrap/main.go
  ├── WireContextLLM(obs) → TurnOrchestrator deps  # 迁出 context_engine
  ├── WireContextEngine() → ContextPreparer only   # 无 LLM 字段
  └── WireCoordinator(turnOrch, ctxPrep, ...)      # D7 持有 LLM
```

---

## 10. 灰区契约

### 10.1 Autocompact（D6 决议）

1. D2 `Prepare` 检测 token 超限 → 返回 `CompressHint`
2. D7 调 D3 摘要（独立 `TurnScopeCompress`）
3. D2 `MergeSummary` 合并 messages（纯 D2，无 LLM）
4. 重新 `Prepare` → 正常 Turn

**D7 拒绝压缩的降级策略（双边共识 G-09）：** 若 D7 无法执行摘要（D3 超时/Breaker/配额耗尽），降级顺序为：
1. **Truncation** — D2-S15 使用纯截断策略（保留最近 N 条消息），无需 LLM
2. **排队重试** — 若为瞬时故障（Breaker half-open），短暂延迟后重试
3. **显式用户错误** — 若所有降级手段耗尽，返回 EngineEvent 错误并保持消息不丢失

**博弈含义：** 降级策略确保 D2 不会因为"D7 拒绝压缩"而被逼回 D2→D3 的旧路径。这是 D7 Turn Leader 产权的完整性保证——D7 承担 LLM 编排责任的全部后果（包括失败路径）。

### 10.2 SubQuery（OQ3）

1. D2-S19 暴露 `NestedScope`（session view + read-only tools）
2. D7 创建 `TurnRequest{Scope: subquery}` 递归 `RunTurn`
3. Flow 发布仍归 D7 Hub-Spoke（DM-018）；D2 不 Publish

**递归深度限制（双边共识 G-08）：** `MaxDepth = 3`（主 Turn + SubQuery + SubSubQuery），与 D4-S19 / `nested/` 现有限制对齐。超出深度返回错误而非静默截断。

**博弈含义：** 递归嵌套 Turn 创造子博弈完美均衡——同一 `RunTurn` 代码路径在主 Turn 上的所有机制改进（Breaker 感知、取消传播、span 锚定）自动应用到所有嵌套层。

### 10.3 D3-S5 vs D2-S18

保持 `cross-domain-boundaries.md` §2.1.3 不变：D3 内容拒优先，D2 tool 兜底。

---

## 11. 风险与回滚

| 风险 | 缓解 |
|------|------|
| FastPath 回归 | slice c 独立 PR；P0 T 先绿 |
| Wave Worker 仍旧路径 | v2.0-f 后改 `wave/runners` |
| 双接线期混乱 | Legacy adapter 1 周期 + 日志 deprecation |

**回滚：** v2.0-f 前可随时回退到 `QueryLoopExecutor` adapter；registry 双轨保持追溯。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 0.1 | 2026-06-14 | 初稿：接口 + 状态机 + Gherkin + T 映射 + slice |
| 0.2 | 2026-06-15 | 双边共识落盘：MaxDepth=3、CompressHint 降级策略、D2-THIN-T01 lint rationale、Breaker P0 sad path Gherkin |
