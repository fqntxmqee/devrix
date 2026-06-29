# Design: devrix-d4-dsaft-restructuring (DM-20260629-004)

**Change ID:** `devrix-d4-dsaft-restructuring`
**Demand ID:** DM-20260629-004
**Status:** S3_Design
**Template:** `devrix-d3-dsaft-restructuring` design.md (DM-20260629-003 S7_Archived)

---

## §0 设计目标

把 D4 v1.0.0 演进到 v1.5.0，按 D7/D2/D3 DSAFT 6 子 Change 模板，把 5 类债务落到 8 PR：

```
v1.1 (PR-1)  orchtypes 包 + 治理常量前置
v1.2 (PR-2/3) god-fn split pt1/pt2
v1.3 (PR-4)  registry-sync
v1.4 (PR-5/6) value-flow + span-coverage
v1.5 (PR-7)  boundary-decision
v1.5 (PR-8)  S7_Archive 收口
```

---

## §1 子 Change #0 design（PR-1）

### 1.1 `orchtypes/` 治理包设计

**位置**: `internal/layers/multiagent/orchtypes/`

**目录结构**:
```
internal/layers/multiagent/orchtypes/
├── spans.go              (~21 lines, moved from multiagent/spans.go)
├── events.go             (~30 lines, NEW) — 7 EngineEvent 常量
└── boundary_decision.go  (~30 lines, NEW) — 3 boundary debt 常量 (PR-7 落地，但 file 提前建)
```

**职责**:
- 单一导入路径: `multiagent/orchtypes`
- 跨域治理常量集中（避免 multiagent root 包 + llmgateway/orchtypes 多处分散）
- coverage.RegisterProvider() 钩子集中在 spans.go

### 1.2 `WorkerEngine` inline 设计

**Before**:
```go
// run/worker_engine.go — 独立 44 LOC 文件
type WorkerEngine struct { ... }
func NewWorkerEngine(inner, cfg, agentID) contracts.IEngine { ... }
func (w *WorkerEngine) Process(...) <-chan ... { ... }
```

**After**:
```go
// provision/factory.go — 内嵌私有函数 (~30 LOC 块)
type workerEngine struct {
    inner contracts.IEngine
    agentID, workerRole, systemPrompt, modelTier string
}
func newWorkerEngine(inner, cfg, agentID) contracts.IEngine {
    if inner == nil || cfg.ParentID == "" { return inner }
    return &workerEngine{ ... }
}
func (w *workerEngine) Process(...) <-chan ... { ... }
```

**Caller sync**:
- `provision/factory.go:112` `run.NewWorkerEngine(...)` → `newWorkerEngine(...)`
- `provision/factory.go:119` 同上
- `run/worker_engine_test.go` 删除或迁 `provision/factory_test.go`

### 1.3 `coverage.RegisterProvider` 钩子迁移

**Before**:
```go
// multiagent/spans.go
package multiagent
func init() { coverage.RegisterProvider(spansProvider{}) }
```

**After**:
```go
// multiagent/orchtypes/spans.go
package orchtypes
func init() { coverage.RegisterProvider(spansProvider{}) }
```

**Impact**:
- `coverage` 全局 registry 收集所有 `init()` 调用 — 顺序无关
- 删除 `multiagent/spans.go` → `import _ "multiagent/orchtypes"` 不需要（`init()` 自注册）

---

## §2 子 Change #1 design（PR-2, PR-3）

### 2.1 cli_adapter.go 拆分（PR-2）

| 文件 | 目标 LOC | 内容 |
|------|---------|------|
| `external/cli_session.go` | <300 | CLIConfig/CLISession/CLIAgentTool 字段 + session lifecycle (CloseSession/CleanupBySessionID/Stop/ensureSession/dropSession/closeSession/idleSweeper/reapIdle) + Info/ExecutionTimeout |
| `external/cli_execute.go` | <250 | Execute() + 4 helper (buildCLIPrompt/parseCLIStream/validateCLISession/extractToolCallMetadata) |

**Helper extraction**:
- `cli_execute.go::Execute()` 长 method 内拆 3-4 个 helper：
  - `buildCLIPrompt(input, cfg)` → string
  - `parseCLIStream(stream)` → []Event
  - `validateCLISession(sess)` → error

### 2.2 cursor_adapter.go 拆分（PR-3）

| 文件 | 目标 LOC | 内容 |
|------|---------|------|
| `external/cursor_session.go` | <300 | CursorConfig/CursorAgentTool 字段 + readLoop + CleanupBySessionID/Stop/CloseSession + Info/ExecutionTimeout |
| `external/cursor_execute.go` | <200 | Execute + handleSystem/Assistant/Thinking/ToolCall/Result + helpers (buildCursorRequest/parseCursorStream/emitCursorEvent/formatCursorToolCallLabel/cursorToolCallDetail/truncateCursorDetail) |

