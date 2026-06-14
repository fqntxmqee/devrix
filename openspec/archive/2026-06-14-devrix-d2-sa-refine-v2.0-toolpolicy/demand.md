---
demand-id: DM-20260614-015
title: D2 v2.0 Slice-4 — worker_tools 迁至 orchestration/toolpolicy
priority: P0
status: S5_Accepted
dsaft_domain: [context-engine, orchestration]
parent: DM-20260614-009
created: 2026-06-14
---

# D2 v2.0 Slice-4 — worker_tools → orchestration/toolpolicy

## 验收

| AC | 结果 |
|----|------|
| `worker_tools.go` 从 D2 移除 | ✅ |
| 新包 `orchestration/toolpolicy/` | ✅ |
| `AgentRoleToolFilter` 接口 + bootstrap 注入 | ✅ |
| D2 engine 无 orchestration import | ✅ |
| forkWorkerSessionContext 测试保留在 D2 | ✅ |
| 测试全绿 | ✅ |

## 代码路径

```
internal/layers/orchestration/toolpolicy/
├── filter.go       # FilterToolsForAgentRole, explore/plan read-only set
└── filter_test.go

internal/layers/contextengine/
├── agent_role_filter.go   # AgentRoleToolFilter interface
└── process_overlay_test.go
```

## 后续 Slice

- 物理目录 prepare/persist/policy (DM-014)
