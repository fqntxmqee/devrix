# Design: MUPS 5 节点 × Tool 元数据 Control Plane + Channel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Stage:** S3_Design
**Created:** 2026-07-01
**Architecture Framework:** 六段式（详见 `../../specs/project/architecture-design.md`）

---

## ① 架构目标

### 1.1 业务目标

LLM 在 Execute 阶段"探索"和"产出"混为同一种 emit text 行为，**没有"该停了"的内生信号** — 这是 2026-07-01 sess_1782885908460_4000 中 LLM 反复 `read_file` × 9 + 0 synthesize review 的根因。PR #373 修了 D1 表现层红卡，但**根因未动**。本设计要做的是 **让 tool 自收敛**：每个 tool 自带 `emission_class` + `convergence_contract` + `iteration_bound` 三个自约束，Execute 节点按 emission_class 路由 4 Channel，每个 Channel 在 register time 就挂 LTL-Lite L4-L6 termination invariant。

### 1.2 技术目标

| 指标 | 基线 | 目标 |
|------|------|------|
| ToolSpec 字段数 | 9 (v2) | **15 (v3)** 含 6 新 control plane 字段 |
| LTL-Lite invariant 类型 | **无 ltl/ 子包**（observability/instrument/ 现仅 logger/metrics/telemetry/tracer） | **L4 (Bounded) + L5 (Quotient) + L6 (Synthesize)** — 本 change 在 observability/instrument/ltl/ 新建子包 |
| Verify input contract 强校验 | 无 | **4 元组** (deliverable / evidence / source_uncertainty / emission_class) |
| Verdict 全链路透传 | verdict.Reason 局部变量丢失 | `meta["verify_exit_reason"]` 透传到 D1 + **Learn FeedbackMemory** |
| Filter 维度 | agent+risk 2 维 | **agent+emission_class+task_kind 3 维**（workspace defer，OOS-10） |
| 截断透明度 | D2 ToolResultBudget 隐藏截断无 marker | **TruncateMarker** 强制附加，LLM 知道 complete=false |

### 1.3 约束条件

- **SemVer**：v1.0 → v3.0 across 4 PR 联动；**0 行为变更**（现有 9 字段不动，新增 6 字段有 R3 cycle 0 默认值）
- **现有契约稳定**：ToolSpec v3 必须 100% 向后兼容 v2 (无 breaking change)
- **物理目录稳定**：0 git mv / 0 git rm
- **测试覆盖**：Phase A 单元 + Phase B 单元 (probe_channel_test 3+) + Phase C 集成 + Phase D 集成
- **CI 必过**：`go test -race ./...` 全过 + `verify-archive.sh` 11/11 PASS

---

## ② 架构原则

### 2.1 设计原则（前 7 条已是宪法）

| # | 原则 | 应用位置 |
|---|------|----------|
| P1 | 元数据是 control plane，不是声明 | ToolSpec v3 runtime 必查 |
| P2 | 每个 tool 是 self-bounded actor | tool 自身带 iteration_bound |
| P3 | LLM 自我循环由工具层强收敛 | ProbeToolChannel Bounded(n) |
| P4 | Verify input contract 强校验 | VerifyContract 4 元组 |
| P5 | Verdict 全链路透传 | meta["verify_exit_reason"] 7 跳 |
| P6 | 截断对 LLM 透明 | TruncateMarker |
| P7 | 4 类正交下沉到 tool 级 | emission_class |

### 2.2 命名规范

| 类型 | 模板 | 例 |
|------|------|-----|
| **ID** | `D{N}-S{N}-A{XX}-T{XX}` (DSAFT) | `D7-S9-A50-T01` |
| **Type** | PascalCase | `VerifyContract`, `EmissionClass` |
| **Function** | camelCase | `computeCalibratedConfidence` |
| **Field** | PascalCase (struct) / camelCase (json) | `IterationBound.MaxN` / `iteration_bound.max_n` |
| **Error** | `sentinel errors.Errorf("%w")` | `ErrProbeChannelBoundExceeded` |
| **Span** | `D{N}_S{N}_A{XX}_{Op}` | `D7_S9_A50_Channel_Route` |
| **Metric** | `mups.{node}.{action}` | `mups.execute.channel.tool_call_total` |

### 2.3 代码风格

- **函数** < 50 行
- **文件** < 800 行
- **包内聚**：D2 = `enforce/tools/{surface,filter,sandbox,compression}/`；D7 Execute = `executionflow/execute/`
- **文件命名**：每 Channel 一个文件 (`fact_channel.go`, `probe_channel.go`)
- **no global** — Channel router 通过构造函数注入

### 2.4 Equilibrium Concept（H7，P1-AC-1）

本 change 的目标均衡 **双声明**：

| 范围 | 均衡概念 | 机制 |
|------|----------|------|
| **单 session** | **Subgame Perfect Equilibrium (SPE)** | ProbeToolChannel `Bounded(n)` hard reject @n+1 为 commitment device；PromptPressure 为 Schelling focal point（cheap talk，不单独依赖） |
| **跨 session** | **Reputation equilibrium** | Phase C 最小接入：`verify_exit_reason` → Learn `FeedbackMemory`；完整 `(declared_class, actual_behavior)` drift 表留 Phase E |

