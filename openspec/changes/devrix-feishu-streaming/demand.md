---
demand-id: DM-20260611-006
title: 飞书 IM 2.0 流式更新 — Cardkit 元素级打字机
source: 用户反馈 + v3.0 Feishu 流式 UX 规划
priority: P1
status: S5_Acceptance
l1-domain: communication
created: 2026-06-11
---

# 飞书 IM 2.0 流式更新

## 1. 原始描述

> 飞书 2.0 流式更新方案，我们没有实现吗？如果没有，帮我规划一下。
>
> （上下文）当前回复 Markdown 在飞书 IM 卡片展示不佳；已实现 JSON 2.0 卡片与多卡聚合（思考/工具/回复分离），但回复仍为每个 text chunk 全卡 `Im.Message.Patch`，非飞书原生 `streaming_mode` + cardkit 元素 PUT。

## 2. 问题陈述

### 2.1 现状：伪流式

| 能力 | devrix 现状 | 飞书 2.0 原生 |
|------|-------------|---------------|
| 发卡 | `Im.Message.Reply` + 内联 card JSON | `POST cardkit/v1/cards` → 消息引用 `card_id` |
| 流式更新 | 每 chunk 全卡 Patch | `PUT .../elements/{element_id}/content` + `sequence` |
| 卡片配置 | 无 `streaming_mode` | `config.streaming_mode: true` |
| 元素标识 | markdown 无 `element_id` | 固定 `element_id`（如 `reply_text`） |
| 客户端效果 | 整卡闪烁、中间态 Markdown 乱 | 打字机动画、格式更稳 |

### 2.2 历史 backlog

- `devrix-feishu-cards` 归档时 **RichCardTextStreamer** 标为 P2 未完成
- `devrix-queryloop-context` v3.0 路线图含 **Feishu 流式 UX**，本需求为其独立拆出

### 2.3 cc-connect 已验证路径

cc-connect 已实现 `createCardEntity` → `StreamRichCardText`（cardkit PUT）→ `updateCardEntity`（结束时全卡更新），可作为实现参考，无需重新设计协议。

## 3. 澄清记录

### Q1: 流式更新覆盖哪些卡片？

**A**: **仅回复卡**走 cardkit 元素流式。思考卡、工具聚合卡、任务进度卡维持现有 Patch 聚合逻辑，避免多卡 sequence 冲突。 — 2026-06-11

### Q2: cardkit 不可用时的行为？

**A**: **自动降级**为当前 `Im.Message.Patch` 全卡更新，功能不中断，仅失去打字机效果。 — 2026-06-11

### Q3: 是否改造 Gateway / Context Engine？

**A**: **Out of Scope**。Gateway 仍发 `event_type=text` chunk；变更限定在 `FeishuAdapter` + 配置层。 — 2026-06-11

### Q4: 飞书应用权限要求？

**A**: 需开通 cardkit 相关 scope（如 `cardkit:card:write`）；在 design.md 记录权限清单与验收检查项。 — 2026-06-11

### Q5: 与 queryloop v3 T26（worker_card）关系？

**A**: 本需求独立交付回复卡流式；T26 worker 任务树卡片不在本变更范围，但复用同一 cardkit 封装。 — 2026-06-11

## 4. 澄清范围

### 4.1 L1–L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | communication | 通信层 / IM 适配 | 已有 |
| L2 | L2-COM-IM | IM 远程对话 | 已有 |
| L3-BE | L3-BE-COM-01 | 飞书入站消息处理与出站推送 | 已有 |
| L4-BE | L4-BE-COM-cardkit | Cardkit 卡片实体生命周期 | **新增** |
| L4-BE | L4-BE-COM-stream-reply | 回复卡元素级流式更新 | **新增** |
| L5 | L5-1-2-04 | Cardkit 双步发卡成功 | 草拟 |
| L5 | L5-1-2-05 | 元素级流式 PUT sequence 递增 | 草拟 |
| L5 | L5-1-2-06 | cardkit 失败降级 Patch | 草拟 |
| L5 | L5-1-2-07 | complete 关闭 streaming_mode | 草拟 |
| L5 | L5-1-2-08 | 流式节流配置生效 | 草拟 |

### 4.2 范围

**In Scope**:

- Cardkit `POST /cardkit/v1/cards` 创建卡片实体
- 回复消息引用 `{"type":"card","data":{"card_id":"..."}}`
- 回复 markdown 元素 `element_id: reply_text` + `streaming_mode: true`
- `PUT .../elements/reply_text/content` 流式更新（累积全文 + sequence）
- `complete` 时关闭流式并写入 footer
- 节流（IntervalMs / MinDeltaChars）与配置项
- cardkit 失败降级 Patch
- 单元测试 + 飞书真机验收指引

**Out of Scope**:

- Gateway 事件模型变更
- 思考卡 / 工具卡元素流式（v1.1 可选）
- 钉钉 / CLI 适配器
- RichCard 多栏进度面板（queryloop T26）
- 图片 URL 上传转 img_key（cc-connect ResolveRichCardMarkdown）

### 4.3 验收标准（摘要）

**P0**:

- [ ] 回复卡首包创建 card_id 并引用发送
- [ ] 流式 text chunk 走元素 PUT，sequence 单调递增
- [ ] cardkit 失败自动降级 Patch，最终内容完整
- [ ] `complete` 后 streaming 关闭，footer 可见

**P1**:

- [ ] 节流配置生效，避免 API 限流
- [ ] 工具调用期间不与回复流式 sequence 冲突

## 5. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `adapters/feishu` | 回复流式路径改造 | 高 |
| `adapters/feishu_card` | streaming 卡片 JSON 构建 | 中 |
| `adapters/feishu_cardkit` | cardkit API 封装（新增） | 中 |
| `shared/config` | `im.feishu.streaming` 配置 | 低 |
| `bootstrap/im_hosts` | 配置注入 | 低 |

## 6. 回归风险

- cardkit 与 Patch 混用导致 sequence 乱序 → 单 card 互斥锁 + 统一计数器
- 限流 code 230020 → 跳过本帧、不降级（参考 cc-connect）
- 飞书客户端 < 7.20 → 文档说明；低版本显示升级兜底文案
