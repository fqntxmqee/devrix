# Tasks: devrix-im-card-ctx

**Demand ID:** DM-20260611-008  
**Design:** [design.md](./design.md)

---

## Phase 1 — D1 摘要 + 单测（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T1 | `summary.go` 新增 `ComputeCtxPct` helper（导出供 D2 复用） | summary | D1-S1-T04 | ~50 | [x] |
| T2 | `summary.go` `buildCompletionSummary` 签名加 `ctxPct` 参数 | summary | D1-S1-T03 | ~30 | [x] |
| T3 | `summary_test.go` 补 5 个 case（含/无 ctx/0/100/garbage） | summary | D1-S1-T03 | ~80 | [x] |
| T4 | `gateway/gateway.go:421-425` 读取 `ctx_pct` metadata 并传参 | gateway | D1-S1-T03 | ~20 | [x] |

**Phase 1 验收**：D1-S1-T03~04 单测绿 ✅

## Phase 2 — D2 emit complete 透传 ctx_pct（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T5 | `pev_engine.go:521-527`（主路径）emit 前计算 ctx_pct 并写入 metadata | pev | D2-S1-T13 | ~30 | [x] |
| T6 | `pev_engine.go:139-148`（milestone-only 路径）metadata 加 `llm_called: "false"` | pev | D2-S1-T13 | ~10 | [x] |
| T7 | `query_loop_run.go:177-191`（query loop 路径）emit 前计算 ctx_pct | query | D2-S1-T14 | ~30 | [x] |

**Phase 2 验收**：集成测试发 complete 事件 → D1 gateway 收到的 metadata 含 `ctx_pct` ✅

## Phase 3 — D5 Span 全埋点（~0.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T8 | `pev_engine.go:530-540` 补 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct` / `pev.max_context_tokens` / `pev.llm_called` | observability | D2-S1-T13 | ~30 | [x] |
| T9 | `query_loop_run.go:194-202` 新增 queryRunSpan + 补同样属性 | observability | D2-S1-T14 | ~40 | [x] |

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

---

## S4-Gate 审查结论

**审查时间**: 2026-06-12
**审查人**: team-reviewer (multi-perspective)
**结论**: ✅ **Approved**

| 维度 | 严重级别 | 结果 |
|------|---------|------|
| OpenSpec 文档完整性 | — | PASS (4 文档齐全 + T 层 4 条注册) |
| 代码质量 | — | PASS (函数 < 50 行、文件 < 800 行、嵌套 ≤ 2) |
| 错误与安全 | — | PASS (ctx% 边界覆盖、纯函数无副作用) |
| 测试完整性 | — | PASS (33 sub-case + race detector) |
| **总计** | CRITICAL: 0 / HIGH: 0 / MEDIUM: 0 / LOW: 2 | **Approved** |

**LOW 项（不阻断）**:
- LOW-1: D2-S1-T13/14 测试位置指向源文件本身；建议后续 PR 拆出独立 `_test.go` 显式断言 metadata/span 属性
- LOW-2: `gateway.ComputeCtxPct` 在 D2 调用属 pre-existing 模式（非新违规）；建议未来 refactor 移至 `internal/shared/` 让 D2 不必依赖 D1 包路径

## S5 验收备注

- ✅ D1-S1-T03/04: 14 case 全绿（boundary / happy / sad path）
- ✅ D2-S1-T13/14: PEV 主路径 + milestone-only + query loop 三条 complete emit 均含 `ctx_pct` / `llm_called`
- ✅ D5 埋点: pev.run / query.run / llm.stream span 5 个新属性 + 1 个诊断属性
- ⏳ T11-T13（Phase 4）需真机 miniMax 调用 + Jaeger 截图 + token=0 根因判定；建议在 PR 合并后由用户在预发环境跑完，作为 release gate

**token=0 排查路径**（用户用 Jaeger 自助定位）:
1. 查 `llm.stream.llm.usage_received`：
   - `false` → provider SSE 协议层不返回 usage（MiniMax M2.7-highspeed 嫌疑）
   - `true` 但 `llm.tokens.prompt=0` → 不应出现的矛盾状态
2. 查 `pev.run.pev.prompt_tokens`：
   - 0 而上一步 > 0 → D2 累加链路问题（极少见，本变更后链路已强化）
3. 都 > 0 但 IM 仍 0 → 检查 D1 gateway 部署版本

