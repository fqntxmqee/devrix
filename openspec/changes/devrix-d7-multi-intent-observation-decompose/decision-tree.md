# Decision Tree: Observe 节点分流方案(5 场景详细 trace)

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Status:** S2_Proposal 配套文档(详细方案)

> 本文档是 `proposal.md §5 Solution` 的展开版,描述 Observe 节点在 5 个典型场景下的完整执行流程,以及"直接 return vs 转发 Plan 节点"的硬规则。S3-Gate 时与 proposal + design + tasks 一起 review。

> **⚠️ 2026-07-07 事实校准**:本文档中提到的 `SpawnPolicy` 5 值联合(`InlineRetry / SpawnChild / SpawnEscalateHuman / DecomposeIntoChildren / DecomposeByIntentSegments`)**与代码事实不符**。实际 `SpawnPolicy` 是 `workmodel/pipeline_round.go:27-34` 的 3 值字符串枚举(`SpawnNone / SpawnDecompose / SpawnInline`),由 D7 Convergence Contract CC-1.1~CC-1.5 锚定,不可改。本 Change 改为方案 β:`Plan` 加 2 可选字段(`IntentSegmentSet + DAG`)承载 multi-intent 语义,SpawnPolicy 完全不动。本文档中遗留的 5 值联合描述仅作"原方案 α 的旧描述",不作为实施依据。**Single-Layer Scope Banner**:6 节点流水线 (Observe/Plan/Execute/Verify/Decision/Learn) 是 **WorkTree 的单层 inner-loop**,跨层递归(`MaxDecomposeDepth = 3`)走 v2 多层 worktree 协议。

---

## 1. 决策矩阵(5 场景 × 7 维度)

| 场景 | segment 数 | 确定性 | 不确定性 | Plan 调用 | Execute 调用 | stream emit 次数 | 总延迟 |
|---|---|---|---|---|---|---|---|
| **S-A 单确定性** | 1 | 1 | 0 | ❌ | ❌ | 1(final) | ~1s |
| **S-B 单不确定性** | 1 | 0 | 1 | ✅ | ✅(单 Worker) | 1(final) | ~4s |
| **S-C 多确定性** | N≥2 | N | 0 | ❌ | ❌ | 1(merged final) | ~1s |
| **S-D 多不确定性** | N≥2 | 0 | N | ✅(产 DAG) | ✅(RunPlanDAG 并行) | 1(rollup final) | ~5s |
| **S-E 混合** | N≥2 | ≥1 | ≥1 | ✅(产 DAG) | ✅(RunPlanDAG 并行) | N partial + 1 final | ~1s 看到快答,~5s 看最终 |

7 个判定维度(每行从 Observe 进 ItemPipelineRunner.Run 开始算):

1. **segment 数** = len(IntentSegmentSet.Segments)
2. **spawn policy** = Plan 节点选定(`InlineRetry` / `DecomposeByIntentSegments` / 旧 `DecomposeIntoChildren`)
3. **Execute 模式** = 单 Worker 串行 / RunPlanDAG 并行 4 worker / 完全跳过
4. **emit 策略** = 0 emit(全 fast-path,只走 round.ArtifactSummary) / 1 final / N partial + 1 final
5. **Learn 调用次数** = 1 / N / 0(纯降级)
6. **Return 路径** = fast-path / 完整 round / parent rollup round
7. **降级路径** = PlanParseReject 重试 / 顺序 await / N/A

---

## 2. S-A 单确定性:`1+1=几?`

**输入**:`Directive = "1+1=几?"`

### Observe

```
IntentSegmenter.Segment(directive)
  → LLMIntentSegmenter returns IntentSegmentSet{
      Segments: [{ID: "seg_a", Text: "1+1=几?", IntentKind: "deterministic", Confidence: 0.95}],
      SourceDirective: "1+1=几?",
    }
  → 1 segment,len=1,kind=deterministic ✓
```

### Plan(直接 InlineRetry,本场景不调用 LLM)

Observe 阶段产 Confidence≥0.85 的 ObsFact statement="1+1=2" → 满足 DM-20260706-011 fast-path gate。ItemPipelineRunner 检测到 `len(Segments) == 1 && all deterministic && has high-strength fact` → **跳过 Plan + Execute + Verify**,只跑 Learn。

### 执行流

```
ItemPipelineRunner.Run()
  ├─ Observe (1 LLM call, ~0.6s)
  │   └─ emit ObservationReport{ObsFact strength=0.85 "1+1=2"}
  ├─ Fast-path gate ✓
  │   ├─ !isRollup ✓
  │   ├─ !isDeliverableSynth ✓
  │   ├─ !isParentRollup ✓
  │   ├─ r.Learner != nil ✓
  │   ├─ !hasObsUncertainty(report) ✓ (无 LLM Uncertainty)
  │   └─ pickHighStrengthBusinessFact(report, 0.85) → "1+1=2" ✓
  ├─ maybeObservationalAnswer(ctx, ..., factStmt="1+1=2")
  │   ├─ emit EngineEvent "1+1=2" ← STREAM EMIT (1 final)
  │   ├─ r.Learner.Learn(ctx, Verdict{Kind: Pass, SourceID: "obs_fact:seg_a_id"})
  │   └─ return round{ArtifactSummary: "1+1=2", ExitReason: "observational_answer"}
  └─ Skip Plan/Execute/Verify entirely
```

### Return

round 返回给 caller。**Observe 阶段产物即终态,不进 Plan**。

**延迟**:~1s(单次 Observe LLM 调用)。

---

## 3. S-B 单不确定性:`查 devrix 项目结构`

**输入**:`Directive = "查 devrix 项目结构"`

### Observe

```
IntentSegmenter.Segment(directive)
  → IntentSegmentSet{Segments: [{ID: "seg_a", IntentKind: "explore", Confidence: 0.4}]}
  → 1 segment,kind=explore ✗ (非确定性)
```

### Plan(走 InlineRetry + 单 Worker)

```
Observe 输出 ObservationReport{ObsUncertainty "如何界定'项目结构'?" strength=0.6}
  → fast-path gate FAIL(hasObsUncertainty = true)
  → 走旧 Plan 路径:StrategicPlanProposer → Plan(spec 拆 1 个 child WorkItem)
  → SpawnPolicy = SpawnChild / InlineRetry
```

### 执行流

```
ItemPipelineRunner.Run()
  ├─ Observe (1 LLM call, ~0.6s)
  ├─ Fast-path gate FAIL ←—— 触发降级
  ├─ Plan (~1 LLM call, ~1s)
  │   └─ Plan{DecomposeIntoChildren: [explore_devrix]}
  ├─ Execute (单 Worker, ~3-5s)
  │   └─ Worker(Hardening) 走 ReAct 循环
  ├─ Verify (~0.5s)
  ├─ Learn (per-child α++)
  └─ emit final EngineEvent(完整 finalText)
```

### Return

完整 round 含 ArtifactSummary(Execute 输出)+ VerdictKind + ExitReason。**走 Plan + Execute + Verify + Learn,4 节点全跑**。

**延迟**:~4-5s。

---

## 4. S-C 多确定性:`1+1=? 2×3=? 巴黎时区?`

**输入**:`Directive = "1+1=? 2×3=? 巴黎时区?"`

### Observe

```
IntentSegmenter.Segment(directive)
  → LLMIntentSegmenter 返回 IntentSegmentSet{
      Segments: [
        {seg_a "1+1=?", deterministic, confidence=0.95},
        {seg_b "2×3=?", deterministic, confidence=0.95},
        {seg_c "巴黎时区?", deterministic, confidence=0.85},
      ],
    }
  → 3 segments,len=3,all deterministic ✓
```

### 每个 segment 都被 Observe LLM 同时识别

```
ObservationReport{
  ObsFact{statement: "1+1=2 (标准算术)", strength: 0.85},
  ObsFact{statement: "2×3=6 (标准算术)", strength: 0.85},
  ObsFact{statement: "UTC+1 冬令时 / UTC+2 夏令时 (CEST)", strength: 0.85},
}
```

### Plan(不调用 + Segment-Merger)

```
Fast-path gate 触发(N 个 deterministic segments with high-strength fact)
  → 走 maybeObservationalAnswerMulti(merged fact statements)
  → 在 Learn 阶段,按 segment 维度单独 BayesianUpdate(3 票 VerdictPass)
  → parent rollup round(无需 PlanDAG,因为 fast-path 不进入 Execute)
```

### 执行流

```
ItemPipelineRunner.Run()
  ├─ Observe (1 LLM call, ~0.7s,单次调用识别 3 个 segment)
  ├─ Fast-path multi-segment gate ✓
  │   ├─ len(Segments) >= 2 ✓
  │   ├─ all segments deterministic ✓
  │   ├─ each has high-strength ObsFact ✓
  │   └─ !hasObsUncertainty ✓
  ├─ maybeObservationalAnswerMulti(ctx, segments=[seg_a, seg_b, seg_c])
  │   ├─ merged Artifacts = "1+1=2;  2×3=6;  巴黎时区 UTC+1/UTC+2"
  │   ├─ emit 1 final EngineEvent (单卡包含 3 个答案,序列化为 markdown bullet)
  │   ├─ r.Learner.Learn × 3(每个 segment 一票 Pass)
  │   └─ return parent rollup round{ArtifactSummary: merged, ExitReason: "observational_answer_multi"}
  └─ Skip Plan/Execute/Verify entirely
```

### Return

单条 final card 含 3 个答案。**不进 Plan 节点**。

**延迟**:~1s。

