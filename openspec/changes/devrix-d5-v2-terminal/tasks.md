# Implementation Tasks: D5 v2.1 终态重构

**Demand ID:** DM-20260619-006  
**Change ID:** devrix-d5-v2-terminal  
**Owner Decisions:** 范围 C；S23 子承诺；DebugFilter→S21；SessionBridge→S0；bridge 本轮删除

---

## 阶段状态（研发流程）

| OpenSpec 阶段 | 状态 | 产出物 |
|---------------|------|--------|
| S1 需求 | ✅ 完成 | `demand.md` |
| S2 提案 | ✅ 完成 | `proposal.md`, `.openspec.yaml` |
| S3 设计 | ✅ 完成 | `design.md`, `specs/**`, `design-review.md` |
| **S3-Gate** | ⏳ 待 Owner 确认 | `design-review.md` → Approved |
| S4 实现 | 🚫 **未启动**（Owner：先不开发） | 代码 + tests |
| S5 验收 | — | `acceptance-report.md` |
| S6/S7 归档 | — | `openspec/archive/` + specs 回写 |

**S4 前置条件：** S3-Gate Approved + 创建 Draft PR（`docs/d5-v2-terminal-spec`）。

---

## Phase A: 规格终态（docs-only）

| ID | 任务 | 文件 | T 锚点 |
|----|------|------|--------|
| A1 | 新建 `d5-domain.md`（North Star + 双轨 + 路径表 + 跨域摘要） | `openspec/specs/d5-observability/d5-domain.md` | — |
| A2 | `spec.md` v3.0：Canonical S21–S24 主表；D7 Turn 主路径；Legacy 下沉 | `spec.md` | — |
| A3 | 新建 `observability-guide.md`（Span↔T + Trace 树 + Runbook） | `observability-guide.md` | P0 矩阵 |
| A4 | 新建 `terminal-state-guide.md` + `dsaft-architecture.md` Stub | 同上目录 | — |
| A5 | 新建 `d5-boundary.md`；刷新 `cross-domain-boundaries.md` D5 段 | `d5-boundary.md`, `architecture/cross-domain-boundaries.md` | — |
| A6 | `a-registry` v4.0：路径同步 + A14/A03/A07–A10 + Code Location 列 | `a-registry.md` | — |
| A7 | `f-registry` v3.0：canonical_s 列 + 诊断 F + 路径更新 | `f-registry.md` | — |
| A8 | `design.md` v3.0 + `layer-delta.md` §v2.1-Terminal | `design.md`, `layer-delta.md` | — |
| A9 | `span-registry.md` + `coverage.md`：query.loop → RETIRED only | 同上 | — |
| A10 | `t-registry` v3.2：canonical_s/canonical_a 校正 | `t-registry.md` | — |
| A11 | `code-layout.md` §4.6 diagnose 子目录完整列表 | `architecture/code-layout.md` | — |
| A12 | `docs/architecture/code-map.md` D5 段同步 | `docs/architecture/code-map.md` | — |

**AC:** AC-A1..AC-A7  
**PR:** `docs/d5-v2-terminal-spec`  
**Gate:** S3-Gate — `grep query.loop` 仅 RETIRED/Legacy；spec 主表无 D5-S1–S9

---

## Phase B1: 根目录归位

| ID | 任务 | 源 → 目标 |
|----|------|-----------|
| B1.1 | `git mv genai_tokens.go genai_tokens_test.go` | → `instrument/metrics/` |
| B1.2 | `git mv llm_log.go llm_log_test.go` | → `diagnose/incident/` |
| B1.3 | 更新 package 声明 + 全仓 import | bootstrap, bridge, tests |
| B1.4 | 删除根目录 `slog_bridge.go`；`observability.go` 调 `instrument/logger` 安装桥 | `observability.go` |

**AC:** AC-B2  
**T:** D5-S21-A07-T01, D5-S23-A04/A05 相关  
**PR:** `chore/d5-root-file-relocate`

---

## Phase B2: bridge 删除

| ID | 任务 | 文件 |
|----|------|------|
| B2.1 | `bridge.go` import 改 `instrument/tracer|metrics|logger` | `bridge.go` |
| B2.2 | 测试文件改 canonical import | `tracer_coverage_test.go`, `names_test.go`, `span_error_test.go` |
| B2.3 | 删除 9 个 bridge 目录 | `tracer/ metrics/ logger/ telemetry/ exporter/ coverage/ incident/ settings/ runtime/` |
| B2.4 | 全仓 `grep observability/tracer` 等 → 0 命中（除 archive/docs） | — |
| B2.5 | layer-lint 若有 bridge 白名单 → 移除 | `internal/lint/layer/` |

**AC:** AC-B1, AC-B5  
**T:** 全量 41 T  
**PR:** `chore/d5-bridge-removal`

---

## Phase B3: PLANNED T 闭合

| ID | 任务 | T |
|----|------|---|
| B3.1 | 评估 D5-S21-A05-T01/T02：补测试或引用既有 integration 标 IMPLEMENTED | A05-T01/T02 |
| B3.2 | 验证 HealthCheck coverage 字段；标 D5-S23-A06-T02 IMPLEMENTED | A06-T02 |
| B3.3 | 更新 `t-registry` Status 列 | `t-registry.md` |

**AC:** AC-B4  
**PR:** 可并入 B2 或独立 `test/d5-planned-t-close`

---

## Phase C: 验收与归档

| ID | 任务 |
|----|------|
| C1 | `go test ./... -race` + obs integration + layer-lint strict |
| C2 | acceptance-report.md |
| C3 | S7 归档 → `openspec/archive/2026-06-19-devrix-d5-v2-terminal/` |
| C4 | 回写 `openspec/specs/` + `demand-archive-index.md` DM-20260619-006 |
| C5 | 删除 `openspec/changes/devrix-d5-v2-terminal/` |

**AC:** AC-B4, AC-B5 + 全部 AC-A*

---

## 依赖顺序

```text
Phase A (docs) ──S3-Gate──► Phase B1 ──► Phase B2 ──► Phase B3 ──► Phase C
```

Phase A 与 B1 可并行（无冲突）；B2 依赖 B1 完成（避免 merge 冲突）。

---

## 估时（仅任务粒度，非排期承诺）

| Phase | 任务数 | 复杂度 |
|-------|--------|--------|
| A | 12 | 中（文档量大但无代码风险） |
| B1 | 4 | 低 |
| B2 | 5 | 中 |
| B3 | 3 | 低 |
| C | 5 | 低 |
