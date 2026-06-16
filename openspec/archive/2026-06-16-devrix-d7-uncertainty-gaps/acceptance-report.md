# Acceptance Report: D7 不确定性处理能力缺口修复

**Demand ID:** DM-20260616-001
**阶段:** S5 Acceptance
**日期:** 2026-06-16
**版本:** v1.0

---

## 验收结果总览

| AC | 标准 | 优先级 | 状态 |
|----|------|--------|------|
| AC1 | PlanAgent 运行时白名单校验 | P0 | PASS |
| AC2 | PlanModeApproveGate 死配置移除 | P0 | PASS |
| AC3 | ConflictGuard AllowAndRegister 原子化 | P1 | PASS |
| AC4 | OrchestratePath emit() sink 推送 | P0 | PASS |
| AC5 | PlanMode nil LLM Enter() 报错 | P1 | PASS |
| AC6 | 死代码标记 Deprecated | P2 | PASS |

## AC1: PlanAgent 运行时 tool call 门控

**验证方法**: `TestPlanAgent_ValidateToolCall_*` 4 个测试

- whitelist 工具 (read, grep, find, ls, git_status, git_log, git_diff) 全部通过
- forbidden 工具 (write, edit, bash, delete, mkdir, rm, mv, cp) 全部拒绝并返回 "forbidden" 错误
- 未知工具被拒绝并返回 "not in the read-only whitelist" 错误
- nil receiver 安全通过（passthrough）

**结论**: PASS

## AC2: PlanModeApproveGate 死配置移除

**验证方法**: `go build ./internal/...` + `go vet ./internal/layers/orchestration/...`

- `PlanModeApproveGate` 从 4 个文件中完全移除: `coordinator/config.go`, `shared/config/coordinator.go`, `shared/config/loader.go`, `bootstrap/wire_coordinator.go`
- 全量编译通过，无残留引用
- boot log 中不再输出 `plan_mode_approve_gate`

**结论**: PASS

## AC3: ConflictGuard AllowAndRegister 原子化

**验证方法**: `TestConflictGuard_AllowAndRegister_*` 4 个测试

- 空 guard 注册成功，Running() 返回 1 个任务
- 同 conflict group 被拒绝，Running() 仍为 1
- 不同 conflict group 被允许，Running() 为 2
- 文件范围交集被拒绝

**结论**: PASS

## AC4: OrchestratePath sink 推送

**验证方法**: 代码审查 + 集成测试通过

- `emit()` 在 sink != nil 时调用 `sink.Publish(ctx, ev)`
- nil sink 时安全跳过（兼容测试环境）
- D7 集成测试中 WaveScheduler 事件正常通过

**结论**: PASS

## AC5: PlanMode nil LLM Enter() 报错

**验证方法**: `TestPlanAgent_HasLLM` (隐式) + 代码审查

- `PlanAgent.HasLLM()` 方法检查 `a != nil && a.llm != nil`
- `PlanMode.Enter()` 调用 `p.planAgent.HasLLM()` 提前失败
- 返回 `ErrLLMNotConfigured`，状态保持 Inactive

**结论**: PASS

## AC6: 死代码标记

**验证方法**: 代码审查

- `classifier_fallback.go:10-13` 添加 "Deprecated: LLM fallback classification is deferred to v1.1"
- `executor.go:22-24` 添加 "Deprecated: WaveScheduler dispatches directly through WorkerRunner"
- 两文件原有测试全部通过

**结论**: PASS

## 测试汇总

```
单元测试: 13 packages, -race, ALL PASS
集成测试: 25 tests (15 existing + 10 added in prior PR), -race, ALL PASS
go vet:   CLEAN
go build: CLEAN
```

## 判断

全部 6 项验收标准 PASS。0 个回归。进入 S6 归档。
