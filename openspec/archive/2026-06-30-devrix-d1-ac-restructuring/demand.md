# Demand: devrix-d1-ac-restructuring (DM-20260629-005)

**Demand ID:** DM-20260629-005
**Status:** S1_Demand
**Priority:** P0（AC 重构，治本 D1 验收质量 outlier）
**Created:** 2026-06-29
**Change ID:** devrix-d1-ac-restructuring
**Triggered By:** D1-D7 AC 全面 Review（2026-06-29，会话）— D1 5.5/10 outlier 治本
**Related:**
- `devrix-d7-dsaft-restructuring` (DM-20260629-001) — D7 6 子 Change 模板 (S7_Archived 2026-06-29)
- `devrix-d2-dsaft-restructuring` (DM-20260629-002) — D2 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d3-dsaft-restructuring` (DM-20260629-003) — D3 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d4-dsaft-restructuring` (DM-20260629-004) — D4 6 子 Change 联动 (S7_Archived 2026-06-29)
- `devrix-d1-dsaft-refactor` (DM-20260628-003) — D1 Gateway 拆分 + contracts DTO + lint-d1-imports CI (S7_Archived 2026-06-28)
- `devrix-d1-sa-refine` (DM-20260614-006) — D1 S/A 重切 v1.0 → v2.0 (S7_Archived 2026-06-15)
- `docs/methodology/dsaft-methodology.md` v4.0.0 — 6 原则
- `docs/methodology/dsaft-refactoring-playbook.md` v1.0.0 — 4 轴 / 6 阶段

---

## §1 背景

D1 v4.1.1（2026-06-16 spec.md）已达"6 价值流 S13-S18 / 16 Canonical A / 18 F / 56 T（P0=26） / 10 canonical span ops" 的稳定 SoT 状态。但 2026-06-29 全域 AC Review 暴露 **D1 是 7 域中唯一 AC 设计 outlier**（5.5/10），缺 5 类关键治理。对齐 D7 v2.6.0 / D2 v9.0.0 / D3 v1.6.0 / D4 v2.2.0 6 子 Change 模板，启动 D1 AC 深度重构。

### 1.1 Gherkin AC 形式不一致（P0, D1 唯一缩写 bullet）

D1 `spec.md` v4.1.1 共 24 条 AC，**全部使用 缩写 bullet**（`- **入站持久化（happy）：** GIVEN ...`）；D2/D3/D4/D7 一致使用 `### Feature:` + `#### Scenario:` + `**Given**/**When**/**Then**/**And**` Gherkin 块（每个 S 8-15 个 Scenario）。

**问题**：
- 缩写 bullet 不易被测试 runner 解析（无法直接生成 test stub）
- AC ↔ T 映射靠人工 `<!-- T: ... -->` 注释，无强约束
- 评审/验收需要逐行解析

**目标**：24 缩写 bullet → 90 个 `#### Scenario:` Gherkin 块（每 S 8-15 个），覆盖 happy / sad / boundary / concurrent / timeout。

### 1.2 Span Evidence 列缺失 (P0, D1 唯一未对齐)

D1 `t-registry.md` v3.2.0 56 T 行**无 Span Evidence 列**；D2/D3/D4/D7 已加。

**问题**：
- 10 canonical span ops（`d1.capture.persist` / `d1.dispatch.route` / `d1.signal.{thinking,task,conclusion,chain_integrity,task.work_proof,user_feedback}` / `eventbus.{publish_critical,drain}` / `adapter.feishu.encode`）**无 T 绑定证据链**
- 评审/验收无 Span ↔ T 引用依据

**目标**：56 T 行加 Span Evidence 列；44 T 映射到 10 span op / Span Event；12 显式 `—`（注入模式 6 + 启动期 3 + 编译期 3）；覆盖率 44/(56-12) = 100% effective（raw 44/56 ≈ 78.6% informational）；`scripts/d1-span-coverage.sh` 守门 ≥80% PASS。

### 1.3 d7-boundary.md 缺失 (P0, D1 唯一未对齐)

D1 `d1-domain.md` v1.0.0 **无 d7-boundary.md**；D2 v2.0.0 / D3 v1.1.0 / D4 v2.x / D7 v2.x 都有。

**问题**：
- D1 ↔ D7 跨域边界无独立 SoT（`routeLegacyD2` RETIRED、`ensureSessionLeader` 委托 `bootstrap/sessionagents`、D1 capture 禁止 import orchestration/ 已通过 lint-d1-imports.sh 守门 — 但 spec 无登记）
- `boundary-debt:d1-to-d7-orchestration-entry-v1.0` 等 3 项无治理常量

**目标**：新建 `d1-domain.md §Boundary Debt Decisions` + `d7-boundary.md` (NEW) + `communication/orchtypes/boundary_decision.go` (NEW) + 3 单元测试 (Exist + VersionFormat + Unique)。

