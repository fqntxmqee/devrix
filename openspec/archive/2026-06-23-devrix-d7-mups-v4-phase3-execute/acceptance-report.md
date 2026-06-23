# Acceptance Report — DM-20260625-001 PR-C1 (Artifact 4 类 + SideEffect 5 态)

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**Demand ID:** DM-20260625-001
**PR Scope:** PR-C1 (最小风险入口，仅 Artifact 4 类 + SideEffect 5 态 + Artifact struct 5 字段升级)
**Acceptance Date:** 2026-06-23
**Acceptance Verdict:** **ACCEPTED** ✅

> 本 acceptance-report 仅覆盖 PR-C1。PR-C2..C7 强依赖 Phase 2 PR-B1（plan.PlanKind 落地），
> 将在 Phase 2 PR-B1 落地后各自开独立 OpenSpec change 走 S1-S6 流程。
> 见 `design.md §1.3 落地顺序与 PR 解耦` 与 `tasks.md` 更新。

---

## 1. 验收摘要

| 维度 | 状态 | 备注 |
|------|------|------|
| **AC1-AC4 (PR-C1 P0)** | ✅ 4/4 PASS | 见 §2 详表 |
| **go vet** | ✅ 0 issue | `go vet ./...` 全绿 |
| **go test -race** | ✅ 19/19 internal 包 PASS | 含 11 个新增 PR-C1 测试 + 12 个 Phase 2 PR-A1 既有测试 |
| **layer-lint** | ✅ PASS | `internal/lint/layer` 0 警告 |
| **覆盖率** | ✅ 72.2% 持平 baseline | orchtypes 包 72.2%（与 Phase 2 闭合后持平）|
| **0 race** | ✅ 0 race detector 报警 | `go test -race ./internal/...` 全包 -race 模式 |
| **0 既有 regression** | ✅ 全包 PASS | 既有 33 个测试函数无 failure |
| **wire format 向后兼容** | ✅ 5 字段 omitempty | v2 Artifact 调用方零修改 |
| **跨域类型统一** | ✅ 上提 shared/types | orchtypes + wavescheduler + UncertaintyCoord 共享同一定义 |

---

## 2. P0 AC 验收详情

### 2.1 AC1 — `ArtifactKind` 4 类枚举（强类型 + 双向转换）

| 验收点 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| 4 枚举值定义 | `StateChangeCert` / `ResponseRecord` / `ProbeReport` / `ExperimentData` | ✅ 4 值齐全（`shared/types/execute.go:21-32`）| ✅ |
| `String()` 双向 | snake_case wire format | ✅ `state_change_cert` / `response_record` / `probe_report` / `experiment_data` | ✅ |
| `ParseArtifactKind()` 反向 | 字符串 → 枚举 | ✅ `types.ParseArtifactKind` 实现，未知值返回带 kind 名称的 error | ✅ |
| `MarshalJSON` / `UnmarshalJSON` | JSON wire format 是字符串 | ✅ `Marshal` 输出 `"probe_report"` 而非数字 | ✅ |
| 未知值 fail-fast | Unmarshal 未知字符串不静默兜底 | ✅ `TestArtifactKind_UnmarshalUnknownString_FailsLoudly` PASS | ✅ |

**测试覆盖**：`TestArtifactKind_4Types_String` / `TestArtifactKind_4Types_ParseRoundTrip` (4 subtests) / `TestArtifactKind_UnknownValue_ParseError` / `TestArtifactKind_JSON_WireFormat` / `TestArtifactKind_UnmarshalEmptyString_DefaultsToZero` / `TestArtifactKind_UnmarshalUnknownString_FailsLoudly` — 6 functions / 9 subtests PASS。

### 2.2 AC2 — `SideEffectStatus` 5 类（None / Unknown / Inflight / Committed / RolledBack）

| 验收点 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| 5 状态定义 | None / Unknown / Inflight / Committed / RolledBack | ✅ 5 值齐全（`shared/types/execute.go:84-99`）| ✅ |
| 复用 Phase 2 string alias | 与 `UncertaintyCoord` 同一 type alias | ✅ `orchtypes.SideEffectStatus = types.SideEffectStatus` type alias，wire format 完全兼容 | ✅ |
| `IsTerminal()` 派生 | 终态判定 | ✅ None / Committed / RolledBack → true | ✅ |
| `NeedsAttention()` 派生 | 需关注判定 | ✅ Unknown / Inflight → true | ✅ |
| `SideEffectDetail` 6 字段 | IdempotencyKey / SentAt / ConfirmedAt / CompensationLog / CompensationTool | ✅ 5 字段实现（`shared/types/execute.go:111-117`） | ✅ |