**关键设计决策**:多确定性场景**是否合并成 1 卡 vs 多 partial 卡**?当前设计选合并(简洁);若用户后续要 streaming partial,可切到 S-E 模式(给 partial + final)。

---

## 5. S-D 多不确定性:`查 devrix 架构 + 分析 d2 风险 + 评估 v7 演进`

**输入**:`Directive = "查 devrix 架构 + 分析 d2 风险 + 评估 v7 演进"`

### Observe

```
IntentSegmenter.Segment(directive)
  → IntentSegmentSet{
      Segments: [
        {seg_a "查 devrix 架构", explore, confidence=0.6},
        {seg_b "分析 d2 风险", analyze, confidence=0.5},
        {seg_c "评估 v7 演进", commit, confidence=0.5},
      ],
    }
  → 3 segments,all uncertain ✓
```

### Plan(LLM 提议 DAG + validate)

```
Observe 输出 3 个 ObsSignal/ObsUncertainty → fast-path gate FAIL
  → Plan 节点进 StrategicPlanProposer
  → LLM 返回 PlanDAG{
      Nodes: [n_a explore_devrix, n_b analyze_d2_risk, n_c evaluate_v7],
      Edges: [],                  # 无数据依赖,完全独立
      Priorities: {n_a: 70, n_b: 50, n_c: 30},   # devrix 架构先
      MaxParallelism: 4,
    }
  → validateDAG(dag):
      ✓ acyclic(DAG 是 node set with no edges)
      ✓ n_nodes=3 ≤ MaxFanOut=8
      ✓ node IDs unique
      ✓ no dangling edges
  → SpawnPolicy = DecomposeByIntentSegments(segments)
```

### Execute(RunPlanDAG + 4 worker)

```
ItemPipelineRunner.Run()
  ├─ Observe
  ├─ Plan → PlanDAG valid
  ├─ WaveScheduler.RunPlanDAG(dag)
  │   ├─ initialReadySet = [n_a, n_b, n_c] (无 edges,全 ready)
  │   ├─ priority heap pop → n_a priority=70 抢 worker slot 1
  │   ├─ priority heap pop → n_b priority=50 抢 worker slot 2
  │   ├─ priority heap pop → n_c priority=30 抢 worker slot 3
  │   ├─ 3 个 Worker 并行执行(每个 ~3-5s,但同时跑)
  │   │   ├─ Worker 1: explore devrix 架构 (n_a) → 0.5s 后发 partial card
  │   │   ├─ Worker 2: analyze d2 风险 (n_b) → 1s 后发 partial card
  │   │   └─ Worker 3: evaluate v7 演进 (n_c) → 4s 后发 partial card
  │   ├─ (任一 error → cancel 未启动 sibling + drain emit)
  │   └─ all done → parent rollup node 聚合 3 child 结论
  ├─ emit 1 final EngineEvent (parent rollup)
  └─ Learn × 3(per-segment α/β) + ParentEvidence 聚合
```

### Return

1 final card(汇总 3 child 结论)。**走 Plan + RunPlanDAG + parent rollup**。

**延迟**:~5s(取 max(child) ≈ n_c,最慢那个)。

### 5.1 Plan LLM 输出(具体 PlanDAG + AC[])

```
PlanLLMInput{
  directive: "查 devrix 架构 + 分析 d2 风险 + 评估 v7 演进",
  segments: IntentSegmentSet{3 segments, 全部 uncertain},
  prior_parse_reject: nil,
}

PlanLLMOutput{
  dag: PlanDAG{
    Nodes: [
      {ID: "n_a", SegmentID: "seg_a", WorkerHint: "wave",
       ExpectedArtifactTags: ["final_text","evidence"],
       AcceptanceCriteria: [
         {ID: "AC-n_a.1", Description: "架构描述覆盖 D1-D7 全部 7 层",
          CheckKind: MentionsAll, CheckArgs: {tokens: ["D1","D2","D3","D4","D5","D6","D7"]},
          Severity: Required,
          Rationale: "完整性,缺一层不算回答架构"},
         {ID: "AC-n_a.2", Description: "每层职责清晰",
          CheckKind: CustomLLMJudge, CheckArgs: {judge_prompt: "判断 D1-D7 职责描述是否清晰可理解"},
          Severity: Preferred,
          Rationale: "额外加分"},
       ]},
      {ID: "n_b", SegmentID: "seg_b", WorkerHint: "wave",
       ExpectedArtifactTags: ["final_text","risk_register"],
       AcceptanceCriteria: [
         {ID: "AC-n_b.1", Description: "至少识别 3 个 d2 风险",
          CheckKind: MentionsAll, CheckArgs: {tokens: ["风险1","风险2","风险3"]},
          Severity: Required,
          Rationale: "d2 是 devrix 核心域,3 风险是基线"},
         {ID: "AC-n_b.2", Description: "含 Owner/缓解/优先级",
          CheckKind: CustomLLMJudge, CheckArgs: {judge_prompt: "评估风险登记是否含 3 字段"},
          Severity: Required,
          Rationale: "无字段=空话"},
       ]},
      {ID: "n_c", SegmentID: "seg_c", WorkerHint: "wave",
       ExpectedArtifactTags: ["final_text","roadmap"],
       AcceptanceCriteria: [
         {ID: "AC-n_c.1", Description: "v7 演进含 3+ 阶段",
          CheckKind: CustomLLMJudge, CheckArgs: {judge_prompt: "判断 v7 演进是否分阶段且 ≥ 3 阶段"},
          Severity: Required,
          Rationale: "演进路径必须可拆"},
       ]},
    ],
    Edges: [],   # 无数据依赖
    Priorities: {n_a: 70, n_b: 50, n_c: 30},
  },
  rationale: "3 个 segment 互不依赖,完全并行。n_a 最高优先级(架构是基础)...",
}
```

### 5.2 Verify 节点处理(每个 child 独立)

```
PerChildVerify(n_a):
  - AC-n_a.1 (MentionsAll) → 本地机械执行 → 7 token 全部命中 → Pass
  - AC-n_a.2 (CustomLLMJudge) → D3 LLM 调用 → "清晰" → Pass
  - Aggregate: 全部 Pass → VerdictKind = Pass
  - 落 round.Metadata["ac_verdicts"] = JSON([{criterion_id, outcome, evidence}])

PerChildVerify(n_b):
  - AC-n_b.1 (MentionsAll) → 本地 → 3 risk token 全部命中 → Pass
  - AC-n_b.2 (CustomLLMJudge) → D3 LLM → "含 3 字段" → Pass
  - Aggregate: 全部 Pass → VerdictPass

PerChildVerify(n_c):
  - AC-n_c.1 (CustomLLMJudge) → D3 LLM → "分 4 阶段" → Pass
  - Aggregate: Pass → VerdictPass

ParentRollup:
  - 3 child 全部 VerdictPass → emit 1 final card (合并 3 child finalText)
  - Learn × 3 + ParentEvidence aggregator (sum α/β)
```

---

## 6. S-E 混合(用户最初问的):`1+1=? + 查 devrix 架构`

**输入**:`Directive = "1+1=? + 查 devrix 架构"`

### Observe

```
IntentSegmenter.Segment(directive)
  → IntentSegmentSet{
      Segments: [
        {seg_a "1+1=?", deterministic, confidence=0.95},
        {seg_b "查 devrix 架构", explore, confidence=0.6},
      ],
    }
  → 2 segments,mixed ✓ (1 deterministic + 1 uncertain)
```

### Plan(LLM 提议 DAG,部分 fast-path 部分正常)

```
Observe 输出:
  ObsFact{statement: "1+1=2", strength: 0.85}        # seg_a
  ObsUncertainty{question: "如何界定'架构'?", strength: 0.6}  # seg_b
  → fast-path gate FAIL(hasObsUncertainty = true,seg_b)
  → Plan LLM 提议 PlanDAG{
      Nodes: [n_a deterministic, n_b explore],
      Edges: [],  # 无依赖
      Priorities: {n_a: 90, n_b: 50},  # 快答优先
    }
```

### Execute(混合模式)

```
ItemPipelineRunner.Run()
  ├─ Observe (~0.7s)
  ├─ Fast-path gate FAIL ←—— hasObsUncertainty=true 全局挡,但...
  ├─ ▼ 关键设计:per-segment 评估 fast-path eligibility
  │   ├─ seg_a:has fast-path ObsFact ✓ → 立即 fast-path emit
  │   └─ seg_b:no fast-path → 走 Plan/Execute
  ├─ RunPlanDAG
  │   ├─ Worker 1 (slot 1): seg_a fast-path emit partial card "1+1=2" (@ 0.5s)
  │   └─ Worker 2 (slot 2): seg_b normal Plan/Execute (~3-5s,@ 4s emit partial)
  ├─ parent rollup:合并 2 segment 最终答案 → emit 1 final card
  └─ Learn × 2(seg_a Pass 立即 bump α;seg_b 走完整流程)
```

### Return

**用户看到 2 张卡 + 1 张 final**(飞书 IM 顺序为 partial-fast → partial-slow → final)。

**延迟**:
- t=0.5s 看到 "1+1=2"
- t=4s 看到 "devrix 架构: D1-D7 七层架构 + ..."
- t=4.5s 看到 final card 合并版

### 6.1 Plan LLM 输出(具体 PlanDAG + AC[])

