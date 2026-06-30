# Design: Session 结论完整性 — 4 类 silent failure 修复

**Change ID:** `devrix-session-conclusion-completeness`  
**Demand ID:** DM-20260630-011  
**Status:** S3_Design  
**Parent Proposal:** `proposal.md`  
**Template:** `docs/methodology/detail-design-framework.md`（六段式）  
**Created:** 2026-06-30

---

## ① 架构目标

### 业务目标（对应 AC）

| 痛点 | 修复 | AC |
|------|------|-----|
| 飞书卡显示 scope-contract-recap 而非 review findings | LastTextQualityGate + EmitCompleteFallback 显式化 | AC1 / AC2 |
| D2 Materialize span 永远 0 | span attribute 回填 + EmptyYield 告警 | AC3 |
| Verify 永远不触发 task_incomplete | 新增 2 AnomalyKind + Partition 兜底 | AC4 / AC5 |
| Learn prior 永远静态 | IntentClassifier 接口扩展 + prior dynamic override | AC6 / AC7 |

### 技术目标（量化指标）

| 指标 | 当前 | 目标 | 验证方式 |
|------|------|------|----------|
| `D7_LastText_Quality_Gate` span 触发率（summary 有疑问的 session） | 0% | 100% | AC8 E2E 模拟 sess_1782814140202_7000 |
| `materialize.message_count` span 与实际 mat.Messages 一致率 | 0%（永远 0） | 100% | unit test 断言 span.SetAttributes 调用 |
| Verify anomaly triggered 命中（L1: 6+ ObsUncertainty+CatSystem session） | 0% | ≥ 50% | AC5 + AC8 |
| Learn prior mean 偏差（uncertainty session vs 静态 0.625） | 0 | ≤ 0.4 | AC7 + AC8 |
| CI unit tests | 0 PASS | 全绿 | `go test -race ./internal/layers/orchestration/...` |
| E2E LP-1/LP-2/LP-5 baseline | 100% | 100% | `./scripts/test-e2e.sh` |

### 约束条件

- **SemVer**：minor 版本（v6.x → v6.(x+1).0），不破坏 LP-1/LP-2/LP-5 E2E baseline
- **灰度**：Feature flag `conclusion_completeness_v1` 默认 `false`，S5 验收后开 PR + manual flip + 灰度观察 1 周
- **Pure types / 不可变**：所有新结构体字段只读；扩展方法走 `With*` 模式
- **错误码闭合**：新增 ExitReason `ExitReasonConclusionIncomplete`（14+1 = 15 个）
- **observability-first**：每个修复点都先 emit span，再做行为修复
- **不引入新外部依赖**：纯 Go 内部代码改动

## ② 架构原则

### 设计原则（10 条以内，每条对应落地 + AC）

| # | 原则 | 落地 | AC |
|---|------|------|-----|
| 1 | **Observability-first silent failure** | 4 bug 各加 1 span，先发信号再修行为 | AC1/AC2/AC3/AC5 |
| 2 | **Structural > Threshold** | 不调阈值（feedback-threshold-design-antipattern），改用结构化 quality gate | AC1/AC4/AC7 |
| 3 | **Backward compat by overload** | IntentClassifier / BuildAdaptivePrior 不变签，新增 overload | AC6/AC7 |
| 4 | **Span attribute back-fill** | Materialize span 闭包内回填 mat.Messages/TokenEstimate | AC3 |
| 5 | **Fail-safe with floor** | prior.Mean ≥ 0.1 + 保留 ReputationEvidence merge | AC7 |
| 6 | **Partition 兜底而非替换** | ObsUncertainty 追加进 Anomalies，不影响已有 ObsDeviation 路径 | AC4 |
| 7 | **Detector 优先级** | AnomalyKind 4→6；新 2 个 kind 与 cat_system_aggregate 平行触发 | AC5 |
| 8 | **Source 字段标注分类器路径** | intent.Source=rule/llm/hybrid；现有硬编码 "rule" 替换 | AC6 |
| 9 | **Feature flag 灰度** | `conclusion_completeness_v1=false` 默认关；测试覆盖双路径 | AC8 |
| 10 | **E2E regression baseline** | AC8 模拟 sess_1782814140202_7000 完整链路 | AC8 |