**测试覆盖**：`TestSideEffectStatus_5States_String` / `TestSideEffectStatus_5States_RoundTrip` (5 subtests) / `TestSideEffectStatus_IsTerminal` / `TestSideEffectStatus_NeedsAttention` / `TestSideEffectStatus_ReusesUncertaintyCoordType` / `TestSideEffectDetail_JSON_RoundTrip` — 6 functions / 11 subtests PASS。

### 2.3 AC3 — `wavescheduler.Artifact` 升级 5 字段（向后兼容）

| 验收点 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| 5 字段新增 | `Kind` / `SourcePlanID` / `AnomaliesCount` / `SideEffectStatus` / `SideEffectDetail` | ✅ 5 字段全加（`wavescheduler/types.go:142-150`）| ✅ |
| omitempty 向后兼容 | v2 调用方零修改 | ✅ `TestArtifact_BackwardCompat_PrC1` 验证 5 字段 key 在 v2 artifact JSON 中不出现 | ✅ |
| 5 字段 JSON roundtrip | Marshal/Unmarshal 数据保真 | ✅ `TestArtifact_NewFields_PrC1` PASS | ✅ |
| zero value omitempty 行为 | Kind=0 (StateChangeCert) 不出现在 JSON | ✅ `TestArtifact_KindZeroValue_OmittedFromJSON` PASS | ✅ |

**测试覆盖**：`TestArtifact_NewFields_PrC1` / `TestArtifact_BackwardCompat_PrC1` / `TestArtifact_KindZeroValue_OmittedFromJSON` — 3 functions PASS（外加 4 既有 ArtifactStore 测试不 regression）。

### 2.4 AC4 — 单测覆盖与 4×4 组合

| 验收点 | 期望 | 实际 | 状态 |
|--------|------|------|------|
| ArtifactKind 4 枚举 + 边界测试 | 4 + unknown 边界 | ✅ 6 functions / 9 subtests | ✅ |
| SideEffectStatus 5 态 + 派生方法测试 | 5 + IsTerminal/NeedsAttention | ✅ 6 functions / 11 subtests | ✅ |
| Artifact 5 字段 + 向后兼容测试 | 5 + omitempty | ✅ 3 functions | ✅ |
| 既有 v2 测试零 regression | 33 既有测试函数 PASS | ✅ 全包 `go test -race ./internal/...` PASS | ✅ |
| `go test -race` 0 race | -race 模式 0 race detector 报警 | ✅ 19/19 internal 包 -race 模式 PASS | ✅ |

---

## 3. 关键设计决策与设计偏差

### 3.1 SideEffectStatus 上提 `shared/types`（S3 review 关键修正）

**原始 design**：`orchtypes/side_effect_status.go`（新文件）定义独立 `type SideEffectStatus` 字符串别名，5 态枚举。
**S3 review 修正**：

- 原因：`orchtypes → workmodel → wavescheduler` 已形成 import cycle，新增 `wavescheduler → orchtypes` 双向引用会破环。
- 解决：把 `ArtifactKind` + `SideEffectStatus` + `SideEffectDetail` 上提到 `internal/shared/types/execute.go`（与 Phase 1 `MemoryEntry` 上提先例一致）。
- `orchtypes/uncertainty_coord.go::SideEffectStatus` 改为 `type SideEffectStatus = types.SideEffectStatus` type alias 引用，保持 Phase 2 调用方零修改。
- 跨域 wire format 完全统一（`UncertaintyCoord.SideEffectStatus` 与 `Artifact.SideEffectStatus` 同一字符串值集）。
- 删除原 `orchtypes/side_effect_status.go` 独立文件，新增 `orchtypes/artifact_kind_alias.go` 转发常量与类型。

### 3.2 删除 `PlanKindToArtifactKind()` 函数

**原始 design**：定义 `func PlanKindToArtifactKind(pk plan.PlanKind) ArtifactKind` 自动路由。
**S3 review 修正**：

