# Proposal: D7 物理布局对齐 — A 层补全 + S1-S6 代码路径收敛

**Change ID:** `devrix-d7-physical-layout-alignment`
**Demand ID:** DM-20260701-004
**Status:** S2_Proposal
**Created:** 2026-07-01
**Parent Demand:** `demand.md`

---

## 1. Background

DM-20260701-002/003 已完成 D7 **规格层** canonical S1-S6 归一化、S7+ 历史正文迁出、S3 边界澄清，并落地 StrategicPlanReject / child-stats uncertainty 两个小闭环。两个 change 的共同 Out of Scope 是「大规模物理目录迁移」。

但规格归一化后留下 6 个落地偏差，影响 DSAFT 注册表的真实性与可机器校验性：

- `a-registry.md` Canonical 段仅登记部分 A（21 行），ValueFlow 段列 47 A + Hardening 2（承诺 49 A）
- `f-registry.md` Canonical F 段仅展开 D7-S1-A01..A04 + D7-S2-A02 + D7-S5-A01..A03 + D7-S6-A14 共 8 段，其他 S 段缺失
- `code-layout.md §4.2` 仍列 `coordinator/`、`hubspoke/` legacy shim（实际目录已 `git rm`）
- `plan/` 与 `decisionplanning/` 并存，归属未在 code-layout 终态化
- `orchtypes/` 未登记 S 归属
- 无 layout guard，新文件可继续落入无 scenario 语义的目录

## 2. Problem Statement

| ID | 现象 | 根因 | 严重度 |
|----|------|------|--------|
| D7-PL-01 | `a-registry.md` Canonical 段仅 21 A 行，ValueFlow 段列 47 A + Hardening 2 | DM-002 聚焦 S 层归一化，A 层补全未做 | P0 |
| D7-PL-02 | `f-registry.md` 仅 8 段 Canonical F，D7-S2/3/4/6 大段缺失 | DM-002 仅写 Current Path Correction 概览，未到 A→F 1:1 展开 | P0 |
| D7-PL-03 | `code-layout.md §4.2` 仍列 `coordinator/`、`hubspoke/` legacy shim | 仓库已 `git rm`（DM-20260619-005/012），注册表未回写 | P1 |
| D7-PL-04 | `plan/` 在仓库存在但 code-layout 未登记 | DM-002 治理 overlay 未定义 activity 子路径 | P1 |
| D7-PL-05 | `orchtypes/` 30 文件（17 prod + 13 test）无 S 归属 | 跨 S 治理包未在 spec/registry 显式声明 | P1 |
| D7-PL-06 | 无 layout guard，新文件可能落入无 scenario 语义的目录 | DM-002 仅有 spec/registry guard，缺 code-layout 测试 | P1 |
| D7-PL-07 | historical-s-mapping.md / historical T 仍引用 `orchestration/observe/`、`execute/`、`verify/`、`learn/` 路径 | 历史 MUPS 节点包已合并到 mups/{observe,plan,execute,verify,learn}/ 或 orchtypes/，registry 未回写 current path | P1 |

## 3. Proposed Solution

按 **DM-002 governance overlay 决策**（已采纳）：S6 **不强制**单独 scenario 目录迁移；本 change 以 **A/F 注册表补全 + code-layout 终态化 + layout guard** 为主，避免大规模 git mv 引入行为回归。

### 3.1 阶段拆解

| Phase | 内容 | 风险 | 可独立回滚 |
|-------|------|------|-----------|
| **PR-1** | `a-registry.md` Canonical 段补全 S1-S6（47 S + Hardening 2 = 49 行）+ `f-registry.md` Current Path Correction 扩展为完整 A→F 映射 + `code-layout.md §4.2` 终态化（去 ghost shim + 登记 plan/orchtypes/hardening/interfaces 归属） | 纯文档 PR，0 Go 代码改动 | ✅ |
| **PR-2** | `internal/layers/orchestration/layout/{types,guard,guard_test,doc}.go`（新 `layout/` 子包，4 个 test-only .go 文件）— 禁止 resurrect 退役目录/文件 + 禁止 orchestration/ 根下新增无登记 slug 目录 | 仅测试，0 业务代码 | ✅ |
| **PR-3** | `plan/` S5 doc-only 双登记（design Q1 选 B：0 物理改动，0 importer 改） | 极低（纯文档） | ✅ |
| **PR-4** | `orchtypes/` Cross-S kernel 登记（doc-only cross-reference，no Go package alias，no re-export shim） | 极低（纯文档） | ✅ |

