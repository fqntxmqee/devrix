# Spec Delta: D7 multi-intent observation decompose + Plan DAG + parallel Execute

**Change ID:** `devrix-d7-multi-intent-observation-decompose`
**Affected spec:** `openspec/specs/d7-orchestration/spec.md`

---

## 章节导航索引(2026-07-07 S3-Gate review fix)

**说明**:本 spec_delta 因多次增量更新,章节编号呈现**插入式分布**(§3.5/§3.6/§3.7/§3.8/§3.9/§3.10/§3.11/§3.12/§3.13/§3.14 散落各处)。为方便阅读,以下索引表给出**逻辑顺序**与**物理位置**映射。

| 逻辑章节 | 物理位置 | 内容 | 物理位置行号 |
|---------|---------|------|-------------|
| §1 | §1 | IntentSegment (new type) | line 8 |
| §2 | §2 | IntentSegmentSet (new type) | line 43 |
| §3 | §3 | PlanDAG (new type) | line 67 |
| §4 | §4 | SpawnPolicy (updated) | line 198 |
| §5 | §5 | Feature Flag d7.dag_executor.enabled (new) | line 214 |
| §6 | §6 | WaveScheduler RunPlanDAG (new behavior) | line 227 |
| §7 | §7 | Stream Emit Idempotency (new behavior) | line 247 |
| §8 | §3.5 | **AcceptanceCriterion** (new type, Plan↔Verify 契约) | line 103 |
| §9 | §3.6 | **PerCriterionVerdict** (new type, Verify 产出) | line 163 |
| §10 | §11 | PlanNode (updated, AddCriteria 字段) | line 584 |
| §11 | §12 | PlanAcceptanceContractBuilder (new helper) | line 598 |
| §12 | §3.7 | **Decision** (new type, 5 路径枚举) | line 616 |
| §13 | §3.8 | **ChildWorkItemSpec** (new type, sub-task spec) | line 653 |
| §14 | §3.9 | **LearnRequest** (new type, 精简契约) | line 779 |
| §15 | §3.10 | **LearnResponse** (new type) | line 819 |
| §16 | §3.11 | **ReputationRow** (new type, DB schema) | line 851 |
| §17 | §9 | PlanLLMIO (new IO contract) | line 464 |
| §18 | §10 | VerifyLLMIO (new IO contract) | line 526 |
| §19 | §13 | DecisionNodeIO (new IO contract) | line 689 |
| §20 | §14 | LearnNodeIO (new IO contract) | line 912 |
| §21 | §3.12 | **Learn 22-scenario coverage** (new scenarios section) | line 342 |
| §22 | §3.13 | **Plan 26-scenario coverage** (new scenarios section) | line 388 |
| §23 | §3.14 | **Decision 11-row mapping extension** (new scenarios section) | line 435 |

**Acceptance Criteria** 章节(原 line 270)已**移到末尾**(见 "Acceptance Criteria (S5 验收)" at the end before Affected Files),保持 §1-§23 主体连续。

**重构计划**:S4 实现阶段将本 spec_delta 合并到主 `openspec/specs/d7-orchestration/spec.md` (lite-mode) + `CHANGELOG.md`,消除章节散落问题。当前为保持 git diff 可追溯,暂保留插入式编号。

---

## 1. New Type: IntentSegment

```yaml
name: IntentSegment
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Observe 节点产出。一个 directive 可切分为 N 个 IntentSegment,每个 segment
  独立成为 1 个 child WorkItem。Text 是 directive 子串,IntentKind 决定后续
  Plan/Execute 路径,Priority 决定 WaveScheduler ready queue 顺序。
schema:
  fields:
    - name: ID
      type: string (uuid)
      required: true
    - name: Text
      type: string
      required: true
      description: directive 的子串,通常 1 句话
    - name: IntentKind
      type: enum {deterministic, explore, commit, analyze}
      required: true
    - name: Priority
      type: int [0, 100]
      required: false
      default: 50
    - name: Confidence
      type: float [0, 1]
      required: false
      default: 0.5
  errors:
    - ErrIntentSegmentInvalid (kind not in enum / priority out of range)
```

## 2. New Type: IntentSegmentSet

```yaml
name: IntentSegmentSet
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Observe 节点的切分结果容器。SourceDirective 保留原始 directive 用于
  audit,Segments 是 N 个 IntentSegment。
schema:
  fields:
    - name: Segments
      type: []IntentSegment
      required: true
    - name: SourceDirective
      type: string
      required: true
    - name: DetectedAt
      type: time.Time
      required: true
  invariant: len(Segments) >= 1
```

## 3. New Type: PlanDAG

```yaml
name: PlanDAG
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Plan 节点产出。Nodes 是 N 个 PlanNode(每个对应 1 个 IntentSegment → 1 个 child
  WorkItem),Edges 保留依赖关系字段(v1 不解析),Priorities 控制 WaveScheduler
  ready queue。
schema:
  fields:
    - name: Nodes
      type: []PlanNode
      required: true
      constraint: <= MaxFanOut (default 8)
      note: 每个 PlanNode 携带自己的 AcceptanceCriteria(见 §3.5),用于驱动 Verify 节点
    - name: Edges
      type: []DataEdge
      required: false
      note: v1 字段保留,DataEdge.DependsOnOutputs 留空
    - name: Priorities
      type: map[string]int
      required: false
      constraint: key 必须是 Nodes 中某个 ID
    - name: MaxParallelism
      type: int
      required: false
      note: v1 ignored;WaveScheduler 硬上限 4
  invariant:
    - 无环(DFS 检测)
    - Nodes ID 唯一
    - Edges 端点必须存在于 Nodes
```

