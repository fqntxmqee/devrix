# Implementation Tasks: D7 物理布局对齐 — A 层补全 + S1-S6 代码路径收敛

**Change ID:** `devrix-d7-physical-layout-alignment`
**Demand ID:** DM-20260701-004
**Parent Proposal:** `proposal.md`
**Parent Design:** `design.md`
**Created:** 2026-07-01

---

## Phase P0 — OpenSpec Demand (本 change 已完成 ✅)

- [x] 1.1 创建 `.openspec.yaml`
- [x] 1.2 创建 `demand.md`（DM-20260701-004）
- [x] 1.3 创建 `proposal.md`
- [x] 1.4 创建 `design.md`（六段式）
- [x] 1.5 创建 `tasks.md`（本文档）
- [x] 1.6 创建 D7 delta spec

**T:** D7-PL-T01

---

## Phase P1 — PR-1: A/F registry 补全 + code-layout 终态化（纯文档 PR，~400 行）

> **前置**: P0 完成；S3-Gate Approved
> **风险等级**: 极低（纯 markdown，0 Go 代码）
> **预计时长**: 参考值 1-2h（不含 review）
> **回滚**: `git revert <commit>`

- [ ] 2.1 `a-registry.md` Canonical 段补全 D7-S1-A07/A08 + D7-S2-A04..A07 + D7-S4-A06..A09 + D7-S5-A04/A05/A08 + D7-S6-A01..A15 + Hardening-A01/A02
- [ ] 2.2 每个 Canonical A 行的 Code Location 字段反向验证可达（指向现存 `.go` 文件）
- [ ] 2.3 a-registry.md version bump: v5.3.0 → v5.4.0
- [ ] 2.4 `f-registry.md` Canonical 段补全 D7-S1-A01..A04 (F 行展开) + D7-S2 全段 + D7-S3-A01..A04 + D7-S4-A01..A05 + D7-S5-A01..A08 + D7-S6-A01..A15
- [ ] 2.5 `f-registry.md` Current Path Correction 表扩展为完整 A→F 映射
- [ ] 2.6 f-registry.md version bump: v5.3.0 → v5.4.0
- [ ] 2.7 `code-layout.md §4.2` 终态化：去除 `coordinator/`、`hubspoke/` legacy shim 行；登记 `plan/` (S5 DecisionPlanning)、`orchtypes/` (Cross-S 治理)、`hardening/` (Cross-cutting Discipline Keeper)、`interfaces/` (S6 contract + 跨 S)
- [ ] 2.8 `CHANGELOG.md` 加本 change 一行摘要（Date / Change ID / 一句话）
- [ ] 2.9 `t-registry.md` 登记 D7-PL-T01..T12（PLANNED → IMPLEMENTED 在 PR-1/2 合入时翻转）
- [ ] 2.10 创建 PR-1 (`feat/devrix-d7-physical-layout-alignment` 分支首 PR)，squash auto-merge

**T:** D7-PL-T02, D7-PL-T03, D7-PL-T04, D7-PL-T05, D7-PL-T07, D7-PL-T08, D7-PL-T10

**AC 验证**: AC1 / AC3 / AC4 / AC5（PR-1 范围：registry 补全 + code-layout 终态化；AC2 由 PR-2 layout guard 验证，AC8 由 PR-3 验证，AC9 由 PR-4 验证）

---

## Phase P2 — PR-2: Layout guard 测试（纯测试 PR，~400 行）

> **前置**: PR-1 已合入
> **风险等级**: 极低（仅测试代码）
> **预计时长**: 参考值 2-3h
> **回滚**: `git revert <commit>`

- [ ] 3.1 新建 `internal/layers/orchestration/layout/types.go`（~80 行）：CodeLocation / GuardReport / OrphanDirViolation / ResurrectViolation / MissingLocation 类型
- [ ] 3.2 新建 `internal/layers/orchestration/layout/guard.go`（~150 行）：
  - `func ScanLayout(root string, allowList, denyList []string) (*GuardReport, error)`
  - `func parseARegistryLocations(path string) ([]CodeLocation, error)` — 解析 a-registry.md
  - `func parseFRegistryLocations(path string) ([]CodeLocation, error)` — 解析 f-registry.md
  - `func checkFileExists(loc CodeLocation) error` — 校验 `.go` 文件存在
