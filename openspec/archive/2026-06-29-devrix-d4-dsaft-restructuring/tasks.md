# Tasks: devrix-d4-dsaft-restructuring (DM-20260629-004)

**Change ID:** `devrix-d4-dsaft-restructuring`
**Demand ID:** DM-20260629-004
**Status:** S2_Tasks
**Total Tasks:** ~34
**Total AC:** ~28
**Template:** `devrix-d3-dsaft-restructuring` tasks.md (DM-20260629-003 S7_Archived)

---

## §0 任务索引（~34 T / 8 子 Change）

| 子 Change | PR | T 范围 | 工作量 |
|---|---|---|---|
| **#0** legacy-cleanup | PR-1 | T01-T05 | 1 PR / 1 天 |
| **#1** god-fn-split pt1 | PR-2 | T06-T09 | 1 PR / 1 天 |
| **#1** god-fn-split pt2 | PR-3 | T10-T13 | 1 PR / 1 天 |
| **#2** registry-sync | PR-4 | T14-T17 | 1 PR / 1 天 |
| **#3** value-flow-rename | PR-5 | T18-T21 | 1 PR / 1 天 |
| **#4** span-coverage | PR-6 | T22-T27 | 1 PR / 1 天 |
| **#5** boundary-decision | PR-7 | T28-T31 | 1 PR / 1 天 |
| **S7_Archive** | PR-8 | T32-T34 | 1 PR / 1 天 |
| **总计** | 8 PR | **~34 T** | **8-10 天** |

---

## §1 子 Change #0 legacy-cleanup（PR-1, T01-T05）

### T01 建立 `orchtypes/` 治理包基础

- New dir `internal/layers/multiagent/orchtypes/`
- 移动 `multiagent/spans.go` (21 LOC) → `orchtypes/spans.go`
- `package orchtypes` 声明
- `coverage.RegisterProvider()` 钩子保留

### T02 inline `WorkerEngine` wrapping

- 删除 `internal/layers/multiagent/run/worker_engine.go` (44 LOC)
- `WorkerEngine` struct + `NewWorkerEngine()` inline 到 `provision/factory.go` 私有函数 `newWorkerEngine()`
- `factory.go` L112/L119 call site 切换
- `run/worker_engine_test.go` 33 LOC 测试同步迁移到 `provision/factory_test.go` 或删除 inline 版本测试

### T03-T04 `ExecutorMetricsSnapshot` / `ForkerMetricsSnapshot` 审计 + 保留

- 实际只被 own package test 使用 — 但 JSON schema 是 D5 contract，**保留**

### T05 dead exported 实际清单

| Symbol | 状态 | 备注 |
|--------|------|------|
| `agent.go::Creator` | **KEEP** | run.Impl 字段依赖 + 测试 stub 实现 |
| `WorkerEngine` (struct + fn) | **DELETE in T02** | 纯 delegating wrapper |
| `ExecutorMetricsSnapshot` | **KEEP** | D5 JSON schema |
| `ForkerMetricsSnapshot` | **KEEP** | D5 JSON schema |
| `EngineBuilder` interface | **KEEP** | factory 字段依赖 |
| `NewAgentFactory` | **KEEP** | 测试 + bootstrap 多处 caller |
| `CLIAgentTool/CLISession` | **KEEP** | bootstrap + tests 多处 caller |
| `CursorAgentTool` | **KEEP** | bootstrap + tests 多处 caller |
| `multiagent/contracts.go` shim | **KEEP + DEPR标注** | sessionagents/manager.go + 5 测试依赖 |
| `NewAgentObserverChain` | **KEEP** | shim use |
| `MetaToolCallID` const | **KEEP** | runtime reference |
| `AgentState*` consts | **KEEP** | all over |
| `Mode*` consts | **KEEP** | all over |

实际净删除 = **WorkerEngine struct + WorkerEngine related test** ≈ **60 LOC**

---

## §2 子 Change #1 god-fn-split pt1（PR-2, T06-T09）

