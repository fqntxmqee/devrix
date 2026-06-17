# Demand: devrix-tool-surface-phase2-full

**Demand ID:** DM-20260617-008
**Status:** S1
**Parent Demand:** DM-20260617-007 (devrix-tool-surface-contract)
**Priority:** P0 (与父 demand 同一级别)
**Created:** 2026-06-17

## 1. Background

父 demand `DM-20260617-007` (devrix-tool-surface-contract) 阶段 2c (PR #63)
完成了 toolrunner 层的 3 个 global singleton 删除 + 6 个 surface 收编为 1
入口 + devrix tool list CLI + S6 归档。父 change 的 `design.md §2.8`
明确了阶段 2 完整 (PR #64) 的范围: 删除剩余 5 个 global + 全量 caller
改构造期注入。

本 demand 是父 change 的 **执行级 followup**, 不引入新 AC, 不修改设计;
仅完成父 design.md §2.8 "阶段 2 (PR #64)" 描述的工作。

## 2. 父 change 已 lock-in 的设计 (本 change 直接引用)

### 2.1 待删的 5 个 global singleton

| Global | 位置 | 现有 caller | 目标: 通过显式 dep 注入 |
|--------|------|-------------|-------------------------|
| `transcript.SetGlobalWriter / GlobalWriter` | `internal/layers/communication/capture/transcript/wire.go` | `gateway.go:811` (ExpireSession) | `Gateway.Writer` 字段 |
| `flow.SetGlobalHub / GlobalHub` | `internal/layers/orchestration/flow/hub.go` | `delegate_tools.go:159`, `hubspoke/dispatch.go:69`, `subquery_fallback.go:30-31` | `ToolSurface` / 构造期注入 |
| `workmodel.SetGlobalTaskManager / GlobalTaskManager` | `internal/layers/orchestration/workmodel/task_manager.go` | `cli.go:56`, `command_handler.go:156,167`, `orchestrator.go:150,416`, `delegate_tools.go:171,181`, `wire_coordinator.go:95` | 各构造期接受 `*TaskManager` 参数 |
| `sessionqueue.GlobalSessionQueue` | `internal/layers/orchestration/sessionqueue/session_queue.go` | `context_engine.go:181`, `context_engine_builder.go:235`, `execution_flow.go:32`, `wire_wave.go:118`, `flow/hub.go:56` | `EngineDeps.SessionCommandQueue` 已是字段, 仅删除 global var |
| `freefork.SetGlobalForker` (在 freefork 包) | `internal/layers/multiagent/provision/freefork/wire.go` | `multi_agent.go:34` (set), `freefork_injection.go:34` (read) | `freeforkGlobalFunc` 改接受 `Forker` 参数 |

### 2.2 父 design.md §2.8 "阶段 2" 完整引用

> **阶段 2（PR #64，2-3 天）**:
> - `git grep` 验证 6+ global var 零引用
> - 删除 global var + setter 函数
> - 全量单测 + E2E IM 验证
> - 灰度 1 周

本 change 范围 = 阶段 2 完整 (无灰度期, 阶段 2 已经是最终态)。

### 2.3 父 design.md §2.8 "回滚点"

> - 阶段 2 回滚: revert PR #64, 但 global var 已被删 — 需用 `git revert`
>   然后从 git history 恢复; **不推荐阶段 2 后回滚**

回滚路径已 lock, 本 change 不重新讨论。

## 3. 验收标准 (本 change)

| ID | 描述 | 度量 |
|----|------|------|
| **AC-P2-1** | 5 个 global var + 5 个 setter 函数全部删除 (代码层面) | `git grep` 验证 0 引用 (除注释) |
| **AC-P2-2** | 所有 caller 改构造期注入, 不再调 `Set*` / `Get*` 全局访问 | `git grep -n "SetGlobal\|GlobalSessionQueue\|GlobalTaskManager\|GlobalHub\|GlobalWriter\|GlobalForker" internal/` 仅命中注释 |
| **AC-P2-3** | `go test -race ./...` 100% 绿 | 测试输出 |
| **AC-P2-4** | `go vet ./...` 0 warning | 测试输出 |
| **AC-P2-5** | `verify-archive.sh` 全部通过 | S6 阶段执行 |

**No new ACs** — 父 change 的 22 AC 已 lock-in, 本 change 不引入新维度。

## 4. 范围 (Out of Scope)

- 不引入新 surface / filter
- 不修改 D2/D3/D4/D5/D6 library 对外 API
- 不修改父 change 的 acceptance-report / spec.md
- 不重新跑 verify-archive.sh 在父 change 上 (父已 S6_archived)

## 5. 风险

- **H**: EngineDeps 扩字段 → 全部 7 个 engine call site 同步改 → 编译错误风险
- **M**: 5 个 global 跨 12+ 文件引用, 单 commit 太大 → 拆 5 个 sub-commit (按 global 分)
- **L**: 父 change 的 per-agent ⊇ main 等价性测试需要重写 (因 FreeForkSurface 构造方式变了)

## 6. 估时

2-3 天 (per 父 design §2.8). 5 sub-commit × 0.5 天。