```
PlanLLMInput{
  directive: "1+1=? + 查 devrix 架构",
  segments: IntentSegmentSet{2 segments, 1 deterministic + 1 explore},
  prior_parse_reject: nil,
}

PlanLLMOutput{
  dag: PlanDAG{
    Nodes: [
      {ID: "n_a", SegmentID: "seg_a", WorkerHint: "wave",
       ExpectedArtifactTags: ["final_text"],
       AcceptanceCriteria: [
         {ID: "AC-n_a.1", Description: "算术答案正确",
          CheckKind: NumericRange, CheckArgs: {min: 1.5, max: 2.5},
          Severity: Required,
          Rationale: "1+1=2 唯一正确答案(整数范围)"},
       ]},
      {ID: "n_b", SegmentID: "seg_b", WorkerHint: "wave",
       ExpectedArtifactTags: ["final_text","evidence"],
       AcceptanceCriteria: [
         {ID: "AC-n_b.1", Description: "覆盖 D1-D7 全部 7 层",
          CheckKind: MentionsAll, CheckArgs: {tokens: ["D1","D2","D3","D4","D5","D6","D7"]},
          Severity: Required,
          Rationale: "完整性"},
         {ID: "AC-n_b.2", Description: "每层职责清晰",
          CheckKind: CustomLLMJudge, CheckArgs: {judge_prompt: "判断 D1-D7 职责描述是否清晰"},
          Severity: Preferred,
          Rationale: "额外加分"},
       ]},
    ],
    Edges: [],   # 无依赖
    Priorities: {n_a: 90, n_b: 50},   # 快答优先
  },
  rationale: "seg_a 确定性走 fast-path,seg_b 走 Worker;两条线并行,parent rollup 合并。",
}
```

### 6.2 Verify 节点处理(per-segment 评估)

```
PerSegmentVerify(seg_a / n_a):
  - Observe 阶段已识别 fast-path → **跳过 Verify**(Verify 不接受 fast-path segment)
  - 走 r.Learner.Learn(VerdictPass) 直接 bump α
  - 无 ac_verdicts 落盘(没进 Verify)

PerSegmentVerify(seg_b / n_b):
  - AC-n_b.1 (MentionsAll) → 本地机械 → 7 token 命中 → Pass
  - AC-n_b.2 (CustomLLMJudge) → D3 LLM 调用 → "清晰" → Pass
  - Aggregate: 全部 Pass → VerdictPass
  - 落 round.Metadata["ac_verdicts"] = JSON(seg_b verdicts)

ParentRollup:
  - 合并 seg_a fast-path finalText + seg_b verified finalText
  - emit 1 final card
  - Learn × 2:seg_a (fast-path Pass) + seg_b (verified Pass) + ParentEvidence aggregator
```

### 6.3 S-E 关键设计:per-segment fast-path 评估

**全局 fast-path gate 失败**(因为 seg_b 有 ObsUncertainty),**但 n_a 子段仍走 fast-path**:

- Plan LLM 不需要为 seg_a 设计 WorkerHint(看 AcceptanceCriteria AC-n_a.1 走 NumericRange 本地,不调 LLM)
- WaveScheduler 调度时,识别 n_a 是 deterministic + 有 NumericRange AC → 内部走 fast-path emit partial
- n_b 走 Worker + Verify 完整链路
- parent rollup 合并两者

**这与 S-A/S-C 直接在 Observe 阶段 fast-return 的区别**:S-A/S-C 整个 directive 不进 Plan;S-E 进 Plan 但 n_a 子段走 fast-path(Plan LLM 显式声明 AC,Verify 跳过)。

---

## 7. Observe 直接 return vs 转发 Plan 的硬规则

### Observe 直接 return(不走 Plan 节点)的条件(全部满足)

```
✅ len(IntentSegmentSet.Segments) >= 1
✅ ∀ segment ∈ Segments:
     - segment.Confidence >= 0.85
     - segment.IntentKind ∈ {deterministic}
     - 对应 ObservationReport 中存在对应 high-strength ObsFact
✅ ∀ o ∈ report.Observations:
     - o.Source NOT IN {"item_pipeline", "verify_signal"}  (系统注入不算)
     - if o.Kind == ObsUncertainty → 一律 FAIL
✅ !isRollup && !isDeliverableSynth && !isParentRollup
✅ r.Learner != nil
✅ 至少 1 个 segment 有 non-empty ObsFact.Statement
```

### 否则降级,走 Plan 节点

```
- multi-intent(segments >= 2)              → Plan 提议 PlanDAG
- single-uncertain(segments == 1, uncertain) → Plan 提议单 child Plan
- 任何 Uncertainty 出现                          → Plan 提议携带 Uncertainty 处理的 Plan
- 任何 segment 不满足确定性                       → 整个 directive 进 Plan(不切分)
- rollup / human-review item                 → 旧路径,不走 fast-path
```

### 场景 → 执行模式映射

| 场景 | Observe 直接 return? | Plan 路径 | Execute 路径 | Emit 次数 |
|---|---|---|---|---|
| **S-A 单确定性** | ✅ Yes | 跳过 | 跳过 | 1 final |
| **S-B 单不确定性** | ❌ No | InlineRetry | 单 Worker 串行 | 1 final |
| **S-C 多确定性** | ✅ Yes | 跳过(maybeObservationalAnswerMulti) | 跳过 | 1 final |
| **S-D 多不确定性** | ❌ No | DecomposeByIntentSegments → PlanDAG | RunPlanDAG 并行 | 1 final |
| **S-E 混合** | ❌ No | DecomposeByIntentSegments → PlanDAG | RunPlanDAG 混合(fast-path 部分 + Worker 部分) | N partial + 1 final |

### Observe 直接 return 的场景

- S-A 单确定性
- S-C 多确定性

### Observe → Plan 节点 触发的场景

- S-B 单不确定性
- S-D 多不确定性
- S-E 混合

---

## 8. 边界 case / 降级路径

| Case | 降级策略 |
|---|---|
| IntentSegmenter 切出 0 segment | 强制 1-element set(directive = 1 segment) |
| IntentSegmenter LLM 超时 800ms | fallback RuleBasedSegmenter(regex) |
| RuleBased 也命中失败 | 1-element set(directive = 1 segment) |
| validateDAG 失败(环) | PlanParseReject.RejectCycle + LLM 重试 ≤ 2 次 |
| validateDAG 失败(超 fan-out) | PlanParseReject.RejectTooManyNodes + LLM 重试 |
| RunPlanDAG error propagation | cancel 未启动 sibling + drain emit channel |
| stream emit 重复调 | idempotency key (session_id, segment_id) 命中 → noop |
| parent rollup 时有 child 未完成 | 等所有 child 完成才 emit final(无 staleness) |
| 部分 child error | parent rollup 标注 `partial_failure`,Learn 记录 Beta++ |
| feature flag off | 完全走旧 Plan + 顺序 await(DAG 路径未启用) |
| 系统注入 ObsUncertainty(source=item_pipeline) | 由 hasObsUncertainty 过滤掉,不算 Uncertainty |
| multi-segment fast-path(>= 2 segments,全 deterministic) | 合并到 1 个 final card,不走 RunPlanDAG(节省 4 worker slot) |
| spawn policy Conflict | 旧 SpawnPolicy.DecomposeIntoChildren 优先(向后兼容);flag rollout 后收紧 |

---

## 8.5 各场景 Plan LLM 输出 + Verify 节点处理对照表(集中参考)

下表 5 场景 × 4 维度。每行从 Plan LLM 输入到 Verify 节点处理完整列出,**用于 S4 实现时一一映射到代码**。

| 场景 | Plan LLM 是否调用 | PlanLLMInput 关键字段 | PlanLLMOutput 关键字段 | Verify 节点处理 |
|------|------------------|----------------------|----------------------|-----------------|
| **S-A 单确定性**(`1+1=几?`) | ❌ 跳过(Observe 阶段 fast-return) | — | — | ❌ 跳过(走 fast-path);仅 Learn 写 VerdictPass |
| **S-B 单不确定性**(`查 devrix 项目结构`) | ✅ 调用(走旧 SpawnPolicy.InlineRetry) | `directive`, `segments=[seg_a explore]`, `prior_parse_reject=null` | `dag={1 node, no AC[Required LLM judge]}`, `rationale` | 1 child verify:CustomLLMJudge 1 次 → PerCriterionVerdict[1] → Aggregate(Required Pass) → VerdictPass |
| **S-C 多确定性**(`1+1=? 2×3=? 巴黎时区?`) | ❌ 跳过(maybeObservationalAnswerMulti) | — | — | ❌ 跳过(走 fast-path);Learn × 3(每 segment 一票 VerdictPass) |
| **S-D 多不确定性**(`查 devrix 架构 + 分析 d2 风险 + 评估 v7 演进`) | ✅ 调用(走 SpawnPolicy.DecomposeByIntentSegments) | `directive`, `segments=[seg_a explore, seg_b analyze, seg_c commit]`, `prior_parse_reject=null` | `dag={3 nodes, no edges, priorities={70,50,30}}`, `acceptance_criteria=[6 AC: 3 Required 机械 + 3 Required LLM judge]`, `rationale` | 3 child verify 并行:n_a 2 AC(1 MentionsAll 机械 + 1 LLM judge),n_b 2 AC,n_c 1 AC(LLM judge);各 child Aggregate → 3 个 VerdictPass;parent rollup emit final + Learn × 3 + ParentEvidence sum |
| **S-E 混合**(`1+1=? + 查 devrix 架构`) | ✅ 调用(走 SpawnPolicy.DecomposeByIntentSegments) | `directive`, `segments=[seg_a deterministic, seg_b explore]`, `prior_parse_reject=null` | `dag={2 nodes, no edges, priorities={90,50}}`, `acceptance_criteria=[3 AC: seg_a 1 Required NumericRange 机械, seg_b 2 AC(1 MentionsAll 机械 + 1 LLM judge Preferred)]`, `rationale` | seg_a 跳过 Verify(fast-path segment);seg_b verify 2 AC;parent rollup emit final + Learn × 2(seg_a fast-path Pass + seg_b verified Pass) |

