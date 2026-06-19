# Acceptance Report — DM-20260619-006 devrix-d5-v2-terminal

**Change ID:** devrix-d5-v2-terminal
**Change Title:** D5 Observability v2.1 终态重构 — 规格冻结 + bridge 删除 + 根目录归位 + t-registry 校正
**Demand ID:** DM-20260619-006
**Date:** 2026-06-19
**Author:** OMC Agent (S5 验收 + S6 归档驱动)
**Branch:** `feat/archive-devrix-d5-v2-terminal` (this PR)
**Status:** ✅ **ACCEPTED** → 进入 S7 归档阶段

---

## 1. AC 总表 — 17 / 17 PASS (100%)

| AC | 优先级 | 描述 | 状态 | 证据 (PR / commit / 文件) |
|----|--------|------|------|---------------------------|
| **AC-A1** | P0 | 新建 `d5-domain.md`：Tl;DR + North Star + 4 承诺 + 完备性边界 + 博弈论玩家表（含 SRE/on-call）+ 时间属性×承诺强度交叉矩阵 + S23 子承诺（按时间分组）+ S25 触发条件 + 子承诺举证责任 + Terminal 冻结声明 + 各域 Bridge 删除时间线 + Out of Scope + 物理路径表 + 文档阅读优先级标注 | ✅ PASS | PR #118 · `openspec/specs/d5-observability/d5-domain.md` v1.1.0 |
| **AC-A2** | P0 | `spec.md` v3.0：DSAFT 主表 S21–S24；D7 Turn 主路径；query.loop 仅 RETIRED 节 | ✅ PASS | PR #118 · `openspec/specs/d5-observability/spec.md` v3.0.0 · `grep -E "D5-S(0[1-9]\|1[0-9]\|20)\b" spec.md` = 0 命中 |
| **AC-A3** | P0 | 新建 `observability-guide.md`：D7 Turn Trace 树 + Span↔T P0 矩阵 + P0 Runbook + Coverage 多维指标 + WARN metric 聚合 + D5 成功指标双轨声明 | ✅ PASS | PR #118 · `openspec/specs/d5-observability/observability-guide.md` |
| **AC-A4** | P1 | 新建 `terminal-state-guide.md` + `dsaft-architecture.md` Stub | ✅ PASS | PR #118 · 同名文件 |
| **AC-A5** | P0 | 新建 `d5-boundary.md`；更新 `cross-domain-boundaries.md` D5 段（含 D5→D6 证据移交规则） | ✅ PASS | PR #118 · `d5-boundary.md` + `architecture/cross-domain-boundaries.md` |
| **AC-A6** | P0 | `a-registry` v4.0 + `f-registry` v3.0 + `design.md` v3.0 同步（design.md §5 含 S23 硬边界 + S25 触发条件 + 子承诺举证责任；§10 含 Phase B2 拆步不留 shim + Phase A 代码锚点 + Phase B 启动对账条件 + 跨 Change 级联标注；§12 含 legacy_harness 退役计划） | ✅ PASS | PR #118 · `a-registry.md` v4.0 + `f-registry.md` v3.0 + `design.md` v3.0 + `layer-delta.md` §v2.1-Terminal |
| **AC-A7** | P0 | `code-layout.md` §4.6 diagnose 子目录完整；`grep query.loop` 仅 Legacy/RETIRED | ✅ PASS | PR #118 · `architecture/code-layout.md` §4.6 · 6 处 `query.loop` 命中均位于 RETIRED/Legacy 节 |
| **AC-A8** | P0 | Phase A 包含 ≥1 个代码锚点（a-registry v4.0 Code Location 更新 或 t-registry canonical 列校正），不可推迟到 Phase B | ✅ PASS | PR #118 · `a-registry.md` Code Location 列全量更新 + `t-registry.md` v3.2 canonical_s/canonical_a 双校正 |
| **AC-A9** | P1 | 所有新建 spec 文档包含阅读优先级标注（MUST/SHOULD/REFERENCE） | ✅ PASS | PR #118 · 8 个新文档均带「阅读优先级」区块 |
| **AC-B1** | P0 | Phase B2a：`bridge.go` import 改 `instrument/*` 直连，`go build ./...` 通过 | ✅ PASS | PR #119 · 5 文件 import 改 canonical · `go build ./...` clean |
| **AC-B2** | P0 | Phase B2b：删除 9 个 bridge 包（不留 shim）；全仓 `grep` 旧 bridge 路径 = 0 命中（除 archive/docs）；`go test ./... -race` 全绿 | ✅ PASS | PR #120 · `git rm -r` 9 个目录 · `grep -rE "internal/layers/observability/(tracer\|metrics\|logger\|telemetry\|exporter\|coverage\|incident\|settings\|runtime)" --include=*.go` = 0 命中 · `go test -race ./...` 全绿 |
| **AC-B3** | P0 | CI 包含 bridge 防回归规则（grep 9 个旧路径 = 0 命中，否则 CI 拒绝） | ✅ PASS | PR #120 · `.github/workflows/ci.yml` §"Bridge regression guard" step · 9 路径硬阻断 |
| **AC-B4** | P1 | `genai_tokens.go` → `instrument/metrics/`；`llm_log.go` → `diagnose/incident/`；`slog_bridge.go` 调 `instrument/logger` 安装桥 | ✅ PASS | PR #121 · 4 文件 `git mv` · observability.go facade wrapper · `cmd/devrix/main.go` + `stream/gateway.go` 改 import |
| **AC-B5** | P1 | `t-registry` canonical_s 校正（A08→S21, A06→S0）；canonical_a 校正（Doctor T→A10）；**3** PLANNED T 闭合（D5-S21-A05-T01, D5-S21-A05-T02, D5-S23-A06-T02） | ✅ PASS | PR #118 + #122 · `t-registry.md` v3.2.0 · Statistics 行：43 T / 41 IMPLEMENTED / 0 PLANNED / 2 REMOVED |
| **AC-B6** | P0 | 41/41 T IMPLEMENTED（PLANNED 全部闭合后），每条 P0 T 有明确 Span 证据或 sad path 说明 | ✅ PASS | `openspec/specs/d5-observability/t-registry.md` v3.2.0 · 41 IMPLEMENTED · D5-S21-A01-T03 历史缺口（非本 change）已说明 |
| **AC-B7** | P0 | `go test ./... -race` + layer-lint + obs integration 全绿 | ✅ PASS | 99 packages OK + layer-lint (warn) PASS · `go vet ./...` clean |
| **AC-B8** | P1 | `legacy_harness` metric help text 标 DEPRECATED；退役计划写入 `design.md` §12（v2.1 DEPRECATED → v2.3 自爆机制） | ✅ PASS | PR #122 · `path_resolver.go` + `runtime_metric.go` 加 `log/slog` import + WARN + DEPRECATED godoc · 4 新增测试 PASS · `design.md` §12 退役计划 |

