# Delta Spec: Global Singleton Cleanup (Phase 2 Full)

**Demand ID:** DM-20260617-008
**Capability:** tool-surface-global-cleanup
**Domains:** D1, D2, D4, D7 (transcript / flow / sessionqueue / workmodel / freefork)
**Parent:** devrix-tool-surface-contract (DM-20260617-007, S7_archived)

## ADDED Requirements

### REQ-GC-01: transcript.GlobalWriter 删除 + Gateway.Writer 字段注入

The system SHALL delete the package-level global `transcript.globalW`,
`transcript.SetGlobalWriter`, `transcript.GlobalWriter` getter, and the
`transcript.Append` global shortcut. The transcript writer SHALL be held
as `CommunicationGateway.writer *transcript.Writer` and injected via
`NewCommunicationGateway(..., writer)`.

#### Scenario: gateway.ExpireSession 走注入 writer

- **GIVEN** a CommunicationGateway constructed via `NewCommunicationGateway(..., writer)` with `writer = NewFileWriter("/tmp/x.jsonl")`
- **AND** a session with expiry event "session_42 expired"
- **WHEN** `gateway.ExpireSession("session_42")` is invoked
- **THEN** `/tmp/x.jsonl` SHALL contain a JSON line for the expiry event
- **AND** no call to `transcript.GlobalWriter()` or `transcript.SetGlobalWriter` SHALL occur (verified via `git grep`)

#### Scenario: gateway.writer=nil 时 ExpireSession 不报错 (best-effort)

- **GIVEN** a CommunicationGateway constructed with `writer = nil`
- **AND** a session with expiry event
- **WHEN** `gateway.ExpireSession("session_42")` is invoked
- **THEN** the call SHALL return `nil` error
- **AND** no panic SHALL occur (nil-safe)

#### Scenario: 2 test 文件消除 `defer reset(global)` 反模式

- **GIVEN** `internal/layers/communication/capture/session_store_transcript_test.go` exists
- **WHEN** the test file is inspected for `t.Cleanup(func() { transcript.SetGlobalWriter(prevW) })`
- **THEN** the pattern SHALL be replaced with constructor injection (`NewCommunicationGateway(..., writer)`)
- **AND** `git grep -n "SetGlobalWriter\|GlobalWriter" internal/` SHALL return only comment-line matches

### REQ-GC-02: flow.GlobalHub 删除 + Deps.Hub 字段注入

The system SHALL delete `flow.GlobalHub` package-level variable and
`flow.SetGlobalHub` setter. All callers SHALL obtain the hub via
struct field injection: `delegatetools.Deps.Hub`,
`hubspoke.DispatchDeps.Hub`, or as an explicit constructor parameter.

#### Scenario: delegatetools.Snapshot 走 deps.Hub

- **GIVEN** `delegatetools.Deps{Hub: hub}` constructed with a stub hub
- **AND** `hub.Snapshot("session_42")` is set to return `&Snapshot{Count: 7}`
- **WHEN** `delegatetools.Snapshot(ctx, sc, deps)` is invoked with `sc.SessionID = "session_42"`
- **THEN** the result SHALL be `&Snapshot{Count: 7}`
- **AND** no read of `flow.GlobalHub` SHALL occur

#### Scenario: hubspoke.dispatch 走 deps.Hub

- **GIVEN** `hubspoke.DispatchDeps{Hub: hub}` constructed with a stub hub
- **WHEN** `hubspoke.Dispatch(ctx, ev, deps)` is invoked with `ev.Kind = FlowToolCall`
- **THEN** the hub's `Publish` method SHALL be called once
- **AND** `flow.GlobalHub` SHALL NOT be read

#### Scenario: bootstrap.execution_flow 不调 SetGlobalHub

- **GIVEN** `internal/bootstrap/execution_flow.go` exists
- **WHEN** the file is inspected
- **THEN** `flow.SetGlobalHub(...)` calls SHALL be 0 (was 2: SetGlobalHub(nil) + SetGlobalHub(hub))
- **AND** the constructed `Hub` SHALL be passed to downstream callers via the `Deps.Hub` field

### REQ-GC-03: sessionqueue.GlobalSessionQueue 删除 + NewSessionQueue() 局部实例

