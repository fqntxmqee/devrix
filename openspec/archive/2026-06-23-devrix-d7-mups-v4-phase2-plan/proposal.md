# Proposal: D7 MUPS v4.3 Phase 2 PR-B1 — Plan 节点数据契约 + Planner

**Change ID:** `devrix-d7-mups-v4-phase2-plan`
**Demand ID:** DM-20260623-001-PRB1
**Status:** S7_Archived
**Priority:** P0
**Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 2 Plan 节点落地梳理

> **S7 归档时间线**：
> - **S1 需求**（2026-06-23）— DM-20260623-001-PRB1 立项；需求：`demand.md`
> - **S2 提案**（2026-06-23）— 本 proposal.md + `tasks.md`；1 PR + 3 T 点 + 2.5 天工作量
> - **S3 设计**（2026-06-23）— `design.md` §0-§6；PlanKind 4 类 + Plan 不可变 With* + MatchKind 4 规则
> - **S3-Gate A-**（2026-06-23）— design.md 5/5 维度 PASS（inherited from Phase 2 observe-plan design）
> - **S4 实现**（2026-06-23）— PR #167 (PR-B1)；6 files +1458/-0；plan package 完整实现 + 30 tests
> - **S4-Gate A-**（2026-06-23）— go vet 0 issue + go test -race 0 race + 30/30 tests PASS
> - **S5 验收**（2026-06-23）— `acceptance-report.md`；3/3 P0 AC + 5/5 设计同步 + 30/30 tests PASS；✅ ACCEPTED
> - **S6 归档**（2026-06-23）— 本 archive；spec.md v4.2.0→v4.3.0 + t-registry v3.10.0→v3.11.0 + D7-S8-A22 IMPLEMENTED + demand-archive-index.md DM-20260623-001-PRB1 行 + verify-archive.sh 12/12 PASS

---

## 1. Background

`devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001) 已闭环 PR-A1 + PR-RF（落地 PR #163 + #166，6 P0 T 点 D7-S8-A15-T01..T06 IMPLEMENTED）。本 PR 是 Phase 2 第二步（PR-B1），紧随 Observe 节点落地之后，把 Plan 节点数据契约（doc 43）+ Planner interface + MatchKind 4 规则分类器同步为 OpenSpec 三件套 + 1 个 PR。

### 1.1 Phase 2 PR-A1 已落地的契约基础

| Phase 2 PR-A1 资产 | PR-B1 用法 |
|--------------------|-----------|
| `UncertaintyReport` (orchtypes) 含 `QuantizedIntent` (Kind string) | PR-B1 的 `MatchKind(quantizedKind string, ...)` 直接消费 |
| `UncertaintyReport.ComputeOverallStrength()` | PR-B1 的 `strengthFloor` 公式消费 overall strength 上限 |
| `Observation` 4 类不可变 (Kind + Strength) | PR-B1 的 `Plan.SourceObservationIDs` 引用 Observation ID |
| `UncertaintyCoord` 5 维 (Strength/Confidence/Coverage/TimeDecay/Source) | PR-B1 的 `Plan.Strength` 与 `UncertaintyReport.Overall.Strength` 对齐 |

### 1.2 Phase 3 PR-C2 直接前置依赖

PR-C2 ChannelRouter 需要：
- `PlanKind` 枚举（4 个）→ 1:1 路由到 4 Channel
- `Plan.SourceObservationIDs` → 4 Channel 都能引用（虽然 Channel 不直接消费，但 Artifact 端会消费）
- `Plan.BlastRadius.PersistScope` → ExplorationChannel 派生 `SideEffectStatus`

如果 PR-B1 不落地，PR-C2 必须自己实现 PlanKind enum 与 1:1 映射，会导致 PR-B1 和 PR-C2 互相重复且无法跨 PR 共用。

## 2. PR-B1 范围

### 2.1 数据契约层

1. **PlanKind 枚举** (4 类)
   - `CommitmentPlan` (uint8=1) — 1-Step 直接工具调用，强副作用（DB write / HTTP POST / file create）
   - `ProtocolPlan` (uint8=2) — 顺序多步，失败 reverse-order rollback
   - `ScenarioPlan` (uint8=3) — 并行探测 + 多数派投票，read-only
   - `ExplorationPlan` (uint8=4) — 多 agent 并行 + 优先级排序

2. **wire format**: snake_case（`commitment_plan` / `protocol_plan` / `scenario_plan` / `exploration_plan`），与 `shared/types.ArtifactKind` 协同（D5 dashboard 字符串过滤）

3. **BlastRadius** (PP-3 爆炸半径)
   - `FileCount` (int) — 写文件数
   - `APICallCount` (int) — 外部 API 调用数
   - `TokenCost` (int) — LLM token 消耗上限
   - `PersistScope` (3 态) — `PersistTransient` / `PersistSession` / `PersistPermanent`

4. **FailureCriterion** (PP-2 可证伪性)
   - `Field` (string) — 验证的字段名
   - `Op` (string) — 操作符白名单：`eq` / `ne` / `gt` / `lt` / `contains`
   - `Value` (any) — 期望值

5. **Step** — 最小可执行单元
   - `ID` / `Directive` / `ToolName` / `ToolArgs` (map[string]any) / `IdempotencyKey` (强制) / `EstimatedTokens`

6. **Plan struct** — 不可变
   - `ID` / `SessionID` / `Kind` / `Strength` (float64 ∈ [0,1]) / `Steps` (1+ Step) / `FailureCriteria` (1+ Criterion) / `BlastRadius` / `SourceObservationIDs` (1+) / `AnomaliesCount`

### 2.2 Planner 层

1. **Planner interface** — `Plan(ctx, PlanInput) (*Plan, error)`
2. **DefaultPlanner struct** — 规则引擎实现
3. **PlanInput struct** — 输入参数：UncertaintyReport + SessionID + Step templates
4. **MatchKind function** — 4 规则分类器
   - Rule 1: `intent_orchestrate` OR `anomaliesCount≥3` → ExplorationPlan
   - Rule 2: `stepCount==1` → CommitmentPlan
   - Rule 3: `intent_command` OR `stepCount≤3` → ProtocolPlan
   - Rule 4: default → ScenarioPlan
5. **strengthFloor formula** — `0.7 - 0.1·anomalies + min(observations·0.02, 0.2)`
6. **ReverseLookupObservations** — Phase 4 Verify 反向追溯入口

### 2.3 错误层

9 SentinelError + 3 helpers：
- `ErrPlanKindUnset` (PLAN_KIND_8001) — Kind 必须非 0
- `ErrPlanSourceObservationIDsRequired` (PLAN_LINEAGE_8002) — 血缘必填
- `ErrPlanBlastRadiusExceeded` (PLAN_BLAST_8003) — 爆炸半径超阈值
- 6 个支持 error（PP-1 强度越界 / PP-2 操作符非法 / PP-2 字段不可观察 / PP-3 阈值配置等）

## 3. 验收标准

### 3.1 P0 AC

| ID | 验收标准 | 状态 |
|----|---------|------|
| AC1 | PlanKind 4 类枚举 + wire format 协同 | ✅ PASS |
| AC2 | Plan.SourceObservationIDs 必填 + 防御性拷贝 + ReverseLookup | ✅ PASS |
| AC3 | MatchKind 4 规则 + uncertainty-first tie-break + DefaultPlanner 集成 | ✅ PASS |

### 3.2 测试与质量

- 30 个测试 100% PASS（0 race detector warnings）
- 覆盖率 93.5% ≥ 80% gate
- 5 P0 T 点全部 IMPLEMENTED（D7-S8-A22-T01..T03 + 22 supplementary）
- go vet ./... 0 issue

## 4. PR 拆分

| PR | 范围 | 文件数 | 估算 LOC | 风险 | 分支 |
|----|------|--------|---------|------|------|
| PR-B1 | Plan 4 类 + Planner interface + MatchKind 4 Rules | 6 | +1458 / -0 | Low | feat/devrix-d7-mups-v4-phase2-pr-b1 |

PR URL: https://github.com/fqntxmqee/devrix/pull/167

## 5. 不在本次任务范围

- PR-A2 IntentQuantizer: 待独立 change
- PR-A3 AnomalyDetector: 待独立 change
- PR-A4 ObserveNode wiring: 待独立 change
- PR-B2 Plan.Validate 细化（Field 可观察性扩展）: 待独立 change
- PR-B3 LLMPlanner: 待独立 change
- Phase 3 Execute 4 Channel (PR-C2): 并行 PR，依赖本 PR 落地
- Phase 4 Verify ReverseLookup consumer: 待 Phase 4 独立 change
- Phase 5 Learn ReputationEvidence consumer: 待 Phase 5 独立 change

## 6. 关联

### 6.1 前置依赖

- `devrix-d7-mups-v4-phase1-foundation` (Phase 1 OpenSpec: UncertaintyCoord precedent)
- `devrix-d7-mups-v4-phase2-observe-plan` (DM-20260623-001) — 直接前置

### 6.2 后续依赖

- `devrix-d7-mups-v4-phase3-execute` (DM-20260625-001) PR-C2 — 强依赖本 PR 落地

### 6.3 参考

- doc 35 §三.2 (Plan 节点方法论)
- doc 37 §2.2 (Plan 节点数据模型)
- doc 43 (D7 Plan 节点详细技术方案)
- doc 47 (Phase 2 落地方案)
- `openspec/specs/d7-orchestration/spec.md` (D7 域规范)
- `openspec/specs/d7-orchestration/t-registry.md` (D7 T 层注册表)
