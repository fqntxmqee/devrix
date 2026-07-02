# Delta: D7 Orchestration — ProbeToolChannel.Accept 永真 + Read offset/limit + Advisory Bounded

**Change ID:** `devrix-token-design-v2`
**Demand ID:** DM-20260702-008
**Affects:** D7-S9 (Execute — ProbeToolChannel 治本)

---

## ADDED

### Requirement: D7-S9-A50 ProbeToolChannel.Accept 永真 (T09)

`ProbeToolChannel.Accept(call, state)` SHALL 永远 return `(true, nil)`, 触发 `InjectPromptPressure` 软警告:

- 旧: `state.IterationsUsed >= MaxN(15)` → `ErrProbeToolChannelBoundExceeded` 强拒
- 新: 永远 true, 累计 `state.IterationsUsed`, 触发 `InjectPromptPressure`
- 仿 clawcode 无 iteration bound 哲学
- 配套 LTL-Lite Bounded invariant 改 advisory (D5 配套)

#### Scenario: AC Accept 永真

- GIVEN ProbeToolChannel 调用, state.IterationsUsed=20
- WHEN Accept
- THEN return `(true, nil)` (旧: return `(false, ErrProbeToolChannelBoundExceeded)`)
- AND state.InjectionPressure++

#### Scenario: AC InjectPromptPressure 软警告

- GIVEN Accept 触发, state.IterationsUsed ≥ advisory threshold
- WHEN next LLM iteration
- THEN prompt 注入 soft warning "考虑用 read_file offset/limit 自治"
- AND 任务不终止, 走正常 channel 路径

### Requirement: D7-S9-A50 read_file offset/limit (T10)

`read_file` surface SHALL 支持 `ReadInput{Path, Offset, Limit}`:

- 默认 `Offset=0`, `Limit=8192` (兼容旧调用方)
- 仿 clawcode `FileReadTool.ts:497`
- LLM 自治 offset/limit, 不再依赖 D2 截断

#### Scenario: AC offset/limit 默认值兼容

- GIVEN 旧调用方 (无 offset/limit)
- WHEN ReadFile(path)
- THEN 等价于 ReadFile(path, offset=0, limit=8192)
- AND 旧调用方 0 行为变化

#### Scenario: AC offset/limit 真自治

- GIVEN ReadFile(path, offset=100, limit=50)
- WHEN execute
- THEN 读 path[100:150] 返回
- AND 不依赖 D2 截断

### Requirement: D7-S9-A50 Default OpenEnded + Advisory Thresholds (T11)

`orthogonal_flags.go` SHALL 默认 `OpenEnded` + advisory thresholds:

- 旧: read_file/grep/glob = `Bounded(15)` (强)
- 新: `OpenEnded` (永不拒) + advisory thresholds:
  - review: soft@20, hard@30
  - edit: soft@15, hard@20
  - test: soft@18, hard@25

#### Scenario: AC 默认 OpenEnded

- GIVEN 19 工具 surface
- WHEN check ConvergenceContract
- THEN read_file/grep/glob = OpenEnded
- AND write/edit/bash = Action (Bounded 改 advisory)

#### Scenario: AC advisory warning 触发

- GIVEN tool call count ≥ soft threshold
- WHEN next LLM iteration
- THEN prompt 注入 soft warning
- AND 任务不终止

### Requirement: D7-S9-A50 task_kind 推 Advisory (T12)

`per_task_kind.go` SHALL task_kind 维度保留, 阈值改 advisory:

- 旧: `Bounded(15/10/12/8)` for review/edit/test/refactor (hard)
- 新: `advisory@(soft/hard)` 阈值, task_kind 路由保留
- 仿 clawcode task_kind 推 + 软警告

#### Scenario: AC task_kind 路由 + advisory

- GIVEN task_kind=review, agent 调用 read_file 第 20 次
- WHEN per_task_kind check
- THEN 触发 advisory soft warning @ 20
- AND hard warning @ 30
- AND 永不拒 (OpenEnded)

## MODIFIED

### D7-S9 Execute 节点 (Probe/Action/Fact/Experiment 4 Channel)

- `internal/layers/orchestration/mups/execute/toolchannel/probe.go` Accept 改造
- `internal/layers/orchestration/mups/execute/toolchannel/{fact,action,experiment}.go` 新增
- `channel.go` 整合 4 channel
- 仿 clawcode `toolOrchestration.ts:131` 软警告 + channel 永不拒

## Cross-Reference

- d2-spec-delta: PersistToFile (信息不丢) + PerMessageBudget (aggregate 守卫)
- d5-spec-delta: LTL-Lite L4-L6 advisory 配套
- 治本 invariant 量化 (T27): 50 文件 review 旧 15/50 → 新 50/50
- 8K 自我循环治本 = d2 (信息不丢) + d5 (advisory) + d7 (channel 永真) 三件套
