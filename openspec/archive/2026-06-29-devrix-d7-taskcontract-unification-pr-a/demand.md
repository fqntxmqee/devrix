---
demand-id: DM-20260629-007
title: D7 TaskContract 统一 PR-A — interfaces 包 + TaskSpec/TaskReport 双契约接口层 + 字段语义层
priority: P0
status: S1_Demand
dsaft_domain: orchestration
created: 2026-06-29
reporter: 2026-06-29 启动 v7.0 演进第一枪；DM-20260629-006 DESIGN 落地 PR-A scope
parent_design: openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/（DESIGN ONLY 归档）
related:
  - devrix-d7-dsaft-restructuring（DM-20260629-001）v6.0.x 维护收官
  - devrix-d7-six-s-simplification（DM-20260626-001）14 S → 6 S 精简
  - devrix-d7-mups-v5-escape-engine（DM-20260625-003）v5 EscapeEngine
  - devrix-d7-certainty-architecture UncertaintyCoord + VERDICT 4 态
  - devrix-d7-layer-subcontext（DM-20260627-003）D7-S16 占用 Layer SubContext
---

# D7 TaskContract 统一 PR-A

## 1. 背景

2026-06-29 `devrix-d7-taskcontract-unification` (DM-20260629-006) 完成 DESIGN ONLY S6 归档（4-Layer × 3-Phase × 23 AC 设计就位）。本 Change 启动**第一阶段实施 PR-A**：L1 接口层 + L2 字段语义层 + L4 spec 同步（AC17），共 6 AC，~1 周工作量。

### 1.1 父设计引用

完整设计见 `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/design.md`（648 行，六段式 ①-⑥）。本 Change **不复用父设计**，而是按 PR-A scope 重新写紧凑版 design.md（避免文档冗余）。

### 1.2 PR-A 在 4-Layer × 3-Phase 矩阵中的位置

| Layer ↓ \ Phase → | **PR-A**（本 Change） | PR-B | PR-C |
|---|---|---|---|
| **L1 接口层** | **AC1, AC2** ← | — | — |
| **L2 字段语义层** | **AC3, AC4, AC5** ← | — | — |
| **L3 防御运行时层** | — | AC11, AC15 | AC13, AC12, AC14 |
| **L4 治理横切层** | **AC17** ← | AC9, AC10, AC16, AC21, AC22, AC23 | AC6, AC7, AC8, AC18, AC19, AC20 |

## 2. PR-A 6 AC 范围

| ID | 标准 | Layer | T 点 | 优先级 |
|----|------|-------|------|--------|
| **AC1** | `interfaces.TaskSpec` struct + 4 字段 + 2 元字段 + builder + 3 处创建点（Plan / Channel / WorkItem）统一 | L1 | D7-S20-A01-T01..T03 | P0 |
| **AC2** | `interfaces.TaskReport` struct + 5 字段 + 2 元字段 + builder + Channel.Execute 出口 + Learn 节点入口统一 | L1 | D7-S20-A02-T01..T03 | P0 |
| **AC3** | `Dissent` 字段 + ExplorationChannel 全量结果 → top-3 保留 + Learn 节点沉淀至 SkillMemory.SOP | L2 | D7-S21-A01-T01 | P0 |
| **AC4** | `Blockage` 字段 + Verifier 拒绝原因 + LLM 二次分析 → 结构化 missing/required_external/infeasible | L2 | D7-S21-A02-T01 | P0 |
| **AC5** | `Resource` 字段 + ContextBudget Phase B 现有 metric 抽取 per-Plan token/time/step | L2 | D7-S21-A03-T01 | P0 |
| **AC17** | Spec 文档同步：`spec.md` v7.0 spec + `d7-domain.md` §8 Layer 章节 + `a/f/t/span-registry.md` 增量登记 | L4 | D7-S20-A03-T01..T02 | P0 |

**PR-A T 点总计**：7 个（AC1 × 3 + AC2 × 3 + AC17 × 2 + AC3/4/5 × 1 = 11 项；合并 AC3/4/5 单 T 后 = 7 项）

### 2.1 T 编号策略（关键变更）

⚠️ **T 编号重映射**：父设计文档 §2.2 写 `D7-S16/17/18/19`，但 `D7-S16` 已被 `devrix-d7-layer-subcontext` (DM-20260627-003) Layer SubContext 占用（18 T 点全 IMPLEMENTED）。

→ **本 Change 采用 `D7-S20~S23` 重映射**（v7.0 sprint 专属编号段）：
- `D7-S20` = L1 接口层 (TaskSpec + TaskReport)
- `D7-S21` = L2 字段语义层 (Dissent + Blockage + Resource)
- `D7-S22` = L3 防御运行时层 (PR-B + PR-C 实施)
- `D7-S23` = L4 治理横切层 (PR-A AC17 + PR-B + PR-C 收口)

### 2.2 PR-A 不做事项（Out of Scope）