### 1.4 spec.md god doc (P1, 176 行可拆 2)

D1 `spec.md` v4.1.1 176 行**单文件包含 5 段**：Overview / Architecture / Package Map / Cross-Domain / Requirements / Gherkin AC；D2 1622 行 / D3 1060 行（已按 S 价值流拆分）vs D1 176 行单段堆叠。

**问题**：
- 评审需要滚动全文
- 与 terminal-state-guide.md / observability-guide.md 职责重叠

**目标**：spec.md 176 → 90 行（保留 Gherkin AC + 跨域依赖 + Key Patterns），把"Architecture 包结构 + Package Map + 价值流流图"下沉到 `architecture/d1-flow-architecture.md`（NEW，与 D2/D3/D4 `*-flow-architecture.md` 命名对齐）。

### 1.5 observability-guide.md 职责混 (P1, 230 行可拆 2)

D1 `observability-guide.md` v1.0.0 230 行**混 6 段**：Span↔T / Trace 树 / EventBus 必达 / Drain 注入 / T 验收摘要 / Runbook；D2 281 行 / D3 248 行（已分 observability + trace + runbook）。

**问题**：
- Span↔T 矩阵 18 行（已存在 observability-guide.md §1，**与 t-registry Span Evidence 列重复**）
- §7 已知缺口（5 项）需迁到 t-registry §T-Without-Span Tracker

**目标**：observability-guide.md 230 → 100 行（保留 Trace 树 + EventBus 必达 + Runbook）；§1 Span↔T 矩阵删除（Span Evidence 列已含）；§5 T 验收摘要精简（指向 t-registry §Statistics）；§7 已知缺口迁 t-registry §T-Without-Span Tracker。

### 1.6 ValueFlow Alias 缺失 (P1)

D1 `d1-domain.md` §North Star 表**缺 ValueFlow Alias 列**，对齐 D2 v9.0.0 / D3 v1.6.0 / D4 v2.2.0 §North Star 模式。

**目标**：6 canonical S + 0 横切 = 6 ValueFlow Alias：
- S13 CaptureUserIntent → `D1_Capture_User_Intent`
- S14 PresentThinking → `D1_Present_Thinking`
- S15 PresentTaskProgress → `D1_Present_Task_Progress`
- S16 DeliverConclusion → `D1_Deliver_Conclusion`
- S17 ConnectChannel → `D1_Connect_Channel`
- S18 GuaranteeDelivery → `D1_Guarantee_Delivery`

### 1.7 Historical S 残留 (P2)

D1-S1~S12 已退役（DM-20260614-006 Phase 3）但**仍写进 3 spec 文件**（`a-registry.md` Legacy Module Index / `f-registry.md` §Legacy / `t-registry.md` §Legacy T），代码已 0 caller。需下沉到 `openspec/archive/2026-06-14-devrix-d1-sa-refine/legacy-s1-s12.md`（archive dir 已存在可复用）。

---

## §2 治理目标

D1 域从 v4.1.1 / v1.0.0-d1-domain 演进到 v5.0.0，对齐 D7 v2.6.0 / D2 v9.0.0 / D3 v1.6.0 / D4 v2.2.0 模板：

```
v4.2.0 orchtypes 包建立 (PR-1)
  + internal/layers/communication/orchtypes/ 新建
  + boundary_decision.go 前置 3 治理常量
v4.3.0 god-doc-split pt1 (PR-2)
  + spec.md 176 → 90 行 (保留 Gherkin AC)
  + architecture/d1-flow-architecture.md (NEW)
v4.4.0 god-doc-split pt2 (PR-3)
  + observability-guide.md 230 → 100 行 (保留 Trace + Runbook)
  + §1 Span↔T 矩阵删除 (Span Evidence 列已含)
v4.5.0 registry-sync (PR-4)
  + t-registry 56 T 行加 Span Evidence 列
  + Historical S 沉 archive (legacy-s1-s12.md)
  + scripts/d1-span-coverage.sh CI 守门
v4.6.0 value-flow-rename (PR-5)
  + 6 ValueFlow Alias (D1_* 前缀)
  + a/f/t/layer-delta 同步
v4.7.0 gherkin-restructuring (PR-6)
  + 24 缩写 bullet → 90 #### Scenario: 块
  + 覆盖 happy / sad / boundary / concurrent / timeout
v4.8.0 boundary-decision (PR-7)
  + d1-domain.md §Boundary Debt Decisions
  + d7-boundary.md (NEW)
  + 3 单元测试
v5.0.0 S7_Archive (PR-8)
  + 6 artifacts 复制到 archive/2026-06-30-devrix-d1-ac-restructuring/
  + verify-archive.sh 12/12 PASS
  + d1-domain v1.0.0 → v1.1.0
```

---

## §3 验收目标

