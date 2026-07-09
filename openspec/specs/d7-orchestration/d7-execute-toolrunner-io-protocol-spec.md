# Spec: D7 Execute ↔ ToolRunner 4 Channel I/O 协议 (5 场景)

**Domain**: D7 (Orchestration)
**Feature**: execute-toolrunner-io-protocol
**Status**: S2_Proposal
**Versions**: d7-orchestration v4.30.0 → v4.31.0 (follow-up to plan-llm-io-protocol)
**Change ID**: devrix-d7-execute-llm-protocol-doc
**Demand ID**: DM-20260708-005
**Parent**: `devrix-d7-plan-llm-protocol-doc` (DM-20260708-004, S5_Accepted)
**Sibling**: `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, 4 Channel S7_Archived)
**Sibling**: `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, 4 PlanKind S7_Archived)

---

## 1. 范围

本 spec 定义 D7 **Execute 节点** ↔ **ToolRunner** 的 4 Channel I/O 协议。Execute 节点**不直接调用 LLM**, 它通过 `ChannelRouter` 把 `*plan.Plan` 路由到 4 种 Channel (Commit / Protocol / Scenario / Exploration), 每种 Channel 调用一个**可插拔 ToolRunner** 执行 `plan.Step` 序列, 返回 `*wavescheduler.Artifact` 给 Phase 4 Verify。

**关键认知**（与 Observe / Plan 节点的根本区别）：
- **Execute↔ToolRunner 协议 ≠ Execute↔LLM 协议**。Observe/Plan 节点有显式 LLM I/O (frame in / JSON out), Execute 节点**不直接调 LLM** — 它通过 4 Channel 把 `plan.Step` 派发到 ToolRunner (可能是 LLM agent、可能是 shell、可能是 HTTP)。
- **LLM 唯一一次介入 Execute 节点的路径**: `interfaces.FrameDelta.ExecutionMode` 在 Plan↔Execute system_prompt 注入 (≤200 chars), 给 LLM producer 一个 hint。**执行期间不再有 LLM 参与**。
- **4 协议层** (从外到内):
  1. **ChannelRouter.Route** (`channel.go:232`): 路由 `*plan.Plan` → Channel
  2. **Channel.Execute** (`commit.go:75` / `protocol.go:76` / `scenario.go:70` / `exploration.go:85`): 编排 Steps
  3. **ToolRunner.Invoke** (`channel.go:40`): 执行单个 `ToolRequest` → `ToolResult`
  4. **ToolRunner 内部**: 真正的 tool 实现 (shell / HTTP / subagent)

**不变性承诺**: 本 spec **不修改** 任何契约, 只是把散落在 6 个 Go 文件 + 1 个 shared/types 文件里的契约**显式文档化**, 便于 future maintainer / code reviewer / 用户验证。

## 2. 协议总览

```
┌──────────────────────────────────────────────────────────────────────────┐
│  D7 Execute Node (Phase 3 PR-C2)                                        │
│                                                                           │
│  ┌────────────────┐  *plan.Plan (Phase 2 output)                          │
│  │  Plan          │──┐                                                    │
│  │  - Kind (4)    │  │                                                    │
│  │  - Steps[]     │  ▼                                                    │
│  │  - BlastRadius │  ChannelRouter.Route(ctx, p, ChannelRequest)         │
│  │  - Strength    │  (channel.go:232)                                     │
│  │  - ID          │      │                                                │
│  └────────────────┘      │ emit channel.route span                        │
│                          ▼                                                │
│                  ChannelRegistry.Get(PlanKind)                          │
│                  (PlanKind ↔ Channel 1:1 mapping)                        │
│                          │                                                │
│                          ▼                                                │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  4 Channel implementations                                         │  │
│  │  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │  │
│  │  │ CommitChannel    │  │ ProtocolChannel  │  │ ScenarioChannel  │  │  │
│  │  │ (commit.go:39)   │  │ (protocol.go:45) │  │ (scenario.go:37) │  │  │
│  │  │                  │  │                  │  │                  │  │  │
│  │  │ 1 step sync      │  │ N steps seq +    │  │ N probes parallel│  │  │
│  │  │ 5s timeout       │  │ rollback         │  │ + majority vote  │  │  │
│  │  │ IdempotencyKey   │  │ 30s timeout      │  │ 10s timeout      │  │  │
│  │  │ required         │  │ AllowPartial opt │  │ MaxParallel=5    │  │  │
│  │  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘  │  │
│  │           │                     │                     │            │  │
│  │  ┌────────┴────────────────────┴─────────────────────┴──────────┐  │  │
│  │  │ ExplorationChannel (exploration.go:43)                       │  │  │
│  │  │  N experiments parallel + priority sort                     │  │  │
│  │  │  30s timeout, MaxParallel=3                                 │  │  │
│  │  │  SideEffect via sideEffectForScope(PersistScope)            │  │  │
│  │  └────────┬────────────────────────────────────────────────────┘  │  │
│  └───────────┼────────────────────────────────────────────────────────┘  │
│              │                                                            │
│              ▼                                                            │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  ToolRunner.Invoke(ctx, ToolRequest) → (ToolResult, error)         │  │
│  │  (channel.go:40 — pluggable interface, PR-C7 will reconcile       │  │
│  │   with surface.Surface contract)                                   │  │
│  └────────┬───────────────────────────────────────────────────────────┘  │
│           │                                                               │
│           ▼                                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐ │
│  │  ToolRunner implementations:                                        │ │
│  │  - Cursor worker    (CommitChannel → WorkerCursor)                  │ │
│  │  - ClaudeCode worker (ProtocolChannel → WorkerClaudeCode)            │ │
│  │  - SubAgent workers (ScenarioChannel/ExplorationChannel → WorkerSubAgent) │ │
│  └────────┬─────────────────────────────────────────────────────────────┘ │
│           │                                                               │
│           ▼                                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐ │
│  │  *wavescheduler.Artifact                                            │ │
│  │  - Kind:        ArtifactStateChangeCert (Commit)                     │ │
│  │                 ArtifactResponseRecord   (Protocol)                  │ │
│  │                 ArtifactProbeReport      (Scenario)                  │ │
│  │                 ArtifactExperimentData   (Exploration)               │ │
│  │  - SideEffectStatus: SideEffectCommitted (default success)          │ │
│  │                     SideEffectInflight   (timeout)                  │ │
│  │                     SideEffectRolledBack  (rollback OK)              │ │
│  │                     SideEffectNone        (read-only)                │ │
│  │                     SideEffectUnknown     (ctx cancel / err)         │ │
│  │  - WorkerType:   WorkerCursor / WorkerClaudeCode / WorkerSubAgent   │ │
│  │  - Summary, ExitCode, Duration, SideEffectDetail (IdempotencyKey)   │ │
│  └──────────────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────────────────┘
```

