# Execute 4 ToolChannel Routing (Delta)

**Capability:** d7-orchestration
**Change ID:** devrix-mups-tool-classification-and-channel-autonomy (Phase B ⭐)
**Status:** DRAFT (S3 design)
**Version:** 1.0.0
**Implements Change:** DM-20260701-007 Phase B

本 spec 是 `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/` 目录下的 **独立 spec delta**（lite-mode 兼容：d7-orchestration 已 lite-mode 化，不修改 archive 父 spec）。

专门描述 Execute 节点 4 ToolChannel 路由 + LTL-Lite L4-L6 termination invariant。这是治本核心 — **ProbeToolChannel Bounded(n) + PromptPressure** 是 LLM 自我循环的根治。

**命名澄清**（Codex Critical #3 修复 + H7/H8 共识）：
- 现有 per-PlanKind 抽象 **rename 为 `PlanChannel`**（PR-B 前 P0 门禁，允许 1-release `type Channel = PlanChannel` alias）
- 本 change 引入的新抽象叫 **`ToolChannel`**（per-tool-EmissionClass 终止约束），文件 `internal/layers/orchestration/mups/execute/toolchannel/`
- 现有 PlanKind 执行策略文件 `internal/layers/orchestration/mups/execute/channel.go`，interface 改名为 PlanChannel
- 两套接口命名解耦，平行不冲突；调用顺序：PlanChannel → ToolChannel

---

## D7-EXEC-CH-1: Execute 节点 4 ToolChannel 路由

Execute 节点 MUST 按 ToolSpec.EmissionClass 路由到 4 ToolChannel，每个 ToolChannel 在 register time 强制挂 LTL-Lite L4-L6 termination invariant。

**T (DSAFT):** D7-S9-A50-T01..T06 + D5-S25-A01-T01 + A02-T01 + A03-T01

### ToolChannel 接口

```go
// /internal/layers/orchestration/mups/execute/toolchannel/channel.go
type ToolChannel interface {
    Name() string
    EmissionClass() EmissionClass
    Route(tool ToolSpec) bool
    Accept(ctx context.Context, call *ToolCall) error
    OnResult(ctx context.Context, result *ToolResult) error
    InjectPromptPressure(ctx context.Context, remaining int) error
    Invariants() []ltl.Invariant
    Finalize(ctx context.Context) (*ChannelOutcome, error)
}

type ChannelOutcome struct {
    DeliverableText string
    Evidence         []string  // tool_call IDs
    PrimaryClass     EmissionClass
    IterationsUsed   int
    BoundMax         int
}
```

### 4 ToolChannel 路由规则

| ToolChannel | EmissionClass | 路由判定 | LTL-Lite Invariant | PromptPressure (task_kind override) |
|-------------|---------------|----------|---------------------|-------------------------------------|
| **FactToolChannel** | EC_Fact | `tool.EmissionClass == EC_Fact` | `L7-FACT-SAME-Q-5x` (同 query 重复 5 次强切 synthesize) | 剩 2 次 |
| **ActionToolChannel** | EC_Action | `tool.EmissionClass == EC_Action` | `L7-ACTION-POSTSNAPSHOT` (PostSnapshot ≠ PreSnapshot → Verifiable) | 剩 2 次 |
| **ProbeToolChannel** ⚡ | EC_Probe | `tool.EmissionClass == EC_Probe` | `L4-BOUNDED-ITERATIONS` (iter ≥ B(n) → InjectSynthesize) | **review Bounded(15): 软警告@剩5/硬警告@剩2/强制@16; edit Bounded(10): 软@3/硬@1/强制@11; test Bounded(12): 软@4/硬@1/强制@13; observe OpenEnded: 不注入** |
| **ExperimentToolChannel** | EC_Experiment | `tool.EmissionClass == EC_Experiment` | `L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE` (ConcludedAt < Deadline) | 剩 2 次 |

### ProbeToolChannel 治本行为（核心）

ProbeToolChannel MUST 在 `iter >= tool.IterationBound.MaxN` 时：
1. 返回 `ErrProbeToolChannelBoundExceeded` 给 tool_call
2. 注入 system message 到 LLM context：`"⚠️ Iteration bound (n={max_n}) reached. You MUST synthesize now based on existing tool results. Emit final text, do NOT call any more tool."`
3. LLM 收到后只能 emit text，不能再调 tool_call

### PromptPressure 三档时序（Codex Critical #7 修复）

**旧方案（被否决）**：仅 1 档 `remaining=3`（80% used，LLM 已无可挽回）

**新方案**：软警告 + 硬警告 + 强制三档曲线，按 task_kind override：

| task_kind | Bounded(n) | 软警告 | 硬警告 | 强制 synthesize | 注入文本 |
|-----------|-----------|--------|--------|------------------|----------|
| review | 15 | @剩 5 (67%) | @剩 2 (87%) | @iter 16 | `"⚠️ tool calls remaining: {n}/15. Begin synthesizing review."` → 硬警告 `"⚠️ FINAL: {n}/15 remaining. Synthesize NOW."` |
| edit | 10 | @剩 3 (70%) | @剩 1 (90%) | @iter 11 | 同样模板，n=10 |
| test | 12 | @剩 4 (67%) | @剩 1 (92%) | @iter 13 | 同样模板，n=12 |
| observe | OpenEnded | — | — | — | （不注入）|

