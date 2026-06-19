---
demand-id: DM-20260619-006
title: D5 v2.1 终态重构 — 规格对齐 + S23 语义闭合 + Bridge 清债
source: DSAFT Refactoring Playbook §4–§6 + D5 领域对焦（2026-06-19）
priority: P0
status: S3_Design
dsaft_domain: observability
created: 2026-06-19
---

# D5 Observability v2.1 终态重构

## 1. 背景

D5 已完成 v1.0 Registry（DM-20260615-001：S21–S24 价值流）与 v2.0 Structure（DM-20260615-003：物理路径迁移 + 11 bridge）。功能上 41 T 中 39 IMPLEMENTED，诊断工具链（Tracker / Doctor / FaultInject / DebugFilter）已落地。

但 **规格锚点与物理锚点仍未闭合终态**，对标 D7 DM-20260619-005 完成后的文档栈，D5 存在系统性缺口：

| 维度 | D7 终态（参考） | D5 现状 |
|------|----------------|---------|
| 领域 SoT | `d7-domain.md` | ❌ 无 `d5-domain.md` |
| spec.md 主叙事 | Canonical S1–S5 | 仍 D5-S1–S9 Legacy |
| observability-guide | Span↔T + Runbook | ❌ |
| f-registry | 与 A 同步 | 仍 Legacy S1–S9 |
| bridge 清债 | coordinator shim 1 release | 9 bridge 仍被 Facade 内部引用 |
| 主路径 Trace | D7 Turn | spec/design 仍写 query.loop |

2026-06-16~18 诊断工具 change 在 **未扩 A/F 注册表** 的前提下新增代码，导致 S23「诊断辅助」承诺膨胀、T↔A 编号错位（如 Doctor T 挂 A03，A03 实为 GenerateDailyReport）。

## 2. Owner 决议（设计缺省 — 本 change 采用最优方案）

| # | 议题 | 决议 |
|---|------|------|
| Q1 | 重构范围 | **C** — 文档终态 + S23 语义 + bridge 删除 + 根目录归位（物理迁移已完成，不重做 v2.0） |
| Q2 | S23 是否新增 S25 | **否** — S23 内子承诺 C3a–C3e，不增 S 号（T ID 不变） |
| Q3 | DebugFilter 归属 | **S21 Instrument**（A14 FilterDebugLog） |
| Q4 | SessionBridge 归属 | **S0 Facade**（A03 TrackActiveSessions） |
| Q5 | bridge 删除时机 | **本轮**（对齐 D6 v2.0.1 已删 bridge 先例） |
| Q6 | North Star | 见 design.md §1 |

## 3. 问题陈述

| # | 问题 | 影响 |
|---|------|------|
| P1 | spec/f/design 双轨 | 新人读 spec 误判架构；OpenSpec S5 验收无单一 SoT |
| P2 | query.loop 文档幽灵 | D5 裁判域 Trace 树不可信；与 D7 Turn 主路径矛盾 |
| P3 | S23 实现先于注册表 | Tracker/Doctor/FaultInject 无 A/F；T↔A 编号冲突 |
| P4 | bridge 未收尾 | v2.0 Structure 未闭环；Facade 仍走 Deprecated 路径 |
| P5 | 根目录孤儿文件 | `llm_log.go` / `genai_tokens.go` / `slog_bridge.go` 无 scenario 锚点 |
| P6 | 跨域契约薄弱 | 无 `d5-boundary.md`；D2 TrackerSurface→D5 tracker 边界未文档化 |

## 4. 目标行为

```text
openspec/specs/d5-observability/
├── d5-domain.md              # 领域 SoT（新建）
├── spec.md                   # v3.0 Canonical S21–S24 主表
├── observability-guide.md    # Span↔T + D7 Turn Trace + P0 Runbook（新建）
├── terminal-state-guide.md   # 终态叠合 + 文档索引（新建）
├── dsaft-architecture.md     # 五层计数 Stub（新建）
├── d5-boundary.md            # 跨域契约（新建）
├── a-registry.md             # v4.0：路径同步 + S23 A07–A10 + S21-A14 + S0-A03
├── f-registry.md             # v3.0：Canonical + canonical_s 列
├── design.md                 # v3.0：D7 Turn 主路径
├── layer-delta.md            # §v2.1-Terminal
└── span-registry.md / coverage.md  # 去除 query.loop 主路径引用

internal/layers/observability/
├── observability.go / bridge.go     # import instrument/* 直连
├── instrument/                      # S21（含 metrics/genai_tokens, logger/debugfilter）
├── export/                          # S22
├── diagnose/                        # S23（coverage, incident, tracker, doctor, faultinject）
├── configure/                       # S24
└── (无 tracer/metrics/logger/exporter/coverage/... bridge 包)
```

## 5. 验收标准（AC）

