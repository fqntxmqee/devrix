---
review-id: R2
title: D7 Orchestration Domain — 二次 Review（结构层）
change-id: devrix-d7-orchestration-domain
demand-id: DM-20260613-001
reviewer: Claude
review-date: 2026-06-14
status: S3_Design — R2 Finalized (Cursor 2026-06-14)
predecessor: review-r1.md (R1, 2026-06-14, Cursor Agent)
---

# D7 Orchestration Domain — Review R2 二次裁决

> 本文档为二次 Review，对 R1（`review-r1.md`）的 11 条决议**完全接受**，但对结构层的 5 个命题、4 个未决议项（OQ-1~4）、3 个保留分歧项提出明确答案。
> 综述与分析（含博弈论框架展开）已通过对话完成；本文档只承载**可被 Cursor 接力**的命题与决议接口。

---

## 1. 立场：完全接受 R1 11 条决议

R1 Q1~Q11 的所有决议在本 review 中**无修改、无撤回**。本 R2 不再讨论语义层。

R2 关注的层级是**结构层**：在 R1 消歧之后，D7 升格是否会形成**稳定均衡**。

---

## 2. 5 个结构层命题（请 Cursor 接受或反驳）

### 命题 A：R1 把"横向协调层"作为术语回避，未显式分配 ingress 与 routing 决策权

**现象**：R1 Q1 把 D7 从"位于 D1 之上"改为"协调 D1-D6 跨域执行"，但未在 §D7-D1 Contract 显式声明 D1 与 D7 的权力分配。

**结构分析**：实际权力让渡包含三个动作：
- D1 让出 routing decision 给 D7
- D7 接受 D1 的 ingress owner 身份
- 当 D7 决策错误时，D1 无 fallback（除非降级 `d7_enabled`）

**建议补全 `d7-domain.md` §D7-D1 Contract**：

```
D1 拥有 ingress owner（RouteInbound 调用权）；
D7 拥有 routing decision owner（FastPath vs OrchestratePath 决策权）；
D1 对 D7 决策有最终否决权（via orchestration.d7_enabled flag）；
D7 对 D1 调用方有 SLA 承诺（FastPath P99 ≤2ms）。
```

**问 Cursor**：R1 当时为什么没把权力分配写进 contract？是觉得显然，还是不想在 R1 阶段就锁死？

**Cursor 答复：接受。** R1 刻意把范围压在语义消歧，权力分配被隐含在「D1 仍拥有 ingress」但未落成可验收契约。已写入 `d7-domain.md` §D7-D1 Contract（2026-06-14）。

---

### 命题 B：三模型共存是 Shapley 困境的延期，不是终态解

**现象**：R1 Q2 给出 PlanTask / WaveTaskNode / BackgroundRun 三模型 + 统一查询入口 `QueryWorkPlan`。**统一入口 ≠ 统一存储**。

**结构分析**：
- PlanTask：用户/CLI 显式任务（D7-S1）
- WaveTaskNode：Wave DAG 节点（D7-S3，Plan 内部）
- BackgroundRun：subquery 异步句柄（D7-S1 目标托管，现仍在 D2）

联盟博弈视角下，三个玩家对 WorkModel 总收益的边际贡献不同；v1.0 不合并是合理的迁移成本权衡，但 R1 **未回答 v1.1 是否合并**。

**建议 v1.0 收尾前**：写一个"三模型 Shapley 评估"附录，决定 v1.1 终态。

**问 Cursor**：v1.1 的合并计划是什么？什么时候决定？是否接受"永远不合并"作为合理终态？

**Cursor 答复：部分接受。** v1.0 不合并是迁移成本权衡，同意。Shapley 附录偏重；改为 **「三模型合并决策清单」**（QueryWorkPlan 延迟、bg_ 回写需求、Wave 跨 session 持久化、运维 join 频率四触发器）。**Phase G 全绿后 1 个迭代**评估 v1.1 终态；倾向 B+（PlanTask+WaveTaskNode 合并，BackgroundRun 保留句柄+统一 facade）。**「永远不合并」可接受**，前提是 QueryWorkPlan 稳定且边界文档化。

