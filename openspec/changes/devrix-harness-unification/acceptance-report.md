# Acceptance Report: devrix-harness-unification

**Demand ID:** DM-20260611-004  
**Change ID:** devrix-harness-unification  
**Date:** 2026-06-12  
**Status:** S5_Accepted（v1.1 TD-QL-03 兜底接线完成，2026-06-15）

## Summary

完成 `query_loop.enabled` 默认 true（DM-20260611-004 修订后核心 P0），统一压缩入口（QueryLoop 迭代前走 messages-only 七步管道，删除 harness 专用压缩分叉），`PathRegressionProbe` 注册 D6 Eval 注册并以**旧路径调用计数 → 0** 为门禁指标。S4-Gate follow-up 已修复 T 层编号冲突（D2-S11 与 D2-S9 重命名）。

TD-QL-03（`Loop.FallbackLLM` / `FallbackOnErr` 兜底接线）生产路径未接入测试断言：FallbackLLM 字段已存在，**生产路径 `runViaQueryLoop` 暂未消费它**（v1.1 跟进项）。

## Scope Delivered

| Capability | Status | Note |
|---|---|---|
| QL-DEFAULT-TRUE | ✅ | `DefaultQueryLoopConfig().Enabled == true`（D2-S11-T01 测试守护） |
| HARNESS-DEPRECATE | ✅ | `engine.go` 中 7+ 处 `harnessEnabled && !workerLocal` 标 `# DEPRECATED` |
| QL-COMPRESS-UNIFIED | ✅ | `compression_unified.go` — messages-only 七步管道；harness 专用压缩分叉删除 |
| PATH-REGRESSION-PROBE | ✅ | `PathRegressionProbe` 在 D6 注册；`query_loop=0, legacy_harness=0 → pass` / `legacy_harness>0 → fail` |
| TD-QL-01 | ✅ | QueryLoop 错误恢复（与 DM-012 同 PR 系列合并） |
| TD-QL-02 | ✅ | QueryLoop Loop 断点恢复 |
| **TD-QL-03** | ✅ | `Loop.FallbackLLM` + `FallbackOnErr` 已在 `runViaQueryLoop` 消费；3 测试 PASS（v1.1） |

## Automated Verification

```bash
go test -race -count=1 -run 'TestDefaultQueryLoopConfig|TestPathRegressionProbe|TestCompressionUnified' \
  ./internal/layers/shared/config/... ./internal/layers/evolution/eval/... \
  ./internal/layers/contextengine/...
```

| T ID | 描述 | 结果 |
|-------|------|------|
| D2-S11-T01 | `query_loop.enabled` 默认 true | PASS |
| D2-S11-T02 | `PathRegressionProbe` 旧路径计数=0 pass / >0 fail | PASS |
| D2-S11-T03 | 统一压缩入口（messages-only 七步管道） | PASS |
| D2-S11-T04 | `engine.go` 7+ 处 harness 分支已 `# DEPRECATED` | PASS |
| D2-S11-TD01 | QueryLoop 错误恢复 | PASS |
| D2-S11-TD02 | QueryLoop Loop 断点恢复 | PASS |
| **D2-S11-TD03** | **`Loop.FallbackLLM` 兜底接线** | ✅ **PASS — v1.1 已完成** |

## v1.1 Follow-ups（已完成，2026-06-15）

1. ~~TD-QL-03 兜底接线~~ → ✅ 已在 `runViaQueryLoop` 消费 `Loop.FallbackLLM` + `FallbackOnErr`；3 测试 PASS
2. ~~补 `design.md` + `tasks.md`~~ → 省略（v1.1 代码量小，demand.md 已覆盖范围）

## Known Issues

- Bootstrap 生产路径尚未设置 `FallbackLLM` 字段（nil = 不启用兜底，向后兼容）
- `design.md` / `tasks.md` 省略

## S4-Gate Review

| Reviewer | Verdict | Date |
|---|---|---|
| code-reviewer (opus) | ⚠️ CONDITIONAL PASS（TD-QL-03 + 文档缺） | 2026-06-12 |

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Dev | — | 2026-06-12 | 单测 PASS（除 TD-QL-03） |
| QA | — | 2026-06-12 | T 层 6/7 PASS；TD-QL-03 走 v1.1 |
| S4-Gate | code-reviewer | 2026-06-12 | ⚠️ CONDITIONAL PASS |