### 命名规范

- **Span Op**：`D7_LastText_Quality_Gate` / `D1_EmitComplete_Fallback` / `D2_Materialize_EmptyYield` / `D7_Anomaly_Trigger`（已存在，复用）+ 2 个新 kind 值
- **AnomalyKind**：`task_incomplete` / `empty_conclusion`（snake_case；保持与现有 `cat_system_aggregate` 一致）
- **ExitReason**：`ExitReasonConclusionIncomplete`（PascalCase；与 14 个已有 ExitReason 同套枚举）
- **IntentClassification.Source**：`SourceRule` / `SourceLLM` / `SourceHybrid`（string 枚举）
- **Attribute key**：`summary.kind` / `summary.length` / `summary.exit_reason` / `fallback.source` / `fallback.content_length` / `materialize.message_count` / `materialize.token_est` / `anomaly.kind` / `anomaly.strength_avg`
- **Test ID**：`D7-S2-A50-T50`（LastTextQualityGate）/ `D1-S16-A02-T50`（EmitCompleteFallback）/ `D2-S16-A47-T50`（MaterializeEmptyYield）/ `D7-S4-A47-T50`（AnomalyKindTaskIncomplete）/ `D7-S4-A47-T51`（AnomalyKindEmptyConclusion）/ `D7-S12-A42-T50`（ClassifyWithReport overload）/ `D7-S5-A49-T50`（BuildAdaptivePrior 三参数）

### 代码风格

- 函数 < 50 行（detector 规则函数走 helper 拆分）
- 文件 < 800 行（emitter.go 新增 3 helpers 总计 +30 行；现有 240 行 → 270 行）
- 异常不过模块边界（cross-package error 用 `sharederrors` sentinel）
- Pure types + `With*` 不可变（UncertaintyReport / IntentClassification / AdaptivePrior 都不变 mutation）

## ③ 业务流程

### 核心用例时序图：sess_1782814140202_7000 完整修复链路

