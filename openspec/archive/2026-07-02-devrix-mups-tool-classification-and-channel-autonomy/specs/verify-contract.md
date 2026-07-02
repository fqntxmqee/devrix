# VerifyContract + Reason Transmission (Delta)

**Capability:** d7-orchestration
**Change ID:** devrix-mups-tool-classification-and-channel-autonomy (Phase C)
**Status:** DRAFT (S3 design)
**Version:** 1.0.0
**Implements Change:** DM-20260701-007 Phase C

本 spec 是 `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/` 目录下的 **独立 spec delta**（lite-mode 兼容：d7-orchestration 已 lite-mode 化）。

描述 Verify 节点 input contract 强校验 + verdict reason 全链路透传。

---

## D7-VERIFY-CONTRACT-1: Verify Input Contract 4 元组强校验

Verify 节点 MUST 用 VerifyContract 4 元组强校验：
1. `expected_class` (按 tool.EmissionClass 推)
2. `deliverable_text` (mandatory if DeliverableRequired=true, MinChars by task_kind)
3. `evidence` (mandatory if EvidenceRequired=true)
4. `source_uncertainty` (calibrated_confidence 计算，Σ(su×w)/Σ(w) 归一化)

**T (DSAFT):** D7-S10-A50-T01..T03 + D7-S2-A50-T07

### VerifyContract struct + NewVerifyContract

```go
// /Users/fukai/workspace/devrix/internal/layers/orchestration/executionflow/verify/verify_contract.go
type VerifyContract struct {
    ExpectedClass       EmissionClass
    DeliverableRequired bool
    DeliverableMinChars int               // task_kind-dependent: review=20, edit=10, test=30, observe=10
    EvidenceRequired    bool
    MinEvidenceCount    int
    MinSourceQuality    float64           // calibrated_confidence 下限，默认 0.5
}

// NewVerifyContract 显式构造器（Codex Info #4 修复：防 Go 零值陷阱，MinSourceQuality=0 永远过）
func NewVerifyContract(taskKind string, expected EmissionClass) VerifyContract {
    return VerifyContract{
        ExpectedClass:       expected,
        DeliverableRequired: true,
        DeliverableMinChars: defaultMinCharsForTaskKind(taskKind),
        EvidenceRequired:    expected != EC_Fact,
        MinEvidenceCount:    1,
        MinSourceQuality:    0.5,
    }
}

func defaultMinCharsForTaskKind(kind string) int {
    switch kind {
    case "review":   return 20
    case "edit":     return 10
    case "test":     return 30
    case "refactor": return 40
    case "observe":  return 10
    default:         return 20  // safe default
    }
}

func Verify(ctx context.Context, c VerifyContract, out *TurnOutput) (*Verdict, error)
```

### 校验规则（4 元组）

| 校验项 | 触发 | Verdict | Reason |
|--------|------|---------|--------|
| deliverable_text 缺失 | `DeliverableRequired && len(text) < DeliverableMinChars` | FAIL | `deliverable_missing` |
| deliverable_text 太短 | `DeliverableRequired && len(text) < DeliverableMinChars` | FAIL | `deliverable_too_short` |
| evidence 数量不足 | `EvidenceRequired && len(tool_calls) < MinEvidenceCount` | FAIL | `evidence_insufficient` |
| calibrated_confidence 低 | `CC < MinSourceQuality` | PARTIAL | `source_uncertainty_high` |
| 全过 | 所有校验通过 | PASS | `null` |

### Burden of Proof by EmissionClass（H5 / demand §3.3，P1-AC-3）

| EmissionClass | 举证要求 | FAIL reason |
|---------------|----------|-------------|
| Fact | deliverable text 自证 | `deliverable_missing` / `deliverable_too_short` |
| Action | state change evidence 必传 | `evidence_insufficient` / `state_change_failed` |
| Probe | `source_quality` / calibrated_confidence 必填 | `source_uncertainty_high` |
| Experiment | result reproducibility 必传 | `experiment_inconclusive` |

