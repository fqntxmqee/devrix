# Design — devrix-d7-six-s-simplification (DM-20260626-001)

**Change ID:** `devrix-d7-six-s-simplification`
**Demand ID:** DM-20260626-001
**Status:** S3_Design → S4_Implemented → S5_Accepted → S7_Archived (2026-06-26)

---

## 1. 6 S + 1 横切博弈角色详细定义

### 1.1 S1 WorkModel — State Authority

**核心角色：** 状态权威（所有跨 turn 共享状态的单一来源）

**保留：**
- `orchestration/workmodel/` (4658 行)
- WorkItem CRUD + DAG
- PlanMode 状态机
- DiskWorkItemStore (schema v2)
- UncertaintyCoord / ReputationEvidence（从 S12 散落字段上提）
- AdaptivePrior（3 层 fail-safe Bayesian 状态）

**A 数：4**（WorkItem + DAG + ReputationEvidence + UncertaintyCoord）

**并入：** 原 S12 散落的 UncertaintyCoord / ReputationEvidence 字段上提 + AdaptivePrior 状态机整合

### 1.2 S2 SessionOrchestrator — Mediator + Turn Leader + Error Recovery

**核心角色：** 协调者 + Turn 主循环 + 错误恢复

**保留：**
- `orchestration/sessionorchestrator/` (含 `command_handler.go` + `observe_request.go` + `autoclose.go`)
- `orchestration/turn/` (RunTurn 主循环)
- d7.IOrchestrationEntry ingress
- ResumeSession 3 决策路由（A fall through / B user_accept / C user_cancel）
- AutoClose 4 规则

**并入：**
- S7 Pipeline Coordinator 角色（即 sessionorchestrator 自身）
- S12 buildObserveRequest 3 层 fail-safe（归属为 S2 内部子函数）
- S13 processAutoClose（已同址）
- S14 EscapeEngine **调度入口**（实际 Engine 仍在独立包）

**A 数：7**（含 RunTurn + Ingress + ResumeSession + AutoClose + EscapeDispatch + ObservabilityHookup + ErrorRecovery）

### 1.3 S3 WaveScheduler — Mechanism Designer

**核心角色：** 机制设计者（博弈规则的实施者）

**保留：**
- `orchestration/wavescheduler/` (2033 行)
- Pool / Task / WaveDAG
- 资源调度 + 准入控制

**A 数：4**（ScheduleWave + DispatchLoop + ConflictGuard + ResourceAdmission）

### 1.4 S4 ExecutionFlow+Verify — Costly Signaler + Certifier

**核心角色：** 高成本信号发送者 + 验证者

**保留：**
- `orchestration/executionflow/` (631 行)
- FlowEvent 聚合
- ExecutionFlowHub

**并入：**
- 原 S10 Verify：Verifier + 4 态 Verdict + 14 ExitReason（从 `turn/ExitReason` + `observe/verify/` 上提）
- VerifyWithRetry 3 次兜底
- AggregateVerdicts 跨 Verdict 合并

**A 数：9**（FlowEvent 聚合 + Hub + Verifier 验证 + 14 ExitReason + 4 态 Verdict + VerifyWithRetry + AggregateVerdicts + SystemAnomalyDetector + ExitReasonMapping）

**新增 `executionflow/verify/` 子包（v6.0.0）：**
- `anomaly.go`：DetectSystemAnomaly wrapper，4 AnomalyKind + 4 Severity
- 保持 LP-1 兼容：调用 `orchtypes.EvaluateSystemAnomaly` 复用 Phase 4 PR-D4 wiring

### 1.5 S5 DecisionPlanning+Observe — Info Producer + Quantizer

**核心角色：** 信息生产者 + 量化者

**保留：**
- `orchestration/decisionplanning/` (755 行)
- ClassifyIntent (Command-first)
- RuleClassifier / ShadowClassifier
- 4 IntentKind

**并入：**
- 原 S8 observe/：ObserveNode + UncertaintyReport + 4 类 Observation + IntentQuantizer + AnomalyDetector（部分）
- `orchestration/observe/orchtypes/` 14 文件 / 1572 行