The system SHALL delete `sessionqueue.GlobalSessionQueue` package-level
variable. All 5 callers SHALL create a local `*SessionQueue` instance
via `sessionqueue.NewSessionQueue()` and pass it explicitly via
`EngineDeps.SessionCommandQueue` or `flow.HubDeps.Queue`.

#### Scenario: 5 caller 各持有独立 SessionQueue 实例

- **GIVEN** 5 callers (`context_engine.go:181`, `context_engine_builder.go:235`, `wire_wave.go:118`, `execution_flow.go:32`, `flow/hub.go:56`)
- **WHEN** each caller's code is inspected
- **THEN** each SHALL construct `sessionqueue.NewSessionQueue()` locally
- **AND** no reference to `sessionqueue.GlobalSessionQueue` SHALL remain

#### Scenario: EngineDeps.SessionCommandQueue 字段化 (沿用阶段 1)

- **GIVEN** `EngineDeps` already has `SessionCommandQueue contracts.SessionCommandQueue` field (added in PR #63 phase 1)
- **WHEN** the context engine is built
- **THEN** the local `*SessionQueue` SHALL be assigned to `EngineDeps.SessionCommandQueue`
- **AND** the field SHALL flow through to `ContextEngine` constructor

#### Scenario: flow.NewHub 接受显式 Queue

- **GIVEN** `flow.HubDeps{Queue: q}` where `q = NewSessionQueue()`
- **WHEN** `flow.NewHub(deps)` is called
- **THEN** `Hub.q` SHALL be set to `q` (not `sessionqueue.GlobalSessionQueue`)

### REQ-GC-04: workmodel.GlobalTaskManager 删除 + ctor 注入

The system SHALL delete `workmodel.GlobalTaskManager` package-level
variable and `workmodel.init()` function. The
`workmodel.InitGlobalTaskManager` factory SHALL be replaced by
`workmodel.NewTaskManagerFromConfig` (returning `*TaskManager` instead
of writing to global). All 6+ callers SHALL inject the `*TaskManager`
via constructor or struct field.

#### Scenario: Orchestrator.tasks 字段注入

- **GIVEN** `coordinator.NewOrchestrator(..., tasks)` with `tasks = NewTaskManager()`
- **WHEN** `Orchestrator` is constructed
- **THEN** `o.tasks` SHALL be set to the injected `tasks`
- **AND** `o.tasks.Create(...)` SHALL be called by Orchestrator methods (not `workmodel.GlobalTaskManager.Create(...)`)

#### Scenario: CommandHandler.tasks 字段注入

- **GIVEN** `coordinator.NewCommandHandler(..., tasks)` with `tasks = NewTaskManager()`
- **WHEN** `CommandHandler.HandleCommand` is called with `cmd.Subject = "test_task"`
- **THEN** `h.tasks.Create("test_task", ...)` SHALL be invoked
- **AND** `workmodel.GlobalTaskManager` SHALL NOT be read

#### Scenario: cli.NewCLIAdapter 接受 tasks 参数

- **GIVEN** `cli.NewCLIAdapter(..., tasks)` with `tasks = NewTaskManager()`
- **WHEN** the CLI adapter is constructed
- **THEN** the adapter SHALL hold `tasks` as a private field
- **AND** `workmodel.NewCLICommands(tasks)` SHALL be called with the injected tasks

#### Scenario: delegatetools.Deps.Tasks 字段

- **GIVEN** `delegatetools.Deps{Tasks: tasks}` with `tasks = NewTaskManager()`
- **WHEN** `delegatetools.CreateTask(ctx, sessionID, subject, directive, deps)` is invoked
- **THEN** `deps.Tasks.Create(sessionID, subject, directive)` SHALL be called
- **AND** no reference to `workmodel.GlobalTaskManager` SHALL occur

#### Scenario: 6+ caller 全部零引用

- **GIVEN** callers in `cli.go:56`, `command_handler.go:167`, `orchestrator.go:150,416`, `delegate_tools.go:171,181`, `wire_coordinator.go:95`
- **WHEN** `git grep -n "GlobalTaskManager" internal/` is run
- **THEN** the output SHALL contain 0 production-code matches (only comments OK)

### REQ-GC-05: freefork.SetGlobalForker (包内) 删除 + Forker 参数化

The system SHALL delete `freefork.globalForker`, `freefork.SetGlobalForker`,
and `freefork.GlobalForker` getter inside the
`internal/layers/multiagent/provision/freefork/` package. The
`freeforkGlobalFunc` SHALL accept an explicit `freefork.Forker` parameter.
The `WireMultiAgent` function SHALL return the `Forker` to callers.

#### Scenario: freeforkGlobalFunc 接受 Forker 参数

- **GIVEN** a stub `freefork.Forker` implementation `f`
- **WHEN** `freeforkGlobalFunc(ctx, sessionID, prompt, f)` is invoked
- **THEN** `f.Fork(ctx, sessionID, prompt)` SHALL be called once
- **AND** `freefork.GlobalForker()` SHALL NOT be called

#### Scenario: WireMultiAgent 返回 Forker 给 caller

- **GIVEN** `multiagent.WireMultiAgent(opts)` is called
- **WHEN** the function returns
- **THEN** the return tuple SHALL include `freefork.Forker` as the last element
- **AND** the caller SHALL store the returned `Forker` in a local variable

#### Scenario: freefork 包内 0 GlobalForker 引用

- **GIVEN** `internal/layers/multiagent/provision/freefork/wire.go` exists
- **WHEN** `git grep -n "SetGlobalForker\|GlobalForker\|globalForker" internal/layers/multiagent/provision/freefork/` is run
- **THEN** the output SHALL be empty (0 matches)

### REQ-GC-06: 全局零引用验收

The system SHALL pass the static `git grep` verification across
`internal/` with the pattern `SetGlobal\|GlobalSessionQueue\|
GlobalTaskManager\|GlobalHub\|GlobalWriter\|GlobalForker`.

#### Scenario: git grep 仅命中注释

- **GIVEN** all 5 sub-commits (W1-W5) are merged
- **WHEN** `git grep -n "SetGlobal\|GlobalSessionQueue\|GlobalTaskManager\|GlobalHub\|GlobalWriter\|GlobalForker" internal/` is executed
- **THEN** the output SHALL contain ONLY comment-line matches (e.g. `^[^:]+:[0-9]+:\s*//`)
- **AND** production-code matches SHALL be 0

#### Scenario: parent AC4 (6+ global 全删) PARTIAL → PASS

- **GIVEN** parent `openspec/archive/2026-06-17-devrix-tool-surface-contract/acceptance-report.md` shows AC4 as `PARTIAL`
- **WHEN** this change is merged
- **THEN** the parent AC4 status SHALL update to `PASS` (12 → 3 → 0 global vars)
- **AND** no new ACs SHALL be introduced in the parent change

#### Scenario: parent AC14 (SetGlobalXxx API 全删) PARTIAL → PASS

- **GIVEN** parent acceptance-report shows AC14 as `PARTIAL`
- **WHEN** this change is merged
- **THEN** parent AC14 SHALL update to `PASS`
- **AND** all 5 `SetGlobal*` setter functions SHALL be deleted (transcript / flow / workmodel / sessionqueue had no setters / freefork had SetGlobalForker)

#### Scenario: go test -race ./... 100% 绿

- **GIVEN** all 5 sub-commits are merged
- **WHEN** `go test -race ./...` is executed
- **THEN** all tests SHALL pass with 0 race conditions
- **AND** `go vet ./...` SHALL return 0 warnings

## REMOVED Requirements

None — this change is additive cleanup only. Parent change requirements
(REQ-TS-01 through REQ-TS-06) remain authoritative.

## Notes

- **No new ACs** — this change updates 2 parent ACs (AC4, AC14) from PARTIAL to PASS.
  No new acceptance dimensions are introduced.
- **DSAFT compliance** — All changes follow the pattern "struct field injection"
  established by PR #63 phase 1. No DI framework, no service locator, no lazy
  global getters.
- **Test rewrite** — Three test files contain the anti-pattern
  `t.Cleanup(func() { SetGlobalXxx(prev) })`. This change replaces them
  with constructor injection, eliminating the anti-pattern.