# Delta Spec: D7 MUPS v4.3 Phase 3 PR-C1 (Execute Artifact 4 类 + SideEffect 5 态)

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**PR Scope:** PR-C1 (最小风险入口，仅 Artifact 4 类 + SideEffect 5 态 + Artifact struct 5 字段升级)
**Affects:** `internal/shared/types/execute.go` (NEW — 跨域共享类型)
**Affects:** `internal/layers/orchestration/orchtypes/artifact_kind_alias.go` (NEW — type alias 转发)
**Affects:** `internal/layers/orchestration/orchtypes/uncertainty_coord.go` (MODIFIED — SideEffectStatus 改 type alias)
**Affects:** `internal/layers/orchestration/wavescheduler/types.go` (MODIFIED — Artifact +5 fields)
**Affects:** `openspec/specs/d7-orchestration/spec.md` (D7-S9-A25 ADDED)
**Affects:** `openspec/specs/d7-orchestration/t-registry.md` (D7-S9-A25-T01..T04 ADDED)
**Demand ID:** DM-20260625-001
**Date:** 2026-06-23
**PR:** [#164](https://github.com/fqntxmqee/devrix/pull/164)

---

## MODIFIED

（无 — PR-C1 不修改既有 Requirement）

## ADDED

### Requirement: D7-S9-A25 Execute Artifact Data Contract

Phase 3 Execute 节点的最小风险入口。提供跨域共享的 Artifact 数据契约：ArtifactKind 4 类枚举、SideEffectStatus 5 态、Artifact struct 5 字段升级。本 Requirement 不引入执行链路变更（PR-C2..C7 范围），仅落地数据契约层。

**Priority:** P0
**Package:** `internal/shared/types/` + `internal/layers/orchestration/orchtypes/` + `internal/layers/orchestration/wavescheduler/`
**T:** D7-S9-A25-T01 … D7-S9-A25-T04

#### Scenario: ArtifactKind 4 类枚举 + snake_case wire format

- GIVEN 定义 `shared/types.ArtifactKind{StateChangeCert, ResponseRecord, ProbeReport, ExperimentData}`
- WHEN 任意 Artifact 实例
- THEN `Kind` 字段为 4 枚举之一（uint8 0-3）
- AND `String()` 输出 snake_case wire format：`"state_change_cert"` / `"response_record"` / `"probe_report"` / `"experiment_data"`
- AND `MarshalJSON()` 输出字符串（不是数字），便于 D5 dashboard 字符串过滤
- AND `UnmarshalJSON()` 接收字符串，未知值 fail-fast 返回带 kind 名称的 error（不静默兜底）
- AND 测试 `TestArtifactKind_4Types_String` + `TestArtifactKind_4Types_ParseRoundTrip` + `TestArtifactKind_UnknownValue_ParseError` + `TestArtifactKind_JSON_WireFormat` + `TestArtifactKind_UnmarshalEmptyString_DefaultsToZero` + `TestArtifactKind_UnmarshalUnknownString_FailsLoudly` 6 functions / 9 subtests 全部 PASS

#### Scenario: SideEffectStatus 5 态 + IsTerminal/NeedsAttention 派生

- GIVEN 定义 `shared/types.SideEffectStatus{None, Unknown, Inflight, Committed, RolledBack}`（string alias）
- WHEN 任意 Artifact / UncertaintyCoord 实例
- THEN `SideEffectStatus` 字段为 5 枚举之一（字符串值 `"none"` / `"unknown"` / `"inflight"` / `"committed"` / `"rolled_back"`）
- AND `IsTerminal()` 返回 true 当 status ∈ {None, Committed, RolledBack}
- AND `NeedsAttention()` 返回 true 当 status ∈ {Unknown, Inflight}
- AND `orchtypes.SideEffectStatus = types.SideEffectStatus` type alias，Phase 2 UncertaintyCoord 调用方零修改
- AND 测试 `TestSideEffectStatus_5States_String` + `TestSideEffectStatus_5States_RoundTrip` + `TestSideEffectStatus_IsTerminal` + `TestSideEffectStatus_NeedsAttention` + `TestSideEffectStatus_ReusesUncertaintyCoordType` + `TestSideEffectDetail_JSON_RoundTrip` 6 functions / 11 subtests 全部 PASS

#### Scenario: wavescheduler.Artifact struct 5 字段升级（v2 JSON 向后兼容）

- GIVEN `wavescheduler.Artifact` 扩展 5 字段：`Kind` / `SourcePlanID` / `AnomaliesCount` / `SideEffectStatus` / `SideEffectDetail`
- WHEN 任意 Artifact 实例序列化
- THEN 5 字段全部带 `omitempty` JSON tag
- AND zero value（Kind=0/SourcePlanID=""/AnomaliesCount=0/SideEffectStatus=""/*SideEffectDetail=nil）不出现在 JSON 输出
- AND v2 调用方（仅写 v2 字段）序列化结果与升级前**字节相同**
- AND Unmarshal 接收 v2 JSON（5 字段缺失）不报错，零值默认到 `Kind=0 (StateChangeCert)` / `SideEffectStatus="" (None)` 等
- AND 测试 `TestArtifact_NewFields_PrC1`（5 字段 roundtrip）+ `TestArtifact_BackwardCompat_PrC1`（v2 字节兼容）+ `TestArtifact_KindZeroValue_OmittedFromJSON`（omitempty 行为）3 functions 全部 PASS
- AND 33 个既有 v2 Artifact 测试函数 0 regression

#### Scenario: 跨域类型上提 shared/types 打破 import cycle

- GIVEN orchtypes → workmodel → wavescheduler 单向依赖链
- WHEN Artifact.SideEffectStatus 需要与 UncertaintyCoord.SideEffectStatus 同类型（跨域 wire format 统一）
- AND 直接 wavescheduler → orchtypes 双向引用会破环
- THEN 把 `ArtifactKind` + `SideEffectStatus` + `SideEffectDetail` 上提到 `internal/shared/types/execute.go`（Phase 1 `MemoryEntry` precedent）
- AND `orchtypes` 提供 type alias + const re-export（`type SideEffectStatus = types.SideEffectStatus`）保持 Phase 2 调用方零修改
- AND `shared/types` → orchtypes 单向依赖，无 cycle
- AND 测试 `TestSideEffectStatus_ReusesUncertaintyCoordType` 验证 type alias 等价（`WithSideEffect(SideEffectInflight)` 与 `WithSideEffect("inflight")` 行为一致）

---

## REMOVED

### Requirement: orchtypes 内置 SideEffectStatus string alias

> **移除原因**：PR-C1 把 SideEffectStatus 上提到 `shared/types` 打破 import cycle，`orchtypes/side_effect_status.go` 独立文件 + `type SideEffectStatus string` 改为 `type SideEffectStatus = types.SideEffectStatus` type alias。
> 原 `orchtypes/side_effect_status.go` 文件删除（-65 行），常量与派生方法通过 type alias 自动继承。

### Requirement: orchtypes 内置 ArtifactKind uint8 enum

> **移除原因**：同上，ArtifactKind 上提到 `shared/types/execute.go`。
> 原 `orchtypes/artifact_kind.go` 文件删除（-90 行），4 枚举常量与 JSON wire format 方法通过 type alias + const re-export 保留。
> `orchtypes/artifact_kind_alias.go`（NEW ~20 行）作为转发层。

---

## 关键设计决策（PR-C1 5 项偏差）

| 决策 | 原 design | PR-C1 实取 | 理由 |
|------|----------|-----------|------|
| 跨域类型位置 | orchtypes 内部 | shared/types | 避免 wavescheduler → orchtypes 双向引用破环；Phase 1 `MemoryEntry` precedent |
| `PlanKindToArtifactKind()` 路由 | 自动 switch | 删除（PR-C2 重新设计） | plan.PlanKind 未落地（Phase 2 PR-B1 候选项），编译期 fail 会阻断 PR-C1 |
| `DetermineStatus()` 判定 | ToolResult 抽象 | 删除（PR-C2 Channel.Execute 内部实现） | 依赖 ToolResult.ExitCode/Confirmed/Compensated/SideEffect，PR-C1 范围无调用方 |
| `SideEffectDetail.SentAt/ConfirmedAt` | `time.Time` | `int64` unix nano | Phase 2 UncertaintyCoord 字段风格统一；避免跨时区序列化问题 |
| 升级字段数量 | 11 字段（全 Phase 3） | 5 字段（仅 PR-C1 范围） | PR-C1 最小风险入口；5 字段 omitempty 保持 v2 字节兼容 |

---

## 数字汇总

| 项 | 改前 | 改后 | Δ |
|----|------|------|---|
| T 点总数 | 129 | 133（+4 新增） | +4 |
| P0 T 点 | 96 | 100 | +4 |
| IMPLEMENTED T 点 | 129 | 133 | +4 |
| `shared/types` 新文件 | 0 | 1（`execute.go`） | +1 |
| `orchtypes` 新文件 | 0 | 1（`artifact_kind_alias.go`）+ 2 test files | +3 |
| `wavescheduler` 新字段 | 0 字段 | +5 字段（Artifact struct） | +5 |

**实施后统计**：
- D7 T 点：129 → 133（+4 新增）
- D7 P0 T 点：96 → 100（+4 新增）
- 跨域类型：ArtifactKind + SideEffectStatus + SideEffectDetail 从 `orchtypes` 上提至 `shared/types`
- 测试覆盖：9 个新增 test functions + 20 subtests 100% PASS（0 race / 0 regression）

---

## 与既有 change 的关联

| Change ID | 关联点 |
|-----------|--------|
| devrix-d7-mups-v4-phase1-foundation (S5_Accepted 2026-06-20) | Phase 1 OpenSpec 落地 UncertaintyCoord + AdaptiveThreshold wiring + Verifier + ExitReason；本 PR 复用 UncertaintyCoord 字段模式（SideEffectStatus type alias 共享） |
| devrix-d7-mups-v4-phase2-observe-plan (S5_Accepted 2026-06-23, PR #163 merged) | Phase 2 PR-A1 + PR-RF 落地 Observation 4 类 + UncertaintyReport + UncertaintyCoord；本 PR 是 Phase 3 Execute 节点的最小入口 |
| devrix-d7-mups-v4-phase4-verify-promotion (S1-S5 进行中) | Phase 4 Verify 节点升格；PR-C6 VerifyTrigger wiring 依赖本 PR + PR-C5 |
| devrix-d7-mups-v4-phase5-learn (S1-S5 进行中) | Phase 5 Learn 节点；与 Phase 3 衔接依赖 Artifact 5 字段（Kind + SideEffectStatus 路由） |

---

## PR-C2..C7 后续 OpenSpec 计划

PR-C1 已为后续 PR 奠定数据契约基础：

- **PR-C2 4 Channel** — 待 Phase 2 PR-B1（plan.PlanKind 落地）后开新 change。
- **PR-C3 StrategyDecider + RetryPolicy** — L0 硬规则可独立开新 change；L1 LLM 决策等 Phase 5 Learn 闭环后再启用。
- **PR-C4 ToolSpec v3** — 可单 PR 推进（与 Phase 2 路径无依赖）。
- **PR-C5 ExecutionEvidence** — 可单 PR 推进。
- **PR-C6 VerifyTrigger wiring** — 需 PR-C1 + PR-C5 先行。
- **PR-C7 Executor + DispatchWorker v2** — 终点 PR，依赖前 6 个。

每个 PR 各自走 S1-S6 完整流程（proposal.md → design.md → tasks.md → spec_delta.md → t-registry_delta.md → 5 PR 联动 squash auto-merge → verify-archive）。

---

## 完成 Checklist

- [x] 4 个 P0 T 点（PR-C1 范围）注册到 t-registry.md D7-S9-A25 章节
- [x] D7-S9-A25 Requirement ADDED to spec.md
- [x] shared/types/execute.go 创建（ArtifactKind + SideEffectStatus + SideEffectDetail）
- [x] orchtypes/artifact_kind_alias.go 创建（type alias + const re-export）
- [x] orchtypes/uncertainty_coord.go 改 type alias（+8/-5）
- [x] orchtypes/artifact_kind.go + side_effect_status.go 删除（-90 + -65）
- [x] wavescheduler/types.go Artifact +5 fields (+12/-1)
- [x] 9 个 test functions + 20 subtests IMPLEMENTED（PR-C1 范围）
- [x] PR #164 created + auto-merge + squash + delete-branch（CI 全绿）
- [x] archive spec.md (Delta Spec) + t-registry.md (Delta T-Registry) 创建
- [x] live spec.md v4.0.0 → v4.1.0 同步
- [x] live t-registry.md v3.8.0 → v3.9.0 同步
- [x] demand-archive-index.md 新增 DM-20260625-001 行
- [x] ./scripts/verify-archive.sh devrix-d7-mups-v4-phase3-execute 12/12 PASS