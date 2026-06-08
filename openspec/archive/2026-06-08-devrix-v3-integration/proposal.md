# Proposal: Communication V3 集成补全

**Change ID:** devrix-v3-integration
**Demand ID:** DM-20260608-010
**Parent:** DM-20260608-008 (devrix-v3)
**Status:** S3 Planning
**Priority:** P1

---

## Motivation

DM-20260608-008 交付了 V3 **库能力**，但未完成 **规范登记、热路径接线、关键测试与文档同步**。若不补全，会出现：

- L5 验收无法按注册表执行（悬空 ID）
- 钉钉 milestone 消息仍以纯文本出站，Renderer 成为死代码
- TaskFlow stub 与 V3 实现并存，误导维护者
- IM 部署形态无实例注册，多实例运维不可观测

## Goals

| Goal | Priority | 成功信号 |
|------|----------|----------|
| L5 正式登记并映射 DM-008 | P1 | `l5-registry.md` 含 L5-1-5-01~03、L5-1-2-02~03、L5-1-8-02、L5-1-1-02 |
| 测试闭合 P1 缺口 | P1 | 环检测 + 多里程碑 TaskFlow 单测 green |
| DingTalk 渲染接线 | P1 | L5-1-2-03 IMPLEMENTED |
| 清理 stub + 文档 | P2 | 无 V1「未实现」日志；config-environment 含 dingtalk |
| IM Instance 生命周期 | P2 | feishu/dingtalk Register/Unregister |

## Non-Goals

- 不重新打开 DM-008 归档包内容
- 不引入 DM-008 已延后的 metrics/LB/WebSocket scope

## Technical Approach

### 1. L5 规范化

将 DM-008 中 `L5-COMM-14`~`18` **映射**为 D-S 格式 ID，写入 `openspec/l5-registry.md`，Status 初始 PLANNED。

### 2. 测试补全（S4 前设计已定）

| L5 | 测试位置（规划） |
|----|------------------|
| L5-1-5-01 | `milestone/service_test.go` — `TestAddDependency_CycleRejected` |
| L5-1-5-02 | `milestone/taskflow_test.go` — `TestTaskFlowService_MultiMilestoneChain` |
| L5-1-2-02 | 已有 `dingtalk_test.go`（标注 Covers） |
| L5-1-2-03 | `dingtalk_test.go` — `TestOnMessage_usesCardRendererForMilestone` |
| L5-1-8-02 | `renderers/components_test.go` — ProgressBar + StatusBadge |
| L5-1-1-02 | `instance/registry_test.go` 或 `cmd` 级 smoke（轻量） |

### 3. 出站渲染接线

```
OutboundMessage
  ├─ Metadata["render"] == "milestone"  → DingTalkCardRenderer.RenderMilestone
  ├─ Metadata["render"] == "taskflow"   → DingTalkCardRenderer.RenderTaskFlow
  └─ default                            → plain text (existing)
```

与 Feishu `ProgressStyle` 模式对齐，避免破坏现有文本回复。

### 4. Instance Registry on IM

复用 `cmd/devrix/main.go` 模式：

- 环境变量 `DEVRIX_INSTANCE_ID` / `DEVRIX_INSTANCE_NAME`
- Start → Register；defer Stop → Unregister
- Port 取自 adapter 监听端口

### 5. Stub 清理

删除 `internal/layers/communication/task_flow.go`，或改为薄包装调用 `milestone.TaskFlowService` 并 `IsSupported() == true`（design.md 详述，推荐删除）。

## Impact

| 组件 | 变更 |
|------|------|
| `adapters/dingtalk.go` | OnMessage 渲染分支 |
| `cmd/devrix-feishu`, `cmd/devrix-dingtalk` | Instance Registry |
| `milestone/*_test.go`, `renderers/*_test.go` | 新测试 |
| `openspec/l5-registry.md` | 新增 7 条 L5 |
| `openspec/specs/project/config-environment.md` | 多入口表 |
| `communication/task_flow.go` | 删除 |

## Risks

| Risk | Mitigation |
|------|------------|
| 与 DM-009 不可变重构冲突 | S4 开始前检查 `milestone.go`/`taskflow.go` API；测试用行为断言 |
| Renderer 改变钉钉消息格式 | 仅 metadata 触发；默认路径不变 |
| IM 实例 ID 冲突 | 文档约定 env 前缀 `dingtalk-` / `feishu-` |

## Success Criteria

- [ ] 全部 P1 L5 在 registry 中为 IMPLEMENTED
- [ ] `go test ./internal/layers/communication/...` 全绿
- [ ] acceptance-report.md verdict=ACCEPTED
- [ ] S7 归档至 `openspec/archive/2026-06-08-devrix-v3-integration/`