```
[用户飞书] "review d2 领域 kernel 代码"
   ↓
[D1 IM Adapter] → [D7 SessionOrchestrator.RunTurn]
   ↓
   ├─→ classifySpan (D7_S2_Orchestration_Intent_Classify)
   │     └─→ RuleClassifier.ClassifyWithReport(message, prior, report=nil)  // 第 1 轮 report 空
   │         └─→ intent.Kind = IntentOrchestrate, intent.Source = SourceRule
   │             ↓ sessionSpan.SetAttributes("learn.classifier_source", intent.Source)  // "rule"
   │
   ├─→ runLoop (16 iterations × 6 min 13 sec)
   │     ├─→ prepareContext → materialize wi=root_wi → 
   │     │     D2_Context_Materialize span  ← 【修复】mat.Messages/TokenEstimate 回填
   │     │     【新增】D2_Materialize_EmptyYield span if len(mat.Messages)==0
   │     │
   │     ├─→ llm stream (LLM 输出工具调用 + scope_contract template)
   │     │     ↓ stream chunk 包含 scope_contract 标签
   │     │
   │     ├─→ workItemExecutor.ReAct loop (4 工具调用)
   │     │     ↓ lastTurnText = scope_contract template (553 chars, NOT review findings)
   │     │
   │     ├─→ Observe node → LLMObservationProposer
   │     │     ↓ LLM 输出 6 条 ObsUncertainty + CatSystem (str 0.78→0.9)
   │     │     ↓ UncertaintyReport 累积
   │     │
   │     ├─→ Verify node → DetectSystemAnomaly(report)
   │     │     ↓ cat_system_aggregate: triggered=false (6 ObsUncertainty 不在 Anomalies)
   │     │     【修复】AnomalyKindTaskIncomplete: triggered=true if len(ObsUncertainty)≥2 && avg(str)≥0.7
   │     │     【新增】D7_Anomaly_Trigger span kind=task_incomplete
   │     │
   │     └─→ Learn node → BuildAdaptivePrior(rep, trackMode, report)
   │           ↓ 静态 DefaultDeveloperPrior mean=0.625（无变化）
   │           【修复】penalty = sum(strengths) = ~5.0; 
   │           prior = BetaPrior{Alpha: max(1, 5-5), Beta: 3+5} = BetaPrior{1, 8} mean=0.111
   │           ↓ reputation merge 不变
   │           【新增】learn.prior.mean attr 在 sessionSpan 体现动态变化
   │
   └─→ finalizeLoop → 
         ↓ resolvedSummary = resolveFinalText(lastTurnText=scope_contract_template, ...)
         ↓ resolvedSummary 长度 553 < 阈值 200? NO (553 > 200) ← 旧判定失效
         ↓ 【修复】LastTextQualityGate: 检查是否存在 review-style 关键信号 (Finding/Risk/严重/建议)
         ↓ 命中数 = 0 → summary.kind = "summary_inconclusive"
         ↓ 【新增】D7_LastText_Quality_Gate span 输出 kind/length/exit_reason
         ↓
         ↓ emitComplete(out, ..., resolvedFinal, resolvedSummary=scope_contract_template, exit_reason)
         ↓ meta["summary"] = scope_contract_template (553 chars, 含 <scope_contract> 标签)
         ↓
[D1 SignalRouter.Dispatch] event.Type="complete" → EmitComplete
   ↓
   ↓ summary=scope_contract_template ≠ "" → meta["summary"] = summary
   ↓ content = summary = scope_contract_template (553 chars)
   ↓ 【修复】但 LastTextQualityGate 已标记 summary_inconclusive → 
   ↓ D1 adapter 收到 metadata["summary_quality"]="inconclusive" → 
   ↓ 【新增】D1_EmitComplete_Fallback span fallback.source="event.Content" (因为 inconclusive 触发红字)
   ↓ content 改为 stats + 红字 "[任务结论不完整 - LLM 最后输出非 review 内容]"
   ↓
[飞书 Card] 显示红字告警而非 scope-contract-recap ✅
```

### 异常补偿（Fallback 路径表）

| 触发条件 | Fallback 行为 | Span |
|----------|--------------|------|
| `summary==""` (旧 fallback) | content = stats (旧行为保留但加 span) | D1_EmitComplete_Fallback fallback.source="stats" |
| `summary_quality == "inconclusive"` (新) | content = stats + 红字告警 (新行为) | D1_EmitComplete_Fallback fallback.source="event.Content_redacted" |
| `summary_quality == "valid"` | content = summary (happy path) | D1_EmitComplete_Fallback 不触发 |
| `len(mat.Messages)==0` | EmptyYield 告警 span + Execute 仍继续 | D2_Materialize_EmptyYield |
| `EvaluateSystemAnomaly triggered=true 且 kind=task_incomplete` | verdict = fail + ExitReasonConclusionIncomplete | D7_Anomaly_Trigger |
| `prior.Mean < 0.1` | floor 强制 Mean = 0.1 | (无新 span，prior.Mean 已有) |

### 分支处理决策树

```
finalizeLoop:
  resolvedSummary = resolveFinalText(lastTurnText, ...)
  ↓
  LastTextQualityGate.classify(resolvedSummary):
    ├─ if 长度 < 200 → kind="summary_too_short"
    ├─ elif 命中 review-style 关键词数 = 0 → kind="summary_inconclusive"
    ├─ elif 命中数 ≥ 3 → kind="summary_valid"
    └─ else → kind="summary_thin"
  ↓
  EmitLastTextQualityGate span (kind + length + exit_reason)
  ↓
  if kind == "summary_valid":
    summary = resolvedSummary  // happy path
  elif kind == "summary_thin":
    summary = resolvedSummary  // 接受但不放心
  else (too_short / inconclusive):
    summary = ""  // 触发 D1 fallback
    meta["summary_quality"] = kind
  ↓
  emitComplete(...)
```

