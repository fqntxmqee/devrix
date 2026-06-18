# Spec: QueryLoop Span 对齐 v1.1

**Change ID:** devrix-queryloop-spans-v1.1
**Demand ID:** DM-20260612-014
**Status:** S7_Archived (2026-06-18; S1_Cancelled)

## 1. 变更性质

为 `runViaQueryLoop` 增加 iteration / llm_call 两层 span 细化。变更在 S1 阶段取消。

## 2. Span 结构（草案）

```
queryloop.run  (顶层, harness v1.0)
├── queryloop.iteration.{N}
│   ├── queryloop.llm_call
│   ├── tool.execute
│   └── context.aggregate
```

## 3. 上游约束

- 不破坏 harness v1.0 顶层 span 结构
- 沿用 OpenTelemetry 约定
- 不引入新 schema

## 4. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** S1_Cancelled → Archived；草案保留作为未来重开参考。