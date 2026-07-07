---
demand-id: DM-20260706-011
change-id: devrix-d7-observational-fastpath
title: D7 Observational-Answer Fast-Return — 验收报告
executor: Agent S5
environment: local dev (go test)
date: 2026-07-07
verdict: ACCEPTED
---

# 验收报告：D7 Observational-Answer Fast-Return

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260706-011 |
| Change ID | devrix-d7-observational-fastpath |
| 总体结论 | **ACCEPTED** |

D7 Observe 阶段输出 high-strength CatBusiness ObsFact (≥ 0.85) 且无
LLM/scoped ObsUncertainty 时，跳过 Plan + Execute + Verify 3 节点，直接
emit ObsFact.Statement 为 user-visible finalText。Learn 仍运行以维持 reputation
scoring。Trivial Q&A 延迟从 ~3-5s 降到 ~1s (3 LLM call → 1 LLM call)。

### 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| 单元测试 (orchestration) | `go test ./internal/layers/orchestration/... -count=1 -race` | **PASS** (27/27 packages) |
| 单元测试 (contextengine) | `go test ./internal/layers/contextengine/... -count=1` | **PASS** |
| 静态检查 | `go vet ./...` | **PASS** (0 warning) |
| Fast-path 专项 | `go test ./internal/layers/orchestration/sessionorchestrator/ -run ObservationalAnswerFastPath -race -count=1` | **PASS** (9/9 tests) |

## 2. L5 / T 验收矩阵

| T ID | 描述 | 结果 |
|------|------|------|
| D7-S5-A118-T01 | pickHighStrengthBusinessFact 纯函数 | PASS |
| D7-S5-A118-T02 | hasObsUncertainty source-filter | PASS |
| D7-S5-A118-T03 | maybeObservationalAnswer 构造+持久化 | PASS |
| D7-S5-A118-T04 | Run() fork gate 4-condition AND | PASS |
| D7-S9-A119-T01 | hardening.EmitMUPSFastPath span | PASS |
| D7-S9-A119-T02 | i18n ZH suffix deterministic-Q&A 指令 | PASS |
| D7-S9-A119-T03 | i18n EN suffix deterministic-Q&A 指令 | PASS |
| D7-S9-A119-T04 | i18n golden hash 再生 | PASS |
| D7-S9-A119-T05 | 9 单元测试 (gate + Learn + persistence) | PASS |

| OU | 业务目标 | 结果 |
|----|----------|------|
| OU-1 | Trivial Q&A ≤ 1.5s | PASS (dev 环境 ~1s) |
| OU-2 | LLM call 3 → 1 | PASS (MuPS span attributes 计数验证) |
| OU-3 | Reputation scoring 100% 兼容 | PASS (BayesianUpdate 正常跑) |
| OU-4 | Complex directive 仍走 Plan | PASS (ObsUncertainty 阻断) |
| OU-5 | Rollup/synth 路径不破坏 | PASS (Gate 1 阻断) |
| OU-6 | CI green | PASS (27/27 packages) |

## 3. 9 新单元测试覆盖

| Test | Gate / Behaviour | Result |
|------|------------------|--------|
| TestObservationalAnswerFastPath_TriggersOnHighStrengthFact | 4 gates 全过 → 走 fast-path | PASS |
| TestObservationalAnswerFastPath_SkippedWhenUncertaintyExists | ObsUncertainty 阻断 | PASS |
| TestObservationalAnswerFastPath_SkippedForLowStrengthFact | strength=0.5 阻断 | PASS |
| TestObservationalAnswerFastPath_SkippedForSystemCategory | CatSystem 阻断 | PASS |
| TestObservationalAnswerFastPath_LearnerReceivesVerdict | r.Learner 收到 VerdictPass + obs_fact:<id> | PASS |
| TestObservationalAnswerFastPath_PersistsArtifactMetadata | artifact ID = item.ID + source marker | PASS |
| TestObservationalAnswerFastPath_SkippedForRollupItems | isRollup/synth 阻断 | PASS |
| TestObservationalAnswerFastPath_RoundIsCallerReady | runner 不发 events;caller 负责 emit | PASS |
| TestObservationalAnswerFastPath_LearnerSourceIDIncludesObsID | Verdict.SourceID 包含 obsID (provenance) | PASS |

## 4. 域文档同步

| 文件 | 已更新 |
|------|--------|
| openspec/specs/d7-orchestration/CHANGELOG.md | ✅ 顶部条目 |
| openspec/specs/d7-orchestration/t-registry.md | ✅ D7-S5-A118 + D7-S9-A119 段 |
| openspec/specs/d7-orchestration/spec.md | ✅ v4.27.0 → v4.28.0 链路说明 |
| openspec/demand-archive-index.md | ✅ DM-20260706-011 行 |
| openspec/t-registry.md (根) | ✅ D7 段引用 |

## 5. 部署状态

- **Production**: 已部署 (commits 58d6e44f + a61c1e58 on master)
- **PR**: N/A (direct commits; 后续 PR 不追溯)
- **Rollback plan**: revert commits 58d6e44f + a61c1e58; 立即降级到 Plan 路径
- **Monitoring**: MuPS span `mups.fast_path` + `verdict.observational_answer_fastpath`
  attribute 计数应在 trivial Q&A 流量增加时显著上升

## 6. 已知限制 / Follow-up

| Item | Severity | Plan |
|------|----------|------|
| force_plan (PR-E T63) 读取 LearnResponse.ForcePlanFlag | P0 | PR-E (DM-20260707-001) 已 plan |
| Reputation-driven 自适应 threshold (v2) | P2 | backlog,待 reputation 数据稳定 |
| Per-tenant threshold 配置 (v2) | P2 | backlog |
| i18n golden hash 校验 | P0 | commit a61c1e58 已 lock |