## 3.5 New Type: AcceptanceCriterion(DM-20260707-001 P0)

```yaml
name: AcceptanceCriterion
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Plan 节点生成,Verify 节点消费。
  每个 PlanNode 携带一组 AC,声明"这个节点产出的 artifact 应该满足什么条件"。
  Verify 节点拿 Artifact + AC[] 逐条校验,产出 PerCriterionVerdict[]。

  这是 Plan ↔ Verify 的契约桥梁:**让 Verify 不用猜该验什么**。
schema:
  fields:
    - name: ID
      type: string
      format: "AC-{node_id}.{n}"  # AC-n_a.1, AC-n_a.2, AC-n_b.1
      required: true
    - name: Description
      type: string
      required: true
    - name: CheckKind
      type: enum
      values:
        - ContainsString      # artifact 包含指定字符串
        - NotContainsString   # artifact 不包含指定字符串
        - MentionsAll         # artifact 提到 N 个 token 全部
        - MentionsAny         # artifact 提到 N 个 token 中任一
        - NumericRange        # 数值在 [min, max]
        - LengthRange         # 文本长度在 [min, max]
        - JSONPath            # JSONPath 表达式提取值
        - CustomLLMJudge      # 委托 LLM judge(带 judge_prompt)
      required: true
    - name: CheckArgs
      type: map[string]any
      required: false
      description: |
        per CheckKind 的具体参数:
        - ContainsString: {target: "D7", case_insensitive: true}
        - NumericRange: {min: 0.5, max: 1.0}
        - MentionsAll: {tokens: ["D1","D2",...,"D7"]}
        - JSONPath: {path: "$.layers[*].name"}
    - name: Severity
      type: enum {Required, Preferred}
      required: true
      description: |
        Required = 校验失败 → VerdictFail(blocker)
        Preferred = 不通过 → 仍可 VerdictPass(只记录到 evidence)
    - name: Rationale
      type: string
      required: false
      description: |
        Plan LLM 解释"为什么这个 AC 重要"。Verify LLM judge 用此
        理解意图,避免机械通过/拒绝。
errors:
  - ErrAcceptanceCriterionCheckUnsupported (CheckKind 不在 enum)
  - ErrAcceptanceCriterionArgsMissing (CheckKind 要求的 CheckArgs 字段缺失)
```

## 3.6 PerCriterionVerdict (Verify 产出)

```yaml
name: PerCriterionVerdict
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Verify 节点对单个 AcceptanceCriterion 的判定结果。
  Verify 节点汇总 PerCriterionVerdict[] + Artifact Summary 产出最终 Verdict。
schema:
  fields:
    - name: CriterionID
      type: string
      required: true
    - name: Outcome
      type: enum {Pass, Fail, Skipped, Error}
      required: true
      note: |
        - Pass: 机械 CheckKind 命中 / LLM judge 通过
        - Fail: 机械 CheckKind 未命中 / LLM judge 拒绝
        - Error: CheckKind 执行异常(LLM timeout / JSON parse fail 等),
                 区别于 Fail(语义错),Err 是执行失败
        - Skipped: **当前保留未使用**(v1 不产出,留给 v2 sub-criterion
                 复用 AC subset 时跳过非目标 AC 的场景);当前 aggregation
                 规则假定 Skipped 不出现,若出现按 Error 处理(防歧义)
    - name: Evidence
      type: string
      required: false
      description: 支撑判定的证据(LLM judge 解释 / matched tokens / extracted value)
    - name: Error
      type: string
      required: false
      description: Error 状态下的具体错误
aggregation_rule: |
  - ∃ Required criterion with Outcome=Fail → VerdictKind = VerdictFail
  - 所有 Required criterion Pass 且 ∃ Preferred criterion Fail → VerdictKind = VerdictPartial
  - 全部 Pass → VerdictKind = VerdictPass
  - 任一 Error 且无 Fail → VerdictKind = VerdictIndeterminate
```

---

## 4. Updated: SpawnPolicy

```yaml
name: SpawnPolicy
delta: |
  现有 SpawnPolicy 枚举值:
    - InlineRetry
    - SpawnChild
    - SpawnEscalateHuman
    - DecomposeIntoChildren (legacy,保留)
    - DecomposeByIntentSegments (NEW,DM-20260707-001)
  
  DecomposeByIntentSegments 携带 IntentSegmentSet 字段,Plan 节点
  在 segment 数 >= 2 时倾向选择;len(Segments)==1 时退化到 InlineRetry。
```

## 5. New Feature Flag: `devrix.d7.dag_executor.enabled`

```yaml
name: d7.dag_executor.enabled
type: bool
default: false
introduced: DM-20260707-001
description: |
  Feature flag。true 时 multi-intent directive 自动切分 + 并行执行;
  false 时 fallback 到旧 Plan + 顺序 await 路径(零行为变化)。
rollout: 5% → 100% across 2 weeks
```

## 6. New Behavior: WaveScheduler RunPlanDAG

```yaml
name: RunPlanDAG
owner: D7 (wavescheduler subpackage)
introduced: DM-20260707-001
signature: |
  func (s *WaveScheduler) RunPlanDAG(ctx context.Context, dag PlanDAG) (<-chan SegmentEmit, error)
constraints:
  - 4 worker hard cap (semaphore)
  - ready queue = priority heap (desc)
  - 任一 child error → cancel 未启动 sibling + drain emit
  - context cancel 终止所有 worker
output: |
  channel close = 全部 child 完成(包括 parent rollup)
emit: |
  每个 child round → SegmentEmit{IsPartial: true}
  parent rollup round → SegmentEmit{IsFinal: true}
```

