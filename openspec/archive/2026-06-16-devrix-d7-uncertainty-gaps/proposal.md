# Proposal: D7 不确定性处理能力缺口修复

**Change ID:** devrix-d7-uncertainty-gaps
**Demand ID:** DM-20260616-001
**阶段:** S2 Proposal
**版本:** v1.0

---

## 1. 背景

2026-06-16 对 D7 编排层进行了设计理念 vs 代码实现的忠实度审查。D7 的五层不确定性管理体系（分类→探索→规划→执行→反馈）在设计层面完备，但代码实现存在 5 个关键缺口，涉及安全、可观测性、可用性和并发正确性。

## 2. 目标

修复 5 个关键缺口，使代码实现与设计理念对齐：

1. **PlanAgent 运行时门控**: 将 tool call 白名单从 prompt-only 升级为运行时拦截
2. **PlanModeApproveGate**: 决策实现门控逻辑或移除死配置
3. **ConflictGuard TOCTOU**: Allow+Register 合并为原子操作
4. **OrchestratePath sink**: 恢复 FlowEvent → sink 推送
5. **PlanMode nil LLM**: Enter() 即失败，避免状态不一致

## 3. 非目标

- 不重构 WaveScheduler dispatch loop 整体架构
- 不改变 RuleClassifier 置信度门控逻辑
- 不修改 DAG 校验或 LLM→rule 回退链路
- 不新增 DSAFT S/A 层（此为修复性变更，非能力新增）

## 4. 方案概述

### Gap 1: PlanAgent 运行时门控

在 `PlanAgent` 中增加 `ValidateToolCall(name string) error` 方法，由 tool 执行管线在调用前检查。PlanAgent 的 prompt 注入保持不变（defense-in-depth 的第二层）。

### Gap 2: PlanModeApproveGate

**方案 A（推荐）**: 移除 `PlanModeApproveGate` 配置项。原因是当前 PlanMode 的 Approve 流程通过 CLI 命令（`/plan approve`）显式触发，不需要额外配置开关——用户始终需要显式 approve。

**方案 B**: 在 OrchestratePath.Run() 中增加门控检查，当 `PlanModeApproveGate=true` 时，未 approve 的 plan 不触发 Wave 调度。

推荐方案 A：减少配置复杂度，Approval 由 CLI 命令自然实现。

### Gap 3: ConflictGuard 原子化

新增 `AllowAndRegister(candidate TaskNode, slotID SlotID) bool` 方法，在同一临界区内完成检查和注册，消除 TOCTOU 窗口。原 `Allow()` + `Register()` 保留但标记为内部使用。

### Gap 4: OrchestratePath sink 推送

修改 `emit()` 函数，调用 `sink.Publish()` 推送事件。需要在 `OrchestratePath` 构造时传入有效的 `EventPublisher`。

### Gap 5: PlanMode nil LLM

在 `PlanMode.Enter()` 中增加 `p.planAgent == nil || p.planAgent.llm == nil` 检查，返回明确错误。

## 5. 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| Gap 1 拦截层需要接入 tool pipeline | 中 | 在 PlanAgent 内部独立实现，通过接口暴露，不修改 D2 tool pipeline |
| Gap 3 原子化改变 API | 低 | 新增方法，原方法保留兼容 |
| Gap 4 sink 可能为 nil | 低 | emit() 增加 nil check |
| Gap 5 可能影响现有 /plan 命令测试 | 低 | 测试中传 nil LLM 的场景需要更新断言 |

## 6. Decision

| 决策 | 选择 | 理由 |
|------|------|------|
| PlanAgent 门控方式 | **内部拦截层** | 不依赖 D2 pipeline 修改，独立可控 |
| PlanModeApproveGate | **方案 A：移除** | 减少死配置；Approve 由 CLI 命令自然实现 |
| ConflictGuard API | **新增 AllowAndRegister** | 向后兼容，渐进迁移 |
| sink nil 处理 | **nil check + skip** | 兼容测试环境（sink 为 nil） |
| PlanMode nil LLM | **Enter() 显式报错** | 快速失败，避免状态不一致 |

## 7. DSAFT 场景与活动

| 场景 | 活动 | T 点 |
|------|------|------|
| S2 IntentFast | A06 TurnOrchestrator.runLoop | D7-S2-A06-T05 (PlanAgent runtime gate) |
| S3 WaveScheduler | A01 DispatchTask | D7-S3-A01-T03 (ConflictGuard atomic), D7-S3-A01-T04 (sink emit) |
| S5 Exploration | A02 PlanMode lifecycle | D7-S5-A02-T05 (nil LLM guard), D7-S5-A02-T06 (approve gate cleanup) |
