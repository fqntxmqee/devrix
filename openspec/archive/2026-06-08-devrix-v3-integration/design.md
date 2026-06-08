# Design: Communication V3 集成补全

**Change ID:** devrix-v3-integration
**Demand ID:** DM-20260608-010
**Status:** S3 Planning

---

## 1. 架构边界

### 1.1 执行路径分工

```
                    ┌─────────────────────┐
                    │  D2 PEV milestone   │
                    │  _runner (生产主路径) │
                    └──────────┬──────────┘
                               │ uses
                    ┌──────────▼──────────┐
                    │ MilestoneService    │
                    │ (D1-S5)             │
                    └──────────┬──────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
┌────────▼────────┐  ┌─────────▼─────────┐  ┌──────▼──────┐
│ TaskFlowService │  │ PlannerAdapter     │  │ Gateway     │
│ (编排 API)      │  │ (bridges/milestone)│  │ events      │
└─────────────────┘  └────────────────────┘  └─────────────┘
```

**结论：** 本变更 **不** 将 TaskFlowService 注入 PEV；仅补测试 + 删除误导 stub。

### 1.2 与 DM-009 边界

| 主题 | DM-010 (本变更) | DM-009 (code-integrity) |
|------|-----------------|-------------------------|
| Milestone 可变方法 | 不改 API | 可能引入 With* 模式 |
| GetInstances 副作用 | 仅补 IM 注册测试 | 可能重构 CQS |
| D1 基础 L5 (commands 等) | 不重复 | devrix-d1-d6-testing 负责 |

S4 开发顺序建议：**DM-010 测试与接线优先**，DM-009 重构后 rerun DM-010 L5。

---

## 2. DingTalk 出站渲染

### 2.1 触发协议

扩展 `types.OutboundMessage.Metadata`（无 struct 变更）：

| Key | Value | 行为 |
|-----|-------|------|
| `render` | `milestone` | 需 `Metadata["milestone_json"]` 或 gateway 填充 Milestone 字段（见 2.2） |
| `render` | `taskflow` | 需 TaskFlow 快照或 ID |
| （缺省） | — | 现有 `msg.Content` 文本 |

**最小实现（推荐）：** gateway 在投射 `milestone_progress` 事件为出站时设置：

```go
msg.Metadata["render"] = "milestone"
msg.Metadata["milestone_name"] = name
msg.Metadata["milestone_status"] = string(status)
msg.Metadata["milestone_progress"] = fmt.Sprintf("%.2f", progress)
```

Adapter 内构造临时 `types.Milestone` 供 Renderer（避免 JSON 往返）。

### 2.2 Adapter 伪代码

```go
func (a *DingTalkAdapter) OnMessage(msg *types.OutboundMessage) {
    content := msg.Content
    if msg.Metadata["render"] == "milestone" {
        m := milestoneFromMetadata(msg.Metadata)
        content = NewDingTalkCardRenderer().RenderMilestone(m)
    }
    // ... SendSessionMessage(webhook, content)
}
```

### 2.3 Feishu 对齐（可选 P2）

Feishu 已有 `ProgressStyle`；本变更 **不强制** Feishu 复用 DingTalk Renderer，仅在 design 留扩展点。

---

## 3. Instance Registry on IM

### 3.1 生命周期

```
main()
  → NewInstanceRegistry(60s)
  → Register(InstanceInfo{ID, Name, Address, Port})
  → adapter.Start()
  → <-ctx.Done()
  → adapter.Stop()
  → Unregister(ID)
```

### 3.2 默认 ID

| 入口 | 默认 DEVRIX_INSTANCE_ID |
|------|---------------------------|
| devrix-dingtalk | `devrix-dingtalk` |
| devrix-feishu | `devrix-feishu` |

Port：dingtalk `8081`，feishu `8080`（与现有常量一致）。

---

## 4. Stub 处理

**推荐：删除** `internal/layers/communication/task_flow.go`

理由：
- 无引用（dead code）
- 日志声称「V3 将实现」与现状矛盾

若 grep 发现引用，改为：

```go
// Deprecated: use milestone.TaskFlowService
```

---

## 5. 测试设计

### L5-1-5-01 — Cycle rejection

```go
// Covers: L5-1-5-01
func TestMilestoneService_AddDependency_CycleRejected(t *testing.T) {
    // m1 -> m2 -> m3, then add m1 depends on m3 => error
}
```

### L5-1-5-02 — Multi-milestone chain

```go
// Covers: L5-1-5-02
func TestTaskFlowService_MultiMilestoneChain(t *testing.T) {
    // m1 (no deps) -> m2 (depends m1)
    // Start -> Complete m1 -> Complete m2 -> progress 1.0, status completed
}
```

### L5-1-2-03 — Renderer wired

```go
// Covers: L5-1-2-03
func TestDingTalkAdapter_OnMessage_milestoneRender(t *testing.T) {
    // OutboundMessage with render=milestone metadata
    // assert sendContents[0] contains milestone name / progress bar chars
}
```

### L5-1-8-02 — UI components

```go
// Covers: L5-1-8-02
func TestProgressBar_Render(t *testing.T) { ... }
func TestStatusBadge_Render(t *testing.T) { ... }
```

### L5-1-1-02 — IM instance lifecycle

轻量方案：在 `registry_test.go` 增加 Register→Unregister 集成断言；cmd 级测试标记 P2 optional。

---

## 6. 文档变更

### 6.1 config-environment.md §5 多入口表

新增行：

| 钉钉机器人 | `cmd/devrix-dingtalk/main.go` | `devrix.yaml` + `~/.devrix/config.yaml`（`im.platform.provider=dingtalk`） |

### 6.2 project.md（可选 P2）

D1-S2 Adapters 行补充「钉钉 Webhook」。

---

## 7. OpenSpec 产物清单

| 文件 | S3 状态 |
|------|---------|
| demand.md | ✅ |
| proposal.md | ✅ |
| design.md | ✅ |
| tasks.md | ✅ |
| specs/communication_delta.md | ✅ |
| acceptance-report.md | S5 填写 |
| .openspec.yaml | ✅ |
