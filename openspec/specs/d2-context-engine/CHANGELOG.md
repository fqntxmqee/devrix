# D2 Context Engine — Changelog

> **时间线列表（Lite-Mode）**。每个 change 一行 + 一句话摘要 + 链接到 `archive/`。
>
> - **spec.md 详细 Scenario 演进** = 在 `archive/<change>/specs/` 各 change 目录
> - **当前符合代码的设计契约** = [spec.md](spec.md)（v9.0.0，≤ 200 行）
> - **域 SoT** = [d2-domain.md](d2-domain.md)（v9.0.0，North Star / 物理路径 / 实现状态）
> - **D7 边界契约** = [d7-boundary.md](d7-boundary.md)（Leader/Follower 契约）
> - **变更类型说明** = IMPLEMENTED（已合入代码）/ PARTIAL（部分合入）/ SUPERSEDED（被替代）/ OBSOLETE（已废弃）
> - **最近 30 天** = 2026-05-31 ~ 2026-06-30，共 28 条 d2 change

---

## 时间线（最近 30 天）

| Date | Change ID | 摘要 | 状态 | 归档 |
|------|-----------|------|------|------|
| 2026-07-01 | devrix-d2-d7-review-hardening | D2 安全硬化 (P0-A PlanModeWriteParity + SymlinkContainment + AutocompactWriteback) + D2 fail-closed (P1-B1 5 surface: nil bashAST→Deny / sandbox disabled warn / bashAST parse→Deny / unknown threshold strictest / bash audit redaction) + D2 压缩并发 (P1-B2 memory/manager CompressedView mu 保护 + async_compact session-scoped ctx + microcompact 跳过 tool msg) + D2 JSONL 卫生 (P2 materialize/store.go strict 模式 + LoadAgent 镜像 + truncateForLog); 15 T IMPLEMENTED (D2-S15-A80/A81/A82/A83 + D2-S17-A80 + D2-S18-A80/A81/A82/A83/A84/A85) | IMPLEMENTED | [archive](../../archive/2026-07-01-devrix-d2-d7-review-hardening/) |
| 2026-06-30 | devrix-d2-spec-lite | d2 spec.md 1622→152 lite-mode (12 AC) | IMPLEMENTED | [archive](../../archive/2026-06-30-devrix-d2-spec-lite/) |
| 2026-06-29 | devrix-d2-dsaft-restructuring | v8.2→v9.0 8 PR / 44 T / 14 G 全 PASS（legacy + god fn + registry + value flow + span + boundary debt） | IMPLEMENTED | [archive](../../archive/2026-06-29-devrix-d2-dsaft-restructuring/) |
| 2026-06-28 | devrix-d7-layer-subcontext-phase3 | Wave ContextResolver → Materialize Policy 接线 | IMPLEMENTED | [archive](../../archive/2026-06-28-devrix-d7-layer-subcontext-phase3/) |
| 2026-06-28 | devrix-d7-layer-subcontext | Per-Layer SubContext + ChildDownlink Phase 1+2（D2-S16-A20/A21/A22 物理落地） | IMPLEMENTED | [archive](../../archive/2026-06-28-devrix-d7-layer-subcontext/) |
| 2026-06-27 | devrix-d2-agents-md-project-layout | .devrix/AGENTS.md D{N}→path 映射 + coverage test | IMPLEMENTED | [archive](../../archive/2026-06-27-devrix-d2-agents-md-project-layout/) |
| 2026-06-20 | devrix-context-budget-phase-c-nested | SubAgent 嵌套 budget bypass fix（maxContextTokens=0 → Phase A no-op 修复） | IMPLEMENTED | [archive](../../archive/2026-06-20-devrix-context-budget-phase-c-nested/) |
| 2026-06-20 | devrix-context-budget-and-isolation-phase-b | 3-mode brief/fork/full + MaxSubagentDepth=3，D5-spans P95=21707 | IMPLEMENTED | [archive](../../archive/2026-06-20-devrix-context-budget-and-isolation-phase-b/) |
| 2026-06-20 | devrix-context-budget-and-isolation | Phase A 5/5 AC + PR #128/#129 + D5 spans 51K → P99 限 | IMPLEMENTED | [archive](../../archive/2026-06-20-devrix-context-budget-and-isolation/) |
| 2026-06-19 | devrix-spec-sync-d2-layer-delta-soften | layer-delta.md 软化（S15-S18 边界细化） | IMPLEMENTED | [archive](../../archive/2026-06-19-devrix-spec-sync-d2-layer-delta-soften/) |
| 2026-06-19 | devrix-d2-structure-closure | v2.2 6 Phase + 7 layout guards + 19/19 AC，scenario orchestrator 生产 wired + legacy 退役 | IMPLEMENTED | [archive](../../archive/2026-06-19-devrix-d2-structure-closure/) |
| 2026-06-18 | devrix-queryloop-spans-v1.1 | QueryLoop spans 增补 + 命名规约 | IMPLEMENTED | [archive](../../archive/2026-06-18-devrix-queryloop-spans-v1.1/) |
| 2026-06-18 | devrix-d2-queryloop-dismantle | D2-S10/S16 QueryLoop **物理删除**（DM-20260618-010 v8.0.0）→ D7-S2-A06 RunTurnLoop | IMPLEMENTED | [archive](../../archive/2026-06-18-devrix-d2-queryloop-dismantle/) |
| 2026-06-17 | devrix-queryloop-legacy-decommission | query_loop.enabled 退役，turn_runtime.compress_per_turn 替代 | IMPLEMENTED | [archive](../../archive/2026-06-17-devrix-queryloop-legacy-decommission/) |
| 2026-06-15 | devrix-harness-unification-v1.1 | Harness unification 1.1 修正（D2-S20 REMOVED 收尾） | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-harness-unification-v1.1/) |
| 2026-06-15 | devrix-harness-unification | Harness 路径合并 + 5 surface 接入 D2-S18 | IMPLEMENTED | [archive](../../archive/2026-06-15-devrix-harness-unification/) |
| 2026-06-14 | devrix-d2-sa-refine-v2.0-workmodel | workmodel 物理迁入 D7 orchestration/workmodel/ | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v2.0-workmodel/) |
| 2026-06-14 | devrix-d2-sa-refine-v2.0-toolpolicy | toolpolicy 物理合并入 enforce/ | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v2.0-toolpolicy/) |
| 2026-06-14 | devrix-d2-sa-refine-v2.0-sessionqueue | sessionqueue 物理合并入 executionflow/ | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v2.0-sessionqueue/) |
| 2026-06-14 | devrix-d2-sa-refine-v2.0-physical-dirs | 物理目录重整（policy/→enforce/ 等 5 处） | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v2.0-physical-dirs/) |
| 2026-06-14 | devrix-d2-sa-refine-v2.0-delegate | delegate_* 工具物理迁入 D7 delegatetools/ | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v2.0-delegate/) |
| 2026-06-14 | devrix-d2-sa-refine-v1.1 | v1.1 D2-SA-Refine 修正（wire_coordinator 升级） | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine-v1.1/) |
| 2026-06-14 | devrix-d2-sa-refine | D2-SA-Refine（DM-20260614-009）v1.0 物理拆面 + Leader/Follower 拓扑 | IMPLEMENTED | [archive](../../archive/2026-06-14-devrix-d2-sa-refine/) |
| 2026-06-10 | devrix-queryloop-context | QueryLoop 运行时（query_loop.enabled）+ UserContext API 边界 | IMPLEMENTED | [archive](../../archive/2026-06-10-devrix-queryloop-context/) |
| 2026-06-10 | devrix-harness-bootstrap | Harness Bootstrap 分阶段启动 + ToolPool + SystemPromptAssembler 四层组装 | IMPLEMENTED | [archive](../../archive/2026-06-10-devrix-harness-bootstrap/) |
| 2026-06-09 | devrix-d234-domain-testing | D2/D3/D4 域测试框架统一 | IMPLEMENTED | [archive](../../archive/2026-06-09-devrix-d234-domain-testing/) |
| 2026-06-08 | devrix-context-engine-v4 | V4 Async Autocompact + Snappy 快照压缩 | IMPLEMENTED | [archive](../../archive/2026-06-08-devrix-context-engine-v4/) |
| 2026-06-07 | devrix-context-engine-v3 | V3 PEV Plan + Milestone DAG + LongTerm SQLite | IMPLEMENTED | [archive](../../archive/2026-06-07-devrix-context-engine-v3/) |
| 2026-06-07 | devrix-context-engine-v2 | V2 Autocompact + PEV Verify commands + ITokenCounter | IMPLEMENTED | [archive](../../archive/2026-06-07-devrix-context-engine-v2/) |
| 2026-06-07 | devrix-context-engine | V1 上下文引擎基线（LoadOrInit + Bootstrap + QueryLoop） | IMPLEMENTED | [archive](../../archive/2026-06-07-devrix-context-engine/) |