| 维度 | 现状 (v4.1.1) | 目标 (v5.0.0) | 验证 |
|------|--------------|--------------|------|
| AC 形式 | 24 缩写 bullet | 90 `#### Scenario:` Gherkin 块 | `grep "^#### Scenario:" spec.md | wc -l` ≥ 90 |
| Span Evidence 列 | 0 行 | 56 行全标注 | `grep "Span Evidence" t-registry.md` ≥ 56 |
| Span Coverage | 0% | ≥80% effective | `scripts/d1-span-coverage.sh` PASS |
| d7-boundary.md | 不存在 | 存在 + 3 boundary debt | `test -f d7-boundary.md` |
| spec.md LOC | 176 | ≤ 100 | `wc -l spec.md` |
| observability-guide.md LOC | 230 | ≤ 120 | `wc -l observability-guide.md` |
| ValueFlow Alias | 0 | 6 | `grep "D1_*" d1-domain.md` ≥ 6 |
| Historical S 沉 archive | 0 | 1 (legacy-s1-s12.md) | `test -f archive/.../legacy-s1-s12.md` |
| orchtypes/boundary_decision.go | 不存在 | 存在 + 3 单测 PASS | `go test ./internal/layers/communication/orchtypes/...` |
| d1-domain Version | 1.0.0 | 1.1.0 | git diff |
| d1-domain §Boundary Debt Decisions | 不存在 | 3 row | grep |
| d1-domain v5.0.0 | 0 (无 §ValueFlow Alias) | 6 alias 块 | grep |
| verify-archive.sh | n/a | 12/12 PASS | ./scripts/verify-archive.sh devrix-d1-ac-restructuring |

---

## §4 风险与缓解

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Gherkin 块改造 90 个 Scenario 破坏测试注释引用 | Mid | Mid | 每 Scenario 末尾加 `<!-- T: D1-S{N}-A{NN}-T{XX} -->` 注释，保留 t-registry 映射；`grep` 守门 |
| Span Evidence 列新增破坏 t-registry 列宽 / 编辑体验 | Low | Low | 表格化，列宽对齐；按 D3 v3.4.0 模板复刻 |
| d7-boundary.md 新建涉及 D7 跨域同步 | Mid | Mid | 抄 D3 `d7-boundary.md v1.1.0` 模板；D1 侧只登记 D1 拥有的 boundary decision |
| orchtypes/boundary_decision.go 重复 D4 orchtypes 包 | Low | Low | 命名空间独立 `communication/orchtypes`；consumer 端按需引用 |
| Historical S 沉 archive 丢失追溯 | Low | Mid | 复用 `openspec/archive/2026-06-14-devrix-d1-sa-refine/` dir；legacy-s1-s12.md 含 frozen index + 迁移路径表 |
| spec.md 拆 2 文件破坏旧链接 | Mid | Mid | spec.md 头部加 "See also" 引用 d1-flow-architecture.md；grep 0 死链守门 |
| observability-guide.md 拆破坏 §1 Span↔T 引用 | Low | Mid | 同步更新 t-registry.md §T-Without-Span Tracker；删 §1 时同步删除 t-registry 旧 link |
| 8 子 Change 跨 5-7 天 baseline 不稳 | Mid | Mid | 每日 1 PR squash auto-merge；CI gate 即时验证 |

---

## §5 与 D7/D2/D3/D4 模板对齐

| 维度 | D7 (DM-001) | D2 (DM-002) | D3 (DM-003) | D4 (DM-004) | **D1 (本 DM)** |
|------|------------|------------|------------|------------|---------------|
| LOC | ~15000+ | ~9000 | 4826 | 4108 | **~5500** (spec.md 176 + observability 230 + t-registry 155 + a-registry 165 + f-registry 211 + span-registry 73 + d1-domain 90) |
| PR | 11 | 8 | 8 | 8 | **8** |
| T | 55 | 44 | 40 | 34 | **~34** |
| Span Coverage | 94% | 88% | 100% | 100% | **≥80%** |
| 死代码 | ~775 LOC | ~1298 LOC | 0 (审计) | 15 exported | **0** (D1 spec/无死) |
| God fn / God doc | 4 个拆 4 文件 | 5 个拆 5 文件 | 4 个拆 4 文件 | 2 god fn 拆 4 文件 | **2 god doc 拆 4 文件** (spec.md 176 + observability 230) |
| Boundary Debt | 3 RESOLVED | 2 (1+1 待定) | 4 RESOLVED | 3 RESOLVED | **3 RESOLVED** |
| **D1 独有债务** | RollupReport envelope + WorkTree 上行反馈 | DM-018 slice-c + 跨域 fixture | runtime span op 字面量稳定 | 18 F 路径漂移 + 7 EngineEvent 字面量 | **24 缩写 bullet AC + Span Evidence 列缺失 + d7-boundary.md 缺失** |

---

**END of Demand**