### LTL-Lite L4 (Bounded) Invariant

```go
// /internal/layers/observability/instrument/ltl/invariants/termination/bounded.go
type BoundedInvariant struct {
    MaxN     int
    Channel  string
}

func (b *BoundedInvariant) Check(state *ChannelState) (bool, string) {
    if state.IterationsUsed >= b.MaxN {
        return false, fmt.Sprintf("iter=%d >= bound=%d", state.IterationsUsed, b.MaxN)
    }
    return true, ""
}
```

### LTL-Lite L5 (Quotient) Invariant

```go
// /internal/layers/observability/instrument/ltl/invariants/termination/quotient.go
type QuotientInvariant struct {
    Threshold float64
    Metric    func(state *ChannelState) float64
}
```

### LTL-Lite L6 (Synthesize) Invariant

```go
// /internal/layers/observability/instrument/ltl/invariants/termination/synthesize.go
type SynthesizeInvariant struct {
    MinDeliverableChars int
}
```

### LTL-Lite 命名澄清（Codex Warning #6 修复）

**L4-L6 是 termination 不变量大类**（umbrella），每个不变量有名字：

| L 编号 | 大类名 | 具体 invariant | 适用 Channel |
|--------|--------|----------------|--------------|
| L4 | Termination/Bounded | `L4-BOUNDED-ITERATIONS` | ProbeToolChannel (Bounded(n)) |
| L5 | Termination/Quotient | `L5-QUOTIENT-THRESHOLD` | ExperimentToolChannel (Quotient) |
| L6 | Termination/Synthesize | `L6-SYNTHESIZE-MIN-CHARS` | ProbeToolChannel (synthesize 阶段) |
| L7 | Termination/FactCache | `L7-FACT-SAME-Q-5x` | FactToolChannel |
| L7 | Termination/ActionVerify | `L7-ACTION-POSTSNAPSHOT` | ActionToolChannel |
| L7 | Termination/ExperimentDeadline | `L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE` | ExperimentToolChannel |

L7 是 Termination 大类下其它 3 个不变量（避免 L4/L5/L6 已占用 3 个 slot 后，新不变量仍要 sequential 编号）。

### Gherkin Scenarios

```gherkin
Feature: Execute Node 4 ToolChannel Routing

  Scenario: read_file routes to ProbeToolChannel
    Given a read_file tool call with EmissionClass=Probe
    When the ToolChannelRouter routes it
    Then it is dispatched to ProbeToolChannel
    And ProbeToolChannel's invocations count starts at 1

  Scenario: bash routes to ActionToolChannel
    Given a bash tool call with EmissionClass=Action
    When the ToolChannelRouter routes it
    Then it is dispatched to ActionToolChannel
    And ActionToolChannel's PostSnapshot is captured

  Scenario: grep routes to ProbeToolChannel (H12)
    Given a grep tool call with EmissionClass=Probe and IterationBound=Bounded(15)
    When the ToolChannelRouter routes it
    Then it is dispatched to ProbeToolChannel
    And ProbeToolChannel's invocations count starts at 1

  Scenario: free_fork routes to ExperimentToolChannel
    Given a free_fork tool call with EmissionClass=Experiment
    When the ToolChannelRouter routes it
    Then it is dispatched to ExperimentToolChannel
    And ExperimentToolChannel's deadline timer starts

  Scenario: ProbeToolChannel Bounded(15) hard stops at iter 16 (Channel side)
    Given a read_file tool with IterationBound=Bounded(15)
    When ProbeToolChannel receives iter 16 tool_call
    Then it returns ErrProbeToolChannelBoundExceeded
    And injects synthesize-now system message to LLM
    # 注：LLM 实际 emit text 是 integration test (TestProbeToolChannelBoundedIntegration) 验证，
    #    本 Scenario 仅断言 Channel 侧的错误返回 + system message 注入

  Scenario: ProbeToolChannel PromptPressure soft warning at remaining=5 (review)
    Given a read_file tool with IterationBound=Bounded(15)
    And iter=10 has been accepted (review task_kind)
    When ProbeToolChannel receives iter 11 tool_call
    Then soft PromptPressure is injected
    And the injected text contains "⚠️ tool calls remaining: 5/15. Begin synthesizing review."

  Scenario: ProbeToolChannel PromptPressure hard warning at remaining=2 (review)
    Given a read_file tool with IterationBound=Bounded(15)
    And iter=13 has been accepted (review task_kind)
    When ProbeToolChannel receives iter 14 tool_call
    Then hard PromptPressure is injected
    And the injected text contains "⚠️ FINAL: 2/15 remaining. Synthesize NOW."

  Scenario: ProbeToolChannel PromptPressure edit task_kind override
    Given a read_file tool with IterationBound=Bounded(10)
    And iter=7 has been accepted (edit task_kind)
    When ProbeToolChannel receives iter 8 tool_call
    Then soft PromptPressure is injected (n=3/10)
    And the injected text contains "Begin synthesizing edit."

  Scenario: ProbeToolChannel observe task_kind does NOT inject
    Given a tool with IterationBound=OpenEnded
    And task_kind=observe
    When ProbeToolChannel receives iter 100 tool_call
    Then it accepts without error
    And no PromptPressure is injected

  Scenario: ActionToolChannel PostSnapshot state-change verification
    Given a write_file tool with ConvergenceContract=StateChangeRequired
    When ActionToolChannel receives PreSnapshot
    And the action executes
    Then PostSnapshot is captured
    And PreSnapshot vs PostSnapshot diff is non-empty (state changed)
    And OnCall returns OK

  Scenario: L4 Bounded Invariant detects bound hit
    Given ChannelState with IterationsUsed=15, MaxN=15
    When BoundedInvariant.Check is called
    Then it returns false
    And the message contains "iter=15 >= bound=15"

  Scenario: Integration test — LLM emits text after Synthesize injection (review)
    Given a review task session
    When ProbeToolChannel injects synthesize-now at iter 16
    Then LLM emits text in next turn
    And the text contains structured markers (Summary, Issues)
    # 集成测试，在 tests/integration/probe_channel_synthesize_test.go
```