**A 数：8**（ClassifyIntent + IntentQuantize + UncertaintyReport + ObserveNode + TaskDecompose + IntentKindMapping + ClassifierShadow + 未知意图 fallback）

### 1.6 S6 MUPS Pipeline — Pipeline Coord + Memory Curator

**核心角色：** 5 节点管道协调者 + 记忆管理者

**保留：**
- `orchestration/execute/` (1090 行)
- 4 Channel + ChannelRouter (PR-C2)
- C2/W8 1:1 映射

**并入：**
- 原 S11 learn/：Learner + 4 类 LearningAsset + 3 通道记忆 (skill/feedback/scheduled)
- `orchestration/learn/` 8 文件 / 1972 行

**A 数：15**（4 Channel + ChannelRouter + 3 通道记忆 + 5 类 Asset + Learning 注入 + ReputationUpdate + CrossSession + ObserverLoop + 5 节点接线 + 错误恢复）

### 1.7 Cross-cutting: Hardening — Discipline Keeper

**核心角色：** 横切纪律守护者

**实现：** `orchestration/hardening/` (NEW)
- `metrics.go`（从散落各包上提）
- `circuit_breaker.go`（从 `escape/circuit_breaker.go` 拆分监控部分）
- `error_recovery.go`（S2 调用的横切恢复逻辑）

**A 数：2**（MetricsCollector + ErrorRecoveryPolicy）

## 2. 5 个新 P0/P1 Span 详细设计

### 2.1 `D7_Channel_Route` (S6-A48 P0)

**触发点：** `ChannelRouter.Route` 入口（`execute/channel.go:206`）

**Span 属性：**
- `session_id` — Plan.SessionID
- `channel.kind` — Channel.Name()（commit / protocol / scenario / exploration）
- `plan.kind` — Plan.Kind.String()
- `score` — Plan.Strength (float64 [0,1]) 3-decimal
- `fallback` — bool "true"/"false"（未来若支持 fallback 选择路径）

**失败路径覆盖：**
- `!p.Kind.IsKnown()` → `end(err)` 记录 UnknownKind 失败
- `r.registry.Get(p.Kind)` 失败 → `end(err)` 记录 NotFound 失败
- `ch.Execute` 失败 → `end(err)` 记录 ChannelExecute 失败

### 2.2 `D7_Memory_Persist` (S6-A49 P0)

**触发点：** `SkillMemory.Store` 入口（`learn/memory.go:141`）

**Span 属性：**
- `session_id` — asset.SessionID
- `channel` — "skill" (MemorySkill.String())
- `asset.kind` — asset.Class.String()（LearningSOP / LearningProtocol）
- `ttl_ms` — `ttlRemainingMs(asset)`（clamped ≥ 0）
- `payload_size` — `assetPayloadSize(asset)`（AssetKey + ContentHash + SessionID 长度加总）

**注释（设计决策）：** 5 个 AssetContent 派生类型（PendingAssetContent / SOPAssetContent / ProtocolAssetContent / KnowledgeAssetContent / ConclusionAssetContent）不反射计算 size，用 AssetKey + ContentHash + SessionID 长度作为 proxy（保持 Span attribute 计算 O(1)）

### 2.3 `D7_System_Anomaly_Detect` (S4-A47 P0)

**触发点：** NEW `executionflow/verify/anomaly.go::DetectSystemAnomaly`

**Span 属性：**
- `session_id` — 调用方传入
- `anomaly.kind` — AnomalyKind.String()（cat_system_aggregate / rate_spike / quota_exceeded / schema_violation / verifier_abstain_loop）
- `severity` — Severity.String()（none / low / medium / high）
- `threshold` — strconv.Itoa(threshold)（默认 3）
- `evidence_id` — `sessionID:total:catSystem` 格式

**4 AnomalyKind 设计：**
- `cat_system_aggregate`：v6.0.0 默认（Phase 4 PR-D4 bool aggregator）
- `rate_spike` / `quota_exceeded` / `schema_violation` / `verifier_abstain_loop`：预留 v6.1 4 类探测器，调用方通过 `overrideKind` 参数显式设置

**4 Severity 设计（按 ratio 派生）：**
- ratio = 1.0 → high
- ratio ≥ 0.75 → medium
- ratio < 0.75 (但 triggered) → low
- !triggered → none

