# Demand: Communication Layer V3 — 功能补全

**Demand ID:** DM-20260608-008
**Status:** Delivered
**Priority:** P1
**Created:** 2026-06-08
**Change ID:** devrix-v3

---

## 原始描述

V2 完成通信层可靠性增强后，V3 需补全 Milestone DAG、TaskFlow、钉钉 Adapter、跨平台 UI 组件与多实例注册能力。

## 澄清范围

**In Scope:**
- Milestone DAG + 环检测 + 执行顺序
- TaskFlow 创建/启动/进度/完成
- 钉钉 Webhook Adapter + Markdown 卡片渲染
- UI 组件（MilestoneCard / ProgressBar / StatusBadge）
- 内存 Instance Registry + 健康检查
- `cmd/devrix-dingtalk` 入口

**Out of Scope / 延后（P2）：**
- 钉钉 WebSocket 长连接（V3 默认 Webhook）
- `internal/shared/config/instance.go` 独立配置
- Prometheus `/metrics` 端点（沿用 observability 层）
- 负载均衡 sticky session / X-Forwarded-For

## 验收标准

| ID | 描述 | 优先级 |
|----|------|--------|
| L5-COMM-14 | Milestone DAG 创建、进度、完成、环检测 | P1 |
| L5-COMM-15 | TaskFlow 启动并完成里程碑链 | P1 |
| L5-COMM-16 | 钉钉 Webhook 入站路由 + Session 出站 | P2 |
| L5-COMM-17 | UI 组件渲染 Milestone / Progress | P2 |
| L5-COMM-18 | Instance Registry 注册与健康检查 | P2 |

## 变更历史

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-06-08 | 创建 | 初始规划 |
| 2026-06-08 | 交付 | 代码与单元测试落地，S7 归档 |
