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

### Phase A — 规格终态（docs + registry 代码锚点）

| AC | 描述 | 优先级 | 博弈论来源 |
|----|------|--------|-----------|
| AC-A1 | 新建 `d5-domain.md`：Tl;DR + North Star + 4 承诺 + 完备性边界 + 博弈论玩家表（含 SRE/on-call）+ 时间属性×承诺强度交叉矩阵 + S23 子承诺（按时间分组）+ S25 触发条件 + 子承诺举证责任 + Terminal 冻结声明 + 各域 Bridge 删除时间线 + Out of Scope + 物理路径表 + 文档阅读优先级标注 | P0 | G1,G7,G8,G12,G13,G16,G20 |
| AC-A2 | `spec.md` v3.0：DSAFT 主表 S21–S24；D7 Turn 主路径；query.loop 仅 RETIRED 节 | P0 | — |
| AC-A3 | 新建 `observability-guide.md`：D7 Turn Trace 树 + Span↔T P0 矩阵 + P0 Runbook（含 on-call 排障动线 + SRE 验收清单）+ Coverage 多维指标（ratio/completeness/link_integrity/recency）+ WARN metric 聚合说明 + D5 成功指标双轨声明（过程指标 + 验证指标） | P0 | G15,G17 |
| AC-A4 | 新建 `terminal-state-guide.md` + `dsaft-architecture.md` Stub | P1 | — |
| AC-A5 | 新建 `d5-boundary.md`；更新 `cross-domain-boundaries.md` D5 段（含 D5→D6 证据移交规则） | P0 | G9 |
| AC-A6 | `a-registry` v4.0 + `f-registry` v3.0 + `design.md` v3.0 同步（design.md §5 含 S23 硬边界 + S25 触发条件 + 子承诺举证责任；§10 含 Phase B2 拆步不留 shim + Phase A 代码锚点 + Phase B 启动对账条件 + 跨 Change 级联标注；§12 含 legacy_harness 退役计划） | P0 | G1,G6,G10,G11,G13,G16 |
| AC-A7 | `code-layout.md` §4.6 diagnose 子目录完整；`grep query.loop` 仅 Legacy/RETIRED | P0 | — |
| AC-A8 | Phase A 包含 ≥1 个代码锚点（a-registry v4.0 Code Location 更新 或 t-registry canonical 列校正），不可推迟到 Phase B | P0 | G10 |
| AC-A9 | 所有新建 spec 文档包含阅读优先级标注（MUST/SHOULD/REFERENCE） | P1 | — |

### Phase B — 代码清债

| AC | 描述 | 优先级 | 博弈论来源 |
|----|------|--------|-----------|
| AC-B1 | Phase B2a：`bridge.go` import 改 `instrument/*` 直连，`go build ./...` 通过 | P0 | G11 |
| AC-B2 | Phase B2b：删除 9 个 bridge 包（不留 shim）；全仓 `grep` 旧 bridge 路径 = 0 命中（除 archive/docs）；`go test ./... -race` 全绿 | P0 | G11 |
| AC-B3 | CI 包含 bridge 防回归规则（grep 9 个旧路径 = 0 命中，否则 CI 拒绝） | P0 | G11 |
| AC-B4 | `genai_tokens.go` → `instrument/metrics/`；`llm_log.go` → `diagnose/incident/`；`slog_bridge.go` 调 `instrument/logger` 安装桥 | P1 | — |
| AC-B5 | `t-registry` canonical_s 校正（A08→S21, A06→S0）；canonical_a 校正（Doctor T→A10）；**3** PLANNED T 闭合（D5-S21-A05-T01, D5-S21-A05-T02, D5-S23-A06-T02） | P1 | — |
| AC-B6 | 41/41 T IMPLEMENTED（PLANNED 全部闭合后），每条 P0 T 有明确 Span 证据或 sad path 说明 | P0 | — |
| AC-B7 | `go test ./... -race` + layer-lint + obs integration 全绿 | P0 | — |
| AC-B8 | `legacy_harness` metric help text 标 DEPRECATED；退役计划写入 `design.md` §12（v2.1 DEPRECATED → v2.3 自爆机制） | P1 | G6 |

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
| L5 Test | canonical_s/a 列；3 PLANNED 闭合 | `t-registry.md` v3.2 |

## 11. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 文档与代码短期不一致 | 误导排障 | Phase A 先合并 specs；grep AC-A7 |
| bridge 删除回归 | 编译失败 | grep + 全量 test（S4） |
| 博弈论对焦改决议 | 设计返工 | Decision 在 design.md；gaming-analysis OQ 表 |
| S23 继续膨胀 | 万能 S | 子承诺上限；OQ-1 对焦 |

## 12. S3 设计文档清单

见 `README.md`。博弈论讨论入口：**`gaming-analysis.md`** + **`d5-requirements-clarifications.md` §6**。