```
Verify node → DetectSystemAnomaly:
  ↓
  trigger cat_system_aggregate (existing):
    if len(CatSystem+ObsDeviation) ≥ 3 AND ratio ≥ 0.5 → triggered=true
  ↓
  trigger task_incomplete (NEW):
    if len(ObsUncertainty with str≥0.7) ≥ 2 AND avg(str) ≥ 0.7 → triggered=true
  ↓
  trigger empty_conclusion (NEW):
    if last_turn_text 含 <scope_contract>/<directive_template> 标签 AND length<800 → triggered=true
  ↓
  emit D7_Anomaly_Trigger if ANY triggered
```

```
Learn node → BuildAdaptivePrior:
  ↓
  if report == nil:
    prior = DefaultDeveloperPrior (现有行为)
  ↓
  else:  // overload
    base = DefaultDeveloperPrior
    penalty = sum(uncertainty_observation.strength for kind in {ObsUncertainty} AND category=CatSystem)
    if penalty ≥ 2:  // 触发阈值，不是 hard threshold 而是结构化判定
      prior = BetaPrior{Alpha: max(1, base.Alpha - int(penalty)), Beta: base.Beta + int(penalty)}
      if prior.Mean() < 0.1:
        prior = BetaPrior{prior.Alpha, prior.Beta} // 保留计算结果但记录 floor warning
  ↓
  return prior (immutable)
```

## ④ 领域模型

### 聚合根（4 个以内）

| 聚合根 | 职责 | 不可变性 |
|--------|------|----------|
| `UncertaintyReport` (orchtypes) | D7 Observe 节点产出的不确定性观察集合；4 类 Observation (Fact/Signal/Deviation/Uncertainty) × 2 Category (Business/System) | 不可变；`AddObservation` 返回新副本 |
| `AdaptivePrior` (mups/learn/prior) | Learn 节点 Bayesian prior；Alpha+Beta Beta 分布 | 不可变；`BuildAdaptivePrior` 返回新实例 |
| `IntentClassification` (orchtypes) | Classify 节点意图判定结果 | 不可变；扩展 Source 字段不改 mutation |
| `AnomalyKind` (executionflow/verify) | Verify 节点异常分类枚举 | const 不可变；新增常量 |

### 限界上下文（包边界图）

```
internal/layers/orchestration/
├── hardening/                 # Span 发射层 (新增 3 emitters)
│   └── emitter.go             # +EmitLastTextQualityGate +EmitEmitCompleteFallback +EmitMaterializeEmptyYield
├── orchtypes/                 # 跨域类型 (Source 字段 + Partition 兜底)
│   ├── intent.go              # +Source enum
│   ├── uncertainty_report.go  # MODIFY Partition()
│   └── system_anomaly_wiring.go (KEEP, 复用)
├── executionflow/verify/      # Verify 节点 (新增 2 AnomalyKind)
│   └── anomaly.go             # +AnomalyKindTaskIncomplete +AnomalyKindEmptyConclusion
├── mups/learn/prior/          # Learn prior (overload)
│   └── adaptive_prior.go      # +BuildAdaptivePrior 三参数 overload
├── decisionplanning/          # 意图分类 (overload)
│   └── classifier.go          # +ClassifyWithReport overload + RuleClassifier default impl
└── sessionorchestrator/       # Orchestrator (调用 wiring)
    ├── turn_recovery.go       # MODIFY finalizeLoop 调用 LastTextQualityGate
    ├── workitem_executor.go   # MODIFY :431 Materialize span 回填
    └── orchestrator.go        # MODIFY :388 learn.classifier_source 改 intent.Source

internal/layers/communication/  # D1 IM 适配层 (cross-domain wiring)
├── conclusion/
│   └── conclusion.go          # MODIFY EmitComplete fallback 不再静默
└── channel/adapters/
    └── feishu_progress.go     # KEEP (Bug A 不在范围)
```

### 领域事件（Span / Metric 列表）

