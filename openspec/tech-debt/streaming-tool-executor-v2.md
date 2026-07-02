# Tech Debt: StreamingToolExecutor 二期对齐（clawcode 参照）

**来源：** clawcode `src/services/tools/StreamingToolExecutor.ts` vs Devrix `query/streaming_executor.go`
**主路径：** DM-20260610-012 QueryLoop（v1 基础版已交付）
**承载 change:** **DM-20260702-009 (D2 Tool Input-Aware Concurrency + Auto-Mode Security Classifier)** — 4 项关闭, 2 项保留
**状态:** 4/6 CLOSED (TD-STE-01/02/03/06) + 2/6 保留 (TD-STE-04/05, P2)
**优先级:** P0 (CLOSED 项) + P2 (保留项)

## 状态总览

| TD ID | 标题 | 状态 | 关闭路径 |
|-------|------|------|----------|
| TD-STE-01 | 混合批次调度 | **CLOSED** by DM-20260702-009 **T18** (partitionToolCalls) | 走 PR-B |
| TD-STE-02 | Bash sibling abort | **CLOSED** by DM-20260702-009 **T26** (siblingAbortController) | 走 PR-F |
| TD-STE-03 | fallback 时 discard 在途工具 | **CLOSED** by DM-20260702-009 **T27** (StreamingToolExecutor.Discard) | 走 PR-F (依赖 TD-QL-03 CLOSED, DM-20260618-010) |
| TD-STE-04 | 工具 progress 流 | 保留 P2 | OOS-NEW-7 |
| TD-STE-05 | synthetic error 统一 | 保留 P2 | OOS-NEW-8 |
| TD-STE-06 | ConcurrencySafe 注册表 | **CLOSED** by DM-20260702-009 **T16-T17** (per-input IsConcurrencySafe) | 走 PR-A |

## 背景

Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

## 现状 vs 目标 (历史快照, 已被本 doc 关闭)

| 能力 | Devrix v1 | clawcode | 目标 / 状态 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | **TD-STE-01 → CLOSED T18** |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | **TD-STE-02 → CLOSED T26** |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | **TD-STE-03 → CLOSED T27** |
| 工具 progress 中途 yield | agent tool stream only | `pendingProgress` 即时 yield | TD-STE-04 保留 P2 |
| 合成 error 类型 | permission/exec | sibling_error / interrupted / streaming_fallback | TD-STE-05 保留 P2 |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | **TD-STE-06 → CLOSED T16-T17** |

---

## CLOSED 项 — 关闭记录

### TD-STE-01: 混合批次调度 — CLOSED by T18

**参考:** clawcode `canExecuteTool` + `processQueue`

**关闭路径:** DM-20260702-009 T18 partitionToolCalls 改造
- `internal/bootstrap/turn_adapter.go:277` 改造为 `partitionToolCalls` batch 模式
- batch 间串行 (LLM 顺序保留) + batch 内并发 (errgroup, 9 并发阈值)
- 仿 clawcode `src/services/tools/toolOrchestration.ts:84-118`

**关闭时间:** 计划 W1 D3-D5 (PR-B)

**回归基线:** `review50_e2e_concurrent_test.go` (T19) — 50 read_file 拆 ~10 batch, 总 wall time < 串行 / 3

### TD-STE-02: Bash sibling abort — CLOSED by T26

**参考:** clawcode `createChildAbortController(toolUseContext.abortController)`

**关闭路径:** DM-20260702-009 T26 BashTool siblingAbortController 集成
- `internal/layers/contextengine/enforce/tools/bash/sibling_abort.go` 新建
- 仅 abort 同 batch 并行 Bash 兄弟, **不** abort 父 QueryLoop turn
- 兄弟 Bash 返 synthetic `tool_result`: `Cancelled: parallel tool call errored`

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `sibling_abort_test.go` — mock 双 Bash, 第一个 error → 第二个 cancelled

### TD-STE-03: discard on fallback — CLOSED by T27

**触发:** QueryLoop fallback model 切换前 (依赖 TD-QL-03)

**关闭路径:** DM-20260702-009 T27 StreamingToolExecutor.Discard() + fallback 路径 wiring
- `internal/bootstrap/streaming_executor.go` 新建 — Discard() 方法
- `internal/bootstrap/discard_on_fallback.go` 新建 — QueryLoop fallback 路径 wiring
- 在途/queued 工具注入 `streaming_fallback` synthetic result
- 新 iteration 使用 fresh executor

