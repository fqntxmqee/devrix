---
demand-ids: [DM-20260625-005, DM-20260625-006]
title: D7 复审 review fixes 系列（9 错误吞咽 + 1 死代码 + 3 god function 拆分）
priority: P0
status: S7_Archived
dsaft_domain: orchestration
created: 2026-06-25
archived: 2026-06-25
---

# D7 复审 review fixes 系列

## 1. 背景

2026-06-24 D7 层 4-agent 并行深度 review 覆盖 `internal/layers/orchestration/{workmodel,plan,execute,learn,sessionorchestrator,decisionplanning,wavescheduler,escape,executionflow}/`，从代码质量、并发安全、错误处理、命名一致性、测试覆盖 5 个维度扫描。

**review 结论**：发现 14 个问题需修复（按错误吞咽 / 死代码 / god function 三大类），按修复性质拆 2 个 DM 处理：

| DM | 路径 | 范围 |
|----|------|------|
| **DM-20260625-005** | hotfix（`feedback-devrix-bugfix-skip-openspec`） | 9 处错误吞咽 + 1 处死代码扩展点 |
| **DM-20260625-006** | 标准 S1-S6 | 3 个 god function 拆分（C4/H1/H2） |

## 2. 拆分理由

- **DM-005 hotfix 路径依据**：9 处错误吞咽是 silent failure（违反 `feedback-devrix-bugfix-skip-openspec` 2026-06-17 用户确认的 hotfix 规则——bug fix 跳过 S1-S3 完整流程）。
- **DM-006 标准 S1-S6 路径依据**：3 个 god function 拆分是 pure refactor（行为不变），需要完整 S3 设计 + S4-Gate review。
- **PR 拆分原则**：按 `feedback-devrix-bugfix-pr-grouping` 同一会话/同一类问题的多 bug fix 聚合成一个 PR，但跨性质（hotfix vs refactor）拆 2 PR。

## 3. 14 个修复点

### DM-20260625-005（PR #205）— 9 错误吞咽 + 1 死代码

| Fix | 严重度 | 文件 | 改动 |
|-----|--------|------|------|
| C1 turn/orchestrator.go:821 | High | `PersistTurn` 错误 → slog.Warn | 1 行 |
| C2 wavescheduler/scheduler.go:544-549 | High | 删除 `finalizeTask` 4 参数空函数 + 调用点 | -5 行 |
| C3 executionflow/hub/hub.go:155-160 | High | 4 处 task 状态更新 → slog.Warn | 4 行 |
| H4 sessionorchestrator/command_handler.go:146 | High | `interruptHandle` → slog.Warn | 1 行 |
| H5 escape/arbitrator.go:562, 622 | High | `Save/Delete` → slog.Warn | 2 行 |
| H6 workmodel/work_tree.go:637 | High | `os.Remove` → slog.Warn + IsNotExist 过滤 | 2 行 |
| H7 workmodel/cli_commands.go:377 | High | `planMode.Enter` 错误返回用户（之前默默吞掉） | 2 行 |
| H8 runregistry/registry.go:52 | High | `os.MkdirAll` → slog.Warn + 失败置空 outputDir | 2 行 |
| M5 workmodel/unified_tools.go:284 | Medium | `json.Unmarshal` → slog.Warn + 返回 nil | 1 行 |

### DM-20260625-006（PR #206）— 3 god function 拆分

| Fix | 严重度 | 文件 | 改动 |
|-----|--------|------|------|
| C4 ProcessMessage (183→75 行) | High | `sessionorchestrator/orchestrator.go` | 5 phase helper |
| H1 runLoop (519→40 行) | High | `turn/orchestrator.go` + 新文件 `orchestrator_loop_helpers.go` | 4 phase helper + 2 carrier struct |
| H2 LLMArbitrator.Arbitrate (132→21 行) | High | `escape/arbitrator.go` | 4 phase helper + `buildForceExit` 工厂 |

## 4. 与已有 change 的关系

- **不重复** MUPS v4 7 个 Phase S7_Archived change（Phase 1-7 共 9 个 archive）
- **不重复** `devrix-d7-mups-v4-review-fixes` (DM-20260625-002) — 那是 MUPS 自身 review，本 series 是 D7 层整体 review
- **不重复** `devrix-d6-evolution-review-fixes` (DM-20260621-011) — 那是 D6 域
- **姊妹篇** `devrix-d7-6s-bootstrap-slim` (DM-20260626-007) — 6 S 域升级最终收尾

## 5. 验收

- `go build ./...` PASS
- `go vet ./...` PASS
- `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS 0 race
- LP-1 `TestAutoClose_FullLP1Loop` 已知 flake（~10% fail rate）通过空 commit re-trigger CI 绕过
- `escape.TestHumanArbitrator_ResumeSession_Roundtrip` 已知 CI flake，本地 3/3 PASS