---

## D7-EXEC-CH-2: LTL-L4–L6 vs L0–L3 Safety Cross-Check（H8，P1-AC-2）

Termination invariants (L4–L6) MUST NOT override safety invariants (L0–L3). At least **3 rules** MUST be documented and tested:

| # | Rule | Rationale |
|---|------|-----------|
| CC-1 | `Bounded(n)` hard reject MUST NOT bypass readonly/destructive permission guards | L0 safety dominates termination |
| CC-2 | `Quotient` threshold MUST NOT skip `CheckPermission` on Action tools | Adverse selection on state change |
| CC-3 | `Synthesize` force MUST still emit audit log span before session end | Observability invariant |

**T (DSAFT):** D5-S25-A01-T02 + D7-S9-A50-T08

---

## D7-EXEC-CH-3: Shadow Mode Rollout（H11，P1-AC-5）

Before hard enforce, ToolChannelRouter MUST support shadow mode:

| Flag | Behavior |
|------|----------|
| `EnableMupsChannels=true`, `EnableMupsChannelsEnforce=false` | Log `would_reject=true`, increment metric, **do not block** |
| `EnableMupsChannelsEnforce=true` | Hard reject @bound |

**Migration gate:** false positive rate < 5% over 1 week before enabling enforce.

**T (DSAFT):** D7-S9-A50-T07

---

## D7-EXEC-CH-4: OnResult Behavior Reclassification（H9）

ProbeToolChannel `OnResult` MUST upgrade repeated calls:
- Same tool + same query fingerprint, `call_count > 3` → treat as Probe pricing (even if declared Fact)
- PerTaskKindFilter MUST NOT downgrade Probe tools to `OpenEnded` when `task_kind=review`

**T (DSAFT):** D7-S9-A50-T03 + D2-S15-A02-T06

---

## 引用

- 父 spec: `openspec/specs/d7-orchestration/spec.md`（lite-mode 化，本 delta 不修改）
- Proposal: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/proposal.md`
- Design: `openspec/changes/devrix-mups-tool-classification-and-channel-autonomy/design.md`
- 配套 spec delta: `specs/tool-spec-v3.md` (Phase A) + `specs/verify-contract.md` (Phase C)
- 现有 mups/execute/ 4 PlanKind Channel: `internal/layers/orchestration/mups/execute/channel.go` (DM-20260625-001-PRC2 已归档)
- Obsidian synthesis: `brain/01知识探索/项目/20260620-certain-architecture/core-concepts/54-tools-metadata-ideal-state-and-channel-autonomy.md`

---

## 更新历史

- 2026-07-01：v1 创建（4 Channel 路由 + LTL-Lite L4-L6）
- 2026-07-01：v1.1 Codex Critical/Warning 修复
  - Channel→ToolChannel 重命名 + 与现有 4 PlanKind Channel 命名解耦（Critical #3）
  - 路径全改：sessionorchestrator/executionflow/execute/ → mups/execute/toolchannel/（Critical #5）
  - PromptPressure 三档时序 + task_kind override（Critical #7）
  - LTL-Lite 命名 L4/L5/L6 + L7 umbrella 澄清（Warning #6）
  - ProbeToolChannel hard stop Gherkin 拆 Channel 侧 + integration 测（Warning #9）
  - 父 spec 引用改为 lite-mode 兼容（Warning #1）
- 2026-07-01：v1.2 博弈论共识（PlanChannel rename、grep→Probe、L0-L3 cross-check、shadow mode、OnResult reclassification）