### 8.5.1 Plan LLM 输入 3 要素(每场景通用)

```
type PlanLLMInput struct {
  Directive        string                  // 用户原始输入
  Segments         IntentSegmentSet        // Observe 阶段切分结果
  PriorParseReject *PlanParseReject        // 首轮 nil,重试时携带
}
```

### 8.5.2 Plan LLM 输出 3 要素(每场景必含)

```
type PlanLLMOutput struct {
  DAG                 PlanDAG               // validateDAG 必须通过
  AcceptanceCriteria  []AcceptanceCriterion // 每个 node 至少 1 Required
  Rationale           string                // 可选,辅助 Verify LLM judge
}
```

### 8.5.3 3-shot example(Plan prompt appendix 必含)

S4 实现时 Plan LLM 调用的 prompt 必须含以下 3 个 example(无 example LLM 不知道 schema):

1. **全 deterministic 场景**(对应 S-C):3 个算术 question,PlanDAG 3 nodes(no edges),AC[Required NumericRange]
2. **全 uncertain 场景**(对应 S-D):3 个 explore/analyze/commit 任务,PlanDAG 3 nodes(no edges),AC[MentionsAll + CustomLLMJudge]
3. **混合场景**(对应 S-E):1 deterministic + 1 explore,PlanDAG 2 nodes,AC[NumericRange + MentionsAll + CustomLLMJudge]

### 8.5.4 Verify 节点处理 3 步(每 child 通用)

```
Step 1: 接收 PlanLLMOutput + Artifact + PlanRationale
Step 2: PerCriterionExecutor.Execute:
        - 机械 CheckKind (ContainsString/NotContains/Numeric/Length/MentionsAll/MentionsAny/JSONPath) → 本地 < 5ms
        - CustomLLMJudge (≤ 3) → 串行调 D3 LLM ~1s
        - 收集 PerCriterionVerdict[] (顺序对齐)
Step 3: PerCriterionExecutor.Aggregate:
        - ∃ Required Fail → VerdictFail
        - 全部 Required Pass + ∃ Preferred Fail → VerdictPartial
        - 全部 Pass → VerdictPass
        - 任一 Error 且无 Fail → VerdictIndeterminate
        - 落 round.Metadata["ac_verdicts"] = JSON
```

### 8.5.5 与上游 spec_delta.md 章节对照

| 维度 | spec_delta.md 位置 | decision-tree.md 引用 |
|------|---------------------|------------------------|
| PlanDAG 节点结构 | §3 + §11(扩展 AC[] 字段) | §5.1 / §6.1(具体 PlanLLMOutput) |
| AcceptanceCriterion | §3.5 | §5.1 / §6.1(具体 AC[])+ §8.5.2(3 要素) |
| PerCriterionVerdict | §3.6(aggregation_rule) | §8.5.4 Step 3 |
| PlanLLMIO | §9 | §8.5.1 输入 + §8.5.2 输出 + §8.5.3 3-shot example |
| VerifyLLMIO | §10 | §8.5.4 Verify 节点处理 |
| PlanAcceptanceContractBuilder | §12 | §8.5.4 Step 1(调用 Build 校验) |

---

## 8.6 Decision Node 5 路径详细表(独立 stage,D7 6 节点流水线)

Verify 节点产出 VerdictKind 后,Decision 节点(独立 stage)立即跑映射表,产出 5 路径决策。**D7 6 节点流水线**(Observe/Plan/Execute/Verify/Decision/Learn,都是独立 stage,Decision 不再合并到 Verify)。

### 8.6.1 决策映射表(10 行 Verdict-based + 1 plan_error = 11 行总,纯规则引擎 0 LLM)

| Verdict | Other Conditions | Decision | 后续动作 | 持久化 |
|---------|------------------|----------|----------|--------|
| **Pass** | (default) | **A accept** | emit final + Learn | decision.kind=accept |
| **Partial** | Tolerance=high OR ChildBudget=0 | **A accept** | emit final + Learn | decision.kind=accept |
| **Partial** | Partial AC 可独立分解 + ChildBudget>0 | **C child_worker** | spawn 子 Worker + 子 AC[] 复用 | decision.kind=child_worker + next_spec |
| **Partial** | 其它 | **A accept** | emit final + Learn | decision.kind=accept |
| **Fail** | AttemptNo < MaxRetry=1 | **B retry** | RoundMeta.AttemptNo++ 重跑 | decision.kind=retry |
| **Fail** | AttemptNo >= MaxRetry=1 | **E human_review** | 飞书卡"❓" + emit abort | decision.kind=human_review |
| **Indeterminate** | RiskLevel=high | **E human_review** | 飞书卡"❓" + emit abort | decision.kind=human_review |
| **Indeterminate** | RiskLevel=normal/low | **B retry** | 重跑 | decision.kind=retry |
| **Error(全Err)** | Network/Timeout 类 | **B retry** | 重跑(1 次) | decision.kind=retry |
| **(任意 Verdict)** | IsChildSegment + SiblingDecidedCount==SiblingTotalCount | **D parent_rollup** | 触发 parent rollup 节点 | decision.kind=parent_rollup |

### 8.6.2 5 场景的 Decision 路径分布

| 场景 | seg_a 决策 | seg_b 决策 | seg_c 决策 | parent rollup 决策 |
|------|-----------|-----------|-----------|-------------------|
| **S-A 单确定性** | — (fast-path 不进 Verify) | — | — | — |
| **S-B 单不确定性** | A accept (LLM judge 易通过) | — | — | — |
| **S-C 多确定性** | — (fast-path) | — | — | — |
| **S-D 多不确定性** | A accept (mentions_all pass) | A accept (mentions + LLM judge pass) | A accept (LLM judge pass) | D parent_rollup(等 3 child 都 A) |
| **S-E 混合** | — (fast-path) | A accept (mentions + LLM judge pass) | — | D parent_rollup(等 seg_b 完成) |

### 8.6.3 边界 case / 降级路径

| Case | 降级策略 |
|------|----------|
| 静态映射表未命中(空 decision) | 降级 A accept + slog.Warn("decision_map_miss") |
| C child_worker 触发但 ChildBudgetRemaining=0 | 降级 A accept + log warning |
| C child_worker 触发但 InheritACSubset 为空 | ErrChildWorkItemInheritACEmpty → 降级 A accept |
| D parent_rollup 触发但 parent 已结束 | 降级 A accept(避免死锁) |
| B retry 触发但 AttemptNo 已经 >= MaxRetry | 跳过 retry 直接 E human_review |
| E human_review 触发但 RiskLevel 未在 metadata | 降级 B retry + log warning |
| Decision 输出 NextWorkItemSpec 但 Kind != child_worker | 视为无效(违反 §3.7 invariant)+ 降级 A |
| 子 Worker 跑失败(context cancel / spawn 失败) | parent decision 降级 A accept + 显示 partial_failure |

### 8.6.4 "下一层 WorkTree" 语义(关键设计决策)

✅ **当前 segment 内 spawn 子 Worker**(v1 选):

```
parent WorkItem (n_b partial fail)
  ├─ AC-b.1 MentionsAll Pass
  ├─ AC-b.2 MentionsAll Pass  
  └─ AC-b.3 CustomLLMJudge Fail  ← 这条触发 child_worker
       ↓
       child WorkItem (spawn)
       ├─ 复用 AC-b.3 (Required Fail 的子集)
       ├─ 独立 round 跑 ItemPipelineRunner
       ├─ Verify 沿用同一 PerCriterion executor
       └─ parent.onChildComplete → parent decision(可能 D parent_rollup)
```

❌ 重新分 segment 再 Plan 一次(留 v2):粒度粗,feedback 路径 +1-2s。

### 8.6.5 Decision 5 路径完整 trace(S-D 多不确定性 + 假设 n_c 触发 C)

```
ItemPipelineRunner.Run() 包含 Verify(节点 4)+ Decision(节点 5)两个独立 stage:
  ├─ Execute (RunPlanDAG 并行 3 worker)
  │   ├─ Worker n_a → emit partial + PerCriterion 2/2 Pass
  │   ├─ Worker n_b → emit partial + PerCriterion 2/2 Pass
  │   └─ Worker n_c → emit partial + PerCriterion 0/1 Fail(LLM judge 1 次没过)
  ├─ Verify (节点 4 独立 stage)
  │   ├─ PerCriterion executor
  │   │   ├─ n_a: 2/2 Pass → VerdictKind=Pass
  │   │   ├─ n_b: 2/2 Pass → VerdictKind=Pass
  │   │   └─ n_c: 0/1 Fail(LLM judge 不通过)→ VerdictKind=Fail
  ├─ Decision (节点 5 独立 stage)
  │   ├─ Decision.Decide (verdict=Fail, AttemptNo=0 < MaxRetry=1, IsChildSegment=true, ...)
  │   │   └─ Match 第 5 行 → Decision.Kind = retry
  │   ├─ n_c AttemptNo++ → RoundMeta.AttemptNo=1
  │   ├─ 重跑 n_c ItemPipelineRunner
  │   │   └─ Worker n_c 重试 → 1/1 Pass(LLM judge 这次通过)
  │   └─ n_c 重试后 Verdict=Pass → A accept
  ├─ parent rollup(等 3 child 都 A):D parent_rollup
  │   └─ 聚合 3 child finalText → emit final card
  ├─ Learn × 3 + ParentEvidence aggregator
  └─ decision metadata 落 reputation:decision_kind=accept(retry 后)
```

