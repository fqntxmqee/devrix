# Proposal: D7 MUPS v4.3 Phase 4 — Verify 节点升格 (Verify Promotion)

**Change ID:** `devrix-d7-mups-v4-phase4-verify-promotion`
**Demand ID:** DM-20260623-002
**Status:** S2_Proposal → S3_Design → S4_Implemented → S7_Archived
**Priority:** P0
**Created:** 2026-06-23
**Author:** MUPS v4.3 Phase 4 Verify 节点落地梳理

---

## 1. 背景

MUPS v4.3 5 节点管道（Observe → Plan → Execute → Verify → Learn）前三节点（Observe PR-A1, Plan PR-B1, Execute PR-C1/C2）已 S7_Archived。但 Verify 节点仍以分散子集形式存在（doc 17/18 L1+L2 verifier），缺：
- 节点级独立抽象（Verifier interface）
- 4 态 Verdict（Pass/Partial/Indeterminate/Fail）的 typed enum 化（目前散落在 UncertaintyCoord.FromVerifier 的 string switch）
- 多 Verifier 聚合（当前单 verifier 调用，无 AggregateVerdicts 入口）
- Verdict → ExitReason 的语义映射（当前 orchestrator.go 内联 8 ExitReason，缺 Verifier 维度）
- Evidence 提取（verifier LLM 输出 → 结构化 Evidence）
- SystemAnomaly 异常聚合（CatSystem 异常信号聚合为 SystemAnomaly 决策）

Verify 节点是 Phase 5 Learn 的直接前置（Learn 消费 Verdict + UncertaintyCoord），缺 Verdict 数据契约意味着 Phase 5 LP-1 闭环（Observe.Receive prior ← Learn.ReputationStore）无法落地。

## 2. 问题陈述（4 Problems）

### P1: 4 态 Verdict 仍以 string 散落

`orchtypes.UncertaintyCoord.FromVerifier` 内联 `case "pass"/"partial"/"indeterminate"/"fail"`，无 typed enum，无 String/Marshal/Unmarshal，跨域消费方无法 compile-time 检查。

### P2: 多 Verifier 聚合入口缺失

Phase 4 节点级设计（doc 45 §五）要求 Multi-aspect Verification（Compliance/Timeliness/RootCause/Statistical 四类子 verifier 各产出 Verdict），但当前仅单 verifier 调用路径，无聚合函数。

### P3: Verifier parse failure 误判为 FAIL

`doc 17 §4.3` 描述的 VerifyWithRetry 3 次兜底机制：parser 失败重试 3 次仍失败 → 应当 INDETERMINATE（高不确定性，需人工 review）而非 FAIL（条件不满足）。当前实现（workmodel/uncertainty.go）parser 失败直接 fail-fast，触发 `ErrUncertaintyReportInvalid` 拒绝 retry。

### P4: SystemAnomaly 异常聚合缺失

doc 35 §三.5 要求 CatSystem 类 Observation（ObsDeviation/CatSystem）聚合为 SystemAnomaly 决策（forced UncertaintyCoord.Value=0.95）。当前 UncertaintyCoord.FromVerifier 的 `systemAnomaly bool` 参数未在 ObserveNode wiring 中落地（CatSystem 异常被 CatBusiness Strength 主导的 strengthFloor 吞没）。

## 3. 解决方案（4 PR × 8 T 点）

### PR-D1: AggregateVerdicts (G3-1) — 数据契约入口

- **范围**：4 态 VerdictKind typed enum + 4 AggregationStrategy 枚举 + AggregateVerdicts 函数
- **T 点**：D7-S10-A32-T01 (VerdictKind enum) / T02 (AggregationStrategy + AggregateVerdicts)
- **依赖**：Phase 3 Artifact（PR-C1/C2）已就绪
- **风险**：Low — 纯函数 + 新 enum，无现有调用方

### PR-D2: VerdictToExitReason (G8-1 P0-3 修复) — 语义映射

- **范围**：VerdictToExitReason 函数（Verdict → ExitReason）+ 14 ExitReason 枚举扩展（8 → 14）+ VerifyWithRetry parse failure → INDETERMINATE 修复
- **T 点**：D7-S10-A33-T03 (VerdictToExitReason + 14 ExitReason) / T04 (VerifyWithRetry 修复)
- **依赖**：PR-D1（VerdictKind typed enum）
- **风险**：Medium — 修改 orchestrator.go 内联 ExitReason 计算逻辑，需保持现有 8 行为兼容

### PR-D3: EvidenceExtractor — 结构化 Evidence

- **范围**：Evidence struct + EvidenceExtractor interface（从 Verifier LLM 输出提取结构化 Evidence：判定依据/置信度/反例）
- **T 点**：D7-S10-A34-T05 (Evidence struct) / T06 (EvidenceExtractor interface)
- **依赖**：PR-D1（VerdictKind）
- **风险**：Low — 新结构 + 接口，无现有调用方

### PR-D4: SystemAnomaly 异常聚合 — 节点级 wiring

