---
demand-id: DM-20260614-013
title: D2 v2.0 Slice-3 — queue/ 迁至 orchestration/sessionqueue
priority: P0
status: S5_Accepted
dsaft_domain: [context-engine, orchestration]
parent: DM-20260614-009
created: 2026-06-14
---

# D2 v2.0 Slice-3 — contextengine/queue → orchestration/sessionqueue

## 验收

| AC | 结果 |
|----|------|
| `contextengine/queue/` 删除 | ✅ |
| 新包 `orchestration/sessionqueue/` | ✅ |
| 契约 `contracts.SessionCommandQueue` + `RenderQueueNotifications` | ✅ |
| D2 QueryLoop 经接口注入，无 orchestration import | ✅ |
| flow.Hub delegate-progress → sessionqueue | ✅ |
| bootstrap 接线 GlobalSessionQueue | ✅ |
| D2 Thin lint 无 queue 豁免 | ✅ |
| 测试全绿 | ✅ |

## 架构

```
flow.Hub.Publish → sessionqueue.Enqueue(ModeDelegateProgress)
bootstrap → EngineDeps.SessionCommandQueue = GlobalSessionQueue
D2 query.Loop → contracts.SessionCommandQueue.Drain (interface)
D4 delegate → sessionqueue.Enqueue(ModeTaskNotification)
```

## 后续 Slice

- 物理目录 prepare/persist/policy (DM-014)
- worker_tools.go → D7 (DM-015)
