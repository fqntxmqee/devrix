# Context Engine V3 Design

**Change ID:** devrix-context-engine-v3
**Layer:** 2 - Context Engine
**Status:** S4 In Progress (Grill signed-off 2026-06-07)
**Version:** 3.0.0-draft
**Based on:** `openspec/archive/2026-06-07-devrix-context-engine-v2/design.md`
**Demand:** DM-20260607-006

---

## 一、架构目标

### 1.1 V3 增量目标

| 业务目标 | V2 | V3 | 量化 |
|---------|----|----|------|
| PEV 阶段 | Execute→Verify | Plan→Execute→Verify | plan.enabled 时 100% 走三阶段 |
| Milestone DAG | 无 | LLM 生成 + 拓扑执行 | 支持 ≤10 milestone/任务 |
| milestone_progress | 通信层占位 | PEV 发射 | 每 milestone 状态变更 ≥1 事件 |
| LongTerm Memory | stub | SQLite | Recall P99 <50ms（本地） |

### 1.2 层间边界

```
Layer 2 (Context Engine V3)              Layer 1 (Communication)
────────────────────────────            ─────────────────────────
PlanEngine.plan() ────────────────────▶ contracts.IMilestonePlanner
  (via bridges/milestone adapter)              └── milestone.Service
PEVEngine ──milestone_progress────────▶ EngineEvent → Gateway 四流
LongTermMemory ──SQLite────────────────▶ ~/.devrix/memory.db（L2 自有）
Plan/Execute LLM ──────────────────────▶ ILLMGateway（Layer 3 via bridge）
```

**禁止：**
- L2 import `communication/milestone` 或 `llmgateway` 具体包
- L1 import `contextengine/pev` 内部实现

### 1.3 PEV 完整循环

```
Process(userMessage)
  → [可选] LongTerm.Recall(query) 注入 system context
  → 压缩管道（V2 不变）
  → if plan.enabled && shouldPlan():
        Plan: LLM → MilestonePlanJSON → validate → IMilestonePlanner.Create*
        emit milestone_progress (0%)
  → for each milestone in executionOrder:
        Execute(milestone context)
        Verify(milestone)
        UpdateProgress / Complete / Fail
        emit milestone_progress
  → else:
        Execute→Verify（V2 路径）
  → [可选] LongTerm.Store(summary)
  → emit complete
```

---

## 二、PEV Plan 阶段

### 2.1 触发条件

```go
func shouldPlan(cfg *config.PlanConfig, state *types.PEVState, msg types.Message) bool {
    if !cfg.Enabled {
        return false
    }
    if strings.HasPrefix(msg.Content, "/plan") {
        return true
    }
    if state.ActiveMilestoneID != "" {
        return false // 已有活跃任务，继续执行
    }
    return cfg.AutoDetect && len(msg.Content) > cfg.MinCharsForPlan
}
```

默认：`plan.enabled=false`（安全回退 V2）。

### 2.2 LLM 输出格式

```json
{
  "task_id": "task_abc",
  "milestones": [
    {
      "id": "ms_1",
      "name": "分析代码结构",
      "description": "...",
      "dependencies": []
    },
    {
      "id": "ms_2",
      "name": "实现修复",
      "description": "...",
      "dependencies": ["ms_1"]
    }
  ]
}
```

校验规则：
- `id` regex `^[a-z0-9_-]+$`，≤10 milestones
- `dependencies` 引用必须存在
- DAG 无环（Kahn 拓扑排序）

校验失败 → 记录 `CTX_PLAN_4020`，降级单步 Execute（不创建 DAG）。

### 2.3 IMilestonePlanner 契约

```go
// internal/shared/contracts/milestone.go

type IMilestonePlanner interface {
    CreateBatch(taskID string, milestones []*types.Milestone) error
    GetExecutionOrder(taskID string) ([]*types.Milestone, error)
    UpdateProgress(id string, progress float64) error
    Complete(id string) error
    Fail(id string, reason string) error
}
```

`bridges/milestone/wire.go` 将 `milestone.IMilestoneService` 适配为上述接口。

---

## 三、Milestone 驱动 Execute/Verify

### 3.1 执行上下文

每个 milestone 执行时，在 system prompt 追加：

```
当前里程碑: {name} ({id})
描述: {description}
依赖已完成: {completed_deps}
```

### 3.2 进度事件

