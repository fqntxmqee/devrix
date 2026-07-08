# Design: D7 MUPS Frame Delta Phase 1+2 spans 端到端触发

**Change ID:** `devrix-d7-frame-delta-phase1-2-span-trigger`
**Demand ID:** DM-20260706-001
**Status:** S7_Archived (2026-07-08)
**Parent Proposal:** `proposal.md`
**Parent Demand:** `demand.md`
**Template:** `docs/methodology/detail-design-framework.md`（六段式）
**Created:** 2026-07-06
**Parent Change:** `devrix-d7-mups-frame-delta-closure` (DM-20260705-010, S7_Archived 2026-07-05)

---

## ① 架构目标

### 1.1 业务目标

关闭 `devrix-d7-mups-frame-delta-closure` (DM-20260705-010) Phase 4 §8.3 follow-up gap:在 e2e 测试场景下,Phase 1 (`d7.s9.execute.plan_frame_delta.inject`) + Phase 2 (`d7.s5.observe.prior_delta.span`) span 在 `TestIntegration_D7FrameDelta_E2E_SpansAndMonotonicity` 的 memory exporter 中实际触发计数 ≥ 期望值。

| 痛点（来自 Phase 4 §8.3 实证） | 本 Change 对应 AC | S3-Rewrite 拆分后归属 |
|-------------------------------|-------------------|------------------------|
| **Phase 1 span 不触发**:synthetic LLM stub 不触发 `StrategicPlanProposal.FrameDelta` 非零,导致 `InjectPlanFrameDelta` 早返回不发射 span | AC1 | **本 change 处理 (e2e scenario 扩展 + callback)** |
| **Phase 2 span 不触发**:Round 1 `prevExecCtx == nil` 返回零值;Round 2-5 因 Phase 1 未触发,prior 数据累积空,链路退化 | AC2 | **本 change 处理 (e2e scenario 扩展 + seed)** |
| **Phase 2 production wiring 缺失**:`observation_proposer.go:257` 硬编码 `nil` | (S3-Rewrite 新增) | **sibling DM-20260706-004 处理 (production-side 修复)** |
| **spec/code 一致性失衡**:Phase 3 (deterministic 0 LLM) 有 e2e 覆盖,Phase 1+2 只有 unit 覆盖,testutil_only 修复需 testutil callback 注入 | AC4 | **本 change 处理** |
| **测试金字塔缺一块**:frame-delta 协议 e2e tier 缺 Phase 1+2,后续 v1.1 TraceID / v2.0 跨域 FrameDelta 抽象上提前置条件不齐 | AC8 (新) | **本 change 处理** |

### 1.2 技术目标(S3-Rewrite 量化指标更新)

