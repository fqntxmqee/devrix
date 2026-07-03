# Proposal: D7 任务树收敛契约（向下传播 + 向上反馈）

**Change ID:** `d7-convergence-contract`  
**Demand ID:** DM-20260703-001  
**Created:** 2026-07-03  
**Status:** Draft  
**Related:** PR #379 (`fix/d7-session-loop-anomaly-exit`), 会话 `sess_1783064119386_3000` 复盘  
**Demand:** [`demand.md`](demand.md)

---

## Problem Statement

D7 编排域已有完整的 MUPS 五节点管道、SpawnPolicy R0–R8、Rollup Gate 与 Child Bubble 机制，但 **向下传播（decompose/spawn）** 与 **向上反馈（terminal → rollup → session complete）** 之间缺少可执行的 **收敛契约（Convergence Contract）** 文档与代码 invariant。

具体表现：

1. **R1（max depth）在 deliverable 判定之前强制 SpawnInline**，导致 deliverable=complete 的叶子 WI 无法 SpawnNone → 无法 TaskStatus=completed → 父节点 `ShouldRollupAfterChildren` 永不开。
2. **向上 rollup 门禁只看 TaskStatus**，不看 deliverable；与 Verify 层语义断层。
3. **向下 decompose 的 ChildSpecs 缺少 repo 存在性校验**，LLM 幻觉路径可 spawn 并行兄弟，放大向上阻塞。
4. PR #379 移除 16 轮 cap 后，缺少 complementary terminal 闭环，会话可从「8 分钟失败」变为「30+ 分钟不停止」。
5. Review / 测试覆盖停在组件级（`ShouldRollupAfterChildren` 单测），缺少 **4 层 decompose + 并行兄弟** 集成场景。

## Proposed Solution

引入 **Convergence Contract v1** 作为 D7 编排的 SSOT 补充：

- **§CC-1** Round Terminalization：deliverable 无 continuation → 必须 SpawnNone + terminal
- **§CC-2** Downward Propagation：SpawnDecompose 触发条件 + ScopeValidator + 预算
- **§CC-3** Upward Feedback：ChildOutcomeStats → NeedsRollup → Rollup LLM → 逐层上浮
- **§CC-4** Session Exit：有界退出 + best-effort 兜底

详细决策树见 [`design.md`](design.md)（含 **当前 vs 目标** 对比）。

## Scope

### In Scope

- SpawnPolicyEvaluator 顺序调整（R0.5）
- `ApplyRoundTerminalization` 统一 terminal 入口
- `MaxInlineRetriesAtMaxDepth` 计数与 escalate/fail
- `ScopeValidator`（decompose 前）
- `MaybeParentRollup` 扩展（非仅 root Goal）
- `MaybeSiblingBestEffortRollup`
- 集成测试矩阵 T1–T7
- OpenSpec delta spec + pipeline-architecture 交叉引用

### Out of Scope

- 重写 MUPS 五节点
- LLM 直接设置 SpawnPolicy
- 跨 session 调度
- Review 内容质量本身（只解决编排收敛）

## Impact Analysis

| Component | Change Required | Details |
|-----------|-----------------|---------|
| `workmodel/spawn_policy.go` | Yes | R0.5、max-depth inline retry |
| `sessionorchestrator/item_pipeline.go` | Yes | ApplyRoundTerminalization |
| `workmodel/rollup_gate.go` | Yes | sibling best-effort、parent rollup 扩展 |
| `workmodel/scope_validator.go` | Yes | 新文件 |
| `sessionorchestrator/session_loop_signals.go` | Yes | subtree stuck、统一 SessionExit |
| `openspec/specs/d7-orchestration/` | Yes | 引用 convergence contract |
| D2/D3 LLM | No | 接口不变，输入 directive 更结构化 |

## Success Criteria

- [ ] T3 集成测试：1 子 complete + 1 子 stuck @ max depth → session 在 N 轮内 complete/escalate
- [ ] T1：leaf@maxDepth deliverable=complete → SpawnNone + completed，不再被 focus
- [ ] `review d2 领域 kernel目录下代码` 回归：≤ 合理 MUPS 上限内 complete，非 task_incomplete
- [ ] OpenSpec delta spec 与 design.md 决策树与代码一致

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| R0.5 过早 SpawnNone 导致 deliverable 实际不完整 | Med | Med | VerifyDeliverable 仍为 SSOT；集成测试 T1/T2 |
| ScopeValidator 误拒合法路径 | Med | Low | fallback DefaultDecomposeProposer；blocklist 可配置 |
| Sibling best-effort 丢失并行有效工作 | Low | Med | 仅 inline retries 耗尽后触发；保留 complete 子 artifact |
