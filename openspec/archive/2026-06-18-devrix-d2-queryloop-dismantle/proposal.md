# Proposal: D2 QueryLoop 拆解

**Change ID:** `devrix-d2-queryloop-dismantle`  
**Demand ID:** DM-20260618-010  
**Status:** Draft  
**Created:** 2026-06-18  
**Tech Debt:** `openspec/tech-debt/queryloop-location.md` (TD-QL-LOC Z1/Z2)  
**Prior Art:** DM-20260617-001 (Z0 signals only, PR #54)

---

## Problem Statement

D2 领域定位是 **纯上下文组装服务**（Prepare / 单次 ToolRound / Persist），循环调度与 LLM 调用权归 D7。现状：

1. **主 ingress** 已走 D7 `RunTurnLoop`（`loop_first` 默认）✅  
2. **`query/loop.go`** 仍持有完整 while-loop，且被 Wave/SubQuery/Background **活跃依赖** ❌  
3. bootstrap 反向注入 `QueryLLMCaller` 进 D2 Engine，语义上 D2 仍绑定 LLM 轮次 ❌  

这与 `d2-domain.md` North Star 和 DM-020 终态设计不一致。Z0 只加了 Deprecated 信号，未做结构性拆解。

## Proposed Solution

### 核心策略

**先迁调用方 → 再删 Loop → 最后瘦 D2 Engine**

```
Phase 1: D7 SubTurn API
  turn.TurnScope = main | sub | background | wave_worker
  turn.SubTurnRequest { ParentSession, AgentID, MaxTurns, ReadOnlyTools, FlowReporter }

Phase 2: 迁移三条活跃路径
  enforce/subquery.go  → 调 D7 SubTurnExecutor（不再 Loop.Run）
  enforce/background.go → 同上（async wrapper 保留）
  wire_wave.go         → SubAgentRunner.Start → D7 SubTurn

Phase 3: 删除 legacy
  删 query/loop.go, query_loop_export.go 部分
  删 turn/query_llm_caller.go
  engine.go Process 路径改为 Deprecated facade 或删除
  删 routing_mode=rule_orchestrate / query_loop.enabled

Phase 4: Spec + recovery 收编
  D2-S16 → REMOVED
  TD-QL-01~03 迁入 D7 turn/recovery（或登记 defer）
```

### 目标架构

```text
┌─────────────────────────────────────────────────────────┐
│ D7 RunTurnLoop (唯一 while-loop)                        │
│   scope=main      ← FastPath ingress                    │
│   scope=sub       ← delegate_subquery                   │
│   scope=background← async delegate / background tools   │
│   scope=wave_worker ← Wave SubAgentRunner               │
│                                                         │
│   each iteration:                                       │
│     D2.Prepare → D7.InvokeLLM(D3) → D2.ExecuteToolRound │
│     → D2.Persist                                        │
└─────────────────────────────────────────────────────────┘
         │                    │
         ▼                    ▼
    D2 (stateless)       D3 Gateway
    prepare/             StreamChat
    enforce/             (LLM only)
    persist/
```

D2 **零** import：`orchestration/`, `llmgateway/`, `multiagent/`（维持现状 + CI 硬阻断 D2→D3）。

## Scope

### In Scope

- D7 `SubTurnExecutor` / `TurnScope` 扩展
- `enforce/subquery.go` + `background.go` 迁 D7
- `wire_wave.go` SubAgent 迁 D7
- 删除 `query/loop.go` 及关联测试
- 删除 `QueryLLMCaller` adapter
- D2 `engine_builder.go` 去除 QueryLLMCaller 硬依赖
- Spec delta（D2-S16 REMOVED, D7 SubTurn ADDED）
- 更新 `queryloop-location.md` → CLOSED

### Out of Scope

- WorkTree v2.1 legacy 清理（TD-WT-02/03）
- D4 Worker 实现变更
- 飞书 UI
- 完整 TD-QL-04 D6 探针（可 follow-up）

## Impact Analysis

| 组件 | Change | Details |
|------|--------|---------|
| `contextengine/query/` | **DELETE** | `loop.go` + tests |
| `contextengine/engine.go` | **MODIFY** | Process 不再 Run loop |
| `contextengine/engine_builder.go` | **MODIFY** | 无 QueryLLMCaller |
| `enforce/subquery.go` | **MODIFY** | D7 executor 替代 Loop.Run |
| `enforce/background.go` | **MODIFY** | 同上 |
| `bootstrap/wire_wave.go` | **MODIFY** | SubAgent → D7 |
| `bootstrap/delegate.go` | **MODIFY** | SubQueryRunner → D7 |
| `bootstrap/context_engine.go` | **MODIFY** | 去除 loop wiring |
| `turn/orchestrator.go` | **MODIFY** | SubTurn scopes |
| `turn/query_llm_caller.go` | **DELETE** | 拆面 adapter |
| `coordinator/routing.go` | **MODIFY** | 删 rule_orchestrate |
| D2/D7 spec | **MODIFY** | S16 REMOVED, SubTurn ADDED |

## Success Criteria

- [ ] AC1–AC10（见 `demand.md`）全部 PASS
- [ ] `grep -r 'Loop\.Run' internal/` 零命中（测试 stub 除外）
- [ ] `grep -r 'QueryLLMCaller' internal/` 零命中
- [ ] `d2_query_loop_legacy_invocations_total` 注册移除或恒为 0

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SubQuery sidechain 行为回归 | 高 | 保留 `enforce/subquery_test.go` 场景，改调 D7 stub |
| Wave SubAgent 取消语义 | 中 | SubTurn ctx cancel → RunRegistry/Background cancel 对齐 |
| 413/fallback 丢失 | 中 | Phase 4 显式 port 或 defer + tech-debt 更新 |
| 集成测试覆盖面大 | 中 | 分 Phase PR，每 Phase 独立 CI 绿 |

## Suggested PR Split

| PR | Phase | 可独立合并 |
|----|-------|-----------|
| #1 | Phase 1 — SubTurn API + tests | ✅ |
| #2 | Phase 2 — SubQuery/Background/Wave 迁移 | 依赖 #1 |
| #3 | Phase 3 — 删 Loop + legacy config | 依赖 #2 |
| #4 | Phase 4 — recovery + spec 合并 | 依赖 #3 |

## Next Steps

1. S3-Gate Review 本 proposal + design.md
2. `/openspec-apply devrix-d2-queryloop-dismantle` 从 Phase 1 开始
3. 完成后 `/openspec-archive` + 关闭 TD-QL-LOC
