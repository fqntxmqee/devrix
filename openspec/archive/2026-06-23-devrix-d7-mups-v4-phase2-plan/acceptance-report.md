# Acceptance Report — DM-20260623-001-PRB1 (Phase 2 PR-B1)

**Change ID:** `devrix-d7-mups-v4-phase2-plan`
**Demand ID:** DM-20260623-001-PRB1
**PR Scope:** PR-B1（Plan 4 类 + Planner + MatchKind 4 Rules + Plan.Validate）
**Acceptance Date:** 2026-06-23
**Author:** MUPS v4.3 Phase 2 Plan 节点落地梳理
**Status:** ✅ S5_Accepted

---

## 1. 验收范围

本报告验收 PR-B1（Plan Data Contract + Planner）的实现质量与设计一致性。

| 维度 | 范围 |
|------|------|
| **代码变更** | 6 新文件 +1458/-0；plan package 完整实现 |
| **测试变更** | 30 tests / 93.5% coverage / 0 race detector warnings |
| **文档变更** | spec.md v4.2.0→v4.3.0 (D7-S8-A22 Requirement) + t-registry.md v3.10.0→v3.11.0 (T01..T03 IMPLEMENTED) |
| **不做的事** | PR-A2/A3/A4 范围不动 / PR-B2/B3 不动 / Phase 3 PR-C2 独立推进 / Phase 4/5 入口预留 |

## 2. 验收标准达成

### 2.1 P0 验收（AC1-AC3）

| ID | 验收标准 | 状态 | 证据 |
|----|---------|------|------|
| **AC1** | PlanKind 4 类枚举 + String() snake_case + MarshalJSON/UnmarshalJSON + ParsePlanKind + KindUnset omitempty | ✅ PASS | D7-S8-A22-T01 IMPLEMENTED；`plan_test.go::TestPlanKind_4Types_AreDistinct` + `TestPlanKind_String_SnakeCase` + `TestPlanKind_MarshalJSON_KnownValues` + `TestPlanKind_UnmarshalJSON_RoundTrip` + `TestParsePlanKind` 5/5 PASS |
| **AC2** | Plan.SourceObservationIDs 必填 + 防御性拷贝 + ReverseLookupObservations Phase 4 入口 | ✅ PASS | D7-S8-A22-T02 IMPLEMENTED；`TestPlan_SourceObservationIDs_Required` + `TestPlan_NewPlan_CopiesObservationIDs` + `TestPlan_SourceObservationIDs_ReverseLookup_{Exact,DuplicateIDs,Empty}` 5/5 PASS |
| **AC3** | MatchKind 4 规则 + uncertainty-first tie-break + DefaultPlanner 集成 | ✅ PASS | D7-S8-A22-T03 IMPLEMENTED；`TestMatchKind_4Rules` + `TestDefaultPlanner_Plan_{EmptyObservationIDs_FailsFast,CommitmentFromSingleStep,ExplorationFromAnomalies,StrengthMatchesFormula,ValidationFailurePropagates,BlastRadiusPropagated}` 8/8 PASS |

### 2.2 测试与质量

| 项 | 目标 | 实际 | 状态 |
|----|------|------|------|
| 单元测试 PASS | 100% | 30/30 PASS, 0 race | ✅ PASS |
| 覆盖率 | ≥ 80% | 93.5% | ✅ PASS（超 13.5 pp） |
| go vet | 0 issue | 0 issue | ✅ PASS |
| go build | 0 error | 0 error | ✅ PASS |
| go test -race | 0 warning | 0 warning | ✅ PASS |

### 2.3 跨域一致性

| 检查项 | 结论 | 说明 |
|--------|------|------|
| 与 `shared/types.ArtifactKind` wire format 协同 | ✅ PASS | PlanKind 4 类 snake_case 与 ArtifactKind 4 类 snake_case 命名空间独立；D5 dashboard 字符串过滤不会冲突 |
| 与 Phase 1 UncertaintyCoord JSON 兼容 | ✅ PASS | Plan 不引用 UncertaintyCoord；SourceObservationIDs 仅存 ID 字符串，不存 UncertaintyCoord 字段 |
| 与 Phase 2 PR-A1 Observation 不可变模式 | ✅ PASS | Plan 不可变 + 防御性拷贝；与 Observation 不可变模式一致 |
| SentinelError 模式 | ✅ PASS | 9 sentinels + 3 helpers；sharederrors.WithCode 包装；与 Phase 1/2 同款 |

### 2.4 决议闭环（C2/W8 MatchKind 签名收紧）

