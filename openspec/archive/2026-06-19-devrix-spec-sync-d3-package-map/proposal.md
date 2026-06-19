# Proposal: D3 LLM Gateway spec 补登

**Change ID:** devrix-spec-sync-d3-package-map
**Demand ID:** DM-20260619-002
**Status:** S2_Proposal
**Date:** 2026-06-19
**Author:** Devrix Team

> **docs-only change**：本 change 仅改 D3 spec 三份文档（spec.md / design.md / model-resolution-trace.md），不动 D3 域代码、不改 D-S 编号。

---

## 1. 背景

D3 LLM Gateway 在 2026-06-14 完成 v2.0 物理路径迁移（DM-20260614-019 `devrix-d3-sa-refine-v2.0`），7 路径迁移 + 8 bridge + contracts.go 拆分全部落地：
- `adapter/` → `stream/adapter/`
- `gateway/router/` → `route/`
- `gateway/gateway/` → `stream/`
- `breaker/` + `retry/` → `protect/`
- `token/` → `budget/`
- `safety/` → `guard/`
- `config/` + `shared/config/` → `configure/`

但 D3 spec **未完全同步 v2.0 状态**，存在 4 处不一致。

## 2. 问题陈述

### 2.1 `protect/` 子包在 spec Package Map 中完全缺失

`internal/layers/llmgateway/protect/` 实际包含 10 个 .go 文件：
- `circuit_breaker.go` + `state.go`（熔断器）
- `retry.go` + `retry_jitter_test.go`（重试）
- `breaker_observer.go` + `observer.go`（可观测性）
- 以及 4 个 test 文件

`protect/errorclass/` 子包（含 `classifier.go`）作为 protect 的内部子包也未被 spec 提及。

### 2.2 `configure/shared_config.go` v2.0 路径未跟进

代码已迁移到 `internal/layers/llmgateway/configure/shared_config.go`（v2.0），但 spec.md §2.1 仍标注 v1 路径 `internal/shared/config/llmgateway.go`。`configure/` 子包在 spec Package Map 中也未列出。

### 2.3 `stream/adapter/protocol.go` 已实现但 §13 FR-5 仍标"待实施"

代码 `internal/layers/llmgateway/stream/adapter/protocol.go` 已实现 v1.1 F4 `Protocol()` 方法，但 spec.md §13 FR-5 仍标"待实施"。

### 2.4 design.md v3.2.0 自称 v2.0 Phase F 实施中，实际已完成

design.md line 9 标 Version 3.2.0 / Last Updated 2026-06-14；line 25 称"§10.2 物理路径计划：'v2.0 启动时执行'占位"；line 776 称"10.2 v2.0 物理路径（Phase F 实施中）"。但 v2.0 已于 2026-06-14 落地并归档，design.md 状态未刷新。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | spec.md §10 Package Map 补充 `protect/` + `protect/errorclass/` 子包 | P0 |
| AC2 | spec.md §2.1 路径从 `shared/config/llmgateway.go` 更新为 `configure/shared_config.go` | P0 |
| AC3 | spec.md §10 Package Map 补充 `configure/` 子包 | P0 |
| AC4 | spec.md §13 FR-5 状态从"待实施"改为"已实施（v1.1 F4）" | P0 |
| AC5 | design.md v3.2.0 → v3.3.0，§10.2 状态从"实施中"改为"已完成（DM-019）" | P0 |
| AC6 | `go vet ./...` 0 错 | P1 |
| AC7 | `verify-archive.sh openspec/changes/devrix-spec-sync-d3-package-map` 全部 PASS | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖（已闭环）| `devrix-d3-sa-refine-v2.0` (DM-20260614-019) — v2.0 物理路径迁移 |
| 依赖（已闭环）| `devrix-d3-sa-refine-v1.1` (DM-20260614-017) — v1.1 韧性可见性 |
| 约束 | docs-only，不动 .go 源码 |
| 约束 | D3 spec Scenarios 行为不变（仅同步状态与目录树） |
| 约束 | 沿用 Devrix GitHub Flow：`feat/spec-sync-d3-package-map` 分支 + squash merge + auto-merge |
| 约束 | 归档后 `demand-archive-index.md` 追加新行 |

## 5. 变更范围

### 修改 3 个
- `openspec/specs/d3-llm-gateway/spec.md` — §2.1 路径更新；§10 Package Map 补 protect/、configure/；§13 FR-5 状态更新
- `openspec/specs/d3-llm-gateway/design.md` — v3.2.0 → v3.3.0；§10.2 状态更新
- `openspec/specs/d3-llm-gateway/model-resolution-trace.md` — `Last Updated: 2026-06-19` + 同步状态

### 新建 5 个（4 件套 + yaml）
- `openspec/changes/devrix-spec-sync-d3-package-map/.openspec.yaml`
- `openspec/changes/devrix-spec-sync-d3-package-map/proposal.md`（本文件）
- `openspec/changes/devrix-spec-sync-d3-package-map/design.md`（S3 生成）
- `openspec/changes/devrix-spec-sync-d3-package-map/tasks.md`（S4 生成）
- `openspec/changes/devrix-spec-sync-d3-package-map/acceptance-report.md`（S5 生成）

### 不变更
- `internal/layers/llmgateway/**` 全部代码
- D-S 编号（D3-S/A/F/T）