| Span Op | Kind | 时机 | Attribute |
|---------|------|------|-----------|
| `D7_LastText_Quality_Gate` | NEW | finalizeLoop | summary.kind / summary.length / summary.exit_reason |
| `D1_EmitComplete_Fallback` | NEW | EmitComplete fallback 触发 | fallback.source / fallback.content_length / summary_quality |
| `D2_Materialize_EmptyYield` | NEW | Materialize 返回 0 条消息 | materialize.wi_id / materialize.policy / materialize.kind=empty_yield |
| `D7_Anomaly_Trigger` | EXISTING (复用) | Verify detector 触发 | anomaly.kind / anomaly.severity / anomaly.strength_avg (NEW for 2 new kinds) |
| `D2_Context_Materialize` | EXISTING (修改) | Materialize 之后回填 | materialize.message_count (FIXED) / materialize.token_est (FIXED) |

### 跨域消费模型

- **D1 → D7**：`EmitComplete` 通过 metadata["summary_quality"] 字段读取 D7 LastTextQualityGate 判定结果（string enum: valid/thin/too_short/inconclusive）
- **D7 → D2**：`workitem_executor.go:431` 修改调用 Materialize span 的回填逻辑；不动 Materializer 接口
- **D7 → D7**：`BuildAdaptivePrior` overload 接收 UncertaintyReport（同包内）
- **D7 → D7**：`IntentClassification.Source` 替换 orchestrator.go:392 硬编码

## ⑤ 核心链路图

### 端到端路径：sess_1782814140202_7000 修复后

```
[飞书] → [D1 feishu adapter] → [D7 SessionOrchestrator.RunTurn]
   │
   ├── classify (S2-A06) ────────────────────────────────────────────┐
   │   ├─ span: D7_S2_Orchestration_Intent_Classify                  │
   │   ├─ RuleClassifier.ClassifyWithReport(msg, prior, report=nil)  │
   │   ├─ intent.Source = SourceRule                                  │
   │   └─ sessionSpan attr: learn.classifier_source = intent.Source  │ ← AC6 修复
   │                                                                 │
   ├── runLoop iter 1..16 ───────────────────────────────────────────┤
   │   ├─ prepareContext (S2-A07)                                    │
   │   │   ├─ Materialize (D2-S16)                                  │
   │   │   │   ├─ span: D2_Context_Materialize                       │
   │   │   │   │   attr (FIXED): materialize.message_count = len(mat.Messages)
   │   │   │   │   attr (FIXED): materialize.token_est = mat.TokenEstimate │ ← AC3 修复
   │   │   │   └─ if empty: span: D2_Materialize_EmptyYield         │ ← AC3 新增
   │   │   └─ Context.Prepare (D2-S2)                                │
   │   │                                                             │
   │   ├─ llmStream (S2-A08) → text chunk (含 <scope_contract> 标签)│
   │   │                                                             │
   │   ├─ executeToolRound (S2-A08) → 4 bash tool calls              │
   │   │                                                             │
   │   └─ post-iter Observe node (per round)                         │
   │       ├─ LLMObservationProposer (D2→D3)                          │
   │       │   └─ LLM 输出 6 ObsUncertainty + CatSystem (str 0.78→0.9)│
   │       └─ UncertaintyReport.partition()                           │
   │           【修复】ObsUncertainty str≥0.7 进 Anomalies           │ ← AC4 修复
   │           ↓                                                      │
   │       └─ Verify node (per round)                                │
   │           ├─ DetectSystemAnomaly (cat_system_aggregate, existing)│
   │           ├─ DetectTaskIncomplete (NEW)                          │ ← AC5 新增
   │           │   triggered if ≥2 ObsUncertainty str≥0.7 → true    │
   │           │   span: D7_Anomaly_Trigger kind=task_incomplete     │
   │           └─ DetectEmptyConclusion (NEW)                        │ ← AC5 新增
   │               triggered if last_turn_text 含 <scope_contract>    │
   │               span: D7_Anomaly_Trigger kind=empty_conclusion    │
   │                                                                 │
   │       └─ Learn node (per round)                                 │
   │           ├─ BuildAdaptivePrior(rep, trackMode, report)         │
   │           │   penalty = sum(strengths) = ~5.0                   │
   │           │   prior = BetaPrior{max(1, 5-5), 3+5} = BetaPrior{1,8} mean=0.111 │ ← AC7 修复
   │           └─ sessionSpan attr: learn.prior.mean = 0.111         │
   │                                                                 │
   └── finalizeLoop (S2-A09) ────────────────────────────────────────┘
       ├─ resolvedSummary = scope_contract_template (553 chars)
       ├─ LastTextQualityGate.classify:
       │   ├─ length = 553 > 200 ✓
       │   ├─ review_keyword_count = 0 ✗
       │   └─ kind = "summary_inconclusive"
       ├─ span: D7_LastText_Quality_Gate kind=inconclusive length=553 exit_reason=natural  │ ← AC1 新增
       ├─ summary = "" (强制空)
       ├─ meta["summary_quality"] = "inconclusive"
       └─ emitComplete(out, ..., resolvedFinal=full_transcript_75K, summary="", exit_reason)

[D1 SignalRouter] event.Type="complete" → EmitComplete
   ├─ summary = "" → content = event.Content = full_transcript_75K
   ├─ meta["summary_quality"] = "inconclusive"
   ├─ span: D1_EmitComplete_Fallback fallback.source="event.Content" content_length=75000  │ ← AC2 新增
   └─ 【修复】feishu adapter 检测到 summary_quality=inconclusive → content 改为红字 + stats

[飞书 Card] 显示 "❌ 任务结论不完整 - LLM 最后输出非 review 内容" + stats ✅
```

