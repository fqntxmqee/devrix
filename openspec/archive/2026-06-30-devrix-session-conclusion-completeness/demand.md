---
demand-id: DM-20260630-011
title: Session 结论完整性 — 4 类 silent failure 修复（B/C/D/E）
priority: P0
status: S1_Proposal
dsaft_domain: orchestration
created: 2026-06-30
---

# Session 结论完整性 — 4 类 silent failure 修复

## 1. 背景

2026-06-30 排查 sess_1782814140202_7000（用户向 devrix 发"review d2 领域 kernel 代码"指令）时，Jaeger 1584 spans 暴露 4 类 silent failure 模式同时发生，导致：

1. **飞书回复卡片显示 scope-contract recap 而非 review findings**（用户已观察到 553 字符重复内容 + 无正常结果）
2. **D2 Materialize span 永远报告 message_count=0**（observability 盲区）
3. **D7 Verify 节点永远不触发 task_incomplete anomaly**（即使 LLM 产生 6 条 ObsUncertainty+CatSystem）
4. **Learn 节点 prior 完全静态化**（rule 硬编码 + DefaultDeveloperPrior mean=0.625 固定）

4 类问题**根因独立但症状联动**：LLM 把最后一条 turn 写成 `<scope_contract>...</scope_contract>` 模板（553 字符）而非真正的 review findings → D7 turn_loop 把任何非空文本写入 `lastTurnText` → D7 finalizeLoop 用 `lastTurnText` 作为 `summary` → D1 EmitComplete 在 `summary==""` 时 fallback 到 `event.Content`（已 StripPriorOutputSummary 后的 transcript 残留）→ 飞书卡片显示 553 字符 scope contract recap。

> 当前 4 类 silent failure 模式下，用户**只看到症状**（卡片内容不对 + 没正常结果），**不知道内部 4 处独立的 silent failure 链路**。这与 devrix-d7-error-aggregation-and-metrics (DM-20260621-010) 的"silent failure 模式"主题一致，但本次具体到 session conclusion 路径。

排查过程详见 `/Users/fukai/brain/02知识沉淀/永久笔记/项目/devrix-sess_1782814140202_7000-排查分析.md`。

## 2. 问题陈述

### 2.1 Bug B — D7 Orchestrator 不区分"工作中输出"与"总结输出"

**现象**：LLM 最后一条 turn 输出 `<scope_contract>...</scope_contract>` 模板（553 字符）而非 review findings，被 D7 当作 `lastTurnText` → `summary` → meta["summary"]，触发了 D1 三层 fallback 链。

**根因定位**：
- `internal/layers/orchestration/sessionorchestrator/turn_loop.go:184-188` —— 任何非空 LLM 文本都写入 `lastTurnText`，不区分工作内容 vs 总结
- `internal/layers/orchestration/sessionorchestrator/turn_recovery.go:53-60` —— `resolvedSummary` 直接从 `lastTurnText` 推导，无质量校验

### 2.2 Bug C — D2 Materialize span 永远报告 message_count=0

**现象**：本次会话 15 次 Materialize 调用，Jaeger 全程显示 `materialize.message_count=0, materialize.token_est=0`，但 D2 实际生成了非零 messages。

**根因定位**：
- `internal/layers/orchestration/sessionorchestrator/workitem_executor.go:431`
  ```go
  end := hardening.EmitContextMaterialize(ctx, sessionID, itemID, string(req.Policy.Mode), 0, 0)  // ← 硬编码 0,0
  mat, err := e.Materializer.Materialize(ctx, req)  // 返回值 mat.Messages/TokenEstimate 从未回填到 span
  end(err)
  ```

**影响**：D5 observability 完全失明，无法在 Jaeger 区分"Materialize 真返回 0 条消息" vs "instrumentation 没回填"。这是 **false instrumentation** —— observability 看起来正常但实际值是缺失的。

### 2.3 Bug D — D7 Verify 节点 anomaly 检测只覆盖 cat_system_aggregate

