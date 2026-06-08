# Demand: Feishu Card Adapter Redesign

**Demand ID:** DM-20260608-006
**Status:** Delivered
**Priority:** P1
**Created:** 2026-06-08
**Change ID:** devrix-feishu-cards

---

## 原始描述

飞书适配层卡片模型与 cc-connect 不对齐，缺少即时 OK 确认、done_emoji、RenderText 回退及完整 Card 元素支持。

## 澄清范围

**In Scope:**
- 统一 `core/card.go` Card 模型
- OK reaction + done_emoji 双状态机制
- feishu_card.go 完整渲染器
- RenderText 平台回退
- 7 种常用颜色（design 决策，12 色留后续）

**Out of Scope:**
- 流式预览 RichCardTextStreamer（P2）
- 其他平台 Adapter 变更

## 验收标准

| 标准 | 优先级 | 状态 |
|------|--------|------|
| Card 模型与 cc-connect 兼容 | P0 | ✅ |
| RenderText 回退 | P0 | ✅ |
| OK + done_emoji | P0 | ✅ |
| CardListItem/Select/Note 元素 | P0 | ✅ |
| 12 种颜色 | P1 | ⏸ 延后（当前 7 色） |

## 变更历史

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-06-08 | 创建 | proposal + design |
| 2026-06-08 | 交付 | 代码落地，S7 归档 |
