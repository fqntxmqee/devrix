# Implementation Tasks: Session 结论完整性 — 4 类 silent failure 修复

**Change ID:** `devrix-session-conclusion-completeness`  
**Demand ID:** DM-20260630-011

---

## Phase 1: OrchTypes 基础 (PR-A)

### Phase 1.1: IntentClassification.Source 字段 (AC6)
- [ ] 1.1.1 `orchtypes/intent.go` 新增 `Source` 字段（`SourceRule` / `SourceLLM` / `SourceHybrid` 枚举）
- [ ] 1.1.2 `decisionplanning/classifier.go` `IntentClassification` 接口扩展 `WithSource(s Source)` 方法

### Phase 1.2: UncertaintyReport.Partition 兜底 (AC4)
- [ ] 1.2.1 `orchtypes/uncertainty_report.go` `Partition()` 在 `o.Kind == ObsUncertainty && o.Strength >= 0.7` 时把 observation 加入 `Anomalies`
- [ ] 1.2.2 `orchtypes/uncertainty_report_test.go` 新增 3 个测试 case（不达标 / 达标+CatBusiness / 达标+CatSystem）

### Phase 1.3: AdaptivePrior overload (AC7)
- [ ] 1.3.1 `mups/learn/prior/adaptive_prior.go` 新增 `BuildAdaptivePrior(rep, trackMode, report)` 三参数 overload
- [ ] 1.3.2 `mups/learn/prior/adaptive_prior_overload.go` (NEW) 实现 overload body（penalty 公式 + floor Mean ≥ 0.1）
- [ ] 1.3.3 `mups/learn/prior/adaptive_prior_test.go` 新增 5 个测试 case（nil report / 1 obs / 6 obs / 边界 / floor）

### Phase 1.4: IntentClassifier.ClassifyWithReport overload (AC6)
- [ ] 1.4.1 `decisionplanning/classifier.go` `IntentClassifier` interface 不变更；新增 `ClassifyWithReport(ctx, message, prior, report)` overload 到 interface 文档
- [ ] 1.4.2 `RuleClassifier.ClassifyWithReport` 默认实现 = `ClassifyWithPrior` + `intent.WithSource(SourceRule)`
- [ ] 1.4.3 `decisionplanning/classifier_with_prior_test.go` 新增 overload 测试

**Quality Gate (PR-A):**
- [ ] `go test -race ./internal/layers/orchestration/orchtypes/... ./internal/layers/orchestration/mups/learn/... ./internal/layers/orchestration/decisionplanning/... -count=1`
- [ ] `go run ./cmd/devrix-layer-lint --root=internal/layers --strict` 通过（无跨层违规）
- [ ] OpenSpec files 完整：`.openspec.yaml` + `demand.md` + `proposal.md` + `design.md` + `tasks.md`

---

## Phase 2: Verify 节点 AnomalyKind + 跨域 wiring (PR-B)

### Phase 2.1: ExecutionFlow Verify 新增 AnomalyKind (AC5)
- [ ] 2.1.1 `executionflow/verify/anomaly.go` 新增 `AnomalyKindTaskIncomplete` + `AnomalyKindEmptyConclusion` 常量
- [ ] 2.1.2 `executionflow/verify/anomaly_kind_incomplete.go` (NEW) 实现 `DetectTaskIncomplete` + `DetectEmptyConclusion` 规则
- [ ] 2.1.3 `executionflow/verify/anomaly_kind_incomplete_test.go` (NEW) 5 个 case 覆盖

### Phase 2.2: Hardening Span Emitters (AC1/AC2/AC3)
- [ ] 2.2.1 `hardening/emitter.go` 新增 `EmitLastTextQualityGate` (sessionID, kind, length, exitReason)
- [ ] 2.2.2 `hardening/emitter.go` 新增 `EmitEmitCompleteFallback` (sessionID, fallbackSource, contentLength, summaryQuality)
- [ ] 2.2.3 `hardening/emitter.go` 新增 `EmitMaterializeEmptyYield` (sessionID, wiID, policy)
- [ ] 2.2.4 3 个 unit test 文件覆盖 (emitter_lasttext_test.go / emitter_emitcomplete_test.go / emitter_materialize_test.go)

### Phase 2.3: D7 finalizeLoop wiring (AC1)
- [ ] 2.3.1 `sessionorchestrator/turn_recovery.go` `finalizeLoop` 调用 `LastTextQualityGate.classify(resolvedSummary)` 后 emit span + 设置 `meta["summary_quality"]`
- [ ] 2.3.2 `LastTextQualityGate` 关键词词典走 `i18n.DefaultKeywords` 路径

### Phase 2.4: D7 workitem_executor span 回填 (AC3)
- [ ] 2.4.1 `sessionorchestrator/workitem_executor.go:431` 修改 `EmitContextMaterialize` 闭包：在 `Materialize()` 之后回填 `materialize.message_count` + `materialize.token_est`
- [ ] 2.4.2 `if len(mat.Messages) == 0 && mat.TokenEstimate == 0` 时 emit `D2_Materialize_EmptyYield` span

