# Proposal: D7 MUPS 5 节点 frame delta 闭环

**Change ID:** `devrix-d7-mups-frame-delta-closure`
**Demand ID:** DM-20260705-010
**Created:** 2026-07-05
**Status:** S1_Proposal
**Demand:** [`demand.md`](demand.md)
**OpenSpec YAML:** [`.openspec.yaml`](.openspec.yaml)

---

## 1. Problem Statement

D7 MUPS 5 节点管道（Observe→Plan→Execute→Verify→Learn）的设计原则是"**不确定性驱动渐进消解**"。但 trace `38144cebcf8dda7a123827d96a731bc5`（sess_1783255992426_6000, wi_d0_s0_goal）实测显示，5 节点之间的 LLM I/O 协议未达成此目标——链路是 5 个独立 LLM 调用拼成的序列，而非逐步收敛过程。

### 三大根因

#### 根因 #1 — Observe 输入太薄（断链 A）

Observe 节点的 LLM user frame **仅含 `directive` 本身**（69 字符）。LLM 在 24 秒推理时间里只能基于 directive 文本猜——它看不到：

- 上一轮 Execute 工具调用历史
- Plan 已规划的 scope_in（Plan 还没跑过）
- 历史上同类 directive 的成功路径
- 本 WorkItem 的 SemanticID / Depth / SiblingIndex

→ Observe 输出是"问题清单"而非"假设路径"。

#### 根因 #2 — Plan → Execute 信号丢失（断链 B）

Plan 节点的 LLM system_prompt 要求返回 `execution_mode` + `child_specs` + `deliverable_contract`，但 **Execute 节点的 system_prompt 是固定的"你正在分层工作树中执行一个 WorkItem"**，根本没把这三项作为 frame 字段注入。Execute 5 个 sub-turn 全部用同一份固定 prompt。

→ LLM 不知道自己是不是 decompose 子任务、期望产出 schema 是什么、哪些不确定性是 Observe 已经标记的。

#### 根因 #3 — 缺失 delta 回写

Execute 第 2-5 轮 prompt tokens 从 3487 涨到 7229（+107%），这部分增长**全部来自累积的 tool_result**，**没有任何来自 Observe 或 Plan 的结构化 delta**。也没有 Execute → Observe 的"已收敛度"度量回写。

→ 链路是 5 个独立调用，不是 1 个收敛过程。

### 实证数据

```
trace 38144cebcf8dda7a123827d96a731bc5 / sess_1783255992426_6000 / wi_d0_s0_goal:

Observe:  1 LLM call  | input 69 chars   | output 3 obs_uncertainty (str 0.85/0.80/0.75)
Plan:     1 LLM call  | input 928 chars  | output execution_mode=decompose + 2 child_specs
Execute:  5 LLM calls | input 2225+Δ    | sub-turn 1 盲执行 → sub-turn 4 才定位到 plan 目录
          tokens 3487 → 4625 → 4879 → 6848 → 7229  (+107% 全部来自累积 tool_result)
Verify/Learn/Decide: 0 LLM call (deterministic)
```

## 2. Proposed Solution（候选方案）

### 方案 A — 最小修复：只补 Plan→Execute 断链

在 Execute system_prompt 追加 `execution_mode` + `child_specs[].directive_suffix` 摘要。

- 优点：1 文件 +30 行，0 LLM 调用增加
- 缺点：Observe→Plan 仍独立，Execute→Observe 无回写
- 范围：~50 行 + 3 测试

### 方案 B — 双端闭合：Observe + Plan→Execute 全部 frame delta

Observe user frame 新增 `prior_artifact_summary`（承接 Execute 上轮收敛度）+ `known_gaps`（承接 Plan 已知缺口）；Execute system_prompt 注入 Plan frame delta。

- 优点：3 节点全部 frame delta，链路闭合
- 缺点：Execute system_prompt 长度 +200 字符风险
- 范围：~300 行 + 8 测试

### 方案 C — 方案 B + convergence_metric 回写（推荐）

在 B 基础上新增 Execute→Observe 的 `convergence_metric`（每 sub-turn 工具结果 diff + claim 数），由 deterministic 计算（0 LLM 调用），通过 Observe user frame 闭合链路。

- 优点：链路显式 Markov 化，每个节点的输入/输出都有显式 delta 度量；Jaeger span 可观测
- 缺点：新增 convergence_metric 计算逻辑（~100 行）
- 范围：~400 行 + 12 测试

**推荐方案 C**——链路显式收敛，Jaeger 可观测，0 行为变化场景下 5 节点重构 M1-M5 落地契约 0 修改。

## 3. 候选方案对比矩阵

| 维度 | 方案 A | 方案 B | 方案 C (推荐) |
|------|--------|--------|----------------|
| 闭合断链 A (Observe 输入) | ❌ | ✅ | ✅ |
| 闭合断链 B (Plan→Execute) | ✅ | ✅ | ✅ |
| 闭合断链 C (Execute→Observe 回写) | ❌ | ❌ | ✅ |
| Execute LLM context 注入量 | +50 chars | +200 chars | +200 chars |
| 新增 LLM 调用数 | 0 | 0 | 0 |
| 新增 Jaeger span 数 | 0 | 0 | 1 (convergence_metric) |
| 代码增量（行）| ~50 | ~300 | ~400 |
| 测试增量（T 点）| 3 | 8 | 12 |
| 现有契约修改 | 0 | 0 | 0 |
| 与 DM-20260705-008 决策表兼容性 | ✅ | ✅ | ✅ |

## 4. 下一步（待 S2 提案 + S3 设计）

- **S2 提案阶段**：在 `tasks.md` 拆解 T 点（D7-S5-A111 Observe delta 字段、D7-S9-A112 Execute frame 注入、D7-S9-A113 convergence_metric），登记到 `openspec/specs/d7-orchestration/t-registry.md`
- **S3 设计阶段**：在 `design.md` 详细设计 frame delta schema、convergence_metric deterministic 算法、Jaeger span 命名
- **S3-Gate**：发起三方博弈论 review（codex + cursor），三方共识后进 S4
- **S4 实现**：分 3 个 Phase（Phase 1 = Plan→Execute 注入；Phase 2 = Observe→Plan 闭合；Phase 3 = convergence_metric 回写），每 Phase 一个 PR

## 5. 关联

- 父 Change: `devrix-d7-orchestration-domain` (DM-20260613-001)
- 兄弟 Change:
  - `devrix-d7-uncertainty-resolution-traceability` (DM-20260704-006) — Obs→Verify→Decide 闭环
  - `d7-mups-strategy-injection` (DM-20260705-008) — Strategy 抽象
  - `mups-go-struct-driven` (DM-20260705-003) — Frame 9 字段契约
  - `mups-node-prompt-dedup` (DM-20260705-004) — Prompt 净化
  - `d7-observe-closed-classifier-prompt` (DM-20260705-009) — 封闭式分类器定位
- 关联 trace: `38144cebcf8dda7a123827d96a731bc5`