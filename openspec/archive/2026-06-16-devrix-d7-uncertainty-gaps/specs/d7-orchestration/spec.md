# D7 Orchestration — Uncertainty Gap Closure Spec

**Change ID:** devrix-d7-uncertainty-gaps
**Demand ID:** DM-20260616-001
**版本:** v1.0
**关联:** `../../design.md`

---

## MODIFIED: PlanAgent Read-Only Sandbox

### Feature: Runtime Tool Call Validation

PlanAgent 的只读白名单从纯 prompt 约束升级为运行时强制门控。

#### Scenario: Allowed tool passes validation

```gherkin
GIVEN a PlanAgent with the default read-only whitelist
WHEN ValidateToolCall is called with "read"
THEN no error is returned
```

#### Scenario: Forbidden tool is rejected

```gherkin
GIVEN a PlanAgent with the default read-only whitelist
WHEN ValidateToolCall is called with "write"
THEN an error is returned containing "forbidden in plan mode"
```

#### Scenario: Unknown tool is rejected

```gherkin
GIVEN a PlanAgent with the default read-only whitelist
WHEN ValidateToolCall is called with "unknown_tool"
THEN an error is returned containing "not in the plan mode read-only whitelist"
```

#### Scenario: Nil PlanAgent passes through

```gherkin
GIVEN a nil PlanAgent
WHEN ValidateToolCall is called with "write"
THEN no error is returned (passthrough: no sandbox without PlanAgent)
```

**Mapped A/F/T:**
- D7-S5-A02 (Exploration: PlanMode lifecycle) → F01 (Runtime tool validation)
- D7-S5-A02-F01-T01: whitelist tool passes
- D7-S5-A02-F01-T02: forbidden tool rejected
- D7-S5-A02-F01-T03: unknown tool rejected
- D7-S5-A02-F01-T04: nil receiver safe

---

## MODIFIED: PlanMode LLM Guard

### Feature: PlanMode Enter Validates LLM Availability

PlanMode.Enter() 在 LLM 不可用时立即返回错误，而非在 Execute() 阶段失败。

#### Scenario: Enter with nil LLM returns error

```gherkin
GIVEN a PlanMode created with nil LLM
WHEN Enter is called
THEN ErrLLMNotConfigured is returned
AND the PlanMode state remains Inactive
```

#### Scenario: Enter with valid LLM succeeds

```gherkin
GIVEN a PlanMode created with a valid LLMCompleter
WHEN Enter is called
THEN no error is returned
AND the PlanMode state is Active
```

**Mapped A/F/T:**
- D7-S5-A02 (Exploration: PlanMode lifecycle) → F02 (LLM availability guard)
- D7-S5-A02-F02-T01: nil LLM Enter fails
- D7-S5-A02-F02-T02: valid LLM Enter succeeds

---

## MODIFIED: ConflictGuard Atomic Allow+Register

### Feature: Atomic Conflict Check and Registration

ConflictGuard 提供原子 AllowAndRegister 方法，消除 Allow→Register 之间的 TOCTOU 窗口。

#### Scenario: AllowAndRegister succeeds when no conflict

```gherkin
GIVEN an empty ConflictGuard
WHEN AllowAndRegister is called with a TaskNode in group "A"
THEN the call returns true
AND the task is registered in the guard
```

#### Scenario: AllowAndRegister blocks on conflict group

```gherkin
GIVEN a ConflictGuard with a running task in group "A"
WHEN AllowAndRegister is called with another TaskNode in group "A"
THEN the call returns false
AND the second task is NOT registered
```

#### Scenario: AllowAndRegister allows different groups

```gherkin
GIVEN a ConflictGuard with a running task in group "A"
WHEN AllowAndRegister is called with a TaskNode in group "B"
THEN the call returns true
AND both tasks are registered
```

#### Scenario: AllowAndRegister blocks on file scope intersection

```gherkin
GIVEN a ConflictGuard with a running write task scoped to "src/auth/**"
WHEN AllowAndRegister is called with a write TaskNode scoped to "src/auth/login.go"
THEN the call returns false
```

**Mapped A/F/T:**
- D7-S3-A01 (WaveScheduler: DispatchTask) → F03 (Atomic conflict guard)
- D7-S3-A01-F03-T01: no conflict → registered
- D7-S3-A01-F03-T02: same group → blocked
- D7-S3-A01-F03-T03: different group → allowed
- D7-S3-A01-F03-T04: file scope intersection → blocked

---

## MODIFIED: OrchestratePath FlowEvent Sink

### Feature: OrchestratePath Emits Events to Sink

OrchestratePath 的 emit() 函数将 FlowEvent 推送到 EventPublisher sink，使 IM/WebSocket 可接收编排进度。

#### Scenario: emit pushes to sink when available

```gherkin
GIVEN an OrchestratePath with a non-nil EventPublisher sink
AND a WorkerEvent with Type "text" and Content "task_1 done"
WHEN emit is called
THEN sink.Publish is called with the corresponding EngineEvent
AND the event is also written to the out channel
```

#### Scenario: emit tolerates nil sink

```gherkin
GIVEN an OrchestratePath with a nil EventPublisher sink
WHEN emit is called
THEN no panic occurs
AND the event is written to the out channel
```

**Mapped A/F/T:**
- D7-S3-A01 (WaveScheduler: DispatchTask) → F04 (FlowEvent sink emission)
- D7-S3-A01-F04-T01: event pushed to sink and channel
- D7-S3-A01-F04-T02: nil sink gracefully skipped

---

## REMOVED: PlanModeApproveGate Config

### Feature: Dead Config Removal

移除 `PlanModeApproveGate` 配置项——Approve/Reject 流程由 CLI 命令显式驱动，无需额外配置开关。

#### Scenario: Config struct no longer contains PlanModeApproveGate

```gherkin
GIVEN the Config struct definition
THEN PlanModeApproveGate field does not exist
```

#### Scenario: Default config compiles without PlanModeApproveGate

```gherkin
GIVEN the DefaultConfig function
THEN no reference to PlanModeApproveGate exists
```

**Mapped A/F/T:**
- D7-S5-A02 (Exploration: PlanMode lifecycle) → F05 (Approve gate cleanup)
- D7-S5-A02-F05-T01: field removed from Config
- D7-S5-A02-F05-T02: default config clean

---

## ADDED: Dead Code Markers

### Feature: Deprecated Code Annotation

`LLMFallbackClassifier` 和 `ExecutorSelector` 添加 Deprecated 标记，明确其 v1.1+ 待定状态。

#### Scenario: LLMFallbackClassifier has Deprecated marker

```gherkin
GIVEN the classifier_fallback.go file
THEN the file contains a "Deprecated:" comment
AND the existing tests still pass
```

#### Scenario: ExecutorSelector has Deprecated marker

```gherkin
GIVEN the executor.go file
THEN the file contains a "Deprecated:" comment
AND the existing tests still pass
```

**Mapped A/F/T:**
- D7-S2-A03 (IntentOrchestrate: Classifier) → F06 (Dead code cleanup)
- D7-S2-A03-F06-T01: LLMFallbackClassifier Deprecated, tests pass
- D7-S2-A03-F06-T02: ExecutorSelector Deprecated, tests pass
