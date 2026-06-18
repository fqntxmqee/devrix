# Tasks: Devrix 分层 ID 规范标准化

**Change ID:** devrix-layering-standard
**Demand ID:** DM-20260608-005

> **归档说明 (2026-06-18):** D-S-A-F-T 完整方案未实施；所有"原始任务"未启动。基础设施通过其他渠道落地（见 design.md §5）。

## S0 — Defer 决策（已发生）

| ID | 任务 | 状态 | 日期 |
|----|------|------|------|
| D01 | 决策 D-S-A-F-T 与 L1-L2 冲突 | ✅ DONE (deferred) | 2026-06-08 |
| D02 | 决策"维持 L1-L2 现状" | ✅ DONE | 2026-06-08 |

## 替代基础设施（其他渠道落地）

| ID | 任务 | 渠道 | 状态 | 日期 |
|----|------|------|------|------|
| I01 | 目录 ID 规范 (`code-layout.md`) | devrix-d7-orthogonal-intent-paths | ✅ DONE | 2026-06-15 |
| I02 | D1-D7 分层定义 (`layering.md`) | devrix-layer-isolation v1.0 | ✅ DONE | 2026-06-04 |
| I03 | 代码索引 (`code-atlas.md`) | devrix-layer-isolation v1.0 | ✅ DONE | 2026-06-04 |
| I04 | 顶层 t-registry.md | devrix-layer-isolation v1.0 | ✅ DONE | 2026-06-04 |
| I05 | 各域 t-registry.md | devrix-d7-sa-refine | ✅ DONE | 2026-06-14 |

## 未启动任务（D-S-A-F-T 完整方案）

以下任务明确**不实施**，归档时标注为 CANCELLED：

| ID | 任务 | 状态 |
|----|------|------|
| T01 | 全量重命名 T 点 → D-S-A-F-T 格式 | ❌ CANCELLED |
| T02 | 重构目录结构对齐 D-S 命名 | ❌ CANCELLED |
| T03 | 设计 A/F/T 注册表 | ❌ CANCELLED |
| T04 | 迁移历史 T 点 | ❌ CANCELLED |

## 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S0_Deferred → Archived；out-of-band delivery 全部完成；D-S-A-F-T 完整方案不实施。