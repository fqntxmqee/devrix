---
demand-id: DM-20260614-007
title: D1 入站仅路由 D7 — 退役 D1→D2 legacy 路径
priority: P0
status: S3_Approved
dsaft_domain: [communication, orchestration]
created: 2026-06-14
depends_on: DM-20260614-006
---

# D1 入站仅路由 D7 — 退役 legacy D1→D2

## 1. 背景

D7 v1.0 已落地：`d7.enabled=true` 时 D1 经 `IOrchestrationEntry.ProcessMessage` 进入 D7，D7 再 fan-out 到 D2/D4。  
迁移共存期保留 `d7_enabled=false → D1→D2.Process` 回滚路径；P0 测试（D7-D1-T01、D7-MIG-T01）已绿。

**终态设计：** D1 ingress owner 只 dispatch 到 D7；D2 仅作为 D7 下游执行原语，**禁止** D1 直接调用 `IEngine.Process`。

## 2. 问题陈述

| # | 问题 |
|---|------|
| P1 | `RouteInbound` 仍含 `legacy_d2` 分支，与终态架构不一致 |
| P2 | `CommunicationGateway` 持有 `contextEngine` 用于入站 + snapshot，边界模糊 |
| P3 | `d7.enabled` 默认 false，无配置时行为回退 legacy |
| P4 | 四组合迁移矩阵测试维护 legacy 路径成本 |

## 3. 目标（L5 锚点）

- **Given** 非 Agent 入站消息 **When** `RouteInbound` **Then** 仅调用 `orchestrationEntry.ProcessMessage`（D7-D1-T01）
- **Given** 进程启动 **When** `d7.enabled=false` **Then** 启动失败并明确错误（不再 silent fallback）
- **Given** Session 持久化 **When** process 结束 **Then** snapshot 经 `ISessionSnapshotExporter` 契约导出（非 D1→D2 路由）

## 4. 范围

### In Scope
- 删除 D1 `RouteInbound` legacy D2 分支
- Gateway 构造器移除 `contextEngine` 入参；snapshot 改 optional setter
- `SetOrchestrationEntry(entry)` 去掉 `enabled` 参数
- `DefaultCoordinatorConfig().Enabled = true`；`WireD7` 失败则 main 退出
- 测试/规格同步

### Out of Scope
- D7 内部 D2 调用链（bootstrap `d2Executor`）
- Multi-agent `routeInboundViaAgent` 路径
- D7 目录 scenario-slug 物理迁移

## 5. 验收标准

- [ ] `go build ./...` 通过
- [ ] `go test ./internal/layers/communication/capture/...` 通过
- [ ] `go test -tags='acceptance d1' ./tests/acceptance/p0/ -run D1_` 通过
- [ ] `d7-domain.md` 迁移契约标记 legacy 已退役
