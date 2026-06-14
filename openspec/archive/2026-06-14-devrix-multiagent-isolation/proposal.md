# Proposal: 多 Agent 会话隔离 — Join 合并与 Session 元数据隔离

**Change ID:** devrix-multiagent-isolation
**Demand ID:** DM-20260611-005
**Status:** S2_Proposal
**Priority:** P1

## 1. Background

DM-012 QueryLoop v2 已部分缓解多 Agent 消息竞态：Agent 层独立 `messageBuffer`、Worker `ProcessOverlay`、Fork 消息 prefix。但仍存在真实风险：

- Fork 子 Agent 仍持有父 `*types.Session` 指针（session ID / metadata 共享）
- Join 合并语义未保证排序/去重
- 并发 Fork 时 Session 元数据 / ContextSnapshot 写入未 `-race` 验证
- 与 DM-007 Wave Scheduler SubAgent `ContextPolicy=fresh` 需对齐

## 2. Problem Statement

| 问题 | 位置 | 严重度 |
|------|------|--------|
| 共享 `*types.Session` 指针 | `agent/agent.go` Fork → `creator.Create(..., a.session)` | P0 |
| Join 合并无排序去重 | `agent/agent.go` Join | P0 |
| 压缩触发跨 Agent | D2 compression | P0 |
| 并发 Fork 元数据竞争 | `factory/factory.go` | P0 |
| Wave Scheduler fresh policy 缺 API | DM-007 协调 | P1 |

## 3. Proposed Solution

### 3.1 Join 排序 + 去重

- `Join()` 收集所有子 Agent 完成事件
- 按"agent 完成时间戳"排序（不依赖原始 channel 顺序）
- `tool_call` ID 去重：同一 call_id 多 Agent 回传时取最早
- 集成测试：3 个并发 Fork Agent（不同延迟），Join 后父 Agent 上下文一致

### 3.2 Fork Metadata COW

- Fork 时创建 `SessionView`（不是完整 Session 拷贝）：
  - 共享：SessionID、CreatedAt、Model、TokenBudget（只读）
  - 隔离：metadata、ContextSnapshot、Messages（COW）
- 子 Agent 写 metadata 走 `view.SetMetadata(key, value)`，不影响父
- 父 Agent 写 metadata 需显式 `parent.UpdateFromView(view)` 才同步
- 压缩只读父 view，不修改

### 3.3 ForkSessionView API

```go
// multiagent/factory/factory.go
type SessionView struct {
    ID        string
    CreatedAt time.Time
    Model     string
    TokenBudget types.TokenBudget
    // COW 字段
    metadataMu sync.RWMutex
    metadata   map[string]any
    snapshot   []byte
}

func ForkSessionView(parent *types.Session) *SessionView
func (v *SessionView) SetMetadata(key string, value any)
func (v *SessionView) GetMetadata(key string) (any, bool)
func (v *SessionView) MergeToParent(parent *types.Session) error
```

### 3.4 与 Wave Scheduler 衔接

- DM-007 `ContextPolicy=fresh` 调用 `ForkSessionView(parent)` 注入 SubAgent
- Join 时通过 `MergeToParent` 选择性同步 artifact（metadata 子集 + ContextSnapshot）
- 不全量灌入父 Messages（节省内存）

### 3.5 SessionIsolationProbe

- D6 评测：测量并发 Fork 数量、metadata 写次数、Join 后一致性
- D5 指标 `runtime.fork_session_view_total{policy="cow|snapshot|shared"}`
- CI 阻断 metadata 竞争

## 4. Success Metrics

| 指标 | 基线 | 目标 |
|------|------|------|
| Join 排序一致性 | 不保证 | 100% |
| tool_call ID 去重 | 无 | 100% |
| Fork metadata 写操作不污染父 | 不保证 | 100% |
| 并发 3 Fork + Join `-race` | FAIL | PASS |
| Wave Scheduler fresh policy 接入 | N/A | 完成 |
| SessionIsolationProbe 注册 | N/A | 完成 |

## 5. Implementation Plan

| Phase | 内容 | 估时 |
|-------|------|------|
| P1 | SessionView 数据结构 + ForkSessionView API | 0.5d |
| P2 | Join 排序 + tool_call 去重 | 1d |
| P3 | metadata COW + 并发 `-race` 测试 | 1d |
| P4 | MergeToParent 选择性同步 | 0.5d |
| P5 | SessionIsolationProbe + D5 指标 | 0.5d |
| P6 | DM-007 ContextPolicy resolver 对接 | 0.5d |
| **Total** | | **4d** |

**合并策略**：2 个 PR（SessionView + Join 排序 / COW + D6 探针），可独立回滚。

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| Join 排序变更影响依赖追加顺序的测试 | 全量 PathRegression + Agent 集成测试先到位 |
| metadata COW 大 Session 拷贝开销 | 共享 slice header + 写时复制字段；压测 1000 keys 场景 |
| ForkSessionView 与 Wave Scheduler 协调 | DM-007 同步推进；resolver 协议先 PR |
| 旧 `*types.Session` 直接持有行为 | 加 deprecation warning；下个版本再禁 |

## 7. Out of Scope

- 每个 Fork 完整复制 Session（内存开销大；Wave fresh policy 只需 directive + 元数据子集）
- 重复 DM-012 SubQuery / Delegate 已实现的 overlay 隔离
- Fork 策略注入（CopyOnWrite / Snapshot / Shared）— v2.0
- Task 抽象（local_bash / local_agent）— v1.1
