---
demand-id: DM-20260626-004
title: D7 编排层 turn/ 包合并到 sessionorchestrator/ (v6.0.0 Step 3 落地)
priority: P1
status: S1_Proposal
dsaft_domain: architecture
created: 2026-06-26
---

# D7 turn/ 包合并到 sessionorchestrator/

## 1. 背景

`devrix-d7-six-s-simplification` (DM-20260626-001) 在 v6.0.0 域升级中，把 D7 编排层的 14 S 博弈角色精简为 6 S + 1 横切。S2 SessionOrchestrator 角色定义为 **Mediator + Turn Leader + Error Recovery**，但代码侧 `orchestration/turn/` 仍作为独立子包，与 `sessionorchestrator/` 平级存在：

```
v6.0.0 follow-up #1 已落地：
  execute/ + learn/ → mups/        ✅ (DM-20260626-002 / PR #216)
  hardening/ 横切包                ✅ (DM-20260626-003 / PR #218+#219)
本次 (#3)：
  turn/ + autoclose → sessionorchestrator/   📋 本 change
```

DM-20260626-001 (PR #215) 之后，`autoclose.go` 已合并到 `sessionorchestrator/`（35 文件），但 `turn/` 仍独立（25 文件 6467 行）：

| 当前包路径 | 行数 | 应归 S 层 | 6 S 对齐目标 |
| ---------- | ---- | --------- | ------------ |
| `orchestration/turn/` (25 .go) | 6467 | S2 SessionOrchestrator | `orchestration/sessionorchestrator/` |
| `orchestration/sessionorchestrator/autoclose.go` | ~200 | S2 SessionOrchestrator | （已在 sessionorchestrator/，本次不动） |

## 2. 问题陈述

虽然 v6.0.0 spec/code 语义层已对齐 6 S（execute+learn→mups 已 #2 完成，hardening/ 横切已 #3 完成），但 S2 SessionOrchestrator 角色仍被两个物理包拆开：

**具体后果：**

1. **目录命名与博弈角色不对齐**：v6.0.0 spec 说"S2 = Mediator+Turn Leader"是单一博弈角色，但代码侧 `turn/` 是独立 Go 包，调用方需要 `import orchestration/turn` 才能拿到 RunTurn 主循环入口
2. **`turn/orchestrator.go` (1462 行) 是 DefaultOrchestrator 主体**，紧耦合 LLM gateway + compression + recovery，但与 `sessionorchestrator/orchestrator.go` (SessionOrchestrator 顶层) 物理隔离，跨包调用链长
3. **`turn_tools.go` 已存在 `sessionorchestrator/` 内**（这是 #1 已并入的部分），但 `turn/orchestrator.go` 还在 `turn/`，导致 sessionorchestrator/ 包内部职责不完整
4. **bootstrap 14 wire 收口受阻**：wire_coordinator.go 中需分别 wire `turn.NewOrchestrator` + `sessionorchestrator.NewSessionOrchestrator` 两个独立包，未来 6 S 全部落地后还需再做一次 wire 收敛
5. **`turn/` 包导出 11 个核心类型**（DefaultOrchestrator / OrchestratorDeps / SubTurnRunner / GatewayInvoker / CompressionSummarizer / OrchestratorOption / TurnOrchestrator 接口 等），但调用方跨包 import 频繁（12 个 importer），打破 S2 单包封装

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `internal/layers/orchestration/turn/` 物理目录消失（25 .go 全部 git mv） | P0 |
| AC2 | 25 个原 turn/ .go 文件全部位于 `internal/layers/orchestration/sessionorchestrator/turn_*.go` 或 `sessionorchestrator/subturn*.go` 等命名（保留 `sessionorchestrator/` 内已有文件 + 新增 turn 相关文件），`package sessionorchestrator` 声明一致 | P0 |
| AC3 | 全仓 `grep -rln "orchestration/turn\"" internal/ cmd/` 返回 0 命中（import path 全部替换） | P0 |
| AC4 | 全仓 `grep -rln "turn\.NewOrchestrator\|turn\.OrchestratorDeps\|turn\.DefaultOrchestrator\|turn\.TurnOrchestrator\|turn\.SubTurnRunner\|turn\.GatewayInvoker\|turn\.CompressionSummarizer"` 返回 0 命中（包外引用全部更新为 sessionorchestrator. 前缀） | P0 |
| AC5 | `go build ./...` PASS（0 错误） | P0 |
| AC6 | `go vet ./...` PASS（0 警告） | P0 |
| AC7 | `go test ./internal/layers/orchestration/... -race -count=1` 全部 PASS（23/23 包，与 hardening 落地后 baseline 持平） | P0 |
| AC8 | `internal/bootstrap/wire_coordinator.go` + `internal/bootstrap/turn_*.go` (8 bootstrap 文件) import path 同步更新 | P0 |
| AC9 | `decisionplanning/llm_decomposer.go` + `llm_decomposer_test.go` import path 同步更新 | P1 |
| AC10 | `sessionorchestrator/turn_tools.go` + `turn_tools_test.go` 已引用 hardening + turn，需更新内部 turn.X → sessionorchestrator.X 引用 | P0 |
| AC11 | 新增 4 P0 T（D7-S2-A01-T01 mups/execute package exists 等同类编号）：`D7-S2-A50-T01`（turn/ 物理删除）+ `D7-S2-A50-T02`（sessionorchestrator/ 包扩展到 ~60 文件）+ `D7-S2-A50-T03`（零残留 import）+ `D7-S2-A50-T04`（build/test/vet 全绿）全部 IMPLEMENTED | P1 |
| AC12 | LP-1（Bayesian reputation）/ LP-2（Memory 3 通道）/ LP-5（Cross-session traceability）路径 0 变化（仅物理迁移） | P0 |
| AC13 | `verify-archive.sh devrix-d7-6s-package-merge` 12/12 PASS 0 FAIL | P1 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | devrix-d7-six-s-simplification (DM-20260626-001) S7_Archived，spec 层已对齐 6 S |
| 依赖 | devrix-d7-mups-package-migration (DM-20260626-002) S7_Archived，mups/ 子树已落地 |
| 依赖 | devrix-d7-hardening-cross-cutting (DM-20260626-003) S7_Archived，hardening/ 横切包已落地（receiver methods 现在留在 turn/，迁 sessionorchestrator/ 后调用 hardening 不变） |
| 约束 | **不允许** 改任何函数签名、行为、对外接口 — 纯物理迁移 + import path 替换 |
| 约束 | **不允许** 改 type 名字（DefaultOrchestrator / OrchestratorDeps / SubTurnRunner 等保留），避免调用方大改 |
| 约束 | LP-1 / LP-2 / LP-5 路径 0 变化 |
| 约束 | `turn/orchestrator.go` + `orchestrator_test.go` 内部对 `turn.XXX` 的引用必须改为 `sessionorchestrator.XXX`（同包内引用是 bare name，无影响） |
| 约束 | `turn/exit_reason.go` + `verdict_to_exit_reason.go`（72 + 49 = 121 行）临时随 turn/ 迁 sessionorchestrator/，由后续 follow-up #4（`devrix-d7-6s-verify-promotion`）promote 到 `executionflow/verify/` |
| 约束 | hardening/ 已落地，`turn/recovery.go` + `turn/recovery_test.go` 是 hardening 落地后的子集（receiver methods 留 turn/，迁 sessionorchestrator/ 时 0 变化） |
| 约束 | `multiagent/` 域内任何同名 `execute` / `turn` 不在本 change 范围（不同域不同职责） |

## 5. 变更范围

### 新增（git mv 后归类为迁移）

- `internal/layers/orchestration/sessionorchestrator/` 新增 25 个 .go 文件（git mv 自 `turn/`）：
  - `orchestrator.go` (1462 行, DefaultOrchestrator 主体)
  - `orchestrator_test.go` (2100 行)
  - `orchestrator_toolcap_test.go` (263 行)
  - `compression_summarizer.go` + `compression_summarizer_test.go` (98 + 144 = 242 行)
  - `contracts.go` (142 行, TurnOrchestrator 接口)
  - `doc.go` (17 行, 包说明, 需更新为 sessionorchestrator 语境)
  - `exit_reason.go` (72 行, 14 ExitReason 临时留 sessionorchestrator/, 等 #4 promote)
  - `fake_gateway_test.go` (40 行)
  - `focus_hint.go` (8 行)
  - `llm.go` + `llm_test.go` (102 + 495 = 597 行, GatewayInvoker + LLM 接口)
  - `recovery.go` + `recovery_test.go` (84 + 130 = 214 行, receiver methods 子集)
  - `resolve_await.go` (8 行)
  - `runturn_main_path_test.go` (38 行)
  - `subturn.go` (380 行, SubTurnRunner)
  - `subturn_fork_test.go` (135 行)
  - `subturn_test.go` (466 行)
  - `tool_stream.go` + `tool_stream_test.go` (30 + 63 = 93 行)
  - `tracing.go` (44 行)
  - `verdict_to_exit_reason.go` + `verdict_to_exit_reason_test.go` (49 + 97 = 146 行, 等 #4 promote)

### 修改

- `internal/layers/orchestration/sessionorchestrator/turn_tools.go` + `turn_tools_test.go` — 内部 `turn.XXX` 引用改为 `sessionorchestrator.XXX`（2 文件）
- `internal/bootstrap/wire_coordinator.go` — `orchestration/turn` import path 替换
- `internal/bootstrap/turn_wiring.go` — 同上
- `internal/bootstrap/turn_adapter.go` — 同上
- `internal/bootstrap/turn_adapter_test.go` — 同上
- `internal/bootstrap/turn_adapter_persist_test.go` — 同上
- `internal/bootstrap/turn_adapter_permission_test.go` — 同上
- `internal/bootstrap/turn_adapter_surface_test.go` — 同上
- `internal/bootstrap/context_engine.go` — 同上
- `internal/bootstrap/context_engine_builder.go` — 同上
- `internal/bootstrap/plan_llm_completer.go` — 同上
- `internal/layers/orchestration/decisionplanning/llm_decomposer.go` + `llm_decomposer_test.go` — `orchestration/turn` import path 替换
- `openspec/specs/d7-orchestration/d7-domain.md` v2.1.0 → v2.2.0 §① S2 SessionOrchestrator 章节包路径描述更新
- `openspec/specs/d7-orchestration/design.md` v4.1.0 → v4.2.0 §① S2 SessionOrchestrator 包路径描述更新
- `openspec/specs/d7-orchestration/t-registry.md` v4.3.0 → v4.4.0（新增 D7-S2-A50-T01..T04）
- `openspec/t-registry.md` (root) v5.3.0 → v5.4.0（新增 DM-20260626-004 增量条目）

### 删除

- `internal/layers/orchestration/turn/` 整目录删除（git mv 已自动处理）

### 不变更

- D7 14 S → 6 S 文档语义保持不变
- D7 5 个新 P0/P1 Span emit 路径 0 变化
- `internal/shared/types/` 跨域类型不动
- `internal/layers/multiagent/` 域不动
- `hardening/` 横切包 0 变化（receiver methods 现在随 turn/ 迁入 sessionorchestrator/，但 import hardening 不变）
- `escape/circuit_breaker.go` 不动（V5 EscapeEngine 核心机制）
- `sessionorchestrator/autoclose.go` 不动（已在 sessionorchestrator/）
- 4 个其他 follow-up PR 范围（hardening-cross-cutting 已 #3 完成 / 6s-verify-promotion / 6s-observe-merge / 6s-bootstrap-slim）不在本次范围

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **sessionorchestrator/ 包扩大至 ~60 文件 ~15000 行** | 中 | v6.0.0 设计目标 — S2 是 Mediator+Turn Leader 复合角色；接受包大小换取角色内聚；Go 增量编译无性能影响 |
| **同包内命名冲突** (turn.DefaultOrchestrator vs sessionorchestrator.SessionOrchestrator) | 低 | type 名不同（DefaultOrchestrator vs SessionOrchestrator），无冲突；详细比对见 design.md §3 Decision 1 |
| **turn.X 跨包调用 12 处** | 中 | 全仓 `grep -rln "turn\."` 列出引用文件，逐一替换为 sessionorchestrator.X；同包内 bare name 引用 0 影响 |
| **测试 fixture 间接引用 turn 包** | 中 | turn/fake_gateway_test.go 自身测试文件随迁；bootstrap/turn_adapter_*.go (5 测试) + decisionplanning/llm_decomposer_test.go 引用 turn 包，需同步更新 import path |
| **PR 合并顺序冲突** | 低 | 本次 change 依赖 #1+#2+#3 都已 S7_Archived，无外部冲突 |
| **CI 镜像缓存** | 低 | 删除旧 turn/ 目录后强制 re-build；CI 单测 100% PASS 是硬门禁 |
| **receiver methods (compressMessagesForRecovery + invokeStreamWithRecovery) 在 sessionorchestrator 内** | 低 | hardening/ 落地时已确认 receiver 紧耦合 `*DefaultOrchestrator`；迁 sessionorchestrator/ 后 receiver 类型不变（仍为 `*DefaultOrchestrator`），调用 hardening.IsContextLengthError 不变 |
| **exit_reason.go + verdict_to_exit_reason.go 临时留 sessionorchestrator/** | 低 | 这 2 个文件 (121 行) 临时随 turn/ 迁入 sessionorchestrator/，后续 #4 (`devrix-d7-6s-verify-promotion`) 再 promote 到 `executionflow/verify/`；本次范围明确标注 |
| **IDE/Goland 索引需要重新同步** | 极低 | 文档同步说明 + README 更新 |

## 7. 调研依据（pre-S2 调研结果）

S1 阶段已完成的 import 关系调研（2026-06-26）：

### 7.1 `orchestration/turn/` 包内部统计

- **25 个 .go 文件，6467 行**（含测试）
- **11 个核心导出 type**：`DefaultOrchestrator` + `OrchestratorDeps` + `OrchestratorOption` + `TurnOrchestrator` 接口 + `SubTurnRunner` + `SubTurnConfig` + `GatewayInvoker` + `LLMInvokerDeps` + `CompressionSummarizer` + `CompressionSummarizerDeps` + `PreparedTurnAdapter`
- **6 个核心导出函数**：`NewOrchestrator` + `NewSubTurnRunner` + `NewGatewayInvoker` + `NewCompressionSummarizer` + `NewPreparedTurnAdapter` + `FormatToolResultContentForLLM`

### 7.2 `orchestration/turn/` 外部 importer（12 处）

```
internal/bootstrap/wire_coordinator.go
internal/bootstrap/turn_adapter_persist_test.go
internal/bootstrap/turn_wiring.go
internal/bootstrap/turn_adapter_permission_test.go
internal/bootstrap/turn_adapter_test.go
internal/bootstrap/context_engine_builder.go
internal/bootstrap/context_engine.go
internal/bootstrap/plan_llm_completer.go
internal/bootstrap/turn_adapter_surface_test.go
internal/bootstrap/turn_adapter.go
internal/layers/orchestration/decisionplanning/llm_decomposer_test.go
internal/layers/orchestration/decisionplanning/llm_decomposer.go
internal/layers/orchestration/sessionorchestrator/turn_tools_test.go
internal/layers/orchestration/sessionorchestrator/turn_tools.go
```

注：`sessionorchestrator/turn_tools.go` + `_test.go` 已 import turn 包（2 处），本次范围包含其内部 turn.X → sessionorchestrator.X 引用更新（AC10）。

### 7.3 与 sessionorchestrator/ 同名冲突检查

| turn/ 导出 | sessionorchestrator/ 导出 | 冲突？ |
| ---------- | ------------------------- | ------ |
| `DefaultOrchestrator` | `SessionOrchestrator` | ❌ 不冲突（不同 type） |
| `NewOrchestrator` | `NewSessionOrchestrator` | ❌ 不冲突（不同函数） |
| `TurnOrchestrator` (接口) | 无 | ❌ 不冲突 |
| `OrchestratorDeps` | 无 | ❌ 不冲突 |
| `SubTurnRunner` | 无 | ❌ 不冲突 |
| `GatewayInvoker` | 无 | ❌ 不冲突 |
| `CompressionSummarizer` | 无 | ❌ 不冲突 |
| `PreparedTurnAdapter` | 无 | ❌ 不冲突 |
| `FormatToolResultContentForLLM` | 无 | ❌ 不冲突 |
| `OrchestratorOption` | `OrchestratorOption` | ✅ **同名 type alias？需 design.md Decision 2 详查** |
| `NewOrchestratePath` (turn 没有，sessionorchestrator 有) | - | - |

`OrchestratorOption` 同名需要 design.md 进一步确认：检查 sessionorchestrator/ 是否已定义同名 type，turn/ 是否 import sessionorchestrator 或反之。

## 8. 关联

- **前置：** `devrix-d7-six-s-simplification` (DM-20260626-001) S7_Archived
- **前置：** `devrix-d7-mups-package-migration` (DM-20260626-002) S7_Archived
- **前置：** `devrix-d7-hardening-cross-cutting` (DM-20260626-003) S7_Archived
- **后续（其他 3 个 follow-up）：**
  - `devrix-d7-6s-verify-promotion` (DM-20260626-005 / PLANNED) — exit_reason.go + verdict_to_exit_reason.go 从 sessionorchestrator/ promote 到 executionflow/verify/
  - `devrix-d7-6s-observe-merge` (DM-20260626-006 / PLANNED) — observe/orchtypes → decisionplanning/
  - `devrix-d7-6s-bootstrap-slim` (DM-20260626-007 / PLANNED) — wire_coordinator.go 14 wire → 6 wire（依赖 #4 #5 #6 全部完成）