| 指标 | 实测 baseline (post #443 + #444) | 目标 (S3-Rewrite) | 触发 AC |
|------|--------------------------------|-------------------|---------|
| **Phase 1 span 触发数** | 2 (e2e scenario 仅 2 round cycles) | ≥ 5 (e2e scenario 扩展到 5 round cycles) | AC1 |
| **Phase 2 span 触发数** | 2 (e2e);0 (production) | ≥ 5 (e2e);production 由 sibling DM-20260706-004 修复 | AC2 |
| **Phase 3 span 触发数** | 2 (e2e scenario 仅 2 round cycles) | ≥ 5 (e2e scenario 同步扩展) | AC3 |
| **stub vs production 差异文档化** | design.md §5 + testutil 注释明确 "testutil only, NOT production" | AC4 |
| **sibling DM-20260706-004 兼容性** | testutil seed helper 在 production wiring 后仍 work | AC5 (S3-Rewrite 新增) |
| **71 frame 测试 + 16 D7-FD unit 测试** | 100% PASS,0 行为变化 | AC6 |
| **跨链 prompt size 单调性** | last ≤ first*3 (e2e scenario 扩展后仍满足) | AC7 |
| **L5-MUPS-FD-6 T-IDs 登记** | `openspec/specs/d7-orchestration/t-registry.md` D7-FD 段 | AC8 (新) |
| **testutil_only 代码增量** | ≤ 100 行 (含 3 sub-test) | scope |
| **production code 增量** | 0 行 (testutil_only scope 守住) | scope |

### 1.3 约束条件(S3-Rewrite)

- **testutil_only scope** — `internal/layers/orchestration/sessionorchestrator/{convergence_metric,observe_frame_delta,execute_plan_frame_inject}.go` 0 修改;`internal/layers/orchestration/hardening/emitter.go` 0 修改;**`observation_proposer.go` + `item_observe.go` 0 修改 (Phase 2 production wiring 由 sibling DM-20260706-004 处理)**
- **append-only 注入原则不变** — production FrameDelta 5 字段 `PriorArtifactSummary + KnownGaps + ExecutionMode + ChildSpecs + DeliverableContract` 0 修改
- **0 LLM 承诺不变** — `SequenceLLMStub.FrameDeltaInject` callback 不调真 LLM,只 hook 输出 JSON 注入
- **M1 ObservationFrame 9 字段 / M2 StrategicPlanFrame 16 字段契约** 0 修改 (DM-20260705-003)
- **DM-20260705-008 Strategy 抽象** — 不冲突 (testutil_only scope 不进决策表)
- **DM-20260704-006 ResolutionContract** — 不冲突 (testutil seed 不进 ResolutionClaim)
- **三层 fail-safe / Pessimistic Commit L3** — 不破坏 (testutil seed 在 setup/teardown 显式 reset)
- **sibling DM-20260706-004 协调** — 本 change AC5 显式验证 testutil `SeedPriorExecContext` 与 production wiring 兼容

---

## ② 架构原则

### 2.1 设计原则（5 条）

| # | 原则 | 落地方式 | 触发 AC |
|---|------|---------|---------|
| P1 | **testutil_only 严格隔离** | 所有修改限定在 `tests/testutil/` + `tests/integration/d7/` | scope |
| P2 | **callback 注入 ≠ production 行为** | `SequenceLLMStub.FrameDeltaInject` 是 testutil-only hook,生产代码不暴露 | AC4 |
| P3 | **stub 状态显式 reset** | setup/teardown 显式 `obsConfig.MemoryExporter.Reset()` + `prevExecCtx.Store(nil)` | AC5 |
| P4 | **prior 累积纯函数确定性** | `SeedPriorExecContext` 调用 deterministic factory,prior round state 可重现 | AC5 |
| P5 | **last ≤ first*3 单调性独立验证** | 独立 sub-test `TestIntegration_D7FrameDelta_MonotonicWithSeed`,不动原 `TestIntegration_D7FrameDelta_ConvergenceMonotonic` | AC6 |

### 2.2 命名规范

| 元素 | 规范 | 示例 |
|------|------|------|
| **新增 testutil 类型** | `*LLMStub` 后缀(IAdapter 实现) | `FrameDeltaInjectLLMStub`(可选) |
| **callback 字段名** | `FrameDeltaInject func(idx int) FrameDelta` | `s.FrameDeltaInject(turn)` |
| **helper 函数名** | `Seed*` 前缀(显式 seed 状态) | `SeedPriorExecContext(stack, workItemID, priorRound)` |
| **sub-test 函数名** | `TestIntegration_D7FrameDelta_<描述>` | `TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` |
| **L5 T-ID** | `L5-MUPS-FD-6` (后续 -7/-8 顺延) | `D7-S9-A112-L5-MUPS-FD-6-T01` (e2e sub-test 1) |
| **Span Op 名称** | 沿用 `internal/layers/observability/instrument/telemetry/names.go` 已注册 op | `OpD7_S9_Execute_PlanFrameDelta_Inject` |

### 2.3 代码风格

- 函数 < 50 行 (`SeedPriorExecContext` 预计 ~25 行,`FrameDeltaInject` callback 预计 ~15 行)
- 文件 < 800 行 (`tests/testutil/d7_llm_stub.go` 当前 105 行,新增 +30 行)
- 异常不过模块边界 (testutil 仅暴露 1 个 helper 函数 + 1 个 callback 字段,不抛错到 production)
- 不可变性:`FrameDelta` 5 字段纯值对象,callback 返回新对象(`With*` 模式,不变更 input)
- 0 panic / 0 业务 log:testutil silent 行为,不污染 stdout

---

## ③ 业务流程

### 3.1 核心用例 — Phase 1+2 spans 真实触发链路

```
TestIntegration_D7FrameDelta_Phase1And2SpanTrigger(t)

├── setup
│   ├── obsCfg := memoryExporterObsConfig()    // Exporter=memory
│   ├── stack := testutil.NewD7TestStack(t, D7StackOptions{
│   │     LLMStub:  seq,                       // SequenceLLMStub w/ FrameDeltaInject callback
│   │     ObsConfig: obsCfg,
│   │   })
│   └── testutil.SeedPriorExecContext(stack, "wi-1", priorRound)  // Round 1 prior seed
│
├── execute
│   └── routeAndWait(t, stack, sessionID, "review d7 plan directory")
│       │
│       ├── Round 1 Observe   → llm (stub idx=0)  → BuildObservePriorDelta(seeded prior) → emit Phase 2 span ✓
│       ├── Round 1 Plan      → llm (stub idx=1, FrameDeltaInject(1) returns non-zero) → emit Phase 1 span (downstream)
│       ├── Round 1 Execute   → llm (stub idx=2) → InjectPlanFrameDelta(non-zero) → emit Phase 1 span ✓
│       ├── Round 1 Emit      → ComputeConvergenceMetric → emit Phase 3 span ✓
│       ├── Round 2 Observe   → BuildObservePriorDelta(prevExecCtx=non-nil) → emit Phase 2 span ✓
│       ├── Round 2 Plan      → emit Phase 1 span ✓
│       ├── Round 2 Execute×2 → emit Phase 1+3 spans ✓
│       └── Round 2 Emit      → emit Phase 3 span ✓
│
└── verify
    ├── memExporter.Spans() inspection
    │   ├── OpD7_S9_Execute_PlanFrameDelta_Inject: count >= 5  → AC1 PASS
    │   ├── OpD7_S5_Observe_PriorDelta_Inject: count >= 4        → AC2 PASS
    │   └── OpD7_S9_Execute_ConvergenceMetric_Emit: count >= 5   → AC3 PASS
    └── prompt size monotonicity (last ≤ first*3)                → AC6 PASS
```

### 3.2 异常补偿 — 4 类 Fallback 路径

| 异常场景 | 触发条件 | Fallback 路径 | 影响 |
|---------|---------|--------------|------|
| `FrameDeltaInject` callback 返回零值 | 测试场景配置错误 | `InjectPlanFrameDelta` 早返回 baseline,e2e Phase 1 span count = 0 | 测试 fail 显式报错,告知 caller 检查 callback |
| `SeedPriorExecContext` 未调用 | Round 1 prevExecCtx=nil | Round 1 `BuildObservePriorDelta` 返回零值,e2e Phase 2 Round 1 span count = 0 | 仅 Round 1 span count = 0,Round 2-5 仍触发,AC2 阈值 ≥ 4 仍可满足 |
| testutil 状态污染 | 上一 sub-test 残留 prior | sub-test 间通过 `prevExecCtx.Store(nil)` reset | setup/teardown 强制 reset |
| MemoryExporter.Reset() 未调用 | 跨 sub-test span 累积 | `obsCfg.MemoryExporter.Reset()` in t.Cleanup | sub-test 隔离,每 sub-test 独立计数 |

### 3.3 分支处理决策树

```
e2e Phase 1+2 span 测试
├─ callback `FrameDeltaInject` 配置?
│  ├─ YES → InjectPlanFrameDelta → emit span ✓ → AC1 PASS
│  └─ NO  → 早返回 baseline → AC1 FAIL (显式报错)
├─ `SeedPriorExecContext` 配置?
│  ├─ YES → BuildObservePriorDelta(seeded prior) → emit span ✓ → AC2 PASS
│  └─ NO  → Round 1 prevExecCtx=nil → Round 1 span=0 → Round 2-5 span 累积 ≥ 4 → AC2 PASS (阈值 ≥ 4)
└─ both callback + seed 配置?
   ├─ YES → 2 链路全覆盖, AC1+AC2+AC3+AC4 全 PASS
   └─ NO  → 至少 AC3 Phase 3 span 仍 PASS (ConvergenceMetric 0 LLM 不依赖外部状态)
```

---

## ④ 领域模型

### 4.1 聚合根（本 Change 无新增）

本 Change **无新聚合根**,仅修改 testutil 限界上下文。父 change (DM-20260705-010) 已定义的核心聚合根保持不变:

- `FrameDelta` (5 字段纯值对象) — DM-20260705-010 §4.1
- `ConvergenceMetric` (3 字段纯值对象) — DM-20260705-010 §4.1
- `WorkItemExecContext` (atomic.Pointer,prior round state 承载) — DM-20260705-010 §4.1 + DM-20260629-007/008

### 4.2 限界上下文 — D7 Test Framework

```
tests/testutil/                                  # D7 Test Framework 限界上下文
├── d7_llm_stub.go                               # SequenceLLMStub (本 change MODIFIED +FrameDeltaInject)
├── d7_stack.go                                  # NewD7TestStack (本 change MODIFIED +SeedPriorExecContext)
├── d7_frame_delta_helpers.go                    # NEW: SeedPriorExecContext + FrameDeltaInjectLLMStub wrapper
└── ...
tests/integration/d7/
├── d7_frame_delta_e2e_test.go                   # 本 change MODIFIED (+3 sub-test)
└── ...
```

**白名单 (testutil_only):**
- `tests/testutil/d7_*` + `tests/integration/d7/d7_frame_delta_e2e_test.go`
- 任何超出此白名单的修改需 S3-Gate review 重审批

### 4.3 领域事件 — Span emit 不变

本 Change 不修改 span emit 代码,仅通过 callback 让 span 实际触发。沿用父 change 已注册的 3 span op:

| Span Op | 触发函数 | Attribute |
|---------|---------|-----------|
| `d7.s9.execute.plan_frame_delta.inject` | `InjectPlanFrameDelta` (production) | `plan_frame_delta_schema_hash` + `plan_frame_delta_injection_chars` + `injection_status` |
| `d7.s5.observe.prior_delta.span` | `BuildObservePriorDelta` (production) | `prior_artifact_summary` + `known_gaps` + `span_tag_complete` |
| `d7.s9.execute.convergence_metric.emit` | `ComputeConvergenceMetric` (production) | `convergence.uncertainty_reduction_rate` + `convergence.observed_gaps_closed_count` + `convergence.frame_delta_consumed` |

### 4.4 跨域消费模型 — D2 contextengine + D5 observability

| 跨域消费点 | 现状 | 本 Change 影响 |
|-----------|------|---------------|
| **D2 contextengine** (`internal/layers/contextengine/`) | 已有 `SequenceLLMStub` 经 `IAdapter` 注册到 `adapter.Registry` | 0 修改 |
| **D5 observability** (`internal/layers/observability/`) | `MemoryExporter` 在 `tests/testutil/d7_stack.go` ObsConfig 字段已 wired | 0 修改,本 change 仅使用 |
| **D7 orchestration** (`internal/layers/orchestration/`) | `sessionorchestrator/{execute_plan_frame_inject,observe_frame_delta,convergence_metric}.go` production code | 0 修改 |

---

## ⑤ 核心链路图

### 5.1 端到端 e2e 路径 (Phase 1+2 spans 真实触发)

```
[Test Setup]
    │
    ├─→ SequenceLLMStub 配置
    │   └─→ FrameDeltaInject(turn int) FrameDelta callback
    │       └─→ 返回: turn=0/2/4 → zero value (Observe/Execute 不需 plan delta)
    │              turn=1/3   → non-zero {ExecutionMode="protocol", DeliverableContract="summary", ChildSpecs=[]}
    │
    └─→ SeedPriorExecContext(stack, "wi-1", priorRound)
        └─→ WorkItemExecContext atomic.Pointer.Store(&WorkItemExecContext{ConvergenceMetric: &priorRound})
            └─→ Round 1 BuildObservePriorDelta(prevExecCtx=non-nil) 触发 Phase 2 span

[Test Execute: routeAndWait 5 turns]
    │
    ├─ Turn 1: Observe LLM
    │   ├─ input: directive + prior_artifact_summary (from seed) + known_gaps (from seed)
    │   └─ output: obs_fact (zero uncertainty)
    │       └─ BuildObservePriorDelta(seeded prior) emit Phase 2 span ✓ [AC2]
    │
    ├─ Turn 2: Plan LLM
    │   ├─ input: directive + plan frame
    │   └─ output: strategic_plan (execution_mode=protocol, deliverable_contract=summary)
    │       └─ FrameDeltaInject(1) 返回 non-zero
    │           └─ downstream Execute 端 InjectPlanFrameDelta(non-zero) emit Phase 1 span ✓ [AC1]
    │
    ├─ Turn 3: Execute sub-turn 1
    │   ├─ input: system_prompt (含 plan_frame_delta XML 注入) + tool_history
    │   └─ output: tool_call (read_file)
    │
    ├─ Turn 4: Execute sub-turn 2
    │   └─ output: tool_call (read_file) + final_text
    │
    └─ Turn 5: ConvergenceMetric emit
        └─ ComputeConvergenceMetric(priorRound, subTurns) → emit Phase 3 span ✓ [AC3]

[Test Verify]
    │
    ├─→ memExporter.Spans() inspection
    │   ├─ OpD7_S9_Execute_PlanFrameDelta_Inject count ≥ 5    → AC1 PASS
    │   ├─ OpD7_S5_Observe_PriorDelta_Inject count ≥ 4        → AC2 PASS
    │   └─ OpD7_S9_Execute_ConvergenceMetric_Emit count ≥ 5    → AC3 PASS
    │
    └─→ prompt size monotonicity: last ≤ first*3              → AC6 PASS
```

### 5.2 stub vs running system 差异分析(S3-Rewrite)

| 维度 | stub (testutil SequenceLLMStub + FrameDeltaInject) | running system (production) |
|------|---------------------------------------------------|----------------------------|
| **LLM 输出 schema** | callback 返回 `FrameDelta` 5 字段纯值对象 | Plan LLM 输出 JSON,经 Plan OutputProcessor 解析填充 `plan.FrameDelta` |
| **FrameDelta 注入路径 (Phase 1)** | testutil 直接在 stub 内存中包装 JSON response | production 走 `workitem_executor.go` binder (#443 已 wired) → `InjectPlanFrameDelta(ctx, plan.FrameDelta, baseline)` |
| **prior data 累积 (Phase 2)** | `SeedPriorExecContext` 显式 seed `Item.LastRound.ArtifactSummary` | production 走 `WorkItemExecContext` atomic.Pointer 自然累积 (Round N+1 读 Round N);**`observation_proposer.go:257` hardcoded nil 由 sibling DM-20260706-004 修复** |
| **span 触发条件** | `plan.FrameDelta.IsZero() == false` 由 callback 保证 | production 由 Plan LLM 输出决定 (与 stub 一致);Phase 1 production 已 wired via #443 |
| **0 LLM 承诺** | callback 不调真 LLM | production `ComputeConvergenceMetric` 0 LLM |
| **0 production code 修改** | testutil_only scope | production 已合规 (#443 Phase 1 + Phase 2 emit end() #444);**Phase 2 wiring 由 sibling DM-20260706-004 处理** |

**stub → running system 适配路径:**

1. **Phase 1**: stub callback 返回 `FrameDelta{ExecutionMode: "protocol", ...}` 对应 production Plan LLM 输出 `{"execution_mode":"protocol","deliverable_contract":"summary"}` 的语义。running system 通过 Plan OutputProcessor 解析填充 `plan.FrameDelta`,经 `workitem_executor.go` binder (#443) 触发 `InjectPlanFrameDelta`,与 stub callback 语义一致。
2. **Phase 2**: stub `SeedPriorExecContext(workItemID, priorRound)` 显式 seed `Item.LastRound.ArtifactSummary` 对应 production Round 1 的 `prevExecCtx = nil`(真实 running system Round 1 不会有 prior,因为还没跑过)。production Round 2-5 走 `WorkItemExecContext` atomic.Pointer 自然累积,但需 sibling DM-20260706-004 修复 `observation_proposer.go:257` 硬编码 nil 后才能触达 `BuildObservePriorDelta`。
3. **Phase 3**: stub 0 LLM 与 production 0 LLM 一致。

**AC4 文档化要求 (S3-Rewrite):** 本 Change S5 验收时,acceptance-report §5 必须含此 stub vs running system 差异分析段,**并显式 cross-reference sibling DM-20260706-004 的 production wiring 修复 (AC5 联动)**。

**S3-Rewrite 重要修正:** 原 design.md §5.2 "production 已合规" 表述已过时。Phase 1 production 由 #443 + #444 闭环,Phase 2 production 仍需 sibling DM-20260706-004 修复 (硬编码 nil)。本 change 不混入 Phase 2 production wiring (testutil_only scope 守住)。

### 5.3 单点风险与缓解

| 单点风险 | 影响 | 缓解 | 触发 AC |
|---------|------|------|---------|
| `SequenceLLMStub.FrameDeltaInject` callback 命名冲突 | 中 | 字段命名加 docstring 明确 testutil-only | AC4 |
| `SeedPriorExecContext` 修改 `WorkItemExecContext` atomic.Pointer 影响其他 D7 测试 | 中 | setup/teardown 显式 reset + 字段 zero value 默认不影响其他测试 | AC5 |
| MemoryExporter span 累积跨 sub-test | 低 | `t.Cleanup(obsCfg.MemoryExporter.Reset)` 在每个 sub-test | AC5 |
| Phase 1+2 真实触发后 Phase 3 convergence 因 prior 数据变化 | 中 | 独立 sub-test `TestIntegration_D7FrameDelta_MonotonicWithSeed` 验证 last ≤ first*3 | AC6 |
| testutil callback 增加字段,未来 FrameDelta 5 字段演进时需同步 | 低 | callback 签名 `(idx int) FrameDelta`,FrameDelta 是 pure 值对象,production 字段变化自动适配 | AC7 |

---

## ⑥ 接口 / API 设计

### 6.1 风格 — Pure types + 不可变值对象 + testutil callback (S3-Rewrite 字段对齐)

**testutil helper 风格:** factory function + With* 模式 + callback 字段

```go
// tests/testutil/d7_llm_stub.go (MODIFIED)
type SequenceLLMStub struct {
    Responses        [][]llmgateway.Chunk
    CallCount        atomic.Int64

    // FrameDeltaInject (DM-20260706-001, AC1) is an OPTIONAL callback
    // that returns a non-zero FrameDelta for a given call index. When
    // non-nil, the returned value is appended to the stub's response JSON
    // so that downstream InjectPlanFrameDelta() triggers the
    // d7.s9.execute.plan_frame_delta.inject span. **TESTUTIL ONLY** —
    // production code never invokes this callback. Default nil = existing
    // behavior (zero FrameDelta, span count 0).
    //
    // Production-equivalent: Plan LLM outputs
    // {"execution_mode":"...","deliverable_contract":"...","child_specs":[...]}
    // which the Plan OutputProcessor parses into plan.FrameDelta. The
    // callback short-circuits this parse for e2e testing.
    FrameDeltaInject func(idx int) orchestration.FrameDelta
}

// tests/testutil/d7_frame_delta_helpers.go (NEW)
// SeedPriorExecContext (DM-20260706-001, AC2) seeds the WorkItemExecContext
// with a prior round state so that Round 1's BuildObservePriorDelta
// returns non-zero and emits d7.s5.observe.prior_delta.span. **TESTUTIL ONLY**.
//
// S3-Rewrite 字段对齐:seed 目标 = `prevExecCtx.Item.LastRound.ArtifactSummary`,
// 非 `prevExecCtx.ConvergenceMetric`。理由:`observe_frame_delta.go:46-52` 的
// `BuildObservePriorDelta` 非零条件依赖 `Item.LastRound.ArtifactSummary` 非空,
// `ConvergenceMetric` 是中间累积态 (Round N 末轮才产生),直接 seed 不会被 BuildObservePriorDelta
// 当作非零条件命中。
//
// production-equivalent: production Round 1 prevExecCtx=nil by design (no
// prior round yet); Round 2-5 prevExecCtx accumulates naturally via
// WorkItemExecContext atomic.Pointer in ItemPipelineRunner.
// Sibling DM-20260706-004 修复 observation_proposer.go:257 hardcoded nil
// 后,production 链路打通,本 helper 仍兼容 (testutil mock state 与 production wiring 独立)。
//
// Callers MUST reset via t.Cleanup to avoid leaking state to other tests.
func SeedPriorExecContext(t *testing.T, stack *D7TestStack, workItemID string, priorRound orchestration.ConvergenceMetric) {
    t.Helper()
    // S3-Rewrite 伪代码 (与 codex review P0-3 一致):
    // 1. Locate WorkItem via test stack: stack.TaskManager.FindByID(workItemID)
    // 2. Build WorkItemExecContext{Item: wi, Item.LastRound.ArtifactSummary: priorRound.LastArtifactSummary, ConvergenceMetric: &priorRound}
    // 3. atomic.Pointer.Store(&execCtx)
    // 4. t.Cleanup(func() { atomic.Pointer.Store(nil) })
}
```

**S3-Rewrite 字段对齐 rationale:**

- `BuildObservePriorDelta` (observe_frame_delta.go:46-52) 非零条件:`prevExecCtx.Item.LastRound.ArtifactSummary` 非空
- `ConvergenceMetric` 是 Round N 末轮 EmitConvergence 累积态,Round 1 无 prior 时 `prevExecCtx == nil`
- 原 design.md §6.1 "seed ConvergenceMetric" 是 codex P0-3 指出的字段错位,修正为 "seed `Item.LastRound.ArtifactSummary`"
- WorkItem locate: 通过 `stack.TaskManager.FindByID(workItemID)` 在 test stack 内部查 (testutil 已持有 `*workmodel.TaskManager`)

### 6.2 契约 (Span + Trace + 错误码)

| 元素 | 契约 | 验证 |
|------|------|------|
| **Span Op 触发** | 沿用父 change 已注册的 3 span op,attribute 6 个全可见 | memory exporter inspection |
| **callback 返回类型** | `func(idx int) orchestration.FrameDelta` (5 字段纯值对象) | static type check |
| **Seed 函数输入 (S3-Rewrite)** | `(t *testing.T, stack *D7TestStack, workItemID string, priorRound orchestration.ConvergenceMetric)` (priorRound 内部展开为 `Item.LastRound.ArtifactSummary`) | unit test |
| **错误码** | testutil silent 行为,0 panic / 0 business log;错误由 test framework 报 (t.Fatalf) | test framework |
| **TraceID 透传** | 不涉及 (FrameDelta v1.0 0 TraceID,父 change §6.4 v1.1 引入) | scope |
| **sibling DM-20260706-004 兼容** | testutil `SeedPriorExecContext` 在 production wiring 后仍 work (testutil mock state 与 production wiring 独立) | sibling S5 联动验证 |

### 6.3 幂等保障表

| 操作 | 幂等保障 | 触发条件 |
|------|---------|---------|
| `SequenceLLMStub.Stream()` | `CallCount atomic.Int64` 累加,`pickResponses(idx)` 幂等 | 同一 idx 多次调用返回同一 response |
| `SeedPriorExecContext()` | `atomic.Pointer.Store` 覆盖写,先 store 后 cleanup | 同 workItemID 多次 seed,后者覆盖 |
| `MemoryExporter.Reset()` | `t.Cleanup` 在每个 sub-test 末尾调用 | sub-test 隔离 |

### 6.4 版本演进路径

```
v1.0 (本 Change, DM-20260706-001)
  - SequenceLLMStub.FrameDeltaInject callback (testutil only)
  - SeedPriorExecContext helper (testutil only)
  - 3 sub-test 覆盖 AC1+AC2+AC3+AC4+AC6

v1.1 (后续 Phase 5 follow-up)
  - 真实飞书 session running system Jaeger trace 重放验证
  - out of scope for v1.0 (running system 验证需 user action)
```

---

## 附录

### 附录 A：File Manifest

| 操作 | 路径 | 描述 | LOC |
|------|------|------|-----|
| MODIFIED | `tests/testutil/d7_llm_stub.go` | SequenceLLMStub +FrameDeltaInject 字段 + docstring | +30 行 |
| MODIFIED | `tests/testutil/d7_stack.go` | NewD7TestStack 内部追加 WorkItemExecContext 注册 (供 SeedPriorExecContext 使用) | +15 行 |
| NEW | `tests/testutil/d7_frame_delta_helpers.go` | SeedPriorExecContext + FrameDeltaInjectLLMStub wrapper | ~50 行 |
| MODIFIED | `tests/integration/d7/d7_frame_delta_e2e_test.go` | +3 sub-test (Phase1And2SpanTrigger + MonotonicWithSeed + SeedPriorEffect) | ~150 行 |
| NEW | `openspec/specs/d7-orchestration/t-registry.md` (D7-FD 段) | L5-MUPS-FD-6 T-IDs 登记 | +10 行 |
| NEW | `openspec/specs/d7-orchestration/CHANGELOG.md` (顶部) | IMPLEMENTED 条目 | +5 行 |
| NEW | `openspec/specs/d7-orchestration/mups-frame-delta-spec.md` §3.4 | "Phase 1+2 span e2e 触发条件" 新章节 | +15 行 |

**总计:** ~275 行 (testutil 95 + test 150 + 域文档 30)

### 附录 B：Rollback Plan

| 层级 | 回滚机制 | 触发条件 |
|------|---------|---------|
| **PR revert** | `git revert <commit>` (squash merge) | 任何 sub-test 失败 |
| **callback 字段移除** | `SequenceLLMStub.FrameDeltaInject` 字段 default nil → 旧行为 (span count = 0) | testutil callback 字段未使用可平滑移除 |
| **helper 函数移除** | `SeedPriorExecContext` 函数未调用 → 旧行为 (Round 1 prevExecCtx=nil) | helper 函数未调用可平滑移除 |
| **域文档回滚** | `git revert` CHANGELOG.md + t-registry.md + mups-frame-delta-spec.md | 文档同步问题 |

### 附录 C：回归风险评估

| 风险 | 影响 | 测试策略 |
|------|------|---------|
| `SequenceLLMStub.FrameDeltaInject` 字段添加影响其他 D7 测试 | 低 (default nil,旧行为) | `go test -race ./tests/integration/d7/...` 全 PASS |
| `SeedPriorExecContext` 函数注册 WorkItemExecContext 影响其他 D7 测试 | 低 (helper 函数不调即 0 副作用) | 同上 |
| testutil 修改导致 layer-lint 报警 | 中 | `layer-lint` CI step 已 PASS (DM-20260705-010 引入 ObsConfig 字段同模式) |
| span attr key 与父 change spec 不一致 | 低 (本 change 不修改 span emit 代码) | 沿用父 change 已对齐的 attr (`convergence.uncertainty_reduction_rate`) |

### 附录 D：S3 检查清单自检 + S3-Gate Review

**S3 Checklist:**
- [x] ① 架构目标 — 业务目标 + 技术目标 + 约束条件 三段齐全
- [x] ② 架构原则 — 5 条原则 + 命名规范 + 代码风格 三段齐全
- [x] ③ 业务流程 — 核心用例时序图 + 4 类 Fallback + 决策树 三段齐全
- [x] ④ 领域模型 — 无新聚合根 (testutil_only scope) + 限界上下文 + 领域事件 (沿用) + 跨域消费 0 影响
- [x] ⑤ 核心链路图 — 端到端路径 + stub vs running system 差异分析 (AC4 文档化) + 5 单点风险
- [x] ⑥ 接口/API 设计 — 风格 + 契约 + 幂等 + 版本演进 四段齐全
- [x] 附录 A — File Manifest (7 文件 +275 行)
- [x] 附录 B — Rollback Plan (4 层)
- [x] 附录 C — 回归风险评估 (4 风险)
- [x] 附录 D — S3 Checklist 自检

**S3-Gate Review 结论:** 待启动 — 需 user `/bind workspace-codex` + `/bind workspace-cursor` 解锁 cc-connect relay,然后 codex + cursor + claude 三方 review。

**S3-Gate 三方 review 重点:**
- codex:production code 路径 0 修改 + testutil callback 语义清晰度
- cursor:stub vs production 行为差异文档化充分性 + e2e 覆盖度
- claude 主导:design.md §5 stub vs running system 差异分析 + AC4 文档化

### 附录 E：下一步

- S3-Gate 三方 review (待 user `/bind` cc-connect relay)
- S4 implementation:`SequenceLLMStub.FrameDeltaInject` + `SeedPriorExecContext` helper + 3 sub-test
- S5 acceptance:`TestIntegration_D7FrameDelta_Phase1And2SpanTrigger` + `MonotonicWithSeed` 全 PASS
- S6 archive:`openspec/archive/2026-07-06-devrix-d7-frame-delta-phase1-2-span-trigger/` + `demand-archive-index.md` 登记 + verify-archive.sh 12/12 PASS