## 7. New Behavior: Stream Emit Idempotency

```yaml
name: EmitIdempotency
owner: D7 (d1 feishu integration)
introduced: DM-20260707-001
contract: |
  IdempotencyKey = (session_id, segment_id)
  重复 emit 同一 key → noop (slog.Debug)
  dedup table scope = session 生命周期
```

## 8. Updated: LearnRequest

```yaml
name: LearnRequest
delta: |
  新加 WorkItemID 字段(per-segment attribution)
  ParentEvidence aggregator:sum α/β across children → 写入 parent reputation row
```

---

## 3.12 Learn Node 22 场景覆盖(NEW,§5.6.1 + §8.7.10 对应)

**问题**:之前 AC35-AC42 覆盖 5 场景 S-A~S-E,缺失**上游错误 / 用户主动 / force_plan / Learn 失败 / 跨 session + 特殊**5 个维度的 17 场景。本节扩展 AC43-AC54 共 12 条新 AC,覆盖 22 场景的全链路 Learn 行为。

**22 场景分类与 AC 映射**:

| 维度 | 场景 | AC 覆盖 |
|------|------|--------|
| 基础 5 场景 | S-A / S-B / S-C / S-D / S-E | **AC43**(覆盖全部 5 场景) |
| 上游错误 7 场景 | E1 / E2 / E4-pre | **AC44**(Plan 错误不 Learn) |
| 上游错误 4 场景 | E3 / E5 / E6 / E4-post | **AC45**(Execute/Verify 错误正常 Learn) |
| 用户主动 1 场景 | U1 user-cancel | **AC46**(β++) |
| 用户主动 1 场景 | U2 user-accept | **AC47**(α++ fast-track) |
| 用户主动 1 场景 | U3 user-modify | **AC48**(本轮不 Learn) |
| force_plan 1 场景 | F1 首次触发 | **AC49**(β/(α+β) > 0.7 + 写 metadata) |
| force_plan 1 场景 | F2 下次降级 | **AC50**(下 directive 走 Plan 路径) |
| Learn 失败 1 场景 | L1 DB 写挂 | **AC51**(silent + retry 3 + warn) |
| Learn 失败 1 场景 | L2 队列满 | **AC52**(降级 Sync.Learn) |
| Learn 失败 1 场景 | L3 未 Drain | **AC53**(DeferToNextSession marker) |
| 跨+特殊 2 场景 | C1 + X1 | **AC54**(跨 session 复用 + emit 失败独立) |

**12 条新 AC 对应 tasks T60-T65**:

| AC | 实现 T 点 |
|----|----------|
| AC43 | T60(classifyScenario 5 基础场景)+ T65(测试) |
| AC44 | T61(Plan 错误 audit log)+ T65 |
| AC45 | T61(Execute/Verify 错误 β++)+ T65 |
| AC46 | T62(feishu_user_action.go user-cancel)+ T65 |
| AC47 | T62(user-accept α++ fast-track)+ T65 |
| AC48 | T62(user-modify 本轮不 Learn)+ T65 |
| AC49 | T63(F1 写 metadata)+ T65 |
| AC50 | T63(F2 下次降级 Plan)+ T65 |
| AC51 | T64(L1 silent retry)+ T65 |
| AC52 | T64(L2 sync fallback)+ T65 |
| AC53 | T64(L3 defer next session)+ T65 |
| AC54 | T60(跨 session cold start)+ T61(emit 失败独立 metric)+ T65 |

**用户拍板决策(2026-07-07)**:

1. **上游错误 Learn 触发**:Plan 错误不 Learn / Execute 错误 Learn
2. **用户主动行为**:user-cancel β++ / user-accept α++ / user-modify 不影响
3. **Learn 自身失败**:silent + 内部 retry + 最终 warn
4. **force_plan 链路**:F1 触发 metadata + F2 下次 Plan 路径
5. **跨 session + 特殊语义**:segment_id 跨 session 复用 + emit 失败独立 metric

## 3.13 Plan 节点 26 用例场景全覆盖(NEW,§5.6.2 + §5.7.1 + §8.8 对应)

**问题**:之前 AC1-AC54 覆盖 happy path + 22 场景 Learn,**Plan 节点 21 个边缘场景**(Plan LLM 错误 / 字段异常 / Parse Reject / fast-path 命中 / force_plan 链路)下 Execute / Decision / Learn 的具体行为未明示。本节扩展 AC55-AC66 共 12 条新 AC,覆盖 Plan 节点 26 场景的全链路行为。

**Plan 26 场景分类与 AC 映射**:

