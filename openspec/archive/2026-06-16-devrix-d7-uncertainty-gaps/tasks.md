# Tasks: D7 不确定性处理能力缺口修复

**Demand ID:** DM-20260616-001
**阶段:** S4 Implementation
**版本:** v1.0

---

## 实施任务

| # | 任务 | 文件 | 状态 |
|---|------|------|------|
| 1 | PlanAgent.HasLLM() 方法 | `workmodel/plan_agent.go` | done |
| 2 | PlanMode.Enter() LLM nil 检查 | `workmodel/plan_mode.go` | done |
| 3 | PlanAgent.ValidateToolCall() 方法 | `workmodel/plan_agent.go` | done |
| 4 | ValidateToolCall 单元测试 | `workmodel/plan_agent_whitelist_test.go` | done |
| 5 | ConflictGuard.AllowAndRegister() 方法 | `wave/conflict.go` | done |
| 6 | AllowAndRegister 单元测试 | `wave/conflict_test.go` | done |
| 7 | emit() sink.Publish() 推送 | `coordinator/orchestrate_path.go` | done |
| 8 | PlanModeApproveGate 移除 — coordinator config | `coordinator/config.go` | done |
| 9 | PlanModeApproveGate 移除 — shared config | `shared/config/coordinator.go` | done |
| 10 | PlanModeApproveGate 移除 — YAML loader | `shared/config/loader.go` | done |
| 11 | PlanModeApproveGate 移除 — bootstrap | `bootstrap/wire_coordinator.go` | done |
| 12 | LLMFallbackClassifier Deprecated 标记 | `coordinator/classifier_fallback.go` | done |
| 13 | ExecutorSelector Deprecated 标记 | `coordinator/executor.go` | done |
| 14 | go vet 通过 | — | done |
| 15 | 全量单元测试通过 (13 packages, -race) | — | done |
| 16 | D7 集成测试通过 (25 tests, -race) | — | done |

## 统计

| 指标 | 值 |
|------|-----|
| 新增方法 | 3 (`HasLLM`, `ValidateToolCall`, `AllowAndRegister`) |
| 修改方法 | 2 (`Enter`, `emit`) |
| 新增测试函数 | 8 |
| 修改文件数 | 13 |
| 移除配置项 | 1 (`PlanModeApproveGate`，跨 4 个文件) |
| 编译/测试/vet | 全绿 |
