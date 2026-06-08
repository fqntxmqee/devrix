# Proposal: Communication Layer V2 - Reliability Enhancement

**Change ID:** devrix-v2
**Layer:** 1 - Communication
**Type:** Enhancement
**Based on:** devrix-foundation (V1)

---

## Motivation

V1 实现了核心链路，但在可靠性方面存在不足：

1. **无 ShortId** - requestId 是长格式，不够友好
2. **无认证机制** - Adapter 没有身份验证，存在安全风险
3. **无 IM Adapter** - 只支持 CLI，无法远程控制
4. **无心跳保活** - WebSocket 连接可能断开不知
5. **领域事件不完整** - 缺少 connection.lost/restored 等事件

## V2 Goals

| Goal | Description | Priority |
|------|-------------|----------|
| ShortId | 5位短ID生成，易读防脏话 | P1 |
| Auth | Adapter 共享密钥 + Token 认证 | P1 |
| IM Adapter | 飞书/Telegram/Discord 接入 | P1 |
| Heartbeat | WebSocket 心跳检测连接状态 | P1 |
| 完整事件 | connection.lost/restored 等 | P2 |

## Technical Approach

### ShortId 生成

使用基于随机数的 5 位字符串，字符集：`0123456789ABCDEFGHJKLMNPQRSTUVWXYZ`（去掉了易混淆的 I, O）。

### Auth 机制

Adapter 注册时需要提供共享密钥，Gateway 验证后才建立连接。Token 使用 JWT 格式。

### IM Adapter

复用 cc-connect 的飞书 adapter 架构，实现：
- WebSocket 实时消息接收
- 消息回复和发送
- 权限请求卡片

### Heartbeat

每个 WebSocket 连接维护心跳：
- 客户端每 30s 发送 ping
- 服务器 60s 内未收到 ping 认为断连
- 触发 connection.lost 事件

## Scope

**In Scope:**
- ShortId 生成器
- Auth 中间件
- 飞书 Adapter 完善
- Heartbeat 实现
- 完整领域事件

**Out of Scope:**
- 钉钉 Adapter (V3)
- Milestone (V3)
- 多实例部署 (V3)

## Risks

| Risk | Mitigation |
|------|------------|
| ShortId 碰撞 | 使用 5 位 + 随机 + 时间戳，几乎不可能碰撞 |
| Auth 性能 | Token 验证使用内存缓存 |
| WebSocket 断连 | 实现自动重连逻辑 |

## Timeline

- V2 Delta Spec: 本 PR
- V2 实现: 1-2 周
- V2 测试: 3-5 天

---

## Open Questions

1. ShortId 是否需要全局唯一性检查？
2. Auth Token 过期时间设置为多少合适？（建议 24h）
3. Heartbeat 间隔是否可配置？