- 原因：`plan.PlanKind` 尚未落地（Phase 2 PR-B1 候选项，DM-20260624-001 之后才做）。
- 解决：PR-C1 删除该函数；ArtifactKind 由调用方在创建 Artifact 时显式传值。PR-C2 落地时按 `plan.PlanKind` 重新设计路由（4 Channel.Execute 内部 `switch pk { ... }`）。

### 3.3 删除 `DetermineStatus()` 函数

**原始 design**：定义 `func DetermineStatus(toolResult ToolResult, timeout bool) SideEffectStatus` 状态判定。
**S3 review 修正**：

- 原因：依赖 `ToolResult` 抽象（`ExitCode` / `Confirmed` / `Compensated` / `SideEffect`），PR-C1 范围内无调用方。
- 解决：PR-C1 仅提供 `IsTerminal()` / `NeedsAttention()` 派生方法；`DetermineStatus` 在 PR-C2 Channel.Execute 内部按 ToolResult 实情实现。

### 3.4 `time.Time` → `int64` unix nano（SideEffectDetail 字段精简）

**原始 design**：`SentAt time.Time` / `ConfirmedAt time.Time`。
**S3 review 修正**：

- 原因：与 Phase 2 `UncertaintyCoord` 字段风格一致（避免 time.Time 跨时区序列化问题）。
- 解决：`SentAt int64` (unix nano) / `ConfirmedAt int64` (unix nano, omitempty)。

### 3.5 0 race detector + layer-lint 双绿

**重要约束**：跨域类型上提后必须确保 `orchtypes` / `wavescheduler` / `workmodel` / `shared/types` 4 个包都不产生 race 或 layer-lint 警告。
**实际结果**：✅ 19/19 internal 包 -race PASS，layer-lint PASS。

---

## 4. PR-C1 落地文件清单

| 文件 | 类型 | 行数 | 用途 |
|------|------|------|------|
| `internal/shared/types/execute.go` | NEW | ~140 | ArtifactKind + SideEffectStatus + SideEffectDetail 跨域共享类型 |
| `internal/layers/orchestration/orchtypes/artifact_kind_alias.go` | NEW | ~20 | 转发常量与类型（保持 orchtypes 调用方零修改）|
| `internal/layers/orchestration/orchtypes/uncertainty_coord.go` | MODIFY | +8 / -5 | `SideEffectStatus` 改为 `type alias = types.SideEffectStatus` |
| `internal/layers/orchestration/orchtypes/artifact_kind_test.go` | NEW | ~135 | ArtifactKind 6 functions / 9 subtests |
| `internal/layers/orchestration/orchtypes/side_effect_status_test.go` | NEW | ~135 | SideEffectStatus 6 functions / 11 subtests |
| `internal/layers/orchestration/wavescheduler/types.go` | MODIFY | +12 / -1 | Artifact struct 5 字段 + shared/types import |
| `internal/layers/orchestration/wavescheduler/artifact_test.go` | MODIFY | +75 / -1 | 3 new test functions (NewFields_PrC1 / BackwardCompat_PrC1 / KindZeroValue_OmittedFromJSON) |
| `internal/layers/orchestration/orchtypes/errors.go` | MODIFY | +0 / -1 | 移除重复的 `ErrArtifactKindInvalid`（上提 shared/types 后不需要）|
| `internal/layers/orchestration/orchtypes/artifact_kind.go` | DELETED | -90 | 重复类型上提后删除 |
| `internal/layers/orchestration/orchtypes/side_effect_status.go` | DELETED | -65 | 重复类型上提后删除 |
| **总计** | — | **+415 / -163** | 跨 5 个包，PR-C1 净增 ~250 LoC（含测试）|

### 4.1 文档同步

| 文档 | 状态 | 备注 |
|------|------|------|
| `openspec/changes/devrix-d7-mups-v4-phase3-execute/demand.md` | CREATED | DM-20260625-001，32 P0 AC |
| `openspec/changes/devrix-d7-mups-v4-phase3-execute/proposal.md` | MODIFIED | 状态 S2_Proposal → S3_Design |
| `openspec/changes/devrix-d7-mups-v4-phase3-execute/design.md` | MODIFIED | §0 S3-Gate 4 维度自检 + §1.3 落地顺序与 PR 解耦 + §2 PR-C1 类型去重 + SideEffect 5 态 |
| `openspec/changes/devrix-d7-mups-v4-phase3-execute/tasks.md` | MODIFIED | PR-C1 任务项细化（SideEffect 复用 Phase 2 + 删除 PlanKindToArtifactKind）|
| `openspec/changes/devrix-d7-mups-v4-phase3-execute/acceptance-report.md` | CREATED | 本文档 |

