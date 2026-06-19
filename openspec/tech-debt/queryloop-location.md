# Tech Debt: D2 QueryLoop 位置错位 — Legacy Path Decommission

**TD ID:** TD-QL-LOC  
**Status:** **CLOSED** (2026-06-18)  
**Closed by:** `devrix-d2-queryloop-dismantle` (DM-20260618-010), PR #91  
**Archive:** `openspec/archive/2026-06-18-devrix-d2-queryloop-dismantle/`

---

## Closure Summary

D2 `query/loop.go`、D2 `QueryLLMCaller`、`d2_query_loop_legacy_invocations_total` 与 `query_loop.enabled` 配置均已删除；`contextengine/query/` 孤儿目录亦于 residual cleanup 移除。所有 LLM↔Tool 循环由 D7 `RunTurn` / `SubTurn` 持有；D2 仅提供 Prepare / ToolRound / Persist 原语。

| Z 阶段 | 原目标 | 结果 |
|--------|--------|------|
| Z0 | Deprecated + metric | ✅ 已由 DM-20260617-001 完成，后被 Z2 取代 |
| Z1 | thin wrapper → D7 | ✅ 由 SubTurn / PreparedTurnRunner 直接替代 |
| Z2 | 删除 Loop + config + adapter | ✅ DM-20260618-010 |

**后续 tech-debt：** TD-QL-02/06 见 `queryloop-error-recovery.md`（413 恢复已闭合于 `turn/recovery.go`）。

---

## Historical Context (pre-2026-06-18)

<details>
<summary>展开：dismantle 前的债务描述（只读归档）</summary>

D2 域内 `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 曾持有 while(tool_use) 循环主逻辑（PEV 时代遗留）。DM-020 将编排上移 D7；DM-20260617-001 为 legacy 路径加了 Deprecated 与 metric；DM-20260618-010 完成物理删除与调用方迁移。

</details>