**T (DSAFT):** D7-S10-A50-T04 (`TestBurdenOfProofByClass`)

### calibrated_confidence 计算公式（Codex Critical #2+#6 修复）

**旧公式（被否决 — 数学崩）**：
```yaml
calibrated_confidence = Σ(source_uncertainty × emission_class_weight) / 4
# 权重和 0.25+0.50+0.30+0.20 = 1.25，分母 4 → 几乎所有 Verdict 触发 PARTIAL
```

**新公式**（按权重和归一化）：
```yaml
calibrated_confidence = Σ(source_uncertainty × emission_class_weight) / Σ(emission_class_weight)

emission_class_weight（Codex Critical #6 修复：EC_Fact > EC_Action 排序）：
  EC_Fact:        0.50  # 确定性读，要么返内容要么 error
  EC_Action:      0.35  # 状态变更可观察但可失败/回滚
  EC_Probe:       0.20  # 探索性，依赖 LLM 判断
  EC_Experiment:  0.10  # 最不确定
  # Σ(w) = 1.15
```

**验算**：
- 单 `EC_Fact + User(0.85)` 会话：CC = 0.85 × 0.50 / 0.50 = **0.85** ✓
- 单 `EC_Action + User(0.85)` 会话：CC = 0.85 × 0.35 / 0.35 = **0.85** ✓
- 单 `EC_Action + LLM(0.4)` 会话：CC = 0.4 × 0.35 / 0.35 = **0.4** ✓
- 多类混合：归一化保证 CC ∈ [0, 1]

### Reason 透传链路（Codex Info #2 修复 — RenderArgs struct param）

```
D7 Verify(contract, turnOut)
       │
       ├─ verdict.Reason ∈ [DeliverableMissing, EvidenceInsufficient,
       │                   StateChangeFailed, ExperimentInconclusive,
       │                   PermissionDenied, SourceUncertaintyHigh,
       │                   DeliverableTooShort, ExplorationLoopAborted]
       │
       ↓ meta["verify_exit_reason"] = verdict.Reason
       ↓ meta["emission_class"]     = primary_class
       ↓ meta["source_uncertainty"] = verdict.CC
       
D7 buildSessionCompleteEvent (sessionorchestrator/session_complete.go:17-44)
       │
       ↓ 透传 meta 到 EngineEvent
       
D1 EmitComplete (conclusion/conclusion.go:91-154)
       │
       ↓ 透传 meta 到 OutboundMessage.Metadata
       
D1 feishu.OnMessage("complete") (communication/channel/adapters/feishu.go:138-148)
       │
       ↓ 读 meta["verify_exit_reason"] / "emission_class" / "source_uncertainty"
       ↓ 构造 RenderArgs{VerifyExitReason, EmissionClass, SourceUncertainty, ...}
       ↓ 调 finalizeStructuredSession(ctx, RenderArgs)  ← struct param 避免 break PR #373 5-param 签名
       
D1 feishu_progress.go finalizeStructuredSession(RenderArgs)
       │
       ↓ render title: "❌ 任务失败 (ProbeToolChannel: exploration_loop_aborted @ iter 12/15, source_uncertainty=0.62)"
       ↓ render footer: "❌ 任务未完成 (reason: deliverable_missing)"
       
D7 Learn FeedbackMemory (mups/learn/feedback_memory.go)  ← H6/H10 最小接入
       │
       ↓ FeedbackMemory.Record(sessionID, verify_exit_reason, emission_class)
       ↓ 跨 session reputation 起点（完整 drift audit → Phase E）
```

### Learn FeedbackMemory（H6/H10，P1-AC-4）

Phase C **最小 In Scope**：
- 写入：`verify_exit_reason` + `emission_class` + `source_uncertainty`
- 读取：后续 session Filter/Plan 可选 consult（本 change 不强制消费）
- **Out of Scope**：完整 `tool → declared_class → drift_rate` 表（OOS-11 / Phase E）

**T (DSAFT):** D7-S2-A50-T08 (`TestReasonInFeedbackMemory`)

### RenderArgs struct (Codex Info #2 修复)

