# Design: QueryLoop Span 对齐 v1.1

**Change ID:** devrix-queryloop-spans-v1.1
**Demand ID:** DM-20260612-014

> **归档说明 (2026-06-18):** 设计仅停留在提案草案；变更已取消。

## 1. 设计目标

为 `runViaQueryLoop` 内的两层调用增加 span：
1. **iteration span**：每轮迭代独立 span
2. **llm_call span**：每次 LLM 调用独立 span

## 2. Span 结构（草案）

```
queryloop.run  (顶层 span, harness v1.0 已对齐)
├── queryloop.iteration.0
│   ├── queryloop.llm_call  (LLMGateway.Complete)
│   ├── tool.execute  (tool runner 调用)
│   └── context.aggregate
├── queryloop.iteration.1
│   ├── queryloop.llm_call
│   └── ...
└── queryloop.iteration.N
```

## 3. 实现要点（草案）

### 3.1 iteration span

```go
// 伪代码
func (q *QueryLoop) runViaQueryLoop(ctx context.Context, req *Request) (*Response, error) {
    span, ctx := tracer.StartSpanFromContext(ctx, "queryloop.run")
    defer span.End()

    for iter := 0; iter < q.maxIterations; iter++ {
        iterCtx, iterSpan := tracer.StartSpanFromContext(ctx, fmt.Sprintf("queryloop.iteration.%d", iter))
        result, err := q.iterate(iterCtx, req)
        iterSpan.End()
        if err != nil || result.Final {
            return result, err
        }
    }
    return nil, ErrMaxIterations
}
```

### 3.2 llm_call span

```go
func (q *QueryLoop) iterate(ctx context.Context, req *Request) (*IterationResult, error) {
    // ...
    llmSpan, llmCtx := tracer.StartSpanFromContext(ctx, "queryloop.llm_call")
    resp, err := q.llm.Complete(llmCtx, llmReq)
    llmSpan.End()
    // ...
}
```

## 4. Span Attributes

- `queryloop.iteration.{N}`:
  - `iteration.index`: N
  - `iteration.has_tool_call`: bool
- `queryloop.llm_call`:
  - `llm.model`: model name
  - `llm.tokens.input`: int
  - `llm.tokens.output`: int

## 5. 上游约束

- 必须不破坏 harness v1.0 的顶层 span 结构
- 必须沿用 OpenTelemetry 约定（不引入新 schema）

## 6. 取消决策

**Decision (2026-06-18):** 6 天未推进；痛点不明确；变更取消。设计草案保留作为未来重开参考。

## 7. 后续路径

- 如 trace 痛点明确化 → 基于本草案重开
- 可与 devrix-tracing 后续 iteration 合并实施