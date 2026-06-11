# Tech Debt: StreamingToolExecutor 二期对齐（clawcode 参照）

**来源：** clawcode `src/services/tools/StreamingToolExecutor.ts` vs Devrix `query/streaming_executor.go`  
**主路径：** DM-20260610-012 QueryLoop（v1 基础版已交付）  
**建议承载：** 独立 PR（~250 行）或并入 DM-20260611-004 Phase 2  
**优先级：** P1

## 背景

Devrix v1 `StreamingToolExecutor` 仅在 **整批工具全部 concurrency-safe** 时才并行。  
clawcode 支持 **混合批次**（只读工具并行 + 写工具独占）、并行 Bash 兄弟取消、fallback discard、执行中 progress 流式输出。

## 现状 vs 目标

| 能力 | Devrix v1 | clawcode | 目标 |
|------|-----------|----------|------|
| 混合批次并发 | 全 safe 才并行 | safe 可与 safe 并行；unsafe 独占 | TD-STE-01 |
| Bash 并行 sibling abort | 无 | `siblingAbortController` | TD-STE-02 |
| fallback 时 discard 在途工具 | 无 | `discard()` + synthetic error | TD-STE-03 |
| 工具 progress 中途 yield | agent tool stream only | `pendingProgress` 即时 yield | TD-STE-04 |
| 合成 error 类型 | permission/exec | sibling_error / interrupted / streaming_fallback | TD-STE-05 |
| per-tool `isConcurrencySafe` | 硬编码 switch | 工具定义回调 | TD-STE-06 |

## 待办项

### TD-STE-01: 混合批次调度（P1）

**参考：** clawcode `canExecuteTool` + `processQueue`

```
规则：
- executing=0 → 任意工具可启动
- 新工具 isConcurrencySafe && 所有 executing 均 safe → 可并行
- 新工具 !safe → 必须 executing=0；否则排队
- 结果仍按 LLM 返回顺序 emit
```

**验收：**

- 单测：`read_file×2 + bash` → 两个 read 并行，bash 等 read 完成后执行
- 单测：仅 `bash×2` → 严格串行

### TD-STE-02: Bash sibling abort（P1）

**参考：** clawcode `createChildAbortController(toolUseContext.abortController)`

- Bash 工具失败时 abort 同批其它 Bash 子进程
- 被 abort 的工具返回 synthetic `tool_result`（`Cancelled: parallel tool call errored`）
- **不** abort 父 QueryLoop turn

**验收：** 集成测试 mock 双 Bash，第一个 error → 第二个 cancelled

### TD-STE-03: discard on fallback（P1）

**触发：** QueryLoop fallback model 切换前（依赖 TD-QL-03）

- 调用 `StreamingToolExecutor.Discard()`
- 在途/queued 工具注入 `streaming_fallback` synthetic result
- 新 iteration 使用 fresh executor

**验收：** 单测 fallback 路径无 orphan tool_use

### TD-STE-04: 工具 progress 流（P2）

- 长运行工具（bash、agent tool）可通过 context 注入 progress callback
- Executor 将 progress 作为 `tool_progress` 事件经 emit 上行（IM 可选展示）

### TD-STE-05: synthetic error 统一（P2）

| Reason | tool_result 语义 |
|--------|------------------|
| `sibling_error` | 并行兄弟失败取消 |
| `user_interrupted` | 用户拒绝/ESC |
| `streaming_fallback` | 模型 fallback 丢弃在途 |

### TD-STE-06: ConcurrencySafe 注册表（P2）

- 从 `IsConcurrencySafeTool(name string)` 硬编码迁移到 `ToolRegistry` 元数据
- 与 permission mode 过滤解耦

## 不在此 tech-debt

- QueryLoop 413/fallback 主链 → `queryloop-error-recovery.md` TD-QL-01~03
- Wave Worker cancel → DM-007 §12
- Background task stop → DM-20260611-009

## 建议 PR 顺序

1. TD-STE-01（混合调度，最大收益）
2. TD-STE-02 + TD-STE-03（与 TD-QL-03 同 PR 可合并）
3. TD-STE-04~06

## L5 草拟（实施前登记）

| L5 ID | Given-When-Then | 优先级 |
|-------|-----------------|--------|
| L5-2-8-01 | Given read×2+bash 同批 When ExecuteBatch Then read 并行且 bash 最后 | P0 |
| L5-2-8-02 | Given bash 并行首错 When sibling abort Then 第二 bash 合成 cancelled | P0 |
| L5-2-8-03 | Given fallback When discard Then 无 orphan tool_use | P1 |
