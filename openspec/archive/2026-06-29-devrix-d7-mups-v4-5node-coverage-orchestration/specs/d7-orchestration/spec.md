# Project Specification Delta — D7 MUPS 5-node Span 全覆盖 + 目录结构治理

**Change ID:** devrix-d7-mups-v4-5node-coverage-orchestration
**Demand ID:** DM-20260625-019
**Delta Type:** MODIFIED (D7 5-node Span 全覆盖 + mups/execute/ + mups/learn/ 目录治理)
**SOT:** `openspec/specs/d7-orchestration/`

---

## 1. 修改总览

| 内容 | 文件 | 类型 | 行为变化 |
|------|------|------|----------|
| 1. 5 节点 Span 注册到 coverage registry | `observability/diagnose/coverage/registry_test.go` + `orchestration/sessionorchestrator/spans.go` (PR #235) | P0 | 5 节点 Span 在 coverage 报告/dashboard 可视，Jaeger 可按 5 节点检索 |
| 2. D7_MUPS_Pipeline 根 Span | `orchestration/sessionorchestrator/orchestrate_path.go` + `observability/instrument/telemetry/names.go` (PR #235) | P0 | Jaeger 中 mupsSpan.parent == orchSpan.SpanContext（端到端 5 节点串联）|
| 3. mups/execute/ 5 个 channel_ 前缀清理 | `orchestration/mups/execute/*` (PR #236) | P0 | 目录浏览无 channel_ 噪音 |
| 4. mups/learn/ 4 subpackage 拆分 | `orchestration/mups/learn/{asset,memory,reputation,prior}/` (PR #236) | P0 | 4 子领域有清晰边界 |
| 5. import cycle 打破（DefaultPendingMaxRetries 上提 asset/）| `orchestration/mups/learn/asset/` (PR #236) | P0 | 无 import cycle |

---

## 2. ADDED Requirements (No ADDED — only MODIFIED; 轻量变更)

> 本 Change 是**目录结构 + Span 注册**的轻量变更（per `openspec/specs/project/architecture-design.md §1.3 范围与详细度裁剪`），不引入新契约 / 新 Scenario。详尽验收对照见 `acceptance-report.md §3`。

### Requirement: 5 节点 Span 覆盖率 ≥ 100% ✅ IMPLEMENTED

`openspec/specs/d7-orchestration/observability.md` MUST 明确列出 5 节点 Span（TaskGraph/Executor/Channel/Memory/Anomaly）在 coverage registry 中的注册状态为 INSTRUMENTED。

**Priority:** P0
**T:** D7-S8-A30-T01
**Design:** `design.md §2 Implementation Notes`

<!-- T: D7-S8-A30-T01 -->

---

### Requirement: D7_MUPS_Pipeline 根 Span 端到端串联 ✅ IMPLEMENTED

D7 编排层 MUST 在 `orchestrate_path.go` 中 emit D7_MUPS_Pipeline 根 Span，5 节点子 Span（taskgraph.synthesize / executor.select / channel.route / memory.persist / system.anomaly_detect）作为其 children。

**Priority:** P0
**T:** D7-S8-A31-T01
**Design:** `design.md §2 Implementation Notes`

<!-- T: D7-S8-A31-T01 -->

---

### Requirement: mups/ 目录结构治理（pure physical migration） ✅ IMPLEMENTED

`internal/layers/orchestration/mups/` 子树 MUST 满足：
- `execute/` 子目录无 `channel_` 前缀
- `learn/` 子目录拆为 4 subpackage：`asset/` + `memory/` + `reputation/` + `prior/`
- 无 import cycle（DefaultPendingMaxRetries 上提至 `learn/asset/`）
- 0 函数签名变化（pure physical migration）

**Priority:** P0
**T:** D7-S9-A30-T01 + D7-S12-A30-T01 + D7-S12-A30-T02
**Design:** `design.md §3 Verification`

<!-- T: D7-S9-A30-T01 -->
<!-- T: D7-S12-A30-T01 -->
<!-- T: D7-S12-A30-T02 -->

---

## 3. 行为不变保证

- **0 函数签名变化**：git diff --stat 0 变化，pure physical migration
- **23 orchestration packages -race PASS**：0 FAIL
- **6 T 点 100% DONE**：PR #235 + #236 squash merge 2026-06-26

---

## 4. 跨域边界影响

| 域 | 影响 | 备注 |
|----|------|------|
| D5 observability | coverage registry 期望列表加 5 Op | 由 §2.1 §registry_test.go 验证 |
| D7 orchestration | sessionorchestrator + mups/ 物理迁移 | 22/22 packages -race PASS |

---

## 5. Out of Scope

- 修改 5 节点业务逻辑（仅注册 + 目录治理）
- 修改 5 节点 Span 命名（保留 runtime 字面量）
- 修改 `mups/observe/` + `mups/plan/`（这两个目录已 v6.0.0 收敛）
- 修改 sessionorchestrator 外部接口（pure physical migration）

---

## 6. 关联变更

- **前置**：`devrix-d7-six-s-simplification` (DM-20260626-001) v6.0.0 域升级
- **前置**：`devrix-d7-mups-v4-phase5-learn` (DM-20260623-003) Learn 节点升格
- **前置**：`devrix-d7-mups-package-migration` (DM-20260626-002) 物理包路径迁移
- **后续**：D7 维护期可基于 4 subpackage 边界添加新能力
