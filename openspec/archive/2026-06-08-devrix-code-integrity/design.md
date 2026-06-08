# Design: Devrix 代码健康规范

## 1. Root Cause Analysis

### 1.1 不可变性原则失效

| 原因 | 说明 | 证据 |
|------|------|------|
| 规范与实际脱节 | CLAUDE.md 不可变性条款在项目早期设定，未随 Go 社区实践调整 | 项目使用 `sync.RWMutex`，天然支持通过 method 控制的 mutation |
| 缺乏可操作定义 | "不可变性"未区分值对象 vs 实体 vs service，团队无法达成共识 | 代码 review 从未拒收可变方法 |
| Go 指针 receiver 默认可变 | Go 编译器不强制不可变，需要额外纪律 | `func (p *PermissionRequest) Resolve()` 在 Go 中完全合法 |

### 1.2 Type assertion panic

```go
// manager.go:297-302
func (m *ConnectionManager) emitEvent(event *types.DomainEvent) {
    slog.Debug("emitting event",
        "type", event.Type,
        "connection_id", event.Data.(*types.EventConnectionLostData).ConnectionID, // ← panic!
    )
}
```

`handleConnectionRestored()` (line 281-293) 也调用 `emitEvent`，但它的 `Data` 是 `*EventConnectionRestoredData`。这是两个 call site 共享同一个 `emitEvent` 但未做类型判断的设计缺陷。

### 1.3 D1/D6 L5 空白

D1 Communication 作为最早开发的层，当时 L5 体系尚未建立，后续也未回溯补全。D6 Evolution 作为辅助层，优先级一直低于业务层。

### 1.4 命名问题来源

`CLRenderer` 为历史拼写遗留，`min` 函数在 Go 1.18 版本时自定义是正确做法（当时无 built-in `min`）。`GetInstances` 的副作用是在读路径上顺手更新状态，属于 CQS 违反。

## 2. Solution Design

### 2.1 Group A: 分层不可变性规范

**追加到**: `openspec/specs/project/coding.md` §9 "代码完整性"

追加内容：

```markdown
## 9. 代码完整性

### 9.1 不可变性分层策略

不再一刀切禁止原地修改，而是按类型分类管控：

| 类型类别 | 允许变异 | 要求 | 示例 |
|---------|---------|------|------|
| 值对象 (Value Object) | ❌ 不可变 | `With*()` 返回新副本 | Attachment, AuthConfig |
| 聚合根/实体 (Aggregate/Entity) | ✅ 受控可变 | method + 内部锁，禁止直接赋值字段 | Session, Milestone |
| Service/Manager | ✅ 内部可变 | 状态只读对外暴露 | AuthService, SessionStore |
| 基础设施 | ✅ 自由 | Timer, sync.Mutex, map | — |

### 9.2 值对象不可变模式

```go
// CORRECT — With* 返回新副本
func (a *Attachment) WithName(name string) *Attachment {
    return &Attachment{
        Type: a.Type, Name: name,
        Path: a.Path, Content: a.Content,
    }
}

// WRONG — 直接赋值
a.Name = name
```

### 9.3 实体受控可变模式

```go
// CORRECT — 显式 method + 内部锁
func (s *Session) SetState(state SessionState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.State = state
    s.UpdatedAt = time.Now()
}

// WRONG — 外部直接赋值字段
s.State = SessionStateThinking
```

### 9.4 不安全类型断言

禁止使用 `.(*ConcreteType)` 形式的非安全类型断言。必须使用 type switch 或 `ok` 模式：

```go
// CORRECT — type switch
switch data := event.Data.(type) {
case *types.EventConnectionLostData:
    // 处理
default:
    slog.Warn("unknown event type")
}

// WRONG — 直接断言，类型不匹配时 panic
event.Data.(*types.EventConnectionLostData)
```

### 9.5 命令查询分离 (CQS)

读方法（Get/List/Count 等）不得修改状态。需要刷新状态时使用独立写方法：

```go
// CORRECT — 分离读和写
func (r *InstanceRegistry) GetInstances() []*InstanceInfo  // 只读
func (r *InstanceRegistry) RefreshHealth() error           // 显式刷新

