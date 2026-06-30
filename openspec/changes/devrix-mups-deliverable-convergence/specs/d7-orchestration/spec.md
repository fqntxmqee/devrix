# Delta: D7 Orchestration — MUPS Deliverable Convergence

**Change ID:** `devrix-mups-deliverable-convergence`  
**Demand ID:** DM-20260630-012  
**Affects:** D7-S2, D7-S5, D7-S9, D7-S15, D7-S16

---

## ADDED

### Requirement: D7-S16-A76 StrategicPlanProposer @ Plan (G3)

Plan 阶段可选 LLM 战略提案器：经 D2 Prepare 后调用 D3，输出 JSON 战略计划（execution_mode、scope_in、child_specs、deliverable_schema、react_iters_hint）。`ValidateStrategicPlan` 规则门控后映射为 `plan.PlanInput`。LLM/校验失败时 fallback 至 DefaultPlanner + 现有 rule decompose。

#### Scenario: LLM proposes single-mode review
- GIVEN UncertaintyReport 与 directive「review internal/layers/contextengine/kernel/」
- WHEN StrategicPlanProposer 返回 `execution_mode=single` 且 scope_in 合法
- THEN ItemPipeline 不调用 DefaultDecomposeProposer 固定 2 路拆分
- AND Plan.Steps 长度为 1

#### Scenario: LLM failure fail-safe
- GIVEN StrategicPlanProposer InvokeStream 失败或 JSON 非法
- WHEN ItemPipeline Plan 节点执行
- THEN fallback DefaultPlanner 行为与 change 前一致（无回归）

---

### Requirement: D7-S9-A32 DeliverableVerifier

Verify 阶段对 ExpectedReturn / deliverable_schema 做程序校验。首期 schema `p0_p1_file_line`：Artifact 须含 file:line citation 与 P0/P1 结构；`stop_reason=max_iters` 且无 citation 判 incomplete。

#### Scenario: Review deliverable valid
- GIVEN Execute 产出含 `internal/.../contracts.go:42` 与 P0 finding
- WHEN VerifyDeliverable(p0_p1_file_line)
- THEN DeliverableStatus=complete AND Verdict 可为 Pass

#### Scenario: max_iters without findings
- GIVEN Artifact summary 为探索过渡句且无 file:line
- AND metadata.stop_reason=max_iters
- WHEN VerifyDeliverable(p0_p1_file_line)
- THEN DeliverableStatus=incomplete AND WorkItem 不得映射为 Completed

---

### Requirement: D7-S15-A41 StructuredDeliverable upward bubble

WorkItemPipelineRound 携带 StructuredDeliverable（findings JSON）。Verify 解析写入；Context bubble 向父 Observe 传递 findings digest；Rollup Execute 合并子 deliverable 为单一报告。

#### Scenario: Parent Observe receives child findings
- GIVEN 子 WorkItem 完成且 DeliverableStatus=complete
- WHEN 父节点下一轮 Observe
- THEN inbound signal 含 child findings 摘要（非仅 verdict=pass）

---

### Requirement: D7-S2-A73 Session Deliverable Gate

RunSessionTurnLoop 终止时 complete.Content 优先 ExtractSessionDeliverable；经 LastTextQualityGate；summary/final 均 bad 时 D1 展示 TaskIncompleteMessage。

#### Scenario: Complete prefers rollup over last child transition
- GIVEN Goal rollup ArtifactSummary 含 P0/P1 报告
- AND 最后处理的子 WorkItem ArtifactSummary 为过渡句
- WHEN session turn loop 结束
- THEN complete.Content 为 rollup 报告而非过渡句

---

## MODIFIED

### Requirement: D7-S5-A22 DefaultPlanner / MatchKind

MatchKind 不得在无 anomaly 且 StrategicPlan 指定 single/step=1 时因 Goal kind alone 强制 ExplorationPlan。QuantizedKind 来自 StrategicPlan 映射优先于 planQuantizedKind 硬编码。

#### Scenario: Single plan from LLM overrides orchestrate default
- GIVEN StrategicPlan execution_mode=single
- WHEN DefaultPlanner.MatchKind 输入 stepCount=1 anomalies=0
- THEN PlanKind=CommitmentPlan（非 ExplorationPlan）

---

### Requirement: StatusAfterSpawnNone (D7-S1)

Partial Verdict 仅在 DeliverableStatus=complete 时映射 TaskStatusCompleted；incomplete 时保持 InProgress 以触发 inline/rollup。

#### Scenario: Partial without deliverable stays open
- GIVEN VerdictPartial AND DeliverableStatus=incomplete
- WHEN ApplyPipelineRound + StatusAfterSpawnNone
- THEN TaskStatus=InProgress（非 Completed）

---

## REMOVED

(None — DefaultDecomposeProposer 保留为 fallback)

---

## L5 Test Points (register in t-registry)

| ID | Description |
|----|-------------|
| D7-S5-A22-T01 | StrategicPlanProposer JSON + ValidateStrategicPlan |
| D7-S5-A22-T02 | Plan gate rejects invalid scope |
| D7-S9-A32-T01 | DeliverableVerifier p0_p1_file_line |
| D7-S9-A32-T02 | Partial incomplete not Completed |
| D7-S2-A73-T03 | Session complete prefers ExtractSessionDeliverable |
| D7-S2-A73-T04 | Item pipeline LastTextQualityGate on complete |
| D7-S15-A41-T02 | StructuredDeliverable bubble + rollup merge |
| D7-S16-A76-T01 | Bootstrap wire StrategicPlanProposer |