**Nested Stackelberg 结构**（双 review 共识）：
1. **Platform（D7）** 先动：注册 ToolSpec v3 + ToolChannel invariants（不可观察类型 → 可观察 routing）
2. **LLM（Agent）** 后动：在已知 bound 下选 tool_call vs synthesize
3. **Verify** 作为 **principal's audit**：按 EmissionClass 分配 burden of proof（demand §3.3）
4. **Learn** 作为 **punishment/reputation**：跨 session 累积 verify 失败信号

### 2.5 Learn 边界（H6/H10）

| 本 change In Scope | Phase E / OOS-11 |
|--------------------|------------------|
| `verify_exit_reason` 写入 `FeedbackMemory` | 完整 `tool → declared_class → drift_rate` 表 |
| session 结束时单次写入 | 半自动 emission_class 分类器 |
| 供后续 session Filter/Plan 读取 | cheap talk 自动降级/升级 |

**写入点**：`session_complete.go` 在设置 `meta["verify_exit_reason"]` 同时调用 `learn.FeedbackMemory.Record(sessionID, reason, emissionClass)`。

### 2.6 PlanKind × EmissionClass 交叉一致性（H9）

| 规则 |  enforcement |
|------|-------------|
| `task_kind=review` 时 Probe 工具 bound 不得为 `OpenEnded` | PerTaskKindFilter 收紧 + cross-consistency 单测 |
| 同 tool 同 query `call_count>3` | ProbeToolChannel `OnResult` 行为重分类 → 临时按 Probe 计价 |
| PlanChannel（PlanKind）与 ToolChannel（EmissionClass）正交 | PR-B 前 rename 消歧（P0-AC-8） |

### 2.7 Shadow Mode 迁移（H11）

Phase B 分两阶段 rollout：

```
Week 1: EnableMupsChannels=true, EnableMupsChannelsEnforce=false
        → log would_reject=true, 不 block tool_call
        → 运维收集 false positive rate，目标 <5%

Week 2+: EnableMupsChannelsEnforce=true
        → hard reject @bound
```

指标：`mups.execute.channel.would_reject_total` / `tool_call_total`。

### 2.8 L0–L3 vs L4–L6 cross-check（H8，P1-AC-2）

Execute 4 ToolChannel 引入的 L4–L6 终止 invariant（L4 Bounded / L5 Quotient / L6 Synthesize）必须与现有 L0–L3 安全 invariant 在 **类型检查粒度** 上 cross-check（≥3 条），避免新增 bound 绕过既有 readonly/permission/audit 约束：

| # | 规则 | enforcement |
|---|------|-------------|
| CC-1 | `Bounded(n)` 不得放宽 `ReadOnly=true` tool 的 readonly 语义 | execute-channels.md D7-EXEC-CH-2 + ProbeToolChannel route 前置 `ReadOnly` check |
| CC-2 | `Quotient(t)` 不得绕过 `CheckPermission` 校验 | execute-channels.md D7-EXEC-CH-2 + ExperimentToolChannel route 前置 `perm.Request` |
| CC-3 | `Synthesize` 不得跳过 `audit()` 调用 | execute-channels.md D7-EXEC-CH-2 + ToolChannelRouter `Finalize()` 内强制 emit audit span |
| CC-4 | 任意 ToolChannel 不得 override `Destructive` tool 的 sandbox 隔离 | execute-channels.md D7-EXEC-CH-2 + ActionToolChannel route 前置 `Destructive → bashAST` |

**冲突检测**：CI grep gate — `mups/execute/toolchannel/*.go` 中禁止出现 `ReadOnly=*false, ReadOnly=*true, ReadOnly = false` 等字面量（强制 ReadOnly 只读）。

### 2.9 禁止 Silent Metadata Default（H12，P0-AC-9/10）

PR-A 必须落地 2 条 hard 约束：

| 约束 | 实现 |
|------|------|
| **P0-AC-9**：read_file / grep / glob 必须显式 `Probe + Bounded(15)` | tool-spec-v3.md 19 工具默认 metadata 表（read_file / grep / glob 显式标注 EC_Probe + IterationBound.Bounded(15)） |
| **P0-AC-10**：缺 `EmissionClass` 的 surface CI fail | tasks.md T14 `TestAllSurfacesHaveEmissionClass` — grep `surface/*.go` 缺 `EmissionClass:` 字面量即 fail；禁止 runtime fallback `Action+OpenEnded` 维持 pooling |

**理由**：silent default 维持"pooling"（all tools equal）行为，LLM 自我循环无法收敛。H12 把 silent default 从"软退化"提升为"硬违反"，与 6 新字段 control plane 承诺一致。

---

## ③ 业务流程

### 3.1 核心用例时序图 — Review 类任务（治本案例）