### 8.6.6 与其它 spec 章节对照

| 维度 | spec_delta.md | design.md | proposal.md | decision-tree.md |
|------|---------------|-----------|-------------|------------------|
| Decision 类型 | §3.7 | §2.12 | §5.5 | §8.6.1(映射表) |
| ChildWorkItemSpec | §3.8 | §2.13 | §5.5 | §8.6.4(子 Worker) |
| DecisionNodeIO | §13 | §2.12 | §5.5(3 步决策) | §8.6.5(完整 trace) |
| 5 路径枚举 | §3.7 values | §2.12 DecisionKind | §5.5 5 路径表 | §8.6.1(11 行映射 = 10 baseline + 1 plan_error) |
| 边界 case | — | §2.12 错误处理 | — | §8.6.3(8 行降级) |
| AC29-AC34 | 验收标准 | — | — | §8.6.1 + §8.6.3 全覆盖 |
| tasks T46-T52 | — | §2.12 + §2.13 | — | — |

---

## 8.7 Learn Node 详细表(D7 6 节点流水线最末 stage,per-segment 升格 + 契约精简)

Learn 节点是 D7 **6 节点流水线最末 stage**(独立 stage,不是 Verify 内部 stage),但**异步执行,不阻塞 emit final**。接收节点 5(Decision)+ 节点 4(Verify)+ 节点 3(ArtifactHash 可选)的输出,做 BayesianUpdate + ParentEvidence aggregator + 持久化 reputation row。

### 8.7.1 Learn 节点在 6 节点链路中的位置(链路最末,异步)

```
User
  ↓
1. Observe  (~0.7s)
  ↓
2. Plan     (~1s, S-D/S-E) / Fast-path (S-A/S-C)
  ↓
3. Execute  (RunPlanDAG 4 worker, ~3-5s 并行)
  ↓
4. Verify   (PerCriterion, < 50ms)
  ↓
5. Decision (11 行静态映射, < 5ms, 0 LLM)
  ↓    └─ A accept → 走到 Learn
  ↓    └─ B retry  → 回到节点 1-4 循环
  ↓    └─ C child_worker → 子 1-4 循环
  ↓    └─ D parent_rollup → 等所有 child 都 A → 触发 parent rollup
  ↓    └─ E human_review → 飞书 abort + 仍进 Learn(β++)
  ↓
[Emit Final EngineEvent]   ← 用户先看到这步,主流程结束
  ↓
6. Learn (异步,不阻塞)    ← 这步后台异步跑
  ├─ per-segment Learn × N
  ├─ ParentEvidence aggregator (若有 parent_rollup)
  ├─ BayesianUpdate (α++/β++)
  └─ 持久化 reputation row
```

**关键设计决策**:
- D7 升为 **6 节点流水线**(Observe/Plan/Execute/Verify/Decision/Learn,都是独立 stage)
- Learn **不阻塞主流程**:emit final 完成后立即返回,Learn 异步在后台跑(reputation DB 写挂不影响用户感知)
- Learn **仅依赖 Decision + Verify + ArtifactHash**(契约精简,不收 Plan/Observations/ParentContext)

### 8.7.2 LearnRequest / LearnResponse 数据契约(精简版,完整见 spec_delta §3.9-§3.10)

```yaml
LearnRequest:                                # 6 节点契约精简版
  WorkItemID:        string                  # per-segment attribution
  Decision:          Decision                # 来自节点 5
  Verdict:           Verdict{Kind, SourceID, Confidence, Evidence, IndeterminateReason}  # 来自节点 4
  ArtifactHash:      string                  # 来自节点 3(可选,防重)
  PlanRationaleHash: string                  # ≤64B,SHA256[:16] hex,plan_rationale 指纹(H1 修复)
  # 不收字段(契约精简):
  #   - Plan / PlanLLMOutput(Plan 节点自己持久化,reputation 不冗余)
  #   - Observations(Observe 节点自己持久化,Learn 不消费)
  #   - ParentContext(Learn 内部按 Decision.parent_rollup 从 DB 查 child rows)
  #   - plan_rationale 全量(可能 1-2KB,改存 16B 指纹,反查 DB 即可)
  # IndeterminateReason 关键性:区分 verifier_parse_failure vs env_limited(驱动 PendingAsset routing + β++),
  #                          G8-1 修复扩展,2026-07-07 锁定为 LearnRequest 必带字段。

LearnResponse:
  UpdatedAlpha:    float64
  UpdatedBeta:     float64
  ReputationRowID: string
  BayesianAction:  alpha_bump | beta_bump | no_change | force_plan
```

### 8.7.3 BayesianUpdate 公式(纯函数,无 I/O)

```
prior α₀, β₀  (cold start = Beta(5, 3) via BuildAdaptivePrior)

on Verdict.Pass           → α += 1
on Verdict.Fail           → β += 1
on Verdict.Abstain        → no change

on Decision.Kind=accept   → 累计(正常 signal,上面的 α/β 已生效)
on Decision.Kind=retry    → return prior(不累计,避免 noise)
on Decision.Kind=child_worker → child 完成后单独累计;parent 自身 return prior
on Decision.Kind=human_review → β += 1(强 negative,叠加在 Verdict 上)
on Decision.Kind=parent_rollup → sum child α/β(走 ParentEvidence aggregator)

防重:
if hash(prior.ArtifactMetadataHash) == currentHash { return prior(no_change) }

force_plan 触发:
if β / (α + β) > 0.7 { BayesianAction = force_plan }
```

### 8.7.4 ReputationRow Schema(精简版,完整见 spec_delta §3.11)

```yaml
ReputationRow:
  id:                   uuid
  segment_id:           string        # per-segment 独立 row
  parent_id:            string|null   # 指向 parent rollup row
  alpha:                float         # cold start = 5
  beta:                 float         # cold start = 3
  last_updated:         time.Time
  decision_kind_history:[]string      # 最近 5 次 DecisionKind(滚动)
  source_id_history:    []string      # 最近 5 次 Verdict.SourceID
  artifact_metadata_hash:string       # 防重
  metadata:             JSON          # 含 rationale,user_action,force_plan 等(2026-07-07 新增)
  deprecated_plan_rationale:string   # DEPRECATED 2026-07-07 冻结,迁移到 metadata.rationale,保留列只为兼容旧 row
```

### 8.7.5 5 场景的 Learn 节点行为分布

| 场景 | Learn 次数 | 类型 | 落 reputation row | BayesianAction |
|------|-----------|------|------------------|----------------|
| **S-A 单确定性** | 1 | fast-path VerdictPass (SourceID="obs_fact:seg_a_id") | child row seg_a: α++ | alpha_bump |
| **S-B 单不确定性** | 1 | Verify VerdictPass + Decision=accept | child row seg_a: α++ | alpha_bump |
| **S-C 多确定性** | N=3 | fast-path × 3(per-segment) | child row × 3: α++ each | alpha_bump × 3 |
| **S-D 多不确定性** | N=3 + 1 parent | per-segment Learn × 3 + ParentEvidence aggregator | child row × 3 (α++) + parent rollup row (sum) | α_bump × 3 + parent sum no_change |
| **S-E 混合** | 2 + 1 parent | seg_a fast-path Pass + seg_b verified Pass + parent rollup | seg_a α++ + seg_b α++ + parent rollup sum | α_bump × 2 + parent sum no_change |

### 8.7.6 Learn 节点完整 trace(S-D 假设 n_c 触发 retry 一次后 accept)

```
ItemPipelineRunner.Run() 完整链路:
  ├─ Observe (~0.7s)
  ├─ Plan (~1s,产出 PlanDAG 3 nodes)
  ├─ Execute (RunPlanDAG 3 worker 并行, ~5s)
  │   ├─ n_a Worker → emit partial + PerCriterion 2/2 Pass
  │   ├─ n_b Worker → emit partial + PerCriterion 2/2 Pass
  │   └─ n_c Worker → emit partial + PerCriterion 0/1 Fail(LLM judge 没过)
  ├─ Verify (节点 4) + Decision (节点 5) 独立 stage
  │   ├─ n_c Decision=retry (AttemptNo=0 < MaxRetry=1)
  │   ├─ n_c AttemptNo++ → RoundMeta.AttemptNo=1
  │   ├─ n_c 重跑 → 1/1 Pass(LLM judge 通过)
  │   └─ n_c 重试后 Decision=accept
  ├─ parent rollup 触发 D parent_rollup → 聚合 3 child finalText
  ├─ emit final EngineEvent(主流程结束)   ← 用户先看到
  └─ [Async Learn] 不阻塞,后台跑
       ├─ Learn(n_a): VerdictPass + Decision=accept → α++ → child row n_a: α=6
       ├─ Learn(n_b): VerdictPass + Decision=accept → α++ → child row n_b: α=6
       ├─ Learn(n_c): VerdictFail(1 次 retry 后 Pass) + Decision=accept → α++ → child row n_c: α=6
       ├─ Learn(parent): ParentEvidence aggregator
       │   └─ sum: parent row α=18 (=6+6+6), β=3 (不变,retry 不累计)
       └─ force_plan 触发?β/(α+β) = 3/21 ≈ 0.14 < 0.7 → no
```

### 8.7.7 Learn 节点边界 case 降级

