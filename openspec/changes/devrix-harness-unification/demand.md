---
demand-id: DM-20260611-004
title: Harness 路径统一 — 消除双代码路径分支
source: devrix-harness-architecture-audit
priority: P0
status: S1_Proposal
l1-domain: context-engine
created: 2026-06-11
---

# Harness 路径统一

## 1. 背景

当前 ContextEngine.Process() 中存在 harness / non-harness 双代码路径分支。根据是否启用 Harness，行为完全不同：压缩策略、PEV 执行、工具调用各有两套实现。对比 Claude Code Harness 使用统一的 queryLoop() 覆盖所有场景，双路径导致维护成本和缺陷概率翻倍。

## 2. 问题陈述

### 2.1 ContextEngine.Process() 双路径

| 维度 | harness 路径 | non-harness 路径 |
|------|-------------|-----------------|
| 压缩 | Harness 级压缩流水线 | 独立压缩逻辑 |
| PEV 执行 | 通过 Harness Bootstrap | 直接调用 PEVEngine |
| 工具调用 | 经 Harness 路由 | 直接调用 ToolRunner |
| 状态管理 | HarnessSession | 裸 ContextEngine |

**根因**：Harness 作为"可选增强"被设计为附加层，而非核心路径的默认行为。

### 2.2 分支引入的维护成本

| 问题 | 影响 |
|------|------|
| 新增特性需改两处 | 压缩策略、工具注册、权限检查均需同步 |
| 测试覆盖分裂 | 两个路径的测试套件独立，易遗漏边界 |
| 缺陷概率倍增 | 不同路径下相同语义行为可能不一致 |

### 2.3 Claude Code 的对照

Claude Code 没有"harness 模式"开关。它的 main.tsx → queryLoop() 是所有请求的唯一路径。Bootstrap、Prefetch、Compression、Tool Execution 都是查询循环的组成部分而非可选附加层。这也是它支持 Coordinator Mode、Bridge Remote、Swarm 等多种部署模式但代码路径始终唯一的根本原因。

## 3. 验收标准

### P0 (阻止合并)

- [ ] 消除 ContextEngine.Process() 中的 harness/non-harness 分支，统一为单一执行路径
- [ ] HarnessBootstrap、PreflightEvaluator、PromptRouter 等组件从"可选附加"重构为"核心流程的组成部分"
- [ ] 统一压缩流水线：不再区分 harness 级压缩和引擎级压缩

### P1 (必须完成)

- [ ] 统一测试套件覆盖原双路径的所有行为，P0 L5 验收通过
- [ ] 工具注册和权限检查经过统一路径路由
- [ ] 过渡期内提供兼容 shim（日志警告），允许旧调用方逐步迁移

### P2 (建议完成)

- [ ] 移除所有与双路径相关的废弃代码和条件分支
- [ ] 统一路径事件日志完整记录路径决策过程
- [ ] 兼容 shim 使用率指标：通过 D5 监控旧路径调用频次，辅助判断迁移完成时机
- [ ] 实现路径一致性评测探针（`PathRegressionProbe`）：双路径行为对比测试 + 旧路径调用归零监控，注册到 D6 Eval 框架

## 4. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `contextengine/engine` | Process() 路径合并 | 高 |
| `contextengine/harness` | 组件内联至核心流程 | 高 |
| `contextengine/compression` | 压缩流水线统一 | 中 |
| `contextengine/pev` | PEV 执行统一 | 中 |
| `multiagent/forkjoin` | 多 Agent 使用统一路径 | 中 |
| `observability/metrics` | 双路径迁移指标 | 低 |
| `d6/eval` | PathRegression 探针 | 低 |

## 5. 回归风险

- Harness 路径的既有功能可能因路径合并而退化，需完整的行为对比测试
- 统一路径后的性能不得低于原双路径中较高者
- 外层调用方（Gateway、Multi-Agent）需验证与新路径的兼容性