- [ ] 3.3 新建 `internal/layers/orchestration/layout/guard_test.go`（~250 行，5+ 测试函数）：
  - `TestOrphanDirs` — orchestration/ 根下未在 allow-list 的目录失败
  - `TestNoResurrectRetiredDirs` — coordinator/ hubspoke/ observe/ 等退役目录失败
  - `TestNoRetiredTopLevelFiles` — fastpath.go 等退役文件失败
  - `TestACanonicalLocationsExist` — 扫描 a-registry.md 每个 A 行的 Code Location 必须存在
  - `TestFCanonicalLocationsExist` — 扫描 f-registry.md 每个 F 行的 Code Location 必须存在
  - `TestGhostDirsInCodeLayout` — code-layout.md §4.2 不应再列 ghost 目录
- [ ] 3.4 在 `internal/layers/orchestration/layout/doc.go` 加 package 注释（说明用途 + 链接到本 change design.md）
- [ ] 3.5 创建 PR-2，squash auto-merge

**T:** D7-PL-T06, D7-PL-T11

**AC 验证**: AC2 / AC6 / AC7（layout guard 5+ 测试函数 PASS：TestACanonicalLocationsExist / TestNoResurrectRetiredDirs / TestOrphanDirs）

---

## Phase P3 — PR-3: plan/ S5 doc-only 双登记（design Decision 选 B：doc-only）

> **前置**: PR-2 已合入
> **风险等级**: 极低（纯文档 PR，0 Go 代码改动）
> **预计时长**: 参考值 30min
> **回滚**: `git revert <commit>`

- [ ] 4.1 决策记录：design §附录 D Q1 选 B（doc-only），`plan/` 与 `decisionplanning/` 物理共存（避免 43 importer 改动）
- [ ] 4.2 在 `a-registry.md` `D7-S6-A03` PlanValidate / `D7-S6-A04` PlanGenerate 的 Code Location 字段标注 `internal/layers/orchestration/plan/`（S6 治理 Activity 物理在 S5 路径，对应 design §④ S5 sub-registration carve-out + spec.md L146 S5 carve-out Note；直接登记 plan/ 路径，无 alias 形式）
- [ ] 4.3 在 `a-registry.md` Hardening 段或其他 S5 段加 cross-reference：`plan/` ↔ `decisionplanning/` 双登记说明（plan/ PlanKind/DefaultPlanner 6 .go 文件 = 5 prod + 1 test；decisionplanning/ Classifier/Decomposer 16 .go 文件 = 8 prod + 8 test）
- [ ] 4.4 在 `code-layout.md §4.2` 加一行：`D7-S5 | Plan Generation | plan | orchestration/plan/ | ✅ doc-only 双登记（与 decisionplanning/ 并列 S5）`
- [ ] 4.5 创建 PR-3，squash auto-merge

**T:** D7-PL-T07 (覆盖)

**AC 验证**: AC8（plan/ 归属 S5 在双 doc 登记）

---

## Phase P4 — PR-4: orchtypes/ 归属登记（design Decision 选 A：跨 S kernel）

> **前置**: PR-3 已合入（或与 PR-3 并行）
> **风险等级**: 极低（纯文档 + 0 物理改动）
> **预计时长**: 参考值 30min
> **回滚**: `git revert <commit>`

- [ ] 5.1 在 `orchtypes/doc.go` package 注释加 1 行：`Package orchtypes is the cross-S governance kernel of D7 (types, sentinels, intent/observation primitives).`
- [ ] 5.2 在 `a-registry.md` 加新段 `## D7 Cross-S Kernel (orchtypes/)`：
  - A: D7-X-A01 OrchestrationTypes (intent.go + observation.go + types aliases)
  - A: D7-X-A02 BoundaryDecisions (boundary_decision.go 3 const)
  - A: D7-X-A03 AdaptivePrior (adaptive_prior_overload.go)
  - A: D7-X-A04 AnomalyDetector (anomaly_detector.go)
  - A: D7-X-A05 ProcessConfig (process.go + config.go)
  - A: D7-X-A06 LLMInvoker (llm_invoker.go)
- [ ] 5.3 在 `code-layout.md §4.2` 加一行：`- | Cross-S governance kernel | orchtypes | orchestration/orchtypes/ | ✅ Cross-S (S5 intent + S6 types + S1-S6 共享)`
- [ ] 5.4 在 `f-registry.md` 加新段 `## D7 Cross-S Kernel F 层 (orchtypes/)`
- [ ] 5.5 在 `d7-domain.md` §North Star 加一行：`Cross-S Kernel (orchtypes/): types/sentinels/intent primitives — single source of truth for D7 contract`
- [ ] 5.6 创建 PR-4，squash auto-merge

