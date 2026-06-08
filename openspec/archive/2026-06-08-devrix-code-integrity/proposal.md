# Proposal: Devrix 代码健康规范

**Change ID:** devrix-code-integrity
**Demand ID:** DM-20260608-009
**Status:** S5 Accepted

---

## 1. Background

Devrix 经历多轮功能迭代后，代码规模快速增长。CLAUDE.md 中的"不可变性"等核心规范与实际代码产生显著偏差，同时 `connection/manager.go` 中存在确定性的 panic 风险，D1/D6 的 L5 验收测试完全空白。若不系统性治理，规范将沦为摆设，技术债务持续累积。

## 2. Problem Statement

### 2.1 核心问题

| 问题域 | 严重程度 | 当前状态 | 目标状态 |
|--------|---------|---------|---------|
| 不可变性形同虚设 | CRITICAL | 16 个方法违反，7 个文件涉及 | 规范与代码一致，新代码按不可变模式编写 |
| Type assertion panic | HIGH | 线上运行时确定崩溃 | 零 unsafe assertion，使用 type switch |
| D1 L5 零覆盖 | HIGH | 5/5 PLANNED | 5/5 IMPLEMENTED |
| D6 L5 零覆盖 | HIGH | 2/2 PLANNED | 2/2 IMPLEMENTED |
| 命名/代码异味 | MEDIUM | 3 处不一致 | 清理完成 |

### 2.2 方案对比

#### 不可变性：两条路径

| 维度 | 路径 A: 全量不可变重构 | 路径 B: 规范修正 + 有限可变 |
|------|----------------------|--------------------------|
| 修改文件数 | ~7 个核心文件 + 调用方 | 1 个规范文件 |
| 风险 | 中（调用方需适配） | 低 |
| 长期价值 | 高（真正不可变） | 中（承认现实） |
| 与 Go 惯用 | 部分冲突（指针 receiver 是 Go 习惯） | 更符合 Go 社区习惯 |
| 新开发者学习 | 需理解不可变模式 | 无额外学习 |

**选择**: **路径 B（规范修正 + 有限可变）**，理由：
1. Go 的指针 receiver 模式天然支持可变，全量不可变与 Go 社区惯用冲突
2. 项目中 `sync.RWMutex` 保护的 setter 是 Go 标准做法
3. 改为"值对象不可变、实体有限可变"的分层策略，既保留不可变价值，又避免与 Go 惯用冲突
4. 具体：`PermissionRequest`、`Session`、`Milestone`、`TaskFlow` 等**聚合根/实体**允许通过 method 控制状态变更（需加锁），但**值对象**（如 `Attachment`、`MilestoneDAG` 返回的快照）必须不可变

#### D1/D6 L5 补全：实现 vs 排期

| 维度 | 立即实现 | 标注排期 |
|------|---------|---------|
| 风险控制 | 最好 | 中等 |
| 资源消耗 | 高 | 低 |

**选择**: D1 立即实现（P0，入口域优先级最高），D6 标注明确排期（P1）

## 3. Proposed Solution

### 3.1 整体策略

```
┌─────────────────────────────────────────────┐
│          devrix-code-integrity               │
├─────────────────────────────────────────────┤
│  Group A: 规范修正 (CLAUDE.md + 新规范)      │
│  Group B: type assertion 修复               │
│  Group C: D1 L5 测试补全                    │
│  Group D: D6 L5 排期登记                    │
│  Group E: 命名/异味清理                     │
└─────────────────────────────────────────────┘
```

### 3.2 Group A: 规范分层策略

**新规范引入**: `openspec/specs/project/coding-integrity.md`

将不可变性要求从"一刀切"重构为分层策略：

```
分层不可变性策略:
┌───────────────┬──────────────┬──────────────────────┐
│ 类型类别       │ 允许变异      │ 示例                  │
├───────────────┼──────────────┼──────────────────────┤
│ 值对象         │ ❌ 不可变    │ Attachment, AuthConfig │
│ 聚合根/实体    │ ✅ method 控制│ Session, Milestone     │
│ Service/Manager │ ✅ 内部可变  │ AuthService, Store     │
│ 基础设施       │ ✅ 自由      │ RWMutex, Timer        │
└───────────────┴──────────────┴──────────────────────┘
```

**关键约束**:
- 值对象必须通过 `With*` 方法返回新副本
- 实体的状态变更必须使用显式 method（如 `session.SetState()`），不允许直接赋值
- 违反检测纳入 code review checklist