**前置依赖:** TD-QL-03 (DM-20260618-010) — 已 CLOSED, 不再阻塞

**关闭时间:** 计划 W3 D1-D2 (PR-F)

**回归基线:** `discard_on_fallback_test.go` — fallback 路径无 orphan tool_use

### TD-STE-06: ConcurrencySafe 注册表 — CLOSED by T16-T17

**参考:** clawcode Tool interface 35 字段中 `isConcurrencySafe(input)`

**关闭路径:** DM-20260702-009 T16-T17 ToolSurface v4 + 19 工具默认实现
- `internal/shared/contracts/tool_surface_v4.go` 新建 — interface 扩展
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags_v2.go` 新建 — 19 工具默认
- 19 surface 加 `IsConcurrencySafe(input []byte) bool` 默认实现 (跟 clawcode TOOL_DEFAULTS 一致)

**关闭时间:** 计划 W1 D1-D2 (PR-A)

**回归基线:** `surface_metadata_gate_test.go` 加 1 case (AC8: 0 silent default)

---

## 保留项 (P2) — 走后续 change

### TD-STE-04: 工具 progress 流（P2）

- 长运行工具（bash、agent tool）可通过 context 注入 progress callback
- Executor 将 progress 作为 `tool_progress` 事件经 emit 上行（IM 可选展示）
- 走 OOS-NEW-7

### TD-STE-05: synthetic error 统一（P2）

| Reason | tool_result 语义 |
|--------|------------------|
| `sibling_error` | 并行兄弟失败取消 |
| `user_interrupted` | 用户拒绝/ESC |
| `streaming_fallback` | 模型 fallback 丢弃在途 |

- 走 OOS-NEW-8

---

## 详细字段 (历史记录, 关闭项参考)

### TD-STE-01 (CLOSED) 验收

- 单测：`read_file×2 + bash` → 两个 read 并行，bash 等 read 完成后执行
- 单测：仅 `bash×2` → 严格串行

### TD-STE-02 (CLOSED) 验收

- 集成测试 mock 双 Bash，第一个 error → 第二个 cancelled
- 单测: 第一个 error 后父 turn 仍继续 (不 cancel 父 ctx)
- 单测: 非 Bash 工具不被 abort (e.g. read_file 在同 batch 不受影响)

### TD-STE-03 (CLOSED) 验收

- 单测 fallback 路径无 orphan tool_use
- 单测: 无 fallback 时不触发 discard (无行为变化)

### TD-STE-06 (CLOSED) 验收

- 19 工具加 surface 默认 IsConcurrencySafe 实现
- `surface_metadata_gate_test.go` AC8 case PASS (0 silent default)

### TD-STE-04 (P2 保留) 验收

- 长运行工具调 progress callback → IM 收到 tool_progress 事件
- 单测: bash 跑 5s, 每 1s emit 一次 progress

### TD-STE-05 (P2 保留) 验收

- 3 种 synthetic error 类型在 tool_result 中可区分
- 单测: sibling_error / user_interrupted / streaming_fallback 各自 tool_result 语义

---

## 不在此 tech-debt

- QueryLoop 413/fallback 主链 → `queryloop-error-recovery.md` TD-QL-01~03 (TD-QL-03 已 CLOSED DM-20260618-010)
- Wave Worker cancel → DM-007 §12
- Background task stop → DM-20260611-009

## T 层登记 (关闭路径映射)

| T ID | Given-When-Then | 优先级 | 状态 |
|-------|-----------------|--------|------|
| D2-S8-T01 (DM-20260702-009 T18) | Given read×2+bash 同批 When ExecuteBatch Then read 并行且 bash 最后 | P0 | IMPLEMENTED (计划 W1 D3-D5) |
| D2-S8-T02 (DM-20260702-009 T26) | Given bash 并行首错 When sibling abort Then 第二 bash 合成 cancelled | P1 | IMPLEMENTED (计划 W3 D1-D2) |
| D2-S8-T03 (DM-20260702-009 T27) | Given fallback When discard Then 无 orphan tool_use | P1 | IMPLEMENTED (计划 W3 D1-D2) |
| D7-S9-A50-T16-T17 (DM-20260702-009) | Given 19 工具 When IsConcurrencySafe default Then per-input 决策 + 0 silent default | P0 | IMPLEMENTED (计划 W1 D1-D2) |
