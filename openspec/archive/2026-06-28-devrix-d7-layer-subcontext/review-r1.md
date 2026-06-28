# Review R1 — Layer SubContext 架构终审

**Change ID:** `devrix-d7-layer-subcontext`  
**Demand ID:** DM-20260627-003  
**Review Date:** 2026-06-28  
**Reviewers:** Cursor（架构讨论）+ Claude MiniMax-M3（`game-theory-review.md`）  
**Status:** **S4_Development — R1 决议已冻结**

**Related SoT:**

- `demand.md` LC1–LC6  
- `design.md` v0.3.0（含 §6 Observe 边界）  
- `game-theory-review.md`  
- `#262` Rollup Phase 1（向上 bubble 路径 ✅）

---

## 1. 终审结论

| 维度 | 评分 | 结论 |
|------|------|------|
| **机制设计** | 8.5/10 | Context ≠ Signal ≠ Observation；CG2′ 分层产权方向正确 |
| **博弈论自洽** | 8.0/10 | §6.2 反 Moral Hazard 为关键决议；需补 scope 漂移 / cohort 池 / flag 锁定 |
| **与主线 G1–G5** | 9.0/10 | 补 Execute 向下/同层 Materialize 闭环；与 Rollup 向上路径正交 |
| **验收可测性** | 9.0/10 | LC1–LC6 + T21–T26 可定位 |
| **落地风险** | 7.5/10 | flag 默认 off + 30 天迁移 deadline |

**总评：** **进入 S4 Phase 1 编码**。本 change **不**宣称 WorkTree v2 完成；Phase 2/3 里程保持登记。

---

## 2. 已冻结共识（North Star + 机制）

### 2.1 三层边界（不可妥协）

```text
Execute context  →  Signal（ScopeContract, LastRound, Bubble, ChildDownlink）
Observe 规则     →  Observation（Obs* + Strength + Evidence）→ UncertaintyReport
Plan 规则        →  PlanKind（MatchKind）
```

- **禁止** Execute 每轮 ReAct 自报 ObsFact/ObsSignal/ObsDeviation/ObsUncertainty（LC6 / G3）。
- **允许** Execute 软引导 `<conclusion>` / `<open_questions>`（非 SoT）。

### 2.2 Partition 模型（CG2′）

| Partition | Key | 写入 |
|-----------|-----|------|
| SessionContext | `session:<sid>` | L0 Goal / 主 Turn |
| LayerCohort | `cohort:<sid>:<parent>` | Signal 元数据 only |
| WorkItemPrivate | `wi:<sid>:<wi_id>` | ReAct append 默认 |
| AgentSubContext | `agent:<sid>:<agent_id>` | Phase 3 |

**同层共享** = ScopeContract cohort 域；**不**共享 WorkItemPrivate 全文。

### 2.3 D2 Materialize

- **轻量 path**（OQ-LC-2）：BasePrompt + InjectSignals + LoadPrivateChain + Compress。
- L0 Goal 可选全量 Prepare；depth≥1 + flag=on 走 Materialize。
- D7 只传 partition + policy + directive；禁止直接读 session 单桶组装 WI LLM 请求。

### 2.4 ScopeContract（Goal 范围收敛）

- 持久化：**`WorkItem.ScopeContract` 字段** + LastRound 镜像（OQ-LC-1）。
- `open_questions` 非空 → **阻断 SpawnDecompose**（规则门控 + R-OBS-1 ObsUncertainty）。
- 极具体指令 → 规则推断 ScopeIn，跳过额外 LLM scope 轮。

### 2.5 ChildDownlink

- Phase 1 **规则模板**下行；Phase 2 与 DecomposeProposer 合并（OQ-LC-4）。
- **`ExpectedReturn` 强制非空**；空值阻断 decompose（OQ-LC-4 博弈论补强）。

### 2.6 Observe / Signal

- R-OBS-1..7 规则表（design §6.5）；bubble → ObsFact 保持 #262 路径。
- LLM ObservationProposer → **独立 change**（OQ-LC-7 / Phase 2 / T35）。

### 2.7 PeerStatus（Phase 2）

- **opt-in** policy flag + **cohort_size ≥ 3** 才暴露（OQ-LC-5）。
- terminal-only；summary ≤ 240 chars；禁止 live tail。

### 2.8 Feature Flag

- `FeatureLayerSubContextEnabled` default **false**。
- **`flag_migration_deadline`：Phase 1 验收后 30 天**，depth≥2 WorkTree 强制 flag=on（OQ-LC-10）。

### 2.9 Phase 2 登记（本 PR 不编码）

- OQ-LC-8：`scope_expansion_max_ratio` 1.5x  
- OQ-LC-9：`cohort_signal_budget_max` 8KB（T20d Phase 1 可先做常量 + 单测）  
- Upstream Materialize、PeerStatus 实装、CLI show  
- CG2 design doc **0.4.0 + ADR-001**

---

## 3. 开放问题决议表（OQ-LC-1..10）

| OQ | 决议 | Phase |
|----|------|-------|
| LC-1 ScopeContract 持久化 | WorkItem 字段 + LastRound 镜像 | 1 |
| LC-2 Materialize 深度 | 轻量 path；L0 可选全 Prepare | 1 |
| LC-3 CG2 版本 | 0.4.0 + ADR-001 | 1 docs |
| LC-4 ChildDownlink | 规则模板；ExpectedReturn 非空 | 1 |
| LC-5 PeerStatus | opt-in + cohort≥3 | 2 |
| LC-6 Execute Obs 标签 | **禁止** + open_questions 软引导 | 1 |
| LC-7 ObservationProposer | 独立 change | 2 |
| LC-8 Scope 漂移 | scope_expansion_max_ratio 1.5x | 2 |
| LC-9 cohort 池预算 | cohort_signal_budget_max 8KB | 1 常量 / 2 完整 |
| LC-10 flag 锁定 | 30 天 migration deadline | 1 policy |

---

## 4. S4 三条原则（编码约束）

1. **flag=off 零回归**：WorkItemExecutor 行为与当前 master Prepare 路径一致。  
2. **Signal 不进 Obs taxonomy 原文**：Obs* 仅 Observe 规则产出。  
3. **Materialize 可观测**：Jaeger `D2_Context_Materialize` 含 wi_id / policy / message_count。

---

## 5. 修订记录

| Date | Note |
|------|------|
| 2026-06-28 | R1 冻结：Cursor + Claude game-theory review 共识合入 |
