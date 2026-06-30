# Proposal: Session 结论完整性 — 4 类 silent failure 修复

**Change ID:** `devrix-session-conclusion-completeness`  
**Demand ID:** DM-20260630-011  
**Status:** S2_Design → S3_Design → S4_Implemented → S5_Accepted → S7_Archived  
**Created:** 2026-06-30

---

## 1. Background

2026-06-30 排查 sess_1782814140202_7000（用户向 devrix 发"review d2 领域 kernel 代码"指令）时，Jaeger 1584 spans 暴露 4 类 silent failure 模式同时发生：

1. **Bug B** — 飞书回复卡片显示 scope-contract-recap 而非 review findings（用户已观察 553 字符重复内容 + 无正常结果）
2. **Bug C** — D2 Materialize span 永远报告 `message_count=0`（observability 盲区）
3. **Bug D** — D7 Verify 节点永远不触发 task_incomplete anomaly（即使 LLM 产生 6 条 ObsUncertainty+CatSystem）
4. **Bug E** — Learn 节点 prior 完全静态化（rule 硬编码 + DefaultDeveloperPrior mean=0.625 固定）

完整排查过程：`/Users/fukai/brain/02知识沉淀/永久笔记/项目/devrix-sess_1782814140202_7000-排查分析.md`

### 1.1 Follow-up Hotfix (2026-06-30 22:38, c155e2da)

用户重测 sess_1782826968112_7000 同样指令 → 飞书卡片仍展示 82 字符 transitional 短语
"Now let me look at the cross-package contracts referenced from the kernel package."

**根因升级**：上一次修复（B + D）的 `summary_quality` 路径能识别 summary 是 transitional，
但 D1 fallback 链回退到 `event.Content` 时，`event.Content` 本身也是同一段 LLM 中途截断的
过渡文本（LLM 在 tool_call 序列中间被打断，stream 最后的 text 是 "Now let me..."）。fallback
span 正确发出（D1_EmitComplete_Fallback span 触发并被 WARN 标 unknown operation），
但用户看到的还是 82 chars 的垃圾。

**Hotfix 改动**（无新增 AC，无架构变化，仅在原 DM-20260630-011 scope 内修复 fallback 链）：

1. **D7 turn_recovery.go emitComplete** 新增 `finalQuality` 入参（`ClassifyLastTextQuality(resolvedFinal)`），
   把 fallback Content 的质量信号写到 `meta["final_quality"]`。
2. **D1 conclusion.EmitComplete** 当 `summary_quality ∈ {too_short, inconclusive}` AND
   `final_quality ∈ {too_short, inconclusive}` → 用 `TaskIncompleteMessage`
   （"（任务未能完成，AI 未产生有效结论。请重新发起。）"）替代 fallback Content；
   metadata 打 `task_incomplete=true` 给 dashboard 告警。
3. **三处遗漏的 span 注册**（comm/contextengine/orchestration 三域 spans.go）补齐，
   移除 `observability: unknown operation` WARN noise。
4. `LayerAndComponent` 增加 `D7_LastText_Quality_Gate` 路由 → orchestration/orchestrator。
5. `registry_test.go` expected list 同步 +3 ops，coverage 87 → 90。

**测试**：2 个新 P0 regression 测试 + 5 个既有测试 1:1 保持 PASS，22 orchestration + 23
communication packages `-race` PASS，`lint-d1-imports.sh` PASS（D1↔orchestration boundary
守住），`coverage.AllOperations()` 测试 PASS。

未走完整 S1-S6（hotfix 路径，per `feedback-devrix-bugfix-skip-openspec.md` 用户明确偏好的
bug-fix 流程），用户飞书验收后再决定是否后续补 archive delta。

## 2. Problem Statement

### 2.1 当前失败链路（4 类独立 silent failure 联动）

| Bug | 文件:行 | 现象 | 当前态 |
|-----|---------|------|--------|
| B | `turn_loop.go:184-188` + `conclusion.go:50-84` | LLM 最后 turn 输出 scope contract template（553 字符）→ 被当 `lastTurnText` → `summary` → meta → 触发 D1 三层 fallback → 飞书卡片显示 scope-contract-recap | summary=scope-contract template, Content=full transcript 75K, fallback chain 静默触发 |
| C | `workitem_executor.go:431` | `EmitContextMaterialize` 永远传 `0, 0` 给 span attribute；Materialize 返回值从未回填 | Jaeger 15 次 Materialize span 全 `message_count=0`，observability 失明 |
| D | `anomaly.go:33-51` + `uncertainty_report.go:79-89` | AnomalyKind 只有 cat_system_aggregate（+ 4 个 v6.1 reserved），Partition 只收 ObsDeviation，Verify 永远不触发 task_incomplete | 6 条 ObsUncertainty+CatSystem 全被静默丢弃，`D7_System_Anomaly_Detect × 16 triggered=false` |
| E | `orchestrator.go:388-394` + `adaptive_prior.go:80-92` | `learn.classifier_source="rule"` 硬编码；BuildAdaptivePrior 只 Bayesian merge DefaultPrior + ReputationEvidence，不读 UncertaintyReport | `learn.prior.mean=0.625` 16 rounds 不变；LLM 任何 uncertainty 输出不影响 Learn |