```
User                D1                    D7                      D2                       LLM
 │                   │                   │                       │                        │
 │ "review d2 kernel"│                   │                       │                        │
 ├───────────────────▶                   │                       │                        │
 │                   │ OnMessage (P2P)   │                       │                        │
 │                   ├───────────────────▶                       │                        │
 │                   │                   │ ProcessMessage         │                        │
 │                   │                   │ IntentClassify(PLAN)   │                        │
 │                   │                   ├─────────────────────────────────────────────────▶│
 │                   │                   │                       │   system:               │
 │                   │                   │                       │   "[Tools spec v3]     │
 │                   │                   │                       │    read_file:Probe     │
 │                   │                   │                       │    Bound(15)"          │
 │                   │                   │ RunSessionTurnLoop    │                        │
 │                   │                   ├─────────────────────────────────────────────────▶│
 │                   │                   │   ItemPipeline.Observe│<──┐                    │
 │                   │                   │   ItemPipeline.Plan   │   │                    │
 │                   │                   │   ItemPipeline.Execute│   │                    │
 │                   │                   │     Router routes     │   │                    │
 │                   │                   │     read_file→ProbeCh │   │                    │
 │                   │                   │     ├─ ToolChannel.OnCall │   │                    │
 │                   │                   │     ├─ iter=1 OK      │   │                    │
 │                   │                   │     └─ emit tool_call─┼───┼────────────────────▶│
 │                   │                   │                       │   │  tool_call(read)    │
 │                   │                   │   <tool_result>       │   │◀────────────────────┤
 │                   │                   │   ProbeCh.OnResult    │   │   50K tokens 截断   │
 │                   │                   │     TruncateMarker附  │   │                    │
 │                   │                   │     "[TRUNCATED...]"  │   │                    │
 │                   │                   │     iter=2 OK         │   │                    │
 │                   │                   │   ItemPipeline.Verify │   │                    │
 │                   │                   │     Verify contract   │   │                    │
 │                   │                   │     (intermediate)    │   │                    │
 │                   │                   │   ItemPipeline.Learn  │   │                    │
 │                   │                   │                       │   │                    │
 │                   │                   │ ... loop 12 次 ...    │   │                    │
 │                   │                   │                       │   │                    │
 │                   │                   │   ProbeCh.OnCall      │   │                    │
 │                   │                   │     iter=12           │   │                    │
 │                   │                   │     remaining=3       │   │                    │
 │                   │                   │     InjectPromptPressure                           │
 │                   │                   │     "⚠️ tool calls remaining: 3"                  │
 │                   │                   │     注入 LLM system prompt                        │
 │                   │                   ├─────────────────────────────────────────────────▶│
 │                   │                   │                       │                        │
 │                   │                   │  LLM 反复 read_file   │                        │
 │                   │                   │  iter=13, 14, 15 OK   │                        │
 │                   │                   │                       │                        │
 │                   │                   │  iter=16 拒绝        │                        │
 │                   │                   │   ProbeCh.OnCall      │                        │
 │                   │                   │     Bounded(15) hit   │                        │
 │                   │                   │     SynthesizeNow-    │                        │
 │                   │                   │     Signal injection  │                        │
 │                   │                   ├─────────────────────────────────────────────────▶│
 │                   │                   │                       │  "基于已读内容，给出    │
 │                   │                   │                       │   review"             │
 │                   │                   │                       │◀──────────────────────┤
 │                   │                   │  LLM emit text        │                        │
 │                   │                   │  "Review: ..."       │                        │
 │                   │                   │  EmitSessionComplete   │                        │
 │                   │                   │  meta["verify_exit_   │                        │
 │                   │                   │   reason"]=null       │                        │
 │                   │                   │  meta["emission_      │                        │
 │                   │                   │   class"]=Probe       │                        │
 │                   │                   │                       │                        │
 │                   │                   ├──────────────────────▶                        │
 │                   │  OnMessage(complete)│                      │                        │
 │                   │   finalizeStructuredSession               │                        │
 │                   │    title:✅ 任务已完成                     │                        │
 │                   │    footer:✅ 任务已完成                    │                        │
 ◀───────────────────┤                   │                       │                        │
```

### 3.2 异常补偿表

| 异常 | 触发 | Fallback |
|------|------|----------|
| ProbeToolChannel bound 击穿 | LLM 调 16+ 次 tool_call | 返回 `ErrProbeChannelBoundExceeded`，强制 LLM synthesize |
| Verify contract fail (deliverable 空) | LLM 没 emit text | `VerdictFail + reason=deliverable_missing`，不进 Learn |
| Verify contract fail (insufficient evidence) | tool_call 数 < MinEvidence | `VerdictPartial + reason=evidence_insufficient` |
| CalibratedConfidence < 0.5 | source_uncertainty 加权低 | `VerdictPartial` 不写 PASS |
| TruncateMarker 失败 (LLM context 空间不够) | 单 tool_result 超 100% context | 截断 + marker + retry hint |
| IntentClassifier 置信度 < 0.6 | task_kind 不可推 | 默认 task_kind=observe，IterationBound=OpenEnded |

### 3.3 分支决策树 — ToolChannel 路由

