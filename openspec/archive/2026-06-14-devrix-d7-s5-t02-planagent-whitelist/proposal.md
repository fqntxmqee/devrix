---
proposal-id: devrix-d7-s5-t02-planagent-whitelist
title: PlanAgent 工具白名单契约 — 提案
demand-id: DM-20260614-003
status: S2_Proposal
created: 2026-06-14
last-updated: 2026-06-14
---

# PlanAgent 工具白名单契约 — 提案

## 1. 方案概览

| 方案 | 概述 | 测试可断言性 | 实施成本 | 决策 |
|------|------|-------------|---------|------|
| A. 仅白名单常量 | 导出 `PlanAgentReadOnlyTools` 一个 slice，无方法 | ⭐⭐ 中 | ⭐ 极低 | ❌ |
| **B. 白名单 + 黑名单 + 方法 + prompt 注入** | 导出两个常量 + 两个方法 + `buildPlanPrompt` 注入 | ⭐⭐⭐ 高 | ⭐⭐ 低 | ✅ |
| C. 仅加固 prompt（不改代码） | 改 system prompt 增加更多只读约束文本 | ⭐ 低 | ⭐ 极低 | ❌ |

**决议**：选 **B**。理由：
- B 满足 AC1~AC5 全部 P0 标准（A、C 都不能让测试点枚举"哪些工具绝不能出现"）
- 实施成本与 A 几乎相同（多写一个常量 + 一个方法）
- 与 devrix-d6-validation-metric 的"4 counter + 钩子"模式对称，OpenSpec 风格统一

## 2. 方案 B 详细设计

### 2.1 数据结构

```go
// PlanAgentReadOnlyTools 是 PlanAgent 在只读模式下允许 LLM 调用的工具白名单。
// 来源：Claude Code PlanAgent 设计 — 仅允许对代码库做无副作用的探索。
//
// 注入：buildPlanPrompt 会在 prompt 中以 "Available tools: ..." 形式告知 LLM。
// 校验：测试点 D7-S5-T02 校验此白名单不含 write/edit/bash/delete。
var PlanAgentReadOnlyTools = []string{
    "read",       // 文件读取
    "grep",       // 代码搜索
    "find",       // 文件查找
    "ls",         // 目录列表
    "git_status", // git 状态（只读）
    "git_log",    // git 历史（只读）
    "git_diff",   // git 差异（只读）
}

// PlanAgentForbiddenTools 是 PlanAgent 明确禁止的工具黑名单。
// 出现在白名单中即为契约违反 — 由测试点 D7-S5-T02 校验。
//
// 本变量本身不参与运行时逻辑，仅作为契约声明 + 测试点断言。
var PlanAgentForbiddenTools = []string{
    "write",  // 文件写入
    "edit",   // 文件编辑
    "bash",   // shell 执行
    "delete", // 文件删除
    "mkdir",  // 创建目录
    "rm",     // 移除文件
    "mv",     // 重命名
    "cp",     // 拷贝（可能产生副作用）
}
```

### 2.2 方法

```go
// AllowedTools 返回当前 PlanAgent 实例的只读工具白名单。
// 现阶段为包级常量；如未来按 session 注入，方法签名可保持不变。
func (a *PlanAgent) AllowedTools() []string {
    return PlanAgentReadOnlyTools
}

// IsReadOnlyTool 报告给定工具名是否在只读白名单中。
// nil receiver 安全：返回 false。
func (a *PlanAgent) IsReadOnlyTool(name string) bool {
    for _, t := range PlanAgentReadOnlyTools {
        if t == name {
            return true
        }
    }
    return false
}
```

### 2.3 Prompt 注入

`buildPlanPrompt` 当前逻辑：
```go
var toolsHint string
if len(req.Tools) > 0 {
    toolsHint = "Available tools: " + strings.Join(req.Tools, ", ")
}
```

**调整后**：
```go
// Always include the read-only whitelist in the prompt; merge with
// caller's req.Tools (callers may add custom read-only tools, but never
// write tools — D7-S5-T02 contract).
allowed := a.AllowedTools()
merged := make([]string, 0, len(allowed)+len(req.Tools))
merged = append(merged, allowed...)
for _, t := range req.Tools {
    if !contains(merged, t) {
        merged = append(merged, t)
    }
}
toolsHint := "Available tools (read-only whitelist): " + strings.Join(merged, ", ")
```

## 3. 备选方案

### 3.1 方案 A：仅白名单常量

- 实施：`var PlanAgentReadOnlyTools = []string{...}`，无方法
- 测试：`for _, t := range PlanAgentReadOnlyTools { ... }` 循环断言
- 缺点：
  - 缺黑名单（无法让测试点声明"这个工具绝不能出现"）
  - 缺 `IsReadOnlyTool` 方法（调用方需自己写循环）
  - 与 OpenSpec 风格不一致（其他 D7 域方法都暴露 *Receiver）

### 3.2 方案 C：仅加固 prompt

- 实施：在 `buildPlanPrompt` 中增加更多反 LLM 走偏的指令
- 缺点：
  - prompt 是软约束，LLM 不一定遵守
  - 测试点无法断言"代码层是否定义了白名单"
  - 与 devrix-d6-validation-metric 路径选择相悖（那里用代码层 counter + 钩子，不靠 prompt）

## 4. 关键决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 导出白名单还是私有 | **导出** | 测试点需可枚举；私有则只能测 "AllowedTools() != nil" 这种弱断言 |
| 暴露方法还是仅常量 | **常量 + 方法** | 与 D7 域风格统一；调用方有自检 API |
| 黑名单是否参与运行时 | **仅契约声明** | 运行时阻断由 D6 advisory 兜底；本变更不引入新工具 |
| 修改 `buildPlanPrompt` 还是另起 | **修改** | prompt 中已有 `toolsHint` 注入点；最小变更 |
| nil receiver | **safe-noop** | 与 D6ValidationMetrics 风格一致；防 panic |

## 5. 实施计划

| 阶段 | 工作量估算 | 备注 |
|------|-----------|------|
| S3 设计 | 30 分钟 | 本 proposal + design.md |
| S3-Gate | 15 分钟 | 内部 review |
| S4 实现 | 1.5 小时 | plan_agent.go + 7 测试 + gofmt + go vet |
| S4-Gate | 15 分钟 | review-code.md |
| S5 验收 | 30 分钟 | acceptance-report.md |

总计约 3 小时。
