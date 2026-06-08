---
demand-id: DM-20260608-009
title: Devrix 代码健康规范 — 不可变性、类型安全、L5 补全、命名一致
priority: P0
status: Delivered
l1-domain: architecture
created: 2026-06-08
reviewer: Claude
---

# Devrix 代码健康规范

## 1. 背景

在对 Devrix 项目进行全量规范审计后，发现以下四类问题：

1. **不可变性原则形同虚设** — CLAUDE.md 明确要求"创建新对象，禁止原地修改"，但几乎所有实体类型（`PermissionRequest`、`Session`、`Milestone`、`TaskFlow`、`Connection` 等）使用可变方法，导致规范与代码严重脱节
2. **潜在运行时 panic** — `connection/manager.go:301` 使用 unsafe type assertion，当事件类型不匹配时直接 panic
3. **D1/D6 L5 验收测试覆盖为零** — Communication 域（用户交互入口）5 个 L5 全部 PLANNED，Evolution 域 2 个全 PLANNED
4. **命名与代码异味** — `CLRenderer` 缺字、`min` 函数在 Go 1.21+ 中重复定义、`GetInstances` 读方法改状态违反 CQS

## 2. 问题陈述

### 2.1 不可变性原则废止（CRITICAL）

| 文件 | 违反方法数 | 具体方法 |
|------|-----------|---------|
| `permission.go` | 2 | `Resolve()`, `Expire()` — 原地修改 Response/Status/RespondedAt |
| `session.go` | 1 | `SetState()` — 原地修改 State/UpdatedAt |
| `milestone.go` | 3 | `SetStatus()`, `SetProgress()`, `AddDependency()` |
| `taskflow.go` | 4 | `Start()`, `AdvanceToNext()`, `Fail()`, `SetStatus()` |
| `connection/manager.go` | 3 | `Register()`, `Heartbeat()`, `handleConnectionLost()` |
| `instance/registry.go` | 2 | `Register()`, `GetInstances()` 含副作用 |
| `events.go` | 1 | `WithMetadata()` 变异 receiver map |
| **合计** | **16** | 遍及 7 个文件 |

**影响**: 规范与代码的矛盾会导致开发者对规范体系失去信任。新加入者按规范做会被代码审查驳回（"大家都这么写"）。

### 2.2 运行时类型安全漏洞（HIGH）

`internal/layers/communication/connection/manager.go:301`:

```go
event.Data.(*types.EventConnectionLostData) // unsafe assertion
```

当 `emitEvent` 被 `handleConnectionRestored` 调用时，`Data` 是 `*EventConnectionRestoredData`，直接 panic。这是一个**确定性的运行时崩溃**。

### 2.3 D1/D6 L5 验收测试为零（HIGH）

| 域 | L5 总数 | IMPLEMENTED | PLANNED | 完成率 |
|----|---------|-------------|---------|--------|
| D1 Communication | 5 | 0 | 5 | **0%** |
| D6 Evolution | 2 | 0 | 2 | **0%** |

D1 是用户交互的第一入口（会话管理、命令解析），D6 是运维关键（版本/配置管理）。

### 2.4 命名与代码异味（MEDIUM）

| 问题 | 位置 | 描述 |
|------|------|------|
| `CLRenderer` 缺 `I` | `renderers/message.go:12` | 应为 `CLIRenderer` |
| `min` 重复定义 | `renderers/status.go:59` | Go 1.21+ 已有 built-in `min` |
| `GetInstances` 副作用 | `instance/registry.go:104` | 读方法修改 `inst.Status` 违反 CQS |

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | 不可变性规范明确化：要么全量重构为不可变模式，要么修正 CLAUDE.md 描述为"有限可变"并划定允许变异的边界 | P0 |
| AC2 | `connection/manager.go:301` type assertion 修复，消除 panic 风险 | P0 |
| AC3 | D1 Communication 域 5 个 L5 从 PLANNED 转为 IMPLEMENTED 或标注明确的排期 | P0 |
| AC4 | D6 Evolution 域 2 个 L5 补全 | P1 |
| AC5 | `CLRenderer` 更名为 `CLIRenderer` | P1 |
| AC6 | `min` 函数删除，切换为 Go built-in | P1 |
| AC7 | `GetInstances` 消除副作用，分离为读 + 显式健康刷新 | P2 |
| AC8 | 所有变更通过 S4-Gate（code review）后方可合入 | P0 |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 约束 | 不可变性重构不改变公开接口签名 |
| 约束 | D1 L5 测试必须使用已有 build tag 体系（acceptance） |
| 约束 | `CLRenderer` 改名涉及 `NewCLIRenderer()` 工厂函数及所有调用方 |
| 不依赖 | 外部包 |

## 5. L1–L5 映射草案

| 层级 | 资产 |
|------|------|
| L1 | Architecture / Communication / Evolution |
| L4 | 不可变性规范、type assertion 修复、L5 补全、命名修正 |
| L5 | L5-0-1-01~09（见 L5 注册表） |

## 6. 变更范围

### 6.1 新增

- D1/D6 L5 测试文件（具体路径见 design.md）

### 6.2 修改

- `internal/shared/types/permission.go` — 不可变性重构
- `internal/shared/types/session.go` — 不可变性重构
- `internal/shared/types/milestone.go` — 不可变性重构
- `internal/shared/types/taskflow.go` — 不可变性重构
- `internal/shared/types/events.go` — 不可变性重构
- `internal/layers/communication/connection/manager.go` — type assertion 修复
- `internal/layers/communication/renderers/message.go` — `CLRenderer` 改名
- `internal/layers/communication/renderers/status.go` — 删除 `min`，改用 built-in
- `internal/layers/communication/instance/registry.go` — `GetInstances` 副作用消除
- `openspec/l5-registry.md` — 新增 L5 测试点登记

### 6.3 不变更

- 业务逻辑行为
- 公开接口签名（不可变性重构使用 `With*` 方法而非 `Set*`）
- 配置文件格式
- CI/CD 流程

## 7. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 不可变性重构影响范围大 | 中 | 逐文件渐进修改，每个文件独立提交 + review |
| 现有调用方依赖 `Set*` 方法 | 中 | 保留旧方法但标记 `@Deprecated`，先追加 `With*` |
| D1 L5 测试需要真实 Adapter | 低 | 使用 mock adapter + VCR |
| `CLRenderer` 改名影响第三方 Adapter | 低 | 暂无非飞书 adapter |