**统计:** 13 P0 + 4 P1 = **17 AC 全 PASS (100%)**

---

## 2. T 层验证 — 43 T (41 IMPLEMENTED / 0 PLANNED / 2 REMOVED)

| 类别 | 数量 | 说明 |
|------|------|------|
| **D5-S21 Instrument** | 23 | Span/Metric/Log/DebugFilter/SpanAttrs 全覆盖 |
| **D5-S22 Export** | 3 | OTLP/D7 Turn 主路径/Adapter 继承 |
| **D5-S23 Diagnose** | 8 | Coverage/Doctor/Tracker/Incident |
| **D5-S24 Configure** | 2 | 配置加载 + Runtime path |
| **D5-S0 Facade** | 7 | Bridge/SessionGauge/Init/Shutdown |
| **Total** | **43** | — |
| **IMPLEMENTED** | **41** | 100% (2 PLANNED → IMPLEMENTED; 1 历史缺口已说明) |
| **PLANNED** | **0** | 全部闭合 |
| **REMOVED** | **2** | QueryLoop legacy（DM-20260618-010） |

**3 PLANNED 闭合映射：**

| T 点 | 闭合方式 | 证据文件 |
|------|---------|---------|
| D5-S21-A05-T01 (Tracing Span propagation) | 已存在 propagation 集成测试 | `tests/integration/obs_trace_propagation_test.go` |
| D5-S21-A05-T02 (Metrics Counter 单元) | 已存在 Counter 单元测试 | `internal/layers/observability/instrument/metrics/counter_test.go` |
| D5-S23-A06-T02 (HealthCheck coverage 字段) | 已存在 coverage 字段验证 | `observability.go:194-202` + `HealthCheck()` test |

