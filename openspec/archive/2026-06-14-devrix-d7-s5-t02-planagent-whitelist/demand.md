---
demand-id: DM-20260614-003
title: PlanAgent 工具白名单测试点强化 — 只读探索契约落地
source: devrix-d7-orchestration-domain R2 决议 P1 #7
priority: P1
status: S1_Requirement
dsaft_domain: D7, D2
created: 2026-06-14
last-updated: 2026-06-14
---

# PlanAgent 工具白名单测试点强化

## 1. 原始描述

`devrix-d7-orchestration-domain` R2 决议 §5 P1 第 7 项明确指出：

> PlanAgent 探索阶段被 LLM 自由调用工具，缺乏强制只读边界。需要在代码层定义"白名单 + 黑名单"，让只读模式从 prompt 软约束升级为可被测试点校验的硬契约。

**现状**：

- `internal/layers/contextengine/tasks/plan_agent.go:135-145` 仅在 system prompt 中以文本形式告知 LLM "READ-ONLY MODE"，无代码层白名单。
- `PlanRequest.Tools` 字段当前未参与任何过滤，调用方可以传 `["write", "edit", "bash"]` 而不报错。
- 无 `IsReadOnlyTool(name)` / `AllowedTools()` 之类的可断言接口。
- T 层注册表 D7-S5-T02 标记为 PLANNED（无实现，无测试）。

**目标**：将 PlanAgent 的只读约束从 prompt 软约束升级为可断言的代码契约。具体：

1. 暴露 **白名单** `PlanAgentReadOnlyTools`（导出常量），让测试点可枚举断言。
2. 暴露 **黑名单** `PlanAgentForbiddenTools`（导出常量），让测试点可声明"这些工具绝不能出现在白名单中"。
3. 暴露 `(*PlanAgent).AllowedTools() []string` / `IsReadOnlyTool(name string) bool` 供调用方按名字查询。
4. 把白名单注入 `buildPlanPrompt` 的 `toolsHint`，LLM 看到的工具列表与代码层白名单一致。
5. 测试点 D7-S5-T02 升级为 IMPLEMENTED。

## 2. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `PlanAgentReadOnlyTools` 是非空 `[]string`，且不含 `write` / `edit` / `bash` / `delete` | **P0** |
| AC2 | `PlanAgentForbiddenTools` 是非空 `[]string`，至少含 `write` / `edit` / `bash`（黑名单存在即被声明） | **P0** |
| AC3 | `(*PlanAgent).AllowedTools()` 返回与 `PlanAgentReadOnlyTools` 一致的内容（顺序一致） | **P0** |
| AC4 | `(*PlanAgent).IsReadOnlyTool("read")` == `true`；`IsReadOnlyTool("write")` == `false` | **P0** |
| AC5 | `buildPlanPrompt(req)` 的输出包含 "Available tools:" 与白名单全部条目 | **P0** |
| AC6 | nil receiver 调 `IsReadOnlyTool("x")` 不 panic，返回 `false` | P1 |
| AC7 | 单元测试覆盖白名单与黑名单的不相交性（`Intersection == ∅`） | P1 |

## 3. 范围

### 3.1 新增

- `internal/layers/contextengine/tasks/plan_agent.go`：
  - 导出 `PlanAgentReadOnlyTools []string`
  - 导出 `PlanAgentForbiddenTools []string`
  - 方法 `(*PlanAgent).AllowedTools() []string`
  - 方法 `(*PlanAgent).IsReadOnlyTool(name string) bool`（含 nil-safe）
  - `buildPlanPrompt` 注入白名单（与 `req.Tools` 取并集，但只读工具优先）

- `internal/layers/contextengine/tasks/plan_agent_whitelist_test.go`：
  - 7 个测试覆盖 AC1~AC7

### 3.2 修改

- `openspec/specs/d7-orchestration/t-registry.md`：
  - D7-S5-T02 PLANNED → IMPLEMENTED
  - `Test 位置` 字段填 `contextengine/tasks/plan_agent_whitelist_test.go`

- `openspec/t-registry.md`：
  - D7 域 Total 45 → 45（不变），IMPLEMENTED 35 → 36，PLANNED 8 → 7，P0 25 → 25（D7-S5-T02 是 P0 但已被计入）

### 3.3 不变更

- `PlanAgent.Plan()` 行为不变（仅在 prompt 注入白名单，不阻断 LLM 输出）
- D6 metric 增强（devrix-d6-validation-metric）已归档的代码
- D7 orchestration domain 主体代码

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `internal/layers/contextengine/tasks/plan_agent.go`（现状已存在） |
| 依赖 | LLM 端只读工具的定义（`tool_policy.go` 等）— 本变更不引入新工具，仅声明白名单 |
| 约束 | 不得改动 `Plan()` 方法签名（向后兼容） |
| 约束 | 白名单必须可被测试点枚举（导出常量/包级变量） |
| 约束 | 不强制实施"运行时阻断 LLM 工具调用"——LLM 端沙箱属于 D6 advisory 范畴（D7-D6-T01） |

## 5. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 不遵守只读约束 | 中 | 本变更不解决 LLM 行为，只暴露可断言契约；运行时阻断由 D6 advisory（DM-002）兜底 |
| 白名单与 D2 已注册的 tool policy 冲突 | 低 | 现有 D2 无 `tool_policy.go` 强制只读（已 grep 确认），本变更不引入新工具 |
| 现有 `req.Tools` 字段被调用方滥用（传入 `["bash"]`） | 低 | `IsReadOnlyTool` 暴露给调用方自检；运行时阻断不在本变更范围 |

## 6. 后续路线（v1.1+）

1. PlanAgent 实际 LLM 端 tool policy 实施（与 D6 advisory `D7-D6-T01` 联动）
2. PlanAgent 探索结果 `CriticalFiles` 的真实性校验（防止 LLM 编造路径）
3. PlanAgent 决策清单（白名单 vs 黑名单）改为配置驱动（`devrix.yaml`），不再硬编码