```go
// /Users/fukai/workspace/devrix/internal/layers/communication/channel/adapters/feishu_progress.go
type RenderArgs struct {
    SessionID         string   // existing
    ChatID            string   // existing
    Summary           string   // existing
    ExitReason        string   // existing
    TaskIncomplete    bool     // existing (PR #373)
    VerifyExitReason  string   // NEW — verifier 层 reason
    EmissionClass     string   // NEW — Probe/Fact/Action/Experiment
    SourceUncertainty string   // NEW — CC 数值
}

func finalizeStructuredSession(ctx context.Context, args RenderArgs) error
```

### Gherkin Scenarios

```gherkin
Feature: VerifyContract 4-Group Strong Validation

  Scenario: Deliverable missing triggers FAIL
    Given VerifyContract with DeliverableRequired=true, MinChars=20
    And TurnOutput has FinalText=""
    When Verify is called
    Then Verdict.Kind equals FAIL
    And Verdict.Reason equals "deliverable_missing"
    And Verdict.CalibratedConfidence equals 0.0

  Scenario: Deliverable too short for review task_kind triggers FAIL (Codex Critical #8 修复)
    Given NewVerifyContract("review", EC_Probe) → MinChars=20
    And TurnOutput has FinalText="LGTM"  # 4 chars
    When Verify is called
    Then Verdict.Kind equals FAIL
    And Verdict.Reason equals "deliverable_too_short"

  Scenario: Deliverable accepted at 20 chars for review task_kind
    Given NewVerifyContract("review", EC_Probe) → MinChars=20
    And TurnOutput has FinalText="Found 3 issues in the codebase"  # 31 chars
    When Verify is called
    Then Verdict.Kind is not FAIL on deliverable check

  Scenario: Evidence insufficient triggers FAIL
    Given VerifyContract with EvidenceRequired=true, MinEvidenceCount=3
    And TurnOutput has 2 tool_calls
    When Verify is called
    Then Verdict.Kind equals FAIL
    And Verdict.Reason equals "evidence_insufficient"

  Scenario: Calibrated confidence too low triggers PARTIAL
    Given NewVerifyContract("review", EC_Probe) → MinSourceQuality=0.5
    And TurnOutput has FinalText="comprehensive review..."
    And calibrated_confidence=0.4
    When Verify is called
    Then Verdict.Kind equals PARTIAL
    And Verdict.Reason equals "source_uncertainty_high"
    And Verdict.CalibratedConfidence equals 0.4

  Scenario: All checks pass returns PASS
    Given NewVerifyContract("review", EC_Probe) → MinChars=20, MinSourceQuality=0.5
    And TurnOutput has FinalText="comprehensive review with multiple findings..."
    And calibrated_confidence=0.6
    When Verify is called
    Then Verdict.Kind equals PASS
    And Verdict.Reason equals null
    And Verdict.CalibratedConfidence equals 0.6

  Scenario: NewVerifyContract zero-value test (Codex Info #4 修复)
    Given VerifyContract{} zero value  # Go default: MinSourceQuality=0.0, DeliverableMinChars=0
    When Verify is called with any non-empty FinalText
    Then Verdict.Kind equals PASS  # 因为 MinSourceQuality=0.0 永远过
    # 注：必须用 NewVerifyContract() 构造器才能保证 safe defaults
    # Go 零值不防呆，构造器模式是 Go 习惯用法

  Scenario: VerifyContract zero-value uses safe defaults via NewVerifyContract
    Given NewVerifyContract("review", EC_Probe)
    When the contract is inspected
    Then MinSourceQuality equals 0.5
    And DeliverableMinChars equals 20
    And MinEvidenceCount equals 1
    And DeliverableRequired is true

  Scenario: calibrated_confidence formula (Codex Critical #2 修复 — 8 子用例)
    Given: weight_EC_Fact=0.50, weight_EC_Action=0.35, weight_EC_Probe=0.20, weight_EC_Experiment=0.10
    And source_uncertainty values for each class
    When: CC is computed
    Then: 
      # Case 1: single EC_Fact + User(0.85)
      CC = 0.85 * 0.50 / 0.50 = 0.85
      # Case 2: single EC_Action + User(0.85)
      CC = 0.85 * 0.35 / 0.35 = 0.85
      # Case 3: single EC_Probe + LLM(0.4)
      CC = 0.4 * 0.20 / 0.20 = 0.4
      # Case 4: single EC_Experiment + User(0.85)
      CC = 0.85 * 0.10 / 0.10 = 0.85
      # Case 5: EC_Fact+EC_Action mix
      CC = (0.85*0.50 + 0.85*0.35) / (0.50+0.35) = 0.85
      # Case 6: EC_Probe+EC_Fact mix
      CC = (0.4*0.20 + 0.85*0.50) / (0.20+0.50) = (0.08+0.425)/0.70 = 0.72
      # Case 7: all 4 classes mixed
      CC = (0.85*0.10 + 0.4*0.20 + 0.85*0.35 + 0.85*0.50) / 1.15 = (0.085+0.08+0.2975+0.425)/1.15 = 0.772
      # Case 8: empty CC (no tool calls)
      CC = 0.0

  Scenario: verify_exit_reason in metadata at session_complete
    Given a Verdict with Reason="deliverable_missing"
    When buildSessionCompleteEvent emits EngineEvent
    Then meta["verify_exit_reason"] equals "deliverable_missing"

  Scenario: verify_exit_reason visible in feishu render via RenderArgs struct
    Given OutboundMessage with Metadata["verify_exit_reason"]="deliverable_missing"
    When feishu.OnMessage(complete) is called
    Then finalizeStructuredSession receives RenderArgs.VerifyExitReason="deliverable_missing"
    And title contains "(reason: deliverable_missing)"
```