**T:** D7-PL-T08 (覆盖), D7-PL-T09, D7-PL-T12 (覆盖)

**AC 验证**: AC9

---

## Phase P5 — Acceptance + Archive

> **前置**: PR-1..PR-4 全部合入
> **风险等级**: 极低
> **预计时长**: 参考值 1h

- [ ] 6.1 运行全量回归：`go vet ./...` + `go test ./internal/layers/orchestration/... -race -count=1`
- [ ] 6.2 运行 layout guard 测试：`go test ./internal/layers/orchestration/layout/... -v`
- [ ] 6.3 验证 AC1-AC9 全部 PASS
- [ ] 6.4 创建 `acceptance-report.md`：
  - 12 T 点 IMPLEMENTED 状态
  - AC1-AC9 全部 PASS 验证记录
  - 22 orchestration packages `-race -count=1` 全绿
  - PR-1/2/3/4 squash auto-merge commit hash 引用
- [ ] 6.5 更新 `openspec/demand-archive-index.md`：DM-20260701-004 行加 PR 链接 + ACCEPTED (S7_Archived)
- [ ] 6.6 运行 `scripts/verify-archive.sh` 全 12 项 PASS
- [ ] 6.7 运行 `scripts/verify-spec-links.sh`（lite-mode 7 站收官工具）查本 change 链接 PASS
- [ ] 6.8 归档 change → `openspec/archive/2026-07-01-devrix-d7-physical-layout-alignment/`
- [ ] 6.9 更新 `openspec/specs/d7-orchestration/CHANGELOG.md`：version bump v5.3.0 → v5.4.0 + 一行摘要

**T:** D7-PL-T11 (覆盖), D7-PL-T12

---

## Completion Checklist

- [ ] 12 T 点全部 IMPLEMENTED（D7-PL-T01..T12）
- [ ] AC1-AC9 全部 PASS
- [ ] 22 orchestration packages `go test -race -count=1` 100% PASS
- [ ] `go vet ./...` 0 警告
- [ ] `verify-archive.sh` 12/12 PASS
- [ ] `verify-spec-links.sh` 0 FAIL
- [ ] 4 PR 全部 squash auto-merge
- [ ] spec.md ≤ 200 行（lite-mode 不破）
- [ ] 域 t-registry v4.21.0 → v4.22.0
- [ ] 根 t-registry v5.11.0 → v5.12.0
- [ ] a-registry v5.3.0 → v5.4.0
- [ ] f-registry v5.3.0 → v5.4.0
- [ ] code-layout v1.12.0 → v1.13.0
- [ ] OpenSpec change 包归档完成

---

## T 编号索引（域 t-registry 登记用）

| T ID | 优先级 | 阶段 | Given-When-Then 摘要 |
|------|--------|------|----------------------|
| **D7-PL-T01** | P0 | P0 | change 包齐全（demand/proposal/design/tasks/delta spec） |
| **D7-PL-T02** | P0 | P1 | a-registry Canonical 段 S1-S6 A 行数 ≥ ValueFlow 表承诺（47+Hardening 2） |
| **D7-PL-T03** | P0 | P1 | 每个 Canonical A 的 Code Location 指向现存 `.go` 文件 |
| **D7-PL-T04** | P1 | P1 | f-registry 无 `observe/`、`execute/`、`verify/`、`learn/` 作为 current path |
| **D7-PL-T05** | P1 | P1 | code-layout.md D7 §4.2 与 `ls orchestration/` 一致（无 ghost shim） |
| **D7-PL-T06** | P1 | P2 | layout guard：禁止 resurrect 退役目录/文件 |
| **D7-PL-T07** | P1 | P3 | `plan/` 归属 S5 在 code-layout + a-registry 双登记 |
| **D7-PL-T08** | P1 | P4 | `orchtypes/` 归属登记（Cross-S kernel） |
| **D7-PL-T09** | P2 | P4 | S6 overlay 5+2+1 paths (5 S6 overlay + 2 Cross-S + 1 cross-cutting) 在 design.md 有 activity→path 矩阵 |
| **D7-PL-T10** | P2 | P1 | hardening/ 横切 A 与 S6-A14 / Hardening-A01/A02 映射清晰 |
| **D7-PL-T11** | P1 | P5 | `go test ./internal/layers/orchestration/... -race` 全绿 |
| **D7-PL-T12** | P1 | P5 | acceptance-report 含 12 T 点 IMPLEMENTED + 4 PR commit hash + verify-archive 12/12 PASS |