| Case | 降级策略 |
|------|----------|
| Learn 失败(reputation DB 写挂) | **silent + 内部 retry 3 次 + 最终 slog.Warn("learn_failed")** + **不阻塞 emit final**(3 选 1 推荐策略:L1 silent+重试) |
| artifact_metadata_hash 重复 | no_change(跳过,不计 α/β) |
| ParentEvidence sum 时 child row 缺失 | 降级只更新存在的 child + log warning |
| force_plan 触发 | 写 next_observation_force_plan=true 到 segment_id metadata,下次 Observe 降级 Plan 路径 |
| 队列满(AsyncLearner chan 100 满) | **降级同步执行(Sync.Learn)** + log warning(3 选 1 推荐策略:L2 sync fallback) |
| session 未 Drain 时退出 | **DeferToNextSession marker** + 下次 session.Drain() 兜底(3 选 1 推荐策略:L3 defer) |
| 同一 parent 多次 rollup | 累加(不重置,长期累积) |
| retry decision | 不累计(避免 noise) |
| human_review decision | β++(强 negative,叠加在 Verdict 上) |
| Plan 阶段错误(超时/失败/ctx 取消早) | **不 Learn**(无 segment row 可累加,仅 audit log) |
| Execute/Verify 阶段错误 | **正常 Learn**(β++,VerdictFail 信号) |
| user-cancel | **Learn β++**(用户拒绝 = 强 negative signal) |
| user-accept | **Learn α++**(用户认可 = 强 positive signal) |
| user-modify | 本轮不 Learn(下轮 directive 重新走完整 6 节点) |

### 8.7.8 Learn 节点 vs 上下游契约

```
↑ 上游:Verify(节点 4)+ Decision(节点 5)两个独立 stage
   - 输入:PerCriterionVerdict[] + VerdictKind + Decision{Kind, Reason}
   - 产出:LearnRequest(自动构造)

↓ 下游:Reputation DB(SQLite)
   - 表:reputation(id, segment_id, parent_id, alpha, beta, last_updated, decision_kind_history, source_id_history, artifact_metadata_hash, metadata, deprecated_plan_rationale) = 11 字段(2026-07-07 锁定)
   - 操作:SELECT / INSERT / UPDATE(同一 segment_id 多次 UPDATE 累加)
   - cold start:0 行 → BuildAdaptivePrior Beta(5, 3)

← 跨 session 累积:同一 segment_id 跨 session 复用 reputation row
   - cold start 仅首次
   - 后续直接 SELECT 旧 row 继续累加
```

### 8.7.9 Learn 节点与其它 spec 章节对照

| 维度 | spec_delta.md | design.md | proposal.md | decision-tree.md |
|------|---------------|-----------|-------------|------------------|
| LearnRequest 类型 | §3.9 | §2.14 | §5.6 | §8.7.2 |
| LearnResponse 类型 | §3.10 | §2.14 | §5.6 | §8.7.2 |
| ReputationRow 类型 | §3.11 | §2.14 | §5.6 | §8.7.4 |
| LearnNodeIO | §14 | §2.14 + §2.15(async) | §5.6(3 步) | §8.7.1 + §8.7.3(公式) + §8.7.6(trace) |
| BayesianUpdate 公式 | §14 formula | §2.14 | §5.6 Step 2 | §8.7.3 |
| ParentEvidence aggregator | §14 aggregator | §2.14 | §5.6 | §8.7.6(完整 trace)+ §8.7.8(契约) |
| force_plan 触发 | §14 force_plan | §2.14 | §5.6 | §8.7.3(公式) |
| async 不阻塞 | §14 async | §2.15 | §5.6 | §8.7.1(链路图)+ §8.7.6(trace)|
| 5 场景分布 | — | — | §5.6 | §8.7.5(完整表) |
| 边界 case | §14 errors | §2.14 | §5.6 | §8.7.7(13 行降级) |
| **22 场景覆盖** | **§3.12** | **§2.16(NEW)** | **§5.7(NEW)** | **§8.7.10(NEW 完整 22 行表)** |

### 8.7.10 Learn 节点 22 场景全覆盖(Observe + 5 维度扩展)

**问题**:之前 §8.7.5 仅覆盖 5 场景 S-A~S-E,没有覆盖 Observe 节点所有用例下 Learn 的触发/累计/降级行为。本节按 6 维度扩展至 **22 场景**,确保 Learn 节点在 D7 全链路(Observe→Plan→Execute→Verify→Decision→Learn)的每个分支都有明确定义。

**6 维度分类**:

```
维度 1 (基础 5 场景):S-A / S-B / S-C / S-D / S-E
维度 2 (上游错误 E1-E6):Plan 超时 / Plan 失败 / Worker panic / ctx cancel 早 / ctx cancel 晚 / LLM judge 超时 / Decision map miss
维度 3 (用户主动 U1-U3):user-cancel / user-accept / user-modify
维度 4 (force_plan 链路 F1-F2):force_plan 首次触发 / force_plan 下次降级
维度 5 (Learn 自身失败 L1-L3):DB 写挂 / AsyncLearner 队列满 / session 未 Drain
维度 6 (跨 session + 特殊语义 C1 + X1):segment_id 跨 session 复用 / emit 失败 + Learn 已 enqueue
```

**3 个推荐决策**(用户 2026-07-07 拍板):

| 决策点 | 推荐策略 | 原因 |
|--------|---------|------|
| **上游错误 Learn 触发** | Plan 错误不 Learn / Execute 错误 Learn | Plan 阶段无 segment row 可累加,仅 audit log;Execute 阶段 segment 已存在,正常 β++ 累计 |
| **用户主动行为 Learn** | user-cancel/user-accept 都记入 reputation | user-cancel=强负信号(β++)、user-accept=强正信号(α++);user-modify 不影响本轮 Learn |
| **Learn 自身失败** | silent + 内部 retry 3 次 + 最终 warn | 不阻塞 emit final;L2 降级同步、L3 defer next session |

**完整 22 场景行为表**:

| # | 维度 | 场景 | 触发条件 | Decision (节点 5) | Verdict (节点 4) | Learn 行为 | BayesianAction |
|---|------|------|----------|------------------|------------------|-----------|----------------|
| 1 | 基础 | **S-A** 单确定性 fast-path | `len(Segments)==1 && all deterministic && has high-strength fact` | (no Decision stage) | Pass | enqueue 1 次,SourceID=`obs_fact:seg_a_id` | alpha_bump |
| 2 | 基础 | **S-B** 单不确定性 | normal full path | accept | Pass | enqueue 1 次,SourceID=`verify:seg_a_id` | alpha_bump |
| 3 | 基础 | **S-C** 多确定性 fast-path | `len(Segments)>=2 && all deterministic && all high-strength` | (no Decision stage) | Pass × N | enqueue N 次(per-segment) | alpha_bump × N |
| 4 | 基础 | **S-D** 多不确定性 | multi-intent DAG | accept × N + parent_rollup | Pass × N | enqueue N + parent aggregator (sum) | α_bump × N + parent sum (no_change) |
| 5 | 基础 | **S-E** 混合 | 部分 deterministic 部分 uncertain | accept × 2 + parent_rollup | Pass × 2 | enqueue 2(seg_a fast-path + seg_b verified)+ parent | α_bump × 2 + parent sum |
| 6 | E上游 | **E1** Plan 超时 | `PlanLLMCallTimeout > 30s` | human_review | (no AC) | **NO Learn**(无 segment row)+ audit log | no_change + audit |
| 7 | E上游 | **E2** Plan 失败 | `PlanLLMCallError`(LLM 5xx) | human_review | (no AC) | **NO Learn** + audit log | no_change + audit |
| 8 | E上游 | **E3** Worker panic | `WorkerError`(execute 阶段 panic) | retry (AttemptNo=0<MaxRetry=1) | Fail | enqueue 1 次,VerdictFail → β++ | beta_bump |
| 9 | E上游 | **E4-pre** ctx cancel 早 | ctx.Done() before Plan emit | human_review | (no AC) | **NO Learn**(无 segment)+ audit log | no_change + audit |
| 10 | E上游 | **E4-post** ctx cancel 晚 | ctx.Done() after Verify emit | accept(last good) | Pass(last good) | enqueue 1 次,SourceID=`verify:seg_a_id_last_good` | alpha_bump |
| 11 | E上游 | **E5** LLM judge 超时 | `LLMJudgeTimeout > 15s` | human_review | SystemAnomaly | enqueue 1 次,VerdictKind=SystemAnomaly → β++ | beta_bump |
| 12 | E上游 | **E6** Decision map miss | map miss → default fallback | human_review | SystemAnomaly | enqueue 1 次,β++ | beta_bump |
| 13 | U用户 | **U1** user-cancel | feishu.abort()(用户点飞书取消) | (no Decision) | (no Verdict) | 24h 内同 segment_id 累计 ≥ 3 次 → audit_hold(只 audit 不 β++);否则 enqueue 1 次,SourceID=`user_cancel:seg_a_id` → β++ | beta_bump |
| 14 | U用户 | **U2** user-accept | feishu.accept()(用户点飞书确认) | accept | Pass | enqueue 1 次,SourceID=`user_accept:seg_a_id` → α++(fast-track) | alpha_bump |
| 15 | U用户 | **U3** user-modify | feishu.modify()(用户编辑后续发) | (新 directive) | (新) | **本轮不 Learn**,下轮 directive 重新走完整 6 节点 | no_change |
| 16 | Fforce | **F1** force_plan 触发 | `β/(α+β) > 0.7` (reputation 触发降级) | accept(last) | Pass(last) | enqueue 1 次 + 写 `metadata.next_observation_force_plan=true` | force_plan |
| 17 | Fforce | **F2** force_plan 下次降级 | 下次 Observe 读 metadata | (走 Plan 路径,不再 fast-path) | Pass | enqueue 1 次,α++(Plan 路径信号更强) | alpha_bump |
| 18 | L失败 | **L1** DB 写挂 | sqlite3.BusyError / SQLITE_LOCKED | (不变) | (不变) | silent + **内部 retry 3 次** + 最终 slog.Warn("learn_failed") | no_change + warn |
| 19 | L失败 | **L2** AsyncLearner 队列满 | chan size=100 满,enqueue blocked | (不变) | (不变) | **降级 Sync.Learn**(同步执行)+ log warn | (按正常规则) |
| 20 | L失败 | **L3** session 未 Drain | ctx timeout before session.Drain() | (不变) | (不变) | **DeferToNextSession marker**,下次 session.Drain() 兜底 | (按正常规则) |
| 21 | C跨 | **C1** segment_id 跨 session 复用 | 同 segment_id 第二次出现 | accept(last) | Pass | cold start → BuildAdaptivePrior Beta(5,3) → α=5+1=6,β=3 | alpha_bump |
| 22 | X特殊 | **X1** emit 失败 + Learn 已 enqueue | feishu.SendError on final emit | accept(last) | Pass(last) | Learn 已 enqueue,独立 metric `learn_emitted_after_emit_failure=true`,reputation 正常 α++ | alpha_bump |