### T06 拆 `external/cli_adapter.go` 466 LOC

- → `external/cli_session.go`
- → `external/cli_execute.go`

### T07-T08 内容切分（见 `proposal.md §3`）

### T09 验证

- cli_session.go <300 LOC
- cli_execute.go <250 LOC
- `go test ./internal/layers/multiagent/external/... -race` PASS

---

## §3 子 Change #1 god-fn-split pt2（PR-3, T10-T13）

### T10 拆 `external/cursor_adapter.go` 410 LOC

### T11-T13 内容切分 + 验证

---

## §4 子 Change #2 registry-sync（PR-4, T14-T17）

### T14 18 F 路径全替换

| 原路径 | 新路径 |
|--------|--------|
| `agent/agent.go` (S11) | `provision/factory.go` |
| `factory/factory.go` (S11) | `provision/factory.go` |
| `agent/lifecycle.go` (S12) | `run/lifecycle.go` |
| `agent/agent.go` (S12) | `run/agent.go` |
| `agent/perm_gate.go` (S12) | `run/perm_gate.go` |
| `agent/forkjoin.go` (S13) | `run/forkjoin.go` |
| `sessionview/sessionview.go` (S13) | `isolate/sessionview.go` |
| `agent/worker_engine.go` (S13, removed in T02) | `provision/factory.go` (inline) |
| `tool/registry.go` (S15) | `external/registry.go` |
| `tool/cli_adapter.go` (S15) | `external/cli_session.go + cli_execute.go` |
| `tool/cursor_adapter.go` (S15) | `external/cursor_session.go + cursor_execute.go` |
| `tool/stream_json.go` (S15) | `external/stream_json.go` |
| `contracts.go` (S5) | `kernel/contracts.go` |
| `observer/noop.go` (S5) | `kernel/noop.go` |
| `factory/NewAgentFactory` body | `provision/factory.go` |
| `factory/EngineBuilder` interface | `provision/factory.go` |
| `metrics.go` (execute) | `execute/metrics.go` (no rename, in correct dir) |
| `metrics.go` (provision/freefork) | `provision/freefork/metrics.go` (no rename) |

### T15 Historical S 沉 archive

- 文件 1：`openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md` (NEW ~80 lines)
  - 冻结索引 + 迁移路径表（D4-S1→S11 / D4-S2→S12, S13 / D4-S3→S13 / D4-S4→S11 / D4-S5→kernel / D4-S6→S15）

- 文件 2：`openspec/specs/d4-multi-agent/a-registry.md` 删除 §Historical Modules 章节 (含 D4-S1~S10)
- 文件 3：`openspec/specs/d4-multi-agent/f-registry.md` 删除 §Legacy 章节 (L103-220)
- 文件 4：`openspec/specs/d4-multi-agent/t-registry.md` 删除 §Historical 章节

### T16 d4-domain.md 物理路径表与 code 100% 对齐

### T17 d7-boundary.md 同步

---

## §5 子 Change #3 value-flow-rename（PR-5, T18-T21）

T18-T21 见 `proposal.md §6`

---

## §6 子 Change #4 span-coverage（PR-6, T22-T27）

### T22 `orchtypes/events.go` NEW

```go
package orchtypes

// 7 D4 EngineEvent 字面量常量化（与 D7/D6/D3 治理常量并列）
const (
    EventAgentStarted       = "agent.started"
    EventAgentError         = "agent.error"
    EventAgentTerminated    = "agent.terminated"
    EventAgentIterating     = "agent.iterating"
    EventAgentForked        = "agent.forked"
    EventAgentJoined        = "agent.joined"
    EventPermissionRequired = "permission_required"
)
```

### T23 lifecycle.go + forkjoin.go 字面量 → const 引用

- 7 处 `a.emit("agent.xxx", ...)` → `a.emit(orchtypes.EventAgentXxx, ...)`

### T24 `agent_bridge.go` 同步

- L142-154 6 case switch 走 orchtypes const

### T25 `evolution/guard/observer.go` 同步