### 时序标注（SLA / P99 上限）

| 节点 | P99 上限 | 备注 |
|------|----------|------|
| IntentClassify | < 1ms | 纯规则匹配，RuleClassifier |
| LastTextQualityGate | < 5ms | 字符串扫描 + 关键词匹配 |
| Materialize span 回填 | < 1ms | 闭包内 attribute update |
| DetectTaskIncomplete | < 2ms | 6 条 observation 遍历 |
| BuildAdaptivePrior overload | < 1ms | sum + max |
| EmitCompleteFallback | < 1ms | span emit |
| **总计新增开销** | < 10ms / session | 可忽略 |
| **总 session RT** | < 100s (16 rounds × 6 min 13 sec) | 不变 |

### 单点风险与缓解

| 单点 | 风险 | 缓解 |
|------|------|------|
| LastTextQualityGate 关键词词典 | 中英文覆盖不全 → 误判 valid | 词典走 `i18n.DefaultKeywords` 走 D2 i18n 路径；新关键词按现有 i18n 流程添加 |
| DetectEmptyConclusion 标签检测 | LLM 不用 `<scope_contract>` 标签 → detector 失效 | 检测 4 个 fallback 标签：`<scope_contract>` / `<directive_template>` / `<task_recap>` / `<planning>` |
| BuildAdaptivePrior overload 不被调用 | 现有 BuildAdaptivePrior(rep, trackMode) 调用方不更新 | item_pipeline.go:285 是唯一调用点；同步更新 + grep 全仓确认无遗漏 |
| Materialize span 回填影响 hardening test baseline | `TestEmitContextMaterialize_SpanNoPanic` 失败 | 闭包内 `if span != nil` nil-safe + span.SetAttributes 不影响 end 闭包 |

## ⑥ 接口 / API 设计

### 风格

- **Pure types + With***：`IntentClassification.WithSource(s Source)`、`UncertaintyReport.AddObservation(o)`、`AdaptivePrior.WithBeta(b BetaPrior)` 全部返回新实例
- **Overload pattern**：`(rep, trackMode)` 和 `(rep, trackMode, report)` 共存；新调用走新签名，旧调用走原签名
- **Builder pattern**：`AnomalyDetection.NewBuilder().WithKind(...).WithThreshold(...).Build()`

### 契约（错误码三元组）