## 3. 输入协议: PlanKind + ChannelRequest + ToolRequest

### 3.1 PlanKind (4 选 1 enum)

源: `internal/layers/orchestration/plan/plan.go:30-55`

| # | PlanKind | String | Channel | 触发场景 |
|---|---|---|---|---|
| 0 | `KindUnset` | (空) | (none) | 未分类; wire format 省略字段 |
| 1 | `CommitmentPlan` | `commitment_plan` | CommitChannel | 单步直接命令 (场景 1) |
| 2 | `ProtocolPlan` | `protocol_plan` | ProtocolChannel | 多步幂等协议 (场景 2) |
| 3 | `ScenarioPlan` | `scenario_plan` | ScenarioChannel | 只读并行探测 (场景 3) |
| 4 | `ExplorationPlan` | `exploration_plan` | ExplorationChannel | 沙箱并行实验 (场景 4) |

**PlanKind ↔ Channel 1:1 映射** (`channel.go:130-145` `Register`): `ChannelRegistry` 拒绝两个不同 Channel claim 同一 PlanKind, 防止 wiring 冲突。

### 3.2 ChannelRequest (3 字段)

源: `channel.go:98-107`

| # | 字段 | Go 类型 | Omit 条件 | 角色 |
|---|---|---|---|---|
| 1 | `SessionID` | string | (无) | 会话 ID, 必填 |
| 2 | `PriorVerdictKinds` | []string | (无) | Phase 4 verdict kind 字符串 (PR-C2 留为字符串, 待 Phase 4 type alias 收紧) |
| 3 | `Spec` | `*interfaces.TaskSpec` | nil = legacy | DM-20260629-007 v7.0 TaskContract down-link; 非 nil 时 Channel 优先消费 (PR-A additive) |

**零值合法**: `ChannelRequest{}` 对 legacy 调用方合法 (Spec=nil), 走 legacy extractor (SessionID/PriorVerdictKinds)。

### 3.3 ToolRequest (5 字段)

源: `channel.go:47-53`

| # | 字段 | Go 类型 | 角色 |
|---|---|---|---|
| 1 | `SessionID` | string | 会话 ID, 从 ChannelRequest 透传 |
| 2 | `ToolName` | string | Tool 标识 (e.g. "shell", "http", "subagent") |
| 3 | `Args` | map[string]any | Tool 参数 (e.g. `{"path": "/tmp/x", "directive": "..."}`) |
| 4 | `IdempotencyKey` | string | **PR-C2 AC: side-effecting 工具必填** (CommitChannel 强制; PR-C7 移到 IdempotencyCache 全局检查) |
| 5 | `StepID` | string | 当前 step ID, 用于 audit log + rollback hint 派生 (e.g. "step_1:rollback") |

**rollback 派生** (`protocol.go:191-203`): 复制 `step.ToolArgs` + 加 `__rollback: true` 提示 + 派生 IdempotencyKey `key + ":rollback"` + 派生 StepID `id + ":rollback"`。

### 3.4 Plan.Step 关键字段 (Channel 消费)

源: `internal/layers/orchestration/plan/blast_radius.go:65-72`

| 字段 | 类型 | 必填条件 | Channel 行为 |
|---|---|---|---|
| `ID` | string | 必填 | 用于 Artifact.TaskID + SideEffectDetail.IdempotencyKey 引用 |
| `Directive` | string | 必填 | Tool 的人类可读指令 (audit log) |
| `ToolName` | string | **CommitChannel 必填** (其他 Channel 默认可空) | ToolRunner 路由 |
| `ToolArgs` | map[string]any | 视 tool 而定 | 透传到 ToolRunner.Args |
| `IdempotencyKey` | string | **CommitChannel 必填** | 防重复副作用 |
| `EstimatedTokens` | int | 选填 | ExplorationChannel 排序 tiebreaker |

### 3.5 Plan.BlastRadius (Exploration 消费)

源: `blast_radius.go:13-30`

| 字段 | 类型 | 消费方 |
|---|---|---|
| `FileCount` | int | (无, 4 Channel 均不消费) |
| `APICallCount` | int | (无) |
| `TokenCost` | int | (无) |
| `PersistScope` | PersistScope | **ExplorationChannel 唯一消费** → `sideEffectForScope()` 映射 |

