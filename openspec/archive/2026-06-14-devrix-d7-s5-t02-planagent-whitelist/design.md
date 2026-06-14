---
design-id: devrix-d7-s5-t02-planagent-whitelist
title: PlanAgent 工具白名单 — 设计
proposal-id: devrix-d7-s5-t02-planagent-whitelist
status: S3_Design
created: 2026-06-14
last-updated: 2026-06-14
---

# PlanAgent 工具白名单 — 设计

## 1. 架构图

```
                    ┌──────────────────────────┐
                    │  PlanAgent (LLM-driven)  │
                    ├──────────────────────────┤
   LLM ──prompt──▶  │  buildPlanPrompt(req)    │──▶ LLM response
                    │   ├─ system prefix       │     │
                    │   ├─ "READ-ONLY MODE"    │     │ parsePlanResponse
                    │   └─ "Available tools    │     │     │
                    │       (read-only         │     ▼
                    │       whitelist):       │   PlanResult
                    │       read, grep, ls..." │     │
                    └──────────┬───────────────┘     │
                               │                     ▼
                               ▼                  Tasks
              ┌─────────────────────────────┐
              │  PlanAgentReadOnlyTools     │  ◀── 包级常量（导出）
              │  = ["read", "grep", ...]    │
              │                             │
              │  PlanAgentForbiddenTools    │  ◀── 包级常量（导出）
              │  = ["write", "edit", ...]   │
              └─────────────────────────────┘
                          ▲
                          │
              ┌───────────┴──────────────┐
              │  PlanAgent 公开 API       │
              │  AllowedTools() []string  │
              │  IsReadOnlyTool(n) bool   │  ◀── nil-safe
              └──────────────────────────┘
                          ▲
                          │
              ┌───────────┴──────────────┐
              │  D7-S5-T02 测试点         │
              │  - 白名单不含 write 等    │
              │  - 黑名单存在             │
              │  - AllowedTools 一致      │
              │  - IsReadOnlyTool 正确    │
              │  - prompt 注入白名单      │
              └──────────────────────────┘
```

## 2. 数据结构

### 2.1 导出常量

```go
// PlanAgentReadOnlyTools 是 PlanAgent 在只读模式下允许 LLM 调用的工具白名单。
//
// 语义：出现在此 slice 中的工具名被视为"对代码库无副作用"，可被 LLM 在
// PlanAgent.Plan() 探索阶段调用。
//
// 契约保证（由 D7-S5-T02 测试点校验）：
//   - 非空
//   - 不与 PlanAgentForbiddenTools 出现交集
//   - buildPlanPrompt 输出包含全部条目
var PlanAgentReadOnlyTools = []string{
    "read",       // 文件读取
    "grep",       // 代码搜索（基于文本）
    "find",       // 文件查找（基于 glob）
    "ls",         // 目录列表
    "git_status", // git 状态（只读）
    "git_log",    // git 历史（只读）
    "git_diff",   // git 差异（只读）
}

// PlanAgentForbiddenTools 是 PlanAgent 明确禁止的工具黑名单。
//
// 语义：出现在此 slice 中的工具名被视为"对代码库有副作用"，绝不能出现在
// PlanAgentReadOnlyTools 中。本变量仅作契约声明，不参与运行时逻辑。
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

### 2.2 PlanAgent 公开方法

```go
// AllowedTools 返回 PlanAgent 实例的只读工具白名单。
// 当前为包级常量；未来若按 session 注入，方法签名保持稳定。
func (a *PlanAgent) AllowedTools() []string {
    return PlanAgentReadOnlyTools
}

// IsReadOnlyTool 报告 name 是否在 PlanAgent 只读白名单中。
// nil receiver 安全：返回 false（不 panic）。
func (a *PlanAgent) IsReadOnlyTool(name string) bool {
    if a == nil {
        return false
    }
    for _, t := range PlanAgentReadOnlyTools {
        if t == name {
            return true
        }
    }
    return false
}
```

### 2.3 buildPlanPrompt 调整

**调整前**（plan_agent.go:127-131）：
```go
func buildPlanPrompt(req PlanRequest) string {
    var toolsHint string
    if len(req.Tools) > 0 {
        toolsHint = "Available tools: " + strings.Join(req.Tools, ", ")
    }
    // ... 拼接 prompt ...
}
```

**调整后**：
```go
func buildPlanPrompt(req PlanRequest) string {
    // Always include the PlanAgent read-only whitelist; merge with caller's
    // req.Tools. Caller's additions are appended (duplicates de-duped), but
    // the whitelist always comes first to signal the read-only contract.
    allowed := PlanAgentReadOnlyTools
    merged := make([]string, 0, len(allowed)+len(req.Tools))
    merged = append(merged, allowed...)
    for _, t := range req.Tools {
        found := false
        for _, m := range merged {
            if m == t {
                found = true
                break
            }
        }
        if !found {
            merged = append(merged, t)
        }
    }
    toolsHint := "Available tools (read-only whitelist + extras): " + strings.Join(merged, ", ")
    // ... 拼接 prompt ...
}
```

## 3. 流程

### 3.1 LLM 探索流程

```
Caller  ──req.Tools──▶  PlanAgent.Plan(ctx, req)
                              │
                              ▼
                  buildPlanPrompt(req)
                              │
                              ├─ system prefix (READ-ONLY MODE)
                              ├─ "Available tools: <白名单 + req.Tools 去重>"
                              └─ User Goal + Context
                              │
                              ▼
                  LLM.Complete(ctx, prompt)
                              │
                              ▼
                  parsePlanResponse(response)
                              │
                              ▼
                  PlanResult{Tasks, Exploration, CriticalFiles, Err}