// WRONG — 读方法有副作用
func (r *InstanceRegistry) GetInstances() []*InstanceInfo  // 内部修改了 Status
```
```

**CLAUDE.md 修正**:
```
- 不可变性: 创建新对象，禁止原地修改
+ 不可变性: 值对象不可变 (`With*` 返回新副本)；实体通过 method 加锁变更状态。详见 `openspec/specs/project/coding.md` §9
```

### 2.2 Group B: Type Assertion 修复

**决策**: type switch + 逐 type 日志

```go
// manager.go
func (m *ConnectionManager) emitEvent(event *types.DomainEvent) {
    switch data := event.Data.(type) {
    case *types.EventConnectionLostData:
        slog.Debug("emitting event",
            "type", event.Type,
            "connection_id", data.ConnectionID,
        )
    case *types.EventConnectionRestoredData:
        slog.Debug("emitting event",
            "type", event.Type,
            "connection_id", data.ConnectionID,
        )
    default:
        slog.Warn("emitting unknown event type",
            "type", event.Type,
            "data_type", fmt.Sprintf("%T", event.Data),
        )
    }
}
```

### 2.3 Group C: D1 L5 测试方案

**优先级策略**: 核心交互路径 P0 立即实现，辅助功能 P1 同 Change 实现但不阻塞交付。

| L5 ID | 描述 | 优先级 | 类型 | 位置 | 策略 |
|-------|------|--------|------|------|------|
| L5-1-1-01 | 新会话创建被拒绝 | **P0** | Acceptance | `tests/acceptance/p0/comm_gateway_flow_test.go` | mock Gateway 返回错误，断言错误传播 |
| L5-1-3-01 | /new 命令解析正确 | **P0** | Acceptance | `tests/acceptance/p0/comm_commands_test.go` | 表驱动测试 |
| L5-1-3-02 | /help 命令解析正确 | **P0** | Acceptance | 同上文件 | 同上 |
| L5-1-3-03 | /stop 命令解析正确 | **P0** | Acceptance | 同上文件 | 同上 |
| L5-1-2-01 | 飞书消息解析正确 | **P1** | Unit | `internal/layers/communication/adapters/feishu_test.go` | 构造飞书 webhook JSON |
| L5-1-8-01 | ShortId 唯一性 | **P1** | Unit | `internal/shared/types/shortid_test.go` | 已 PLANNED，同 Change 实现 |

### 2.4 Group D: D6 L5 排期

> 当前无正式版本发布计划文档。以下版本号为基于 `devrix.yaml` v2.0.0 的合理建议，实际排期以未来版本规划为准。

| L5 ID | 描述 | 当前状态 | 建议版本 | 排期理由 |
|-------|------|---------|---------|---------|
| L5-6-1-01 | 版本检测与记录 | PLANNED | v2.1.0 (TBC) | 需等版本管理模块接入 CI |
| L5-6-2-01 | 配置热更新 | PLANNED | v2.2.0 (TBC) | 需等待配置中心设计完成 |

### 2.5 Group E: 命名/异味

#### E1: CLRenderer → CLIRenderer

```go
// message.go
- type CLRenderer struct {
+ type CLIRenderer struct {

- func NewCLIRenderer(ansi config.ANSIConfig) *CLRenderer {
+ func NewCLIRenderer(ansi config.ANSIConfig) *CLIRenderer {
```

影响调用方：
- `renderers/message.go` — 定义
- `renderers/status.go` — 使用（通过 `NewCLIRenderer`）
- `cmd/` — CLI 启动时注册

#### E2: 删除自定义 min

```go
// status.go:59
- func min(a, b int) int { if a < b { return a }; return b }
```

Go 1.21+ built-in `min` 同等语义，直接删除即可。

#### E3: GetInstances 副作用消除

```go
// instance/registry.go:104-105 — 删除以下两行
- if now.Sub(inst.LastSeen) > r.timeout {
-     inst.Status = "unhealthy"
- }
```

