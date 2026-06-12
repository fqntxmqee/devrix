# Acceptance Report: devrix-harness-unification

**Demand ID:** DM-20260611-004  
**Change ID:** devrix-harness-unification  
**Date:** 2026-06-12  
**Status:** S5_CONDITIONAL（v1.1 跟进：TD-QL-03 兜底接线 + design/tasks.md）

## Summary

完成 `query_loop.enabled` 默认 true（DM-20260611-004 修订后核心 P0），统一压缩入口（QueryLoop 迭代前走 messages-only 七步管道，删除 harness 专用压缩分叉），`PathRegressionProbe` 注册 D6 Eval 注册并以**旧路径调用计数 → 0** 为门禁指标。S4-Gate follow-up 已修复 L5 编号冲突（D2-S11 与 D2-S9 重命名）。

TD-QL-03（`Loop.FallbackLLM` / `FallbackOnErr` 兜底接线）生产路径未接入测试断言：FallbackLLM 字段已存在，**生产路径 `runViaQueryLoop` 暂未消费它**（v1.1 跟进项）。

## Scope Delivered

| Capability | Status | Note |
|---|---|---|
| QL-DEFAULT-TRUE | ✅ | `DefaultQueryLoopConfig().Enabled == true`（L5-2-11-01 测试守护） |
| HARNESS-DEPRECATE | ✅ | `engine.go` 中 7+ 处 `harnessEnabled && !workerLocal` 标 `# DEPRECATED` |
| QL-COMPRESS-UNIFIED | ✅ | `compression_unified.go` — messages-only 七步管道；harness 专用压缩分叉删除 |
| PATH-REGRESSION-PROBE | ✅ | `PathRegressionProbe` 在 D6 注册；`query_loop=0, legacy_harness=0 → pass` / `legacy_harness>0 → fail` |
| TD-QL-01 | ✅ | QueryLoop 错误恢复（与 DM-012 同 PR 系列合并） |
| TD-QL-02 | ✅ | QueryLoop Loop 断点恢复 |
| **TD-QL-03** | ⚠️ **PARTIAL** | `Loop.FallbackLLM` 字段已就位；**生产 `runViaQueryLoop` 尚未消费**（v1.1 跟进） |

## Automated Verification

```bash
go test -race -count=1 -run 'TestDefaultQueryLoopConfig|TestPathRegressionProbe|TestCompressionUnified' \
  ./internal/layers/shared/config/... ./internal/layers/evolution/eval/... \
  ./internal/layers/contextengine/...
```

| L5 ID | 描述 | 结果 |
|-------|------|------|
| L5-2-11-01 | `query_loop.enabled` 默认 true | PASS |
| L5-2-11-02 | `PathRegressionProbe` 旧路径计数=0 pass / >0 fail | PASS |
| L5-2-11-03 | 统一压缩入口（messages-only 七步管道） | PASS |
| L5-2-11-04 | `engine.go` 7+ 处 harness 分支已 `# DEPRECATED` | PASS |
| L5-2-11-TD01 | QueryLoop 错误恢复 | PASS |
| L5-2-11-TD02 | QueryLoop Loop 断点恢复 | PASS |
| **L5-2-11-TD03** | **`Loop.FallbackLLM` 兜底接线** | ⚠️ **PARTIAL — v1.1 跟进** |

## v1.1 Follow-ups

1. **TD-QL-03 兜底接线**：`internal/layers/contextengine/query/loop.go::runViaQueryLoop` 在 LLM 错误路径消费 `Loop.FallbackLLM` + `FallbackOnErr`，写完整 `TestL5_2_11_TD03_FallbackLLMWired` 集成测试
2. 补 `design.md` + `tasks.md`（当前仅 demand.md / proposal.md / acceptance-report.md）

## Known Issues

- TD-QL-03 在生产路径未接线（v1.1 跟进）
- `design.md` / `tasks.md` 缺失（v1.1 跟进）

## S4-Gate Review

| Reviewer | Verdict | Date |
|---|---|---|
| code-reviewer (opus) | ⚠️ CONDITIONAL PASS（TD-QL-03 + 文档缺） | 2026-06-12 |

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Dev | — | 2026-06-12 | 单测 PASS（除 TD-QL-03） |
| QA | — | 2026-06-12 | L5 6/7 PASS；TD-QL-03 走 v1.1 |
| S4-Gate | code-reviewer | 2026-06-12 | ⚠️ CONDITIONAL PASS |
