---
demand-id: DM-20260701-004
title: D7 物理布局对齐 — A 层补全 + S1-S6 代码路径收敛
source: 架构审查后续（DM-20260701-002/003 规格归一化后的代码落地）
priority: P1
status: OPEN
l1-domain: D7 Orchestration
created: 2026-07-01
related:
  - DM-20260701-003 (Historical S cleanup)
  - DM-20260701-002 (S layer normalization)
  - DM-20260701-001 (MUPS propagation / convergence)
  - DM-20260626-001 (6 S simplification / v6.0.0 A remap)
---

# D7 物理布局对齐 — A 层补全 + S1-S6 代码路径收敛

## 1. 原始描述

> DM-20260701-002/003 已完成 D7 **规格层** canonical S1-S6 归一化、S7+ 历史正文迁出、S3 边界澄清，以及 StrategicPlanReject / child-stats uncertainty 两个小闭环。
>
> 用户追问：**后续要怎么落地？** 需要把 v6.0.0 设计里定义的 **49+ A 活动** 与 **现行代码物理路径** 对齐，消除「ValueFlow 表有、Canonical A 表缺」「historical mapping 指向已删除包」「S6 治理代码散落在 4 个目录」等漂移，并通过小步 PR 完成可验证的路径收敛。

## 2. 问题陈述

DM-002/003 明确 **Out of Scope** 了「大规模物理目录迁移」。当前状态是：**规格已收敛，代码与 A 注册表尚未完全对齐**。

| ID | 现象 | 根因 | 严重度 |
|----|------|------|--------|
| D7-PL-01 | `a-registry.md` Canonical 段仅登记部分 A（如 S1 仅 A01-A04，S6 仅 A14），ValueFlow 段却列 49 A | 002 聚焦 S 层归一化，A 层补全未做 | P0 |
| D7-PL-02 | `historical-s-mapping.md` / 旧 T 仍引用 `orchestration/observe/`、`execute/`、`verify/`、`learn/` 等路径 | 历史 MUPS 节点包已删或合并，registry 未回写 current path | P1 |
| D7-PL-03 | S6 MUPS Governance 物理分散在 `sessionorchestrator/` + `mups/` + `escape/` + `interfaces/` + `hardening/` | 002 采用 governance overlay，未定义 activity 级子路径 | P1 |
| D7-PL-04 | `plan/` 与 `decisionplanning/` 并存；`orchtypes/` 无 S 归属登记 | S5 决策规划包边界未在 code-layout 终态化 | P1 |
| D7-PL-05 | `code-layout.md` §4.2 仍列 `coordinator/`、`hubspoke/` legacy shim，仓库中已不存在 | 布局注册表滞后 | P2 |
| D7-PL-06 | 无 layout guard：新文件可能继续落入无 scenario 语义的技术目录 | 002 仅有 spec/registry guard，缺 code-layout 测试 | P1 |

### 现行物理目录快照（2026-07-01）

```
internal/layers/orchestration/
├── workmodel/              # S1 ✅
├── sessionorchestrator/    # S2 + 部分 S6（item_pipeline, verify, rollup…）
├── wavescheduler/          # S3 ✅
├── executionflow/          # S4 ✅
├── decisionplanning/       # S5 部分
├── plan/                   # S5 部分（PlanKind / DefaultPlanner）
├── mups/                   # S6 部分
├── escape/                 # S6 部分
├── interfaces/             # S6 contract + S1/S6 共享
├── hardening/              # 横切 Discipline Keeper
├── orchtypes/              # 未登记 S 归属
└── delegatetools/          # delegate 触发 S3 的 F 层
```

## 3. 澄清记录

### Q1: 是否需要强制新建 `mupsgov/` 单目录？
**A**: **否**。沿用 DM-002 governance overlay 决策：S6 **不要求**单独 scenario 目录迁移；本需求以 **A/F 注册 + 可选 activity 子目录** 为主，避免大规模 git mv 引入行为回归。—— 2026-07-01

