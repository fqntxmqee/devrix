# Tasks: devrix-im-card-ctx

**Demand ID:** DM-20260611-008  
**Design:** [design.md](./design.md)

---

## Phase 1 — D1 摘要 + 单测（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T1 | `summary.go` 新增 `ComputeCtxPct` helper（导出供 D2 复用） | summary | L5-1-1-04 | ~50 | [x] |
| T2 | `summary.go` `buildCompletionSummary` 签名加 `ctxPct` 参数 | summary | L5-1-1-03 | ~30 | [x] |
| T3 | `summary_test.go` 补 5 个 case（含/无 ctx/0/100/garbage） | summary | L5-1-1-03 | ~80 | [x] |
| T4 | `gateway/gateway.go:421-425` 读取 `ctx_pct` metadata 并传参 | gateway | L5-1-1-03 | ~20 | [x] |

**Phase 1 验收**：L5-1-1-03~04 单测绿 ✅

## Phase 2 — D2 emit complete 透传 ctx_pct（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T5 | `pev_engine.go:521-527`（主路径）emit 前计算 ctx_pct 并写入 metadata | pev | L5-2-1-13 | ~30 | [x] |
| T6 | `pev_engine.go:139-148`（milestone-only 路径）metadata 加 `llm_called: "false"` | pev | L5-2-1-13 | ~10 | [x] |
| T7 | `query_loop_run.go:177-191`（query loop 路径）emit 前计算 ctx_pct | query | L5-2-1-14 | ~30 | [x] |

**Phase 2 验收**：集成测试发 complete 事件 → D1 gateway 收到的 metadata 含 `ctx_pct` ✅

## Phase 3 — D5 Span 全埋点（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T8 | `pev_engine.go:530-540` 补 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct` / `pev.max_context_tokens` / `pev.llm_called` | observability | L5-2-1-13 | ~30 | [x] |
| T9 | `query_loop_run.go:194-202` 新增 queryRunSpan + 补同样属性 | observability | L5-2-1-14 | ~40 | [x] |

**Phase 3 验收**：Jaeger `pev.run` / `query.run` span 可见新属性 ✅

## Phase 4 — 端到端验证（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T11 | 真机 miniMax 调用：人工肉眼核对 IM 卡片 | — | — | ~30 | [ ] |
| T12 | Jaeger 截图归档：链路 span + ctx_pct 属性 | observability | — | ~30 | [ ] |
| T13 | token=0 根因：按 design §2.1 判定（provider vs Devrix 链路） | — | — | ~60 | [ ] |

**Phase 4 验收**：IM 卡片显示符合预期，token=0 根因已记录 ✅

---

## 合并 PR 策略

- PR1: T1-T4（Phase 1，D1 摘要 + 单测）
- PR2: T5-T7（Phase 2，D2 透传）
- PR3: T8-T13（Phase 3-4，观测 + 验收）

PR 间相互独立，可独立回滚。