## 4. 输出协议: *wavescheduler.Artifact (4 Kind × 5 SideEffect × 3 WorkerType)

### 4.1 ArtifactKind (4 选 1 enum, 1:1 映射 Channel)

源: `internal/shared/types/execute.go:19-33`

| Kind | 整数 | String | 产生方 | 含义 |
|---|---|---|---|---|
| `ArtifactStateChangeCert` | 0 | `state_change_cert` | CommitChannel | 真实副作用 (DB 写 / HTTP POST / 文件创建) |
| `ArtifactResponseRecord` | 1 | `response_record` | ProtocolChannel | 多步协议的结构化响应 (login → fetch → parse) |
| `ArtifactProbeReport` | 2 | `probe_report` | ScenarioChannel | 只读探测结果, 无副作用 |
| `ArtifactExperimentData` | 3 | `experiment_data` | ExplorationChannel | 探索性实验输出, 喂入 Learn 循环 |

**Wire format**: MarshalJSON 输出 snake_case 字符串 (而非整数), D5 dashboard 过滤器可读。

### 4.2 SideEffectStatus (5 态生命周期)

源: `shared/types/execute.go:108-122`

| 状态 | String | 触发条件 | Channel 默认 |
|---|---|---|---|
| `SideEffectNone` | `none` | 只读工具 | Scenario / PersistTransient Exploration |
| `SideEffectUnknown` | `unknown` | ctx 取消 / 网络分区 | (Channel 出错时降级) |
| `SideEffectInflight` | `inflight` | Tool 已调用, 响应未确认 | CommitChannel timeout (StrategyAskNow) |
| `SideEffectCommitted` | `committed` | 副作用已确认 | Commit/Protocol 成功 / PersistSession-Permanent Exploration |
| `SideEffectRolledBack` | `rolled_back` | 副作用已补偿 | ProtocolChannel 失败 + rollback 成功 |

**`IsTerminal()`**: `None / Committed / RolledBack` → 下游 (Verify / Learn) 可放心处理
**`NeedsAttention()`**: `Unknown / Inflight` → StrategyDecider 必须升级到人工

### 4.3 WorkerType (3 选 1)

| WorkerType | Channel | 含义 |
|---|---|---|
| `WorkerCursor` | CommitChannel | 同步 cursor worker, 短时确定操作 |
| `WorkerClaudeCode` | ProtocolChannel | 异步 claude_code worker, 多步工具 |
| `WorkerSubAgent` | ScenarioChannel / ExplorationChannel | 并行 subagent worker, 探测/实验 |

### 4.4 Artifact 关键字段 (Channel 写入)

源: `wavescheduler` package

| 字段 | 类型 | 写入方 | 用途 |
|---|---|---|---|
| `TaskID` | string | Channel | 唯一 ID, CommitChannel=step.ID, Protocol/Scenario/Exploration=`<sessionID>:<channel>` |
| `SessionID` | string | Channel (透传) | 会话 ID |
| `SourcePlanID` | string | Channel (透传) | 触发此 Artifact 的 Plan.ID (反查溯源) |
| `WorkerType` | enum | Channel 硬编码 | 见 §4.3 |
| `Kind` | enum | Channel 硬编码 | 见 §4.1 |
| `StartedAt` | time.Time | Channel | 起始时间戳 |
| `EndedAt` | time.Time | Channel | 终止时间戳 |
| `Duration` | time.Duration | Channel | 派生 (EndedAt - StartedAt) |
| `ExitCode` | int | Channel | 工具返回码 (0 = success) |
| `Summary` | string | Channel | 人类可读摘要 (Protocol/Scenario 拼接 step output) |
| `Error` | string | Channel (失败时) | 错误描述 |
| `SideEffectStatus` | enum | Channel | 见 §4.2 |
| `SideEffectDetail` | *struct | CommitChannel | 包含 IdempotencyKey / SentAt / ConfirmedAt |
| `AnomaliesCount` | int | Channel (透传) | 从 Plan 透传 |

## 5. 5 场景输入输出协议

### 场景 1: CommitChannel 1 step 成功 → ArtifactStateChangeCert

**输入协议**:

```yaml
Plan:
  Kind: CommitmentPlan
  ID: "plan_commit_001"
  SessionID: "sess_commit"
  Steps:
    - ID: "step_1"
      Directive: "deploy build v2.3.0"
      ToolName: "shell"
      ToolArgs: {cmd: "kubectl apply -f deploy.yaml", namespace: "prod"}
      IdempotencyKey: "deploy:v2.3.0:prod"
      EstimatedTokens: 100
  BlastRadius: {FileCount: 1, APICallCount: 1, TokenCost: 100, PersistScope: PersistSession}
  Strength: 0.85

ChannelRequest:
  SessionID: "sess_commit"
  PriorVerdictKinds: []
  Spec: nil  # legacy
```

**期望输出**:

```go
Artifact{
  TaskID: "step_1",                       // CommitChannel: = step.ID
  SessionID: "sess_commit",
  SourcePlanID: "plan_commit_001",
  WorkerType: WorkerCursor,                // 硬编码
  Kind: ArtifactStateChangeCert,           // 硬编码
  StartedAt: ...,
  EndedAt: ...,
  Duration: 47ms,
  ExitCode: 0,                              // 工具返回 0
  Summary: "kubectl apply done",           // = ToolResult.Output
  Error: "",
  SideEffectStatus: SideEffectCommitted,   // 成功路径
  SideEffectDetail: &SideEffectDetail{
    IdempotencyKey: "deploy:v2.3.0:prod",
    SentAt: 1234567890,
    ConfirmedAt: 1234567937,
  },
  AnomaliesCount: 0,
}
```

