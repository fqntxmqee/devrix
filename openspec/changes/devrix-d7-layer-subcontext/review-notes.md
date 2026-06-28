# Review Notes — Layer SubContext（供 Claude 讨论）

**Change ID:** `devrix-d7-layer-subcontext`  
**Demand ID:** DM-20260627-003  
**Date:** 2026-06-27  
**Author:** Cursor（基于用户 WorkTree/Context 讨论整理）  
**Status:** Draft — **请 Claude 逐条回应 §5 开放问题**

---

## 1. 一句话摘要

**WorkItem Execute 应从 D2 Materialize 分层 SubContext 取上下文；同层/上下层靠结构化 Signal 协作；Goal 需 ScopeContract 后再 decompose；Obs*  taxonomy 仅 Observe 规则产出，Execute 不每轮自报。**

---

## 2. 目标 / 为什么 / 怎么做 / 风险（Executive）

### 2.1 目标（North Star）

| ID | 目标 |
|----|------|
| LC1 | depth≥1 的 WI 有与 session 不同的 SubContext（prompt/budget/tools） |
| LC2 | 协作靠 Signal（Bubble/Upstream/PeerStatus/Link），非全文互灌 |
| LC3 | D7 只传 partition+policy；messages 仅来自 D2 |
| LC4 | Goal 开放任务先 ScopeContract，未收敛不 decompose |
| LC5 | 向上与 #262 Rollup 一致：structured/summary only |
| LC6 | Execute 产 Signal；Observe 规则映射 Obs*；Execute ReAct 不每轮自报 Obs  taxonomy |

### 2.2 为什么要做

1. **断层：** ContextGraph F1–F4 已落地，但 Execute 仍 `Prepare(sessionID)` — 规则写了 LLM 看不见。  
2. **成本：** session 单桶随 WorkTree 深度线性膨胀 token。  
3. **协作：** 同层 sibling 需要 cohort **域**（ScopeContract），不需要互相读 ReAct。  
4. **收敛：** 开放型 Goal 无边界 decompose 会发散（review 全库类任务）。  
5. **统一：** Materialize 一次建设，Wave / delegate / WorkItem 共用。

### 2.3 怎么做（三件套）

```text
Partition 模型
  session:<sid>           — L0 Goal / 主 Turn
  cohort:<sid>:<parent>   — 兄弟共享 ScopeContract（非 transcript）
  wi:<sid>:<wi_id>        — Execute 默认 append
  agent:<sid>:<agent_id>  — delegate（Phase 3）

Signal 通道
  父→子  ChildDownlink（Directive, ScopeIn/Out, ExpectedReturn）
  子→父  BubbleStructured（已有）+ Summary（已有 #262）
  同层   UpstreamSignal（BlockedBy）+ 可选 PeerStatus（terminal only）

D2 API
  Materialize(partition, policy, directive, signals) → LLM context
  Append(wi:private, msgs)
```

**Feature flag：** `FeatureLayerSubContextEnabled`（default false，Phase 1 验收后开）。

### 2.4 风险（Top 8）

| ID | 风险 | 缓解 |
|----|------|------|
| R1 | 误解「同层共享」= 全文共享 | CG2′ + 集成测试 A∉B payload |
| R2 | Materialize 热路径延迟 | iter 内缓存；轻量 path |
| R3 | CG2 文档/实现不一致 | design v0.4.0 + spec_delta MODIFIED |
| R4 | sandbox 与 cohort 错配 | sandbox → 强制 private |
| R5 | 与 rollup 重复读全文 | RollupSynth 仅 directive + bubbles |
| R6 | PeerStatus 顺序不确定 | terminal-only；wi_id 排序 |
| R7 | ScopeContract LLM 幻觉 | open_questions 阻断；Verify 对照 repo |
| R8 | 迁移双轨 | flag 默认 off；新 partition 增量 |

| R8 | 迁移双轨 | flag 默认 off；新 partition 增量 |
| R9 | Execute 每轮 Obs 标签污染 | 规范禁止；design §6；单测 wi 链 |
| R10 | ScopeContract→Obs 映射遗漏 | R-OBS-1..7 规则表 + item_observe 单测 |

---

## 3. MUPS Observe 四类 × Execute Context（新增讨论）

### 3.1 问题

是否通过 context 要求 LLM **每一轮**对话将问题收敛到 ObsFact / ObsSignal / ObsDeviation / ObsUncertainty？

### 3.2 决议（Draft，已写入 design §6）

| 策略 | 结论 |
|------|------|
| Execute 每轮 ReAct 强制 Obs* | **❌ 拒绝** — 污染 transcript；虚高 ObsFact；违反 G3/LC2/LP-5 |
| Execute terminal 结构化 Signal | **✅** — LastRound, ScopeContract, Bubble |
| Observe 规则 Signal → Obs* | **✅** — 延续 `item_observe.go`；Strength/Evidence 可单测 |
| Goal ScopeContract → ObsUncertainty 门控 | **✅** — `open_questions` → R-OBS-1 → 阻断 decompose |
| Execute 软引导 `<conclusion>` / `<open_questions>` | **✅ 可选** — 非 SoT |
| Observe LLM Proposer（PR-A4） | **⏸ Phase 2** — 提案 + 规则裁决 |

