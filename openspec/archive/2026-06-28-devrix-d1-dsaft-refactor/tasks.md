# Tasks: D1 DSAFT Refactor

**Change ID:** `devrix-d1-dsaft-refactor`  
**Demand ID:** DM-20260628-003  
**Status:** S7_Archived  
**Design SoT:** `design.md` v1.0.0

---

## Phase 0 — 完整设计 ✅

| ID | Description | Status |
|----|-------------|--------|
| T0a | `design.md` v1.0 — S/A 职责、链路、包结构、边界 | [x] |
| T0b | `acceptance-criteria.md` — LC/AC/T 全覆盖矩阵 | [x] |
| T0c | proposal 更新为 S3_Design + Review 冻结门 | [x] |
| T0d | **Design Review** — design.md §11 清单 | [x] |

## Phase 1 — Agent provision migration ✅

| ID | Description | Status |
|----|-------------|--------|
| T1a–T1d | sessionagents + capture 边界 | [x] |

## Phase 1.5 — AC 闭环 ✅

| ID | Description | Status |
|----|-------------|--------|
| T1.5a–T1.5d | D1-RF-T01..T05 + L5 + 规格同步 | [x] |

## Phase 2 — Gateway split（设计冻结后实施）

| ID | Description | Status | design.md |
|----|-------------|--------|-----------|
| T2a | `session.go` 迁移 | [x] | §5.1 |
| T2b | `outbound.go` + `dispatch.go` 迁移 | [x] | §5.1 |
| T2c | `gateway.go` facade ≤200 LOC | [~] | §5.1 — 324 LOC（span helpers 留 facade） |
| T2d | 移除 `IContextEngine` alias | [x] | §5.1 |
| T2e | D1-RF-T06 text delta 独立 T | [x] | §8 |
| T2f | D1-RF-T07 milestone presenter T | [x] | §8 |

> `ingress.go` 已提取（Phase 2 预研）；Review 通过后继续 T2a–f。

## Phase 3 — Channel DTO decoupling

| ID | Description | Status | design.md |
|----|-------------|--------|-----------|
| T3a | `contracts.WorkerProgressView` + presentation DTOs | [x] | §6.2 |
| T3b | feishu_worker_card 去 wavescheduler import | [x] | §6.2 |
| T3c | CLI `/task`/`/plan` → contracts.TaskCLIHandler | [x] | §4.2, §6.2 |
| T3d | D1-RF-T09 channel adapters import 门禁 | [x] | §8 |

## Phase 4 — Registry + CI

| ID | Description | Status | design.md |
|----|-------------|--------|-----------|
| T4a | a-registry Legacy S1–S12 → layer-delta | [x] | §2.1 |
| T4b | `scripts/lint-d1-imports.sh` + CI | [x] | §8 |
| T4c | S7 archive + canonical spec 回写 | [x] | §9 |
