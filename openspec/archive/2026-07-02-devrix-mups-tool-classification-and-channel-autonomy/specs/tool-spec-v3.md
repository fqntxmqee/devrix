# Tool Spec v3 Schema (Delta)

**Capability:** tool-security
**Change ID:** devrix-mups-tool-classification-and-channel-autonomy (Phase A)
**Status:** DRAFT (S3 design)
**Version:** 1.0.0
**Implements Change:** DM-20260701-007 Phase A

本 spec 是 `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/` 目录下的 **独立 spec delta**（lite-mode 兼容：tool-security 已归档，不修改 archive 父 spec，留在本 change 目录）。

描述 ToolSpec v3 schema（6 新字段在末尾 + 4 type 定义 + JSON tag 一致性）。

---

## L15-TOOL-SPEC-V3: ToolSpec v3 Schema 加 6 字段（末尾）

ToolSpec v3 MUST 在 v2 9 字段基础上 +6 字段（**6 字段位置在末尾**，保证 position struct literal 不破坏），让 tool metadata 从 declaration 升级为 runtime control plane。

**T (DSAFT):** D2-S15-A02-T06..T12

### 新字段 (6 个 — 位置在末尾)

| 字段 | 类型 | R3 cycle 0 默认值 (PR-A 落地) | 作用 |
|------|------|-------------------------------|------|
| `EmissionClass` | `EmissionClass` enum | 按 tool Kind 自动推（见下表） | Execute 节点 4 ToolChannel 路由 |
| `ConvergenceContract` | `ConvergenceContract` struct | 按 tool Kind (read=None, write=StateChange, delegate=EvidenceRequired, free_fork=Quotient) | Verify input contract |
| `IterationBound` | `IterationBound` struct | 按 tool Kind (probe=Bounded(15), action=Bounded(8-10), fact=OpenEnded) | ProbeToolChannel L4 invariant 校验 |
| `SourceUncertainty` | `SourceUncertainty` struct | 按 tool Kind (read=Deterministic(1.0), bash=User(0.85), lsp=LLM(0.4)) | Verify calibrated_confidence |
| `MaxResultSizeChars` | `int` | 按 tool Kind (read=8192, write=4096, bash=8192) | D2 TruncateWithMarker 阈值 |
| `TruncateMarkerText` | `string` | `"[TRUNCATED at %d/%d chars, complete=false, REREAD may help]"` | 截断必附加 marker |

### 新类型 (3 个 + 1 个 enum)

```go
type EmissionClass int
const (
    EC_Fact EmissionClass = iota   // 0
    EC_Action                      // 1
    EC_Probe                       // 2
    EC_Experiment                  // 3
)

type ConvergenceContract struct {
    Kind         int     // 0=None, 1=StateChange, 2=Evidence, 3=Quotient
    Threshold    float64
    MinEvidence  int
}

type IterationBound struct {
    Kind     int     // 0=OpenEnded, 1=Bounded, 2=Quotient
    MaxN     int
    Quotient float64
}

type SourceUncertainty struct {
    Source int     // 0=Deterministic, 1=LLM, 2=User, 3=Memory
    Value  float64
}
```

### 19 工具默认 metadata 值（PR-A 已 INCLUDE 落地）

