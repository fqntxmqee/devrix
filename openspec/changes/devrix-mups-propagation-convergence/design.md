# Design: MUPS+WorkTree 传播闭环修复（P0 重点）

- **Demand ID:** DM-20260701-001
- **Change ID:** devrix-mups-propagation-convergence

> 本文聚焦 P0 四项（C1–C4）的技术设计，P1/P2 给出方向。所有代码片段为示意，最终以实现为准。遵循 `03-conventions` D7 反战术硬编码：自然语言/marker 进 i18n，Go 侧只留结构与机器契约。

---

## C1 UncertaintyReconcile（RH-MUPS-01/02）

### 现状

两个写入路径，方向相反、无协调：

```302:368:devrix/internal/layers/orchestration/sessionorchestrator/item_pipeline.go
	uncertaintyMean := workmodel.ComputeUnifiedUncertainty(...)
	if item.Uncertainty > uncertaintyMean {
		uncertaintyMean = item.Uncertainty   // ← 裸 ratchet，只升不降
	}
```
```42:43:devrix/internal/layers/orchestration/workmodel/resolve.go
	u := ComputeUncertainty(parent, stats, parent.Uncertainty, 0)
	_ = tm.Tree().SetUncertainty(sessionID, parent.ID, u)   // ← 可降，与上面冲突
```

### 设计

新增唯一 reconcile 纯函数，作为 `item.Uncertainty` 的语义入口：

```go
// ReconcileUncertainty merges the previous stored value, the current pipeline
// round signal, and child outcome stats into a single converged value.
// Convergence (all children terminal & passing) MUST allow the value to drop;
// single-round optimism is damped by historical so it cannot unilaterally win.
func ReconcileUncertainty(prevStored, roundSignal float64, stats ChildOutcomeStats) float64 {
	hist := historicalUncertainty(stats)              // child 全 pass → 0
	if stats.Total > 0 && stats.Running == 0 {
		// 终态收敛：以 child 结果为主，prevStored 不再钉死
		return clamp01(0.7*roundSignal + 0.3*hist)
	}
	// 仍在进行：保守，取偏高（防乐观虚降），但不无脑 max
	return clamp01(math.Max(roundSignal, 0.5*prevStored+0.5*hist))
}
```

- `item_pipeline.go` 删除裸 max；改 `round.UncertaintyMean = workmodel.ReconcileUncertainty(item.Uncertainty, unified, stats)`。
- `reevaluateParentAfterChild` 改调用同一函数，`SetUncertainty` 写入结果。两路统一 → RH-MUPS-02 消解。

---

## C2 RollupTerminationGuard（RH-MUPS-03）

### 设计

1. `WorkItemPipelineRound` 增字段 `RollupRetries int`（仿 `IndeterminateRetries`）。
2. `pipeline_round.go` 构造 `TreeEvalContext` 时透传：
   ```go
   if item.NeedsRollup && item.LastRound != nil {
       ctx.RollupRetries = item.LastRound.RollupRetries
   }
   ```
3. `spawn_policy.go` rollup 分支加上限（替换无条件 `SpawnInline`）：
   ```go
   case types.VerdictPartial, types.VerdictFail, types.VerdictIndeterminate:
       if ctx.RollupRound {
           if ctx.RollupRetries >= ctx.MaxRollupRetries {  // 默认 3
               return SpawnEscalateHuman
           }
           return SpawnInline
       }
   ```
4. `item_pipeline.go` 当 `isRollup && verdict != Pass` 时 `round.RollupRetries = prev+1`。
5. `session_turn_loop.go:136` break 前：若存在 `NeedsRollup && !terminal` 的 parent 且已达上限，emit 显式 `human_review` 或 `error`，不静默退出。

---

## C3 RollupOutcomeAggregation（RH-MUPS-04）

### 现状

```56:57:devrix/internal/layers/orchestration/workmodel/rollup_gate.go
	case RollupGateBestEffort:
		return stats.Completed+stats.Failed == stats.Total   // Failed==Total 也放行
```
rollup verify（`rollup_verify.go`）只看 summary 形态，不看子裁决。

### 设计

- 允许进入 rollup（合成"含失败"的交付仍有价值），但**收敛结局**必须反映失败：
  ```go
  // 在 rollup 轮的 apply 处
  if isRollup {
      stats := childOutcomeStats(...)
      if stats.Total > 0 && stats.Failed == stats.Total {
          // 全失败：rollup 产物即便形态合格，也不得 Completed
          status = TaskStatusFailed   // 或 Partial + 显式 reason
      }
  }
  ```
- `buildRollupDirective` 增"失败子集"区段，确保合成 deliverable 显式列出失败（不洗白）。
- `verifyRollupArtifact` 增 input：`childPassRate`；`Failed==Total` 时强制非 Pass verdict。

---

## C4 AcceptanceCriteriaVisibility（RH-MUPS-10）

### 现状

- `AppendDeliverableExecuteHint` 只注入 schema **tag**（机器标记），无可读验收要点。
- verify 失败 `Reason`（如 "missing p0/p1 file:line deliverable"、"rollup summary too short"）**不回灌**下一轮 execute。

### 设计

1. **可读验收要点注入**（i18n，非 Go 硬编码）：
   ```go
   // AppendDeliverableExecuteHint 增 acceptance criteria 行（来源 i18n/format_hints）
   // e.g. p0_p1_file_line → "验收要求：输出须含 file:line 引用与 P0/P1 严重度标注"
   hint := tag + "\n" + i18n.AcceptanceCriteria(schema, locale)
   ```
2. **失败 Reason 回灌**：`WorkItemExecContext` 增 `PriorVerifyReason string`；`item_pipeline` 在 inline 重试时把上一轮 `verdict.Reason` 填入；execute prompt 增"上一轮未通过原因"区段，引导针对性修正。
3. **rollup denylist 去硬编码（RH-MUPS-12）**：`rollupPlanningDenylist` 迁至 i18n / `format_hints`，Go 侧只保留"读取 + 判定"。

---

## P1/P2 方向（摘要）

- **D1 DivergenceBudgetVisibility**：`buildStrategicPlanUserPrompt` 增 `depth/max_depth/remaining_children/remaining_daily/parent_scope_in`；上限常量集中 `workmodel` 单一来源，prompt 与 `CapChildSpecs` 共用；超额返回结构化 `reject{reason, max_allowed}`。
- **D2 SchemaMonotonicNarrowing**：`schema = NarrowestSchema(inferred, strategic)`，`not_applicable` 不能覆盖具体 schema。
- **E1 ScopeSubdivisionContract**：`DefaultChildDownlink` 移除"无脑继承父全量"；`ValidateChildScopes(parent, children)` 校验真子集 + 合并覆盖。
- **E2 ChildUncertaintyBubble**：`observationsFromChildStructuredBubbles` 在 `UncertaintyMean ≥ threshold` 时额外产出 `ObsUncertainty`。

## 测试策略

- 纯函数（C1/C3 reconcile/aggregation）→ 表驱动单元测试。
- C2 终止保证 → rollup 故障注入：mock verify 永远 fail，断言达上限转 escalate，不超 loop。
- C4 → execute prompt 快照测试（含验收要点 + 回灌 reason）。
- 全部关联 `t-registry` 新登记 T 点（见 tasks.md）。