### Q2: 是否重编号历史 A ID（如 D7-S8-A15 → D7-S5-A06）？
**A**: **否**。历史 A/F/T ID 保持不变；Canonical 段新增 **Current ID** 列指向 v6.0.0 目标编号，Legacy ID 列保留追溯。—— 2026-07-01

### Q3: 与 DM-002 Out of Scope 的边界？
**A**: 本需求是 002 的 **Phase 2 落地**：只做 **注册表对齐 + 小步物理收敛 + guard**；不做 WaveScheduler 行为变更、不做跨域 D2/D4 重构。—— 2026-07-01

## 4. 澄清范围

### 4.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | D7 | Orchestration | 已有 |
| L2 | D7-S1 | Work Model | 已有 |
| L2 | D7-S2 | Session Orchestrator | 已有 |
| L2 | D7-S3 | Wave Scheduler | 已有 |
| L2 | D7-S4 | Execution Flow | 已有 |
| L2 | D7-S5 | Decision & Planning | 已有 |
| L2 | D7-S6 | MUPS Governance | 已有 |
| L3-BE | D7-S1-A07/A08 | Rollup / ScopeContract | 草拟 → 正式 |
| L3-BE | D7-S2-A04..A07 | Dispatch / Turn / Session lifecycle | 草拟 → 正式 |
| L3-BE | D7-S4-A06..A09 | Verify 活动链 | 草拟 → 正式 |
| L3-BE | D7-S5-A04..A08 | Observe / Plan / Prior | 草拟 → 正式 |
| L3-BE | D7-S6-A01..A15 | MUPS Execute/Verify/Learn/Escape | 草拟 → 正式 |
| L4 | ARegistryComplete | Canonical A 表补全 | 新增 |
| L4 | CodeLayoutSync | code-layout 注册表同步 | 新增 |
| L4 | LayoutGuard | 新文件 scenario 归属 guard | 新增 |
| L5 | D7-PL-T01..T12 | 见 §6 | 草拟 |

### 4.2 In Scope

- **A 层注册表补全**：`a-registry.md` Canonical S1-S6 段补齐 v6.0.0 目标 A 清单（47 S Activities + Hardening 2 = 49 行），每项带 **现行** code location（非 historical 路径）。
- **F 层路径同步**：`f-registry.md` Current Path Correction 扩展为完整 A→F 映射；historical 路径仅留在 `historical-s-mapping.md`。
- **code-layout.md 终态化**：更新 D7 §4.2（移除已删除 shim；登记 `plan/`、`orchtypes/`、`hardening/`、`interfaces/` 归属）；version bump。
- **可选小步物理收敛**（分 PR，每 PR ≤400 行）：
  - PR-A（PR-3）：`plan/` S5 doc-only 双登记（design Q1 选 B：0 物理改动，0 importer 改）
  - PR-B（PR-4）：`orchtypes/` 登记为 Cross-S kernel（仅 doc，0 物理改动，no Go package alias, no re-export shim）
  - PR-C：S6 activity 文件名/子目录注释与 a-registry 1:1（**不强制**合并目录）
- **Layout guard 测试**：新增 `internal/layers/orchestration/layout/{types,guard,guard_test,doc}.go`（新 `layout/` 子包，4 个 .go 文件 = 0 业务代码）：
  - 禁止在 `orchestration/` 根下新增无登记 slug 目录
  - 禁止 resurrect `coordinator/`、`hubspoke/`、`observe/`、`fastpath.go`
- **OpenSpec 完整 change 包**：proposal / design / tasks / delta spec / acceptance。

### 4.3 Out of Scope

- 新建强制 `mupsgov/` 目录并整体 git mv S6 代码
- 历史 T / A / F ID 全量重编号
- WaveScheduler、D4 delegate、D2 context engine 行为变更
- 174 个 Gherkin Scenario 正文回迁 spec.md
- 跨域（D1/D2/D3/D4）物理布局

## 5. 业务目标