**Go 侧处理** (`commit.go:75-149`):
1. **前置校验**: `len(p.Steps) == 1` ✓; `step.ToolName != ""` ✓; `step.IdempotencyKey != ""` ✓
2. **ctx 派生**: `context.WithTimeout(ctx, 5s)` (configurable, default 5s)
3. **ToolRunner.Invoke**: 1 次调用, 成功返回 `ToolResult{ExitCode: 0, Output: "kubectl apply done"}`
4. **Artifact 构造**: `Kind=ArtifactStateChangeCert, SideEffectStatus=Committed, SideEffectDetail=IdempotencyKey+时间戳`
5. **返回**: `(*Artifact, nil)` 给 Phase 4 Verify

**测试**: `TestExecuteTraceE2E_Commit_Success`

### 场景 2: ProtocolChannel 3 steps + step 2 失败 → rollback → SideEffectRolledBack

**输入协议**:

```yaml
Plan:
  Kind: ProtocolPlan
  ID: "plan_protocol_001"
  SessionID: "sess_proto"
  Steps:
    - {ID: "step_1", Directive: "login",      ToolName: "http",  IdempotencyKey: "login:k1"}
    - {ID: "step_2", Directive: "fetch data", ToolName: "http",  IdempotencyKey: "fetch:k2"}
    - {ID: "step_3", Directive: "parse",      ToolName: "shell", IdempotencyKey: "parse:k3"}

ChannelRequest:
  SessionID: "sess_proto"
  PriorVerdictKinds: []
```

**期望输出** (rollback 成功):

```go
Artifact{
  TaskID: "sess_proto:protocol",           // ProtocolChannel: = "<sessionID>:protocol"
  SessionID: "sess_proto",
  SourcePlanID: "plan_protocol_001",
  WorkerType: WorkerClaudeCode,            // 硬编码
  Kind: ArtifactResponseRecord,            // 硬编码
  ExitCode: 1,                              // 至少 1 step 失败
  Summary: "step_0: login done\n--\nstep_1: <empty, failed>",  // 拼接
  Error: "step 1 (fetch) failed: network error",
  SideEffectStatus: SideEffectRolledBack,  // rollback 成功
}
```

**Go 侧处理** (`protocol.go:76-164`):
1. **前置校验**: `len(p.Steps) >= 1` ✓ (3 steps)
2. **ctx 派生**: `context.WithTimeout(ctx, 30s)` (configurable, default 30s)
3. **顺序执行**: step_1 login → ToolRunner ok; step_2 fetch → ToolRunner returns `network error`
4. **检测失败**: `err != nil` → 跳出循环, 记录 `executedSteps=[0]`
5. **rollback** (`protocol.go:184-208`): 用 `context.Background()` (避免外层 ctx 取消), 反向调用 `runner.Invoke` 带 `__rollback: true` + `IdempotencyKey="login:k1:rollback"` + `StepID="step_1:rollback"`
6. **Artifact 构造**: `Kind=ArtifactResponseRecord, SideEffectStatus=RolledBack, Error=...`
7. **ToolRunner 调用总数**: 3 (login + fetch-failed + login-rollback)
8. **返回**: `(*Artifact, fmt.Errorf("protocol_channel: 1/3 steps failed (rolled back)"))`

**关键约束** (`protocol.go:179-183`): rollback **不短路** — 单步 rollback 失败不阻止后续 rollback, 让所有已执行副作用都被尝试补偿。

**测试**: `TestExecuteTraceE2E_Protocol_RollbackSuccess` + `TestProtocolChannel_Step2_Failed_RollbackStep1` (已有)

### 场景 3: ScenarioChannel 5 并行探测, 3 通过 → majority vote → ArtifactProbeReport

**输入协议**:

```yaml
Plan:
  Kind: ScenarioPlan
  ID: "plan_scenario_001"
  SessionID: "sess_probe"
  Steps:
    - {ID: "p_0", Directive: "probe_a", ToolName: "shell", IdempotencyKey: "probe:0"}
    - {ID: "p_1", Directive: "probe_b", ToolName: "shell", IdempotencyKey: "probe:1"}
    - {ID: "p_2", Directive: "probe_c", ToolName: "shell", IdempotencyKey: "probe:2"}
    - {ID: "p_3", Directive: "probe_d", ToolName: "shell", IdempotencyKey: "probe:3"}
    - {ID: "p_4", Directive: "probe_e", ToolName: "shell", IdempotencyKey: "probe:4"}
  BlastRadius: {PersistScope: PersistTransient}  # read-only
```

**期望输出** (3/5 成功, majority pass):

```go
Artifact{
  TaskID: "sess_probe:scenario",
  SessionID: "sess_probe",
  SourcePlanID: "plan_scenario_001",
  WorkerType: WorkerSubAgent,               // 硬编码
  Kind: ArtifactProbeReport,                // 硬编码
  ExitCode: 0,                              // majority pass
  Summary: "scenario: 3/5 probes succeeded (threshold > 2)\nprobe_0: ...\nprobe_1: ...\nprobe_2: ...",
  Error: "",
  SideEffectStatus: SideEffectNone,         // 只读 (无论结果)
}
```