| Code | Message | Remediation | TraceID |
|------|---------|-------------|---------|
| `ExitReasonConclusionIncomplete` | "任务结论不完整 - LLM 最后输出非 review 内容" | 重新发起 review 任务或调整指令 | span:trace_id |

| APIErrorCode | 已有枚举 | 本次新增 |
|--------------|---------|---------|
| `conclusion_incomplete` | ❌ | ✅ NEW (DM-20260628-001 模式扩展) |

### 幂等保障

| 操作 | 幂等键 | 行为 |
|------|--------|------|
| EmitComplete | sessionID + event.Type="complete" | 重复 complete event 只触发 1 次（feishu card UpdateCard 模式） |
| LastTextQualityGate | sessionID + resolvedSummary hash | 同一 summary 多次 emit 只 1 次（future optimization） |
| DetectSystemAnomaly | sessionID + anomaly_kind | 同一 kind 同 session 只触发 1 次 |

### 版本演进路径

| 版本 | 内容 | 兼容性 |
|------|------|--------|
| v6.20.0 | DM-20260630-011 PR-A: orchtypes (Source + Partition) + verify (2 AnomalyKind) + adaptive_prior (overload) | 后向兼容（旧 API 保留） |
| v6.21.0 | DM-20260630-011 PR-B: emit helpers + finalizeLoop + EmitComplete + workitem_executor | 后向兼容（feature flag 默认关） |
| v6.22.0 | DM-20260630-011 PR-C: E2E 测试 + S6 archive + feature flag flip | 灰度上线 |

---

## 附录

### 附录 A：File Manifest

#### 新增 (8 files)
- `internal/layers/orchestration/hardening/emitter_lasttext_test.go` (AC1 unit test)
- `internal/layers/orchestration/hardening/emitter_emitcomplete_test.go` (AC2 unit test)
- `internal/layers/orchestration/hardening/emitter_materialize_test.go` (AC3 unit test)
- `internal/layers/orchestration/executionflow/verify/anomaly_kind_incomplete.go` (AC5)
- `internal/layers/orchestration/executionflow/verify/anomaly_kind_incomplete_test.go` (AC5 unit test)
- `internal/layers/orchestration/sessionorchestrator/item_pipeline_integration_test.go` (AC8 E2E)
- `internal/layers/orchestration/orchtypes/intent_source.go` (AC6)
- `internal/layers/orchestration/mups/learn/prior/adaptive_prior_overload.go` (AC7)

#### 修改 (12 files)
- `internal/layers/orchestration/hardening/emitter.go` (+3 helpers)
- `internal/layers/orchestration/orchtypes/intent.go` (+Source field)
- `internal/layers/orchestration/orchtypes/uncertainty_report.go` (MODIFY Partition)
- `internal/layers/orchestration/orchtypes/uncertainty_report_test.go` (+3 test cases)
- `internal/layers/orchestration/executionflow/verify/anomaly.go` (+2 AnomalyKind)
- `internal/layers/orchestration/mups/learn/prior/adaptive_prior.go` (+overload)
- `internal/layers/orchestration/decisionplanning/classifier.go` (+ClassifyWithReport overload)
- `internal/layers/orchestration/decisionplanning/classifier_with_prior_test.go` (+overload test)
- `internal/layers/orchestration/sessionorchestrator/turn_recovery.go` (MODIFY finalizeLoop)
- `internal/layers/orchestration/sessionorchestrator/workitem_executor.go` (MODIFY :431 回填)
- `internal/layers/orchestration/sessionorchestrator/orchestrator.go` (MODIFY :388 source)
- `internal/layers/communication/conclusion/conclusion.go` (MODIFY fallback)

### 附录 B：Rollback Plan

