# Implementation Tasks: D5 v2.1 终态重构

**Demand ID:** DM-20260619-006  
**Change ID:** devrix-d5-v2-terminal  
**Owner Decisions:** 范围 C；S23 子承诺；DebugFilter→S21；SessionBridge→S0；bridge 本轮删除不留 shim；Phase A 含代码锚点

---

## 阶段状态（研发流程）

| OpenSpec 阶段 | 状态 | 产出物 |
|---------------|------|--------|
| S1 需求 | ✅ 完成 | `demand.md` |
| S2 提案 | ✅ 完成 | `proposal.md`, `.openspec.yaml` |
| S3 设计 | ✅ 完成 | `design.md`, `specs/**`, `design-review.md` |
| **S3-Gate** | ✅ Approved 2026-06-19 | `design-review.md` §11 |
| S4 实现 | ✅ 完成 | PR #118 (A) · #119 (B2a) · #120 (B2b) · #121 (B1) · #122 (B3) |
| S5 验收 | ✅ ACCEPTED | `acceptance-report.md` (17/17 AC PASS) |
| S6/S7 归档 | ⏳ 当前 | `openspec/archive/2026-06-19-devrix-d5-v2-terminal/` (this PR) |

**S4 前置条件：** S3-Gate Approved + 创建 Draft PR（`docs/d5-v2-terminal-spec`）。

---

## Phase A: 规格终态（docs + registry 代码锚点）

| ID | 任务 | 文件 | T 锚点 | AC |
|----|------|------|--------|-----|
| A1 | 新建 `d5-domain.md`（Tl;DR + North Star + 完备性边界 + 博弈论玩家表 + 时间属性矩阵 + S23 子承诺分组 + S25 触发条件 + 子承诺举证责任 + Terminal 冻结声明 + Bridge 删除时间线 + 文档阅读优先级） | `openspec/specs/d5-observability/d5-domain.md` | — | AC-A1, AC-A9 |
| A2 | `spec.md` v3.0：Canonical S21–S24 主表；D7 Turn 主路径；Legacy 下沉 | `spec.md` | — | AC-A2 |
| A3 | 新建 `observability-guide.md`（Span↔T + Trace 树 + Runbook + on-call 排障动线 + Coverage 多维指标 + WARN metric 聚合 + D5 成功指标双轨） | `observability-guide.md` | P0 矩阵 | AC-A3 |
| A4 | 新建 `terminal-state-guide.md` + `dsaft-architecture.md` Stub | 同上目录 | — | AC-A4 |
| A5 | 新建 `d5-boundary.md`；刷新 `cross-domain-boundaries.md` D5 段（含 D5→D6 证据移交规则） | `d5-boundary.md`, `architecture/cross-domain-boundaries.md` | — | AC-A5 |
| A6 | `a-registry` v4.0：路径同步 + A14/A03/A07–A10 + Code Location 列 ⬅️ **代码锚点** | `a-registry.md` | — | AC-A6, AC-A8 |
| A7 | `f-registry` v3.0：canonical_s 列 + 诊断 F + 路径更新 | `f-registry.md` | — | AC-A6 |
| A8 | `design.md` v3.0 + `layer-delta.md` §v2.1-Terminal（design.md §5 含 S23 硬边界 + S25 触发条件 + 举证责任；§10 含 B2 拆步不留 shim + 代码锚点 + 启动对账 + 跨 Change 级联；§12 含 legacy_harness 退役计划） | `design.md`, `layer-delta.md` | — | AC-A6 |
| A9 | `span-registry.md` + `coverage.md`：query.loop → RETIRED only | 同上 | — | AC-A7 |
| A10 | `t-registry` v3.2：canonical_s/canonical_a 校正 ⬅️ **代码锚点** | `t-registry.md` | — | AC-A6, AC-A8 |
| A11 | `code-layout.md` §4.6 diagnose 子目录完整列表 | `architecture/code-layout.md` | — | AC-A7 |
| A12 | `docs/architecture/code-map.md` D5 段同步 | `docs/architecture/code-map.md` | — | — |

**AC:** AC-A1..AC-A9  
**PR:** `docs/d5-v2-terminal-spec`  
**Gate:** S3-Gate — `grep query.loop` 仅 RETIRED/Legacy；spec 主表无 D5-S1–S9

> ⚠️ **Phase A 必须包含 ≥1 个代码锚点**（A6 或 A10），不可推迟到 Phase B。纯文档承诺强度为零（cheap talk）。

---

## Phase B2a: bridge.go import 改 canonical（不删包）

