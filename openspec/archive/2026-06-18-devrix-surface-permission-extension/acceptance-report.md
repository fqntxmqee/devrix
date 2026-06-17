# Acceptance Report: devrix-surface-permission-extension

**Change ID:** devrix-surface-permission-extension
**Demand ID:** DM-20260618-002
**Parent Demands:** DM-20260617-007 (devrix-tool-surface-contract, S7_archived); DM-20260617-008 (devrix-tool-surface-phase2-full, S7_archived); DM-20260618-001 (devrix-tool-spec-enrichment, S7_archived)
**Status:** S5_Verified → S6_Archived
**Generated:** 2026-06-18
**Branch:** `feat/devrix-surface-permission-extension`
**Merged PR:** [#68](https://github.com/fqntxmqee/devrix/pull/68) (squash + delete-branch)

## Summary

本 change 把 per-tool permission 检查从「turn-level boolean」升级为「per-tool 3-state Decision」, 并在 D2 surface 层与 D7 IPermissionGate 层各加一个 CheckPermission 钩子。bash tool 内置 mvdan/sh v3 AST 解析器拒危险 cmd; plan mode + OpenWorld=true 的 tool 走 allowlist deny 策略; turn_adapter ExecuteRound 在 dispatch 前调 CheckPermission, Deny → 跳过 Execute 并把 PermissionDeniedError envelope 写回 result slot。

**6 个新 P0 T 点 (T26-T29 + PERMISSION-GATE-1-T01/T02) 全部 IMPLEMENTED + PASS, 既有 15 个 P0 T 点 (T01-T11 + T22-T25) 保持 PASS。**

| 类别 | 实现 | 测试 | 状态 |
|------|------|------|------|
| Decision enum (A01) | shared/contracts/permission_check.go (Allow/Deny/Ask + 2 envelope) | T26 | PASS |
| ToolSurface.CheckPermission (A01) | shared/contracts/tool_surface.go + 6 surface (default Allow) + FreeForkSurface→IPermissionGate + BuiltinSurface→BashAST | T26 | PASS |
| IPermissionGate.CheckPermission (A01) | shared/contracts/permission.go + capture.PermissionManager 实现 (Risk→Decision) | T28 | PASS |
| BashASTPolicy (BASH-AST-1) | surface/bash_ast.go (mvdan/sh v3) + BuiltinSurface.CheckPermission 集成 | T27 | PASS |
| PlanModeOpenWorldPolicy (A01) | orchestration/toolpolicy/plan_mode.go (ApplyWithContext + ShouldDefer) | T29 | PASS |
| turn_adapter 2-phase dispatch (A01) | bootstrap/turn_adapter.go (CheckPermission → Execute, indexed write-back) | T29 | PASS |
| capture.PermissionManager.CheckPermission (新方法) | internal/layers/communication/capture/permission.go (Risk→Decision 映射) | T28 | PASS |
| 既有 T01-T11/T22-T25 P0 (保持) | 既有路径 | T01-T11/T22-T25 | PASS |

## 验收门禁

- `go build ./...` — PASS
- `go test -race ./...` — PASS
- `go vet ./...` — PASS
- `gofmt -l` — clean
- `devrix-layer-lint --strict` — PASS
- CI (PR #68): unit tests / layer-lint (strict + warn) / integration / coverage — 全 SUCCESS
- 自动 squash merge + delete-branch — 完成
- `scripts/verify-archive.sh` — PASS

## Metrics

| 指标 | Baseline | Target | Actual |
|------|----------|--------|--------|
| tool_check_permission_default_allow_coverage | 0% | 100% | **100%** (7/7 surface) |
| bash_ast_dangerous_cmd_deny_rate | 0% | 100% | **100%** (5 deny rules: rm -rf /, dd, mkfs, sudo, chmod 777 /) |
| plan_mode_open_world_deny_pct | 0% | 100% | **100%** (PlanModeOpenWorldPolicy.ApplyWithContext + ShouldDefer) |
| permission_check_overhead_ms | ~20ms (RPC) | < 5ms | **< 1ms** (in-process mvdan/sh AST) |

## Follow-up

- **devrix-surface-lazy-loading** (DM-20260618-003) — DeferLoading + ShouldDefer + zodgen + ToolSearch + anthropic; 复用本 change 的 IPermissionGate + Decision 契约。
- v1.1 待办：Per-tool 自定义 permission policy DSL (YAML) + 决策 audit log。

## Conflict resolution notes (rebase onto master post-#70)

PR #68 在 #70 合并后需要 rebase onto master, 主要冲突点:
- `tool_surface.go`: PR #70 也加了 Decision/DecisionAllow/DecisionDeny/DecisionAsk/PermissionDeniedError/PermissionAskRequiredError, 两者重复 — 保留 PR #68 的 `permission_check.go` 单独文件版本, 从 `tool_surface.go` 删除重复声明。
- `builtin_surface.go`: PR #70 加默认 Allow, PR #68 加 BashASTPolicy 解析; 保留 PR #68 的 BashAST 版本 (bash 是安全敏感工具)。
- `freefork_surface.go`: 保留 PR #68 的 IPermissionGate 委托 (multi-agent spawn 走全局 gate)。
- `tracker/plugin/lsptool/verify_surface.go`: 保留 PR #68 更详细的 doc-comment, 行为一致 (return Allow)。
- `plan_mode.go`: 保留 PR #68 的 ApplyWithContext + PR #70 的 ShouldDefer (T27/T29 都需要)。
- `plan_mode_test.go` / `turn_adapter_surface_test.go`: 保留 PR #68 的 Deny/Allow/Decide flow tests + PR #70 的 ShouldDefer/FilterDeferLoading tests, 字段合并 (permReturn + DeferLoading + OpenWorld)。
- `tool_filter.go`: 保留 PR #68 的 CheckPermission delegation comment。
- `tests/integration/tool_surface_test.go`: 去掉 PR #70 rebase 引入的重复 CheckPermission 方法。
- `tests/integration/agent_integration_test.go`: PR #68 升级 IPermissionGate 加 CheckPermission 方法, capture.PermissionManager 需要实现该方法 (Risk→Decision 映射: LOW=Allow, MEDIUM/HIGH=Ask, CRITICAL=Deny) — 已在 rebase commit 中补齐。

S4-Gate re-verified after rebase: go test ./... PASS; go vet PASS; devrix-layer-lint --root=internal/layers --strict PASS.