| 维度 | 场景 | AC 覆盖 |
|------|------|--------|
| 基础 5 场景 | P22-P26 (S-A~S-E) | AC1-AC42 (已覆盖) |
| Plan LLM 错误 3 场景 | P1 / P2 / P3 | **AC55** (Plan error → Decision plan_error 新路径) |
| 字段异常 8 场景 | P4 / P6 / P7 / P8 / P9 / P10 / P11 | **AC56** (PlanFieldValidator 字段级校验) |
| Parse Reject 6 场景 | P12-P17 (RejectCycle / RejectTooManyNodes / RejectDuplicateNode / RejectDanglingEdge / RejectACDuplicateID / RejectNodeCoverageMissing) | **AC57** (RetryWithFeedback ≤ 2 次 + fallback DecomposeIntoChildren) |
| fast-path 命中 2 场景 | P18 / P19 | **AC58** (Plan 跳过 + NO Execute / NO Decision) |
| force_plan 链路 2 场景 | P20 / P21 | **AC59** (force_plan Plan 注入 Required AC[] ≥ 1) |
| 降级路径 Execute/Verify/Learn | (P5 / P4-P17 降级) | **AC60** (DecomposeIntoChildren 顺序串行 + fallback NumericRange) |
| Plan rationale 缺失 1 场景 | P9 | **AC61** (rationale 可选 + Learn metadata 缺字段 + audit log) |
| Plan 空 Children / DAG 0 nodes | P4 / P6 | **AC62** (NO Execute + emit abort) |
| S-E Decision 边界 | P-S-E | **AC63** (fast-path 部分跳过 Decision + parent rollup 统一 A accept) |
| Plan LLM 错误 emit abort | P1 / P2 / P3 | **AC64** (飞书卡 "❌ Plan 阶段失败" + emit abort) |
| Plan LLM 错误 NO Learn | P1 / P2 / P3 | **AC65** (NO Learn + audit `plan_error_no_learn++`) |
| Plan retry 上限 | (P4-P17 重试) | **AC66** (RetryWithFeedback ≤ 2 次上限,避免 LLM 循环) |

**12 条新 AC 对应 tasks T66-T72**:

| AC | 实现 T 点 |
|----|----------|
| AC55 | T69 (PlanErrorDecision) + T70 (Decision 10 行映射表) |
| AC56 | T66 (PlanFieldValidator) + T67 (PlanParseReject 6 子类扩展) |
| AC57 | T67 + T68 (RetryWithFeedback ≤ 2 次) |
| AC58 | T66 (PlanFieldValidator 短路径) |
| AC59 | T71 (force_plan Plan 注入) |
| AC60 | T68 (fallback DecomposeIntoChildren) |
| AC61 | T66 (rationale 可选字段验证) |
| AC62 | T66 (空 Children / DAG 0 nodes 拒绝) |
| AC63 | T70 (Decision S-E 边界) |
| AC64 | T69 (PlanErrorDecision emit abort) |
| AC65 | T69 (PlanErrorDecision NO Learn + audit) |
| AC66 | T68 (RetryWithFeedback 重试上限) |

**用户拍板决策(2026-07-07)**:

1. **Plan LLM 错误处理(P1-P3)**:ItemPipelineRunner 直接 emit abort + Decision 新增 plan_error 路径 + NO Learn
2. **Parse Reject 重试与降级(P4-P17)**:重试 Plan LLM ≤ 2 次 + 降级旧 SpawnPolicy.DecomposeIntoChildren
3. **force_plan Plan 差异(P20-P21)**:强制 Required AC[] ≥ 1 + PriorityHint 注入 prompt
4. **S-E Decision 边界(P-S-E)**:fast-path 部分跳过 Decision,parent rollup 统一决策 A accept
5. **Plan rationale 缺失(P9)**:可选字段,Execute/Decision 不变,Learn metadata 缺字段 + audit log 警告

## 3.14 Decision 节点 11 行映射表扩展(NEW,§5.7.1 决策 1 对应)

**问题**:之前 §3.7 Decision 节点定义 5 路径枚举 + 10 行映射表(Verdict 4 态 × Other Conditions + D parent_rollup),**未覆盖 Plan LLM 错误**(P1-P3),Plan 错误时无 Verdict。本节扩展 10 行 → **11 行**,新增第 11 行 plan_error 路径。

**11 行映射表**:

| # | Verdict / 触发源 | Other Conditions | Decision | 后续动作 | 持久化 |
|---|----------------|-----------------|----------|---------|--------|
| 1 | **Pass** | (default) | **A accept** | emit final + Learn | decision.kind=accept |
| 2 | **Partial** | Tolerance=high OR ChildBudget=0 | **A accept** | emit final + Learn | decision.kind=accept |
| 3 | **Partial** | Partial AC 可独立分解 + ChildBudget>0 | **C child_worker** | spawn 子 Worker + 子 AC[] 复用 | decision.kind=child_worker + next_spec |
| 4 | **Partial** | 其它 | **A accept** | emit final + Learn | decision.kind=accept |
| 5 | **Fail** | AttemptNo < MaxRetry=1 | **B retry** | RoundMeta.AttemptNo++ 重跑 | decision.kind=retry |
| 6 | **Fail** | AttemptNo >= MaxRetry=1 | **E human_review** | 飞书卡"❓" + emit abort | decision.kind=human_review |
| 7 | **Indeterminate** | RiskLevel=high | **E human_review** | 飞书卡"❓" + emit abort | decision.kind=human_review |
| 8 | **Indeterminate** | RiskLevel=normal/low | **B retry** | 重跑 | decision.kind=retry |
| 9 | **Error(全Err)** | Network/Timeout 类 | **B retry** | 重跑(1 次) | decision.kind=retry |
| **10** | **(任意 Verdict)** | IsChildSegment + SiblingDecidedCount==SiblingTotalCount | **D parent_rollup** | 触发 parent rollup 节点 | decision.kind=parent_rollup |
| **11(NEW)** | **plan_error** | (Plan LLM 调用层失败:timeout / 5xx / partial response) | **E human_review** | 飞书卡"❌ Plan 阶段失败" + emit abort | decision.kind=plan_error |