### 2.2 影响

- **用户可见**：飞书回复卡片显示重复/不相关内容；用户无法判断 devrix 是否真的执行了 review 任务
- **运维不可见**：4 处 silent failure 都不会产生告警 / 错误日志 / metric counter；只能靠 Jaeger 人工追溯
- **下游失明**：D6 Evolution judge、D5 Coverage HealthCheck 无法识别这些 silent failure 模式

## 3. Proposed Solution

**核心思路**：把 4 类 silent failure 全部转化为 **observability-first 结构化失败信号**，让 D5 Coverage + D6 Evolution 兜底，而不是调整阈值打补丁。

### 3.1 修复 Bug B — LastTextQualityGate + EmitCompleteFallback 显式化

- 新增 `D7_LastText_Quality_Gate` span：`finalizeLoop` 对 `resolvedSummary` 做 3 类判定（`summary_valid` / `summary_too_short` / `summary_inconclusive`）
- D1 `EmitComplete` 在 `summary==""` 时不再静默 fallback 到 `event.Content`；改走 `D1_EmitComplete_Fallback` span + content=`stats + exit_reason=natural_but_no_summary` 红字提示
- 配套：D7 turn_loop 区分"工作中输出"与"总结输出"（按 stop_reason + LLM tag 检测 scope_contract template）

### 3.2 修复 Bug C — Materialize span 回填 + EmptyYield 告警

- `EmitContextMaterialize` 在 `Materializer.Materialize` 调用**之后**用 `mat.Messages` / `mat.TokenEstimate` 回填 span attribute（通过闭包或 `span.SetAttributes`）
- 新增 `D2_Materialize_EmptyYield` span：`len(mat.Messages)==0 && mat.TokenEstimate==0` 时触发（kind=`empty_yield` + wi_id + materialize.policy）
- 这样 Jaeger 才能真正区分"Materialize 真返回 0 条消息"vs"instrumentation 没回填"

### 3.3 修复 Bug D — 新增 2 个 AnomalyKind + Partition 兜底

- `orchtypes/uncertainty_report.go` `Partition()` 在 `o.Kind == ObsUncertainty && o.Strength >= 0.7` 时把 observation 加入 `Anomalies`
- `executionflow/verify/anomaly.go` 新增 `AnomalyKindTaskIncomplete` + `AnomalyKindEmptyConclusion` 常量 + detector 规则
- 触发条件：`FilterByKind(ObsUncertainty) len ≥ 2 && avg(strength) ≥ 0.7`
- triggered 时 emit `D7_Anomaly_Trigger` span 带 kind=新值，verdict 自动 fail-closed

### 3.4 修复 Bug E — IntentClassifier 接口扩展 + prior dynamic override

- `IntentClassification` 新增 `Source` 字段（`SourceRule` / `SourceLLM` / `SourceHybrid`）
- `IntentClassifier` 接口**不变更现有签名**，新增 `ClassifyWithReport(ctx, message, prior, report)` overload
- `RuleClassifier` 默认实现 `ClassifyWithReport` = `ClassifyWithPrior` + `SourceRule` 标记
- `BuildAdaptivePrior(rep, trackMode, report)` 新增第三参数（向后兼容通过 overload）
- 公式：`prior = BetaPrior{Alpha: max(1, base.Alpha - penalty), Beta: base.Beta + penalty}`，`penalty = sum(uncertainty_strength)`
- Floor：`prior.Mean >= 0.1`（避免 ReputationEvidence merge 跌破）

## 4. Alternatives Considered

| 方案 | 结论 |
|------|------|
| A. 调阈值（短 summary 阈值 / materialize message count 阈值 / uncertainty strength 阈值） | ❌ 用户已确认 threshold-based 设计是 anti-pattern（feedback-threshold-design-antipattern.md）；数值调优是结构性错误 |
| B. 仅补 observability（emit span 但不修行为） | ❌ 不修 fallback chain + Verify anomaly → 用户仍看到错误结果；本次只增信号不修链路，治标不治本 |
| C. 引入 LLM-as-Judge 评估 summary 质量（每次 complete 调 LLM 判分） | ❌ 增 RT/成本；当前 Rule + structural quality gate 已能覆盖 95% case |
| **D. observability-first + 结构性修复（选用）** | ✅ 4 bug 各加 1 个 observability span + 1 处结构性修复；E2E 回归测试覆盖（AC8） |

## 5. Capabilities

| Capability | L1 | Change |
|------------|-----|--------|
| d7-orchestration | D7 | MODIFY finalizeLoop + ClassifyWithReport overload; ADD LastTextQualityGate span + 2 AnomalyKind |
| d1-communication | D1 | MODIFY EmitComplete fallback chain; ADD EmitCompleteFallback span |
| d2-context-engine | D2 | MODIFY EmitContextMaterialize 回填; ADD MaterializeEmptyYield span |
| d5-observability | D5 | MODIFY Operation Registry 增加 5 新 op (D7_LastText_Quality_Gate / D1_EmitComplete_Fallback / D2_Materialize_EmptyYield / D7_Anomaly_Trigger 已存在 + 2 新 kind) |
| d6-evolution | D6 | (no code change) 新 silent failure 模式自动被 LLM-as-Judge 探针覆盖 |