**简化理由**：
- `GetInstances` 无生产代码调用方（仅测试使用）
- 删除副作用后测试依然通过（Status 由 `Register()` 设置）
- 已有的 `HealthCheck()` 方法已覆盖显式状态刷新场景
- 无需新增 `RefreshHealthStatus`

## 3. Key Interfaces / Types

### 3.1 Value Object 不可变模式

```go
// With* 返回新副本（值对象标准模式）
func (a *Attachment) WithName(name string) *Attachment {
    return &Attachment{
        Type:    a.Type,
        Name:    name,
        Path:    a.Path,
        Content: a.Content,
    }
}
```

### 3.2 Entity 可变方法规范

```go
// 实体允许 method 控制的可变，但必须加锁 + 显式命名
func (s *Session) SetState(state SessionState) {  // ✅ 允许
    s.mu.Lock()
    defer s.mu.Unlock()
    s.State = state
    s.UpdatedAt = time.Now()
}

s.State = SessionStateThinking  // ❌ 禁止直接赋值
```

## 4. Data Flow

本 Change 不涉及数据流变更。所有更改均为代码重构和规范更新。

## 5. File Manifest

### 新增

| 文件 | 归属 Group | 说明 |
|------|-----------|------|
| `tests/acceptance/p0/comm_gateway_flow_test.go` (追加) | C | L5-1-1-01 |
| `tests/acceptance/p0/comm_commands_test.go` (追加) | C | L5-1-3-01~03 |
| `internal/layers/communication/adapters/feishu_test.go` (追加) | C | L5-1-2-01 |

### 修改

| 文件 | 归属 Group | 变更内容 |
|------|-----------|---------|
| `CLAUDE.md` | A | 不可变性条款修正 |
| `openspec/specs/project/coding.md` | A | §9 追加"代码完整性"一节 |
| `openspec/l5-registry.md` | C/D | D1 L5 IMPLEMENTED, D6 L5 标注排期 |
| `internal/layers/communication/connection/manager.go` | B | type switch 替换 unsafe assertion |
| `internal/layers/communication/renderers/message.go` | E | `CLRenderer` → `CLIRenderer` |
| `internal/layers/communication/renderers/status.go` | E | 删除自定义 `min` |
| `internal/layers/communication/instance/registry.go` | E | `GetInstances` 删除副作用代码 |

## 6. Regression Risk Assessment

| 变更 | 风险 | 缓解 |
|------|------|------|
| Type assertion 修复 | 低 | 仅修改 panic 行为，不改变业务逻辑 |
| L5 测试追加 | 低 | 新增文件，不影响现有代码 |
| `CLRenderer` 改名 | 中 | grep + CI 编译检查确保无遗漏 |
| `GetInstances` 只读化 | 低 | 调用方读取 status 不受影响 |
| CLAUDE.md 修正 | 低 | 纯文档变更 |
| 不可变性规范发布 | 低 | 新规范，旧方法保留 `@Deprecated` 过渡 |

## 7. Rollback Plan

| 步骤 | 操作 | 条件 |
|------|------|------|
| 1 | git revert 对应 commit | 任何阶段发现问题 |
| 2 | 更新 l5-registry.md 回退 | L5 测试失败 |
| 3 | 恢复 CLAUDE.md 旧文本 | 规范不可接受 |
| 4 | 回退 `CLRenderer` 命名 | 编译失败且有未发现的引用 |

### Decision: 分层不可变 vs 全量不可变

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A: 分层不可变（值对象不可变 + 实体 method 控制） | 兼容 Go 社区实践，迁移成本低 | 不是纯函数式不可变 |
| B: 全量不可变重构（所有类型 With* 模式） | 规范无歧义 | 与 Go 指针 receiver 冲突，工作量极大 |

**选择:** A
**理由:** Go 不是纯函数式语言，`sync.RWMutex` 是标准并发模式。全量不可变在 Go 中需要大量样板代码且与社区惯用冲突。分层策略保留了不可变在值对象上的价值，同时承认实体需要受控变异。