| 工具 | EmissionClass | ConvergenceContract | IterationBound | SourceUncertainty | MaxResultSizeChars |
|------|---------------|---------------------|-----------------|--------------------|----------------------|
| read_file | EC_Probe | **None** | Bounded(15) | Deterministic(1.0) | 8192 |
| write_file | EC_Action | StateChangeRequired | Bounded(10) | Deterministic(1.0) | 4096 |
| edit_file | EC_Action | StateChangeRequired | Bounded(10) | Deterministic(1.0) | 4096 |
| bash | EC_Action | StateChangeRequired | Bounded(8) | User(0.85) | 8192 |
| grep | EC_Probe | None | Bounded(15) | Deterministic(1.0) | 4096 |
| glob | EC_Probe | None | Bounded(15) | Deterministic(1.0) | 2048 |
| **lsp_goto_definition / lsp_hover / lsp_references** (3) | **EC_Fact** | None | OpenEnded | Deterministic(1.0) | 4096 |
| **lsp_workspace_symbol / lsp_code_action** (2) | **EC_Probe** | EvidenceRequired(min=1) | Bounded(12) | LLM(0.4) | 4096 |
| free_fork | EC_Experiment | Quotient(0.8) | Quotient(0.8) | User(0.85) | 2048 |
| query_diagnostics | EC_Fact | None | OpenEnded | Deterministic(1.0) | 4096 |
| verify_plan_execution | EC_Action | StateChangeRequired | Bounded(3) | LLM(0.4) | 2048 |
| ask_user_question | EC_Action | None | Bounded(1) | User(0.85) | 1024 |
| task_* (3 background) | EC_Action | None | OpenEnded | Deterministic(1.0) | 2048 |
| tool_search | EC_Fact | None | OpenEnded | Deterministic(1.0) | 1024 |
| delegate_* (5) | EC_Probe | EvidenceRequired(min=1) | Bounded(3) | LLM(0.4) | 4096 |

**lsp_* 拆分说明**（Codex Warning #11 修复）：deterministic read-only lsp call（goto_definition/hover/references）归 **EC_Fact**；探索性 lsp call（workspace_symbol/code_action）归 **EC_Probe**。

### Gherkin Scenarios

```gherkin
Feature: ToolSpec v3 Schema with 6 New Control Plane Fields

  Scenario: ToolSpec v3 struct has 15 fields (9 existing + 6 new)
    Given the ToolSpec struct in shared/contracts/tool_surface_v3.go
    When I count all struct fields
    Then the count equals 15
    And 9 fields exist before this change
    And 6 new fields are EmissionClass, ConvergenceContract, IterationBound, SourceUncertainty, MaxResultSizeChars, TruncateMarkerText

  Scenario: Existing 9 fields behavior unchanged (backward compatibility)
    Given any tool using ToolSpec v2 metadata
    When I upgrade to ToolSpec v3 with default values
    Then ReadOnly/Destructive/OpenWorld/ConcurrencySafe/DeferLoading behavior is unchanged
    And only 6 new fields are added with R3 cycle 0 defaults

  Scenario: read_file default metadata
    Given the read_file tool's ToolSpec
    When I read its EmissionClass
    Then it equals EC_Probe
    And its IterationBound.Kind equals Bounded
    And its IterationBound.MaxN equals 15
    And its MaxResultSizeChars equals 8192

  Scenario: bash default metadata
    Given the bash tool's ToolSpec
    When I read its EmissionClass
    Then it equals EC_Action
    And its ConvergenceContract.Kind equals StateChangeRequired
    And its SourceUncertainty.Source equals User

  Scenario: free_fork is Experiment class
    Given the free_fork tool's ToolSpec
    When I read its EmissionClass
    Then it equals EC_Experiment
    And its IterationBound.Kind equals Quotient
    And its IterationBound.Quotient equals 0.8
```

---

## 引用

- 父 spec: `openspec/specs/tool-security/spec.md`（lite-mode 已归档，本 delta 不修改，独立留在 change 目录）
- Proposal: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/proposal.md`
- Design: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/design.md`
- Tasks: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/tasks.md`
- 配套 spec delta: `specs/execute-channels.md` (Phase B) + `specs/verify-contract.md` (Phase C)
- Obsidian synthesis: `brain/01知识探索/项目/20260620-certain-architecture/core-concepts/54-tools-metadata-ideal-state-and-channel-autonomy.md`
- Clawcode TS 参照: clawcode/src/Tool.ts:1-792

---

## 更新历史

- 2026-07-01：v1 创建（6 新字段 + 4 type 定义）
- 2026-07-01：v1.1 Codex Critical/Warning 修复
  - 6 新字段位置在 struct 末尾 + JSON tag 一致性（Critical #9 + Info #3）
  - 19 工具默认 metadata 迁移到 PR-A 内（Critical #4）
  - lsp_* EC_Fact/Probe 拆分（Warning #11）
- 2026-07-01：v1.2 H12 共识 — grep/glob → EC_Probe + Bounded(15)；禁止 silent default（见 tasks T14）
  - 父 spec 引用改为 lite-mode 兼容（Warning #1）