**关键设计**:第 11 行 plan_error 与现有 10 行正交:
- 现有 10 行触发源都是 **Verdict**(节点 4 产出)
- 第 11 行触发源是 **Plan LLM 调用层错误**(节点 2 Plan 调用失败时,由 ItemPipelineRunner 调 PlanErrorDecision.Decide() 直接产出)
- 两条决策路径互不污染:Plan error 时无 Verdict,直接走 plan_error 路径;有 Verdict 时走 10 行映射表

**AC 覆盖**:AC67 = "Decision 10 行映射表扩展为 11 行,新增第 11 行 plan_error → E human_review"

**AC68 (NEW, LearnRequest contract C8 修复)**:LearnRequest.Verdict 字段必带 `IndeterminateReason`(枚举值:`""`、`"verifier_parse_failure"`、`"env_limited"`、`"interrupt"`、`"tombstone"` 等)。理由:G8-1 修复扩展已落地,`evidence.go:126` 仅对 `verifier_parse_failure` β++,`asset_builder.go:246` 读此字段路由到 PendingAsset。LearnRequest 缺此字段会**反向破坏 G8-1 修复**。锁定 2026-07-07,后续 LearnRequest 重构必须保留 IndeterminateReason。AC 映射:T25(T53 已有,补字段注释)+ T60。

---

## 9. New IO Contract: PlanLLMIO

```yaml
name: PlanLLMIO
owner: D7
introduced: DM-20260707-001
description: |
  Plan 节点 ↔ LLM 的输入输出契约。LLM 输入包含 directive + segments + 上轮 feedback;
  LLM 输出包含 PlanDAG + acceptance_criteria[],Schema 是 JSON,严格。
inputs:
  PlanLLMInput:
    fields:
      - name: directive
        type: string
        required: true
      - name: segments
        type: IntentSegmentSet
        required: true
      - name: prior_parse_reject
        type: PlanParseReject (JSON)
        required: false
        note: 上轮 PlanParseReject feedback,LLM 据此调整
    schema: plan_llm_input_v1.json  # i18n prompt appendix 强制
outputs:
  PlanLLMOutput:
    fields:
      - name: dag
        type: PlanDAG
        required: true
        schema: plan_dag_v1.json
        constraint: validateDAG(dag) MUST pass before accepted
      - name: acceptance_criteria
        type: []AcceptanceCriterion
        required: true
        constraint: |
          - 每个 node 至少 1 个 Required criterion
          - 不能 Reuse ID across nodes(每个 AC.ID 全局唯一)
      - name: rationale
        type: string
        required: false
        note: LLM 解释"DAG 为什么这样切,AC 选这些指标"
    schema: plan_llm_output_v1.json
feedback_loop:
  RejectCycle:
    signal: validateDAG 返回 ErrCycle
    feedback_to_llm: |
      "上轮 DAG 含环,以下节点形成环: {cycle_path}。请重新切 DAG,保持相同的 segment 覆盖。"
  RejectTooManyNodes:
    signal: validateDAG 返回 ErrTooManyNodes
    feedback_to_llm: |
      "上轮 DAG 节点数={n} 超过 MaxFanOut={cap}。请合并相邻 segment 或提高复用度。"
  RejectACDuplicateID:
    signal: AC[] 含重复 ID
    feedback_to_llm: |
      "上轮 acceptance_criteria 含重复 ID: {ids}。请确保每个 AC.ID 全局唯一。"
errors:
  - ErrPlanLLMOutputInvalidJSON
  - ErrPlanLLMOutputMissingDAG
  - ErrPlanLLMOutputMissingAC
  - ErrPlanLLMIOBudgetExceeded (Plan LLM 累计耗时 > 4s,放弃 retry)
```

## 10. New IO Contract: VerifyLLMIO

```yaml
name: VerifyLLMIO
owner: D7
introduced: DM-20260707-001
description: |
  Verify 节点 ↔ LLM 的输入输出契约。Verify LLM 拿 Artifact + Plan 提供的 AC[],
  对每条 AC 产出 PerCriterionVerdict,最终聚合 VerdictKind。
  Verify 节点本地规则(ContainsString/NotContains/MentionsAll/MentionsAny/Numeric/Length/JSONPath 等机械 CheckKind)LLM 不参与,只有 CustomLLMJudge
  才走 LLM 调用。
inputs:
  VerifyLLMInput:
    fields:
      - name: artifact_summary
        type: string
        required: true
        note: Worker 产出 + 后续拼接
      - name: artifact_metadata
        type: map[string]any
        required: false
      - name: criteria
        type: []AcceptanceCriterion
        required: true
        note: Plan 提供的 AC[];Verify 必须对每条 AC 产出 verdict
      - name: plan_rationale
        type: string
        required: false
        note: Plan LLM 的 rationale,辅助 Verify LLM judge 理解 AC 意图
    custom_judge_threshold: CustomLLMJudge 数量 ≤ 3(防止 LLM 调用爆炸)
outputs:
  VerifyLLMOutput:
    fields:
      - name: per_criterion_verdicts
        type: []PerCriterionVerdict
        required: true
        constraint: |
          - len(criteria) == len(per_criterion_verdicts)(顺序对齐)
          - 每个 criterion 必有 verdict(no skipped)
      - name: overall_verdict
        type: PerCriterionVerdict-derived enum {Pass, Partial, Fail, Indeterminate}
        required: true
        aggregation: 见 §3.6 PerCriterionVerdict aggregation_rule
      - name: evidence
        type: string
        required: true
        note: 支撑 overall_verdict 的一句话解释(给飞书 final card 展示)
errors:
  - ErrVerifyLLMOutputMismatchCount (verdicts.len != criteria.len)
  - ErrVerifyLLMJudgeBudgetExceeded
short_circuit:
  when: |
    0 CustomLLMJudge AND PlanLLM Criteria 全是机械 CheckKind
    (ContainsString/NotContains/MentionsAll/MentionsAny/Numeric/Length/JSONPath,见 §3.5 枚举)
    then: Verify 节点本地跑,不调 LLM,延迟 < 50ms
```