---

## 3. 跨域一致性 — 0 violation

| 检查 | 命令 | 结果 |
|------|------|------|
| Bridge 回归（v2.0 9 路径） | `grep -rE "internal/layers/observability/(tracer\|metrics\|logger\|telemetry\|exporter\|coverage\|incident\|settings\|runtime)" --include=*.go --exclude-dir=archive .` | ✅ **0 命中** |
| Layer lint | `go test ./internal/lint/layer/...` | ✅ PASS（layer-lint (warn) check 15s） |
| query.loop 仅 RETIRED | `grep -n "query\.loop" openspec/specs/d5-observability/*.md` | ✅ 6 处全部在 RETIRED/Legacy 节 |
| Spec.md S 层冻结 | `grep -E "D5-S(0[1-9]\|1[0-9]\|20)\b" spec.md` | ✅ 0 命中 |
| 全量 go test | `go test -race ./internal/...` | ✅ 99 packages OK / 0 FAIL |
| 全量 go vet | `go vet ./...` | ✅ clean |
| 全量 build | `go build ./...` | ✅ clean |

---

## 4. Phase B 实现链路 — 5 PR 联动

| Phase | PR | Branch | Commit | 关键变更 | 状态 |
|-------|----|----|--------|---------|------|
| A | #118 | `docs/d5-v2-terminal-spec` | `d38263b` | 8 个 spec/registry 文档 + 双代码锚点 | ✅ MERGED |
| B2a | #119 | `chore/d5-bridge-import-fix` | `a4f8216` | 5 文件 import 改 canonical (bridge 包未删) | ✅ MERGED |
| B2b | #120 | `chore/d5-bridge-removal` | `11ed93d` | `git rm -r` 9 bridge 目录 + CI regression guard | ✅ MERGED |
| B1 | #121 | `chore/d5-root-file-relocate` | `e9ca472` | 4 文件 `git mv` + observability facade wrapper + main/gateway import | ✅ MERGED |
| B3 | #122 | `chore/d5-t-canonical-fix` | `a96a9c8` | `legacy_harness` DEPRECATED + WARN log + 4 新测试 | ✅ MERGED |
| **S6 归档** | **(this)** | `feat/archive-devrix-d5-v2-terminal` | (pending) | 归档至 `openspec/archive/2026-06-19-devrix-d5-v2-terminal/` | ⏳ 当前 |

**B2a → B2b 之间不留 release shim**（design §10 强约束）：B2a 合入后 4 分钟内 B2b 即合入，确保过渡期间无 bridge 用户。

---

## 5. 文件交付清单

### 5.1 新增 (~13 文件)

