---
demand-id: DM-20260619-005
title: D7 v2.0 结构重构 — 物理路径对齐 + WorkTree Legacy 清债
source: DSAFT Refactoring Playbook v2.0 Structure + Owner 对焦（2026-06-19）
priority: P0
status: S7_Archived
dsaft_domain: orchestration
created: 2026-06-19
---

# D7 v2.0 结构重构

## 1. 背景

D7 Orchestration 在功能层已闭环（`spec.md` v3.8.0：S1–S5 IMPLEMENTED，`t-registry` 66/66 T 全绿）。DM-20260617-009 交付 WorkItem/WorkTree/RunRegistry 核心能力。

然而 **v2.0 Structure（物理路径对齐 S 层）** 尚未落地：

| S 层 | 目标路径 | 当前路径 |
|------|----------|----------|
| D7-S2 | `sessionorchestrator/` | `coordinator/` + `turn/` + `hubspoke/dispatch` |
| D7-S3 | `wavescheduler/` | `wave/` |
| D7-S4 | `executionflow/` | `flow/` + `workplan/` + `imsink/` + `hubspoke/bridge` |
| D7-S5 | `decisionplanning/` | `coordinator/classifier*` |

`coordinator/` 单包（~38 文件）同时承载 S2（入口+Turn 路由）与 S5（分类+拆解），违反 DSAFT「S = 用户价值流」原则。

## 2. Owner 决议（2026-06-19）

| # | 议题 | 决议 |
|---|------|------|
| Q1 | 重构范围 | **C** — 文档同步 + 物理路径 + WorkTree 清债（TD-WT-02/03） |
| Q2 | 总编排权 | **sessionorchestrator** 持有 ProcessMessage，调 decisionplanning 接口 |
| Q3 | hubspoke | **物理拆分** — dispatch→S2，bridge→S4 |
| Q4 | T 层 | **T ID 不变**，测试随实现迁移 |
| Q5 | D2 | **本轮不动** |

默认项（Owner 未反对）：`turn/` 保持独立；S4 用 `executionflow/{hub,workplan,imsink}` 子目录；`coordinator` 保留 1 release shim；North Star 不变。

## 3. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | S2/S5 共包 `coordinator/` | 变更爆炸半径大；新功能易侵蚀 Legacy 双轨约束 |
| P2 | S4 物理分散（flow/workplan/imsink/hubspoke） | 双锚点断裂；onboarding 无法从目录回答「属于哪个 S」 |
| P3 | 规格文档漂移 | design.md / layer-delta / d7-boundary 与代码 IMPLEMENTED 状态矛盾 |
| P4 | WorkTree vs TaskNode 双 SoT（TD-WT-02） | Wave 调度与语义树可能分歧 |
| P5 | sc.Todos 仍可能被误作写入 SoT（TD-WT-03） | D2 scratch 与 D7 WorkTree 产权模糊 |

## 4. 目标行为

```text
internal/layers/orchestration/
├── workmodel/              # D7-S1 ✅（保持）
├── sessionorchestrator/    # D7-S2（自 coordinator 迁出入口/路由/interrupt/fastpath）
├── turn/                   # D7-S2-A06/A07（保持独立）
├── decisionplanning/       # D7-S5（自 coordinator 迁出 classifier/decomposer/executor）
├── wavescheduler/          # D7-S3（自 wave/ 重命名）
├── executionflow/          # D7-S4
│   ├── hub/
│   ├── workplan/
│   └── imsink/
├── delegatetools/          # F 层（保持）
├── sessionqueue/           # F 层（保持）
├── toolpolicy/             # F 层（保持）
├── runregistry/            # S1 支撑（保持）
└── coordinator/            # Legacy shim（1 release re-export，随后删除）

Wave 调度：只读 WorkTree，TaskNode 退化为 dispatch 投影（TD-WT-02）
sc.Todos：只读投影，写入路径 audit 为零（TD-WT-03）
```

## 5. 验收标准（AC）

### Phase A — 文档同步

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-A1 | `design.md` 刷新至 v3.x：S1–S5 IMPLEMENTED，删除过时 PLANNED | P0 |
| AC-A2 | `layer-delta.md` 新增 §v2.0-Structure；旧 §PLANNED 标 HISTORICAL | P0 |
| AC-A3 | `d2-context-engine/d7-boundary.md` Task/PlanMode 🔶→✅ | P0 |
| AC-A4 | `code-layout.md` §4.2 D7 表全部 ✅；hubspoke 拆分登记 | P0 |
| AC-A5 | `dsaft-architecture.md` 从 Stub 升级为真实五层计数 | P1 |

### Phase B — 物理路径

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-B1 | `wave/` → `wavescheduler/`，import 全量更新 + shim 可选 | P0 |
| AC-B2 | `flow/`+`workplan/`+`imsink/` → `executionflow/{hub,workplan,imsink}/` | P0 |
| AC-B3 | `coordinator/` 拆为 `sessionorchestrator/` + `decisionplanning/` | P0 |
| AC-B4 | `hubspoke/dispatch` → S2；`hubspoke/bridge` → S4 executionflow | P0 |
| AC-B5 | `coordinator/` 保留 re-export shim，bootstrap 切新路径 | P0 |
| AC-B6 | `a-registry.md` Code Location 列同步新路径 | P0 |
| AC-B7 | 66/66 T 全绿；T ID 无变更 | P0 |
| AC-B8 | `go test ./...` + layer-lint strict 全绿 | P0 |

### Phase C — WorkTree Legacy 清债

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-C1 | Wave 调度仅读 WorkTree；`wave.TaskNode` 不写盘（TD-WT-02） | P0 |
| AC-C2 | `sc.Todos` 标记 Deprecated 读投影；写入 audit 为零（TD-WT-03） | P0 |
| AC-C3 | `tech-debt/worktree-v2-deferred.md` TD-WT-02/03 标 CLOSED | P0 |

## 6. 非目标

- D2 `IEngine.Process` / PreparedTurnRunner 链改造
- TD-WT-01（自演化 optimizer）、TD-WT-04（跨 Session UI）、TD-WT-05/06
- 新增或重命名 D-S-A-F-T ID
- North Star 变更

## 7. 依赖

| 依赖 | 状态 |
|------|------|
| D7 v1.0–v2.0 功能闭环 | ✅ |
| DM-20260617-009 WorkTree 核心 | ✅ |
| DM-20260618-010 QueryLoop 拆解 | ✅ |
| 66/66 T IMPLEMENTED | ✅ |