**Go 侧处理** (`scenario.go:70-167`):
1. **前置校验**: `len(p.Steps) >= 1` ✓
2. **ctx 派生**: `context.WithTimeout(ctx, 10s)` (default)
3. **并行启动**: 用 `chan struct{}` semaphore 限流 MaxParallel=5; `sync.WaitGroup` 等待
4. **RH-D7-09 ctx-cancel 优先检查** (`scenario.go:123-130`): 在 wg.Wait 之后, majority vote 之前, 显式 `ctx.Err() != nil` → 立即返回 `ErrChannelCtxCancelled` + `SideEffectUnknown`。**不**走"majority 失败"路径。
5. **majority vote** (`scenario.go:144-145`): `success_count > len/2` → Pass。**3/5 满足** (threshold=2)。
6. **Artifact 构造**: `Kind=ArtifactProbeReport, SideEffectStatus=None, ExitCode=0` (pass) / 1 (fail)
7. **返回**: `(*Artifact, nil)` (pass) / `(*Artifact, fmt.Errorf("%w", ErrChannelStepCountMismatch))` (fail)

**关键不变性**: **SideEffect 永远是 `None`** — 探测只读, ExitCode + Error 表达成败, 不上 SideEffectStatus 编码。

**测试**: `TestExecuteTraceE2E_Scenario_MajorityPass` + `TestScenarioChannel_5ParallelProbes` (已有) + `TestScenarioChannel_MajorityVote_ProbeReport` (已有)

### 场景 4: ExplorationChannel 3 实验, 1 失败 + 优先级排序 → ArtifactExperimentData

**输入协议**:

```yaml
Plan:
  Kind: ExplorationPlan
  ID: "plan_explore_001"
  SessionID: "sess_explore"
  Steps:
    - {ID: "e_0", Directive: "impl_v1 (LRU)",  ToolName: "subagent", IdempotencyKey: "exp:v1", EstimatedTokens: 5000}
    - {ID: "e_1", Directive: "impl_v2 (LFU)",  ToolName: "subagent", IdempotencyKey: "exp:v2", EstimatedTokens: 3000}
    - {ID: "e_2", Directive: "impl_v3 (ARC)",  ToolName: "subagent", IdempotencyKey: "exp:v3", EstimatedTokens: 8000}
  BlastRadius: {PersistScope: PersistTransient}  # 沙箱隔离
```

**期望输出** (2 成功 + 1 失败):

```go
Artifact{
  TaskID: "sess_explore:exploration",
  SessionID: "sess_explore",
  SourcePlanID: "plan_explore_001",
  WorkerType: WorkerSubAgent,               // 硬编码
  Kind: ArtifactExperimentData,             // 硬编码
  ExitCode: 0,                              // top_result 成功
  Summary: "top_result: <v2 output>\n[exploration: 2/3 succeeded, 3 total]",
  Error: "",
  SideEffectStatus: SideEffectNone,         // = sideEffectForScope(PersistTransient)
}
```

**Go 侧处理** (`exploration.go:85-208`):
1. **前置校验**: `len(p.Steps) >= 1` ✓
2. **ctx 派生**: `context.WithTimeout(ctx, 30s)` (default)
3. **并行启动**: semaphore MaxParallel=3 (default); buffered `out chan runOut` 避免死锁; `wg.Wait() + close(out)` 后 drain
4. **RH-D7-09 ctx-cancel 优先检查** (`exploration.go:153-160`): 同 scenario, 提前返回
5. **优先级排序** (`exploration.go:172-182` `sort.SliceStable`):
   - **第一键**: `err == nil` 排前 (成功优先)
   - **第二键**: `CompletedAt - StartedAt` 升序 (时长短优先)
   - **第三键**: `EstimatedTokens` 升序 (token 少优先)
6. **Top result 选**: `results[0]` = 优先级最高的成功结果
7. **Artifact 构造**: `Kind=ArtifactExperimentData, SideEffectStatus=sideEffectForScope(p.BlastRadius.PersistScope), Summary=top_result+统计`
8. **错误处理**: 全部失败时 `mostInformativeError` 选最长 error message (信息量最大)
9. **返回**: `(*Artifact, nil)`

**`sideEffectForScope()`** (`exploration.go:233-243`):
| `p.BlastRadius.PersistScope` | Artifact.SideEffectStatus |
|---|---|
| `PersistTransient` | `SideEffectNone` |
| `PersistSession` | `SideEffectCommitted` |
| `PersistPermanent` | `SideEffectCommitted` (需审计警告) |
| `""` (unknown) | `SideEffectUnknown` (Phase 4 必须 verify) |

**测试**: `TestExecuteTraceE2E_Exploration_PartialSuccess` + `TestExplorationChannel_MultiAgent_Parallel` (已有)

### 场景 5 (混合): Commit timeout + Scenario ctx cancel

**子场景 5a: CommitChannel timeout → SideEffectInflight**

```yaml
Plan: {Kind: CommitmentPlan, Steps: [{ToolName: "slow_kubectl", IdempotencyKey: "k1"}]}
ChannelConfig: {CommitChannelConfig.Timeout: 50ms}
ChannelRequest: {SessionID: "sess_timeout"}
```

**ToolRunner 行为**: `runner.Invoke` 在 50ms 内未返回, 抛 `context.DeadlineExceeded`。

**期望输出**:

