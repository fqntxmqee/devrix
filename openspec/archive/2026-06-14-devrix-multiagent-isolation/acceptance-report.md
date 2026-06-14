# Acceptance Report: devrix-multiagent-isolation

**Demand ID:** DM-20260611-005  
**Change ID:** devrix-multiagent-isolation  
**Date:** 2026-06-12  
**Status:** S5_PASS（v1.0 范围内）

## Summary

完成多智能体子 Agent 隔离三大支柱：(1) `SessionView` COW 写时复制（`internal/layers/multiagent/sessionview/cow.go`）— 子 Agent 写入隔离副本不污染父会话；(2) `Join` 工具调用去重（`forkjoin.go`）— 通过统一 metadata key `multiagent.MetaToolCallID` 关联"父调度"与"子完成"事件；(3) D6 `PathRegressionProbe` 增加 `runtime.PathQueryLoop` / `runtime.PathLegacyHarness` 指标以观测旧路径。

S4-Gate follow-up 已修复关键 HIGH：`call_id` ↔ `tool_call_id` 元数据键不一致 bug（生产 contextengine 写 `tool_call_id`，lifecycle/forkjoin 之前读 `call_id`，导致 Join dedup 实际失效）。修复方式：抽取共享常量 `multiagent.MetaToolCallID = "tool_call_id"`，全链路统一。

## Scope Delivered

| Capability | Status | Note |
|---|---|---|
| SV-COW | ✅ | `sessionview/cow.go` — Copy-on-Write 隔离父子视图 |
| JOIN-DEDUP | ✅ | `forkjoin.go` 消费 `multiagent.MetaToolCallID` |
| RUNTIME-OBSERVABILITY | ✅ | `runtime.Record(runtime.PathQueryLoop / PathLegacyHarness)` 接入 D6 probe |
| D4-S3-T05 | ✅ | SessionView COW 集成测试守护（已加入 t-registry） |

## Automated Verification

```bash
go test -race -count=1 ./internal/layers/multiagent/agent/...   # forkjoin_isolation_test
go test -race -count=1 ./internal/layers/multiagent/sessionview/...  # cow_test
go test -race -count=1 ./internal/layers/evolution/eval/...  # PathRegressionProbe
```

| T ID | 描述 | 结果 |
|-------|------|------|
| D4-S3-T01 | SessionView COW 父子隔离 | PASS |
| D4-S3-T02 | 子 Agent 写入不污染父 | PASS |
| D4-S3-T03 | Join 去重 — 同一 tool_call_id 只触发一次 complete | PASS |
| D4-S3-T04 | runtime.PathQueryLoop 计数 0 → pass | PASS |
| D4-S3-T05 | SessionView COW 集成测试 | PASS（2026-06-12 补） |

## 关键修复（2026-06-12 S4-Gate follow-up commit `69e0401`）

| 等级 | 问题 | 修复 | 验证 |
|---|---|---|---|
| HIGH | `call_id` vs `tool_call_id` 键不一致（Join dedup 失效） | 抽 `multiagent.MetaToolCallID` 常量，lifecycle.go / forkjoin.go 改读共享常量 | `forkjoin_isolation_test` PASS |
| MEDIUM | t-registry 缺 D4-S3-T05 编号 | 加入 SessionView COW 集成测试条目 | yaml / 文档同步 |

## Known Issues

- 包级覆盖率：multiagent/agent 76.5% — 新增代码 100% 覆盖，剩余为 builtin / 早期 legacy（非本变更引入）

## S4-Gate Review

| Reviewer | Verdict | Date |
|---|---|---|
| code-reviewer (opus) | ✅ PASS（follow-up 后） | 2026-06-12 |

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Dev | — | 2026-06-12 | 单测 + 集成 PASS |
| QA | — | 2026-06-12 | T 层 100% PASS + Join dedup 真实生效 |
| S4-Gate | code-reviewer | 2026-06-12 | ✅ PASS |