- L52-86 2 case switch 同

### T26 t-registry.md 77 T 行加 Span Evidence 列

### T27 `scripts/d4-span-coverage.sh` (NEW, ~80 lines)

```bash
#!/usr/bin/env bash
# d4-span-coverage.sh — D4 域 Span Evidence 覆盖率守门
set -euo pipefail
cd "$(dirname "$0")/.."

REGISTRY=openspec/specs/d4-multi-agent/t-registry.md
[[ -f "$REGISTRY" ]] || { echo "FAIL: t-registry.md not found"; exit 1; }

# Active span ops
SPAN_OPS=$(grep -oE "D4_[A-Z_]+" internal/layers/observability/instrument/telemetry/names.go | sort -u | tr '\n' '|' | sed 's/|$//')
# Active events
EVENTS=$(grep -oE 'EventAgent[A-Za-z]+ = "[^"]*"' internal/layers/multiagent/orchtypes/events.go | grep -oE '"[^"]*"' | tr -d '"' | sort -u | tr '\n' '|' | sed 's/|$//')

# Count T rows with Span Evidence column
TOTAL=$(awk '/^\| D4-S[0-9]+-A[0-9]+-T[0-9]+ \|/' "$REGISTRY" | wc -l)
MAPPED=$(awk '/^\| D4-S[0-9]+-A[0-9]+-T[0-9]+ \|/' "$REGISTRY" | grep -cE "D4_[A-Z_]+|agent\.[a-z]+|permission_required")
COVERAGE=$(awk "BEGIN {printf \"%.1f\", $MAPPED * 100 / $TOTAL}")

echo "D4 Span Evidence Coverage: ${COVERAGE}% ($MAPPED/$TOTAL)"
awk -v cov="$COVERAGE" 'BEGIN { if (cov+0 < 80) { exit 1 } }' || { echo "FAIL: coverage < 80%"; exit 1; }
echo "PASS"
```

---

## §7 子 Change #5 boundary-decision（PR-7, T28-T31）

### T28 3 boundary decision 审计

| Boundary | 状态 |
|----------|------|
| `BoundaryD4ToD7AgentEventBridge` | RESOLVED (PR-6 7 EventAgent* const 化) |
| `BoundaryD4ToD6EvolutionObserver` | RESOLVED (同上) |
| `BoundaryD4ForbiddenFlowHubPublish` | RESOLVED (PR-6 const switch 治理) |

### T29 `orchtypes/boundary_decision.go` NEW ~30 lines

```go
package orchtypes

// D4 MultiAgent Boundary Debt Decisions (DM-20260629-004 PR-7 #5)
// 命名空间 `boundary-debt:{name}-v{major}.{minor}` 对齐 D2/D3/D7
const (
    BoundaryD4ToD7AgentEventBridge   = "boundary-debt:d4-to-d7-agent-event-bridge-v1.0"
    BoundaryD4ToD6EvolutionObserver  = "boundary-debt:d4-to-d6-evolution-observer-v1.0"
    BoundaryD4ForbiddenFlowHubPublish = "boundary-debt:d4-forbidden-flow-hub-publish-v2.0"
)
```

### T30 `orchtypes/boundary_decision_test.go` NEW ~60 lines — 3 单测

### T31 d4-domain.md §Boundary Debt Decisions 章节 — 3 row 表格 + 治理常量位置

---

## §8 S7_Archive（PR-8, T32-T34）

### T32 6 artifacts 复制到 `openspec/archive/2026-06-30-devrix-d4-dsaft-restructuring/`

- `.openspec.yaml` (NEW)
- `acceptance-report.md` (NEW)
- `demand.md` / `design.md` / `proposal.md` / `tasks.md` (from changes/)
- `specs/d4-multi-agent/spec.md` (NEW or from changes/)

### T33 verify-archive.sh 12/12 PASS

### T34 d4-domain.md v1.0.0 → v1.5.0 + 修订记录 v1.5.0 row

---

**END of Tasks**