## 6. Impact

| Component | Change |
|-----------|--------|
| `hardening/emitter.go` | ADD `EmitLastTextQualityGate` / `EmitEmitCompleteFallback` / `EmitMaterializeEmptyYield` 3 helpers |
| `executionflow/verify/anomaly.go` | ADD 2 AnomalyKind + detector 规则 |
| `orchtypes/intent.go` | ADD `IntentClassification.Source` 字段 |
| `orchtypes/uncertainty_report.go` | MODIFY `Partition()` 增加 ObsUncertainty strength ≥ 0.7 兜底 |
| `mups/learn/prior/adaptive_prior.go` | MODIFY `BuildAdaptivePrior` 签名扩展（overload） |
| `decisionplanning/classifier.go` | ADD `ClassifyWithReport` overload + `RuleClassifier` 默认实现 |
| `sessionorchestrator/turn_recovery.go` | MODIFY `finalizeLoop` 调用 `LastTextQualityGate` |
| `sessionorchestrator/workitem_executor.go` | MODIFY `:431` Materialize span 回填 |
| `sessionorchestrator/orchestrator.go` | MODIFY `:388-394` learn.classifier_source 改 intent.Source |
| `communication/conclusion/conclusion.go` | MODIFY fallback chain 不再静默 |
| `sessionorchestrator/item_pipeline_integration_test.go` | NEW E2E 回归测试（AC8） |
| `verify/anomaly_test.go` | ADD 2 AnomalyKind 测试 |
| `orchtypes/uncertainty_report_test.go` | ADD Partition 兜底测试 |
| `decisionplanning/classifier_with_prior_test.go` | ADD ClassifyWithReport overload 测试 |
| `mups/learn/prior/adaptive_prior_test.go` | ADD BuildAdaptivePrior 三参数测试 |

## 7. Success Criteria

- [ ] AC1：D7_LastText_Quality_Gate span 触发，输出 3 类判定 attribute
- [ ] AC2：D1_EmitComplete_Fallback span 在 summary 空时触发，记录 fallback 链
- [ ] AC3：Materialize span 回填 mat.Messages/TokenEstimate + EmptyYield 告警
- [ ] AC4：Partition() ObsUncertainty strength ≥ 0.7 进 Anomalies + 3 类测试覆盖
- [ ] AC5：新增 2 AnomalyKind + detector 规则 + D7_Anomaly_Trigger span
- [ ] AC6：IntentClassifier 接口扩展 + learn.classifier_source 用 intent.Source
- [ ] AC7：BuildAdaptivePrior 三参数 + prior dynamic override + floor 0.1
- [ ] AC8：E2E 集成测试模拟 sess_1782814140202_7000 5 bug 回归覆盖
- [ ] AC9：不破坏 LP-1/LP-2/LP-5 E2E baseline；CI unit tests 全绿

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| AC4 Partition 修改 ObsUncertainty 进 Anomalies 后让本来不触发的 session 触发 task_incomplete | 高 | Feature flag `conclusion_completeness_v1=false` 默认关闭；Phase 灰度 |
| AC6 IntentClassifier.ClassifyWithReport 在 hot path 上变慢 | 中 | 默认实现 = ClassifyWithPrior（已有性能基线）；新路径仅在 overload 调用时触发 |
| AC7 prior dynamic override 让 DefaultDeveloperPrior mean 暴降至 ≤ 0.2 | 中 | penalty 公式加 floor Mean ≥ 0.1；保留 ReputationEvidence merge 不变 |
| AC3 materialize span 回填改动影响 hardening test baseline | 低 | 复用 hardening bridge nil-safe 模式 + 新增空 mat 单元测试 |
| 跨域改动 (D1+D2+D7) 让 verify-archive.sh 11/11 PASS baseline 飘移 | 低 | 同步更新 d1/d2/d7 specs lite-mode 下追加 change-level delta（参照 DM-20260630-003~009 模式） |

## 9. Out of Scope

1. **Bug A**（飞书卡 dedup 阈值 50 runes 过宽）—— 用户已确认 threshold-based 设计 anti-pattern，需重新设计（按 chunk 类型/stop_reason/event 类型路由）。立独立 Change `devrix-feishu-streaming-dedup-redesign`，**本次不在范围**。
2. Learn node 引入真正 LLM Classifier（hybrid 路径）—— 立独立 Change `devrix-d7-llm-classifier-promotion`，本次只做接口预留（Source 字段 + overload stub）。
3. Materialize workitem 级别缓存 / CoW —— 立独立 Change `devrix-d2-materialize-cache`，本次只做 instrumentation 回填。
4. LLM-as-Judge 评估 summary 质量 —— 立独立 Change `devrix-d6-summary-quality-judge`，本次只做结构化 quality gate。

---

## Archive Information

**Status:** S2_Design (Active)
**Next Step:** S3 design.md 六段式 + tasks.md T 层预登记