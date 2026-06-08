---
demand-id: DM-20260608-010
title: Communication V3 集成补全 — 接线、L5 登记、测试与文档
source: V3 交付复盘（DM-20260608-008）
priority: P1
status: Delivered
l1-domain: D1
parent-demand: DM-20260608-008
created: 2026-06-08
---

# Demand: Communication V3 集成补全

## 1. 背景

`devrix-v3`（DM-20260608-008）已于 2026-06-08 归档。库级能力（Milestone、TaskFlow、钉钉 Adapter、UI 组件、Instance Registry）已落地并通过单元测试，但复盘发现：

1. **L5 ID 悬空**：demand 使用 `L5-COMM-14`~`18`，未登记至 `openspec/l5-registry.md`
2. **热路径未接线**：TaskFlow、DingTalkCardRenderer、IM 侧 Instance Registry 未接入生产路径
3. **测试缺口**：环检测、多里程碑 TaskFlow 链、ProgressBar/StatusBadge 无专项测试
4. **文档债务**：`task_flow.go` V1 stub 与实现矛盾；`config-environment.md` 缺 dingtalk 入口

本需求在 **不扩大 V3 原始 scope** 前提下，闭合上述集成与规范缺口。

## 2. 澄清 Q&A

| # | 问题 | 决策 |
|---|------|------|
| Q1 | TaskFlow 与 PEV `milestone_runner` 关系？ | **PEV 仍为执行主路径**；TaskFlowService 保留为 D1-S5 编排 API，需补多里程碑单测 + 文档说明边界，不在本变更重写 PEV |
| Q2 | 钉钉卡片渲染触发条件？ | 出站消息 `Metadata["render"]=="milestone"` 或 `ContentType=="milestone"` 时走 `DingTalkCardRenderer`，否则仍发纯文本 |
| Q3 | Instance Registry 是否接入 feishu/dingtalk？ | **是（P2）**：启动 Register、Shutdown Unregister，与 CLI 行为对齐 |
| Q4 | 是否重做 DM-008 归档包？ | **否**：归档包只读；本变更通过 parent-demand 追溯，S7 独立归档 |
| Q5 | Prometheus `/metrics`、LB sticky？ | **仍 Out of Scope**（与 DM-008 一致） |

## 3. L5 映射（修正 DM-008 悬空 ID）

| DM-008 临时 ID | 正式 L5 ID | 描述 | 优先级 |
|----------------|------------|------|--------|
| L5-COMM-14 | **L5-1-5-01** | Milestone 环检测拒绝循环依赖 | P1 |
| L5-COMM-15 | **L5-1-5-02** | TaskFlow 多里程碑链顺序执行至完成 | P1 |
| L5-COMM-16 | **L5-1-2-02** | 钉钉 Webhook 入站路由 + Session 出站 | P1 |
| L5-COMM-17 | **L5-1-8-02** | ProgressBar / StatusBadge 渲染输出合法 | P2 |
| L5-COMM-18 | **L5-1-1-02** | IM 入口实例 Register / Unregister | P2 |
| —（新增） | **L5-1-2-03** | 钉钉 milestone 出站走 CardRenderer | P1 |
| —（新增） | **L5-1-5-03** | 删除/替换 V1 TaskFlow stub，无误导日志 | P2 |

## 4. In Scope

- 在 `l5-registry.md` 登记上表 L5（Status=PLANNED → 开发后 IMPLEMENTED）
- 补单元/集成测试（见 `design.md`）
- DingTalk Adapter 出站渲染接线
- `devrix-feishu` / `devrix-dingtalk` Instance Registry 生命周期
- 移除 `communication/task_flow.go` stub 或替换为 V3 委托
- 更新 `openspec/specs/project/config-environment.md` 多入口表
- 本变更 OpenSpec 四件套 + S5 验收报告

## 5. Out of Scope

- 钉钉 WebSocket 长连接
- `config/instance.go` YAML 配置
- Prometheus `/metrics` 端点（D5 observability 承担）
- LB sticky session / X-Forwarded-For
- TaskFlow 替换 PEV milestone_runner
- 不可变性重构（见 DM-20260608-009 `devrix-code-integrity`）

## 6. 验收标准（Given-When-Then）

### L5-1-5-01
- **Given** 里程碑 A→B→A 将形成环
- **When** `AddDependency` 被调用
- **Then** 返回 error，DAG 不变

### L5-1-5-02
- **Given** DAG 含 m1→m2 两节点无其他依赖
- **When** TaskFlow Start 后依次 CompleteMilestone
- **Then** OverallProgress=1.0，Status=completed

### L5-1-2-02
- **Given** 合法钉钉 text webhook payload
- **When** POST `/dingtalk/webhook`
- **Then** RouteInbound 被调用且 HTTP 200

### L5-1-2-03
- **Given** 出站消息标记 milestone 渲染
- **When** DingTalkAdapter.OnMessage
- **Then** SendSessionMessage 内容为 CardRenderer 输出（非裸 Content）

### L5-1-8-02
- **Given** progress=0.5、status=in_progress
- **When** ProgressBar / StatusBadge Render
- **Then** 非空且含预期百分比/状态符号

### L5-1-1-02
- **Given** devrix-dingtalk 启动
- **When** Register 成功且进程退出
- **Then** Unregister 被调用，Registry 中实例移除

## 7. 依赖

| 依赖 | 说明 |
|------|------|
| DM-20260608-008 | V3 库代码基线 |
| DM-20260608-009 | 并行变更；不可变重构若改 Milestone/TaskFlow API，本变更 S4 前需 rebase 对齐 |

## 8. 变更历史

| 日期 | 阶段 | 说明 |
|------|------|------|
| 2026-06-08 | S1/S2 | 复盘缺口，创建 demand |
| 2026-06-08 | S3 | proposal/design/tasks/delta 规划 |
