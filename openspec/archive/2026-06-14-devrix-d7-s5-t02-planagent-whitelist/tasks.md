---
tasks-id: devrix-d7-s5-t02-planagent-whitelist
title: PlanAgent 工具白名单 — 实施任务
demand-id: DM-20260614-003
status: S3_Tasks
created: 2026-06-14
last-updated: 2026-06-14
---

# PlanAgent 工具白名单 — 实施任务

## 1. 任务分解（DSAFT: Activity / Function / Test）

| T ID | 任务 | 归属 A/F | 估算 | 状态 |
|------|------|----------|------|------|
| T-DV-A01 | 在 `plan_agent.go` 导出 `PlanAgentReadOnlyTools` 常量 | D7-S5-A04 | 10 分钟 | PLANNED |
| T-DV-A02 | 在 `plan_agent.go` 导出 `PlanAgentForbiddenTools` 常量 | D7-S5-A04 | 10 分钟 | PLANNED |
| T-DV-F01 | 在 `plan_agent.go` 实现 `(*PlanAgent).AllowedTools()` | D7-S5-A04-F01 | 15 分钟 | PLANNED |
| T-DV-F02 | 在 `plan_agent.go` 实现 `(*PlanAgent).IsReadOnlyTool(name)` | D7-S5-A04-F01 | 15 分钟 | PLANNED |
| T-DV-F03 | 修改 `buildPlanPrompt` 注入白名单 + req.Tools 去重合并 | D7-S5-A04-F02 | 20 分钟 | PLANNED |
| T-DV-T01 | 编写 `plan_agent_whitelist_test.go` 7 个测试（AC1~AC7） | D7-S5-A04-T01 | 60 分钟 | PLANNED |
| T-DV-T02 | 跑 `go test -race -count=1 ./internal/layers/contextengine/tasks/...` | D7-S5-A04-T01 | 10 分钟 | PLANNED |
| T-DV-T03 | 跑 `gofmt -l` + `go vet ./internal/layers/contextengine/tasks/...` | D7-S5-A04-T01 | 5 分钟 | PLANNED |
| T-DV-D01 | 更新 `openspec/specs/d7-orchestration/t-registry.md` D7-S5-T02 → IMPLEMENTED | D7-S5-A04 | 5 分钟 | PLANNED |
| T-DV-D02 | 更新 `openspec/t-registry.md` D7 域 IMPLEMENTED 35 → 36 | D7-S5-A04 | 5 分钟 | PLANNED |
| T-DV-D03 | 编写 `review-code.md` (S4-Gate) | D7-S5-A04 | 15 分钟 | PLANNED |
| T-DV-D04 | 编写 `acceptance-report.md` (S5) | D7-S5-A04 | 20 分钟 | PLANNED |
| T-DV-D05 | S6 归档（移至 `archive/2026-06-14-devrix-d7-s5-t02-planagent-whitelist/`） | D7-S5-A04 | 10 分钟 | PLANNED |

**总计**：约 3 小时 30 分钟

## 2. 依赖关系

```
T-DV-A01, T-DV-A02 (常量) ──┐
                             ├─▶ T-DV-F01, T-DV-F02 (方法) ──▶ T-DV-F03 (prompt 注入)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-T01 (7 测试)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-T02 (go test)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-T03 (gofmt/vet)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-D01/D02 (t-registry)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-D03 (S4-Gate)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-D04 (S5)
                             │                                          │
                             │                                          ▼
                             │                                T-DV-D05 (S6 归档)
```

## 3. 验收任务（AC → T 映射）

| AC | 覆盖测试 |
|----|---------|
| AC1 | T-DV-T01 中 `TestPlanAgent_Whitelist_NoWriteTools` |
| AC2 | T-DV-T01 中 `TestPlanAgent_Blacklist_NonEmpty` |
| AC3 | T-DV-T01 中 `TestPlanAgent_AllowedTools_Consistent` |
| AC4 | T-DV-T01 中 `TestPlanAgent_IsReadOnlyTool_*` |
| AC5 | T-DV-T01 中 `TestPlanAgent_PromptInjectsWhitelist` |
| AC6 | T-DV-T01 中 `TestPlanAgent_NilReceiver_NoPanic` |
| AC7 | T-DV-T01 中 `TestPlanAgent_ListsDisjoint` |

## 4. 风险任务

| 风险 | 任务 | 说明 |
|------|------|------|
| 测试与现有 plan_agent_test.go 冲突 | T-DV-T01 | 新建独立文件 `plan_agent_whitelist_test.go`，避免改动 |
| go vet 警告 | T-DV-T03 | 跑完后立即 fix |
| coverage 下降 | T-DV-T01 + T-DV-T02 | 7 个新测试应覆盖所有新代码行 |

## 5. 完成判定

- [x] 所有 13 个任务完成
- [x] `go test -race -count=1` PASS
- [x] `gofmt -l` 无输出
- [x] `go vet` 无输出
- [x] tasks 域覆盖率 ≥ 80%（不变更或提升）
- [x] D7-S5-T02 IMPLEMENTED 登记
- [x] S4-Gate review-code.md APPROVED
- [x] S5 acceptance-report.md PASS
- [x] S6 archive 完成