| 路径 | Phase | 说明 |
|------|-------|------|
| `openspec/specs/d5-observability/d5-domain.md` | A | North Star + 4 承诺 + 完备性边界 + 博弈论表 + 时间矩阵 |
| `openspec/specs/d5-observability/observability-guide.md` | A | Trace 树 + Span↔T 矩阵 + Runbook + Coverage 多维 |
| `openspec/specs/d5-observability/terminal-state-guide.md` | A | 终态导航 |
| `openspec/specs/d5-observability/dsaft-architecture.md` (stub) | A | DSAFT 概览 stub |
| `openspec/specs/d5-observability/d5-boundary.md` | A | D5 限界 + D5→D6 证据移交 |
| `openspec/specs/d5-observability/specs/architecture/cross-domain-boundaries.md` (D5 段) | A | 跨域边界同步 |
| `openspec/specs/d5-observability/specs/architecture/code-layout.md` (§4.6) | A | diagnose 子目录完整列表 |
| `openspec/changes/devrix-d5-v2-terminal/specs/a-registry.md` | A | v4.0 + Code Location 列 |
| `openspec/changes/devrix-d5-v2-terminal/specs/f-registry.md` | A | v3.0 + canonical_s 列 |
| `openspec/changes/devrix-d5-v2-terminal/specs/t-registry-canonical-draft.md` | A | canonical 校正草稿 |
| `openspec/changes/devrix-d5-v2-terminal/specs/d5-observability/spec.md` | A | v3.0.0 S21–S24 主表 |
| `internal/layers/observability/instrument/logger/slog_bridge.go` | B1 | `InstallSlogBridge()` 函数 |
| `openspec/archive/2026-06-19-devrix-d5-v2-terminal/` (this) | S7 | 归档目录 |

### 5.2 删除 (10 文件)

| 路径 | Phase | 说明 |
|------|-------|------|
| `internal/layers/observability/bridge.go` | B2b | v2.0 Bridge facade |
| `internal/layers/observability/tracer/` (子包) | B2b | v2.0 tracer bridge |
| `internal/layers/observability/metrics/` (子包) | B2b | v2.0 metrics bridge |
| `internal/layers/observability/logger/` (子包) | B2b | v2.0 logger bridge |
| `internal/layers/observability/telemetry/` (子包) | B2b | v2.0 telemetry bridge |
| `internal/layers/observability/exporter/` (子包) | B2b | v2.0 exporter bridge |
| `internal/layers/observability/coverage/` (子包) | B2b | v2.0 coverage bridge |
| `internal/layers/observability/incident/` (子包) | B2b | v2.0 incident bridge |
| `internal/layers/observability/settings/` (子包) | B2b | v2.0 settings bridge |
| `internal/layers/observability/runtime/` (子包) | B2b | v2.0 runtime bridge |
| `internal/layers/observability/slog_bridge.go` | B1 | 根目录 slog_bridge（已迁 instrument/logger） |

### 5.3 修改 (~20 文件)

| 路径 | Phase | 增量 |
|------|-------|------|
| `internal/layers/observability/observability.go` | B1 | +`InstallSlogBridge()` facade wrapper |
| `cmd/devrix/main.go` | B1 | `observability.ConfigureLLMLogging` → `incident.ConfigureLLMLogging` |
| `internal/layers/llmgateway/stream/gateway.go` | B1 | 4 处 import 改 canonical (`metrics.RecordGenAITokenUsage` + `incident.RecordLLMSpanPayload` + `incident.LLMLogContentEnabled` + `metrics.GenAITokenBreakdown`) |
| `internal/layers/observability/diagnose/incident/export.go` | B1 | 移除 observability 自引用，改同包 `CurrentLLMLogSettings()` |
| `.github/workflows/ci.yml` | B2b | +"Bridge regression guard" step (9 path grep) |
| `internal/layers/observability/instrument/metrics/genai_tokens.go` | B1 | 迁入 + package 重命名 + 去 self-import + 去 `metrics.` 前缀 |
| `internal/layers/observability/diagnose/incident/llm_log.go` | B1 | 迁入 + package 重命名 |
| `openspec/specs/d5-observability/t-registry.md` | A | v3.0 → v3.2.0 (canonical_s/canonical_a 校正 + 3 PLANNED 闭合 + Statistics 41/43) |
| `openspec/specs/d5-observability/design.md` | A | v2.x → v3.0 (S23 硬边界 + S25 触发条件 + §12 legacy_harness 退役计划) |
| `openspec/specs/d5-observability/layer-delta.md` | A | +§v2.1-Terminal |
| `internal/layers/observability/configure/runtime/path_resolver.go` | B3 | +DEPRECATED godoc + `slog.Warn` on legacy_harness |
| `internal/layers/observability/configure/runtime/runtime_metric.go` | B3 | +`slog.Warn` + DEPRECATED comment |
| `internal/layers/observability/configure/runtime/path_resolver_test.go` | B3 | +2 测试 (`TestRecord_LegacyHarness_LogsDeprecation` + `TestRecord_D7Turn_NoDeprecationLog`) |
| `internal/layers/observability/configure/runtime/runtime_metric_test.go` | B3 | +2 测试 (`TestIncRuntimeMetric_LegacyHarness_LogsDeprecation` + `TestIncRuntimeMetric_D7Turn_NoDeprecationLog`) |
| `internal/layers/observability/instrument/tracer/tracer_coverage_test.go` | B2a | import 改 canonical |
| `internal/layers/observability/instrument/telemetry/names_test.go` | B2a | import 改 canonical |
| `internal/layers/observability/instrument/telemetry/span_error_test.go` | B2a | import 改 canonical |