- ❌ L3 防御运行时（AC11 Pessimistic + AC13 CoW + AC12 Rule-based + AC14 Similarity + AC15 Hard Evidence）→ PR-B/PR-C
- ❌ Feature Flag 灰度（AC22）→ PR-B
- ❌ Error Code ORCH_*（AC23）→ PR-B（PR-A 仅在 interfaces/errors.go 预留 5 个基础 error）
- ❌ Migration Plan（AC16）→ PR-B
- ❌ Cross-Domain Boundary test（AC21）→ PR-B
- ❌ Convergence span（AC6）→ PR-C
- ❌ AdaptiveThreshold wiring（AC7）→ PR-C
- ❌ Layout guard（AC8）→ PR-C
- ❌ Coverage ≥ 80%（AC18）→ PR-C

## 3. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `ChildDownlink`（已存在）作为 TaskSpec.TraceID 参考 |
| 依赖 | `UncertaintyCoord` + `Verdict`（已存在）作为 TaskReport.Result 字段类型 |
| 依赖 | `Context Budget Phase B`（已存在）作为 Resource 字段抽取基础 |
| 依赖 | MUPS Learn 5 节点管道（已存在）作为 Dissent 沉淀目标 |
| 依赖 | `workmodel/workitem.go` ChildDownlink 7 字段（已存在）作为 TaskSpec.Goal 引用 |
| 约束 | 不破坏现有 v6.0.x API：ChannelRequest / LearnRequest 通过 type alias 保留（PR-B 完整实施；PR-A 仅迁移创建路径，调用方仍可编译） |
| 约束 | P0 复用 v6.0.x 已就位机制，不引入新外部依赖 |
| 约束 | `interfaces` 包 0 import D7 任何子包（Pure types 防 cycle，PR-A Layout guard 守护，PR-C 完整实施） |
| 约束 | 22/22 orchestration packages `go test -race -count=1` 不退化 |

## 4. 风险评估（PR-A 限定）

| 风险 | 影响 | 缓解 |
|------|------|------|
| TaskSpec 引入后 ChannelRequest 调用方断裂 | Channel.Execute 编译失败 | PR-A 保留 ChannelRequest struct 字段，TaskSpec 嵌入为新字段（additive，不破坏）；PR-B 完整迁移 + type alias |
| TaskReport 引入后 Learn 节点接口变更 | mups/learn/learner.go 编译失败 | 同上：LearnRequest 保留，TaskReport 嵌入为新字段 |
| `interfaces` 包 import cycle | 编译失败 | PR-A 仅放 type + builder，零行为；不 import 任何 D7 子包 |
| Dissent 字段数据量大，SkillMemory 写入变慢 | Learn 节点延迟 | Dissent 只保留 top-3（默认）+ summary 哈希引用 |
| Resource 字段需重新埋点 | 增加 instrumentation 改动 | 复用 Context Budget Phase B 现有 metric，仅做字段抽取 |
| Spec 文档漂移（代码改了 spec 没改）| reviewer 误判 | PR-A 提交前置条件：spec.md / d7-domain.md / a/f/t/span-registry.md 5 个 spec 文件同步 |

## 5. 验收门槛（PR-A S5 verify）

- [ ] 22/22 orchestration packages `go test -race -count=1` PASS
- [ ] LP-1（Bayesian reputation）/ LP-2（5 节点 pipeline）/ LP-5（cross-session traceability）100% 兼容
- [ ] `interfaces` 包新增测试覆盖率 ≥ 80%
- [ ] TaskSpec / TaskReport 构造 P99 < 1ms（benchmark）
- [ ] `interfaces` 包 0 import D7 任何子包（layout guard 检查）
- [ ] ORCH_* SentinelError 5 个基础 error 定义（PR-A 范围）
- [ ] 5+1 个 spec 文档同步完成（AC17）

## 6. 关联变更

| Change ID | DM ID | 关系 | 说明 |
|-----------|-------|------|------|
| devrix-d7-taskcontract-unification | DM-20260629-006 | 父 DESIGN | archive 归档的 648 行设计 |
| devrix-d7-dsaft-restructuring | DM-20260629-001 | 前置 | v6.0.x 收官，Span Evidence 94% |
| devrix-d7-six-s-simplification | DM-20260626-001 | 前置 | 14 S → 6 S，本 Change 落在 L1/L2 4 新 S |
| devrix-d7-mups-v5-escape-engine | DM-20260625-003 | 前置 | 5 层 CB，PR-B 接入 TaskReport.Blockage |
| devrix-d7-layer-subcontext | DM-20260627-003 | **T 编号冲突** | D7-S16 已被占，本 Change 重映射 D7-S20~S23 |
| devrix-d7-multiturn-session-state | DM-20260628-004 | 并行 | S3-Gate 互审 |

## 7. 备注

PR-A scope 紧凑（6 AC + 7 T 点 + ~800 行新增 + ~50 行修改），按 design.md §7.2 风险等级"低"，可独立合入。S3-Gate review 通过后即进入 S4 实施。