### 3.3 Group B: Type Assertion 修复

`connection/manager.go:emitEvent` 改用 type switch + safe assertion:

```go
func (m *ConnectionManager) emitEvent(event *types.DomainEvent) {
    switch data := event.Data.(type) {
    case *types.EventConnectionLostData:
        slog.Debug("emitting event", "type", event.Type, "connection_id", data.ConnectionID)
    case *types.EventConnectionRestoredData:
        slog.Debug("emitting event", "type", event.Type, "connection_id", data.ConnectionID)
    default:
        slog.Warn("emitting event with unknown data type", "type", event.Type)
    }
}
```

### 3.4 Group C: D1 L5 测试补全

| L5 ID | 描述 | 测试位置 | 策略 |
|-------|------|---------|------|
| L5-1-1-01 | 新会话创建被拒绝 | `tests/acceptance/p0/comm_gateway_flow_test.go` | mock gateway 拒绝场景 |
| L5-1-3-01 | /new 命令解析正确 | `tests/acceptance/p0/comm_commands_test.go` | 命令解析表驱动测试 |
| L5-1-3-02 | /help 命令解析正确 | 同上文件 | |
| L5-1-3-03 | /stop 命令解析正确 | 同上文件 | |
| L5-1-2-01 | 飞书消息解析正确 | `internal/layers/communication/adapters/feishu_test.go` | 构造飞书消息体 |

### 3.5 Group D: D6 L5 排期登记

| L5 ID | 描述 | 当前 | 目标 |
|-------|------|------|------|
| L5-6-1-01 | 版本检测与记录 | PLANNED | PLANNED + 标注排期至 v2.1.0 |
| L5-6-2-01 | 配置热更新 | PLANNED | PLANNED + 标注排期至 v2.2.0 |

### 3.6 Group E: 命名/异味清理

| 项目 | 操作 | 影响范围 |
|------|------|---------|
| `CLRenderer` → `CLIRenderer` | rename + 所有引用处 | `message.go` 定义，调用方更新 |
| 删除 `min()` 函数 | 改用 Go 1.21+ `built-in min` | `status.go` |
| `GetInstances` 副作用消除 | 读方法不再改状态，分离 `RefreshHealth()` | `instance/registry.go` |

## 4. Success Metrics

| Metric | Target |
|--------|--------|
| 不可变性规范明确化 | `coding.md` §9 追加 + CLAUDE.md 更新 |
| Type assertion 零 unsafe | 所有 `.(*Type)` 调用经审查 |
| D1 L5 实现率 | 5/5 (100%) |
| D6 L5 排期标注 | 2/2 已标注 |
| 命名/异味清理 | 3/3 完成 |
| S4-Gate 合规 | code review 包含不可变性检查项 |

## 5. Implementation Plan

> 纯文档/测试变更先于生产代码变更，确保 P4-P5 有测试保障。

| Phase | 内容 | 风险 | 输出 |
|-------|------|------|------|
| **P1: 规范发布** | `coding.md` §9 追加代码完整性，更新 CLAUDE.md | 无风险 | 规范文档 |
| **P2: D6 排期登记** | `l5-registry.md` 标注 PlannedVersion | 无风险 | 注册表更新 |
| **P3: D1 L5 补全** | 实现 5 个 D1 L5 测试 | 无风险（仅新增测试） | 测试代码 |
| **P4: 安全修复** | type assertion type switch 替换 | 中（有测试保障） | 代码变更 |
| **P5: 命名清理** | `CLRenderer` 改名 + `min` 删除 + `GetInstances` 只读化 | 中（有测试保障） | 代码变更 |

## 6. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 不可变性规范与 Go 社区惯用冲突 | 低 | 分层策略既保留不可变价值，又兼容 Go 指针 receiver |
| L5 测试需要 mock adapter | 低 | `tests/testutil/mock_gateway.go` 已有 |
| `CLRenderer` 改名遗漏引用 | 中 | grep + IDE rename，CI 确保无编译错误 |
| D6 排期过远 | 低 | P2 优先级，容忍 PLANNED 状态 |

## 7. Out of Scope

- 全量不可变重构（采用分层策略）
- 修改 Go 版本或引入新依赖
- 修改 `devrix.yaml` 或 CI/CD 流程
- D6 L5 的立即实现（仅标注排期）