```go
Artifact{
  TaskID: "step_1",
  Kind: ArtifactStateChangeCert,
  ExitCode: 0,                              // 注: 仍为 0, 因为 ToolRunner 没设
  Summary: "",
  Error: "commit_channel: tool call timed out (side-effect status uncertain)",
  SideEffectStatus: SideEffectInflight,     // 关键
  SideEffectDetail: &SideEffectDetail{
    IdempotencyKey: "k1",
    SentAt: 1234567890,
    ConfirmedAt: 0,                          // 未确认
  },
}
```

**Go 侧处理** (`commit.go:117-133`):
- **timeout 双重检测**: `ctx.Err() == context.DeadlineExceeded` (外层) OR `errors.Is(err, context.DeadlineExceeded)` (内层) → 走 inflight 分支
- **Artifact 永不为 nil**: 即使 timeout 也构造 Artifact 给 StrategyDecider 决策
- **err 包裹**: 返回 `NewChannelToolCallTimedOutError` (EXEC_CHANNEL_9006) + Artifact

**StrategyDecider 路径**: `NeedsAttention()=true` (Inflight) → **StrategyAskNow** (PR-C3) — 询问用户"该 tool 可能已成功, 是否已实际生效?"

**子场景 5b: ScenarioChannel 外层 ctx cancel → ErrChannelCtxCancelled**

```yaml
Plan: {Kind: ScenarioPlan, Steps: [5 probes]}
ctx: cancel() # before Execute
```

**期望输出**:

```go
Artifact{
  TaskID: "sess_cancel:scenario",
  Kind: ArtifactProbeReport,
  ExitCode: 1,
  Error: "scenario_channel: ctx cancelled (context canceled)",
  SideEffectStatus: SideEffectUnknown,      // 不是 None
}
err = NewChannelCtxCancelledError("scenario", context.Canceled)  // EXEC_CHANNEL_9007
```

**Go 侧处理** (`scenario.go:123-130`):
- RH-D7-09 fix: 早期 `ctx.Err() != nil` 检查 → 跳过 majority vote, 直接返回 cancel error
- **关键**: 不能误报为"majority failed", 否则 StrategyDecider 走错路径 (cancel ≠ probe failure)

**StrategyDecider 路径**: cancel → **StrategyCancel** (turn abort) — 与"探测失败, 是否重试?"完全不同

**测试**: `TestExecuteTraceE2E_CommitTimeout_Inflight` + `TestExecuteTraceE2E_ScenarioCtxCancel_CtxCancelled` + `TestScenarioChannel_CtxCancel_SurfacesCtxError` (已有)

## 6. 闸门契约 (与 d7-plan-llm-io-protocol-spec.md / d7-mups-v4-phase3-prc2-archived.md 的关系)

本 spec **不重复** 已覆盖的契约:

| 闸门 / 决策 | 本 spec 覆盖范围 |
|---|---|
| 4 PlanKind enum (plan.go:30-72) | ❌ 父 spec d7-mups-v4-phase2-prb1-archived.md |
| 4 Channel 实现 (commit/protocol/scenario/exploration) | ✅ **本 spec §3-§5 显式覆盖** |
| ChannelRouter 1:1 路由 (channel.go:130-145) | ✅ **本 spec §3.1 + §6 显式覆盖** |
| ToolRequest / ToolResult 契约 (channel.go:39-53) | ✅ **本 spec §3.3 显式覆盖** |
| ArtifactKind 4 态 (shared/types/execute.go:19-33) | ✅ **本 spec §4.1 显式覆盖** |
| SideEffectStatus 5 态 (shared/types/execute.go:108-122) | ✅ **本 spec §4.2 显式覆盖** |
| `sideEffectForScope()` 映射 (exploration.go:233-243) | ✅ **本 spec §5 场景 4 + §8 显式覆盖** |
| 7 SentinelError (errors.go:30-78) | ✅ **本 spec §9 显式覆盖** |
| FrameDelta 5 字段 (interfaces/mups_frame_delta.go:59-90) | ❌ 父 spec mups-frame-delta-spec.md |
| Rollback hint 派生 (protocol.go:191-203) | ✅ **本 spec §5 场景 2 显式覆盖** |
| Priority 排序 (exploration.go:172-182) | ✅ **本 spec §5 场景 4 显式覆盖** |
| RH-D7-09 ctx-cancel 优先检查 | ✅ **本 spec §5 场景 5b 显式覆盖** |

## 7. 兜底规则 (Go-side invariants)