- **范围**：SystemAnomalyAggregator（聚合 CatSystem 异常 → SystemAnomaly bool）+ ObserveNode wiring（Anomalies 超过阈值 → SystemAnomaly=true）
- **T 点**：D7-S10-A35-T07 (SystemAnomalyAggregator) / T08 (ObserveNode wiring 集成)
- **依赖**：PR-D2（FromVerifier systemAnomaly 参数已 typed）+ Phase 2 Observation Partition（PR-A1）
- **风险**：Medium — 修改 ObserveNode wiring，可能影响 Plan.strengthFloor 计算

## 4. 验收标准（12 AC）

| ID | 描述 | 归属 PR | 验证 |
|----|------|---------|------|
| AC1 | VerdictKind typed enum 4 态 + String/Marshal/Unmarshal | PR-D1 | unit test |
| AC2 | AggregationStrategy 4 策略 + AggregateVerdicts 函数边界（空/1/N） | PR-D1 | unit test |
| AC3 | VerdictToExitReason 4 Verdict → 14 ExitReason 映射正确 | PR-D2 | unit test |
| AC4 | VerifyWithRetry 3 次 parse failure → INDETERMINATE（非 FAIL） | PR-D2 | integration test |
| AC5 | ExitReason 扩展 8 → 14 向后兼容（既有 8 个 enum 值字符串不变） | PR-D2 | unit test |
| AC6 | Evidence struct + 3 字段（Reason/Confidence/Counterexample） | PR-D3 | unit test |
| AC7 | EvidenceExtractor interface 2 方法（Extract + Validate） | PR-D3 | unit test |
| AC8 | SystemAnomalyAggregator 阈值（AnomaliesCount≥3 → true） | PR-D4 | unit test |
| AC9 | ObserveNode wiring SystemAnomaly 传 FromVerifier | PR-D4 | integration test |
| AC10 | 4 PR 联动 go vet + go test -race + layer-lint 全绿 | 全部 | CI |
| AC11 | 8 P0 T 点全部 IMPLEMENTED + 覆盖率 ≥ 80% | 全部 | coverage |
| AC12 | 跨域一致性（Phase 2 Observation/Phase 3 Artifact/Phase 4 Verdict 三方契约闭合） | 全部 | integration test |

## 5. 工作量估算

| PR | 文件数 | LOC | 测试 | 风险 |
|----|--------|------|------|------|
| PR-D1 | 3 + 1 test | +600/-0 | 12 | Low |
| PR-D2 | 4 + 1 test | +800/-50 | 14 | Medium |
| PR-D3 | 3 + 1 test | +500/-0 | 10 | Low |
| PR-D4 | 3 + 1 test | +700/-30 | 12 | Medium |
| **总计** | **13 + 4 test** | **+2600/-80** | **48** | **6 天** |

## 6. 不做的事

- ❌ 不引入新 LLM 模型（Verifier 子 agent 沿用 Phase 1 verifier prompt）
- ❌ 不重写 UncertaintyCoord（仅扩展，从 PR-A1 + Phase 2 PR-RF 继承）
- ❌ 不实现 Learn 节点（Phase 5 独立 change）
- ❌ 不实现 ObserveNode 重构（仅 wiring 增量，ObserveNode interface 不变）
- ❌ 不影响 Phase 3 PR-C1/C2 既有代码（ArtifactKind 4 类 + SideEffectStatus 5 态完全复用）

## 7. 关联

- **前置**：Phase 2 PR-A1 (Observation 4 类) + PR-B1 (Plan 4 类 + Planner) + Phase 3 PR-C1 (Artifact 数据契约) + PR-C2 (Channel + ChannelRouter)
- **后续**：Phase 5 Learn 节点（PR-E1..E5 强依赖本 PR 的 Verdict 数据契约 + Evidence 结构）
- **设计稿**：doc 45 (D7 Verify 节点详细技术方案) + doc 17 (L2 verifier) + doc 18 (L1 ExitReason) + doc 41 (Phase 1 OpenSpec Verifier 部分)

## 8. 风险与缓解

| 风险 | 等级 | 缓解 |
|------|------|------|
| VerdictKind enum 替换 string 导致下游破坏 | Medium | 类型别名 `type VerdictKind = types.VerdictKind`（shared/types precedent Phase 1 MemoryEntry + Phase 3 SideEffectStatus） |
| VerifyWithRetry 行为变更影响现有 verifier 调用方 | Medium | 新增 `ParseVerdictKindWithRetry` 函数 + 默认调用点逐步迁移 |
| SystemAnomaly wiring 修改 ObserveNode 计算路径 | Medium | 仅增量添加 systemAnomaly 字段，未修改既有 strengthFloor 公式 |
| 14 ExitReason 扩展破坏 doc 18 既定清单 | Low | 8 既有 enum 字符串值不变，仅追加 6 个新值 |
| Phase 4-5 跨会话 PR 落地长链路 | Low | PR-D1..D4 严格依赖链单向，PR-D1 必须先落 |