```
ToolCall(toolName, input)
    │
    ├─ lookup ToolSpec.EmissionClass (mups/execute/toolchannel.Registry)
    │
    ├─ EC=Probe ───── ProbeToolChannel
    │                       │
    │                       ├─ iter < Bound.MaxN ──▶ OnCall OK
    │                       │                         inject PromptPressure (task_kind override):
    │                       │                           review  Bounded(15) → soft@5, hard@2, force@16
    │                       │                           edit    Bounded(10) → soft@3, hard@1, force@11
    │                       │                           test    Bounded(12) → soft@4, hard@1, force@13
    │                       │                           observe OpenEnded    → no pressure
    │                       │
    │                       └─ iter ≥ Bound.MaxN ──▶ SynthesizeNowSignal
    │
    ├─ EC=Action ──── ActionToolChannel
    │                       │
    │                       └─ PostSnapshot vs PreSnapshot
    │                            └─ diff ≠ ∅ ──▶ OnCall OK (state changed)
    │                            └─ diff = ∅ ──▶ OnCall WARN (no state change)
    │
    ├─ EC=Fact ─────── FactToolChannel
    │                       │
    │                       └─ query_history cache hit ──▶ reuse cached result
    │                       └─ cache miss ──▶ OnCall OK
    │
    └─ EC=Experiment ─ ExperimentToolChannel
                            │
                            └─ deadline < ConcludedAt ──▶ OnCall OK
                            └─ deadline ≥ ConcludedAt ──▶ AbortWithReason(experiment_timeout)
```

---

## ④ 领域模型

### 4.1 聚合根

| 聚合根 | 限域 | 职责 |
|--------|------|------|
| **ToolSpecRegistry** | D2 `enforce/tools/surface/` | 19 工具的 metadata 真值表（含 v3 6 字段默认） |
| **ToolChannelRouter** | D7 `mups/execute/toolchannel/` | 4 ToolChannel 路由 + PromptPressure 注入（per EmissionClass） |
| **VerifyContract** | D7 `executionflow/verify/` | Input contract 校验 + verdict 计算 |
| **FilterChain** | D2 `enforce/tools/filter/` | 3 维 Filter 链 (v2：agent + emission_class + task_kind) |

### 4.2 限界上下文（包边界）

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  D2 ContextEngine                                                            │
│   ┌─────────────────────────┐  ┌────────────────────────┐  ┌──────────────┐ │
│   │ enforce/tools/surface/  │  │ enforce/tools/filter/  │  │ compression/ │ │
│   │   builtin_surface.go    │  │   v2/                  │  │   truncate_  │ │
│   │   lsptool_surface.go    │  │     per_emission_class │  │   marker.go  │ │
│   │   ... 8 surface         │  │     per_task_kind      │  │   ...        │ │
│   │   orthogonal_flags.go   │  │   per_agent.go (v2)    │  │              │ │
│   └─────────────────────────┘  └────────────────────────┘  └──────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  D7 Orchestration                                                            │
│   ┌─────────────────────────────────┐  ┌────────────────────────┐         │
│   │ mups/execute/                   │  │ executionflow/verify/  │         │
│   │   channel.go (PlanKind, 现有)   │  │   verify_contract.go   │  NEW     │
│   │   {commit,protocol,scenario,    │  │   contracts.go (现有)  │         │
│   │    exploration}.go (现有)       │  │   rollup_verify.go     │         │
│   │   toolchannel/          NEW ★   │  │                        │         │
│   │     channel.go (ToolChannel)    │  │                        │         │
│   │     fact_channel.go             │  │                        │         │
│   │     action_channel.go           │  │                        │         │
│   │     probe_channel.go     ⚡      │  │                        │         │
│   │     experiment_channel.go       │  │                        │         │
│   └─────────────────────────────────┘  └────────────────────────┘         │
│   ┌─────────────────────────────────┐                                       │
│   │ sessionorchestrator/            │                                       │
│   │   session_complete.go (改)      │                                       │
│   │   verdict_to_exit_reason.go     │                                       │
│   │   hard_evidence_gate.go         │                                       │
│   └─────────────────────────────────┘                                       │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  D5 Observability                                                            │
│   ┌─────────────────────────────────┐                                       │
│   │ observability/instrument/       │                                       │
│   │   logger/ metrics/              │                                       │
│   │   telemetry/ tracer/ (现有)     │                                       │
│   │   ltl/                  NEW ★   │                                       │
│   │     invariants/termination/     │                                       │
│   │       bounded.go   (L4)         │                                       │
│   │       quotient.go  (L5)         │                                       │
│   │       synthesize.go (L6)        │                                       │
│   └─────────────────────────────────┘                                       │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│  D1 Communication                                                            │
│   ┌─────────────────────────┐  ┌────────────────────────┐                   │
│   │ channel/adapters/       │  │ channel/conclusion/    │                   │
│   │   feishu.go             │  │   conclusion.go        │                   │
│   │   feishu_progress.go    │  │                        │                   │
│   └─────────────────────────┘  └────────────────────────┘                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 领域事件 / Span