**场景分布统计**:

| 维度 | 场景数 | Learn 触发 | α_bump | β_bump | no_change | force_plan |
|------|--------|-----------|--------|--------|-----------|------------|
| 基础 (5) | 5 | 5 | 5 | 0 | 0 | 0 |
| 上游错误 (7) | 7 | 4 (E3/E4-post/E5/E6) | 2 (E4-post) | 2 (E3/E5/E6) | 3 (E1/E2/E4-pre)+ audit | 0 |
| 用户主动 (3) | 3 | 2 (U1/U2) | 1 (U2) | 1 (U1) | 1 (U3) | 0 |
| force_plan (2) | 2 | 2 | 2 | 0 | 0 | 1 (F1) |
| Learn 失败 (3) | 3 | 0 (走降级) | 0 | 0 | 3 (L1/L2/L3 warn) | 0 |
| 跨+特殊 (2) | 2 | 2 | 2 | 0 | 0 | 0 |
| **总计** | **22** | **15** | **12** | **3** | **7** | **1** |

**关键覆盖说明**:

1. **Plan 阶段错误不 Learn**(E1/E2/E4-pre):Plan 失败时尚无 segment_id,reputation 无 row 可累加;仅 audit log + audit metric `plan_error_no_learn++`
2. **Execute/Verify 错误正常 Learn**(E3/E5/E6):segment 已存在,β++ 记录负信号
3. **用户主动入 reputation**(U1/U2):user-cancel=强负、user-accept=强正,SourceID 加前缀区分(`user_cancel:` / `user_accept:`)
4. **user-modify 不影响本轮**(U3):新 directive 走完整 6 节点
5. **Learn 失败三级降级**(L1/L2/L3):silent retry → sync fallback → defer next session,均不阻塞 emit final
6. **force_plan 是 Learn 输出而非触发**(F1):Learn 算出 β/(α+β) > 0.7 时,写 metadata 标记下次 Observe 降级,而不是 Learn 自己降级
7. **emit 失败独立于 Learn**(X1):Learn 已 enqueue 后,emit 失败不影响 reputation 落库,只是 metric 区分
8. **跨 session 复用**(C1):segment_id 是稳定 key,同一 segment_id 跨 session 复用 reputation row,冷启动仅首次

**验收映射**:本表 22 行 = spec_delta.md §3.12 AC43-AC54 的 12 条新 AC(每条 AC 覆盖 1-2 个场景)。
| AC35-AC42 | 验收标准 | — | — | §8.7 全覆盖 |
| tasks T53-T59 | — | §2.14 + §2.15 | — | — |
| **AC55-AC66** | **Plan 26 场景验收** | **§3.13** | **§2.17(NEW)** | **§5.7.1(NEW)** | **§8.8(NEW 完整 26 行表)** |
| **T66-T72** | **Plan 字段验证 + ParseReject + force_plan** | — | **§2.17** | **—** | **§8.8** |

---

## 8.8 Plan 节点 26 用例场景 → Execute / Decision / Learn 行为全覆盖

**问题**:之前 §8.5 (Plan LLM 输出 + Verify 处理) + §8.6 (Decision 5 路径) + §8.7 (Learn 22 场景) 仅覆盖 happy path 5 场景,**Plan 节点的 21 个边缘场景**(Plan LLM 错误 / 字段异常 / Parse Reject / fast-path 命中 / force_plan 链路)下 Execute / Decision / Learn 的具体行为未明示。本节按 6 维度扩展至 **26 场景**,确保 Plan → Execute → Decision → Learn 全链路在 Plan 节点每个输出状态下都有明确定义。

**6 维度分类**:

```
维度 1 (基础 5 场景 P22-P26):S-A / S-B / S-C / S-D / S-E
维度 2 (Plan LLM 调用错误 P1-P3):PlanLLMCallTimeout / PlanLLMCallError(5xx) / PlanLLMCallPartialResponse
维度 3 (Plan LLM 字段异常 P4-P11):空 Children / 空 DAG / DAG 0 nodes / AC CheckKind 越界 / AC Required=0 / rationale 缺失 / priorities 越界 / segments ID 缺失
维度 4 (Plan Parse Reject P12-P17):RejectCycle / RejectTooManyNodes / RejectDuplicateNode / RejectDanglingEdge / RejectACDuplicateID / RejectNodeCoverageMissing
维度 5 (Plan fast-path 命中 P18-P19):单确定性 fast-path 跳过 Plan / 多确定性 fast-path 跳过 Plan
维度 6 (Plan force_plan 链路 P20-P21):下次 directive 走 Plan 路径(读 metadata)/ Plan 强制 Required AC[]
```

**4 个推荐决策**(用户 2026-07-07 拍板,详见 `proposal.md §5.7.1`):

| 决策点 | 推荐策略 | 理由 |
|--------|---------|------|
| **Plan LLM 错误(P1-P3)处理** | ItemPipelineRunner 直接 emit abort + Decision 新增 plan_error 路径 | Plan error 无 segment row,与 Learn §5.6.1 决策一致(Plan 错误不 Learn) |
| **Parse Reject 重试与降级(P4-P17)** | 重试 Plan LLM ≤ 2 次 + 降级旧 SpawnPolicy.DecomposeIntoChildren | 重试给 LLM 修正机会,降级保 happy path 完整性 |
| **force_plan Plan 差异(P20-P21)** | 强制 Required AC[] ≥ 1 + PriorityHint 注入 prompt | 防止下次又走 fast-path,Plan 路径保持一致 |
| **S-E Decision 边界(P-S-E)** | fast-path 部分跳过 Decision,parent rollup 统一决策 A accept | 保持 fast-path 优势,parent rollup 提供决策完整性 |

**完整 26 场景行为表**:

