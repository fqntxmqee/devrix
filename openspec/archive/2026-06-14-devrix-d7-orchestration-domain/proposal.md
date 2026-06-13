# Proposal: D7 Orchestration Domain

**Change ID:** devrix-d7-orchestration-domain
**Demand ID:** DM-20260613-001
**Status:** S2_Clarified (Review R1 incorporated)

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

升格 ORCH 为 **D7 Orchestration Domain**（编排域），作为**横向协调层**编排 D2+D4 跨域执行（D1 仍拥有 ingress）：

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
3. **决策显式化**：D7-S5-P2 提供规则+command-first 分类；自动拆解（S5-P3）推迟 v1.1
4. **ORCH 升格**：现有 ORCH 包代码迁移到 D7-S3/D7-S4，业务语义不变

**Review R1 修订（2026-06-14）：** 见 `demand.md`、`review-r1.md`。核心变更：三模型职责分离、S2/S3 路由矩阵、迁移共存契约、Phase 顺序调整。

## 4. Success Metrics

| 指标 | 目标 | 测量方式 |
|------|------|----------|
| D2 `loop.go` 行数 | ≤200 行（从 ~430 行） | `cloc` |
| D2 import D4 的包 | 0 | `go vet` / import lint |
| 快速路径延迟增量 | ≤2ms（D7 路由开销） | benchmark |
| Task 模型唯一归属 | 100% Task 操作经 D7-S1 | 代码审计 |

## 5. Implementation Plan（修订 — Review R1）

| 阶段 | 内容 | 影响范围 |
|------|------|----------|
| **Phase A**：需求澄清 + 文档 | demand.md、review-r1、tasks.md、d7-domain R1 | 文档 |
| **Phase B**：D7 骨架 + re-export | `internal/layers/d7/`、contracts、feature flag | 接口 |
| **Phase C**：S5-P2 Classify + S2 ProcessMessage | 规则/command-first 分类、FastPath | 新增代码 |
| **Phase D**：HandleInterrupt + D1 入口 | gateway 灰度、`d7_enabled` | D1 gateway |
| **Phase E**：D7-S1 迁移 + D2 瘦身 | tasks/ 迁入、loop 去 hooks | D2 query/loop |
| **Phase F**：D7-S3/S4 包路径迁移 | re-export 桥接 | orchestration/ |
| **Phase G**：回归验收 | P0 T 全绿 + 4 组合迁移矩阵 | 全量测试 |
| **Phase H**（v1.1）：S5-P3 自动拆解 | SynthesizeTaskGraph、CreateWorkPlan | 新增代码 |

> **顺序修订：** Phase D 与 E 同 release 或相邻 release，避免 `d7_enabled=true` 且 loop 仍含 delegate hooks。S3/S4 已实现，Phase F 可与 B 合并。

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
