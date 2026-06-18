# Spec: IM Card Context Field

**Change ID:** devrix-im-card-ctx
**Demand ID:** DM-20260615-002 (alt: PR #27 / PR #28 / PR #79 series)
**Status:** S7_Archived (2026-06-18)

## 1. 变更性质

为 IM 卡片消息添加 ctx% (上下文占用率) 字段；token 链路打通至 IM 卡片。

## 2. 涉及域

- D1 Communication（IM 消息格式）
- D2 Context Engine（ctx 计算）
- D5 Observability（trace 追踪）

## 3. 接口契约

- IM card JSON 扩展 `ctx_percent` 字段（0-100）
- IM card JSON 扩展 `tokens_in / tokens_out` 字段
- LLM 调用 trace 关联 ctx 状态

## 4. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** PR #27 (T09) + PR #28 (T10) + PR #79 (T11) 全部 merged；feature live。