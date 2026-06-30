# Implementation Tasks: D2+D7 代码审查硬化

**Change ID:** `devrix-d2-d7-review-hardening`  
**Demand ID:** DM-20260630-013

---

## Phase P0-A — D2 安全 Critical

### T-P0-A1 PlanModeWriteParity

- [ ] 1.1 `edit_tool.go` — 在 `resolveWorkspacePath` 后调用 `EnforcePlanModeWrite`
- [ ] 1.2 单测 `edit_tool_plan_mode_test.go` — plan 内允许 / plan 外拒绝

**T:** D2-S18-A80-T01, D2-S18-A80-T02

### T-P0-A2 SymlinkContainment

- [ ] 2.1 `tool_runner.go::resolveWorkspacePath` — `EvalSymlinks` + realpath ⊆ workDir
- [ ] 2.2 单测 — symlink 逃逸拒绝 + 内部 symlink 允许

**T:** D2-S18-A81-T01, D2-S18-A81-T02

### T-P0-A3 AutocompactWriteback

- [ ] 3.1 实现 `sessionAutocompactSink`（kernel 或 prepare/compression）— token 替换 placeholder
- [ ] 3.2 `context_engine_builder.go` — async 启用时 wire 非 NoOp sink；或默认 `async.enabled=false` + warn
- [ ] 3.3 失败路径 — Degraded + 保留 middle 或 sync fallback
- [ ] 3.4 单测 + 集成 `async_compact_writeback_test.go`

**T:** D2-S15-A80-T01, D2-S15-A80-T02

**P0-A Quality Gate:**

- [ ] `go test ./internal/layers/contextengine/enforce/tools/... -race -count=1`
- [ ] `go test ./internal/layers/contextengine/prepare/compression/... -count=1`

---

## Phase P0-B — D7 并发 Critical

### T-P0-B1 PerInvocationEmit

- [ ] 4.1 `ItemPipelineRunOpts` + `Run(..., opts)` 签名
- [ ] 4.2 `WorkItemExecutor` — `ExecuteOpts.Emit` 参数化
- [ ] 4.3 `session_turn_loop.go` — 移除 `o.itemPipeline.Emit =` 字段写入
- [ ] 4.4 `wire_item_pipeline.go` — 适配新签名
- [ ] 4.5 并发单测 — 两 session `-race` PASS

**T:** D7-S2-A80-T01, D7-S2-A80-T02

### T-P0-B2 OnReleaseOnce

- [ ] 5.1 `pool.go` — 单 hook 注册 API；禁止 append 无界
- [ ] 5.2 `scheduler.go` — 删除 `Start`/`dispatchLoop` 重复 `OnRelease`
- [ ] 5.3 单测 — 100 次 Start hook 计数不变

**T:** D7-S3-A84-T01

**P0-B Quality Gate:**

- [ ] `go test ./internal/layers/orchestration/sessionorchestrator/... ./internal/layers/orchestration/wavescheduler/... -race -count=1`

---

## Phase P1-A — D7 错误可观测

### T-P1-A1 Orchestrator / TurnLoop

- [ ] 6.1 `orchestrator.go` — `EnsureGoal` 错误 slog.Warn（非 `_, _`）
- [ ] 6.2 `session_turn_loop.go` — `AwaitRunningChildren` 返回值处理 + emit
- [ ] 6.3 `turn_state.go` — `EndTurn` purge handle

**T:** D7-S2-A81-T01, D7-S2-A82-T01, D7-S2-A83-T01, D7-S2-A85-T01

### T-P1-A2 ItemPipeline / WorkModel

- [ ] 7.1 `item_pipeline.go` — `SetRoundPhase` 失败 warn + span
- [ ] 7.2 `resolve.go` — rollup 路径错误 warn
- [ ] 7.3 `child_downlink.go` — `DefaultChildExpectedReturn` schema tag

**T:** D7-S2-A84-T01, D7-S15-A42-T01, D7-S16-A77-T01

### T-P1-A3 Escape / MUPS

- [ ] 8.1 `arbitrator.go` — Timer + ctx cancel；goroutine leak 测试
- [ ] 8.2 `mups/execute` — ctx cancel early exit（exploration + scenario）

**T:** D7-S14-A48-T01, D7-S9-A33-T01

