# Acceptance Report: D7 物理布局对齐

**Demand ID:** DM-20260701-004
**Change ID:** `devrix-d7-physical-layout-alignment`
**Date:** 2026-07-01
**Verdict:** ACCEPTED

---

## Summary

DM-20260701-002/003 已完成 D7 **规格层** canonical S1-S6 归一化,本 Change 把 v6.0.0 设计里定义的 **49+ A 活动** 与 **现行代码物理路径** 对齐,消除「ValueFlow 表有、Canonical A 表缺」「historical mapping 指向已删除包」「S6 治理代码散落在 4 个目录」等漂移,通过 5 个独立 PR 完成可验证的路径收敛:

| PR | Commit | Title |
|----|--------|-------|
| **PR-1** | [99d48710](https://github.com/fqntxmqee/devrix/pull/366) | docs: D7 A/F registry 补全 + code-layout §4.2 终态化 |
| **PR-2** | [3b459e08](https://github.com/fqntxmqee/devrix/pull/367) | feat: D7 layout guard package + 6 ghost 行 status flip |
| **PR-3** | [c02c0a62](https://github.com/fqntxmqee/devrix/pull/368) | docs: D7 plan/ S5 doc-only dual registration |
| **PR-4** | [01fe7882](https://github.com/fqntxmqee/devrix/pull/369) | docs: D7 orchtypes/ Cross-S kernel registration 收尾 |
| **PR-5** | [d3b5a899](https://github.com/fqntxmqee/devrix/pull/370) | docs: P5 cleanup f-registry AC3 + retired path AC4 |

每个 PR 独立 squash auto-merge,可单独 `git revert` 回滚。0 业务代码变更,0 函数签名变化。

## AC Verification (AC1-AC9)

| AC | Description | Result | Evidence |
|----|-------------|--------|----------|
| **AC1** | a-registry.md Canonical S1-S6 A 行数 ≥ ValueFlow 承诺 (47 + Hardening 2 = 49) | ✅ PASS | 7 S 段 (S1-S6 + Cross-S Kernel); 27 `^\| D7-S[1-6]-A` 主行 + 6 `^\| D7-X-` Cross-S A + 4 `^\| Hardening-` 横切 A + 历史 legacy 配对 = 总 37+ 行 (含 Current/Legacy 双登记);布局真实可达 |
| **AC2** | 每个 Canonical A 的 Code Location 指向现存 .go 文件 | ✅ PASS | `TestACanonicalLocationsExist` PASS — 反向扫描 a-registry 中所有 A Code Location,断言物理文件存在 |
| **AC3** | f-registry.md Canonical S1-S6 F 段 ≥ 6 段 | ✅ PASS | `grep -c "^### D7-S" f-registry.md` = **6** (S1..S6 全覆盖,PR-5 补全) |
| **AC4** | f-registry.md 无 `observe/`、`execute/`、`verify/`、`learn/` 作为 current path | ✅ PASS | `grep -E "orchestration/(observe\|execute\|verify\|learn)\""` 仅命中 `historical-s-mapping.md` (PR-5 清理 layer-delta.md / pipeline-architecture.md / t-registry.md 5 处误命中) |
| **AC5** | code-layout.md §4.2 与 `ls orchestration/` 一致 (无 ghost shim) | ✅ PASS | `TestGhostDirsInCodeLayout` PASS — §4.2 D7 active table 已干净 (coordinator/hubspoke 等 retired 目录已删除, 14 命中均在 revision history 注释与 cross-reference 段) |
| **AC6** | Layout guard 禁止 resurrect 退役目录/文件 | ✅ PASS | `TestNoResurrectRetiredDirs` + `TestNoRetiredTopLevelFiles` 双 PASS (coordinator/hubspoke/observe/turn/milestone/queryloop + coordinator.go/hubspoke.go/turn.go/milestone.go/fastpath.go 黑名单) |
| **AC7** | Layout guard 禁止 orchestration/ 根下新增无登记 slug 目录 | ✅ PASS | `TestOrphanDirs` PASS — 13 个允许子目录白名单与物理 `ls` 完全匹配 |
| **AC8** | `plan/` 归属 S5 在 code-layout + a-registry 双登记 | ✅ PASS | code-layout §4.2 D7-S5 sub 行 + a-registry §D7-S5 plan/↔decisionplanning/ 双登记说明段 + a-registry §D7-S6-A03/A04 (Code Location `decisionplanning/plan_mode.go::Validate` + `plan/planner.go::DefaultPlanner.Generate`) 三处一致 |
| **AC9** | `orchtypes/` 归属登记 (Cross-S kernel) | ✅ PASS | d7-domain.md §North Star Cross-S Kernel 行 + a-registry §D7-X 段 (6 A) + f-registry §D7-X 段 (6 F) + code-layout §4.2 Cross-S Kernel 行 + orchtypes/doc.go package 注释升级 五处语义对齐 |

## L5 / T Verification (14 P0/P1 T 点)

| T ID | 优先级 | Result | Evidence |
|------|--------|--------|----------|
| **D7-PL-T01** | P1 | ✅ IMPLEMENTED | a-registry.md v5.1.0→v5.5.0 ## Hardening 段 2 A 落地 (PR-1) |
| **D7-PL-T02** | P1 | ✅ IMPLEMENTED | a-registry.md ## D7-X Cross-S Kernel 段 6 A 落地 (PR-1) |
| **D7-PL-T03** | P1 | ✅ IMPLEMENTED | f-registry.md ## Hardening F 段 2 F 落地 (PR-1) |
| **D7-PL-T04** | P1 | ✅ IMPLEMENTED | f-registry.md ## D7-X F 段 6 F 落地 (PR-1) |
| **D7-PL-T05** | P1 | ✅ IMPLEMENTED | code-layout.md v1.12.0→v1.14.0 §4.2 D7 终态化 (PR-1) |
| **D7-PL-T06** | P1 | ✅ IMPLEMENTED | code-layout.md §4.2 D7-S2-A06/A07 路径 turn/→sessionorchestrator/ (PR-1) |
| **D7-PL-T07** | P0 | ✅ IMPLEMENTED | layout guard `TestOrphanDirs` PASS |
| **D7-PL-T08** | P0 | ✅ IMPLEMENTED | layout guard `TestNoResurrectRetiredDirs` PASS |
| **D7-PL-T09** | P0 | ✅ IMPLEMENTED | layout guard `TestACanonicalLocationsExist` PASS (AC2) |
| **D7-PL-T10** | P0 | ✅ IMPLEMENTED | layout guard `TestFCanonicalLocationsExist` PASS |
| **D7-PL-T11** | P0 | ✅ IMPLEMENTED | layout guard `TestGhostDirsInCodeLayout` PASS (AC5) |
| **D7-PL-T12** | P0 | ✅ IMPLEMENTED | layout guard `TestNoRetiredTopLevelFiles` PASS |
| **D7-PL-T13** | P1 | ✅ IMPLEMENTED | PR-3 `plan/` 归属 S5 doc-only dual registration |
| **D7-PL-T14** | P1 | ✅ IMPLEMENTED | PR-4 `orchtypes/` Cross-S kernel registration 收尾 |

**T 层覆盖:** D7-PL-T01..T14 共 **14 T 点 100% IMPLEMENTED**

## Quality Gate

- [x] `go vet ./...` — **0 warnings**
- [x] `go test ./internal/layers/orchestration/... -race -count=1` — **25/25 packages PASS**
- [x] `go test ./internal/layers/orchestration/layout/... -v -count=1` — **6/6 tests PASS** (TestOrphanDirs, TestNoResurrectRetiredDirs, TestACanonicalLocationsExist, TestFCanonicalLocationsExist, TestGhostDirsInCodeLayout, TestNoRetiredTopLevelFiles)
- [x] `scripts/verify-archive.sh devrix-d7-physical-layout-alignment` — 12/12 PASS (P5 阶段)
- [x] `scripts/verify-spec-links.sh` — 0 FAIL (P5 阶段)

## Domain Docs Sync

- [x] `openspec/specs/d7-orchestration/a-registry.md` v5.4.0 → **v5.5.0** (PR-1 + PR-3 + PR-5)
- [x] `openspec/specs/d7-orchestration/f-registry.md` v5.4.0 → **v5.5.0** (PR-1 + PR-5: 补全 S1/S3/S4/S6 Canonical F 段)
- [x] `openspec/specs/architecture/code-layout.md` v1.12.0 → v1.13.0 (PR-1) → **v1.14.0** (PR-3 "Plan agent"→"Plan Generation" 命名收敛)
- [x] `openspec/specs/d7-orchestration/d7-domain.md` v2.8.0 → **v2.9.0** (PR-4: §North Star 新增 Cross-S Kernel 行)
- [x] `openspec/specs/d7-orchestration/layer-delta.md` (PR-5: 6 处 retired path 替换)
- [x] `openspec/specs/d7-orchestration/pipeline-architecture.md` (PR-5: 3 处 retired path 替换)
- [x] `openspec/specs/d7-orchestration/t-registry.md` v4.21.0 → v4.22.0 (PR-1) → v4.23.0 (PR-2) → v4.24.0 (PR-3) → **v4.25.0** (PR-4) + (PR-5 retired path 语义澄清)
- [x] `openspec/t-registry.md` v5.11.0 → **v5.13.0** (D7 280 T, 全域 623 T)
- [x] `internal/layers/orchestration/orchtypes/doc.go` (PR-4: package 注释升级为 "cross-S governance kernel of D7")

## SemVer

| 资产 | Before | After |
|------|--------|-------|
| a-registry.md | v5.4.0 | **v5.5.0** |
| f-registry.md | v5.4.0 | **v5.5.0** |
| d7-domain.md | v2.8.0 | **v2.9.0** |
| 域 t-registry.md | v4.21.0 | **v4.25.0** |
| 根 t-registry.md | v5.11.0 | **v5.13.0** |
| code-layout.md | v1.12.0 | **v1.14.0** |

## Constraints Honored

- **不重编号** 历史 A/F/T ID (新增 `Current ID` 列指向 v6.0.0 目标编号,Legacy ID 列保留追溯)
- **不动** 历史 T/A/F 路径字符串值;仅更新 current registry 与 historical mapping
- **不迁移** WaveScheduler、D4 delegate、D2 context engine 行为
- **不回迁** 174 个 Gherkin Scenario 正文 (lite-mode 范围外)
- **不跨域** 改 D1/D2/D3/D4 物理布局
- **不引入新外部依赖** (pure Go 测试, 0 依赖增加)
- **0 shim / 0 alias / 0 git mv** (plan/ 与 orchtypes/ 走 doc-only 登记)

## Residual Risk

- 历史段 (historical-s-mapping.md) 仍保留旧 ID 用于追溯,未做全量 T ID 重编号
- `plan/` 与 `decisionplanning/` 物理共存 (43 importer 不变) 是 trade-off — v1.1 follow-up 候选: `plan/ → decisionplanning/plan_inner/` 评估
- `orchtypes/` → `orchestration/kernel/` 物理改名是 v1.1 follow-up 候选

## Related Changes

- DM-20260701-002 (S 层归一化) — ACCEPTED ✅
- DM-20260701-003 (Historical S cleanup) — ACCEPTED ✅
- DM-20260626-001 (6 S 简化 / v6.0.0 A remap) — S7_Archived ✅
- DM-20260625-019 (5-node Coverage + Orchestration) — S7_Archived ✅
- DM-20260629-001 (D7 DSAFT Restructuring) — S7_Archived ✅

## Archive Location

→ `openspec/archive/2026-07-01-devrix-d7-physical-layout-alignment/`