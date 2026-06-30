---
demand-id: DM-20260630-013
change-id: devrix-d2-d7-review-hardening
status: ACCEPTED
verified-at: 2026-07-01
prs:
  - 361  # P1-A + P1-B (orchestrator / turn_loop / turn_state / item_pipeline / resolve / child_downlink / escape / mups/execute + D2 enforce + D2 compression)
  - 362  # P2 (规约清理 + 数据卫生: arbitrator i18n + strategic_plan_proposer i18n + work_tree.SetStore mu + decompose_proposer_test 扩展 + JSONL strict)
---

# Acceptance Report: D2+D7 代码审查硬化

## Summary

P0 + P1 + P2 hardening complete. Resolves the 24 RH-D2 / RH-D7 findings from the
2026-06-30 D7 orchestration + D2 contextengine code review (covering security,
concurrency, compression closure, error observability, spec hygiene). 30/30 T
points IMPLEMENTED, 2 PRs squash-merged to master, all 24 orchestration +
22 contextengine packages pass `go test -race -count=1`.

The change touches four layers of the production hardening stack:

1. **P0 (安全 + 并发 Critical)** — covered in earlier commits; 14 P0 T points
   covering `PlanModeWriteParity` + `SymlinkContainment` +
   `AutocompactWriteback` + `PerInvocationEmit` + `OnReleaseOnce`.
2. **P1-A (D7 错误可观测)** — 7 T points: orchestrator.EnsureGoal +
   turn_loop.AwaitRunningChildren + turn_loop 4 silent errors + turn_state
   EndTurn + item_pipeline SetRoundPhase + resolve.go 4 silent errors +
   child_downlink DefaultChildExpectedReturn schema tag.
3. **P1-A3 (D7 escape ctx cancel)** — 2 T points: escape Arbitrator Timer +
   ctx cancel 200 cycles no-leak + mups/execute ErrChannelCtxCancelled.
4. **P1-B (D2 fail-closed)** — 5 T points: nil bashAST → Deny + sandbox
   disabled warn + bashAST parse → Deny + unknown threshold strictest +
   bash audit redaction.
5. **P1-B2 (D2 压缩并发)** — 3 T points: CompressedView mu 保护 +
   async_compact session-scoped ctx + microcompact 跳过 tool msg.
6. **P2 (规约 + 数据卫生)** — 5 T points: arbitrator i18n +
   strategic_plan_proposer i18n + work_tree.SetStore mu + JSONL strict 模式 +
   decompose_proposer_test 扫描 1→6 文件。

## P1-A Acceptance (7 T points, RH-D7-03/04/05/06/07/08/14)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P1-A1-1 orchestrator.EnsureGoal 错误 slog.Warn | PASS | `internal/layers/orchestration/sessionorchestrator/orchestrator.go` 4 处 `EnsureGoal` 调用全部改 slog.Warn 而非 `_, _` |
| T-P1-A1-2 turn_loop.AwaitRunningChildren 返回值处理 | PASS | `internal/layers/orchestration/sessionorchestrator/session_turn_loop.go` AwaitRunningChildren 错误 → emit warning |
| T-P1-A1-3 turn_loop 4 处静默错误 slog.Warn | PASS | turn_loop cancel + nonblock + drainDone + handlePurge 4 处 `_ = err` → `if err != nil { slog.Warn(...) }` |
| T-P1-A1-4 turn_state.EndTurn purge handle | PASS | `internal/layers/orchestration/sessionorchestrator/turn_state.go` EndTurn 删 handles[sessionID] |
| T-P1-A2-1 item_pipeline.SetRoundPhase warn span | PASS | `sessionorchestrator/item_pipeline.go` SetRoundPhase 失败时 emit warn + span attr `phase_write_failed=true` |
| T-P1-A2-2 resolve.go 4 处 _ = 改 warn | PASS | `workmodel/resolve.go` rollup 路径 4 处 `_ = err` → `slog.Warn` |
| T-P1-A2-3 child_downlink.DefaultChildExpectedReturn schema tag | PASS | `workmodel/child_downlink.go` DefaultChildExpectedReturn 加 `validate:"required"` schema tag |

## P1-A3 Acceptance (2 T points, RH-D7-09/12)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P1-A3-1 escape Arbitrator Timer + ctx cancel 200 cycles no-leak | PASS | `mups/escape/arbitrator.go` Timer + ctx cancel 干净退出；200 cycles `goleak.VerifyNone` PASS |
| T-P1-A3-2 mups/execute ErrChannelCtxCancelled | PASS | `mups/execute/errors.go` 新增 sentinel error；exploration + scenario channel 早退 |

## P1-B Acceptance (5 T points, RH-D2-05/06/07)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P1-B1-1 nil bashAST → Deny | PASS | `enforce/tools/builtin_surface.go` nil bashAST 直接 Deny |
| T-P1-B1-2 sandbox disabled 启动 warn | PASS | `enforce/tools/sandbox.go` `enabled=false` 时 slog.Warn + 启动告警 |
| T-P1-B1-3 bashAST parse → Deny | PASS | `enforce/tools/bash_ast.go` 生产 parse fail → Deny |
| T-P1-B1-4 unknown threshold strictest | PASS | `enforce/tools/per_risk.go` 未识别 threshold 走 strictest |
| T-P1-B1-5 bash audit redaction | PASS | `enforce/tools/tool_runner.go` bash audit 截断 + token 模式 redact |

