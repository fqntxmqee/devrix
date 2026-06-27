# Review R1 — TurnLoop × WorkTree × MUPS 主线终审

**Change ID:** `devrix-d7-workitem-rollup-pipeline`  
**Demand ID:** DM-20260627-001  
**Review Date:** 2026-06-27  
**Reviewer:** 架构终审（trace `58e6c55d…` 排查 + OpenSpec S3 设计包）  
**Status:** S4_Development — R1 决议已冻结（2026-06-27）  
**Scope:** TurnLoop + WorkTree + MUPS 逻辑一致性、完整性、确定性；对齐主线「**将不确定性分层递归探索为确定性过程**」

**Related SoT:**

- `openspec/specs/d7-orchestration/workitem-pipeline-unification-design.md` (G1–G5)  
- `design.md` §4.2 层级闭环模型  
- `proposal.md` Phase 1/2 边界  

---

## 1. 终审结论（Executive Summary）

| 维度 | 评分 | 结论 |
|------|------|------|
| **与主线对齐（设计 SoT）** | 8.5/10 | TurnLoop（G5 时钟）× WorkTree（G4 SoT）× MUPS（G2 信号机）三角分工清晰，§4.2 层级闭环与 north star **一致** |
| **与主线对齐（可运行系统）** | 4/10 → **6.5/10**（Rollup change 落地后） | 骨架在；Rollup change 是 **回归主线的关键补洞**；Path B 为 compat，非终态 |
| **逻辑一致性** | 有条件通过 | T1–T7 张力需在 spec/I3-Rollup 中显式化（见 §3） |
| **完整性** | 设计完整 / 实现 ~55% | Rollup + Reopen + Focus 顺序闭合后 ~70%；Decompose/Verify/Parallel 为剩余里程 |
| **确定性（控制面）** | 设计达标 | SpawnPolicy、状态机、LP-5、Escape 可单测；实现被工具 bypass 削弱 |
| **确定性（内容面）** | 故意非确定 | LLM Execute/Rollup；需 FailureCriteria + verify 口径统一（§5） |

**总评：** 架构 **在最终主线上**；本 change **应合入**，但不得宣称「WorkTree v2 完成」。合入后须更新 unification-design Phase D 状态，并登记 Phase 2/3 剩余里程。

---

## 2. 主线形式化与对齐

### 2.1 North Star

```text
不确定性（高 U、open-ended、多 hypothesis）
  → 分层（WorkTree L0→L1→L2…）
  → 递归（Decide 开子层 / 子 terminal 后父 Rollup）
  → 探索（Execute：ReAct / Wave / 多 Step）
  → 确定性过程（SpawnPolicy、Verdict、LP-5、状态机、Escape）
  → 收敛（Pass / Escalate / ForceExit）+ deliverable
```

### 2.2 三件套正交分工

| 组件 | 角色（G#） | 不做什么 |
|------|-----------|----------|
| **RunSessionTurnLoop** | Session 时钟（G5）：选 focus、迭代、直到工作集清空 | 不承载业务 LLM 逻辑 |
| **WorkTree** | 工作语义 SoT（G4）：层级、Status、LastRound、依赖 | 不直接调 LLM |
| **MUPS / ItemPipeline** | 每层信号机（G2）：Observe→…→Decide，产出 round | 不管理 session 全局调度 |

### 2.3 主线环节矩阵

| 环节 | SoT | 现状 | 本 change 后 |
|------|-----|------|--------------|
| TurnLoop ingress | G5 | ✅ | ✅ + Fallback 顺序（T01c） |
| 每层 MUPS + LastRound | G2 | ✅ 单轮 | ⚠️ Rollup 第二轮 |
| 向下规则 Decide | G3 R0–R8 | ✅ | ✅ |
| 向下 LLM Decompose | DecomposeProposer | ❌ bypass | Phase 2 |
| 向上 Rollup + bubble | §4.2.3 | ❌ | ✅ Path A/B |
| 并行 Explore | G1 Wave | ❌ stub | Phase 2 |
| Verify ↔ FailureCriteria | Plan 模板 | ❌ ExitCode | T08b 待定义 |
| Session deliverable | G5 complete | ❌ 空 | ✅ |

---

## 3. 逻辑张力登记（T1–T7）

| ID | 张力 | 影响 | 决议 / 动作 |
|----|------|------|-------------|
| **T1** | I3 terminal+Locked vs Rollup 同 WI R2 | Path B 无法同 wi 跑第二遍 MUPS | **增 I3-Rollup 例外** + `ReopenForRollup`（tasks T01b）；spec_delta MODIFIED |
| **T2** | I6 每 iter 一 pipeline vs Rollup RoundNo++ | 可接受 | Jaeger 增加 `round_no`；文档明确第二次 iter |
| **T3** | D4 ChannelRouter vs 现状 WorkItemExecutor | G1 并行探索不成立 | 标 **Phase D 漂移**；Phase 2 接 Wave |
| **T4** | D3 ParallelExplore ephemeral vs Decompose 持久子 | Rollup 输入弱 | Phase 2 定义 ephemeral→父 LastRound materialize |
| **T5** | Path B vs G2/G3（todo_write bypass） | 结构审计链断 | **ADR：compat 路径**；验收保留 Path A fixture |
| **T6** | Session processAutoClose Learn vs Item Learn | Reputation 以哪轮为准 | Rollup Pass 应覆盖 R1 Fail；tasks 补 Learn 口径 |
| **T7** | max_iters→Fail→R8 SpawnNone | open-ended 永不 R5 Decompose | **tech-debt**；可选 Partial 路由或 Path C |

**S4 阻塞项：** T1、TurnLoop Fallback（T01c）、Verify 口径（T08b）。

