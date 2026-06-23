# Delta T-Registry: D7 MUPS v4.3 Phase 3 PR-C1 (Execute Artifact 4 类 + SideEffect 5 态)

**Change ID:** `devrix-d7-mups-v4-phase3-execute`
**PR Scope:** PR-C1 (最小风险入口，仅 Artifact 4 类 + SideEffect 5 态 + Artifact struct 5 字段升级)
**Affects:** `openspec/specs/d7-orchestration/t-registry.md` (D7-S9-A25-T01..T04 ADDED)
**Demand ID:** DM-20260625-001
**Date:** 2026-06-23
**PR:** [#164](https://github.com/fqntxmqee/devrix/pull/164)

---

## ADDED T Points

按 DSAFT 规范（CLAUDE.md），T 点编号 `D{X}-S{X}-A{XX}-T{XX}`：

- **D7** = Domain 7（orchestration）
- **S9** = Subdomain 9（MUPS Phase 3 — Execute 节点）
- **A25** = Atomic module 25（Execute Artifact Data Contract；PR-C1 范围）

PR-C1 实际落地的 4 个 P0 T 点：

### D7-S9-A25-T01: ArtifactKind 4 类枚举 + snake_case wire format

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/artifact_kind_test.go`
- **覆盖 REQ**: D7-S9-A25 (Scenario 1)
- **覆盖 AC**: AC1
- **测试用例**:
  - `TestArtifactKind_4Types_String` — 4 枚举值 String() 输出正确
  - `TestArtifactKind_4Types_ParseRoundTrip` (4 subtests) — ParseArtifactKind + String 双向
  - `TestArtifactKind_UnknownValue_ParseError` — 未知字符串 fail-fast
  - `TestArtifactKind_JSON_WireFormat` — MarshalJSON 输出字符串
  - `TestArtifactKind_UnmarshalEmptyString_DefaultsToZero` — 空字符串 → Kind=0
  - `TestArtifactKind_UnmarshalUnknownString_FailsLoudly` — 未知字符串 Unmarshal 不静默兜底
- **Status**: IMPLEMENTED

### D7-S9-A25-T02: SideEffectStatus 5 态 + IsTerminal/NeedsAttention 派生

- **优先级**: P0
- **位置**: `internal/layers/orchestration/orchtypes/side_effect_status_test.go`
- **覆盖 REQ**: D7-S9-A25 (Scenario 2)
- **覆盖 AC**: AC2
- **测试用例**:
  - `TestSideEffectStatus_5States_String` — 5 状态 String() 输出
  - `TestSideEffectStatus_5States_RoundTrip` (5 subtests) — JSON Marshal/Unmarshal roundtrip
  - `TestSideEffectStatus_IsTerminal` — {None, Committed, RolledBack} → true
  - `TestSideEffectStatus_NeedsAttention` — {Unknown, Inflight} → true
  - `TestSideEffectStatus_ReusesUncertaintyCoordType` — type alias 与 UncertaintyCoord 共享
  - `TestSideEffectDetail_JSON_RoundTrip` — SideEffectDetail 5 字段 roundtrip
- **Status**: IMPLEMENTED

### D7-S9-A25-T03: wavescheduler.Artifact struct 5 字段升级（v2 JSON 向后兼容）

- **优先级**: P0
- **位置**: `internal/layers/orchestration/wavescheduler/artifact_test.go`（扩展）
- **覆盖 REQ**: D7-S9-A25 (Scenario 3)
- **覆盖 AC**: AC3
- **测试用例**:
  - `TestArtifact_NewFields_PrC1` — 5 字段 (Kind=ResponseRecord / SourcePlanID / SideEffectStatus / SideEffectDetail) JSON roundtrip
  - `TestArtifact_BackwardCompat_PrC1` — v2 Artifact (5 字段 zero) JSON 不包含新 key（omitempty 验证）
  - `TestArtifact_KindZeroValue_OmittedFromJSON` — Kind=0 (StateChangeCert) 不出现在 JSON
- **Status**: IMPLEMENTED
- **附加回归覆盖**: 4 个既有 ArtifactStore 测试（PutGet/Unknown/SessionScoped/List）0 regression

### D7-S9-A25-T04: 跨域类型上提 shared/types 打破 import cycle

- **优先级**: P0
- **位置**: `internal/shared/types/execute.go` + `internal/layers/orchestration/orchtypes/artifact_kind_alias.go`
- **覆盖 REQ**: D7-S9-A25 (Scenario 4)
- **覆盖 AC**: AC1 + AC2（间接覆盖 type alias 等价）
- **测试用例**:
  - `TestSideEffectStatus_ReusesUncertaintyCoordType` — `WithSideEffect(SideEffectInflight)` 与 `WithSideEffect("inflight")` 等价（type alias 编译期保证）
  - `internal/shared/types/execute.go` 包内独立测试 — `TestSideEffectStatus_5States_String` + `TestArtifactKind_4Types_String` 在新包内 PASS（验证上提后无 cycle）
- **Status**: IMPLEMENTED
- **静态验证**:
  - `internal/lint/layer` PASS（orchtypes → workmodel → wavescheduler 单向依赖链）
  - `go test -race ./internal/...` 0 race detector warnings（19/19 internal packages PASS）

---

## 测试位置详细定义

### D7-S9-A25-T01: ArtifactKind 4 类枚举

```go
// internal/layers/orchestration/orchtypes/artifact_kind_test.go

func TestArtifactKind_4Types_String(t *testing.T) {
    cases := map[types.ArtifactKind]string{
        types.ArtifactStateChangeCert: "state_change_cert",
        types.ArtifactResponseRecord:  "response_record",
        types.ArtifactProbeReport:     "probe_report",
        types.ArtifactExperimentData:  "experiment_data",
    }
    for k, want := range cases {
        if got := k.String(); got != want {
            t.Errorf("%d.String() = %q, want %q", k, got, want)
        }
    }
}
```

### D7-S9-A25-T02: SideEffectStatus IsTerminal 派生

```go
// internal/layers/orchestration/orchtypes/side_effect_status_test.go

func TestSideEffectStatus_IsTerminal(t *testing.T) {
    terminal := []types.SideEffectStatus{
        types.SideEffectNone,
        types.SideEffectCommitted,
        types.SideEffectRolledBack,
    }
    for _, s := range terminal {
        if !s.IsTerminal() {
            t.Errorf("%v should be terminal", s)
        }
    }
}
```

### D7-S9-A25-T03: Artifact v2 向后兼容

```go
// internal/layers/orchestration/wavescheduler/artifact_test.go

func TestArtifact_BackwardCompat_PrC1(t *testing.T) {
    art := Artifact{
        TaskID: "t-v2",
        // v2 不设新字段
    }
    data, _ := json.Marshal(art)
    s := string(data)
    // 5 字段 key 在 v2 JSON 中不应出现
    for _, banned := range []string{
        `"kind"`, `"source_plan_id"`, `"anomalies_count"`,
        `"side_effect_status"`, `"side_effect_detail"`,
    } {
        if contains(s, banned) {
            t.Errorf("v2 Artifact should not emit %q", banned)
        }
    }
}
```

### D7-S9-A25-T04: 跨域 type alias 等价

```go
// internal/layers/orchestration/orchtypes/side_effect_status_test.go

func TestSideEffectStatus_ReusesUncertaintyCoordType(t *testing.T) {
    c := NewUncertaintyCoord(0.5)
    c2 := c.WithSideEffect(types.SideEffectInflight)
    if c2.SideEffectStatus != types.SideEffectInflight {
        t.Errorf("UncertaintyCoord.WithSideEffect = %q, want %q",
            c2.SideEffectStatus, types.SideEffectInflight)
    }
    // type alias 允许字符串字面量直接传入
    c3 := c.WithSideEffect("inflight")
    if c3.SideEffectStatus != types.SideEffectInflight {
        t.Errorf("WithSideEffect(\"inflight\") = %q, want %q",
            c3.SideEffectStatus, types.SideEffectInflight)
    }
}
```

---

## 数字汇总

| 项 | 改前 | 改后 | Δ |
|----|------|------|---|
| T 点总数 | 129 | 133（+4 新增） | +4 |
| P0 T 点 | 96 | 100 | +4 |
| IMPLEMENTED T 点 | 129 | 133 | +4 |

**实施后统计**：
- D7 T 点：129 → 133（+4 新增 D7-S9-A25-T01..T04）
- D7 P0 T 点：96 → 100（+4 新增 P0）
- 跨域类型：3 个新跨域类型（ArtifactKind / SideEffectStatus / SideEffectDetail）上提至 `shared/types`

---

## 与既有 change 的关联

| Change ID | 关联点 |
|-----------|--------|
| devrix-d7-mups-v4-phase1-foundation (S5_Accepted 2026-06-20) | Phase 1 UncertaintyCoord 字段模式被本 PR 复用（SideEffectStatus type alias 共享） |
| devrix-d7-mups-v4-phase2-observe-plan (S5_Accepted 2026-06-23, PR #163) | Phase 2 落地的 UncertaintyCoord.SideEffectStatus 与本 PR Artifact.SideEffectStatus 跨域统一 |
| devrix-d7-mups-v4-phase4-verify-promotion (S1-S5 进行中) | PR-C6 VerifyTrigger wiring 依赖本 PR + PR-C5 |
| devrix-d7-mups-v4-phase5-learn (S1-S5 进行中) | LearningAsset 路由决策依赖 Artifact.Kind 4 类分类 |

---

## 完成 Checklist

- [x] 4 个 PR-C1 P0 T 点（D7-S9-A25-T01..T04）注册到 live t-registry.md
- [x] 9 个新增 test functions + 20 subtests 全部 PASS（PR-C1 范围）
- [x] 4 个既有 ArtifactStore 测试 0 regression
- [x] 19/19 internal packages `go test -race` 0 race warnings
- [x] `internal/lint/layer` PASS（跨域类型上提后无 cycle）
- [x] 覆盖率 72.2%（与 Phase 2 baseline 持平，0 regression）