**现象**：本次会话 Observe 节点产生 6 条 `ObsUncertainty+CatSystem`（str 0.78→0.9），但 `D7_System_Anomaly_Detect × 16` 全部 `triggered=false`，Verify 永远不把"任务未完成"作为异常信号上报。

**根因定位**：
- `internal/layers/orchestration/executionflow/verify/anomaly.go:33-51` —— AnomalyKind 只有 5 种（`cat_system_aggregate` + 4 个 v6.1 reserved），无 `task_incomplete` / `empty_conclusion` 类规则
- `internal/layers/orchestration/orchtypes/uncertainty_report.go:79-89` `Partition()` —— Anomalies 只收录 `CatSystem + ObsDeviation`，`ObsUncertainty` 即使 str ≥ 0.7 也不进入
- `internal/layers/orchestration/orchtypes/system_anomaly_wiring.go:49-53` `EvaluateSystemAnomaly` —— 纯 boolean aggregator，无 task-level 信号

### 2.4 Bug E — Learn 节点 classifier_source=rule 硬编码 + prior 完全静态化

**现象**：本次会话 `learn.classifier_source=rule` × 16 rounds 全部相同；`learn.prior.mean=0.625`（DefaultDeveloperPrior Beta(5,3)）从 round 1 到 round 16 不变；6 条 ObsUncertainty 完全没影响 Learn 的 prior 更新。