---

## 4. 完整性（§4.2 Layer Closure）

| 职责 | SoT | 实现 | change 后 |
|------|-----|------|-----------|
| 向下：规则 Decide | ✅ | ✅ | ✅ |
| 向下：LLM Decompose | ✅ | ❌ | ❌ P2 |
| 向下：并行 Explore | ✅ | ❌ | ❌ P2 |
| 本层 Execute | ✅ | ⚠️ ReAct only | ⚠️ |
| 本层 Verify | ✅ FC | ❌ ExitCode | ⚠️ T08b |
| 本层 Rollup 验收 | ✅ §4.2.3 | ❌ | ✅ |
| 向上 bubble + complete | ✅ | ⚠️ | ✅ |
| Escape / depth | ✅ | ✅ | ✅ |

**剩余里程（主线顺序）：**  
① Rollup+Reopen（本 change P0）→ ② DecomposeProposer+工具门禁（P1）→ ③ Verify↔FC（P1）→ ④ Wave（P1）→ ⑤ free_fork 投影（单独 change）→ ⑥ D1 去重（交付体验）

---

## 5. 确定性分层

### 5.1 控制面（应确定 — 设计正确）

- `SpawnPolicyEvaluator`：纯函数 R0–R8  
- `GetFocus` 排序：kind → uncertainty → created_at → id  
- LP-5 血缘、Escape/depth/daily limit  
- Bubble 格式与 CB 截断  

### 5.2 内容面（故意非确定）

- Execute / Rollup synthesize / DecomposeProposer（LLM）  
- unification-design explicit non-goal：不保证 LLM 结论正确  

### 5.3 应确定但未确定（风险）

| 问题 | 后果 |
|------|------|
| Verify 仅 ExitCode | Pass/Fail 与任务语义脱钩 |
| 工具 bypass 拆树 | 同输入多路径 |
| Path B 无子 Execute 产物 | Rollup 仅标题无证据 |
| ephemeral 不持久化 | 崩溃不可复现 |

### 5.4 Phase 1 验收口径（R1 建议冻结）

| 层级 | 内容 | 方法 |
|------|------|------|
| **结构确定** | 2× MUPS 同 root wi、无 checklist MUPS、bubble、complete 非空 | 自动化 IT |
| **内容确定** | P0/P1 章节 | **二选一**：rollup heuristic verify **或** stub LLM fixture（写入 spec_delta） |

---

## 6. 与 trace 案例交叉验证

| 现象 | SoT 覆盖 | 本 change |
|------|----------|-----------|
| 父不 rollup | §4.2.3 | ✅ |
| 11× checklist MUPS | §5.9 | ✅ |
| complete 空 | G5 | ✅ |
| free_fork 未汇总 | OOS | ❌ |
| 口头 parallel 无 Wave | G1 P2 | ❌ |
| locked root 无法 R2 | §4.2.8 / T1 | T01b |
| focus nil 即 break | §4.2.8 | T01c |

---

## 7. S4 前三条冻结原则

1. **结构确定优先于内容完美** — 先 Rollup + Reopen + Focus + complete；Phase 2 再治理 bypass。  
2. **Path B 不写入 G2/G3 不变式** — compat ADR；验收必须含 Path A fixture。  
3. **Verify 口径一次说清** — 消除 demand P0/P1 与 proposal Phase 3 矛盾。

---

## 8. 决议项（2026-06-27 用户确认 — 按 R1 推荐）

| ID | 问题 | **决议** |
|----|------|----------|
| OQ-1 | `NeedsRollup` | ✅ 显式 `bool` + LastRound 审计 |
| OQ-2 | Rollup PlanKind | ✅ 复用 `CommitmentPlan` + rollup FC 模板 |
| OQ-3 | Phase 1 禁 free_fork？ | ✅ 不禁止；标非 SoT |
| OQ-4 | checklist | ✅ Virtual Bubble（此前已决议） |
| R1-V1 | Rollup verify | ✅ **C**：IT stub LLM + 生产 heuristic（len≥500、P0/P1、planning 黑名单） |
| R1-V2 | Learn Verdict | ✅ Rollup 终局 Verdict 写 Reputation |
| S4-PR | PR 范围表述 | ✅ 不得写「WorkTree v2 完成」 |
| S4-FF | Feature flag | ✅ `FeatureWorkItemRollupEnabled` 默认 true |
| S4-IT | 验收 fixture | ✅ Path A + Path B 双集成 fixture |

---

## 9. 文档变更清单（R1）

| 文件 | 变更 |
|------|------|
| `design.md` | §4.2 层级闭环；§13 终审摘要 |
| `review-r1.md` | 本文档 |
| `tasks.md` | T01b/T01c/T08b |
| `proposal.md` | Review Checklist 引用 R1 |
| `.openspec.yaml` | 登记 review-r1 |

**S4 后待更新（S5/S7）：** `spec_delta.md` MODIFIED I3-Rollup；`unification-design.md` Phase D 状态；`tech-debt` T7/Path C。

---

## 10. S4 进入检查清单

- [x] OQ-1～3 + R1-V1/V2 拍板（2026-06-27）  
- [x] 同意 review-r1 三条原则  
- [ ] T1：`ReopenForRollup` + 状态机 delta 写入 spec_delta  
- [ ] T01c：TurnLoop focus nil → Fallback → continue  
- [ ] R1-V1：rollup verify 写入 spec_delta + tasks T08b  
- [ ] Path A 集成 fixture 与 Path B trace fixture 均存在  

---

**维护：** S5 验收后更新 §2 评分；S7 归档时将 §8 决议合入 `acceptance-report.md`。