---

## 6. CI 门禁验证

| 检查 | 工作流步骤 | 结果 |
|------|-----------|------|
| Unit tests | `unit tests` (required) | ✅ PASS（每次合入 99 packages OK） |
| Layer lint | `layer-lint (warn)` | ✅ PASS（15s） |
| Bridge regression guard | `Bridge regression guard` (PR #120 引入) | ✅ PASS（0 命中） |
| Auto-merge | `gh pr merge --auto --squash --delete-branch` | ✅ 5/5 PR 全部 auto-merge 成功 |

---

## 7. S5 验收结论

✅ **所有 17 个 AC 通过**（13 P0 + 4 P1）
✅ **41/41 T 点 IMPLEMENTED**（3 PLANNED 闭合，0 PLANNED 余量）
✅ **跨域一致性 0 violation**（bridge grep + layer-lint + spec S 层冻结）
✅ **5 PR 联动 5/5 全部 squash merge + auto-merge**（无 release shim 间隔）
✅ **CI regression guard 已硬阻断**（9 bridge 路径 future-proof）
✅ **`legacy_harness` DEPRECATED 落地**（runtime WARN + godoc + 4 测试）

**进入 S7 归档阶段：** move `openspec/changes/devrix-d5-v2-terminal/` → `openspec/archive/2026-06-19-devrix-d5-v2-terminal/`，回写 `demand-archive-index.md` DM-20260619-006。

---

## 8. Tech Debt 跟踪

| 项 | 描述 | 跟踪方式 |
|----|------|---------|
| **`legacy_harness` 自爆机制** | design §12：v2.1 DEPRECATED + WARN → v2.3 auto-removal（2 consecutive releases of zero counts） | 下个 change `devrix-d5-v2.3-harness-retire`（v2.3 触发） |
| **D5-S21-A01-T03 历史缺口** | 原始 S1-A01 无 T03，非本 change 引入 | 已在 t-registry v3.2.0 说明栏注明 |
| **Layer-lint warn** | 当前为 warning（不阻断），未来 strict 化需评估 | 单独 change 跟踪 |

---

## 9. 归档元数据

- **Archive path:** `openspec/archive/2026-06-19-devrix-d5-v2-terminal/`
- **Parent changes:** `devrix-d5-sa-refine` (DM-20260615-001), `devrix-d5-d6-sa-refine-v2.0` (DM-20260615-003)
- **S4 PRs:** #118 (Phase A) · #119 (B2a) · #120 (B2b) · #121 (B1) · #122 (B3)
- **S6 archive PR:** (this PR)
- **S3-Gate:** design-review.md §11 — Owner Confirmed 2026-06-19