| 规则 | 触发位置 | 行为 |
|---|---|---|
| Plan nil 拒绝 | `channel.go:236-238` | `p == nil` → `ErrChannelPlanNil` (EXEC_CHANNEL_9001 wrapper) |
| KindUnset 拒绝 | `channel.go:244-248` | `!p.Kind.IsKnown()` → `ErrChannelUnsupported` (EXEC_CHANNEL_9002) |
| Channel 未注册 | `channel.go:249-253` | `registry.Get` miss → `ErrChannelNotFound` (EXEC_CHANNEL_9001) |
| Channel 重复注册 | `channel.go:138-141` | 同 PlanKind 已有 Channel → `ErrChannelUnsupported` wiring 冲突 |
| CommitChannel step count | `commit.go:79-81` | `len(p.Steps) != 1` → `ErrChannelStepCountMismatch` (EXEC_CHANNEL_9003) |
| CommitChannel ToolName | `commit.go:83-85` | `step.ToolName == ""` → `ErrChannelStepInvalid` (EXEC_CHANNEL_9005) |
| CommitChannel IdempotencyKey | `commit.go:86-91` | `step.IdempotencyKey == ""` → fail-fast (PR-C2 AC14) |
| Protocol/Scenario/Exploration empty steps | `protocol.go:80-82` / `scenario.go:74-76` / `exploration.go:89-91` | `len(p.Steps) == 0` → `ErrChannelStepCountMismatch` |
| Runner nil 拒绝 | `errors.go:115-121` (NewChannelToolRunnerNilError) | Constructor → `ErrChannelToolRunnerNil` (EXEC_CHANNEL_9004) |
| ToolCall timeout | `commit.go:117-133` | `ctx.DeadlineExceeded` → `SideEffectInflight` + `ErrChannelToolCallTimedOut` (EXEC_CHANNEL_9006) |
| Ctx cancel 优先 | `scenario.go:123-130` / `exploration.go:153-160` | `ctx.Err() != nil` 跳过 vote → `ErrChannelCtxCancelled` (EXEC_CHANNEL_9007) |
| Rollback 不短路 | `protocol.go:184-208` | 单步 rollback 失败不中断后续 rollback (全部补偿尝试) |
| Rollback 用 `context.Background()` | `protocol.go:185` | 外层 ctx 已 cancel, rollback 仍要跑完 (避免 half-compensated 状态) |
| 排序稳定 | `exploration.go:172-182` | `sort.SliceStable` + 三键 (success / duration / EstimatedTokens) |
| 错误信息丰富 | `exploration.go:200` `mostInformativeError` | 全失败时选 error message 最长的 (triage 友好) |

## 8. sideEffectForScope 映射表 (Exploration 唯一消费)

源: `exploration.go:233-243`

```go
func sideEffectForScope(scope plan.PersistScope) types.SideEffectStatus {
    switch scope {
    case plan.PersistTransient:
        return types.SideEffectNone
    case plan.PersistSession, plan.PersistPermanent:
        return types.SideEffectCommitted
    default:
        return types.SideEffectUnknown  // 空字符串或未识别 scope
    }
}
```

| PersistScope | SideEffectStatus | 含义 | 触发场景 |
|---|---|---|---|
| `PersistTransient` (`"transient"`) | `SideEffectNone` | 不持久化, 沙箱隔离 | 沙箱内 LLM 评估 / dry-run |
| `PersistSession` (`"session"`) | `SideEffectCommitted` | 仅本会话内可见 | 临时文件 / session-only cache |
| `PersistPermanent` (`"permanent"`) | `SideEffectCommitted` | 跨会话持久化 | 真实数据写入 (需审计警告) |
| `""` (空 / 未设置) | `SideEffectUnknown` | Phase 4 必须 verify | Plan 未指定 scope, 默认保守 |

**不变性**: `SideEffectCommitted` ≠ 安全; 真实可逆性由 Phase 4 Verify 评估, Phase 3 只承诺"已确认工具返回成功"。

## 9. Error Code 表 (7 闭集)

源: `errors.go:30-78` (sentinel) + `errors.go:84-154` (NewXxxError helpers)

| Code | Sentinel | 触发条件 | HTTP 类比 | Phase 4 处理 |
|---|---|---|---|---|
| `EXEC_CHANNEL_9001` | `ErrChannelNotFound` | ChannelRegistry 无该 PlanKind 对应 Channel (wiring 漏配) | 500 | 重启 + 重新注册 |
| `EXEC_CHANNEL_9002` | `ErrChannelUnsupported` | Channel.Supports()=false / PlanKind 未知 / Channel 重复注册 | 400 | 拒绝 Plan, 上报 bug |
| `EXEC_CHANNEL_9003` | `ErrChannelStepCountMismatch` | Step 数不符 (Commit 期望 1 / 其它期望 ≥1) | 422 | Plan 重新走 Plan 节点 (PP-1) |
| `EXEC_CHANNEL_9004` | `ErrChannelToolRunnerNil` | Constructor 时 runner=nil (代码 bug) | 500 | 立即崩溃, 不应生产出现 |
| `EXEC_CHANNEL_9005` | `ErrChannelStepInvalid` | Step 字段缺失 (空 ToolName / 缺 IdempotencyKey) | 422 | Plan 重新走 Plan 节点 |
| `EXEC_CHANNEL_9006` | `ErrChannelToolCallTimedOut` | ToolRunner 超时 (SideEffect 状态不确定) | 504 | **StrategyAskNow** (PR-C3) |
| `EXEC_CHANNEL_9007` | `ErrChannelCtxCancelled` | 外层 ctx 取消 (turn abort) | 499 | **StrategyCancel** |

**与 PR-C2 SentinelError 模式对齐** (DM-20260625-001): 所有错误通过 `sharederrors.WithCode(code, msg, err)` 包装, 跨包日志/dashboard 可按 code 聚合。

## 10. Test 覆盖 (5 cases, 全部 NEW)

| # | Test | 场景 | 关键断言 | 已存在 |
|---|---|---|---|---|
| 1 | `TestExecuteTraceE2E_Commit_Success` | §5.1 | 1 step ok → ArtifactStateChangeCert + SideEffectCommitted + SideEffectDetail.IdempotencyKey | 已部分 (TestCommitChannel_CommitmentPlan_OK) |
| 2 | `TestExecuteTraceE2E_Protocol_RollbackSuccess` | §5.2 | step 2 fail → rollback step 1 → ArtifactResponseRecord + SideEffectRolledBack + 3 runner calls | 已部分 (TestProtocolChannel_Step2_Failed_RollbackStep1) |
| 3 | `TestExecuteTraceE2E_Scenario_MajorityPass` | §5.3 | 3/5 pass → ArtifactProbeReport + SideEffectNone + majority vote 正确 | 已部分 (TestScenarioChannel_MajorityVote_ProbeReport) |
| 4 | `TestExecuteTraceE2E_Exploration_PartialSuccess` | §5.4 | 2/3 成功 + 优先级排序 → ArtifactExperimentData + top_result 选中 + sideEffectForScope(PersistTransient)=None | 已部分 (TestExplorationChannel_MultiAgent_Parallel) |
| 5 | `TestExecuteTraceE2E_CommitTimeout_Inflight` | §5.5a | 50ms timeout → SideEffectInflight + ErrChannelToolCallTimedOut (EXEC_CHANNEL_9006) | 已部分 (TestCommitChannel_Timeout_InflightSideEffect) |

