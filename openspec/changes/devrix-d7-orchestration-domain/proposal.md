# Proposal: D7 Orchestration Domain

**Change ID:** devrix-d7-orchestration-domain
**Demand ID:** DM-20260613-001
**Status:** S3_Design

## 1. Background

Devrix 当前采用 6 域 DSAFT 架构（D1-D6）。分析表明 D2（Context Engine）承担了大量超出其"上下文引擎"业务能力的职责，包括：任务管理（tasks/）、后台任务编排（background.go）、多 agent 委托回调（delegate）、跨 agent 消息队列（queue/）。同时 ORCH 包虽然包含 WaveScheduler（DAG 调度）、ExecutionFlowHub、WorkPlan 读模型等编排能力，但被限定为"读模型包"，没有独立的域身份。

核心矛盾：系统缺少一个**统一的编排域**来回答"做什么、按什么顺序做、谁来做、做得怎么样了"。

## 2. Problem Statement

### 2.1 D2 职责溢出

- D2 `query/loop.go` 直接 import `multiagent/delegate`（DSAFT 跨域 import 违规）
- D2 包含 Task 持久化（tasks/）——任务有独立数据模型和生命周期，不应归上下文引擎
- D2 包含 BackgroundTask 注册表（background.go）——后台任务编排不是"上下文管理"
- D2 `Loop.Run` 400+ 行混合 LLM↔Tool 交互原语与编排逻辑（delegate hooks、queue drain、attachments）

### 2.2 规划阶段缺失

PEV 退役后，系统没有正式的"规划"活动。Explore/Plan/Execute 三个本质不同的场景共用同一个 `Loop.Run()`，仅靠工具列表区分。

### 2.3 ORCH 身份不足

ORCH-S3 WaveScheduler 已具备 DAG 调度、WorkerPool、ConflictGuard、ContextPolicy，但被定义为"读模型包"，无法主导执行路径。D2 的 delegate 回调绕过 ORCH 直接操作 D4。

### 2.4 决策能力散落

意图分类、任务路由、阶段转换等决策散落在 LLM prompt 和 D2 循环隐式逻辑中，没有结构化的决策层。

## 3. Proposed Solution

升格 ORCH 为 **D7 Orchestration Domain**（编排域），作为 D1-D6 之上的协调层：

```
D7 Orchestration Domain
  ├── D7-S1: Work Model       — Task/Plan 数据模型单一权威来源
  ├── D7-S2: Session Orchestrator — 新主入口，替代 D1→D2.Process
  ├── D7-S3: Wave Scheduler   — 从 ORCH-S3 升格（DAG 调度）
  ├── D7-S4: Execution Flow   — 从 ORCH-S1/S2 升格（事件聚合）
  └── D7-S5: Decision & Planning — 新增（意图分类 + 任务拆解）
```

核心变更：

1. **入口上移**：D1 `RouteInbound` 从调用 `D2.Process` 改为调用 `D7.ProcessMessage`
2. **D2 瘦身**：Task、Background、Queue、Delegate 回调从 D2 迁出
3. **决策显式化**：D7-S5 提供规则+LLM 分层的意图分类与任务拆解
4. **ORCH 升格**：现有 ORCH 包代码迁移到 D7-S3/D7-S4，业务语义不变

## 4. Success Metrics

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| D2 `loop.go` 行数 | ≤200 行（从 ~430 行） | `cloc` |
| D2 import D4 的包 | 0 | `go vet` / import lint |
| 快速路径延迟增量 | ≤2ms（D7 路由开销） | benchmark |
| Task 模型唯一归属 | 100% Task 操作经 D7-S1 | 代码审计 |

## 5. Implementation Plan

| 阶段 | 内容 | 影响范围 |
|------|------|----------|
| **Phase 1**：D7 域定义 + A/F 注册 | 创建 D7 包骨架、注册表、layering 更新 | 文档 + 接口 |
| **Phase 2**：D7-S1 Work Model | Task 数据模型从 D2/D4/ORCH 统一迁入 | D2 tasks/, ORCH workplan/ |
| **Phase 3**：D7-S5 Decision & Planning | 意图分类 + 任务拆解 | 新增代码 |
| **Phase 4**：D7-S2 Session Orchestrator | 新主入口，D1 调用上移 | D1 gateway, D2 engine |
| **Phase 5**：D7-S3/S4 升格 | ORCH 现有代码迁移 | ORCH 包路径变更 |
| **Phase 6**：D2 瘦身 | 移除 D2 中迁出的职责 | D2 query/loop, tasks/ |
| **Phase 7**：回归验收 | P0 T 层全绿 + 覆盖率 ≥80% | 全量测试 |

## 6. Risks & Mitigations

| 风险 | 可能性 | 影响 | 缓解 |
|------|--------|------|------|
| 快速路径性能退化 | 中 | 中 | D7-S2 `ExecuteFastPath` 作为零分配 proxy |
| D2→D7 迁移期间双入口 | 高 | 中 | Feature flag + 渐进迁移 |
| Task 数据模型兼容性 | 中 | 高 | 向后兼容的序列化格式 + 迁移脚本 |
| 现有 delegate 回调耦合 | 高 | 高 | Phase 4 先做 adapter wrapper，再改调用链 |

## 7. Out of Scope

- D6 校验规则的完善（D6-S4 已有基础，非本 change）
- D2 压缩管道的重构（D2-S2 保持，仅移除编排逻辑）
- D4 agent 本身的改造（D4-S2 RunAgent 保持，仅迁移调用方）