| ID | 任务 | 文件 |
|----|------|------|
| B2a.1 | `bridge.go` import 改 `instrument/tracer\|metrics\|logger` | `bridge.go` |
| B2a.2 | 测试文件改 canonical import | `tracer_coverage_test.go`, `names_test.go`, `span_error_test.go` |
| B2a.3 | `go build ./...` + `go test ./...` 通过（bridge 包仍存在） | — |

**AC:** AC-B1
**PR:** `chore/d5-bridge-import-fix`

---

## Phase B2b: 删除 9 bridge 包（不留 shim）

| ID | 任务 | 文件 |
|----|------|------|
| B2b.1 | 删除 9 个 bridge 目录 | `tracer/ metrics/ logger/ telemetry/ exporter/ coverage/ incident/ settings/ runtime/` |
| B2b.2 | 全仓 `grep` 旧 bridge 路径 → 0 命中（除 archive/docs） | — |
| B2b.3 | layer-lint 若有 bridge 白名单 → 移除 | `internal/lint/layer/` |
| B2b.4 | CI 增加 bridge 防回归规则（grep 9 个旧路径 = 0 命中） | `.github/workflows/` |
| B2b.5 | `go test ./... -race` + obs integration + layer-lint 全绿 | — |

**AC:** AC-B2, AC-B3
**T:** 全量 41 T
**PR:** `chore/d5-bridge-removal`

> ⚠️ **不留 shim。** B2a↔B2b 之间不留 release 间隔。shim = 道德风险（B2b 被无限期推迟）。

---

## Phase B1: 根目录归位

| ID | 任务 | 源 → 目标 |
|----|------|-----------|
| B1.1 | `git mv genai_tokens.go genai_tokens_test.go` | → `instrument/metrics/` |
| B1.2 | `git mv llm_log.go llm_log_test.go` | → `diagnose/incident/` |
| B1.3 | 更新 package 声明 + 全仓 import | bootstrap, bridge, tests |
| B1.4 | 删除根目录 `slog_bridge.go`；`observability.go` 调 `instrument/logger` 安装桥 | `observability.go` |

**AC:** AC-B4
**T:** D5-S21-A07-T01, D5-S23-A04/A05 相关
**PR:** `chore/d5-root-file-relocate`

---

## Phase B3: t-registry 校正 + PLANNED T 闭合

| ID | 任务 | T |
|----|------|---|
| B3.1 | canonical_s 校正：A08→S21, A06→S0；canonical_a 校正：Doctor T→A10 | t-registry.md |
| B3.2 | 评估 D5-S21-A05-T01：补 propagation 集成测试或标 IMPLEMENTED | A05-T01 |
| B3.3 | 评估 D5-S21-A05-T02：补 Counter 单元测试或标 IMPLEMENTED | A05-T02 |
| B3.4 | 验证 HealthCheck coverage 字段；标 D5-S23-A06-T02 IMPLEMENTED | A06-T02 |
| B3.5 | `legacy_harness` metric help text 标 DEPRECATED | `configure/runtime/path_resolver.go` |

**AC:** AC-B5, AC-B6, AC-B8

---

## Phase C: 验收与归档

| ID | 任务 |
|----|------|
| C1 | `go test ./... -race` + obs integration + layer-lint strict |
| C2 | `grep` bridge 防回归 CI 规则验证（0 命中） |
| C3 | acceptance-report.md（按 AC-A1..AC-A9 + AC-B1..AC-B8 逐条验收） |
| C4 | S7 归档 → `openspec/archive/2026-06-19-devrix-d5-v2-terminal/` |
| C5 | 回写 `openspec/specs/` + `demand-archive-index.md` DM-20260619-006 |
| C6 | 删除 `openspec/changes/devrix-d5-v2-terminal/` |

**AC:** 全部 AC-A* + AC-B*

---

## 依赖顺序

```text
Phase A (docs + code anchors) ──S3-Gate──► Phase B2a ──► Phase B2b ──► Phase B1 ──► Phase B3 ──► Phase C
```

- Phase A 必须包含 ≥1 个代码锚点（A6 a-registry v4.0 或 A10 t-registry canonical 列）
- B2a（改 import）先于 B2b（删包），降低爆炸半径
- B2a↔B2b 之间不留 release shim
- B1（git mv）可与 B2a/B2b 并行（无文件冲突）
- B3 依赖 B2b 完成后的稳定 T 基线

---

## 估时（仅任务粒度，非排期承诺）

| Phase | 任务数 | 复杂度 |
|-------|--------|--------|
| A | 12 | 中（文档量大但无代码风险） |
| B2a | 3 | 低 |
| B2b | 5 | 中 |
| B1 | 4 | 低 |
| B3 | 5 | 低 |
| C | 6 | 低 |
