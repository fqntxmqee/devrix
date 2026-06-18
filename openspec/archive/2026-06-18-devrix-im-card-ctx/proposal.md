# Proposal: 飞书 IM 完成卡 — ctx 比例 + token 链路全埋点

**Change ID:** devrix-im-card-ctx  
**Demand ID:** DM-20260611-008  
**Status:** s7_archived（2026-06-18；ACCEPTED P1; PR #27 + #28 + #79 全部合并）

## 1. 背景

devrix 飞书 IM 完成卡底部摘要当前仅展示 `用时 / 消耗 / 模型`，缺少：
- **ctx 比例**：用户无法判断上下文窗口剩余量
- **可观测性**：当 `消耗: 0 tokens` 出现时，无 Jaeger 链路佐证根因

## 2. 问题陈述

| 问题 | 位置 | 影响 |
|------|------|------|
| `消耗: 0 tokens` | `summary.go:60-67` | 用户无感 token 实际调用量；无法定位是 provider 协议问题还是 Devrix 链路问题 |
| 无 `ctx: X%` | `summary.go:60-67` | 上下文窗口利用率不可见；无法主动压缩 |
| 关键 span 属性缺失 | `pev_engine.go:520-525` | 已有 `pev.total_tokens`，但缺 prompt/completion 拆分 + ctx_pct，定位效率低 |

## 3. Alternatives Considered

### 方案 A：metadata 透传 + 摘要拼接（推荐）

D2 PEV 在 emit `complete` 前计算 `ctx_pct`，放入 metadata；D1 gateway 读取并拼到摘要。

- **优点**：最小侵入；与现有 `usage/duration/model` 透传路径一致；T 层测试稳定
- **缺点**：ctx_pct 跨 PEV run 不累加（仅当前 run 内）

### 方案 B：Session 累计 token，存进 SessionContext

新增 `sc.CumulativeTokens`，每次 PEV 完成累加；摘要用累计值。

- **优点**：跨 PEV run 真实累计
- **缺点**：需修改 SessionContext 持久化（snapshot/store.go）；schema 变动；本期 P1 内过度设计

### 方案 C：仅加 span 属性不改 IM 卡

只加观测层，IM 侧不动。

- **优点**：改动小
- **缺点**：用户看不到 ctx%，诉求未满足

## 4. Decision

选择 **方案 A**：

- IM 摘要：`用时: X, 消耗: Y tokens, ctx: Z%, 模型: M`（Z=0 时省略）
- Span：在 `pev.run` 上加 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct`；milestone-only 路径加 `pev.llm_called=false`
- 跨 PEV run 累计方案（方案 B）作为 future backlog

## 5. What Changes

1. `pev_engine.go:512-516`（主路径）emit `complete` 前计算 `ctx_pct`，写入 metadata
2. `pev_engine.go:138`（milestone-only 路径）metadata 增加 `llm_called: "false"` + `ctx_pct: "0"`
3. `query_loop_run.go:181-184`（query loop 路径）emit `complete` 前计算 `ctx_pct`，写入 metadata
4. `pev_engine.go:520-525` span 补 `pev.prompt_tokens` / `pev.completion_tokens` / `pev.ctx_pct`
5. `query_loop_run.go:185+` 同步补 `query.run` span
6. `gateway/gateway.go:421-425` 读取 `ctx_pct` metadata，传入 `buildCompletionSummary`
7. `gateway/summary.go` `buildCompletionSummary` 签名增加 `ctxPct` 参数；格式化 `ctx: X%`
8. `gateway/summary_test.go` 新增 ctx% 案例（0 省略 / 25% / 100%）

## 6. Files Affected

| 文件 | 变更类型 |
|------|---------|
| `internal/layers/communication/gateway/summary.go` | 修改函数签名 |
| `internal/layers/communication/gateway/summary_test.go` | 新增测试 |
| `internal/layers/communication/gateway/gateway.go` | 读取 ctx_pct metadata |
| `internal/layers/contextengine/pev_engine.go` | 3 处 emit complete + span 属性 |
| `internal/layers/contextengine/query_loop_run.go` | 1 处 emit complete + span 属性 |
| `internal/layers/observability/telemetry/names.go` | 新增 `PEVCtxPctAttrs` helper（可选） |

## 7. T 层测试点

- D1-S2-T09 `summary_buildCompletionSummary_withCtxPct`
- D1-S2-T10 `summary_buildCompletionSummary_zeroCtxPct_omitted`
- D1-S2-T11 `pev_runSpan_containsCtxPct`
