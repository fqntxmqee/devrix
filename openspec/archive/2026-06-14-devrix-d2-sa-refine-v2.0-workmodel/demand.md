---
demand-id: DM-20260614-012
title: D2 v2.0 Slice-2 — tasks/ 迁至 orchestration/workmodel
priority: P0
status: S5_Accepted
dsaft_domain: [context-engine, orchestration]
parent: DM-20260614-009
created: 2026-06-14
---

# D2 v2.0 Slice-2 — contextengine/tasks → orchestration/workmodel

## 验收

| AC | 结果 |
|----|------|
| `contextengine/tasks/` 删除 | ✅ |
| 新包 `orchestration/workmodel/` | ✅ |
| TaskManager / PlanAgent / PlanMode / ToolSuite 迁入 D7 | ✅ |
| task_* 工具由 workmodel.RegisterTaskTools 注册 | ✅ |
| bootstrap / flow / delegatetools / CLI 接线更新 | ✅ |
| D7 WireD7 使用 LocalWorkModel(GlobalTaskManager) | ✅ |
| D2 Thin lint 移除 tasks/ 豁免 | ✅ |
| 测试全绿 | ✅ |

## 代码路径

```
internal/layers/orchestration/workmodel/
├── task_manager.go      # GlobalTaskManager, CRUD, disk persist
├── task_store.go        # DiskTaskStore
├── tool_suite.go        # task_* tool backend
├── register_tools.go    # RegisterTaskTools → D2 toolrunner
├── plan_agent.go        # D7-S5 read-only planning
├── plan_mode.go         # Plan mode state machine
├── cli_commands.go      # /task, /plan CLI
└── *_test.go
```

## 后续 Slice

- queue/ → D7-S4 (DM-013)
- 物理目录 prepare/persist/policy (DM-014)
