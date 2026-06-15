# Tasks: devrix-d7-orthogonal-intent-paths

**Change ID:** devrix-d7-orthogonal-intent-paths
**Demand ID:** DM-20260615-004
**Status:** S4_Implementation

> 估算仅参考，非承诺。

## Phase 1: 实现核心代码

| Task | Owner | T层 | 估算 | 状态 | 文件 |
|------|-------|-----|------|------|------|
| T1.1 | coordinator | — | 0.5d | ✅ | `internal/layers/orchestration/coordinator/command_handler.go` (新增) |
| T1.2 | coordinator | — | 0.5d | ✅ | `internal/layers/orchestration/coordinator/orchestrate_path.go` (新增) |
| T1.3 | coordinator | — | 0.3d | ✅ | `internal/layers/orchestration/coordinator/orchestrator.go` (改 switch 4 case + 加 option) |
| T1.4 | workmodel | — | 0.1d | ✅ | `internal/layers/orchestration/workmodel/cli_commands.go` (导出 Help) |

## Phase 2: 单测

| Task | Owner | T层 | 估算 | 状态 | 文件 |
|------|-------|-----|------|------|------|
| T2.1 | coordinator | D7-S2-A01-T04 | 0.3d | ✅ | `command_handler_test.go` (3 AC: plan / task / sink / nil) |
| T2.2 | coordinator | D7-S2-A01-T05 | 0.4d | ✅ | `orchestrate_path_test.go` (5 AC: pipeline / nil-decomp / nil-sched / wave-fail / summarize) |
| T2.3 | coordinator | D7-S2-T01 | 0.2d | ✅ | `orchestrator_test.go` (更新 Command / AntiFabrication / ShadowNotCalled 3 测试) |

## Phase 3: 文档同步

| Task | Owner | 估算 | 状态 | 文件 |
|------|-------|------|------|------|
| T3.1 | docs | 0.1d | ✅ | `openspec/changes/devrix-d7-orthogonal-intent-paths/specs/d7-orchestration/spec.md` (Gherkin Scenarios) |
| T3.2 | docs | 0.1d | pending | `openspec/specs/d7-orchestration/spec.md` (D7-S2-A01 状态 + Revision History) |
| T3.3 | docs | 0.1d | pending | `openspec/specs/d7-orchestration/a-registry.md` (D7-S2-A01 标注) |
| T3.4 | docs | 0.1d | pending | `openspec/specs/d7-orchestration/t-registry.md` (新增 3 P0 T) |

## Phase 4: 验证门禁

| Task | 命令 | 状态 |
|------|------|------|
| T4.1 | `go vet ./...` | ✅ 0 errors |
| T4.2 | `go test -race -count=1 ./internal/layers/orchestration/coordinator/...` | ✅ PASS |
| T4.3 | `go test -count=1 ./...` (全 repo) | ✅ 0 FAIL |
| T4.4 | `internal/lint/layer.TestD2_D3Ban_*` | ✅ ≤4 whitelist entries |
| T4.5 | `go test -race ./internal/bootstrap/...` | ✅ PASS |

## 累计

- 估算：~2.7d
- 实际：本次 session 内完成
- 文件变更：+5 / ~7 / 0 delete

## 关联

- T 注册表：`openspec/specs/d7-orchestration/t-registry.md` (待 T3.4 同步)
- A 注册表：`openspec/specs/d7-orchestration/a-registry.md` (待 T3.3 同步)
- 域 spec：`openspec/specs/d7-orchestration/spec.md` (待 T3.2 同步)