---

## 历史归档（早于 30 天）

如需查阅 30 天前的 d2 历史，访问 `openspec/archive/` 目录，命名格式 `YYYY-MM-DD-devrix-{name}`。`openspec/demand-archive-index.md` 包含全部归档记录的元信息（Demand ID / 标题 / 归档日期 / PR / Verdict）。

---

## 状态映射（spec.md 索引）

| spec.md 段 | 描述 | 对应 archive 历史 |
|-----------|------|-----------------|
| Overview | D2 Context Follower + Prepare/ToolRound/Persist 三原语 + D7 Leader 关系 | devrix-d2-sa-refine + devrix-d2-queryloop-dismantle + devrix-d2-dsaft-restructuring |
| 核心设计原则 | 8 条（Context Follower / 不可变 / 七步压缩 / Snappy / Deferred Complete / Tool Surface / Hard Ban D2→D3 / Trace 树） | devrix-context-engine-v4 + devrix-harness-unification + devrix-d7-layer-subcontext* |
| S 层职责 | canonical S15-S18 + S16 REMOVED + S19/S20 历史 | devrix-d2-dsaft-restructuring v8.5→v9.0 PR-5 |
| DSAFT 结构 | 1 D + 4 canonical S + 22 A + 120 F + 180 T | devrix-d2-dsaft-restructuring |
| Architecture | Leader/Follower 拓扑 + 跨域边界 | devrix-d7-boundary.md + devrix-d2-sa-refine |
| 关键 Scenario 范式 | 1 canonical：S15 Materialize SubTurn | devrix-d7-layer-subcontext-phase3 + devrix-context-budget-phase-b |
| 关键链路口 | 6 端到端路径 | 全部 archive change 累积形成 |

---

## 维护规则

- **新增 change 时**：归档时（`changes/<id>/` → `archive/<date>-<id>/`）追加一行，按 `Date | Change ID | 摘要 | 状态 | 归档` 格式
- **架构级变更时**：修订 [spec.md](spec.md) 主体段（Overview / 核心设计原则 / S 层职责 / 关键 Scenario 范式）+ [d2-domain.md](d2-domain.md)（North Star / 实现状态）
- **超 300 行时**：精简为一行摘要 + 归档链接；超期条目（> 30 天）折叠到「历史归档」段
- **禁止**：复制 Requirement/Scenario 详细文本到本文件；创建子文件（lite-mode 不需要）