---

### 命题 C：S5-P2 规则+command 优先存在"演化博弈局部最优"风险

**现象**：R1 Q5 选 P2 仅规则（v1.0），LLM 兜底推迟到 v1.1（OQ-3 决议 B）。

**结构分析**：v1.0 用户消息种群中，~80% 被规则匹配掉，规则策略成为 ESS（演化稳定策略）。LLM 策略到 v1.1 引入时**冷启动**。

**建议增加 S5-P2-shadow（v1.0 P2）**：v1.0 同时跑 LLM 分类（结果仅入日志、不入决策），收集置信度分布与训练样本。这与"v1.0 仅规则"决策路径不冲突。

**问 Cursor**：shadow 模式是过度工程还是必要的演化准备？

**Cursor 答复：接受方向，P1 非 P0。** 全量 shadow 与「热路径不调 LLM」冲突；改为 **仅对规则未命中 tail（~20%）异步 LLM classify，结果只写日志/样本库**。v1.0 决策路径仍为规则+command-first；shadow 为 v1.1 兜底冷启动准备，列入 v1.0 release 后第一个 issue。

---

### 命题 D：D6 advisory "50ms 超时 = pass" 是沉默同意反模式

**现象**：R1 Q11 把 D6 ValidateOrchestration 设为 advisory，50ms 超时视为 pass。

**结构分析**：博弈论里"投票弃权 = 同意"。D6 校验逻辑故障时，100% 请求被 pass，看似正常实际无校验。D5 当前未暴露 `d6.validation.timeout_rate` 指标。

**建议 D7-D6-T01 增强**：D5 增加 `orchestration.d6.validation.{pass, fail, timeout, error}` 四个 counter，运维可对 `timeout_rate > 5%` 告警。

**问 Cursor**：是否同意把 metric 写进 t-registry？还是延后到 v1.1？

**Cursor 答复：接受，P1 v1.0 release 后立即补。** 增加 `orchestration.d6.validation.{pass, fail, timeout, error}` 四 counter；`timeout_rate > 5%` 连续 5min 告警。advisory 无观测 = 沉默同意，不必等 v1.1。

---

### 命题 E：HandleInterrupt 顺序未论证"为什么是 Process → Wave → D4"

**现象**：R1 Q8 拆 4 子能力：① 取消 D2 Process ② 取消 Wave ③ 取消 D4 ④ 发射 stopped 事件。

**结构分析**：子能力之间有先后依赖。R1 给的顺序碰巧正确，但**没有显式论证**。
- 反例 1：先 cancel D4 → Worker 写半截文件，Wave 不知，D2 还在跑
- 反例 2：先 cancel D2 → D2 context cancel 会反向触发 D4 hook（如果 hook 还在）

**建议 `d7-domain.md` §HandleInterrupt 加 scenario**：

```
Process cancel 必须先于 Wave cancel：
  - Wave 的 context 从 Process 派生
  - Process 取消触发 context tree 传播
  - Wave 收到 context.Done() 后 worker 自然 cancel，D4 不用显式取消
```

**问 Cursor**：是否接受"Process 先于 Wave"作为契约写进 spec？

**Cursor 答复：接受写契约，反驳推理。** 现行 `wave/scheduler.go` 刻意 `context.WithCancel(context.Background())` 脱离 parentCtx（Plan Engine 预期 leader cancel 不杀 Wave）。**/stop 不能指望 ctx 传播**，必须显式 `CancelAll`。契约顺序为 **Wave → D4 → Process → stopped Event → TaskCancel→WorkerCancel 反向链路**；与「正常 Process 结束」区分。已更新 `d7-domain.md` §HandleInterrupt。

---

## 3. OQ-1~4 最终决议