| Span | 触发 | 维度 |
|------|------|------|
| `D2_S15_A02_ToolSpec_Lookup` | ToolSpec.v3 lookup | tool_name, emission_class |
| `D2_S15_A02_TaskKind_Inference` | IntentClassifier | task_kind, confidence |
| `D7_S9_A50_Channel_Route` | Router pick Channel | channel_name, tool_name |
| `D7_S9_A50_Channel_OnCall` | ToolChannel.OnCall | channel_name, iter, bound_max |
| `D7_S9_A50_Channel_PromptPressure` | 剩 ≤ 3 时注入 | remaining, tool_name |
| `D7_S9_A50_Channel_BoundHit` | iter ≥ Bound.MaxN | iter, bound_max, primary_class |
| `D2_S15_A02_Truncate_Marker` | 截断发生 | max_chars, result_chars |
| `D7_S10_A50_Verify_Contract_Check` | Verify | contract_kind, expected_class |
| `D7_S2_A50_Verdict_Reason_Set` | verdict.Reason | verdict_kind, reason, cc |
| `D1_S17_Adapter_Complete_Render` | feishu render | title, footer, has_reason |

### 4.4 跨域消费

| Source → Sink | 数据 |
|---|---|
| **D2 ToolSpec.v3 → D7 ChannelRouter** | `EmissionClass` + `IterationBound` lookup |
| **D7 Channel → D7 Verify** | `ChannelOutcome{deliverable_text, evidence[], primary_class}` |
| **D7 Verify → D7 SessionComplete** | `meta["verify_exit_reason", "emission_class", "source_uncertainty"]` |
| **D7 SessionComplete → D1 EmitComplete** | meta 透传 |
| **D1 EmitComplete → D1 feishu** | meta["verify_exit_reason"] 读 + render |
| **D7 Channel → D2 TruncateMarker** | `TruncateWithMarker(text, MaxResultSizeChars, TruncateMarkerText)` |

---

## ⑤ 核心链路图

### 5.1 端到端路径（Review 类任务，治本案例）

```
[Feishu P2P message]
       ↓ <1ms
D1 feishu.OnMessage           ← P99 200ms
       ↓ 1跳
D1 RouteInbound → Capture     ← P99 50ms
       ↓ 1跳
D7 ProcessMessage             ← P99 100ms
       ↓ IntentClassifier ~30ms
       ↓ (Promise 1)
D7 RunSessionTurnLoop
       ↓ 5 节点 Pipeline ~50ms
       ↓ (Promise 2) → ItemPipeline.Execute
       ↓ tool_call(read_file, 1)
D7 ChannelRouter              ← P99 5ms (metadata lookup)
       ↓ route to ProbeToolChannel
       ↓ ProbeToolChannel.OnCall(iteration=1)
       ↓ iter < Bound(15) → Accept
D2 queryloop.tool_runner      ← P99 100ms
       ↓ execute read_file
       ↓ 拿到 50K tokens
D2 compression.TruncateWithMarker(text, 8K, "[TRUNCATED...]")
       ↓ 返回 8K + marker
       ↓ tool_result(text="[TRUNCATED...] 8K chars ...")
       ↓ ProbeToolChannel.OnResult
       ↓ iter=2 OK
       ↓ ... 12 轮 ... (每轮 ~250ms)
       ↓ iter=12 remaining=3 → Inject Prompt Pressure
       ↓ LLM 仍 read_file iter=13, 14, 15
       ↓ iter=16 → BoundHit
       ↓ SynthesizeNow signal
       ↓ LLM 收到 system prompt 注入
       ↓ LLM emit text "Review: ..."
       ↓ (Promise 1) ~3s 完成
D7 Verify contract check      ← P99 10ms
       ↓ deliverable 50+ chars, evidence 15+ tool calls
       ↓ VerdictPass + source_uncertainty=0.4
D7 buildSessionCompleteEvent
       ↓ meta["verify_exit_reason"] = ""
       ↓ meta["emission_class"] = "Probe"
       ↓ meta["source_uncertainty"] = "0.4"
D1 conclusion.EmitComplete
       ↓ meta 透传
D1 feishu.OnMessage("complete")
       ↓ finalizeStructuredSession
       ↓ render title "✅ 任务已完成 (ProbeToolChannel, source_uncertainty=0.4)"
       ↓ render footer "✅ 任务已完成"

总 P99: ~5s (主要 LLM token 生成)
```

### 5.2 时序标注 (SLA / P99)

| 跳 | 跳数 | 单跳 P99 | 累积 P99 |
|----|------|----------|----------|
| Feishu → D1 OnMessage | 1 | 200ms | 200ms |
| → D7 ProcessMessage | 1 | 100ms | 300ms |
| → D7 5 节点 Pipeline | 1 | 50ms | 350ms |
| → D7 Execute ChannelRouter | 1 | 5ms | 355ms |
| → D2 queryloop.tool_runner | 1 | 100ms | 455ms |
| → D2 TruncateWithMarker | 1 | 5ms | 460ms |
| → D7 Verify | 1 | 10ms | 470ms |
| → D1 EmitComplete | 1 | 50ms | 520ms |
| → D1 feishu render | 1 | 100ms | **620ms** (元数据增加 < 5ms 开销) |

**单点风险与缓解**：

| 风险点 | 影响 | 缓解 |
|--------|------|------|
| IntentClassifier 误判 | task_kind 错 → iteration_bound 误 | 4 类 task_kind confidence < 0.6 时默认 observe (OpenEnded) |
| ProbeToolChannel bound 过紧 (≤ 5) | 真实 review 任务被截 | 用户可配置 (per-task override) |
| VerifyContract 部分场景漏检 | 探索性 deliverable 仍过关 | DM-005 协同实现 fail-closed |
| TruncateMarker 被 LLM 忽略 | "complete=false" 看不到 | system prompt 默认启用 marker awareness |

