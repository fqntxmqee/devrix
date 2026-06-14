---
demand-id: DM-20260614-014
title: D2 v2.0 — 物理目录收敛（persist/policy/prepare/nested）
priority: P1
status: S5_Accepted
dsaft_domain: [context-engine]
parent: DM-20260614-009
created: 2026-06-14
---

# D2 v2.0 — 物理目录收敛（DM-014）

## 阶段 1–2：persist / policy ✅

| AC | 结果 |
|----|------|
| `snapshot/` → `persist/snapshot/` | ✅ |
| `transcript/` → `persist/transcript/` | ✅ |
| `permission/` → `policy/permission/` | ✅ |
| `toolrunner/` → `policy/toolrunner/` | ✅ |

## 阶段 3：prepare ✅

| AC | 结果 |
|----|------|
| `memory/` → `prepare/memory/` | ✅ |
| `compression/` → `prepare/compression/` | ✅ |
| `prompt/` → `prepare/prompt/` | ✅ |
| `conversation/` → `prepare/conversation/` | ✅ |

## 阶段 4：nested ✅

| AC | 结果 |
|----|------|
| `query/subquery.go` → `nested/subquery.go` | ✅ |
| `query/background.go` → `nested/background.go` | ✅ |
| `query/fork.go` → `nested/fork.go` | ✅ |
| `query/flow_report.go` → `nested/flow_report.go` | ✅ |

## 阶段 5：prepare 接线收敛 ✅

| AC | 结果 |
|----|------|
| `compression_factory.go` → `prepare/compression/query_loop_factory.go` | ✅ |
| `compression_observer.go` → `prepare/compression/step_observer_bridge.go` | ✅ |
| `tracing_step_observer.go` → `prepare/compression/tracing_step_observer.go` | ✅ |
| `autocompact_summarizer.go` → `prepare/compression/llm_summarizer.go` | ✅ |
| `token_counter.go` → `token/resolver.go` | ✅ |
| `ICompressionObserver` 类型别名 → `compression.CompressionEventSink` | ✅ |
| 测试全绿 | ✅ |

## 终态目录

```
contextengine/
├── prepare/       # D2-S15
│   ├── memory/
│   ├── compression/   # pipeline + LLM summarizer + QueryLoop factory + tracing observer
│   ├── prompt/
│   └── conversation/
├── persist/       # D2-S17
├── policy/        # D2-S18
├── nested/        # D2-S19
├── query/         # D2-S16
├── token/         # D2-S4 counter + resolver
└── harness/       # D2-S20
```