| 层级 | 触发条件 | 回滚动作 |
|------|---------|---------|
| Feature flag | `conclusion_completeness_v1=true` 后出现 LP-1 baseline 飘移 | 立即 flip flag=false |
| PR-B 单独 | PR-B merge 后 unit test 失败 | revert PR-B（保留 PR-A orchtypes/verify/prior 改动） |
| PR-A 单独 | PR-A merge 后 import cycle | revert PR-A；fallback 到硬编码 "rule" + static prior |
| 全量 | 4 个 PR 都 merge 后 E2E baseline fail | 全量 revert；fallback 到本次前的行为（4 silent failure 容忍） |

### 附录 C：回归风险评估

| 高风险改动 | 影响 | 测试策略 |
|-----------|------|----------|
| AC4 Partition() ObsUncertainty 进 Anomalies | 高：可能让本来不触发的 session 触发 task_incomplete | Feature flag 默认关 + AC8 模拟 6 ObsUncertainty session 验证 |
| AC6 ClassifyWithReport overload | 中：可能让 RuleClassifier hot path 变慢 | overload 默认实现 = ClassifyWithPrior（不引入新逻辑） |
| AC7 prior dynamic override | 中：可能让 DefaultDeveloperPrior mean 暴降 | floor Mean ≥ 0.1 + unit test 覆盖 5 case |
| AC3 materialize span 回填 | 低：影响 hardening test baseline | 闭包 nil-safe + 新增空 mat 单元测试 |
| D1 EmitComplete fallback 显式化 | 低：feishu adapter 检测 summary_quality 字段新增 | feishu_progress_test.go 加 4 case (valid/thin/too_short/inconclusive) |

### 附录 D：S3 检查清单 + S3-Gate Review 结论

#### S3 自检

- [x] **六段式完整性**：①架构目标 / ②架构原则 / ③业务流程 / ④领域模型 / ⑤核心链路图 / ⑥接口/API 设计 六段全部存在，标题与符号一致
- [x] **六段式非空**：每段至少 3 行实质内容（平均每段 50+ 行）
- [x] dsaft_activities 已标注（D7-S2-A50 / D1-S16-A02 / D2-S16-A47 / D7-S4-A47 / D7-S12-A42 / D7-S5-A49）
- [x] 每个 A 的 F 编排关系已写入 ④领域模型 / ③业务流程
- [x] specs delta 文档已规划（archive/<date>/specs/d1|d2|d7/spec.md）
- [x] 每个 Requirement 有对应的 T 层注释（命名规范段）
- [x] 重大决策已记录（Decision: observability-first + structural fix over threshold tuning）
- [x] **S3-Gate Review 结论**：见下方

#### S3-Gate Review 结论

**Reviewer:** Cursor Agent (架构自查)  
**Verdict:** **Approved** (with 2 minor suggestions)

**Pros:**
- 4 bug 全部用 observability-first + structural fix，不调阈值，对齐 feedback-threshold-design-antipattern 原则
- 跨域改动 (D1+D2+D7) 用 overload + feature flag 双保险，零破坏性
- E2E 回归测试 (AC8) 模拟真实会话 sess_1782814140202_7000，可直接验证 5 AC 同时生效

**Suggestions (minor):**
1. LastTextQualityGate 关键词词典是否需要 i18n 路径？→ 已纳入 §⑤ 单点风险缓解（用 D2 i18n）
2. DetectEmptyConclusion 标签检测是否覆盖未来 LLM 标签演化？→ 已纳入 §⑤ 单点风险缓解（4 fallback 标签）
3. E2E 测试的 LLM mock 成本？→ 复用现有 TestItemPipeline_ScopeContractRecap fixture，不需新 mock

**Approved for S4 implementation.**

### 附录 E：下一步

1. 创建 `feat/devrix-session-conclusion-completeness` 分支
2. S4 实现（按 tasks.md 4 个 PR-A/B/C 分批 + tasks.md 实施）
3. S4-Gate 自检（review-code.md 清单）
4. S5 验收（acceptance-report.md + go test -race 全绿）
5. S6-交付 PR 创建 + auto-merge
6. S6-归档 archive（DM-20260630-011 → openspec/archive/2026-06-30-devrix-session-conclusion-completeness/）
7. 重启 devrix 脚本验收（feedback-devrix-restart-via-script.md）