---

## Phase P1-B — D2 fail-closed + 压缩

### T-P1-B1 Enforce Fail-Closed

- [ ] 9.1 `builtin_surface.go` — nil bashAST → Deny
- [ ] 9.2 `sandbox.go` — disabled 启动 warn + metric
- [ ] 9.3 `bash_ast.go` — 生产 parse fail → Deny
- [ ] 9.4 `per_risk.go` — unknown threshold → strict
- [ ] 9.5 `tool_runner.go` — bash audit redaction

**T:** D2-S18-A82-T01/T02, D2-S18-A83-T01, D2-S18-A84-T01, D2-S18-A85-T01

### T-P1-B2 Compression Concurrency

- [ ] 10.1 `memory/manager.go` — CompressedView 持 `messagesMu`
- [ ] 10.2 `async_compact.go` — session-scoped ctx
- [ ] 10.3 `compression_steps.go` — microcompact 跳过 tool 消息

**T:** D2-S15-A81-T01, D2-S15-A82-T01, D2-S15-A83-T01

---

## Phase P2 — 规约 + 卫生

### T-P2-1 D7 规约清理

- [ ] 11.1 `arbitrator.go` — 战术 prompt → i18n
- [ ] 11.2 `strategic_plan_proposer.go` — appendix → i18n
- [ ] 11.3 `work_tree.go` — `SetStore` mutex 或 bootstrap-only lint
- [ ] 11.4 `decompose_proposer_test.go` — 扩展 NoTacticalHardcoding 扫描范围

**T:** D7-S14-A49-T01, D7-S16-A78-T01, D7-S1-A80-T01

### T-P2-2 D2 数据卫生

- [ ] 12.1 `materialize/store.go` — JSONL bad line 计数/strict 模式

**T:** D2-S17-A80-T01

---

## Registry & Docs (S3 → S6)

- [ ] 13.1 `openspec/specs/d2-context-engine/t-registry.md` — 登记 15 条 T（PLANNED）
- [ ] 13.2 `openspec/specs/d7-orchestration/t-registry.md` — 登记 15 条 T（PLANNED）
- [ ] 13.3 S6 合入 spec.md lite-mode 契约段 + CHANGELOG 一行

---

## Completion Checklist

- [ ] demand.md AC 全部可勾选
- [ ] acceptance-report.md（S5 填写）
- [ ] P0 T 测试点 100% IMPLEMENTED
- [ ] `openspec/specs/` delta 合入（S6 门禁）
- [ ] demand-archive-index 条目（S7）

---

## 审查项 → 需求映射速查

| 审查 ID | OpenSpec Requirement | Phase |
|---------|---------------------|-------|
| RH-D7-01 | D7-S2-A80 | P0-B |
| RH-D7-02 | D7-S3-A84 | P0-B |
| RH-D7-03 | D7-S2-A81 | P1-A |
| RH-D7-04 | D7-S2-A82 | P1-A |
| RH-D7-05 | D7-S15-A42 | P1-A |
| RH-D7-06 | D7-S2-A84 | P1-A |
| RH-D7-07 | D7-S2-A83 | P1-A |
| RH-D7-08 | D7-S16-A77 | P1-A |
| RH-D7-09 | D7-S14-A48 | P1-A |
| RH-D7-10 | D7-S2-A80-T02 | P0-B |
| RH-D7-11 | D7-S1 SetStore | P2 |
| RH-D7-12 | D7-S9-A33 | P1-A |
| RH-D7-13 | D7-S14/D7-S16 i18n | P2 |
| RH-D7-14 | D7-S2-A85 | P1-A |
| RH-D2-01 | D2-S18-A80 | P0-A |
| RH-D2-02 | D2-S18-A81 | P0-A |
| RH-D2-03/04 | D2-S15-A80 | P0-A |
| RH-D2-05~07 | D2-S18-A82/A84 | P1-B |
| RH-D2-06 | D2-S18-A83 | P1-B |
| RH-D2-08~10 | D2-S15-A81~A83 | P1-B |
| RH-D2-11 | D2-S17-A80 | P2 |
| RH-D2-12 | backlog（God 文件） | — |

**否定项（不登记）**: D7 worker panic double-complete；D2 sandboxast recover fail-open