---

## TruncateMarker（D2 截断透明）

D2 在 tool_result 截断时 MUST 附加 TruncateMarker text，让 LLM 看到 complete=false 信号。

```go
// /Users/fukai/workspace/devrix/internal/layers/contextengine/compression/truncate_marker.go
func TruncateWithMarker(text string, maxChars int, marker string) (string, bool)
```

### Gherkin

```gherkin
Feature: TruncateMarker Appended When Result Exceeds MaxChars

  Scenario: Result below MaxChars not truncated
    Given text length < MaxChars
    When TruncateWithMarker is called
    Then return value equals input
    And truncated flag equals false

  Scenario: Result exceeds MaxChars truncated with marker
    Given text length 10K, MaxChars=8K
    When TruncateWithMarker is called
    Then truncated flag equals true
    And returned text contains marker "[TRUNCATED at 8192/10240 chars, complete=false]"

  Scenario: LLM visible marker in tool_result
    Given a read_file result of 50K tokens
    When ToolSpec.MaxResultSizeChars=8192
    Then tool_result.text contains "[TRUNCATED at 8192/50000 chars, complete=false, REREAD may help]"
```

---

## 引用

- 父 spec: `openspec/specs/d7-orchestration/spec.md`（lite-mode 化，本 delta 不修改）
- Proposal: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/proposal.md`
- Design: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/design.md`
- 配套 spec delta: `specs/tool-spec-v3.md` (Phase A) + `specs/execute-channels.md` (Phase B)
- Obsidian: `brain/01知识探索/项目/20260620-certain-architecture/core-concepts/54-tools-metadata-ideal-state-and-channel-autonomy.md`

---

## 更新历史

- 2026-07-01：v1 创建（VerifyContract 4 元组 + Reason 透传）
- 2026-07-01：v1.1 Codex Critical/Warning 修复
  - CC 公式 Σ(su×w)/Σ(w) 归一化 + EC_Fact>EC_Action 权重排序（Critical #2+#6）
  - DeliverableMinChars 按 task_kind 区分 + NewVerifyContract 显式构造器（Critical #8 + Info #4）
  - RenderArgs struct param 避免 break PR #373 5-param 签名（Info #2）
  - 8 个 CC 子用例 + 短 review 边缘 case Gherkin（Warning #8）
