---
demand-id: DM-20260614-011
title: D2 v2.0 Slice-1 — delegate_tools 迁至 D7 delegatetools
priority: P0
status: S5_Accepted
dsaft_domain: [context-engine, orchestration]
parent: DM-20260614-009
created: 2026-06-14
---

# D2 v2.0 Slice-1 — delegate_tools → orchestration/delegatetools

## 验收

| AC | 结果 |
|----|------|
| `delegate_tools.go` 从 D2 移除 | ✅ |
| 新包 `orchestration/delegatetools/` | ✅ |
| bootstrap 接线更新 | ✅ |
| D2 contextengine 无 multiagent/orchestration import | ✅ `d2_thin_test` |
| 测试全绿 | ✅ |

## 代码路径

```
internal/layers/orchestration/delegatetools/
├── delegate_tools.go      # RegisterTools, delegate_* runners
├── subquery_fallback.go   # D4 fallback → D2 query
└── *_test.go
```

## 后续 Slice

- tasks/ → workmodel (DM-012)
- queue/ → D7-S4 (DM-013)