---

## ⑥ 接口/API 设计

### 6.1 风格

- **Pure types**：`ToolSpec`, `EmissionClass`, `IterationBound` 等用 struct 不可变 (With* 返回新副本)
- **Builder**：`VerifyContractBuilder` 4 字段链式
- **With\*** 不可变模式：`IterationBound.WithMaxN(n int) IterationBound`
- **Functional options**：`NewProbeToolChannel(opts ...ProbeToolChannelOption) *ProbeToolChannel`

### 6.2 核心契约 — ToolSpec v3

```go
// /Users/fukai/workspace/devrix/internal/shared/contracts/tool_surface_v3.go
type ToolSpec struct {
    Name        string                          // 不变
    Description string                          // 不变
    Parameters  string                          // 不变
    Risk        types.RiskLevel                 // 不变
    ReadOnly        bool                        // 不变
    Destructive     bool                        // 不变
    OpenWorld       bool                        // 不变
    ConcurrencySafe bool                        // 不变
    DeferLoading    bool                        // 不变
    // ---- v3 新增 6 字段 ----
    EmissionClass       EmissionClass       `json:"emission_class"`
    ConvergenceContract ConvergenceContract `json:"convergence_contract"`
    IterationBound      IterationBound      `json:"iteration_bound"`
    SourceUncertainty   SourceUncertainty   `json:"source_uncertainty"`
    MaxResultSizeChars  int                 `json:"max_result_size_chars"`
    TruncateMarkerText  string              `json:"truncate_marker_text"`
}

type EmissionClass int
const (
    EC_Fact EmissionClass = iota    // 0
    EC_Action                       // 1
    EC_Probe                        // 2
    EC_Experiment                   // 3
)

type ConvergenceContract struct {
    Kind         int     `json:"kind"`     // 0=None, 1=StateChange, 2=Evidence, 3=Quotient
    Threshold    float64 `json:"threshold"`
    MinEvidence  int     `json:"min_evidence"`
}

type IterationBound struct {
    Kind     int     `json:"kind"`     // 0=OpenEnded, 1=Bounded, 2=Quotient
    MaxN     int     `json:"max_n"`
    Quotient float64 `json:"quotient"`
}

type SourceUncertainty struct {
    Source int     `json:"source"`  // 0=Deterministic, 1=LLM, 2=User, 3=Memory
    Value  float64 `json:"value"`
}
```

### 6.3 ToolChannel interface

```go
// /Users/fukai/workspace/devrix/internal/layers/orchestration/mups/execute/toolchannel/channel.go
// 注：本 ToolChannel 与 mups/execute/channel.go 中的 Channel (per PlanKind) 是平行不冲突的两套接口
type ToolChannel interface {
    Name() string
    EmissionClass() EmissionClass
    Route(tool ToolSpec) bool
    Accept(ctx context.Context, call *ToolCall) error
    OnResult(ctx context.Context, result *ToolResult) error
    InjectPromptPressure(ctx context.Context, remaining int) error
    Invariants() []ltl.Invariant
    Finalize(ctx context.Context) (*ChannelOutcome, error)
}

type ChannelOutcome struct {
    DeliverableText string
    Evidence         []string  // tool_call IDs
    PrimaryClass     EmissionClass
    IterationsUsed   int
    BoundMax         int
}
```

### 6.4 VerifyContract

```go
// /Users/fukai/workspace/devrix/internal/layers/orchestration/executionflow/verify/verify_contract.go
type VerifyContract struct {
    ExpectedClass       EmissionClass
    DeliverableRequired bool
    DeliverableMinChars int               // task_kind-dependent: review=20, edit=10, test=30, observe=10
    EvidenceRequired    bool
    MinEvidenceCount    int
    MinSourceQuality    float64           // calibrated_confidence 下限，默认 0.5
}

// NewVerifyContract 显式构造器（防 Go 零值陷阱：MinSourceQuality=0 永远过）
func NewVerifyContract(taskKind string, expected EmissionClass) VerifyContract

func Verify(ctx context.Context, c VerifyContract, out *TurnOutput) (*Verdict, error)

// CC 公式（Codex Critical #2+#6 修复）：单类 CC = su，混合类 Σ(su×w)/Σ(w) 归一化
// emission_class_weight: EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20, EC_Experiment=0.10
```

**`finalizeStructuredSession` 改 RenderArgs struct param**（Codex Info #2 修复 — 避免 break PR #373 5-param 签名）：

```go
// /Users/fukai/workspace/devrix/internal/layers/communication/channel/adapters/feishu_progress.go
type RenderArgs struct {
    SessionID         string
    ChatID            string
    Summary           string
    ExitReason        string
    TaskIncomplete    bool
    VerifyExitReason  string  // NEW — verifier 层 reason
    EmissionClass     string  // NEW — Probe/Fact/Action/Experiment
    SourceUncertainty string  // NEW — CC 数值
}

func finalizeStructuredSession(ctx context.Context, args RenderArgs) error
```

**错误码三元组**：