```

### 3.2 测试点 D7-S5-T02 流程

```
TestCase 1: 白名单不含 write/edit/bash/delete
  for _, t := range PlanAgentReadOnlyTools { assert t not in {"write","edit","bash","delete"} }

TestCase 2: 黑名单非空
  assert len(PlanAgentForbiddenTools) > 0

TestCase 3: AllowedTools 一致
  assert agent.AllowedTools() == PlanAgentReadOnlyTools (same slice)

TestCase 4: IsReadOnlyTool 正确
  assert agent.IsReadOnlyTool("read") == true
  assert agent.IsReadOnlyTool("write") == false
  assert agent.IsReadOnlyTool("unknown") == false

TestCase 5: prompt 注入白名单
  prompt := buildPlanPrompt(PlanRequest{UserGoal: "test"})
  for _, t := range PlanAgentReadOnlyTools { assert prompt contains t }

TestCase 6: nil receiver
  var a *PlanAgent
  assert a.IsReadOnlyTool("read") == false (no panic)

TestCase 7: 不相交性
  for _, t := range PlanAgentReadOnlyTools { assert t not in PlanAgentForbiddenTools }
```

## 4. 测试点

| T ID | 描述 | 优先级 | 测试位置 |
|------|------|--------|----------|
| **D7-S5-T02** | PlanAgent 只读模式拒绝写操作；工具白名单不含 write/edit/bash | **P0** | `contextengine/tasks/plan_agent_whitelist_test.go` |
|   └─ T02-AC1 | 白名单非空 + 不含 write/edit/bash/delete | P0 | 同上 |
|   └─ T02-AC2 | 黑名单非空 + 含 write/edit/bash | P0 | 同上 |
|   └─ T02-AC3 | `AllowedTools()` 与常量一致 | P0 | 同上 |
|   └─ T02-AC4 | `IsReadOnlyTool` 正确 | P0 | 同上 |
|   └─ T02-AC5 | `buildPlanPrompt` 注入白名单 | P0 | 同上 |
|   └─ T02-AC6 | nil receiver no-op | P1 | 同上 |
|   └─ T02-AC7 | 黑白名单不相交 | P1 | 同上 |

## 5. 兼容性

| 项 | 影响 |
|----|------|
| `PlanAgent.Plan()` 签名 | 不变 |
| `PlanRequest` / `PlanResult` | 不变 |
| 调用方代码（plan_mode.go 等） | 不变（不调用 `AllowedTools` / `IsReadOnlyTool` 也不破坏） |
| 现有 plan_agent_test.go（如有） | 不变 |
| `buildPlanPrompt` 输出 | **新增** "Available tools (read-only whitelist + extras): ..." 段；与现有段 "Available tools: ..." 兼容（caller 传入 req.Tools 时去重） |

## 6. 不变更

- LLM 端 tool policy 实施（由 D6 advisory 兜底）
- D2 `contextengine/` 主体代码
- D7 `internal/layers/d7/` 代码
- D5 observability 代码
- `PlanAgent.Plan()` 主流程
- 现有 prompt 内容（仅修改 `toolsHint` 拼接）

## 7. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| LLM 不遵守白名单 | 中 | prompt 仍带 "READ-ONLY MODE" 软约束；运行时阻断由 D6 advisory 兜底 |
| 黑白名单漂移（被改） | 低 | 测试点 AC7 校验不相交性，漂移即失败 |
| 调用方误传 `req.Tools = ["bash"]` | 低 | 注入 prompt 仍包含白名单（merged[0:]），LLM 看到白名单第一 |
| PlanAgent 文件位置（`contextengine/tasks/`） | 低 | 本变更不迁文件；D7 v1.0 域迁移由独立 change 处理 |