### 3.3 三层边界

```text
Execute context  →  Signal（ScopeContract, LastRound, Bubble）
Observe 规则     →  Observation（Obs*）→ UncertaintyReport
Plan 规则        →  PlanKind（MatchKind）
```

**请 Claude 评估：** 是否与 pipeline-unification G3 完全一致；Phase 2 ObservationProposer 是否应独立 change。

---

## 4. 与已合并工作的关系

| 已交付 | 本 change 关系 |
|--------|----------------|
| #262 Rollup Phase 1 | **向上路径 ✅**；本 change 补 **Execute 向下/同层** |
| ContextGraph F1–F4 | Link/Bubble 规则 → Materialize InboundSignals |
| context-budget Phase A–C | Compress/fold 复用；SubTurn 统一 Phase 3 |
| `RunParallelExplore` stub | PeerStatus Phase 2；依赖 Wave |

---

## 5. 请 Claude 讨论的开放问题

### OQ-LC-1：ScopeContract 持久化

**选项：**

- A) `WorkItem.ScopeContract` 字段（查询方便，WorkTree v3 migration）  
- B) 仅 `Goal.LastRound` 扩展（无 schema 变更）  
- C) cohort meta only（子 WI 从 cohort 读）

**Cursor 默认：** A + LastRound 镜像（审计）

**请 Claude 评估：** 对 DecomposeProposer Phase 2、disk store 版本、CLI show 的影响。

---

### OQ-LC-2：Materialize 实现深度

**选项：**

- A) 轻量 path：BasePrompt + signal inject + private jsonl + TruncateToTokens  
- B) 全量 `PrepareOrchestrator` A01–A04 每 WI 跑一遍

**Cursor 默认：** A（性能 + 语义清晰）；B 仅 L0 Goal 可选

**请 Claude 评估：** 是否与 D2 边界冲突；i18n/tool catalog 如何复用。

---

### OQ-LC-3：CG2 修订版本策略

**提案：** `workitem-context-graph-design.md` bump **0.3.0 → 0.4.0**，CG2 拆为 Transcript 隔离 + Cohort 域共享。

**请 Claude 评估：** 是否需要 ADR；F3 已 merge 的测试语义是否需 MODIFIED。

---

### OQ-LC-4：ChildDownlink vs DecomposeProposer timeline

rollup Phase 2 计划 LLM DecomposeProposer。ChildDownlink 是否在 Phase 1 用 **规则模板** 先行，Phase 2 再 LLM 填充 ExpectedReturn？

**Cursor 默认：** Phase 1 规则 + Goal ScopeContract 下行；Phase 2 合并 proposer 输出。

---

### OQ-LC-5：PeerStatus 默认策略

并行 explore 时 PeerStatus 是否 **opt-in**（spawn flag）还是 cohort 默认 inject terminal status？

**Cursor 默认：** opt-in policy flag，避免噪声。

---

### OQ-LC-6：Execute 每轮 Obs  taxonomy（已决议，请确认）

**议题：** 是否允许 Execute 每轮 LLM 自报 Obs*？

**Cursor 默认：** **禁止**（design §6.2）。替代：Signal + Observe 规则映射 + 可选软引导块。

**请 Claude 确认或提出反例场景。**

---

### OQ-LC-7：Phase 2 LLM ObservationProposer

是否在 Observe 节点增加 LLM 提案 Obs*（输入不含 wi 全文）+ 规则校验？

**Cursor 默认：** Phase 2 独立登记（T35），不与 Layer SubContext Phase 1 合并 PR。

---

## 6. 建议 Review 顺序

1. 读 `demand.md` §3 目标 + §7 Q&A  
2. 读 `proposal.md` §3 模型 + §9 Review 讨论点  
3. 读 `design.md` §2–§6 分层、信号、**Observe 边界** + §10 风险  
4. 扫 `specs/*/spec_delta.md` 验收场景  
5. 回应 §5 开放问题 → 更新 design/tasks

---

## 7. 文档索引

| 文件 | 用途 |
|------|------|
| `demand.md` | 原始诉求、LC1–LC5、验收标准 |
| `proposal.md` | Problem / Solution / Capabilities / Phase |
| `design.md` | 架构、时序图、D2 API、**§6 Observe 边界**、CG2′ |
| `tasks.md` | 分 Phase 任务清单 |
| `specs/d7-orchestration/spec_delta.md` | D7 规范增量 |
| `specs/d2-context-engine/spec_delta.md` | D2 规范增量 |

---

## 8. 修订记录

| Date | Note |
|------|------|
| 2026-06-27 | 初稿，供 Claude 讨论 |
| 2026-06-27 | §3 MUPS Observe 四类 × Execute 边界；OQ-LC-6/7 |