| Error | Type | HTTP-style | TraceID field |
|-------|------|-----------|---------------|
| `ErrToolSpecMissing` | SentinelError | 404 | `tool_name` |
| `ErrProbeChannelBoundExceeded` | SentinelError | 429 | `iteration`, `bound_max` |
| `ErrVerifyDeliverableMissing` | SentinelError | 422 | `expected_class`, `min_chars` |
| `ErrChannelInvariantFailed` | Wrapped | 500 | `channel`, `invariant` |
| `ErrTruncateMarkerFailed` | Wrapped | 500 | `max_chars`, `result_chars` |

### 6.5 幂等

| API | 幂等性 | Key |
|-----|--------|-----|
| `ToolSpecRegistry.Register(spec)` | Idempotent by `Name` | Name |
| `ChannelRouter.Route(tool)` | Pure function | (tool.Name, tool.IterationBound) |
| `Verify(...)` | Pure | (contract, output_hash) |
| `TruncateWithMarker(...)` | Pure | (text, max) |

### 6.6 版本演进

| Version | Phase | 变更 |
|---------|-------|------|
| v3.0 | Phase A 落地 | ToolSpec v3 type 定义 + 19 工具 metadata 重标 |
| v3.1 | Phase B 落地 | + Channel 4 路由 + LTL-Lite L4-L6 |
| v3.2 | Phase C 落地 | + VerifyContract + Reason 透传 + TruncateMarker |
| v3.3 | Phase D 落地 | + Filter v2 + Task Kind 路由 |

---

## 附录 A: File Manifest（4 Phase）

### Phase A

- `internal/shared/contracts/tool_surface.go` **EXTEND** (加 6 字段在末尾 + 4 type 定义)
- `internal/shared/contracts/tool_surface_test.go` EXTEND (struct literal 兼容性单测)
- `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go` (extend 44-142) **含 19 工具默认 metadata**
- `internal/layers/contextengine/enforce/tools/surface/builtin_surface.go` (extend 35-50)
- `internal/layers/contextengine/enforce/tools/surface/lsptool_surface.go` (lsp_* 拆 EC_Fact/Probe)
- `internal/layers/contextengine/enforce/tools/surface/tracker_surface.go`
- `internal/layers/contextengine/enforce/tools/surface/freefork_surface.go`
- `internal/layers/contextengine/enforce/tools/surface/verify_surface.go`
- `internal/layers/contextengine/enforce/tools/surface/askuser_surface.go`
- `internal/layers/contextengine/enforce/tools/surface/backgroundtask_surface.go`
- `internal/layers/contextengine/enforce/tools/surface/tool_search_surface.go`

### Phase B

- `internal/layers/orchestration/mups/execute/toolchannel/channel.go` NEW
- `internal/layers/orchestration/mups/execute/toolchannel/fact_channel.go` NEW
- `internal/layers/orchestration/mups/execute/toolchannel/action_channel.go` NEW
- `internal/layers/orchestration/mups/execute/toolchannel/probe_channel.go` NEW ⚡
- `internal/layers/orchestration/mups/execute/toolchannel/experiment_channel.go` NEW
- `internal/layers/orchestration/mups/execute/toolchannel/probe_channel_test.go` NEW
- `internal/layers/observability/instrument/ltl/invariants/termination/bounded.go` NEW
- `internal/layers/observability/instrument/ltl/invariants/termination/quotient.go` NEW
- `internal/layers/observability/instrument/ltl/invariants/termination/synthesize.go` NEW
- `internal/layers/orchestration/mups/execute/channel.go` EXTEND (PlanKind Channel 内部调 ToolChannelRouter)

### Phase C

- `internal/layers/orchestration/executionflow/verify/verify_contract.go` NEW
- `internal/layers/orchestration/executionflow/verify/verify_contract_test.go` NEW
- `internal/layers/orchestration/sessionorchestrator/session_complete.go` (改 41-44)
- `internal/layers/communication/channel/conclusion/conclusion.go` (改 154)
- `internal/layers/communication/channel/adapters/feishu.go` (改 138-148)
- `internal/layers/communication/channel/adapters/feishu_progress.go` (改 finalizeStructuredSession → RenderArgs struct param)
- `internal/layers/contextengine/compression/truncate_marker.go` NEW
- `internal/layers/contextengine/compression/truncate_marker_test.go` NEW

### Phase D

- `internal/layers/contextengine/enforce/tools/filter/v2/per_emission_class.go` NEW
- `internal/layers/contextengine/enforce/tools/filter/v2/per_task_kind.go` NEW
- `internal/layers/contextengine/enforce/tools/filter/per_agent.go` (改 27-125 → v2)
- `internal/layers/contextengine/prepare/orchestrator.go` (改 111)

---

## 附录 B: Rollback Plan

| Phase | 单 PR 回滚 | 链路影响 |
|-------|-----------|----------|
| A | `git revert PR-A` | 现有 9 字段不变，新增 6 字段有 default，0 行为差 |
| B | `git revert PR-B` | Channel router fall through 到 single-channel mode |
| C | `git revert PR-C` | Verify 回退到 accept-any-text mode |
| D | `git revert PR-D` | Filter v2 fall back 到 Filter v1 |

**链接回滚测试**：每 Phase PR 含独立 feature flag (registry.RuntimeConfig.EnableMupsChannels)，可运行时关。

