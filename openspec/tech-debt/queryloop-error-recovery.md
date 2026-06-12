# Tech Debt: QueryLoop 错误恢复补全

**来源：** DM-20260611-001（Superseded）剩余缺口  
**主路径：** DM-20260610-012 QueryLoop（已归档）  
**建议承载：** DM-20260611-004（Legacy Harness 退役）或独立小 PR  
**优先级：** P1

## 背景

QueryLoop v1/v2 已交付 while-true 循环、StreamingToolExecutor、孤儿 tool 补偿。以下能力在 Claude Code `queryLoop` 中存在，Devrix QueryLoop 路径尚未对齐。

## 待办项

### TD-QL-01: Prompt Too Long (413) 恢复链（P1）

- **现状：** QueryLoop 路径无 413 → collapse drain → reactive compact → 重试
- **目标：** 复用现有 `compression/` 七步管道，在 `query/loop.go` 捕获 413 后依次触发
- **验收：** 集成测试：超限上下文自动压缩后成功完成 tool 轮次

### TD-QL-02: max_output_tokens 恢复（P1）

- **现状：** 无 64k 扩容 + recovery message 渐进恢复
- **目标：** LLM 返回 truncation 信号时注入 recovery user message 并重试（最多 3 次）
- **验收：** 单测 + mock provider truncation 场景

### TD-QL-03: Loop 级 fallback model（P1）

- **现状：** `llmgateway/retry` 有 fallback，QueryLoop 未统一接入 overload/5xx 恢复
- **目标：** `query/loop.go` 在 primary 失败时经 gateway retry executor 切换 fallback
- **验收：** 集成测试 mock primary 失败 → fallback 成功

### TD-QL-04: D6 评测探针（P2）

- `LoopDepthProbe`：不同 max_turns 下任务完成率
- `ToolConcurrencyProbe`：Streaming vs 串行端到端延迟对比

### TD-QL-05: IM 事件契约（P0 回归）

- **背景：** YOLO + tool calls 曾 suppress `complete`，导致飞书无 Done（已修复 `query_loop_run.go`）
- **目标：** T 层回归：`complete` 事件在 QueryLoop 所有模式（含 YOLO）下必达 IM 层

### TD-QL-06: 恢复时 orphan message tombstone（P1）

**参考：** clawcode `query.ts` — recovery 前 yield tombstone 移除 UI/transcript 中孤儿 assistant messages

- **现状：** 仅有入口 `FilterIncompleteToolCalls`，Loop 内 API 失败恢复时不清理已 yield 的 assistant chunk
- **目标：** 413/fallback/max_tokens 恢复路径上，对已 emit 但未配对的 assistant tool_use 发 tombstone/rollback 事件
- **验收：** 集成测试 recovery 后 transcript 无 dangling tool_use

### TD-QL-07: fallback 与 StreamingToolExecutor 联动（P1）

**参考：** clawcode fallback 分支 `streamingToolExecutor.discard()`

- **依赖：** TD-QL-03（Loop fallback）+ `streaming-tool-executor-v2.md` TD-STE-03
- **目标：** 切换 fallback model 前 discard 在途 executor，防止 orphan tool_result
- **验收：** 与 TD-STE-03 共用 T 层

## 不在此 tech-debt

- Wave Scheduler / D7 编排 → **DM-20260611-007**
- Legacy Harness 双路径删除 → **DM-20260611-004**
- Process() 事件通道背压 → **DM-20260611-003**

## 建议 PR 顺序

1. TD-QL-05（回归测试，防复发）
2. TD-QL-01 + TD-QL-03 + TD-QL-07（错误恢复 + fallback discard）
3. TD-QL-06 + TD-QL-02
4. TD-QL-04
