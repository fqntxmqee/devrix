# Tasks: D7 Execute ↔ ToolRunner 4 Channel 5 场景输入输出协议沉淀

**T-layer registration path**: `openspec/specs/d7-orchestration/t-registry.md`
**Status legend**: PLANNED / IMPLEMENTED

## A. Spec Doc (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A122-T01 | IMPLEMENTED | §3 输入协议: PlanKind 4 + ChannelRequest 3 + ToolRequest 5 字段表 | `d7-execute-toolrunner-io-protocol-spec.md:67-130` |
| D7-S5-A122-T02 | IMPLEMENTED | §4 输出协议: ArtifactKind 4 + SideEffectStatus 5 + WorkerType 3 矩阵 | `d7-execute-toolrunner-io-protocol-spec.md:132-180` |
| D7-S5-A122-T03 | IMPLEMENTED | §5 场景 1: CommitChannel 1 step success → ArtifactStateChangeCert | `d7-execute-toolrunner-io-protocol-spec.md:184-217` |
| D7-S5-A122-T04 | IMPLEMENTED | §5 场景 2: ProtocolChannel 3 steps + step 2 fail → rollback → SideEffectRolledBack | `d7-execute-toolrunner-io-protocol-spec.md:219-256` |
| D7-S5-A122-T05 | IMPLEMENTED | §5 场景 3: ScenarioChannel 5 probes + 3 pass → majority vote → SideEffectNone | `d7-execute-toolrunner-io-protocol-spec.md:258-294` |
| D7-S5-A122-T06 | IMPLEMENTED | §5 场景 4: ExplorationChannel 3 experiments + 优先级排序 → sideEffectForScope | `d7-execute-toolrunner-io-protocol-spec.md:296-345` |
| D7-S5-A122-T07 | IMPLEMENTED | §5 场景 5 (混合): Commit timeout (9006) + Scenario ctx cancel (9007) | `d7-execute-toolrunner-io-protocol-spec.md:347-394` |
| D7-S5-A122-T08 | IMPLEMENTED | §7 Go-side invariants 表 (15 兜底规则) | `d7-execute-toolrunner-io-protocol-spec.md:401-419` |
| D7-S5-A122-T09 | IMPLEMENTED | §8 sideEffectForScope 映射表 (3 PersistScope → 3 SideEffect) | `d7-execute-toolrunner-io-protocol-spec.md:421-432` |
| D7-S5-A122-T10 | IMPLEMENTED | §9 Error code 表 (EXEC_CHANNEL_9001-9007 闭集) | `d7-execute-toolrunner-io-protocol-spec.md:434-446` |
| D7-S5-A122-T11 | IMPLEMENTED | §10 Test 覆盖表 (5 NEW + 已有 23 tests 互补) | `d7-execute-toolrunner-io-protocol-spec.md:448-475` |

## B. Test 补充 (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A122-T12 | PLANNED | TestExecuteTraceE2E_Commit_Success (场景 1) — 1 step ok + SideEffectCommitted + IdempotencyKey 透传 | `execute_trace_e2e_test.go:NEW` |
| D7-S5-A122-T13 | PLANNED | TestExecuteTraceE2E_Protocol_RollbackSuccess (场景 2) — step 2 fail + 3 runner calls + SideEffectRolledBack | `execute_trace_e2e_test.go:NEW` |
| D7-S5-A122-T14 | PLANNED | TestExecuteTraceE2E_Scenario_MajorityPass (场景 3) — 3/5 pass + SideEffectNone + majority vote | `execute_trace_e2e_test.go:NEW` |
| D7-S5-A122-T15 | PLANNED | TestExecuteTraceE2E_Exploration_PartialSuccess (场景 4) — 2/3 成功 + 优先级排序 + sideEffectForScope(PersistTransient)=None | `execute_trace_e2e_test.go:NEW` |
| D7-S5-A122-T16 | PLANNED | TestExecuteTraceE2E_CommitTimeout_Inflight (场景 5) — 50ms timeout + SideEffectInflight + EXEC_CHANNEL_9006 + Scenario ctx cancel + EXEC_CHANNEL_9007 不混淆 | `execute_trace_e2e_test.go:NEW` |

## C. 集成 (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A122-T17 | PLANNED | 主 spec.md §11 加 1 行 reference 指向 d7-execute-toolrunner-io-protocol-spec.md | `spec.md:NEW` |
| D7-S5-A122-T18 | PLANNED | 主 CHANGELOG.md +1 row: devrix-d7-execute-llm-protocol-doc (2026-07-09) | `CHANGELOG.md:NEW` |

## D. 验证 (P0)

| T ID | Status | Description | Evidence |
|------|--------|-------------|----------|
| D7-S5-A122-T19 | PLANNED | `go test -v -run TestExecuteTraceE2E ./internal/layers/orchestration/mups/execute/...` 5/5 PASS | TBD |
| D7-S5-A122-T20 | PLANNED | `go test -race ./internal/layers/orchestration/mups/execute/...` 全部 PASS (含 23 已有 + 5 NEW) | TBD |
| D7-S5-A122-T21 | PLANNED | 22/22 orchestration packages go test -race 回归 | TBD |
| D7-S5-A122-T22 | PLANNED | `go vet ./...` 0 warning | TBD |
| D7-S5-A122-T23 | PLANNED | PR 提交 (1 spec + 5 trace test + 1 spec.md ref + 1 CHANGELOG) | TBD |

**Total**: 23 T-points (11 spec sections + 5 trace test + 2 integration + 5 verification).