**LP-1 兼容：** wrapper 调用 `orchtypes.EvaluateSystemAnomaly`（Phase 4 PR-D4 已存在的 wiring），不动 UncertaintyCoord Value=0.95 路径

### 2.4 `D7_TaskGraph_Synthesize` (S5-A33 P1)

**触发点：** `TaskDecomposer.SynthesizeTaskGraph` validation 后（`decisionplanning/decomposer.go:57`）

**Span 属性：**
- `session_id` — 调用方传入
- `taskgraph.node_count` — `len(nodes)` 字符串
- `taskgraph.edge_count` — `sum(len(n.DependsOn))` 字符串
- `taskgraph.dag_depth` — `dagDepth(nodes)` 字符串
- `taskgraph.cycle_detected` — `hasCycle(nodes)` 字符串

**dagDepth helper：**
- 最长路径 BFS（拓扑式深度）
- 起点：DependsOn 为空的根节点
- 深度 = 1 + max(dependency depth)
- cycle 时 cap 在 len(nodes)（避免无限递归）
- 复杂度：O(V + E)

**为什么在 validation 后 emit：** cycle_detected 应该是 validated truth 而非 raw input；放在 validateGraph 之后保证 Span 反映最终判定

### 2.5 `D7_Executor_Select` (S5-A34 P1)

**触发点：** `WaveScheduler.dispatchOne` runner 查找前后（`wavescheduler/scheduler.go:325`）

**Span 属性：**
- `session_id` — 调用方传入
- `candidates_count` — `len(s.runners)` 字符串
- `selected_kind` — `string(node.WorkerType)` 或 "none"（未命中）
- `score` — "1.000"（命中）/ "0.000"（未命中）
- `policy` — "kind_match"（当前 selection 是 map lookup）

**为什么选 dispatchOne 入口：** 这是 S3 WaveScheduler 派发的实际 selection 点；runner 命中后 dispatch goroutine 已经在 Wave_Schedule span 下，Executor_Select 不会增加额外的 span 树深度

## 3. 关键设计决策

### 3.1 `d7spans` 包级别 bridge setter

**设计动机：** 5 个 Span emit 点分布在 5 个不同包（execute / learn / decisionplanning / executionflow/verify / wavescheduler），每个包都注入 obsBridge 会让构造器签名扩散。

**设计实现：**
- 包级别 `var bridge *observability.Bridge` + `sync.RWMutex`
- `SetBridge(b *observability.Bridge)` 由 `bootstrap/wire_coordinator.go` 一次调用
- 每个 emit 函数读包级别 bridge（无锁或 RLock），nil/disable 时整体 no-op
- 调用约定：emit 返回 `func(error)` 闭包，调用方在函数返回前调用 `end(nil)` 或 `end(err)`

**不重写构造器的优势：**
- 5 个包零构造器改
- 跨包传递 obsBridge 的依赖图 0 变化
- 后续新增 Span emit 点只需调用 `d7spans.EmitXxx()` 一行

### 3.2 `executionflow/verify/anomaly.go` wrapper 模式

**设计动机：** Phase 4 PR-D4 的 `orchtypes.EvaluateSystemAnomaly` 已有完整 wiring（UncertaintyCoord Value=0.95 传播），直接修改会破坏 LP-1 兼容。

**设计实现：**
- wrapper 复用 `orchtypes.EvaluateSystemAnomaly` 计算 triggered
- 在 wrapper 内独立计算 ratio / severity / evidence_id（4 Severity + 4 AnomalyKind 都派生）
- Span emit 包装在 wrapper 内部，对外保持简单签名

**未来 v6.1 扩展：**
- 4 个 AnomalyKind（rate_spike / quota_exceeded / schema_violation / verifier_abstain_loop）由调用方通过 `overrideKind` 参数显式传入
- DetectSystemAnomaly 内部再增加 4 个具体的 Detector 子函数（每个独立 emit 自己的 Span）

### 3.3 taskgraph.synthesize 在 validation 后 emit

**设计动机：** cycle_detected 应该反映 validated truth，避免 Span 与 ValidationReport 矛盾。