### 3.2 关键约束

- **不重编号** 历史 A/F/T ID；新增 `Current ID` 列指向 v6.0.0 目标编号，Legacy ID 列保留追溯
- **不动** 历史 T / A / F 路径的字符串值；仅更新 current registry 与 historical mapping
- **不迁移** WaveScheduler、D4 delegate、D2 context engine 行为
- **不回迁** 174 个 Gherkin Scenario 正文（lite-mode 范围外）
- **不跨域** 改 D1/D2/D3/D4 物理布局
- **不新增** `mupsgov/` 单目录（沿用 governance overlay）

## 4. Success Metrics

### 4.1 主目标

| ID | 标准 | 验证方式 |
|----|------|----------|
| AC1 | `a-registry.md` Canonical S1-S6 A 行数 ≥ ValueFlow 表承诺（47+Hardening 2 = 49） | `grep -c "^### D7-S" a-registry.md` ≥ 6 段；每段行数对齐 ValueFlow |
| AC2 | 每个 Canonical A 的 Code Location 指向现存 `.go` 文件 | 新增 `internal/layers/orchestration/layout/guard_test.go::TestACanonicalLocationsExist` 扫描 + file path 存在性 |
| AC3 | `f-registry.md` Canonical S1-S6 F 段 ≥ 6 段（S1-S6 全覆盖） | `grep -c "^### D7-S" f-registry.md` ≥ 6 |
| AC4 | `f-registry.md` 无 `observe/`、`execute/`、`verify/`、`learn/` 作为 current path | `grep -E "orchestration/(observe\|execute\|verify\|learn)\""` 仅命中 historical-s-mapping.md |
| AC5 | `code-layout.md §4.2` 与 `ls orchestration/` 一致（无 ghost shim） | `grep -E "coordinator\|hubspoke" code-layout.md` 0 命中 |
| AC6 | Layout guard：禁止 resurrect 退役目录/文件 | `internal/layers/orchestration/layout/guard_test.go::TestNoResurrectRetiredDirs` PASS |
| AC7 | Layout guard：禁止 orchestration/ 根下新增无登记 slug 目录 | `internal/layers/orchestration/layout/guard_test.go::TestOrphanDirs` PASS |
| AC8 | `plan/` 归属 S5 在 code-layout + a-registry 双登记 | 双 doc 引用（doc-only 路径） |
| AC9 | `orchtypes/` 归属登记（Cross-S kernel） | design §④ 决定 |

### 4.2 范围外

| ID | 不做 | 理由 |
|----|------|------|
| OOS-1 | 新建强制 `mupsgov/` 目录并整体 git mv S6 代码 | DM-002 governance overlay 决策 |
| OOS-2 | 历史 T / A / F ID 全量重编号 | 跨 30+ 历史 change，会破坏所有 git blame / 文档链接 |
| OOS-3 | WaveScheduler、D4 delegate、D2 context engine 行为变更 | 各自领域，独立 change 推进 |
| OOS-4 | 174 个 Gherkin Scenario 正文回迁 spec.md | spec.md lite-mode 严格 ≤ 200 行 |
| OOS-5 | 跨域（D1/D2/D3/D4）物理布局 | 域自治 |
| OOS-6 | 引入新外部依赖 | Pure Go 测试 |
| OOS-7 | `plan/` 物理迁移到 `decisionplanning/` | 43 importer 跨包影响面大；Q1 选 B（doc-only 双登记）|
| OOS-8 | `coordinator/` `hubspoke/` `turn/` legacy shim 复活 | 已 `git rm`（DM-20260619-005/012 + DM-20260626-004），layout guard 持续拦截 |
| OOS-9 | S7+ Historical S F/A 段回迁 | DM-003 决策迁出；仅 historical-s-mapping.md 留追溯 |

### 4.3 AC ↔ PR 映射

| AC | 验证主体 | 归属 PR |
|----|---------|---------|
| AC1 | a-registry.md Canonical S1-S6 A 补全 | **PR-1** |
| AC2 | layout guard::TestACanonicalLocationsExist | **PR-2** |
| AC3 | f-registry.md Canonical S1-S6 F 补全 | **PR-1** |
| AC4 | f-registry.md 无 retired path（仅 historical-s-mapping.md 命中） | **PR-1** |
| AC5 | code-layout.md §4.2 与 `ls orchestration/` 一致 | **PR-1** |
| AC6 | layout guard::TestNoResurrectRetiredDirs | **PR-2** |
| AC7 | layout guard::TestOrphanDirs | **PR-2** |
| AC8 | `plan/` 双登记（code-layout + a-registry） | **PR-3**（doc-only，无 Go 代码）|
| AC9 | `orchtypes/` Cross-S 登记（a/f/code-layout/d7-domain） | **PR-4**（doc-only，无 Go 代码）|

