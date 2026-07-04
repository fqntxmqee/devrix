# Proposal: D7 MUPS 不确定性驱动 Spawn（Deliverable 解耦）

**Change ID:** `d7-uncertainty-spawn-decouple`  
**Demand ID:** DM-20260704-001  
**Created:** 2026-07-04  
**Status:** S5 Accepted

---

## Problem Statement

MUPS 的设计原则是 **不确定性驱动**：高 U → 发散（decompose / explore）；证据齐、U 降 → 收敛（rollup / terminal）。任务类型（审查、实现、调研）不应决定拓扑。

当前实现把 **DeliverableContract 输出格式**（`planings_meta`、`findings_json` 等）作为 Spawn 的主 continuation 信号。其后果：

1. Execute 完成 scope 内探索（工具读取、U 实际下降），Verify 因 **格式** 判 incomplete → **SpawnInline** 在同一 leaf 重跑。
2. Strategic Plan 可选 `execution_mode=single`，锁死 CommitmentPlan，**跳过** decompose 与 rollup 收敛通道。
3. 三轮格式失败后 **escalate_human**，且 rationale 误标为 R7 indeterminate。

用户侧表现：探索做了、汇总没做、会话像中断——根因是 **收敛相未触发**，不是审查任务特殊。

---

## Proposed Solution

引入 **Uncertainty Spawn Contract（CC-U1～U6）**：

| 契约 | 要点 |
|------|------|
| **CC-U1** | Spawn continuation **主信号** = `UncertaintyMean` + 子树拓扑 + 证据进度；deliverable incomplete **单独** 不得触发 depth-0 第 N 次 inline→escalate |
| **CC-U2** | Deliverable verify 职责 = **呈现/提取**（CC-1.5、StructuredDeliverable）；不阻塞 explore→rollup 链 |
| **CC-U3** | Partial + 证据进度高 + U 低于阈值 → **RollupSynth**（`NeedsRollup` 或 ephemeral synthesis WI） |
| **CC-U4** | Strategic `single` 需 `U < SingleModeThreshold`；否则 coerce decompose |
| **CC-U5** | Observe 注入 verify/deliverable **结构化 ObsSignal** 参与下轮 U |
| **CC-U6** | `spawnRationale` 准确区分 CC-1.2 inline / R7 indeterminate / rollup exhausted |

---

## Scope

### In Scope
- `SpawnPolicyEvaluator` 决策树调整
- `deliverableContinuationRequired` 与 spawn 解耦（保留 terminalization）
- Rollup-on-format-failure 路径
- StrategicPlan single-mode U gate
- Observe signal 扩展
- spawnRationale 修正
- 单测 + stub 集成 + OpenSpec delta

### Out of Scope
- 任务类型分支或 review 关键词
- Execute 战术 prompt 重写
- findings_json parser 别名（可独立 hotfix）
- 回滚 DM-20260703-001 CC-1.2 计数器本身

---

## Impact Analysis

| Component | Change | Details |
|-----------|--------|---------|
| `spawn_policy.go` | **Yes** | 新 R-U 规则；Partial 分叉看 U+evidence |
| `strategic_plan_proposer.go` | **Yes** | single 模式 U gate |
| `rollup_gate.go` | **Yes** | `MaybeRollupSynthOnFormatFailure` |
| `item_observe.go` | **Yes** | deliverable/verify ObsSignal |
| `deliverable_contract_verify.go` | **Minor** | 可选：planning_meta 仅检 JSON 体（Phase 2） |
| `session_complete.go` | **Yes** | salvage before task_incomplete |
| D1 Feishu | **Indirect** | 更多 rollup complete，更少假中断 |

---

## Architecture Considerations

- **与 DM-20260703-001 互补**：DM-001 定义 CC-1 terminalization / rollup gate **存在**；本 change 定义 **何时进入** rollup vs inline。
- **PlanKind 语义不变**：Commitment / Exploration 仍由 `MatchKind` 产出；改变的是 **Partial 后 Spawn 查 U 先于 deliverable**。
- **LP-5 血缘保留**：Rollup synth 仍产 Artifact → Verify → Learn，SourcePlanID 可追溯。
- **Anti-pattern 禁止**：不在 Go 中写「review 应 decompose 为 N 路」战术；仅 U + scope 覆盖率 + DivergenceBudget。

---

## Success Criteria

- [x] L5-D7-U-01～U-05 单测/集成全绿
- [ ] 复现会话类路径：读文件 + 格式失败 → rollup synth，非 3× inline + escalate（T-ACC-2/3 手动）
- [x] `spawnRationale` 无 R7 误标（CC-1.2 场景）
- [ ] OpenSpec delta 合入 `openspec/specs/d7-orchestration/`（S5 验收后，T-DOC-2）

---

## Risks & Mitigations

| Risk | Prob | Impact | Mitigation |
|------|------|--------|------------|
| 过多 rollup 轮次增 latency | Med | Med | 每 WI 最多 1 次 format-failure rollup；复用 MaxRollupRetries |
| U 阈值误触发 decompose | Med | Med | 表驱动测试 + `EvidenceProgress` 与 tool_calls 双条件 |
| 与 DM-001 规则冲突 | Low | High | design.md 显式规则顺序；同文件注释交叉引用 |
