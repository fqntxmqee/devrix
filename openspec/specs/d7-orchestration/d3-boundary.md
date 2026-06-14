# D7 ↔ D3 跨域边界规范

**Capability:** d7-d3-boundary  
**Status:** Active  
**Version:** 1.0.0  
**Last Updated:** 2026-06-15  
**Change ID:** devrix-d7-turn-orchestration  
**Demand ID:** DM-20260614-020  
**Parent (D7):** `openspec/specs/d7-orchestration/d7-domain.md`  
**Parent (D3):** `openspec/specs/d3-llm-gateway/spec.md`

---

## 1. 关系摘要

| 域 | 角色 | 一句话 |
|----|------|--------|
| **D7** | Turn Leader / ILLMGateway 主消费方 | 唯一有权决定何时、以何种参数调用 D3 |
| **D3** | LLM 能力提供方 | 暴露 `IGateway` / `ITierResolver`，不参与编排决策 |

**产权声明（DM-020 — 双边共识 G-07）：** D7 是唯一有权决定何时、以何种参数调用 D3 的域。D2 拥有"请求 LLM 结果"的权利（通过 CompressHint），但不拥有"执行 LLM 调用"的权利。

---

## 2. 调用链 SoT

```text
D7-S2-A06 RunTurnLoop
    ├── D7-S2-A07 InvokeLLM
    │       ├── D3-S1 ITierResolver.ResolveModel    # RouteModel before StreamChat
    │       └── D3-S2 IGateway.ChatStream           # LLM 流式调用
    ├── D2-S15 PrepareExecutionContext               # 上下文组装（D7 调用 D2）
    ├── D2-S18 ExecuteToolRound                      # 工具执行（D7 调用 D2）
    └── D2-S17 PersistSessionState                   # 会话持久化（D7 调用 D2）
```

**实现锚点：** `internal/bridges/llm/bridge.go` — 跨域契约实现，Bridge 不属于任一域内。

---

## 3. 边界规则

| 方向 | 规则 | 锚点 |
|------|------|------|
| D7 → D3 | `ILLMGateway.ChatStream`；先 `ITierResolver.ResolveModel` | `internal/bridges/llm/` |
| D3 → D7 | 仅 `Chunk` / `Result` / `error` 返回；Breaker 事件经 D5/D7 EngineEvent | `shared/contracts/` |
| D2 → D3 | **禁止** — import lint CI 硬阻断（v2.0-d） | `internal/lint/layer/d2_no_d3_test.go` |
| Bridge | `internal/bridges/llm/`；SoT 跨域锚点；D7 消费，D3 提供 | — |

---

## 4. 职责矩阵

| 能力 | D7 | D3 | D2 |
|------|----|----|-----|
| LLM 调用决策 | ✅ SoT | ❌ | ❌ |
| RouteModel (tier 选择) | ✅ 调用方 | ✅ 执行方 | ❌ |
| StreamChat | ✅ 编排方 | ✅ 执行方 | ❌ |
| Breaker / Retry / Fallback | ❌ | ✅ SoT | ❌ |
| GuardContent (safety) | ❌ | ✅ SoT | ❌ |
| Token 预算 | ❌ | ✅ SoT | ❌ |
| 上下文组装 / 压缩 | ❌ | ❌ | ✅ D2-S15 |
| CompressHint → LLM 摘要 | ✅ 调 D3 | ✅ 执行 | ✅ 提议 + 合并 |
| 工具执行 | ❌ | ❌ | ✅ D2-S18 |
| 会话持久化 | ❌ | ❌ | ✅ D2-S17 |

---

## 5. 契约接口

| 接口 | 定义位置 | 实现 | 消费 |
|------|----------|------|------|
| `IGateway` | `shared/contracts` | D3 `llmgateway.Gateway` | D7-S2-A07 InvokeLLM |
| `ITierResolver` | `shared/contracts` | D3 `llmgateway.Router` | D7-S2-A07（RouteModel 前） |
| `Chunk` / `Request` / `Result` | `shared/contracts` | D3 kernel 类型 | D7 消费 |
| `BreakerStateObserver` | `shared/contracts` | D3 breaker observer | D7 订阅 breaker 事件 |

---

## 6. 灰区契约

### 6.1 Autocompact（DM-020）

1. D2 `Prepare` 检测 token 超限 → 返回 `CompressHint`
2. D7 调 D3 摘要（独立 `TurnScopeCompress`）
3. D2 `MergeSummary` 合并 messages（纯 D2，无 LLM）
4. 重新 `Prepare` → 正常 Turn

**降级策略（双边共识 G-09）：**
1. Truncation — D2-S15 纯截断（保留最近 N 条），无需 LLM
2. 排队重试 — Breaker half-open 短暂延迟后重试
3. 显式错误 — 所有降级耗尽，EngineEvent 错误，消息不丢失

### 6.2 Breaker 事件（DM-020 / D6-A 决议）

- 事件名：`flow.breaker.opened` / `flow.breaker.closed` / `flow.breaker.halfopened`
- 复用 EngineEvent 字段（`SessionID` / `FlowID` / `Timestamp`）
- D7 订阅并根据 Breaker 状态选择路由/fallback

---

## 7. 依赖规则（目标态）

```text
✅ D7 → D3（IGateway / ITierResolver via bridges/llm）
✅ D7 → D2（ContextPreparer / ToolRoundExecutor / SessionPersister）
❌ D2 → D3（禁止：llmgateway, bridges/llm）
❌ D2 → D7（已有 lint）
❌ D3 → D7（D3 不依赖编排层）
```

---

## 8. 相关文档

| 文档 | 用途 |
|------|------|
| `d7-domain.md` | D7 Turn Leader SoT |
| `openspec/specs/architecture/cross-domain-boundaries.md` §2.1 + §2.4 | D3 vs D2/D7 跨域 SoT |
| `openspec/specs/d2-context-engine/d7-boundary.md` | D2↔D7 跨域 SoT |
| `openspec/changes/devrix-d7-turn-orchestration/design.md` | 设计决策 + 接口契约 |
| `openspec/changes/devrix-d7-turn-orchestration/gaming-analysis.md` | 博弈论分析 |

---

## 9. 变更记录

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0.0 | 2026-06-15 | 初版：D7↔D3 边界声明（DM-020 Phase B v1.0 Registry） |