**设计实现：**
- `validation := d.validateGraph(nodes)` 在前
- `hasCycle(nodes)` 在后（虽然 validateGraph 内部已经检测过，这里再调一次保证 Span attribute 准确）
- 不依赖 ValidationReport 的内部状态（避免耦合）

### 3.4 executor.select 在 dispatchOne 入口 emit

**设计动机：** WaveScheduler 派发前的 selection 决策点。

**设计实现：**
- emit 早于 `runner, ok := s.runners[node.WorkerType]` 查找
- score 1.000/0.000 二态反映当前 selection 是 map lookup
- 未来若实现多 candidate 评分（priority / load / capability），score 改为 float64 [0,1]

## 4. 测试设计

### 4.1 d7spans 包：10 个 T 点（T01-T10）

| T ID | 测试 | 路径 |
|------|------|------|
| D7-S6-A48-T01 | EmitChannelRoute happy path | 5 emit + 5 fail-safe |
| D7-S6-A48-T02 | EmitChannelRoute nil-bridge | |
| D7-S6-A49-T03 | EmitMemoryPersist happy path | |
| D7-S6-A49-T04 | EmitMemoryPersist nil-bridge | |
| D7-S4-A47-T05 | EmitSystemAnomalyDetect happy path | |
| D7-S4-A47-T06 | EmitSystemAnomalyDetect nil-bridge | |
| D7-S5-A33-T07 | EmitTaskGraphSynthesize happy path | |
| D7-S5-A33-T08 | EmitTaskGraphSynthesize nil-bridge | |
| D7-S5-A34-T09 | EmitExecutorSelect happy path | |
| D7-S5-A34-T10 | EmitExecutorSelect nil-bridge | |

**fail-safe 测试设计：** 调 `resetBridge()` 清空包级别 bridge，验证 emit 仍然返回非 nil 闭包且 `end(nil)` 不 panic

### 4.2 executionflow/verify 包：6 个 T 点（T11-T16）

| T ID | 测试 | 覆盖 |
|------|------|------|
| D7-S4-A47-T11 | triggered + SeverityHigh | ratio = 1.0 边界 |
| D7-S4-A47-T12 | triggered + SeverityMedium | ratio = 0.8 边界 |
| D7-S4-A47-T13 | not triggered + SeverityNone | ratio < 0.5 |
| D7-S4-A47-T14 | overrideKind (forward-compat) | 4 AnomalyKind 切换 |
| D7-S4-A47-T15 | nil-bridge fail-safe | |
| D7-S4-A47-T16 | default thresholds (count=0, ratio=0) | workmodel.Default* 派生 |

### 4.3 decisionplanning 包：4 个 T 点（T17-T20）

| T ID | 测试 | 覆盖 |
|------|------|------|
| D7-S5-A33-T17 | dagDepth empty graph | nil 输入 |
| D7-S5-A33-T18 | dagDepth linear chain | 链式 t1→t2→t3→t4 = 4 |
| D7-S5-A33-T19 | dagDepth branching graph | 钻石型 t1→t2,t3→t4 = 3 |
| D7-S5-A33-T20 | SynthesizeTaskGraph Span emit fail-safe | "A → B → C" goal → 3 nodes 验证 |

## 5. 跨包依赖图

```
bootstrap/wire_coordinator.go
  └── d7spans.SetBridge(obsBridge)

5 Span emit points:
  ├── execute/channel.go
  │     └── d7spans.EmitChannelRoute
  ├── learn/memory.go
  │     └── d7spans.EmitMemoryPersist
  ├── executionflow/verify/anomaly.go
  │     ├── d7spans.EmitSystemAnomalyDetect
  │     └── orchtypes.EvaluateSystemAnomaly (LP-1 兼容)
  ├── decisionplanning/decomposer.go
  │     └── d7spans.EmitTaskGraphSynthesize
  └── wavescheduler/scheduler.go
        └── d7spans.EmitExecutorSelect

d7spans/emitter.go
  └── observability.Bridge (already exists)
```

**依赖图特点：**
- 0 循环：5 emit 点 → d7spans → observability（单向）
- 0 构造器改：5 emit 点不接收 obsBridge 参数
- LP-1 兼容：executionflow/verify 仅 wrapper，复用现有 orchtypes 路径