**根因定位**：
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go:388-394` —— 硬编码 `learn.classifier_source="rule"`
- `internal/layers/orchestration/decisionplanning/classifier.go:79-141` —— RuleClassifier 纯规则匹配，无 LLM 调用
- `internal/layers/orchestration/mups/learn/prior/adaptive_prior.go:46-50` —— DefaultDeveloperPrior/DefaultOperatorPrior 静态常量
- `internal/layers/orchestration/mups/learn/prior/adaptive_prior.go:80-92` `BuildAdaptivePrior` —— 仅 Bayesian merge DefaultPrior + ReputationEvidence，**不读 UncertaintyReport.Observations**

## 3. 验收标准

| ID | 标准 | 优先级 | 关联 Bug |
|----|------|--------|----------|
| AC1 | `D7_LastText_Quality_Gate` 新 span 在 D7 finalizeLoop 阶段触发，对 `resolvedSummary` 做 3 类判定（`summary_valid` / `summary_too_short` / `summary_inconclusive`），输出 `summary.kind` + `summary.length` + `summary.exit_reason` 3 个 attribute | P0 | B |
| AC2 | D1 EmitComplete fallback 链路在 `summary==""` 时**不再静默** fallback 到 `event.Content`；触发 D2 fallback 路径时记录 `D1_EmitComplete_Fallback` span，attribute 含 `fallback.source`（`event.Content` / `stats`） + `fallback.content_length` | P0 | B |
| AC3 | `hardening.EmitContextMaterialize` 在 `Materializer.Materialize` 调用**之后**用 `mat.Messages` / `mat.TokenEstimate` 回填 span attribute，新增 `D2_Materialize_EmptyYield` span 在 `len(mat.Messages)==0` 时触发（kind=`empty_yield` + wi_id） | P0 | C |
| AC4 | `orchtypes.UncertaintyReport.Partition()` 在 `o.Kind == ObsUncertainty && o.Strength >= 0.7` 时把 observation 加入 `Anomalies`；`evaluate.go` 测试覆盖 3 种 case（不达标 / 达标 + CatBusiness / 达标 + CatSystem） | P0 | D |
| AC5 | `executionflow/verify/anomaly.go` 新增 2 个 `AnomalyKind`：`AnomalyKindTaskIncomplete` + `AnomalyKindEmptyConclusion`，对应 detector 规则 = `FilterByKind(ObsUncertainty) len ≥ 2 && avg(strength) ≥ 0.7`，triggered 时 emit `D7_Anomaly_Trigger` span 带 kind=新值 | P0 | D |
| AC6 | `DecisionPlanning.IntentClassifier` 接口扩展 `ClassifyWithPrior` 接收 `*orchtypes.UncertaintyReport` 参数（**不变更现有签名**，新增 `ClassifyWithReport` 重载），RuleClassifier 实现默认回退到 `ClassifyWithPrior`；`learn.classifier_source` 从硬编码 `"rule"` 改为 `intent.Source` 字段（rule / llm / hybrid） | P1 | E |
| AC7 | `BuildAdaptivePrior(rep, trackMode, report)` 新增第三参数 `*orchtypes.UncertaintyReport`，当 `len(report.ObsUncertainty with strength ≥ 0.7) ≥ 2` 时 override 静态 DefaultDeveloperPrior，使用 `prior = BetaPrior{Alpha: max(0, base.Alpha - penalty), Beta: base.Beta + penalty}` 公式（penalty = `sum(uncertainty_strength)`），保留 ReputationEvidence merge | P1 | E |
| AC8 | 新增 E2E 集成测试 `TestSession_ConclusionCompleteness_5BugsRegression`，模拟 sess_1782814140202_7000 重现场景（review 任务 + LLM 输出 scope-contract-recap + 6 条 ObsUncertainty），断言：(a) AC1 span 触发 kind=`summary_inconclusive`；(b) AC2 span 触发 fallback.source=stats；(c) AC3 materialize.message_count > 0；(d) AC5 Verify anomaly triggered=true 且 kind=AnomalyKindEmptyConclusion；(e) AC7 prior.mean 显著下降至 ≤ 0.4（vs 静态 0.625） | P0 | 全 |
| AC9 | 不变更用户接口（devrix.yaml 配置项、`SessionOrchestrator.ProcessMessage` 签名、feishu adapter 用户可见行为）；所有新增 span 走 hardening.EmitXxx 包级 bridge（NewNoOp 兜底） | P0 | 不变更约束 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | DM-20260621-010 devrix-d7-error-aggregation-and-metrics（silent failure 模式三联固化模式，本次复用） |
| 依赖 | DM-20260630-003 devrix-spec-lite-mode（spec.md lite-mode 5 原则 + 4 反模式；本次不写大 spec.md 段） |
| 依赖 | DM-20260624-001 devrix-d7-mups-v4-phase6-observe-learner-wiring（UncertaintyReport + Observer 子模块基础） |
| 约束 | **不动阈值类设计**（dedup / 相似度 / rate limit）；Bug A（飞书卡 dedup 阈值过宽）由后续独立 Change 重新设计解决，不在本次范围 |
| 约束 | 不破坏现有 LP-1 / LP-2 / LP-5 E2E baseline；CI slim down (DM-20260619-005) 单测检查时间 ≤ 5min |
| 约束 | 新增 span 必须走 hardening bridge（zero-overhead when bridge nil） |
| 约束 | 不引入新外部依赖；不修改 `devrix.yaml` 用户配置 schema |

## 5. 变更范围

### 新增

- `internal/layers/orchestration/hardening/emitter.go` 新增 `EmitLastTextQualityGate` / `EmitEmitCompleteFallback` / `EmitMaterializeEmptyYield` 3 个 span helper
- `internal/layers/orchestration/executionflow/verify/anomaly.go` 新增 `AnomalyKindTaskIncomplete` / `AnomalyKindEmptyConclusion` 常量 + detector 规则
- `internal/layers/orchestration/orchtypes/intent.go` `IntentClassification.Source` 字段新增（`SourceRule` / `SourceLLM` / `SourceHybrid`）
- `internal/layers/orchestration/orchtypes/uncertainty_report.go` `Partition()` 新增 `ObsUncertainty strength ≥ 0.7` 兜底
- `internal/layers/orchestration/mups/learn/prior/adaptive_prior.go` `BuildAdaptivePrior` 签名扩展（向后兼容：旧调用走 variadic 或新增 overload）
- `internal/layers/orchestration/sessionorchestrator/item_pipeline_integration_test.go` 新增 E2E 回归测试（AC8）

### 修改

- `internal/layers/orchestration/sessionorchestrator/turn_recovery.go` `finalizeLoop` 调用 `LastTextQualityGate` 校验 `resolvedSummary`
- `internal/layers/communication/conclusion/conclusion.go` `EmitComplete` 在 `summary==""` 时不再静默 fallback，触发 span + 走 `D1_EmitComplete_Fallback` 路径
- `internal/layers/orchestration/sessionorchestrator/workitem_executor.go:431` `prepareContext` 调用 Materialize 后回填 span attribute + 检查 empty yield
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go:388-394` `learn.classifier_source` 改为 `intent.Source` 字段
- `internal/layers/orchestration/decisionplanning/classifier.go` `RuleClassifier` 新增 `ClassifyWithReport` overload（默认 = `ClassifyWithPrior` + SourceRule 标记）