---

## 11. Updated: PlanNode (扩展 AddCriteria 字段)

```yaml
name: PlanNode
delta: |
  v1 字段:
    - ID, SegmentID, WorkerHint, ExpectedArtifactTags
  v1.1 新增(DM-20260707-001 升级):
    - AcceptanceCriteria []AcceptanceCriterion  # 见 §3.5
invariant: |
  len(AcceptanceCriteria) >= 1
  ∀ c ∈ AcceptanceCriteria: CheckArgs per CheckKind Validate 通过
```

## 12. New Helper: PlanAcceptanceContractBuilder

```yaml
name: PlanAcceptanceContractBuilder
owner: D7
introduced: DM-20260707-001
description: |
  工具方法,从 PlanLLMOutput 抽取 PlanDAG + AC[],验证它们一致:
    - ∃ NodeCoverageMissing(AC 引用了不存在的 node)
    - ∃ DuplicateCriterionID
    - ∀ Node: ≥ 1 Required criterion
  通过后才能让 Verify 节点消费。
errors:
  - ErrPlanAcceptanceContractInconsistent
```

---

## 3.7 New Type: Decision(DM-20260707-001 P0)

```yaml
name: Decision
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Decision 节点(D7 6 节点流水线独立第 5 stage)产出。5 路径决策枚举,决定"接下来做什么"。
  Verify 节点(节点 4)产出 VerdictKind 后,Decision 节点(节点 5)立即跑映射表,
  产出 Decision{Kind, Reason, NextWorkItemSpec?}。
schema:
  fields:
    - name: Kind
      type: enum
      values:
        - accept           # A 接受并 emit final + Learn
        - retry            # B 重试当前 segment(AttemptNo++)
        - child_worker     # C spawn 子 Worker 在当前 segment 内继续
        - parent_rollup    # D 触发 parent rollup 节点
        - human_review     # E 飞书卡标"❓ 需人工确认" + emit abort
      required: true
    - name: Reason
      type: string
      required: true
      description: 人类可读解释(落 reputation metadata,飞书 final card 显示)
    - name: NextWorkItemSpec
      type: *ChildWorkItemSpec
      required: false
      note: 仅 Decision.Kind == child_worker 时携带
  invariants:
    - Decision.Kind == child_worker → NextWorkItemSpec != nil
    - Decision.Kind == accept / retry / parent_rollup / human_review → NextWorkItemSpec == nil
errors:
  - ErrDecisionMapMiss (静态映射表未命中,降级 accept + log warning)
```

## 3.8 New Type: ChildWorkItemSpec(DM-20260707-001 P0)

```yaml
name: ChildWorkItemSpec
owner: D7
lifecycle: stable
introduced: DM-20260707-001
description: |
  Decision.Kind=child_worker 时携带,描述"spawn 哪个子 Worker 跑什么 AC 子集"。
  子 Worker 复用 parent 的 SessionWorkItem + WorkItemID 不同,per-segment reputation 独立。
schema:
  fields:
    - name: ParentWorkItemID
      type: string
      required: true
    - name: SubSegmentIDs
      type: []string
      required: true
      note: parent AC[] 中 Required Fail 的子集
    - name: InheritACSubset
      type: []AcceptanceCriterion
      required: true
      note: 子 Worker 跑这组 AC,Verify 沿用同一聚合规则
    - name: MaxBudget
      type: int
      required: false
      default: 2
      note: v1 硬上限 2,降级路径
  invariant: len(InheritACSubset) >= 1
errors:
  - ErrChildWorkItemBudgetExhausted
  - ErrChildWorkItemInheritACEmpty
```

---

## 13. New IO Contract: DecisionNodeIO

