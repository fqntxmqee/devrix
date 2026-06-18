---
demand-id: DM-20260611-008
title: 飞书 IM 完成卡 — 追加 ctx 比例 + token 链路全埋点
source: 用户反馈（IM 卡片 "消耗: 0 tokens" 排查）
priority: P1
status: S1_Proposed
dsaft_domain: communication
created: 2026-06-11
---

# 飞书 IM 完成卡 — 追加 ctx 比例 + token 链路全埋点

## 1. 原始描述

> tokens 显示还是 0，请你用 jaeger 看看调用链路，调用大模型后的 token 数据是否有被累计后透传到 IM 侧。另外上下文的大小，在 IM 卡片底部也透传一下，例如格式 ctx: 10%。

## 2. 问题陈述

### 2.1 现象

飞书 IM 完成卡底部摘要：

```
用时: 6s, 消耗: 0 tokens, 模型: MiniMax-M2.7-highspeed
```

| 问题 | 定位 | 影响 |
|------|------|------|
| `消耗: 0 tokens` | D1 `summary.go` + D1 `gateway.go` | 用户无法判断本轮 LLM 实际调用量；怀疑 token 累计链路断 |
| 无 `ctx: X%` | D1 `summary.go` | 用户无法判断上下文窗口剩余量；无法触发主动压缩 |
| 缺乏定位手段 | D5 观测层 | 即便代码路径正确，也无法用 Jaeger 验证 token 透传 |

### 2.2 现状数据流

```
LLM SSE 帧
  └─→ sse_parser.go:85-99  chunk.Usage (PromptTokens/CompletionTokens)
        └─→ openai_stream.go out<-AdapterChunk{Parsed:chunk}
              └─→ llm/gateway.go:272-273  usage = ac.Parsed.Usage
                    └─→ llm/gateway.go:95  streamSpan.SetAttributes(LLMUsageAttrs(...)) ✓
                          └─→ D2 query/loop.go:144-145  usage.PromptTokens += iterUsage.PromptTokens ✓
                                └─→ D2 pev_engine.go:319  usage = chunk.Usage ✓
                                      └─→ pev_engine.go:512-516  emit(complete, usage=...) ✓
                                            └─→ D1 gateway.go:421-425  buildCompletionSummary(...) ✓
                                                  └─→ D1 summary.go:60-67  "用时: ..."
```

链路完整，**唯一可被 0 卡住的位置 = LLM SSE 帧本身不含 usage**。需要观测层佐证。

### 2.3 ctx 比例需求

`ctx: X%` 中的 X 含义：**当前轮 prompt_tokens / sc.TokenBudget.MaxContextTokens × 100**

- prompt_tokens：调用大模型前的输入 token 数（即 SSE 帧 `usage.prompt_tokens`）
- MaxContextTokens：默认 128000（devrix.yaml `max_context_tokens`），可配

## 3. 范围

**In scope**：

- D1 摘要 `buildCompletionSummary` 增加 `ctx: X%` 输出（当 `ctx_pct > 0`）
- D1 `case "complete"` 读取 `ctx_pct` metadata
- D2 `pev_engine.go` 两处 `emit(complete)` 增加 `ctx_pct` 计算
- D2 `query_loop_run.go` `emit(complete)` 增加 `ctx_pct` 计算
- D2 `pev_engine.go:138` 硬编码 `usage: "0"` 路径标注"无 LLM 调用"并打对应 span
- D2 PEV 链路 span 补齐 token 关键属性（已有 `pev.total_tokens`，补 `pev.prompt_tokens` / `pev.ctx_pct` / `pev.completion_tokens`）
- D3 LLM gateway span 已具备 `llm.tokens.prompt/completion/latency_ms`，无需新增
- 单测：`summary_test.go` 新增 ctx% 案例

**Out of scope**：

- 累计多轮 token（仅当前 PEV run 内的 token 累加，跨 PEV run 由 session 层 future scope 处理）
- 跨 LLM 切换时 ctx 重置逻辑
- 飞书卡片 UI 改造（仅修改摘要文本）

## 4. 验收

- [ ] {T}-1-2-{09,10,11} 单测绿
- [ ] 真机 miniMax 调用：IM 卡片显示 `用时: X, 消耗: Y tokens, ctx: Z%, 模型: M`
- [ ] Jaeger 截图：可见 `pev.run` span 含 `pev.prompt_tokens` / `pev.ctx_pct` / `pev.completion_tokens`；`llm.stream` span 含 `llm.tokens.prompt` / `llm.tokens.completion`
- [ ] token=0 根因定位：Jaeger `llm.tokens.prompt=0` 但 query loop 收到 usage → 问题在 provider 协议；反之 → 问题在 Devrix 链路