```go
// EngineEvent metadata
{
  "event_type": "milestone_progress",
  "milestone_id": "ms_1",
  "progress": "50%",
  "task": "分析代码结构"
}
```

由 `PEVEngine` 在 `UpdateProgress` / `Complete` 后通过 `eventCh` 发射（与 V1 四流契约一致）。

### 3.3 Verify  per-milestone

复用 V2 `verify_mode`（basic/commands/none）；milestone 级 Verify 失败时：
- `iteration < max` → 重试当前 milestone Execute
- 否则 `IMilestonePlanner.Fail(id, reason)`

---

## 四、LongTerm Memory

### 4.1 Schema

```sql
CREATE TABLE memory_entries (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_memory_topic ON memory_entries(topic);
CREATE INDEX idx_memory_session ON memory_entries(session_id);
```

路径默认 `~/.devrix/memory.db`，配置项 `longterm.db_path`。

### 4.2 API

```go
type ILongTermMemory interface {
    Recall(ctx context.Context, query string, limit int) ([]MemoryEntry, error)
    Store(ctx context.Context, entry MemoryEntry) error
}
```

Recall：`WHERE topic LIKE ? OR content LIKE ? ORDER BY created_at DESC LIMIT ?`

Store：Process 结束时，若 `longterm.auto_store=true` 且 topic 在白名单内写入。

### 4.3 注入上下文

Recall 结果格式化为 system prompt 附录：

```
## 项目记忆（LongTerm）
- [{topic}] {content_preview}
```

上限 `longterm.recall_max_entries`（默认 5），总 token 受 `longterm.recall_max_tokens` 约束。

---

## 五、配置设计

```yaml
context_engine:
  plan:
    enabled: false          # 默认关闭，等同 V2
    auto_detect: true
    min_chars_for_plan: 200
    model: deepseek-v4      # Plan 专用模型
    max_milestones: 10
    timeout: 15s
  longterm:
    enabled: true
    db_path: ~/.devrix/memory.db
    auto_store: false
    topics: ["architecture", "decisions", "bugs"]
    recall_max_entries: 5
    recall_max_tokens: 2000
```

---

## 六、可观测性

### 6.1 IPEVObserver 扩展

```go
type IPEVObserver interface {
    EmitVerifyCommand(sessionID, cmd string, result VerifyCommandResult)
    EmitPlanCompleted(sessionID string, milestoneCount int)           // V3
    EmitMilestoneProgress(sessionID, milestoneID string, progress float64) // V3
}
```

### 6.2 Metrics

| 指标 | 标签 |
|------|------|
| `devrix_ctx_plan_total` | `outcome=success\|degraded\|error` |
| `devrix_ctx_milestone_duration_seconds` | `milestone_id`（基数 ≤10） |
| `devrix_ctx_longterm_recall_total` | `hit=true\|false` |

---

## 七、错误码

| Code | 名称 | 场景 |
|------|------|------|
| CTX_PLAN_4020 | PlanValidationFailed | JSON/DAG 校验失败，降级 |
| CTX_PLAN_4021 | PlanLLMTimeout | Plan LLM 超时 |
| CTX_MEMORY_4005 | FeatureNotImplemented | longterm.enabled=false |
| CTX_MEMORY_4022 | LongTermDBError | SQLite 读写失败 |

> `CTX_MEMORY_4005` 保留；`longterm.enabled=true` 后不再对 Recall 返回此码。

---

## 八、测试策略

| 层级 | 覆盖 L5 |
|------|---------|
| 单元 | L5-CTX-19, 20, 22, 23, 25 |
| 集成 | L5-CTX-21, 24 |
| 验收 P0 | L5-CTX-19, 21, 22 |

---

## 九、Grill 决议记录（2026-06-07）

| # | 问题 | 决议 |
|---|------|------|
| 1 | Plan 模型是否与 Execute 分离 | **是** — `plan.model` 独立，默认 `deepseek-v4`；timeout 15s |
| 2 | milestone_id 是否进入 metrics 标签 | **是** — 单任务 ≤10；禁止 task_id/session_id 组合标签 |
| 3 | LongTerm 是否加密 at-rest | **否** — V3 明文 SQLite，与 V2 快照决议一致 |
| 4 | Milestone 失败后后续节点 | **fail-fast** — Fail 当前节点，跳过后续，info + complete |
| 5 | Plan 默认开关 | `plan.enabled=false`；`longterm.auto_store=false` |