### 4.4 Rollback Plan

每个 PR 独立 squash auto-merge，回滚即 `git revert <commit>`：
- PR-1 失败：spec/registry 仍保持 v5.3.0（增量补全是纯文档，0 行为变更）
- PR-2 失败：layout guard 仅测试代码，业务零影响；可 `git revert` 后业务代码保留
- PR-3 失败：plan/ 双登记是 doc-only，回退 spec/registry 行
- PR-4 失败：orchtypes/ Cross-S 登记是 doc-only，回退 spec/registry 行

## 5. Implementation Plan

```text
P0  OpenSpec demand / proposal / design / tasks / delta spec         ← 本 change
P1  PR-1 A/F registry 补全 + code-layout 终态化（纯文档 PR）         ← PR-1
P2  PR-2 layout guard 测试                                              ← PR-2
P3  PR-3 plan/ S5 doc-only 双登记（design Q1 选 B：0 物理改动）         ← PR-3
P4  PR-4 orchtypes/ Cross-S kernel 登记（doc-only cross-reference）       ← PR-4
P5  acceptance + archive
```

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| git mv 导致 import 大面积变更 | High | 单 PR 单包；纯 doc-only（无 shim / 无 alias） |
| A 补全与实际代码不符 | Med | AC2 测试扫描 file path 存在性 |
| 与 historical T 路径不一致 | Med | T ID 不改；仅更新 current registry，historical 留 mapping doc |
| Layout guard 误伤（误报孤儿目录） | Low | allow-list 已知共存子目录（interfaces/hardening/orchtypes），design §④ 明列 |
| PR-3 plan/ 双登记覆盖不全 | Low | doc-only revert + 重新登记 a-registry/code-layout；0 物理改动 |

## 7. Out of Scope

详见 §4.2。

## 8. Dependencies

| 依赖 | 用途 | 状态 |
|------|------|------|
| DM-20260701-002 | S 层归一化（spec/registry） | ACCEPTED ✅ |
| DM-20260701-003 | Historical S cleanup | ACCEPTED ✅ |
| DM-20260626-001 | 6 S 精简（v6.0.0 A remap） | S7_Archived ✅ |
| DM-20260619-005 | D7 v2.0 structure | S7_Archived ✅（已 git rm coordinator/hubspoke） |
| DM-20260626-002/004/005 | 包路径迁移 / turn 合并 / verify promote | S7_Archived ✅ |
| `internal/layers/orchestration/` 物理目录 | layout guard 测试扫描源 | ✅ |
| `openspec/specs/d7-orchestration/a-registry.md` | 当前规格 | v5.3.0 |
| `openspec/specs/d7-orchestration/f-registry.md` | 当前规格 | v5.3.0 |
| `openspec/specs/architecture/code-layout.md` | 当前规格 | v1.12.0 |
| `openspec/specs/d7-orchestration/t-registry.md` | 当前规格 | v4.21.0 |
| `openspec/t-registry.md` | 当前规格 | v5.11.0 |

## 9. Open Questions (待 S3 design 决定)

| ID | 问题 | 候选方案 |
|----|------|----------|
| Q1 | `plan/` S5 归属登记 | A. git mv → `decisionplanning/plan/` (43 importer 替换)<br>B. 保留 plan/ 包，doc-only 双登记为 D7-S5 物理子目录（与 decisionplanning/ 并列）<br>C. 加 `package decisionplanning` shim（与 `package plan` 同目录冲突，不可行）<br>推荐: B（0 物理改动 + 0 行为变更 + 仅 spec/registry 登记） |
| Q2 | `orchtypes/` 归属 | A. 登记为 Cross-S kernel（治理包，跨 S）<br>B. 单独 `orchestration/kernel/` 子目录<br>推荐: A（治理包跨 S 更准确，0 物理改动） |
| Q3 | `mups/execute/` + `mups/learn/` 是否在 design 阶段补 activity→path 矩阵 | A. 仅 a-registry 行内 Code Location<br>B. design.md §④ 加一张完整矩阵<br>推荐: B（避免 PR 返工） |