### 不变更

- `internal/layers/communication/channel/adapters/feishu_progress.go:53-58` dedup 阈值（**用户已确认 threshold 设计 anti-pattern**；后续独立 Change 重新设计）
- devrix.yaml 用户配置 schema
- `SessionOrchestrator.ProcessMessage` 公开签名
- `wavescheduler.Artifact` / `taskreport.Result` 等下游契约（避免连锁影响）
- D5 observability Operation Registry 已有的 56 条 op（新增 op 走 v6.x minor 版本递增）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| AC4 `Partition()` 修改 ObsUncertainty 进入 Anomalies 后，可能让本来不触发的 session 现在 trigger task_incomplete，导致下游 Verdict 变化 | 高 | AC8 回归测试覆盖 + Phase 灰度（先开 feature flag `conclusion_completeness_v1=false` 默认关闭） |
| AC6 `IntentClassifier.ClassifyWithReport` 接口扩展，可能让 RuleClassifier 在 hot path 上变慢 | 中 | 默认实现 = `ClassifyWithPrior`（已有性能基线）；新增路径仅在 `ClassifyWithReport` 调用时触发 |
| AC7 prior dynamic override 可能让 DefaultDeveloperPrior mean 在某些 session 暴降至 ≤ 0.2，影响下游 Confidence 字段 | 中 | penalty 公式加 floor：`prior.Mean >= 0.1`；保留 ReputationEvidence merge 不变 |
| AC3 materialize.span 回填改动，可能影响现有 hardening test 的 `TestEmitContextMaterialize_SpanNoPanic`（workitem_executor_test.go:168） | 低 | 复用 hardening bridge nil-safe 模式 + 新增空 mat 单元测试 |
| 跨域改动（D1 + D2 + D7）导致 verify-archive.sh 11/11 PASS baseline 飘移 | 低 | 同步更新 d1/d2/d7 domain specs 在 lite-mode 5 原则下追加 change-level delta（参照 DM-20260630-003~009 模式） |

## 7. 后续（Out of Scope）

1. **Bug A**（飞书卡 dedup 阈值 50 runes 过宽）—— 用户已确认 threshold-based 设计是 anti-pattern，需重新设计（例如按 chunk 类型 / stop_reason / event 类型路由，而非按长度匹配）。立独立 Change `devrix-feishu-streaming-dedup-redesign`，本次**不在范围**。
2. Learn node 在 `ClassifyWithReport` 之外引入真正的 LLM Classifier（hybrid 路径）—— 立独立 Change `devrix-d7-llm-classifier-promotion`，本次只做接口预留（Source 字段 + overload stub）。
3. Materialize 在 workitem 级别做缓存 / CoW —— 立独立 Change `devrix-d2-materialize-cache`，本次只做 instrumentation 回填。

---

**变更自检**：

- [x] DM ID 已分配且无冲突：DM-20260630-011（前序 DM-20260630-010 为 devrix-verify-spec-links，今日序号递增）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 至少 1 个 P0 验收标准：AC1 / AC2 / AC3 / AC4 / AC5 / AC8 / AC9 共 7 个 P0
- [x] Out of Scope 已明确声明（§7）
- [x] DSAFT 域标注正确：primary orchestration, cross D1/D2/D7（已写入 .openspec.yaml）
- [x] 不含工时估算（按 §6 禁止）