```yaml
name: DecisionNodeIO
owner: D7
introduced: DM-20260707-001
description: |
  Decision 节点(D7 6 节点流水线独立第 5 stage)的输入输出契约。Decision 节点纯规则引擎,
  0 LLM 调用,延迟 < 5ms。输入是 Verdict(节点 4)+ AC[] + Verdicts + RoundMeta,
  输出是 Decision{Kind, Reason, NextWorkItemSpec?}。
inputs:
  DecisionNodeInput:
    fields:
      - name: Verdict
        type: VerdictKind
        required: true
        note: 来自 §3.6 PerCriterionVerdict aggregation 4 态
      - name: ACs
        type: []AcceptanceCriterion
        required: true
        note: 原始 PlanLLMOutput 中的 AC[],用于判断"哪些 AC Fail 可独立分解"
      - name: Verdicts
        type: []PerCriterionVerdict
        required: true
        note: PerCriterion executor 产出
      - name: RoundMeta
        type: RoundMeta
        required: true
        note: round 元数据,用于决策映射
  RoundMeta:
    fields:
      - name: AttemptNo
        type: int
        required: true
        default: 0
        note: 当前 segment 已重试次数(0 = 首次)
      - name: ChildBudgetRemaining
        type: int
        required: true
        default: 2
        note: 剩余可 spawn 子 Worker 数,降级路径
      - name: RiskLevel
        type: enum {high, normal, low}
        required: true
        default: normal
      - name: IsChildSegment
        type: bool
        required: true
      - name: SiblingDecidedCount
        type: int
        required: true
      - name: SiblingTotalCount
        type: int
        required: true
outputs:
  DecisionNodeOutput:
    fields:
      - name: Decision
        type: Decision
        required: true
        schema: decision_v1.json
        constraint: |
          - 决策映射表 11 行全测覆盖(10 baseline + 1 plan_error,见 §3.14)
          - 落 round.Metadata["decision"] = JSON{kind, reason, next_spec}
decision_table:
  rows: 11
  columns: [Verdict, OtherConditions, Decision]
  rows:
    - [Pass, "(default)", accept]
    - [Partial, "Tolerance=high OR ChildBudget=0", accept]
    - [Partial, "Partial AC 可独立分解 + Budget>0", child_worker]
    - [Partial, "其它", accept]
    - [Fail, "AttemptNo < MaxRetry=1", retry]
    - [Fail, "AttemptNo >= MaxRetry", human_review]
    - [Indeterminate, "RiskLevel=high", human_review]
    - [Indeterminate, "RiskLevel=normal/low", retry]
    - ["Error(全Err)", "Network/Timeout 类", retry]
    - ["(任意)", "IsChildSegment + SiblingDecidedCount==SiblingTotalCount", parent_rollup]
    - ["plan_error", "(Plan LLM timeout / 5xx / partial response)", human_review]
errors:
  - ErrDecisionMapMiss (静态映射表未命中 → 降级 accept + log warning)
  - ErrChildWorkItemBudgetExhausted (ChildBudget=0 + Decision=child_worker)
  - ErrChildWorkItemInheritACEmpty (子 AC[] 为空)
short_circuit:
  when: |
    Decision.Kind == accept AND RoundMeta.IsChildSegment == false
    then: 不触发 child WorkItem,直接 emit final + Learn
```

---

## 3.9 New Type: LearnRequest(DM-20260707-001 P0)

```yaml
name: LearnRequest
owner: D7 (mups/learn subpackage)
lifecycle: stable
introduced: DM-20260707-001
description: |
  Learn 节点输入(D7 6 节点流水线最末 stage,独立 stage)。
  接收 **节点 5 Decision** + **节点 4 Verify** + 节点 3 ArtifactHash(可选)。
  契约精简:不再收 Plan / Observations / ParentContext(由 Learn 内部按需查 DB)。
schema:
  fields:
    - name: WorkItemID
      type: string
      required: true
      note: per-segment attribution
    - name: Decision
      type: Decision
      required: true
      note: 来自节点 5(Decision 节点)
    - name: Verdict
      type: Verdict
      required: true
      note: 来自节点 4(Verify 节点)
      fields: [Kind, SourceID, Confidence, Evidence, IndeterminateReason]
    - name: ArtifactHash
      type: string
      required: false
      note: 来自节点 3(Execute 节点),防重
    - name: PlanRationaleHash
      type: string
      required: false
      note: ≤64B,SHA256[:16] hex,plan_rationale 指纹(H1 修复,2026-07-07),用于 reputation.metadata.rationale_hash 关联 + 诊断反查 DB
  不收字段(契约精简):
    - Plan / PlanLLMOutput(已由 Plan 节点持久化,reputation 不冗余)
    - Observations(已由 Observe 节点持久化,Learn 不消费)
    - ParentContext(改为 Learn 内部按 Decision.parent_rollup 从 DB 查 child rows 后 sum)
errors:
  - ErrLearnRequestWorkItemIDEmpty
  - ErrLearnRequestVerdictMissing
  - ErrLearnRequestDecisionMissing
```

## 3.10 New Type: LearnResponse(DM-20260707-001 P0)

```yaml
name: LearnResponse
owner: D7 (mups/learn subpackage)
lifecycle: stable
introduced: DM-20260707-001
description: |
  Learn 节点输出。返回 BayesianUpdate 后的 α/β + BayesianAction 枚举。
schema:
  fields:
    - name: UpdatedAlpha
      type: float
      required: true
    - name: UpdatedBeta
      type: float
      required: true
    - name: ReputationRowID
      type: string
      required: true
    - name: BayesianAction
      type: enum
      values:
        - alpha_bump     # α++,VerdictPass
        - beta_bump      # β++,VerdictFail
        - no_change      # retry / hash 命中
        - force_plan     # β/(α+β) > 0.7,下次 Observe 降级
      required: true
errors:
  - ErrLearnResponseDBWriteFailed (DB 写挂,silent log)
```

## 3.11 New Type: ReputationRow(DM-20260707-001 P0)

```yaml
name: ReputationRow
owner: D7 (mups/learn/reputation subpackage)
lifecycle: stable
introduced: DM-20260707-001
description: |
  reputation 持久化 row schema。跨 session 累积 α/β,per-segment 独立。
  parent_rollup row = sum(child α/β),不重新贝叶斯。
schema:
  fields:
    - name: ID
      type: string (uuid)
      required: true
    - name: SegmentID
      type: string
      required: true
      note: per-segment 独立 row
    - name: ParentID
      type: string
      required: false
      note: 指向 parent rollup row(parent_rollup 时填)
    - name: Alpha
      type: float
      required: true
      default: 5  # cold start BuildAdaptivePrior
    - name: Beta
      type: float
      required: true
      default: 3  # cold start BuildAdaptivePrior
    - name: LastUpdated
      type: time.Time
      required: true
    - name: DecisionKindHistory
      type: []string
      required: true
      default: []
      note: 最近 5 次 DecisionKind(滚动)
    - name: SourceIDHistory
      type: []string
      required: true
      default: []
      note: 最近 5 次 Verdict.SourceID
    - name: PlanRationale
      type: string
      required: false
      deprecated: DM-20260707-001
      note: 字段已移除(Learn 契约精简,不再收 Plan),Plan 节点自己持久化 plan_rationale 到 plan 表
    - name: ArtifactMetadataHash
      type: string
      required: false
      note: 防重,同一 artifact 重复 learn → no_change
  invariant:
    - Alpha >= 0 AND Beta >= 0
    - len(DecisionKindHistory) <= 5
    - len(SourceIDHistory) <= 5
```