## P1-B2 Acceptance (3 T points, RH-D2-08/09/10)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P1-B2-1 CompressedView mu 保护 | PASS | `prepare/compression/memory/manager.go` CompressedView 持 `messagesMu` |
| T-P1-B2-2 async_compact session-scoped ctx | PASS | `prepare/compression/async_compact.go` 改 session-scoped ctx 而非 `context.Background()` |
| T-P1-B2-3 microcompact 跳过 tool msg | PASS | `prepare/compression/compression_steps.go` microcompact 跳 `MessageRoleTool` + 含 `tool_calls` 消息 |

## P2 Acceptance (5 T points, RH-D7-10/11/13 + RH-D2-11)

| T point | Result | Evidence |
|---------|--------|----------|
| T-P2-1 arbitrator 战术 prompt → i18n | PASS | `mups/escape/arbitrator.go` JSON prompt 走 `i18n.EscapeArbitratorJSONSchemaHint(loc)` |
| T-P2-2 strategic_plan_proposer → i18n | PASS | `sessionorchestrator/strategic_plan_proposer.go` 删常量，走 `i18n.StrategicPlanAppendix(loc)` |
| T-P2-3 work_tree.SetStore mu 保护 | PASS | `workmodel/work_tree.go` SetStore 加 `sync.RWMutex` 保护 |
| T-P2-4 JSONL strict 模式 | PASS | `contextengine/materialize/store.go` Load + LoadAgent strict 模式 + badLines 计数 + truncateForLog helper |
| T-P2-5 decompose_proposer_test 扫描 1→6 文件 | PASS | `workmodel/decompose_proposer_test.go::NoTacticalHardcoding` 扫描扩展 1→6 文件 |

## Test Execution

```text
go test -race -count=1 ./internal/layers/orchestration/...      → 24/24 PASS
go test -race -count=1 ./internal/layers/contextengine/...      → 22/22 PASS
go vet ./...                                                   → 0 issues
gofmt -l $(git diff --name-only origin/master...HEAD)           → clean
```

## CI Verification

| Check | Result | Duration |
|-------|--------|----------|
| layer-lint (D1 boundary + D7 main-path) | PASS | 11s |
| unit tests (full repo) | PASS | 3m24s |
| PR #361 squash merge | OK | 2026-07-01 |
| PR #362 squash merge | OK | 2026-07-01 |

## Findings Closure

All 24 RH-D2/RH-D7 findings closed (or registered as backlog):

| Finding | Severity | Status | T points |
|---------|----------|--------|----------|
| RH-D7-01 | Critical | CLOSED | P0-B1 |
| RH-D7-02 | Critical | CLOSED | P0-B2 |
| RH-D7-03 | Warning | CLOSED | T-P1-A1-1 |
| RH-D7-04 | Warning | CLOSED | T-P1-A1-2 |
| RH-D7-05 | Warning | CLOSED | T-P1-A2-2 |
| RH-D7-06 | Warning | CLOSED | T-P1-A2-1 |
| RH-D7-07 | Warning | CLOSED | T-P1-A1-3 |
| RH-D7-08 | Warning | CLOSED | T-P1-A2-3 |
| RH-D7-09 | Warning | CLOSED | T-P1-A3-1 |
| RH-D7-10 | Info | CLOSED | P0-B1 (Executor Emit 字段) |
| RH-D7-11 | Warning | CLOSED | T-P2-3 |
| RH-D7-12 | Warning | CLOSED | T-P1-A3-2 |
| RH-D7-13 | 规约 | CLOSED | T-P2-1 + T-P2-2 |
| RH-D7-14 | Warning | CLOSED | T-P1-A1-4 |
| RH-D2-01 | Critical | CLOSED | P0-A1 (PlanModeWriteParity) |
| RH-D2-02 | Critical | CLOSED | P0-A2 (SymlinkContainment) |
| RH-D2-03/04 | Critical | CLOSED | P0-A3 (AutocompactWriteback) |
| RH-D2-05~07 | Warning | CLOSED | P1-B1 (5 fail-closed surface) |
| RH-D2-06 | Warning | CLOSED | P1-B1-5 (bash audit redaction) |
| RH-D2-08~10 | Warning | CLOSED | P1-B2 (3 compression concurrency) |
| RH-D2-11 | Warning | CLOSED | T-P2-4 (JSONL strict) |
| RH-D2-12 | Backlog | DEFERRED | God 文件拆分（context_engine.go 514 行、runProcess ~348 行），仅登记 backlog |

**否定项（不登记）**: D7 worker panic double-completeTask；D2 sandboxast recover fail-open（审查复核路径不存在）

## DoD Checklist

- [x] 30 P0 T 测试点 100% IMPLEMENTED（D2 15 + D7 15）
- [x] `go test -race ./internal/layers/orchestration/... ./internal/layers/contextengine/...` 全绿
- [x] `scripts/lint-d1-imports.sh` + layer-lint PASS
- [x] t-registry 30 条 T 点 IMPLEMENTED（D2 114→129 + D7 251→266）
- [x] spec.md lite-mode 契约段 + CHANGELOG 一行（S6 门禁）
- [x] S5 验收 verdict: ACCEPTED
- [x] S6-交付：PR #361 + PR #362 squash merged
- [x] S6-归档：本 acceptance-report.md + archive/ 目录就位