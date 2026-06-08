# Demand: Communication Layer V2 — 可靠性增强

**Demand ID:** DM-20260607-009
**Status:** Delivered
**Priority:** P1
**Created:** 2026-06-07
**Change ID:** devrix-v2

---

## 原始描述

V1 通信层核心链路已通，但缺少 ShortId、Adapter 认证、IM 接入、心跳保活、完整领域事件与限流等可靠性能力。

## 澄清范围

**In Scope:**
- ShortId 生成（5 位，防混淆字符集）
- JWT Auth 中间件
- Feishu Adapter 完整实现
- Connection Manager + heartbeat
- connection.lost / connection.restored 等领域事件
- Token bucket 限流

**Out of Scope:**
- Milestone DAG / TaskFlow（V3）
- 钉钉 Adapter（V3）

## 验收标准

| ID | 描述 | 优先级 |
|----|------|--------|
| L5-COMM-09 | ShortId 格式合法且唯一 | P1 |
| L5-COMM-10 | Auth 注册/校验 JWT | P1 |
| L5-COMM-11 | Feishu 消息收发与卡片回调 | P1 |
| L5-COMM-12 | 心跳超时触发 connection.lost | P2 |
| L5-COMM-13 | 限流返回 429 | P2 |

## 变更历史

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-06-07 | 创建 | 初始规划 |
| 2026-06-08 | 交付 | 代码落地，S7 归档 |