---

## 附录 C: S3 Checklist

- [x] ① 业务目标 + 技术目标 + 约束
- [x] ② 设计原则 + 命名 + 代码风格
- [x] ③ 时序图 + 异常补偿 + 决策树
- [x] ④ 聚合根 + 限界上下文 + 领域事件 + 跨域消费
- [x] ⑤ 端到端路径 + 时序 SLA + 单点风险
- [x] ⑥ 接口/契约 + 错误码 + 幂等 + 版本演进
- [x] File Manifest
- [x] Rollback Plan
- [x] 回归风险评估

## 附录 D: S3-Gate Review 结论（Codex Critical #1 必需项）

**Review Status:** ⏳ **PENDING — S3-Gate Review 待执行**

**Review Type:** Grill Review（4-PR 跨域架构级变更 — review-design.md §3.1）

**关键 Grill Topic:**

| Topic | 关注点 | 状态 |
|-------|--------|------|
| **Channel 命名解耦** | 现有 mups/execute/ 4 PlanKind Channel + 新 ToolChannel 命名是否清晰，调用顺序（PlanKind→EmissionClass）是否合理 | 已修复 — naming 解耦为 Channel/ToolChannel 2 套接口 |
| **observability-domain ownership** | LTL-Lite L4-L6 新增 invariants 在 observability/instrument/ltl/ 是否合理（observability 是公共域，跨 D2/D7 使用 OK） | 待审 |
| **PR-B 风险** | mups/execute/toolchannel/ 新子包是否影响现有 4 PlanKind Channel 接口稳定性（DM-20260625-001-PRC2 已归档） | 待审 |
| **PR-A 治本范围** | 19 工具默认 metadata 迁移在 PR-A 内（治本叙事不能留 Phase E 尾巴） | 已修复 — OOS-6 移除，PR-A INCLUDE 19 工具 metadata |
| **CC 数学** | Σ(su×w)/Σ(w) 归一化 + EC_Fact>EC_Action 权重排序 8 子用例覆盖 | 已修复 — 单类 CC = su 直觉一致 |
| **PromptPressure 时机** | 软警告@5/硬警告@2/强制@0 + task_kind override（review/edit/test/observe） | 已修复 |
| **feishu render 签名** | finalizeStructuredSession 改 RenderArgs struct param 避免 break PR #373 5-param 签名 | 已修复 |

**Codex Critical Findings (2026-07-01 review):**

| # | Status | Description |
|---|--------|-------------|
| C1 | ✅ Fixed | status: s1_requirement → s3_design + 新建 demand.md |
| C2 | ✅ Fixed | CC 公式 Σ(su×w)/Σ(w) 归一化（单类 CC = su） |
| C3 | ✅ Fixed | Channel→ToolChannel 重命名 + 与 mups/execute/ 现有 4 PlanKind Channel 命名解耦 |
| C4 | ✅ Fixed | PR-A INCLUDE 19 工具默认 metadata 迁移（OOS-6 移除，无 Phase E 尾巴） |
| C5 | ✅ Fixed | 文件路径全改 (sessionorchestrator/executionflow/ → mups/execute/toolchannel/ + executionflow/verify/) |
| C6 | ✅ Fixed | EC_Fact=0.50 > EC_Action=0.35 > EC_Probe=0.20 > EC_Experiment=0.10 |
| C7 | ✅ Fixed | PromptPressure 软警告@剩5 / 硬警告@剩2 / 强制@0 + task_kind override |
| C8 | ✅ Fixed | DeliverableMinChars 按 task_kind 区分（review=20, edit=10, test=30, observe=10） |
| C9 | ✅ Fixed | "0 test change" claim 加 struct literal grep gate + 6 新字段放末尾 |

**Review Decision:** ⏳ 待用户评审；当前 Critical 0 项

---

## 更新历史

- 2026-07-01：v1 创建（6 段式架构设计，4 Phase + 23 T + 7 原则 + 6 特征）
- 2026-07-01：v1.1 Codex Critical 9 项全部修复 + Appendix D S3-Gate Review 占位
  - 命名解耦（Channel/ToolChannel）+ 路径全改 + CC 数学修复 + PR-A 范围 INCLUDE + PromptPressure 软硬警告 + MinChars by task_kind + JSON tag 一致性 + struct literal grep gate + finalizeStructuredSession RenderArgs struct param + lsp_* EC_Fact/Probe 拆分 + dsaft_scenarios 扩 9 个 + T 点全 DSAFT 化 + 新建 demand.md
- 2026-07-01：v1.2 博弈论双 review 共识并入（§2.4 Equilibrium、§2.5 Learn 边界、§2.6 cross-consistency、§2.7 shadow mode；Filter 三维）
- 2026-07-01：v1.3.1 S3-Gate Re-Review patch — T-ID 冲突修复（tasks.md Phase D D2-S15-A02-T06 → T15，跨 7 处同步）+ §2.8 L0–L3 vs L4–L6 cross-check（H8/P1-AC-2，4 条 CC）+ §2.9 禁止 Silent Metadata Default（H12/P0-AC-9/10）；codex 复审 PASS（critical_count: 0）