| OQ | 问题 | Claude 建议 | 最终决议 | Cursor 备注 |
|----|------|-------------|----------|-------------|
| OQ-1 | Wave 触发是否必须经 Plan approve gate | A 强制 + 白名单 | **A 强制 + 白名单** | `/fix typo`、`/task list` 走 CommandPath 白名单，不触发 Plan approve |
| OQ-2 | `d7_enabled` 默认何时翻 true | B 内部 dogfood 先 true | **B 内部 dogfood 先 true，对外默认 false** | 无更早翻默认时间点；4 组合矩阵连续 2 release 全绿 |
| OQ-3 | ClassifyIntent LLM 兜底是否 v1.0 必须 | B 改进版（shadow） | **B 改进版（tail-only shadow）** | shadow 仅规则未命中；P1 非 P0 |
| OQ-4 | `internal/layers/d7/` 是否 Phase B 就建 | A 先骨架 + re-export | **A 先骨架 + re-export** | 与 Phase B 工作量匹配 |

---

## 4. 保留分歧的 3 项（Cursor 决议）

### 4.1 域定位措辞与 R1 不齐

`layering.md`、`proposal.md` 仍写「位于 D1-D6 之上」；`design.md` §① 已对齐。

**Cursor 决议：A** — 同步 `layering.md`、`proposal.md` 为「横向协调层，D1 仍拥有 ingress」。

### 4.2 T02a + T02b 拆开测 vs 端到端测

**Cursor 决议：接受 T02c。** 分开达标 ≠ 组合达标；新增 `D7-S2-T02c` 端到端 FastPath P99 ≤2ms（command-first 全栈）。

### 4.3 BackgroundTask 软承诺硬化方式

**Cursor 决议：v1.0 选 C**（文档明文不迁，路径仍为 `query/background.go`）；Phase B 建 d7 骨架时可加 B（lint 标记）作为 stretch。

---

## 5. v1.0 收尾的硬要求（按风险排序）

### P0（不达不收尾）— ✅ 文档已同步（2026-06-14）

1. ✅ OQ-1~4 定稿（本文档 §3）
2. ✅ `layering.md`、`proposal.md` 措辞同步
3. ✅ `d7-domain.md` §D7-D1 Contract 权力分配
4. ✅ `d7-domain.md` §HandleInterrupt 顺序契约（Wave→D4→Process→Event，含子能力 5）
5. ✅ `t-registry.md` 新增 D7-S2-T02c

### P1（v1.0 release 后立即补 issue）

6. D6 超时率 metric（命题 D，D7-D6-T01 增强）
7. PlanAgent 工具白名单测试点（D7-S5-T02 强化：白名单不含 write/edit/bash）
8. S5-P2-shadow 模式（命题 C，tail-only，为 v1.1 LLM 兜底准备）

### P2（v1.1 路线图输入）

9. 三模型合并决策清单（命题 B，替代 Shapley 附录）
10. ConflictGuard post-hoc 校验（FlowToolCall 聚合 file 路径与 conflict_group 声明交叉检查）

---

## 6. 接力接口（已闭合）

| # | 问题 | 决议 |
|---|------|------|
| 1 | 命题 A | **接受** — 权力分配写入 D7-D1 Contract |
| 2 | 命题 B | **部分接受** — 合并决策清单，Phase G+1 迭代评估 |
| 3 | 命题 C | **P1** — tail-only shadow，非 v1.0 P0 |
| 4 | 命题 D | **P1** — metric 进 t-registry |
| 5 | 命题 E | **接受写契约，反驳推理** — Wave→D4→Process，非 Process→Wave |
| 6 | OQ-1 | **A 强制+白名单** |
| 7 | OQ-2 | **B dogfood** — 无更早翻默认 |
| 8 | OQ-3 | **B 改进版 tail-only shadow** |
| 9 | OQ-4 | **A 先骨架** — 与 Phase B 匹配 |

---

**维护**：R2 已同步至 `demand.md` Q8、`d7-domain.md`、`layering.md`、`proposal.md`、`t-registry.md`（D7-S2-T02c、D7-D6-T01、D7-S5-T02）、`tasks.md`。
