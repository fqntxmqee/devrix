# Demand: D7 Proposer UserContextPrepend + ReportSummary 信息密度修复

**Demand ID**: DM-20260706-008
**Change ID**: devrix-d7-proposer-prepend-and-report-summary-fix
**Title**: D7 Proposer UserContextPrepend 边界强制 + UncertaintyReport 信息密度恢复
**Priority**: P0
**Submitter**: devrix-orchestration team
**Date**: 2026-07-06 (retroactive S6 archive 2026-07-08)

## Problem

`sess_1783333760211_6000` 飞书卡片失败事件的 trace
`0369df062c125dfe2aad9de21363730e` 揭示 D7 编排层 3 个 LLM proposer
存在 **两类同源 bug**:

| Bug 类别 | 现象 | 协议层偏差 |
|---------|------|-----------|
| **A: UserContextPrepend 边界吞** | `messagesForLLMInvoke` 没被调用,LLM 拿不到 AGENTS.md prepend | D2→D3 API 边界 |
| **B: ReportSummary 信息坍塌** | `uncertaintyReportSummary` 只数 anomalies,ObsUncertainty/ObsDeviation 实际内容丢失 | D7 内部节点间契约 |

后果:LLM 在 Observe / Plan 节点看不到 AGENTS.md(从而看不到 D7 域路径约定),Plan 节点收不到 ObsUncertainty 的 `Question` 字段(只能看到 anomaly 计数),导致 Plan 输出 scope_in 字面路径(如 `d7 领域`)而不是具体路径(如 `internal/layers/orchestration/plan/`),下游 Execute / Verify 节点无法消费,飞书卡片❌。

## Required Outcome

### Outcome 1 — D2→D3 边界强制

所有 `proposer.InvokeStream` 调用必须经过 `messagesForLLMInvoke(msgs, prepared.UserContextPrepend)`
包装;任何绕过此路径的 proposer 调用视为协议违规。

**实现路径**:
- PR #449: Observe (LLMObservationProposer) + Plan (StrategicPlanProposer) 改为走 messagesForLLMInvoke
- PR #450: Execute (workitem_executor.go) 早已 wired(无改动,trace 验证自洽)
- PR #460: IntentSegmenter(PR-A2 新增的 5th InvokeStream call site)补 wired

### Outcome 2 — UncertaintyReport 信息保真

`uncertaintyReportSummary` 签名从 `(anomalyCount int)` 扩展为 `(report UncertaintyReport)`,
扫描 `report.Observations`,序列化 ObsUncertainty.Question + ObsDeviation.Statement
(strength ≥ 0.7 阈值过滤)。**实现不应把 4 维对象坍塌为 1 字符串**;序列化是必要的
(LLM 文本消费),但必须保留 CatBusiness Observations 的语义内容。

### Outcome 3 — partition by Category 设计必须被尊重

CatBusiness 走 `BusinessObservations`(LLM-driven ObsUncertainty 的归宿),
CatSystem + ObsDeviation / 高强度 ObsUncertainty 走 `Anomalies`。
**任何下游消费者必须明确自己消费的 partition**,不能假设 "Anomalies = 全部 Observations"。

### Outcome 4 — 防御性 CI guard

新增 `scripts/check-d7-d3-prepend-boundary.sh`:
- 扫描 `internal/layers/orchestration/sessionorchestrator/` 内所有
  `LLMInvoker.InvokeStream` / `InvokeNonStream` 调用点
- 验证每个调用站点所在函数**前 30 行**已调过 `messagesForLLMInvoke`
- 验证 `messagesForLLMInvoke` 至少被 2 处引用(Observe + Plan + Execute + Turn)
- allow-list `semantic_verifier_default.go`(Verify 节点的 template-mimicry 检测)

## Out of Scope

- **Plan LLM output 缺 `source_observation_ids` 字段**:design.md §7.2 强约束,
  但被 `PrepareStrategicScopeIn` Gate 兜底,未产生线上事故。归 hardening backlog。
- **D2 prior_delta_empty span fallback 缺失**:DM-20260705-010 协议要求,本 Change 不涉及。
- **CI guard 接入 CI heavy steps**:`check-d7-d3-prepend-boundary.sh` 当前本地跑;
  接入 GitHub Actions heavy steps 归 follow-up PR(与 `check-orch-rename.sh` 并列)。

## Success Criteria

| AC | 内容 |
|----|------|
| AC1 | D2→D3 边界强制 — `grep` 全仓 InvokeStream, 0 proposer 绕过 messagesForLLMInvoke |
| AC2 | UncertaintyReport 内容保真 — `uncertaintyReportSummary` 序列化 ObsUncertainty.Question + ObsDeviation.Statement, strength ≥ 0.7 阈值 |
| AC3 | Execute 节点协议自洽 — Execute LLM 收到 AGENTS.md + plan_frame_delta |
| AC4 | Regression 全绿 — orchestration packages `go test -race ./...` PASS |
| AC5 | CI guard 兜底 — `check-d7-d3-prepend-boundary.sh` PASS |

## Stakeholders

- devrix D7 orchestration team(D7 owner)
- D2 contextengine(UserContextPrepend 边界供应方)
- D3 llmgateway(`messagesForLLMInvoke` 函数定义)
- D5 observability(`messagesForLLMInvoke` 加 span attribute)
- DM-20260707-001(后续 Change 引入 IntentSegmenter,跨域 wire 需同步)