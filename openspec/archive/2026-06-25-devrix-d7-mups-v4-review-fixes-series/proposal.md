# Proposal: D7 复审 review fixes 系列

**Change ID:** devrix-d7-mups-v4-review-fixes-series
**Demand IDs:** DM-20260625-005 + DM-20260625-006
**Status:** S4-Implemented（PR #205 + #206 squash merged，待 S5 验收 + S6 归档）
**Priority:** P0 (silent failure + 不可读 god function 是 D7 编码规范 P0 违反)
**Date:** 2026-06-25

---

## 1. Background

2026-06-24 D7 层 4-agent 并行深度 review（覆盖 9 个 orchestration 子包 + 5 节点管道核心代码），从代码质量、并发安全、错误处理、命名一致性、测试覆盖 5 个维度扫描。

**review 结论**：⚠️ **14 个问题需修复** —— 按性质分 3 大类（错误吞咽 / 死代码 / god function），按路径分 2 个 DM。

**本 change 范围**：
- DM-20260625-005（hotfix 路径）：9 错误吞咽 + 1 死代码 → PR #205
- DM-20260625-006（标准 S1-S6 路径）：3 god function 拆分 → PR #206

**与已有 change 的关系**：
- **不重复** `devrix-d7-mups-v4-review-fixes` (DM-20260625-002, PR #192) — 那是 MUPS 自身 review，本 series 是 D7 层整体 review
- **不重复** `devrix-d6-evolution-review-fixes` (DM-20260621-011) — 那是 D6 域 review
- **不重复** `devrix-d7-6s-bootstrap-slim` (DM-20260626-007) — 那是 6 S 域升级最终收尾
- **Pre-conditions**: MUPS v4 7 个 Phase S7_Archived（2026-06-24） + EscapeEngine v5 (2026-06-25)

**hotfix 路径依据**：DM-20260625-005 按 `feedback-devrix-bugfix-skip-openspec`（2026-06-17 用户确认）执行 —— 跳过 S1-S3 完整立项，**直接进入 S4 实现 + S4-Gate 审查**。理由：

1. 9 处错误吞咽是 silent failure（P0 级违反 D7 编码规范），按 `feedback-devrix-bugfix-skip-openspec` 走 hotfix
2. 1 处死代码（finalizeTask 4 参数空函数）也是 hotfix 类（删除 dead code）
3. 9+1 个修复**都是 isolated change**（一个文件一个独立修复），不需要跨模块协调
4. 9+1 个修复**都是对已落地代码的修复增强**（不是新需求），不需要 S3 设计

**标准 S1-S6 路径依据**：DM-20260625-006 走完整 S1-S6 流程 —— 3 个 god function 拆分是 pure refactor（行为不变），但涉及核心 Turn 循环 / ProcessMessage / EscapeEngine 决策核心，需要完整 S3 设计 + S4-Gate review 保证行为不变性。

---

## 2. Problem Statement

按 S4-Gate 5 维度列出 14 个修复点：

### 2.1 Silent Failure 9 个（DM-20260625-005）

**Problem 1 (P0): turn/orchestrator.go:821 PersistTurn 错误吞咽**
- **位置**：`internal/layers/orchestration/sessionorchestrator/turn/orchestrator.go:821`
- **根因**：`_, _ = h.learner.PersistTurn(...)` 形式错误吞咽
- **影响**：Turn 数据持久化失败时无任何日志，事后排查无迹可循
- **修复**：`slog.Warn` + session_id / task_id / worker_id 结构化字段

**Problem 2 (P0): wavescheduler/scheduler.go:544-549 finalizeTask 死代码**
- **位置**：`internal/layers/orchestration/wavescheduler/scheduler.go:544-549`
- **根因**：4 参数函数体全 `_ =`，无任何 side effect，纯死代码
- **影响**：调用点存在误导（让读者以为有扩展点）
- **修复**：删除函数 + 调用点 + 留说明注释

**Problem 3 (P0): executionflow/hub/hub.go:155-160 task 状态更新错误吞咽**
- **位置**：`internal/layers/orchestration/executionflow/hub/hub.go:155-160`
- **根因**：`SetOwner/UpdateStatus` 4 处错误吞咽
- **影响**：Worker/Task 状态不同步时无任何日志
- **修复**：4 处 `slog.Warn` + session_id / task_id / worker_id

**Problem 4 (P0): sessionorchestrator/command_handler.go:146 interruptHandle 错误吞咽**
- **位置**：`internal/layers/orchestration/sessionorchestrator/command_handler.go:146`
- **根因**：`interruptHandle` 错误吞咽
- **影响**：中断处理失败时无任何日志
- **修复**：`slog.Warn` + session_id / command_kind

**Problem 5 (P0): escape/arbitrator.go:562, 622 Save/Delete 错误吞咽**
- **位置**：`internal/layers/orchestration/escape/arbitrator.go:562, 622`
- **根因**：`escape state` 持久化 2 处错误吞咽
- **影响**：EscapeEngine 状态不同步时无任何日志
- **修复**：2 处 `slog.Warn` + session_id / escape_kind

**Problem 6 (P0): workmodel/work_tree.go:637 os.Remove 错误吞咽**
- **位置**：`internal/layers/orchestration/workmodel/work_tree.go:637`
- **根因**：`os.Remove` 错误吞咽
- **影响**：删除失败时无任何日志（且没过滤 `IsNotExist` 会一直 warn）
- **修复**：`slog.Warn` + `IsNotExist` 过滤（不存在的文件不警告）

**Problem 7 (P0): workmodel/cli_commands.go:377 planMode.Enter 错误吞咽（用户感知）**
- **位置**：`internal/layers/orchestration/workmodel/cli_commands.go:377`
- **根因**：`planMode.Enter` 错误吞咽 + 仍返回 "Entered plan mode" 成功消息
- **影响**：用户以为成功进入 plan mode 但实际 agent 不会运行
- **修复**：错误返回给用户（用户感知），断言改为 "Failed to enter plan mode"

**Problem 8 (P0): runregistry/registry.go:52 os.MkdirAll 错误吞咽**
- **位置**：`internal/layers/orchestration/runregistry/registry.go:52`
- **根因**：`os.MkdirAll` 错误吞咽
- **影响**：registry dir 创建失败时无任何日志
- **修复**：`slog.Warn` + 失败置空 outputDir（避免后续 WriteFile 连锁失败）

**Problem 9 (P1): workmodel/unified_tools.go:284 json.Unmarshal 错误吞咽**
- **位置**：`internal/layers/orchestration/workmodel/unified_tools.go:284`
- **根因**：`json.Unmarshal` 错误吞咽
- **影响**：tool config 解析失败时无任何日志
- **修复**：`slog.Warn` + 返回 nil（让上层用零值）

### 2.2 God Function 3 个（DM-20260625-006）

**Problem 10 (P0): ProcessMessage 183 行 god function**
- **位置**：`internal/layers/orchestration/sessionorchestrator/orchestrator.go::ProcessMessage`
- **根因**：单函数包含 resume short-circuit + classify + dispatch + handle error 全流程
- **影响**：违反 D7 编码规范（"函数 < 50 行"），review 困难，状态机分支不易追踪
- **修复**：拆 5 个 phase helper（tryResumeShortCircuit / classifyIntent / handleClassifyError / prepareRoutePreDispatch / dispatchByIntent / handleExecuteError）

**Problem 11 (P0): runLoop 519 行 god function**
- **位置**：`internal/layers/orchestration/sessionorchestrator/turn/orchestrator.go::runLoop`
- **根因**：单函数包含 prepare + iteration + LLM stream + finalize 全流程
- **影响**：Turn 主循环不可读，状态机分支复杂
- **修复**：拆 4 个 phase helper 到新文件 orchestrator_loop_helpers.go + 2 个 carrier struct（turnLoopState / turnIteration）

**Problem 12 (P0): LLMArbitrator.Arbitrate 132 行 god function**
- **位置**：`internal/layers/orchestration/escape/arbitrator.go::LLMArbitrator.Arbitrate`
- **根因**：单函数包含 invoke + parse retry + generate bounded + validate 全流程
- **影响**：EscapeEngine 决策核心不可读
- **修复**：拆 4 个 phase helper（invokeLLMWithTimeout / parseWithRetry / invokeGenerateBounded / validateActionAndBuild）+ buildForceExit 工厂

---

## 3. 修复清单（14 个）

见 demand.md §3 表格。

---

## 4. 验收

- `go build ./internal/layers/orchestration/...` PASS
- `go vet ./internal/layers/orchestration/...` PASS
- `go test -race -count=1 ./internal/layers/orchestration/...` 22/22 PASS 0 race
- 行为不变性：所有 Span 属性 key/value 不变、所有 decision routing 不变、所有 audit level (0/1/2) 不变
- 回归守护：`TestCommandHandler_Handle_PlanCommand` 改为断言 "Failed to enter plan mode" + 反向断言不含 "Entered plan mode"