---

## §3 子 Change #2 design（PR-4）

### 3.1 18 F 路径全替换 — 见 `tasks.md §T14`

### 3.2 Historical S 沉 archive 设计

**目标 file**: `openspec/archive/2026-06-15-devrix-d4-sa-refine/legacy-s1-s10.md`

**内容**:
- 冻结索引 (D4-S1~S10)
- 迁移路径表

**Source 4 files 删除章节**:
- `a-registry.md` §Historical Modules (~30 lines)
- `f-registry.md` §Legacy 章节 (~117 lines L103-220)
- `t-registry.md` §Historical (~80 lines)
- `d4-domain.md` §Legacy Module Index (~14 lines L58-71)

**Impact**: 4 file 总共 ~241 LOC → archive dir，spec 主体瘦身

---

## §4 子 Change #3 design（PR-5）

### 4.1 ValueFlow Alias 命名规则

- 前缀: `D4_`
- Suffix: 5 canonical S + 1 横切

```yaml
D4-S11 ProvisionAgent → "D4_Provision_Agent"
D4-S12 RunAgentLoop → "D4_Run_Agent_Loop"
D4-S13 IsolateAndMerge → "D4_Isolate_Merge"
D4-S14 ExecuteWorker → "D4_Execute_Worker"
D4-S15 InvokeExternalAgent → "D4_External_Agent_Tool"
D4-S16 ConfigureAgents → "D4_Configure_Agents"
```

### 4.2 影响 5 spec 文件

- `d4-domain.md` §North Star 加 ValueFlow Alias 列
- `a-registry.md` §Header
- `f-registry.md` §Header
- `t-registry.md` §Header
- `layer-delta.md` §Canonical S 加表

---

## §5 子 Change #4 design（PR-6）

### 5.1 7 EngineEvent 治理常量

```go
// internal/layers/multiagent/orchtypes/events.go
package orchtypes

const (
    EventAgentStarted       = "agent.started"
    EventAgentError         = "agent.error"
    EventAgentTerminated    = "agent.terminated"
    EventAgentIterating     = "agent.iterating"
    EventAgentForked        = "agent.forked"
    EventAgentJoined        = "agent.joined"
    EventPermissionRequired = "permission_required"
)
```

### 5.2 Consumer 迁移

- `run/lifecycle.go` 5 emit
- `run/forkjoin.go` 2 emit
- `internal/layers/orchestration/executionflow/bridge/agent_bridge.go` 6 case
- `internal/layers/evolution/guard/observer.go` 2 case

### 5.3 Span Coverage 算法

- Active ops: 6 (D4_Agent_Run/Tool_Call/Fork/Join/Terminate/State_Transition)
- Active events: 7 (EventAgent* + EventPermissionRequired)
- 期望: ≥80% effective (60+/77 T 行映射)

---

## §6 子 Change #5 design（PR-7）

### 6.1 3 boundary debt 常量

```go
// internal/layers/multiagent/orchtypes/boundary_decision.go
package orchtypes

const (
    BoundaryD4ToD7AgentEventBridge    = "boundary-debt:d4-to-d7-agent-event-bridge-v1.0"
    BoundaryD4ToD6EvolutionObserver   = "boundary-debt:d4-to-d6-evolution-observer-v1.0"
    BoundaryD4ForbiddenFlowHubPublish = "boundary-debt:d4-forbidden-flow-hub-publish-v2.0"
)
```

### 6.2 3 单元测试

- Exist (3 常量都存在)
- VersionFormat (regex `^boundary-debt:[a-z0-9\-]+-v\d+\.\d+$`)
- Unique (3 个 ID 互不相同)

---

## §7 关键设计决策

1. **P5 门禁处理**：orchtypes 包基础建立 + 0 aggressive deletion（实际 dead code 少）
2. **Historical S 沉 archive**：复用 `openspec/archive/2026-06-15-devrix-d4-sa-refine/` dir，新文件 ~80 lines
3. **contracts.go 退役策略**：spec v2.0-e 标注；PR-1 不删
4. **Span 覆盖率**：≥80% effective (60+/77 T)
5. **ValueFlow Alias 命名**：`D4_` 前缀
6. **7 EngineEvent 常量化策略**：orchtypes/events.go 治理包
7. **boundary_decision 治理常量**：`boundary-debt:{name}-v{major}.{minor}`

---

**END of Design**