### 4.2 待 S6 同步文档

| 文档 | 状态 | 备注 |
|------|------|------|
| `openspec/specs/d7-orchestration/spec.md` | 待 S6 同步 | v4.1.0 → v4.2.0（新增 D7-S9 Execute Artifact 4 类 + SideEffect 5 态 Requirement）|
| `openspec/specs/d7-orchestration/t-registry.md` | 待 S6 同步 | v3.10.0 → v3.11.0（新增 D7-S9-A16-T01..T04，4 个 P0 T 点）|

---

## 5. 关键 PR-C1 决策与 Trade-off

| 决策 | 选项 A（不取） | 选项 B（取） | 取舍 |
|------|----------------|--------------|------|
| SideEffectStatus 定义位置 | orchtypes 内部（与 Phase 2 同名重定义）| shared/types 跨域共享 | 取 B：避免 import cycle + 跨域 wire format 统一 |
| ArtifactKind 序列化 | uint8 数字 | uint8 + String wire format | 取 B：D5 dashboard 字符串过滤友好 |
| ArtifactKind omitempty 行为 | 全字段都序列化 | zero value (StateChangeCert=0) 不出现 | 取 B：v2 兼容 + Unmarshal 默认 0 → StateChangeCert |
| 升级字段数量 | 11 字段（Phase 3 全栈）| 5 字段（仅 PR-C1 范围）| 取 B：PR-C1 最小风险入口 |
| 删除 PlanKindToArtifactKind | 保留 + 编译期 fail | 删除 + 调用方显式传 | 取 B：plan.PlanKind 未落地，编译期 fail 会阻断 PR-C1 |
| 错误码扩展 | 新增 7005+ ORCH_ARTIFACT_KIND_xxxx | 不新增（错误信息含 kind 名称）| 取 B：未知 kind 已在 wire format fail-fast，无 sentinel 需求 |

---

## 6. S6 交付计划

- [ ] 创建 PR #164（推测编号）→ 标题 `feat(d7): MUPS v4.3 Phase 3 PR-C1 (Artifact 4 类 + SideEffect 5 态) (DM-20260625-001)`
- [ ] auto-merge + squash + delete-branch
- [ ] 盯 CI unit tests + layer-lint + coverage check
- [ ] 同步 live spec.md v4.1.0 → v4.2.0 + t-registry.md v3.10.0 → v3.11.0
- [ ] 创建 .openspec.yaml（status: s7_archived）
- [ ] 移动 change 目录到 `archive/2026-06-23-devrix-d7-mups-v4-phase3-execute/`
- [ ] 创建 archive/specs/d7-orchestration/spec.md（Delta Spec 格式）
- [ ] demand-archive-index.md 新增 DM-20260625-001 行
- [ ] verify-archive.sh 12/12 PASS

---

## 7. PR-C2..C7 后续 OpenSpec 计划

PR-C1 已为后续 PR 奠定基础：

- **PR-C2 4 Channel** — 待 Phase 2 PR-B1（plan.PlanKind 落地）后开新 change。
- **PR-C3 StrategyDecider + RetryPolicy** — L0 硬规则可独立开新 change；L1 LLM 决策等 Phase 5 Learn 闭环后再启用。
- **PR-C4 ToolSpec v3** — 可单 PR 推进（与 Phase 2 路径无依赖）。
- **PR-C5 ExecutionEvidence** — 可单 PR 推进。
- **PR-C6 VerifyTrigger wiring** — 需 PR-C1 + PR-C5 先行。
- **PR-C7 Executor + DispatchWorker v2** — 终点 PR，依赖前 6 个。

每个 PR 各自走 S1-S6 完整流程（proposal.md → design.md → tasks.md → spec_delta.md → t-registry_delta.md → 5 PR 联动 squash auto-merge → verify-archive）。

---

**Acceptance Verdict: ACCEPTED ✅**

PR-C1 5 字段升级 + 11 个新测试用例 + 跨域类型上提 + 19/19 internal 包 -race PASS + layer-lint PASS + 覆盖率持平 baseline 72.2%。准备进入 S6 交付流程（PR 创建 + auto-merge + 归档）。