---

## 14. New IO Contract: LearnNodeIO

```yaml
name: LearnNodeIO
owner: D7
introduced: DM-20260707-001
description: |
  Learn 节点(D7 6 节点流水线最末 stage) ↔ 上下游的输入输出契约。
  Learn 接收节点 5(Decision)+ 节点 4(Verify)+ 节点 3(ArtifactHash 可选)的输出。
  Learn 异步执行,不阻塞 emit final。
  Learn 异步执行,不阻塞 emit final。
inputs:
  LearnNodeInput:
    fields:
      - name: Request
        type: LearnRequest
        required: true
outputs:
  LearnNodeOutput:
    fields:
      - name: Response
        type: LearnResponse
        required: true
        schema: learn_response_v1.json
  BayesianUpdate_formula: |
    prior α₀, β₀  (cold start = Beta(5, 3))
    on Verdict.Pass           → α += 1
    on Verdict.Fail           → β += 1
    on Decision.Kind=accept   → 累计(正常 signal)
    on Decision.Kind=retry    → 不累计(避免 noise)
    on Decision.Kind=child_worker → child 完成后单独累计;parent 自身不累计
    on Decision.Kind=human_review → β += 1(强 negative)
    on Decision.Kind=parent_rollup → sum child α/β
  ParentEvidence_aggregator: |
    触发: Decision.Kind == parent_rollup
    公式: parent_row.α = sum(child_row.α), parent_row.β = sum(child_row.β)
    边界: child row 缺失 → 降级只更新存在的 child,log warning
    边界: 同一 parent 多次 rollup → 累加(不重置)
  force_plan_trigger: |
    条件: β / (α + β) > 0.7
    动作: BayesianAction = force_plan
    效果: 写 next_observation_force_plan=true 到 segment_id metadata
         下次 Observe 阶段读 metadata,降级到 Plan 路径(防低质量直接 fast-path)
async_semantics: |
  - Learn 异步执行,emit final 后再 enqueue,主流程延迟 < 5ms
  - 队列 size = 100(有界,防内存爆)
  - 队列满 → 降级同步执行(Sync.Learn)+ log warning
  - worker 数 = 2(默认)
  - Drain(ctx) 阻塞直到队列空(测试 / session 结束用)
errors:
  - ErrLearnRequestWorkItemIDEmpty
  - ErrLearnRequestVerdictMissing
  - ErrLearnResponseDBWriteFailed (silent log, 不阻塞主流程)
  - ErrArtifactMetadataHashDuplicate (no_change, 不报错)
```


## Acceptance Criteria (S5 验收)

> **章节导航**:AC 列表保留在 Affected Files 之前,符合 OpenSpec 规范(验收标准 + 实施细节并列)。所有 AC1-AC68 保持原有逻辑顺序,见章节导航索引表。

---

## Affected Files (汇总)

- `internal/layers/orchestration/orchtypes/intent_segment.go` (NEW)
- `internal/layers/orchestration/plan/plan_dag.go` (NEW)
- `internal/layers/orchestration/plan/dag_validator.go` (NEW)
- `internal/layers/orchestration/sessionorchestrator/intent_segmenter.go` (NEW)
- `internal/layers/orchestration/sessionorchestrator/item_pipeline.go` (MODIFIED)
- `internal/layers/orchestration/plan/strategic_plan_proposer.go` (MODIFIED)
- `internal/layers/orchestration/plan/spawn_policy.go` (MODIFIED)
- `internal/layers/orchestration/wavescheduler/dag_executor.go` (NEW)
- `internal/layers/orchestration/wavescheduler/runners/dag_runner.go` (NEW)
- `internal/layers/orchestration/mups/learn/learner.go` (MODIFIED)
- `internal/layers/orchestration/mups/learn/reputation/parent_evidence.go` (NEW)
- `internal/layers/communication/feishu/streaming.go` (NEW)
- `internal/layers/contextengine/i18n/format_hints_mups.go` (MODIFIED, IntentSegment appendix)
- `internal/bootstrap/config.go` (MODIFIED, feature flag)
- Tests for each package
- `openspec/specs/d7-orchestration/spec.md` (lite-mode update, ≤ 100 行)
- `openspec/specs/d7-orchestration/t-registry.md` (新增 T 点)

---

## T-Registry Delta

新增 T 点登记在 `openspec/specs/d7-orchestration/t-registry.md`:

```
D7-S15-A61-T01 .. T18 (PR-A grammar/SpawnPolicy)
D7-S15-A62-T01 .. T05 (PR-B WaveScheduler DAG executor)
D7-S15-A63-T01 .. T05 (PR-C streaming + idempotency)
D7-S15-A64-T01 .. T03 (PR-D config + e2e + verify-archive)
```

共 18 个 T 点。