### Phase A — 规格终态（docs-only）

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-A1 | 新建 `d5-domain.md`：North Star + 4 承诺 + Out of Scope + 物理路径表 | P0 |
| AC-A2 | `spec.md` v3.0：DSAFT 主表 S21–S24；query.loop 仅 RETIRED 节 | P0 |
| AC-A3 | 新建 `observability-guide.md`：D7 Turn Trace 树 + Span↔T P0 矩阵 + Runbook | P0 |
| AC-A4 | 新建 `terminal-state-guide.md` + `dsaft-architecture.md` Stub | P1 |
| AC-A5 | 新建 `d5-boundary.md`；更新 `cross-domain-boundaries.md` D5 段 | P0 |
| AC-A6 | `a-registry` v4.0 + `f-registry` v3.0 + `design.md` v3.0 同步 | P0 |
| AC-A7 | `code-layout.md` §4.6 diagnose 子目录完整；`grep query.loop` 仅 Legacy/RETIRED | P0 |

### Phase B — 代码清债

| AC | 描述 | 优先级 |
|----|------|--------|
| AC-B1 | 删除 9 个 bridge 包；`bridge.go`/`slog_bridge.go` 改 canonical import | P0 |
| AC-B2 | `genai_tokens.go` → `instrument/metrics/`；`llm_log.go` → `diagnose/incident/` | P1 |
| AC-B3 | `t-registry` canonical_s 校正（A08→S21, A06→S0）；2 PLANNED T 闭合或删 | P1 |
| AC-B4 | 41/41 T IMPLEMENTED 或 PLANNED 有明确 sad path 说明 | P0 |
| AC-B5 | `go test` + race + layer-lint + obs integration 全绿 | P0 |

## 6. 不在范围

- D2/D7 代码变更（仅边界文档更新）
- 新增可观测能力（采样策略、新 exporter 类型）
- Operation Registry 条目增减（维持 56 ops）
- D5-S 号段重编（S21–S24 冻结）

## 7. 依赖

| 依赖 | 状态 |
|------|------|
| DM-20260615-001 v1.0 Registry | ✅ ACCEPTED |
| DM-20260615-003 v2.0 物理迁移 | ✅ ACCEPTED |
| DM-20260618-010 QueryLoop REMOVED | ✅ 代码已删 |
| DM-20260616~18 诊断工具链 | ✅ IMPLEMENTED |
| DM-20260619-005 D7 v2 Structure | ✅ 参考终态模式 |

## 8. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | D7 Turn 主路径 span 已落地；D6 bridge 已删先例 |
| 硬约束 | 41 T ID 字符串不可 renumber |
| 硬约束 | 56 Operation Registry 本 change 不增删 |
| 硬约束 | D2/D7 Go 代码本轮不改 |
| 流程约束 | S3 设计文档齐全 → S3-Gate → 再 S4 |

## 9. 变更范围

### 新增（文档 — S3 已起草于 change 包）

| 类别 | 文件 |
|------|------|
| 领域 SoT | `specs/d5-domain.md` |
| 跨域 | `specs/d5-boundary.md`, `specs/cross-domain-boundaries-d5-delta.md` |
| 指南 | `specs/observability-guide.md`, `specs/terminal-state-guide.md` |
| 博弈论 | `gaming-analysis.md`, `d5-requirements-clarifications.md` |
| 注册表草案 | `specs/a-registry.md` v4.0, `specs/f-registry.md` v3.0 |
| Delta | `specs/layer-delta.md`, `specs/d5-observability_delta.md` |
| Gherkin | `specs/d5-observability/spec.md` |
| 索引 | `README.md` |

### 修改（S7 归档时写入 `openspec/specs/`）

`spec.md` v3.0 · `design.md` v3.0 · `a-registry` · `f-registry` · `t-registry` · `span-registry` · `coverage.md` · `code-layout.md` §4.6 · `cross-domain-boundaries.md`

### 修改（代码 — S4，本阶段不执行）

bridge 删除 · 根目录 git mv · import 更新

### 不变更

D2/D7 实现 · Operation 56 条 · S 号段 S21–S24

## 10. L1–L5 DSAFT 映射

| 层级 | 本需求产出 | SoT 文件（终态） |
|------|-----------|------------------|
| L1 Domain | D5 Observability 裁判域定位 | `d5-domain.md` |
| L2 Scenario | S0 + S21–S24；S23 子承诺 C3a–C3e | `spec.md` Scenarios |
| L3 Activity | +5 A（A14, A03, A07, A09, A10） | `a-registry.md` v4.0 |
| L4 Function | +诊断 F；全路径 canonical_s | `f-registry.md` v3.0 |
| L5 Test | canonical_s/a 列；2 PLANNED 闭合 | `t-registry.md` v3.2 |

## 11. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 文档与代码短期不一致 | 误导排障 | Phase A 先合并 specs；grep AC-A7 |
| bridge 删除回归 | 编译失败 | grep + 全量 test（S4） |
| 博弈论对焦改决议 | 设计返工 | Decision 在 design.md；gaming-analysis OQ 表 |
| S23 继续膨胀 | 万能 S | 子承诺上限；OQ-1 对焦 |

## 12. S3 设计文档清单

见 `README.md`。博弈论讨论入口：**`gaming-analysis.md`** + **`d5-requirements-clarifications.md` §6**。
