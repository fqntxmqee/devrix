# Delta: Communication Layer — V3 Integration

**Change ID:** devrix-v3-integration
**Demand ID:** DM-20260608-010
**Affects:** D1-S1 Gateway, D1-S2 Adapters, D1-S5 Milestone, D1-S8 Renderers
**Parent:** devrix-v3 (DM-20260608-008)

---

## ADDED

### Requirement: L5 Registry Alignment

V3 验收锚点 MUST 使用 `L5-{D}-{S}-{NN}` 格式登记于 `openspec/l5-registry.md`。

#### Scenario: No dangling L5 IDs
- GIVEN DM-20260608-008 used temporary `L5-COMM-14`~`18`
- WHEN DM-20260608-010 completes S3
- THEN each temporary ID maps to a registered L5-1-* ID
- AND registry Status is PLANNED until S5

---

### Requirement: Milestone Cycle Test Coverage

#### Scenario: Reject cyclic dependency
- GIVEN milestones m1→m2→m3 in DAG
- WHEN AddDependency(m3, m1) is attempted
- THEN error is returned
- AND Covers annotation references L5-1-5-01

---

### Requirement: TaskFlow Multi-Milestone Chain

#### Scenario: Complete two-step chain
- GIVEN TaskFlow DAG with m2 depending on m1
- WHEN Start then CompleteMilestone for m1 then m2
- THEN TaskFlow status is completed
- AND OverallProgress is 1.0

---

### Requirement: DingTalk Milestone Outbound Rendering

#### Scenario: Render milestone card on outbound
- GIVEN OutboundMessage with Metadata render=milestone
- WHEN DingTalkAdapter.OnMessage handles the message
- THEN session webhook receives CardRenderer markdown output
- AND plain-text-only path unchanged when render metadata absent

---

### Requirement: IM Instance Lifecycle

#### Scenario: Register on boot, unregister on shutdown
- GIVEN devrix-dingtalk or devrix-feishu starts
- WHEN main completes graceful shutdown
- THEN instance is removed from InstanceRegistry

---

## MODIFIED

### Requirement: Multi-Entry Configuration Documentation

`openspec/specs/project/config-environment.md` §5 MUST list `cmd/devrix-dingtalk/main.go` with config sources.

---

## REMOVED

### Requirement: V1 TaskFlow Stub

`internal/layers/communication/task_flow.go` misleading "not implemented in V1" handler MUST be removed or replaced; no log hinting V3 is future work.

---

## NOT IN SCOPE (unchanged from DM-008)

- DingTalk WebSocket Connect()
- Prometheus `/metrics` on communication layer
- Load balancer sticky sessions
- TaskFlow replacing PEV milestone_runner