**注**: **已有 23 个 test** (`execute_test.go`, 797 lines) 覆盖 4 Channel golden-path + 错误-path + 并发 + ctx-cancel。**NEW 5 trace test** 主要增强:
- **统一 trace 打印**: 模拟 observe/plan 节点的 `printPlanBanner` / `printPlanProposal` 模式, 每个 case dump ChannelRequest → ToolRequest → Artifact 完整 I/O
- **sideEffectForScope 三态覆盖**: PersistTransient / PersistSession / PersistPermanent 各一个 test
- **混合场景**: Commit timeout + Scenario ctx cancel 串成一个 case 验证两个 EXEC_CHANNEL_9006/9007 不混淆

**运行**:

```bash
go test -v -run TestExecuteTraceE2E \
  ./internal/layers/orchestration/mups/execute/...
```

**当前状态**: 5/5 NEW (2026-07-09 planned), 22/22 orchestration packages go test -race 回归 (待 S5 验证)。

## 11. 关系网

### 上游
- `devrix-d7-plan-llm-protocol-doc` (DM-20260708-004) — 兄弟 spec, 同模板
- `devrix-d7-observe-llm-protocol-doc` (DM-20260708-003) — 兄弟 spec
- `devrix-d7-mups-v4-phase2-prb1` (DM-20260623-001, S7_Archived) — `plan.Plan` 4 PlanKind
- `devrix-d7-mups-v4-phase3-prc2` (DM-20260625-001, S7_Archived) — 4 Channel router (本 spec 的 Go-side 实现 SoT)
- `devrix-d7-mups-v4-phase3-prc1` (DM-20260625-001) — `*wavescheduler.Artifact` 4 态
- `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) — FrameDelta 5 字段契约 (Execute system_prompt 注入)
- `devrix-d7-taskcontract-unification-pr-a` (DM-20260629-007) — `ChannelRequest.Spec` v7.0 down-link
- `devrix-d7-taskcontract-unification-pr-b` (DM-20260629-008) — `ChannelRouter.pessimisticGuard` PR-B additive
- `devrix-d7-route-collapse-taskctx-fix` (DM-20260626-005) — taskCtx cancel sync.Once 释放
- `devrix-d7-dsaft-restructuring` (DM-20260629-001) — execute 4 Channel span 30%→94%
- RH-D7-09 (DM-20260630-013) — scenario/exploration ctx-cancel 优先检查 fix

### 下游
- Phase 4 Verify 消费 `Artifact` 决定 VerdictPass/Fail
- Phase 5 Learn 消费 `Artifact` 更新 ReputationEvidence
- 任何新增 `Channel` (e.g. GraphRAGChannel) 必须扩 `plan.go:30-72` + 新 `Channel` 实现 + 注册到 `ChannelRegistry`
- PR-C7 Executor (待做) 整合 4 Channel + ToolRunner.Surface + IdempotencyCache

### 关联 PR
- #473 (Observe trace validation 16 tests + spec) — 兄弟 spec
- #474 (Plan trace validation + spec) — 兄弟 spec
- 未来 PR: 5 trace test + 本 spec

## 12. 沉淀物清单

| 类型 | 路径 | 描述 |
|---|---|---|
| Spec | `openspec/specs/d7-orchestration/d7-execute-toolrunner-io-protocol-spec.md` | 本文档 |
| Test | `internal/layers/orchestration/mups/execute/execute_trace_e2e_test.go` | 5 NEW trace test |
| Demand | `openspec/changes/devrix-d7-execute-llm-protocol-doc/demand.md` | DM-20260708-005 S1 |
| Proposal | `openspec/changes/devrix-d7-execute-llm-protocol-doc/proposal.md` | S2 lite |
| Tasks | `openspec/changes/devrix-d7-execute-llm-protocol-doc/tasks.md` | 19 T-points |

## 13. 后续工作 (不在本 Change 范围)

- 把本 spec 合并到主 `openspec/specs/d7-orchestration/spec.md` (lite-mode) — S6 归档时决策
- 增强 `sideEffectForScope`: PersistPermanent 加 audit 警告 (e.g. `art.Metadata["audit_warning"]="permanent_scope"`)
- 跨语言 trace: 把 4 Channel I/O 协议 (本 spec §3-§4) 也用 OpenAPI/JSON Schema 形式化, 便于其他语言 client
- 增强 PessimisticCommitGuard (PR-B additive): 在 Channel 失败时插入"悲观兜底" Artifact
- Channel 性能 profile: 加 channel-specific span attributes (commit 1 step, protocol N steps, scenario parallel, exploration priority sort)
- 单元化 Channel: 把 4 Channel 抽到 `mups/execute/channels/<name>/` 子包, 每个子包独立 go.mod (类似 D5 sub-package pattern)
