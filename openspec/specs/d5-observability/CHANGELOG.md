# D5 Observability — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v3.1.0，≤ 200 行）
> - **域 SoT** = [d5-domain.md](d5-domain.md)（v3.0.0，North Star + DSAFT 资产 + 边界 SoT）
> - **D5 Boundary** = [d5-boundary.md](d5-boundary.md)（D5↔D2/D3/D4/D7 跨域边界）
> - **变更类型说明** = IMPLEMENTED / PARTIAL / SUPERSEDED / OBSOLETE
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 5 条 d5 change（含本 change）

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-06-30 | devrix-d5-spec-lite | d5 spec.md 376→150 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d5-spec-lite/) |
| 2026-06-19 | devrix-d5-v2-terminal | d5 v3.0.0 收口（DM-20260618-013）：v2.1 Terminal S21-S24+S0 冻结 + D7 Turn 主路径 canonical + query.loop 全部下沉 RETIRED + legacy_harness DEPRECATED | IMPLEMENTED | [archive](../../archive/2026-06-19-devrix-d5-v2-terminal/) |
| 2026-06-18 | devrix-queryloop-spans-v1.1 | QueryLoop span 族最后清理（DM-20260618-010）：query.loop.* span RETIRED + orchestration.turn.* 主路径 + Coverage 重新对齐 | IMPLEMENTED | [archive](../../archive/2026-06-18-devrix-queryloop-spans-v1.1/) |
| 2026-06-15 | devrix-d5-sa-refine | D5 SA Refine（DM-20260614-019）：S21-S24 + S0 canonical Scenario 收口 + 13 条 Requirements 详细 Gherkin 沉淀 | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-d5-sa-refine/) |
| 2026-06-15 | devrix-d5-d6-sa-refine-v2.0 | D5+D6 SA Refine v2.0（DM-20260614-020）：Operation Registry D5↔D6 跨域对齐 + Boundary Debt 部分登记 | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-d5-d6-sa-refine-v2.0/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d5 历史，访问 `openspec/archive/`，命名格式 `YYYY-MM-DD-devrix-{name}`。主要历史里程碑：
- 2026-06-07 ~ 06-10：V1.0-V1.9 基线（observability / observability-fix / observability-coverage / harness-bootstrap / observability-baggage / observability-enhancement P0-P3 / observability-token-breakdown）
- 2026-06-10：V2.0 QueryLoop span 族 + Orchestration span（queryloop-context）
- 2026-06-15：harness unification v1.1
- 2026-06-17：QueryLoop Legacy 退役（queryloop-legacy-decommission）
- 2026-06-18：QueryLoop 拆解（d2-queryloop-dismantle）

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D5 Observability 公共域 + v2.1 Terminal canonical + query.loop 退役 | devrix-d5-v2-terminal + devrix-queryloop-spans-v1.1 |
| 核心设计原则 | 8 条（Canonical Op / Coverage 独立采样 / Bridge 零侵入 / Metrics 前缀 / Log-Trace-LLM 三联 / Graceful Degradation / Layer-Component 编码 / D7 Turn 主路径） | devrix-d5-sa-refine + devrix-d5-v2-terminal + devrix-harness-unification-v1.1 |
| S 层职责 | canonical D5-S21..S24 + S0 | devrix-d5-sa-refine + devrix-d5-v2-terminal |
| DSAFT 结构 | 1 D + 5 S + 30 A + 45 F + 41 T + 56 Operation | devrix-d5-v2-terminal |
| Scenarios | 5 canonical S 状态表 | devrix-d5-sa-refine（canonical Gherkin 详细文本） |
| Architecture | D7 Turn 主路径图 + Bridge + Coverage + Diagnostic 工具链 | devrix-d5-v2-terminal + devrix-queryloop-spans-v1.1 |
| 关键 Scenario 范式 | 1 canonical：D5-S23 Coverage HealthCheck 运行时命中计数 + Operation Registry 对账 | devrix-d5-sa-refine + devrix-d5-v2-terminal |
| 关键链路口 | 6 端到端路径（D7 Turn / GenAI Token / Coverage / Diagnostic / W3C Baggage / Runtime Path） | devrix-d5-v2-terminal + devrix-harness-unification-v1.1 |

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段 + [d5-domain.md](d5-domain.md)（North Star + DSAFT）+ [d5-boundary.md](d5-boundary.md)（D5↔D2/D3/D4/D7 边界）+ Boundary Debt 决议登记
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）