### Phase 2.5: D7 orchestrator classifier_source wiring (AC6)
- [ ] 2.5.1 `sessionorchestrator/orchestrator.go:388-394` `learn.classifier_source` 改为 `intent.Source.String()`

### Phase 2.6: D1 EmitComplete fallback 显式化 (AC2)
- [ ] 2.6.1 `communication/conclusion/conclusion.go` `EmitComplete` 在 `summary==""` 时不再静默 fallback 到 `event.Content`；改为 emit `D1_EmitComplete_Fallback` span
- [ ] 2.6.2 `conclusion_test.go` 新增 4 个测试 case（summary_valid/thin/too_short/inconclusive 各自的 fallback 行为）

**Quality Gate (PR-B):**
- [ ] `go test -race ./internal/layers/orchestration/... ./internal/layers/communication/... -count=1`
- [ ] `go run ./cmd/devrix-layer-lint --root=internal/layers --strict` 通过
- [ ] E2E LP-1 baseline 100% 持平

---

## Phase 3: E2E 回归测试 + 规格同步 (PR-C)

### Phase 3.1: E2E 集成测试 (AC8)
- [ ] 3.1.1 `sessionorchestrator/item_pipeline_integration_test.go` (NEW) `TestSession_ConclusionCompleteness_5BugsRegression`
- [ ] 3.1.2 模拟 sess_1782814140202_7000 场景：review 任务 + LLM mock 输出 scope-contract-recap + 6 条 ObsUncertainty
- [ ] 3.1.3 断言 5 项：(a) LastTextQualityGate kind=inconclusive (b) EmitCompleteFallback fallback.source=stats (c) materialize.message_count > 0 (d) AnomalyKindEmptyConclusion triggered=true (e) prior.mean ≤ 0.4

### Phase 3.2: Spec Delta 文档
- [ ] 3.2.1 `openspec/archive/2026-06-30-devrix-session-conclusion-completeness/specs/d7-orchestration/spec.md` (delta ADDED)
- [ ] 3.2.2 `openspec/archive/2026-06-30-devrix-session-conclusion-completeness/specs/d1-communication/spec.md` (delta MODIFIED)
- [ ] 3.2.3 `openspec/archive/2026-06-30-devrix-session-conclusion-completeness/specs/d2-context-engine/spec.md` (delta MODIFIED)

### Phase 3.3: S6 Archive 准备
- [ ] 3.3.1 `acceptance-report.md` 编写（PR-C merge 后）
- [ ] 3.3.2 `openspec/archive/2026-06-30-devrix-session-conclusion-completeness/` 目录结构搭建
- [ ] 3.3.3 `demand-archive-index.md` 追加 DM-20260630-011 行

**Quality Gate (PR-C):**
- [ ] E2E LP-1/LP-2/LP-5 baseline 100% 持平
- [ ] `verify-archive.sh` PASS
- [ ] CI `unit tests` 全绿

---

## Phase 4: 提交与归档 (S6-交付 + S6-归档)

### Phase 4.1: PR 创建 + Auto-merge
- [ ] 4.1.1 `git push -u origin feat/devrix-session-conclusion-completeness`
- [ ] 4.1.2 `gh pr create --base master --title "feat(d1+d2+d7): session conclusion completeness — fix 4 silent failures"`
- [ ] 4.1.3 `gh pr merge <N> --auto --squash`
- [ ] 4.1.4 盯 `gh pr checks <N>` 至 `unit tests` PASS

### Phase 4.2: S6 Archive
- [ ] 4.2.1 把 `openspec/changes/devrix-session-conclusion-completeness/` 移到 `openspec/archive/2026-06-30-devrix-session-conclusion-completeness/`
- [ ] 4.2.2 `.openspec.yaml` status: `s7_archived`
- [ ] 4.2.3 更新各域 spec.md (主分支) lite-mode 加 change-level delta (refer to DM-20260630-003~009)

### Phase 4.3: 用户验收
- [ ] 4.3.1 `cd /Users/fukai/workspace/devrix && ./scripts/devrix.sh build`
- [ ] 4.3.2 `./scripts/devrix.sh restart`
- [ ] 4.3.3 飞书发同样指令 "review d2 领域 kernel 代码" 验证 4 类 silent failure 修复

---

## Completion Checklist

- [ ] Phase 1 orchtypes/verify/prior 改动完成
- [ ] Phase 2 跨域 wiring 完成
- [ ] Phase 3 E2E 回归 + spec delta 完成
- [ ] Phase 4 PR 合入 + archive 完成
- [ ] 重启 devrix 后用户飞书验收通过
- [ ] `verify-archive.sh` 12/12 PASS
- [ ] `demand-archive-index.md` DM-20260630-011 行已追加