| # | 维度 | Plan 输出 | Execute 行为 (节点 3) | Decision 行为 (节点 5) | Learn 行为 (节点 6) |
|---|------|----------|---------------------|---------------------|---------------------|
| 1 | 基础 | **P-S-A** 单确定性 fast-path | NO Execute (ItemPipelineRunner 跳过) | NO Decision | Learn α++ (SourceID=`obs_fact:seg_a_id`) |
| 2 | 基础 | **P-S-B** 单不确定性 | 单 Worker 串行 Execute | A accept (default 11 行映射) | Learn α++ (SourceID=`verify:seg_a_id`) |
| 3 | 基础 | **P-S-C** 多确定性 fast-path | NO Execute | NO Decision | Learn α++ × N (per-segment) |
| 4 | 基础 | **P-S-D** 多不确定性 | RunPlanDAG 并行 4 worker | A accept × N + D parent_rollup | Learn α_bump × N + parent sum |
| 5 | 基础 | **P-S-E** 混合 (fast-path + verified) | fast-path 部分跳过,verified 部分 RunPlanDAG | fast-path 部分跳过,parent rollup 统一 A accept (决策 4) | Learn × 2 (seg_a fast-path + seg_b verified) + parent sum |
| 6 | Plan 错误 | **P1** PlanLLMCallTimeout (>30s) | NO Execute | Decision 新增 path:**plan_error → E human_review** + emit abort (决策 1) | NO Learn + audit `plan_error_no_learn++` |
| 7 | Plan 错误 | **P2** PlanLLMCallError (LLM 5xx) | NO Execute | 同 P1 (plan_error → E human_review) | NO Learn + audit |
| 8 | Plan 错误 | **P3** PlanLLMCallPartialResponse (JSON 截断) | NO Execute | 同 P1 (plan_error → E human_review) | NO Learn + audit |
| 9 | 字段异常 | **P4** 空 Children (LLM 返回空数组) | NO Execute + emit abort (Plan DAG 为空无意义) | NO Decision (Plan 节点终止) | NO Learn + audit `plan_empty_children_no_learn++` |
| 10 | 字段异常 | **P5** Children + 空 DAG | **降级顺序串行 Execute** (旧 SpawnPolicy.DecomposeIntoChildren 路径,无 AC,Verify 用 fallback NumericRange) | 走简化 Decision A accept (无 AC,Verdict 用 fallback) | Learn α++ (SourceID=`plan_legacy:seg_a_id`) |
| 11 | 字段异常 | **P6** DAG 0 nodes (DAG={nodes:[]}) | NO Execute + emit abort (Plan DAG 为空) | NO Decision | NO Learn + audit |
| 12 | 字段异常 | **P7** AC[] CheckKind 越界 (枚举不存在) | ParseReject → 重试 Plan LLM ≤ 2 次 (决策 2),仍失败降级 P5 | 同 P5 (若降级) | 同 P5 (若降级) |
| 13 | 字段异常 | **P8** AC[] 全 Required=0 | ParseReject → 重试 ≤ 2 次 + 降级 P5 | 同 P5 (若降级) | 同 P5 (若降级) |
| 14 | 字段异常 | **P9** rationale 缺失 (可选字段) | 正常 Execute (rationale 是 Learn metadata,不影响 Execute) | 正常 Decision | Learn no_change (metadata 缺 rationale 字段,audit log 警告) |
| 15 | 字段异常 | **P10** priorities 越界 (>100 或 <0) | ParseReject → 重试 ≤ 2 次 + 降级 P5 | 同 P5 (若降级) | 同 P5 (若降级) |
| 16 | 字段异常 | **P11** segments ID 缺失/重复 | ParseReject → 重试 ≤ 2 次 + 降级 P5 | 同 P5 (若降级) | 同 P5 (若降级) |
| 17 | Parse Reject | **P12** RejectCycle (DAG 含环) | ParseReject → 重试 Plan LLM ≤ 2 次 + 降级 P5 | 同 P5 (若降级) | 同 P5 (若降级) |
| 18 | Parse Reject | **P13** RejectTooManyNodes (>MaxFanOut=8) | 同 P12 | 同 P5 (若降级) | 同 P5 (若降级) |
| 19 | Parse Reject | **P14** RejectDuplicateNode (节点 ID 重复) | 同 P12 | 同 P5 (若降级) | 同 P5 (若降级) |
| 20 | Parse Reject | **P15** RejectDanglingEdge (edge 端点不存在) | 同 P12 | 同 P5 (若降级) | 同 P5 (若降级) |
| 21 | Parse Reject | **P16** RejectACDuplicateID (AC ID 冲突) | 同 P12 | 同 P5 (若降级) | 同 P5 (若降级) |
| 22 | Parse Reject | **P17** RejectNodeCoverageMissing (节点无 AC) | 同 P12 | 同 P5 (若降级) | 同 P5 (若降级) |
| 23 | fast-path | **P18** 单确定性 fast-path 命中 | NO Execute (Plan 节点直接 short-circuit) | NO Decision | Learn α++ (SourceID=`obs_fact:seg_a_id`) |
| 24 | fast-path | **P19** 多确定性 fast-path 命中 | NO Execute | NO Decision | Learn α++ × N (per-segment) |
| 25 | force_plan | **P20** 下次 directive 走 Plan 路径 | 正常 Execute + **Plan prompt 注入 "强制 Required AC[] ≥ 1" + PriorityHint**(决策 3) | 正常 Decision (与默认 Plan 一致) | Learn α++ (Plan 路径信号更强) |
| 26 | force_plan | **P21** force_plan 触发后 Plan 强制 Required AC[] | 正常 Execute (与 P20 同) | 正常 Decision | Learn α++ |

**场景分布统计**:

| 维度 | 场景数 | Execute 触发 | Decision 触发 | Learn 触发 |
|------|--------|------------|-------------|------------|
| 基础 5 场景 (P22-P26) | 5 | 4 (S-B/S-D/S-E + S-A/S-C fast-path skip) | 3 (S-B/S-D/S-E) | 5 (全 5 场景) |
| Plan LLM 错误 (P1-P3) | 3 | 0 (NO Execute) | 3 (plan_error → E human_review 新路径) | 0 (NO Learn) |
| 字段异常 (P4-P11) | 8 | 4 (P5/P9 + 6 ParseReject 降级 P5) | 4 (同 Execute) | 4 (P5/P9 + 6 ParseReject 降级 P5) |
| Parse Reject (P12-P17) | 6 | 0 (走 ParseReject 重试,降级路径同 P5) | 0 (走 ParseReject 重试) | 0 (走 ParseReject 重试) |
| fast-path 命中 (P18-P19) | 2 | 0 (NO Execute) | 0 (NO Decision) | 2 (Learn α++ per-segment) |
| force_plan (P20-P21) | 2 | 2 (正常 Execute) | 2 (正常 Decision) | 2 (Learn α++) |
| **总计** | **26** | **10** | **12** | **13** |

**关键覆盖说明**:

1. **Plan LLM 错误新增 Decision 路径**(P1-P3):Decision 10 行 Verdict-based 映射表扩展为 **11 行**,新增第 11 行 `plan_error` → E human_review,与现有 10 行 Verdict-based 决策正交(Plan 错误时无 Verdict,直接走 plan_error 路径)
2. **Parse Reject 重试 ≤ 2 次 + 降级旧 Plan**(P4-P17):重试上限 2 次同 §9 feedback_loop 3 子类(避免 LLM 循环);降级到旧 SpawnPolicy.DecomposeIntoChildren 是 v1 兼容路径,无 AC 时 Verify 用 fallback NumericRange
3. **rationale 缺失可选**(P9):rationale 是 Learn metadata 字段,Execute/Decision 行为不变,Learn metadata 缺字段 + audit log 警告(不影响 α/β 累计)
4. **fast-path 命中 Plan 跳过**(P18-P19):Plan 节点直接 short-circuit 到 ItemPipelineRunner.Run,跳过 Execute / Decision 节点
5. **force_plan Plan 路径差异**(P20-P21):仅 Plan prompt 注入强制 Required AC[] 提示,Execute / Decision / Learn 行为与默认 Plan 一致
6. **S-E Decision 边界**(P-S-E 决策 4):fast-path 部分跳过 Decision 节点,parent rollup 阶段对所有 child(fast-path + verified)统一走 A accept,S-E 整体仅 1 个 decision.kind(accept)

**与 Learn 22 场景(§8.7.10)的关系**:
- Plan 节点 26 场景是上游输入
- Learn 节点 22 场景是下游输出
- 两者交集:基础 5 场景 + Parse Reject 降级后 + force_plan 路径 = 13 场景触发 Learn,与 §8.7.10 统计一致

**验收映射**:本表 26 行 = spec_delta.md §3.13 AC55-AC66 的 12 条新 AC(每条 AC 覆盖 1-3 个场景)+ §3.14 1 条 AC 覆盖 Decision 11 行映射表扩展(10 baseline + 1 plan_error)。

---

## 9. 总结:观察节点出口

**核心三句**:

1. **Observe 节点只决定"是否进 Plan"和"如何分 segment",不决定"如何执行"** —— 执行决策在 Plan 节点的 PlanDAG。
2. **Observe fast-path 仅在 segment 全为 deterministic 且无 LLM Uncertainty 时直接 return** —— S-A、S-C。
3. **混合场景(S-E)的快答不靠 Observe fast-path,靠 Execute 节点 RunPlanDAG 内部对部分 segment 走 fast-path** —— 这是用户最初想要的"立刻给能给的,稍等给完整的"体验。

---

## 10. 与其它文档的引用关系

| 维度 | 文档位置 |
|---|---|
| 概念 / 类型 | `proposal.md §2 Problem Statement` + `spec_delta.md §1-3` |
| 任务分解 | `tasks.md` (47 T 点跨 13 模块) |
| 系统图 + 数据流 + 边界 | `design.md §1` |
| 数据结构 / 函数签名 / 错误处理 | `design.md §2`(2.1-2.15) |
| 验收标准 | `spec_delta.md Acceptance Criteria` (AC1-AC42) |
| T 点编号 | `spec_delta.md` 末尾 T-Registry Delta |
| 风险 / Rollout / Feature flag | `proposal.md §6 §7 §5` |
| **Plan LLM 输出 + Verify 节点处理** | `decision-tree.md §8.5`(集中表)+ §5.1/§6.1(具体 PlanLLMOutput) |
| **AcceptanceCriteria 契约 + LLMIO 协议** | `spec_delta.md §3.5-3.6` + §9-12 + `proposal.md §5.2-5.4` + `design.md §2.8-2.11` |
| **Decision Node 5 路径决策** | `proposal.md §5.5` + `spec_delta.md §3.7-3.8` + `§13` + `design.md §2.12-2.13` + `decision-tree.md §8.6` |
| **Learn Node 节点(异步 + per-segment 升格)** | `proposal.md §5.6` + `spec_delta.md §3.9-3.11` + `§14` + `design.md §2.14-2.15` + `decision-tree.md §8.7` |
| **Open Questions / V2 Backlog** | `proposal.md §8.1-8.2` |

---

## 11. 与上游 Change 的关系

- **DM-20260706-011**(observational_answer fast-path):本 Change 的语义前置,提供 S-A / S-C 的 fast-path 实现基线
- **devrix-d7-mups-v4-phase6-observe-learner-wiring**:IntentClassifier 接口扩展,本 Change 扩展 IntentClassifier 接收 IntentSegmentSet
- **devrix-d7-mups-v4-phase4-verify-promotion**:SpawnPolicy.DecomposeIntoChildren 是本 Change 的 DecomposeByIntentSegments 父类
- **devrix-d7-taskcontract-unification**:TaskReport 五元素扩展预留位,本 Change 不会冲突但留 v2 接入点