| 决议 | 状态 | 证据 |
|------|------|------|
| C2/W8: MatchKind 签名收紧为 `(*UncertaintyReport)` | ✅ PASS | `MatchKind(quantizedKind string, stepCount, anomaliesCount int)` 接受报告字段；PR-C2 可直接消费无需再次破坏 API |
| C2/W8: 避免从 PR-A1 stub (`[]Observation`) 升级 | ✅ PASS | MatchKind 不依赖 Observation 列表；只依赖 quantizedKind + stepCount + anomaliesCount |
| C2/W8: PlanKind wire format snake_case | ✅ PASS | `commitment_plan` / `protocol_plan` / `scenario_plan` / `exploration_plan` 4 字符串互斥 |
| C2/W8: D5 dashboard 字符串过滤可统一 | ✅ PASS | 与 shared/types.ArtifactKind 同款约定 |

## 3. 实施质量

### 3.1 PR 信息

| 项 | 值 |
|----|-----|
| PR URL | https://github.com/fqntxmqee/devrix/pull/167 |
| 分支 | feat/devrix-d7-mups-v4-phase2-pr-b1 (from master) |
| 文件数 | 6 |
| 代码行数 | +1458 / -0 |
| 风险等级 | Low |
| Squash merge | ✅ 2026-06-23 |
| Auto-merge | ✅ enabled |

### 3.2 文件清单

| 文件 | 类型 | 行数 | 描述 |
|------|------|------|------|
| `internal/layers/orchestration/plan/plan.go` | NEW | ~150 | PlanKind enum + String/Marshal/Unmarshal/Parse |
| `internal/layers/orchestration/plan/blast_radius.go` | NEW | ~120 | BlastRadius + PersistScope + FailureCriterion + Step |
| `internal/layers/orchestration/plan/errors.go` | NEW | ~100 | 9 sentinels + 3 helpers |
| `internal/layers/orchestration/plan/plan_struct.go` | NEW | ~280 | Plan struct + With* + Validate + ReverseLookupObservations |
| `internal/layers/orchestration/plan/planner.go` | NEW | ~200 | Planner interface + DefaultPlanner + MatchKind + strengthFloor |
| `internal/layers/orchestration/plan/plan_test.go` | NEW | ~700 | 30 tests / 93.5% coverage |

### 3.3 spec.md / t-registry 同步

| 文件 | 变更 | 状态 |
|------|------|------|
| `openspec/specs/d7-orchestration/spec.md` | v4.2.0 → v4.3.0；新增 D7-S8-A22 Requirement (3 Scenarios) | ✅ DONE |
| `openspec/specs/d7-orchestration/t-registry.md` | v3.10.0 → v3.11.0；新增 D7-S8-A22-T01..T03 (3 IMPLEMENTED) + IMPLEMENTED 139→142, P0 106→109 | ✅ DONE |

## 4. 风险评估

| 风险 | 等级 | 缓解措施 |
|------|------|---------|
| Plan 不可变性被破坏（外部 mutation） | Low | NewPlan 防御性拷贝 sourceObservationIDs；With* 全部返回新副本；测试覆盖 |
| MatchKind 边界用例 | Low | uncertainty-first tie-break 显式编码；4 规则覆盖；strengthFloor float drift 容忍 `>= 0.89` |
| PR-C2 依赖未就绪 | Low | PlanKind / Plan.SourceObservationIDs / Plan.BlastRadius 已全部公开；PR-C2 独立 change |
| Phase 4 Verify 反向追溯入口预留 | Low | ReverseLookupObservations 已实现；Phase 4 仅需消费方 |

## 5. 后续 PR 依赖

| PR | 依赖 | 状态 |
|----|------|------|
| PR-C2 (ChannelRouter) | PlanKind 4 类 + SourceObservationIDs + BlastRadius | ✅ 已就绪 |
| Phase 4 Verify | ReverseLookupObservations | ✅ 入口已留 |
| Phase 5 Learn | Plan.AnomaliesCount 累积 | ✅ 字段已留 |

## 6. 结论

**S5 验收结论：✅ ACCEPTED**

- 3/3 P0 AC 全部 PASS
- 5/5 设计同步全部完成
- 30/30 测试 100% PASS（0 race）
- 覆盖率 93.5% ≥ 80% gate
- C2/W8 MatchKind 签名收紧决议闭环
- 跨域一致性 + 决议闭环 + 风险缓解全部到位

PR-B1 S1→S6 流程可进入 S6 归档。