| ID | 目标 | 可验证承诺 |
|----|------|------------|
| D7-PL-G1 | Canonical A 与代码 1:1 | a-registry 每个 S1-S6 Canonical A 行有 ✅ + 可解析的 file path |
| D7-PL-G2 | 路径漂移可检测 | layout guard CI 失败即阻断新漂移 |
| D7-PL-G3 | code-layout 与仓库一致 | §4.2 无 ghost 目录；`plan/`/`orchtypes/` 有明确归属 |
| D7-PL-G4 | 小步可交付 | ≥3 个独立 PR，每 PR 可单独回滚，全量测试绿 |

## 6. Demand 级验收标准（L5 草案）

| T ID | 优先级 | Given-When-Then 摘要 |
|------|--------|----------------------|
| **D7-PL-T01** | P0 | change 包齐全（demand/proposal/design/tasks/delta spec） |
| **D7-PL-T02** | P0 | a-registry Canonical 段 S1-S6 A 行数 ≥ ValueFlow 表承诺（47 S + Hardening 2 = 49） |
| **D7-PL-T03** | P0 | 每个 Canonical A 的 Code Location 指向现存 `.go` 文件 |
| **D7-PL-T04** | P1 | f-registry 无 `observe/`、`execute/`、`verify/`、`learn/` 作为 **current** path |
| **D7-PL-T05** | P1 | code-layout.md D7 §4.2 与 `ls orchestration/` 一致（无 ghost shim） |
| **D7-PL-T06** | P1 | layout guard：禁止 resurrect 退役目录/文件 |
| **D7-PL-T07** | P1 | `plan/` 归属 S5 在 code-layout + a-registry 双登记 |
| **D7-PL-T08** | P1 | `orchtypes/` 归属登记（Cross-S kernel，no Go shim） |
| **D7-PL-T09** | P2 | S6 overlay 5+2+1 paths (5 S6 overlay + 2 Cross-S + 1 cross-cutting) 在 design.md 有 activity→path 矩阵 |
| **D7-PL-T10** | P2 | hardening/ 横切 A 与 S6-A14 / Hardening-A01/A02 映射清晰 |
| **D7-PL-T11** | P1 | `go test ./internal/layers/orchestration/...` 全绿 |
| **D7-PL-T12** | P1 | acceptance-report 含领域文档同步清单 |

## 7. 建议落地分期（供 S3 tasks.md 细化）

```text
Phase P0  OpenSpec demand/proposal/design/tasks     ← 本文件
Phase P1  A-registry + f-registry 补全（纯文档 PR）
Phase P2  code-layout.md 终态 + layout guard 测试
Phase P3  可选物理收敛 PR-A/B/C（每 PR 独立验收）
Phase P4  acceptance + archive
```

### 建议 PR 拆分

| PR | 范围 | 预估 |
|----|------|------|
| PR-1 | A/F registry 补全 + code-layout 文档（纯 markdown） | ~400 行 |
| PR-2 | layout guard 测试（`internal/layers/orchestration/layout/` 新子包） | ~480 行（4 个 test-only .go 文件） |
| PR-3 | `plan/` S5 doc-only 双登记（design Q1 选 B：0 物理改动） | ~30 行 |
| PR-4 | `orchtypes/` Cross-S kernel 登记（doc-only cross-reference，no Go shim） | ~30 行 |

## 8. 风险与约束

| 风险 | 缓解 |
|------|------|
| git mv 导致 import 大面积变更 | 单 PR 单包；纯 doc-only（无 shim / 无 alias） |
| A 补全与实际代码不符 | T03 用测试扫描 file path 存在性 |
| 与 historical T 路径不一致 | T ID 不改；仅更新 current registry，historical 留 mapping  doc |

## 9. 参考文档

| 文档 | 用途 |
|------|------|
| `openspec/specs/d7-orchestration/spec.md` v4.22.0 | Current S1-S6 SoT |
| `openspec/specs/d7-orchestration/historical-s-mapping.md` | Former S7-S21 → current A/F |
| `openspec/specs/d7-orchestration/a-registry.md` §ValueFlow | 49 A 目标清单 |
| `openspec/specs/architecture/code-layout.md` §4.2 | D7 scenario 路径注册表 |
| `openspec/archive/2026-07-01-devrix-d7-s-layer-normalization/design.md` §2 | Target S/A mapping |
