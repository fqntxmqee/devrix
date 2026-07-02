# Tasks: MUPS 5 节点 × Tool 元数据 Control Plane + ToolChannel 自治

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Total T Points:** 33 (Phase A 8 + Phase B-pre 1 + Phase B 12 + Phase C 7 + Phase D 5)

> **T 点 DSAFT 格式**：每个 T 点用 `D{X}-S{X}-A{XX}-T{XX}` 编号（master.md §4.3 + dsaft-methodology.md §四）。25 个 T 点全部注册到对应域 t-registry.md。
>
> **估时**：参考值 — Phase A 2d / Phase B 3d / Phase C 2d / Phase D 2d（proposal.md 禁含估时，tasks.md 允许）
>
> **v1.2 Codex Re-Review 关键重映射**（避 retired S + 占用 T slot）：
> - D2-S1 RETIRED → Phase A 改用 **D2-S15-A02-T06..T12**（D2-S15-A02-T01 已被 RepairToolChain 占用，T02..T05 给 Phase D Filter 用）
> - D2-S2 RETIRED → TruncateWithMarker 改用 **D2-S15-A02-T13**
> - D7-S9-A26 T01..T05 已被现有 4 PlanKind Channel 占用 → Phase B ToolChannel 改用 **D7-S9-A50-T01..T06**（新 A，.openspec.yaml 已列）
> - D5-S3 RETIRED (legacy Logger S) → LTL-Lite 改用 **D5-S25-A01-T01..A03-T01**（新 S）
> - D7-S2-A50-T01..T06 已被 turn→sessionorchestrator merge + API error class 占用 → session_complete meta 透传改用 **D7-S2-A50-T07**

---

## Phase A: ToolSpec v3 Schema（8 T 点）⭐ 治本前置 — INCLUDE 19 工具默认 metadata

### D2-S15-A02-T06: ToolSpec v3 struct EXTEND（6 字段在末尾）

- **文件：** `internal/shared/contracts/tool_surface.go` EXTEND（line 152 末尾）
- **范围：** ToolSpec struct 加 6 新字段在末尾：`EmissionClass` / `ConvergenceContract` / `IterationBound` / `SourceUncertainty` / `MaxResultSizeChars` / `TruncateMarkerText`
- **关键约束**：6 字段位置在 struct 末尾，避免 position struct literal break（D2-S1 已 RETIRED per d2-domain.md v9.0.0, 改用 D2-S15-A02-T06, 跳过 T01 (RepairToolChain) + T02..T05 (Phase D Filter)）
- **JSON tag 一致性**：6 新字段 + 9 老字段全部加 json tag (snake_case)
- **依赖：** —
- **CI：** `go build ./...` + `go vet ./...` + struct literal 兼容性 grep gate PASS

### D2-S15-A02-T07: 4 个新 type 定义

- **文件：** `internal/shared/contracts/tool_surface.go` EXTEND（同文件）
- **范围：** `EmissionClass` enum (Fact/Action/Probe/Experiment) + `ConvergenceContract` struct + `IterationBound` struct + `SourceUncertainty` struct
- **依赖：** D2-S15-A02-T06
- **CI：** `go test -race ./shared/contracts/...`

### D2-S15-A02-T08: 19 工具 orthogonal_flags 默认 metadata ⭐ 治本前置

- **文件：** `internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go` (extend 44-142)
- **范围：** 19 工具全部填 6 新字段（**PR-A 治本叙事不能留 Phase E**）：
  - read_file/grep/glob = **Probe** + **None** + **Bounded(15)** + Deterministic(1.0) + MaxResultSizeChars=8192 + TruncateMarker（H12 共识 — 三工具显式 Probe+Bounded(15)，禁止 Fact+OpenEnded）
  - write_file/edit_file/bash = Action + StateChangeRequired + Bounded(8-10) + User(0.85) / Deterministic(1.0)
  - query_diagnostics/tool_search = Fact + None + OpenEnded + Deterministic(1.0)
  - **lsp_* 拆分**：lsp_goto_definition/lsp_hover/lsp_references = **Fact**（deterministic read-only）；lsp_workspace_symbol/lsp_code_action = **Probe**（探索性）
  - free_fork = Experiment + Quotient(0.8) + Quotient(0.8) + User(0.85)
  - delegate_* = Probe + EvidenceRequired(min=1) + Bounded(3) + LLM(0.4)
- **CI：** `grep -L "EmissionClass:" internal/layers/contextengine/enforce/tools/surface/*.go` = empty

### D2-S15-A02-T09: BuiltinSurface spec 重标（6 工具）

- **文件：** `internal/layers/contextengine/enforce/tools/surface/builtin_surface.go` (extend 35-50)
- **范围：** bash=Action/write=Action/edit=Action/read=**Probe**/grep=**Probe**/glob=**Probe** spec 全部包含 6 新字段（P0-AC-9）
- **依赖：** D2-S15-A02-T08
- **CI：** `go test -race ./...`

### D2-S15-A02-T10: LSPToolSurface spec 重标（5 LSP 工具 EC_Fact/Probe 拆分）

- **文件：** `internal/layers/contextengine/enforce/tools/surface/lsptool_surface.go`
- **范围：** 5 LSP 工具按 T08 拆分（goto_definition/hover/references = Fact, workspace_symbol/code_action = Probe）
- **依赖：** D2-S15-A02-T08
- **CI：** `go test -race ./...`

### D2-S15-A02-T11: FreeFork/Tracker/Verify/AskUser/BackgroundTask/ToolSearch 6 surface spec 重标

- **文件：** `internal/layers/contextengine/enforce/tools/surface/{freefork,tracker,verify,askuser,backgroundtask,tool_search}_surface.go`
- **范围：** 11 工具标 6 新字段（free_fork=Experiment, query_diagnostics=Fact, verify_plan_execution=Action, ask_user_question=Action, task_stop/output/list_background=Action, tool_search=Fact）
- **依赖：** D2-S15-A02-T08
- **CI：** `go test -race ./...`

### D2-S15-A02-T12: ToolSpec v3 测试（含 struct literal 兼容性 + JSON tag 一致性）

- **文件：** `internal/shared/contracts/tool_surface_test.go` EXTEND
- **范围：** 测试 15 字段全部存在 + 9 老字段 + 6 新字段默认值合理（EmissionClass=Action, IterationBound=OpenEnded, MaxResultSizeChars=8192 等）
- **结构兼容性 gate**（Codex Critical #9 修复）：
  - `grep -r "ToolSpec{[A-Z]" --include="*.go" .` 必须是 **0** 个结果（position struct literal 禁止）
  - 现有 23 个 `contracts.ToolSpec{...}` 调用全部用 named field 语法（已 grep 验证，0 break 风险）
  - 6 新字段位置在 struct 末尾（位置 literal 兼容性的 Go 习惯约定）
- **JSON tag 一致性**：15 字段全部 snake_case JSON tag 统一
- **依赖：** D2-S15-A02-T06..T11
- **CI：** `go test -race ./shared/contracts/...` + `grep -r "ToolSpec{[A-Z]" --include="*.go" . | wc -l = 0`

### D2-S15-A02-T14: 禁止 silent metadata default CI gate ⭐ P0-AC-10

- **文件：** `internal/layers/contextengine/enforce/tools/surface/surface_metadata_gate_test.go` NEW
- **范围：** 测试/脚本断言所有 `*_surface.go` 必须显式 `EmissionClass`；缺字段则 `go test` FAIL；禁止 runtime fallback `Action+OpenEnded`
- **依赖：** D2-S15-A02-T08..T11
- **CI：** `go test -race -run TestAllSurfacesHaveEmissionClass ./contextengine/enforce/tools/surface/...`

---

## Phase B-pre: PlanChannel Rename（1 T 点）⭐ PR-B 前 P0 门禁

### D7-S9-A26-T06: Channel → PlanChannel rename + type alias

- **文件：** `internal/layers/orchestration/mups/execute/channel.go` RENAME type + callers
- **范围：** `Channel` interface → `PlanChannel`；1-release `type Channel = PlanChannel` alias；更新 bootstrap wire + 现有 4 PlanKind channel 实现
- **依赖：** Phase A 完成
- **CI：** compile + `grep -c 'type Channel interface' mups/execute/` = 0（P0-AC-8）

---

## Phase B: Execute 4 ToolChannel + LTL-Lite（12 T 点）⭐ 治本核心

### D7-S9-A50-T01: ToolChannel interface + Router + Telemetry ⭐ 新 A 避 A26 冲突

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/channel.go` NEW
- **范围：** `ToolChannel` interface + `ToolChannelRegistry` (per EmissionClass) + `ToolChannelRouter.Route(tool ToolSpec)` + metrics（D7-S9-A26 T01..T05 已被现有 4 PlanKind Channel 占用, 改用 D7-S9-A50 新 A — Codex C1 修复）
- **依赖：** Phase A 完成
- **CI：** `go test -race ./mups/execute/toolchannel/...`

### D7-S9-A50-T02: FactToolChannel + ActionToolChannel 实现

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/{fact,action}_channel.go` NEW
- **范围：** 2 ToolChannel 实现 + LTL-Lite invariant `L7-FACT-SAME-Q-5x` (同 query 重复 5x → 强制 synthesize) + `L7-ACTION-POSTSNAPSHOT` (PostSnapshot ≠ PreSnapshot → Verifiable)
- **依赖：** D7-S9-A50-T01
- **CI：** `go test -race ./mups/execute/toolchannel/...`

### D7-S9-A50-T03: ProbeToolChannel 实现（治本核心）

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/probe_channel.go` NEW
- **范围：** 接受 emission_class=Probe + ReadOnly=false → 强 iteration_bound Bounded(n) 校验 + 到 bound 时 InjectSynthesize + **`OnResult` 行为重分类**（call_count>3 同 query → 升级 Probe，H9）
- **依赖：** D7-S9-A50-T01
- **CI：** `go test -race ./mups/execute/toolchannel/...`

### D7-S9-A50-T04: ProbeToolChannel Bounded(15) Hard Stop 测试

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/probe_channel_test.go` NEW
- **范围：** 给 read_file 标 Probe + IterationBound=Bounded(15)，mock 17 次连续 tool_call → 第 16 次 return `SynthesizeNowSignal`，第 17 次拒绝
- **依赖：** D7-S9-A50-T03
- **CI：** `go test -race -run TestProbeToolChannelBounded ./mups/execute/toolchannel/...`

### D7-S9-A50-T05: PromptPressure 注入测试（task_kind override）

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/probe_channel_test.go`
- **范围：** `TestPromptPressureInjectReview` (Bounded(15) @剩5软警告 @剩2硬警告 @16强制) + `TestPromptPressureInjectEdit` (Bounded(10) @剩3/1/11) + `TestPromptPressureObserve` (OpenEnded 不注入)
- **依赖：** D7-S9-A50-T04
- **CI：** `go test -race -run TestPromptPressure ./mups/execute/toolchannel/...`

### D7-S9-A50-T06: ExperimentToolChannel 实现 + L7-EXPERIMENT-CONCLUDED-BEFORE-DEADLINE

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/experiment_channel.go` NEW
- **范围：** experiment_toolchannel 实现 + deadline < ConcludedAt 校验
- **依赖：** D7-S9-A50-T02
- **CI：** `go test -race ./mups/execute/toolchannel/...`

### D7-S9-A50-T07: Shadow mode（would_reject log-only）⭐ H11

- **文件：** `internal/layers/orchestration/mups/execute/toolchannel/router.go` EXTEND
- **范围：** `EnableMupsChannelsEnforce=false` 时 bound 超限只 log `would_reject=true` + metric，不 block；运维 FP<5% 后切 enforce
- **依赖：** D7-S9-A50-T03
- **CI：** `go test -race -run TestShadowModeWouldReject ./mups/execute/toolchannel/...`

### D7-S9-A50-T08: execute-channels.md L0–L3 cross-check 单测 ⭐ H8

- **文件：** `openspec/changes/.../specs/execute-channels.md` EXTEND + `ltl/invariants/termination/crosscheck_test.go` NEW
- **范围：** ≥3 条 L4–L6 vs L0–L3 兼容规则（Bounded 不得 override readonly guard 等）+ 单测
- **依赖：** D5-S25-A01-T01
- **CI：** `go test -race -run TestLTLCrossCheck ./observability/instrument/ltl/...`

### D5-S25-A01-T01: LTL-Lite L4 (Bounded) BoundedInvariant 实现 ⭐ 新 S 避 S3 retired

- **文件：** `internal/layers/observability/instrument/ltl/invariants/termination/bounded.go` NEW
- **范围：** `BoundedInvariant{MaxN, Channel}` + `Check(state) (bool, string)`，iter ≥ MaxN → return false + 注入 synthesize-now 信号（D5-S3 是 legacy Logger scenario, D5 canonical S21-S24+S0, 开 D5-S25 新 S for LTL-Lite termination invariants — Codex C6 修复）
- **3 单元测试**：TestBoundedInvariantOK / TestBoundedInvariantHit / TestBoundedInvariantMessageContainsIterBound
- **依赖：** D7-S9-A50-T03
- **CI：** `go test -race ./observability/instrument/ltl/invariants/termination/...`

### D5-S25-A01-T02: LTL-Lite L4–L6 vs L0–L3 safety cross-check

- **文件：** `internal/layers/observability/instrument/ltl/invariants/termination/crosscheck.go` NEW
- **范围：** `CrossCheckSafety(ltlInvariant, safetyGuard) error` — Bounded 不得 override readonly/destructive guard；Quotient 不得绕过 permission check；Synthesize 不得跳过 audit log（≥3 条规则，与 `execute-channels.md` 对齐）
- **依赖：** D5-S25-A01-T01
- **CI：** `go test -race -run TestLTLCrossCheck ./observability/instrument/ltl/...`

### D5-S25-A02-T01: LTL-Lite L5 (Quotient) QuotientInvariant 实现

- **文件：** `internal/layers/observability/instrument/ltl/invariants/termination/quotient.go` NEW
- **范围：** `QuotientInvariant{Threshold, Metric func(state) float64}` + `Check(state) (bool, string)`
- **3 单元测试**：TestQuotientInvariantAbove / TestQuotientInvariantBelow / TestQuotientCustomMetric
- **依赖：** D5-S25-A01-T01
- **CI：** `go test -race ./observability/instrument/ltl/invariants/termination/...`

### D5-S25-A03-T01: LTL-Lite L6 (Synthesize) SynthesizeInvariant 实现

- **文件：** `internal/layers/observability/instrument/ltl/invariants/termination/synthesize.go` NEW
- **范围：** `SynthesizeInvariant{MinDeliverableChars}` + `Check(state) (bool, string)`，len(text) < MinChars → return false
- **3 单元测试**：TestSynthesizeInvariantOK / TestSynthesizeTooShort / TestSynthesizeExactlyMinChars
- **依赖：** D5-S25-A02-T01
- **CI：** `go test -race ./observability/instrument/ltl/invariants/termination/...`

---

## Phase C: VerifyContract + Reason 透传 + Learn（7 T 点）

### D7-S10-A50-T01: VerifyContract 类型 + NewVerifyContract + 校验逻辑

- **文件：** `internal/layers/orchestration/executionflow/verify/verify_contract.go` + `verify_contract_test.go` NEW
- **范围：** `VerifyContract` struct + `NewVerifyContract(taskKind, expected)` 显式构造器（防 Go 零值陷阱） + `Verify(ctx, contract, turnOut) (*Verdict, error)` + 4 元 contract 校验（deliverable / evidence / source_uncertainty / emission_class）
- **CC 公式**：单类 CC = su，混合类 Σ(su×w)/Σ(w) 归一化（weight: EC_Fact=0.50, EC_Action=0.35, EC_Probe=0.20, EC_Experiment=0.10）
- **MinChars by task_kind**: review=20, edit=10, test=30, observe=10
- **依赖：** Phase A 完成
- **CI：** `go test -race ./executionflow/verify/...` (含 8 CC 子用例)

### D7-S10-A50-T04: VerifyContract burden of proof by EmissionClass ⭐ P1-AC-3

- **文件：** `internal/layers/orchestration/executionflow/verify/verify_contract_test.go` EXTEND
- **范围：** `TestBurdenOfProofByClass` — Fact=text 自证；Action=state change evidence；Probe=source_quality；Experiment=reproducibility
- **依赖：** D7-S10-A50-T01
- **CI：** `go test -race -run TestBurdenOfProofByClass ./executionflow/verify/...`

### D7-S2-A50-T07: session_complete.go 改 meta 透传 5 元 verdict ⭐ 跳 T01..T06

- **文件：** `internal/layers/orchestration/sessionorchestrator/session_complete.go` (改 41-44 行)
- **范围：** 在 `isBothSummaryAndFinalBad` 旁加 `meta["verify_exit_reason"] = verdict.Reason` + `meta["emission_class"]` + `meta["source_uncertainty"] = verdict.CC`（D7-S2-A50-T01..T06 已被 turn→sessionorchestrator/ merge (T01..T04) + API error classification (T05..T06) 占用 — Codex C3 修复）
- **依赖：** D7-S10-A50-T01
- **CI：** `go test -race ./sessionorchestrator/...`

### D7-S2-A50-T08: verify_exit_reason → Learn FeedbackMemory ⭐ H6/H10

- **文件：** `internal/layers/orchestration/mups/learn/feedback_memory.go` EXTEND + `feedback_memory_test.go`
- **范围：** session_complete 调用 `FeedbackMemory.Record(sessionID, verify_exit_reason, emission_class)`；跨 session 可读
- **依赖：** D7-S2-A50-T07
- **CI：** `go test -race -run TestReasonInFeedbackMemory ./mups/learn/...`

### D7-S10-A50-T02: D1 EmitComplete 透传 meta

- **文件：** `internal/layers/communication/channel/conclusion/conclusion.go` (改 154 行)
- **范围：** EmitComplete 透传 `meta["verify_exit_reason"]` 等到 OutboundMessage.Metadata
- **依赖：** D7-S2-A50-T07
- **CI：** `go test -race ./communication/...`

### D7-S10-A50-T03: D1 feishu render reason 标签（RenderArgs struct param）

- **文件：** `internal/layers/communication/channel/adapters/feishu.go` (改 138-148 行) + `internal/layers/communication/channel/adapters/feishu_progress.go` (改 finalizeStructuredSession → RenderArgs struct param)
- **范围：** OnMessage "complete" 读 meta verify_exit_reason + emission_class + source_uncertainty → finalizeStructuredSession 用 RenderArgs struct param（**避免 break PR #373 5-param 签名**）；render title 改 "❌ 任务失败 (ProbeToolChannel: <reason> @ iter X/Y, source_uncertainty=Z)" + footer "❌ 任务未完成 (reason: <verdict_reason>)"
- **依赖：** D7-S10-A50-T02
- **CI：** `go test -race ./communication/...`

### D2-S15-A02-T13: D2 TruncateWithMarker 实现 ⭐ 跳 T01..T12

- **文件：** `internal/layers/contextengine/compression/truncate_marker.go` + `truncate_marker_test.go` NEW
- **范围：** `TruncateWithMarker(text, maxChars int, marker string) (string, bool)` — 截断时必须附加 marker，marker 含 `[TRUNCATED at X/Y, complete=false, REREAD may help]`，可被 LLM 解析；未截断时返回原文本 + false（D2-S2 已 RETIRED per d2-domain.md v9.0.0, D2 canonical S15-S18 — Codex C5 修复）
- **依赖：** Phase A (D2-S15-A02-T08 MaxResultSizeChars 默认值)
- **CI：** `go test -race ./contextengine/compression/...`

---

## Phase D: Filter v2 + Task Kind 路由（5 T 点，三维无 workspace）

### D2-S15-A02-T02: PerEmissionClassFilter ⭐ 跳 T01

- **文件：** `internal/layers/contextengine/enforce/tools/filter/v2/per_emission_class.go` + `per_emission_class_test.go` NEW
- **范围：** `PerEmissionClassFilter` struct + `Apply(specs, ctx)` 方法 + 6 测试（filter by Fact/Action/Probe/Experiment/composite/empty）（D2-S15-A02-T01 已被 RepairToolChain 占用, 改用 T02..T05 — Codex C2 修复）
- **依赖：** Phase A 完成
- **CI：** `go test -race ./contextengine/enforce/tools/filter/v2/...`

### D2-S15-A02-T03: PerTaskKindFilter + task kind → iteration_bound 推

- **文件：** `internal/layers/contextengine/enforce/tools/filter/v2/per_task_kind.go` + `per_task_kind_test.go` NEW
- **范围：** `PerTaskKindFilter` + `taskKindBound(kind string) IterationBound` 4 类映射（review=Bounded(15), edit=Bounded(10), test=Bounded(12), observe=OpenEnded, refactor=Bounded(8)）+ 5 测试
- **依赖：** D2-S15-A02-T02
- **CI：** `go test -race ./contextengine/enforce/tools/filter/v2/...`

### D2-S15-A02-T04: PerAgentFilter v2（emission_class 二级过滤）

- **文件：** `internal/layers/contextengine/enforce/tools/filter/per_agent.go` (改 27-125 行)
- **范围：** 在现有 6 类 agent 的 allowlist 后加 emission_class 二级过滤：
  - `explore` agent → only Fact + Probe (read-only)
  - `worker` agent → Fact + Action + Probe(explore)
  - `delegate` agent → only Probe (delegate_*) + Action (write back)
- **回归测试**：6 测试（每类 agent 1 个）+ read_file (EC_Probe) 对 explore agent 兼容测试
- **依赖：** D2-S15-A02-T03
- **CI：** `go test -race ./contextengine/enforce/tools/filter/...`

### D2-S15-A02-T05: D2 PrepareOrchestrator task_kind 推

- **文件：** `internal/layers/contextengine/prepare/orchestrator.go` (改 111 行)
- **范围：** PrepareOrchestrator.Prepare 加 task_kind 推（从 user_intent 走 IntentClassifier 已有逻辑，详见 DM-20260618 Phase 5 验证集 90%+）
- **依赖：** D2-S15-A02-T04
- **CI：** `go test -race ./contextengine/prepare/...`

### D2-S15-A02-T15: PlanKind × EmissionClass cross-consistency 单测 ⭐ H9/P1-AC-7

- **文件：** `internal/layers/contextengine/enforce/tools/filter/v2/per_task_kind_test.go` EXTEND
- **范围：** `TestPerTaskKindFilterCrossConsistency` — review 时 read_file/grep/glob (Probe) 不得 OpenEnded；Bounded(15) 收紧
- **依赖：** D2-S15-A02-T03
- **T-ID 编号说明：** T06 已被 Phase A ToolSpec v3 struct 占用；本 T 跳过 T06..T14 改用 T15（T13 仍属 Phase C TruncateWithMarker）
- **CI：** `go test -race -run TestPerTaskKindFilterCrossConsistency ./contextengine/enforce/tools/filter/v2/...`

---

## T 点汇总

| Phase | T 点 (DSAFT) | T 点（旧 T-A* 编号） | 范围 | CI |
|-------|--------------|---------------------|------|-----|
| A | D2-S15-A02-T06..T14 | T-A01..T-A08 | ToolSpec v3 + 19 工具 metadata + silent default gate | `go test -race ./...` |
| B-pre | D7-S9-A26-T06 | — | PlanChannel rename | compile + grep gate |
| B | D7-S9-A50-T01..T08 + D5-S25-A01-T01..T02 + A02-T01 + A03-T01 | T-B01..T-B12 | ToolChannel + shadow + LTL + cross-check | `go test -race ./mups/execute/toolchannel/...` |
| C | D7-S10-A50-T01..T04 + D7-S2-A50-T07..T08 + D2-S15-A02-T13 | T-C01..T-C07 | VerifyContract + Reason + Learn + TruncateMarker | `go test -race ./communication/...` |
| D | D2-S15-A02-T02..T05, T15 | T-D01..T-D05 | Filter v2 三维 + cross-consistency | `go test -race ./contextengine/...` |
| **Total** | **33 T** | | **4 PR 联动** | **`go test -race ./...` 全过 + verify-archive.sh** |

---

## AC ↔ T 点对照（demand P0/P1 映射）

| Demand AC | T 点 (DSAFT) | 验证 |
|-----------|--------------|------|
| P0-AC-1/2 | D7-S9-A50-T03..T04 | `TestProbeToolChannelBounded` PASS |
| P0-AC-3 | D2-S15-A02-T13 | `TestMarkerAlwaysAppended` PASS |
| P0-AC-4 | D7-S10-A50-T01 + T04 | `TestDeliverableMissing` + `TestBurdenOfProofByClass` PASS |
| P0-AC-5 | D7-S2-A50-T07 + D7-S10-A50-T02..T03 | `TestRenderVerifyExitReason` PASS |
| P0-AC-6 | D2-S15-A02-T08..T11 | `grep -L "EmissionClass:" surface/*.go` empty |
| P0-AC-7 | D2-S15-A02-T12 | struct literal grep gate PASS |
| P0-AC-8 | D7-S9-A26-T06 | PlanChannel rename compile PASS |
| P0-AC-9 | D2-S15-A02-T08..T09 | read/grep/glob Probe+Bounded(15) PASS |
| P0-AC-10 | D2-S15-A02-T14 | `TestAllSurfacesHaveEmissionClass` PASS |
| P1-AC-1 | design.md §2.4 | Spec review |
| P1-AC-2 | D7-S9-A50-T08 + D5-S25-A01-T02 | execute-channels cross-check ≥3 |
| P1-AC-3 | D7-S10-A50-T04 | `TestBurdenOfProofByClass` PASS |
| P1-AC-4 | D7-S2-A50-T08 | `TestReasonInFeedbackMemory` PASS |
| P1-AC-5 | D7-S9-A50-T07 | shadow metrics + sign-off |
| P1-AC-6 | D7-S9-A50-T05 | PromptPressure baseline 报告 |
| P1-AC-7 | D2-S15-A02-T15 | `TestPerTaskKindFilterCrossConsistency` PASS |
| P1-AC-8 | Phase D 范围 | 无 PerWorkspaceFilter 代码 |

### 旧 AC 编号（proposal §4.1 兼容）

| AC | T 点 (DSAFT) | 验证 |
|----|--------------|------|
| AC1 | D2-S15-A02-T06..T07 | `grep -E "EmissionClass|ConvergenceContract|IterationBound|SourceUncertainty|MaxResultSizeChars|TruncateMarkerText" shared/contracts/tool_surface.go \| awk '/^type ToolSpec struct/,/^}/' \| grep -c ... = 6` |
| AC2 | D2-S15-A02-T08..T11 | `grep -L "EmissionClass:" surface/*.go` empty |
| AC3 | D7-S9-A50-T01..T03, T06 | 4 ToolChannel 单元测试 PASS |
| AC4 | D7-S9-A50-T03..T04 | `TestProbeToolChannelBounded` PASS |
| AC5 | D7-S9-A50-T05 | `TestPromptPressureInject{Review,Edit,Observe}` PASS |
| AC6 | D5-S25-A01-T01 + A02-T01 + A03-T01 | `L4-Bounded, L5-Quotient, L6-Synthesize` 9 invariant 测试 PASS |
| AC7 | D7-S10-A50-T01 | `TestDeliverableMissing` + `TestMinCharsByTaskKind` PASS |
| AC8 | D7-S2-A50-T07 | `TestReasonInMeta` PASS |
| AC9 | D7-S10-A50-T02..T03 | `TestRenderVerifyExitReason` PASS |
| AC10 | D2-S15-A02-T13 | `TestMarkerAlwaysAppended` + `TestShortOutputNoMarker` PASS |
| AC11 | D2-S15-A02-T02 | `TestFilterByEmissionClass` PASS |
| AC12 | D2-S15-A02-T03 | `TestReviewGetsBounded15` + 5 测试 PASS |
| AC13 | D2-S15-A02-T05 | `TestTaskKindInference` PASS |
| AC14 | D7-S10-A50-T01 | `TestCalibratedConfidenceFormula` + 8 子用例 PASS |
| AC15 | D7-S10-A50-T01 | `TestZeroValueUsesSafeDefaults` PASS |

---

## PR 拆解

| PR | Title | 包含 T 点 | 依赖 | 估时 |
|----|-------|-----------|------|------|
| **PR-A** | feat(mups): ToolSpec v3 schema + 19 工具默认 metadata + silent default gate | D2-S15-A02-T06..T14 | — | 2d |
| **PR-B** ⭐ | feat(mups): PlanChannel rename + Execute 4 ToolChannel + shadow + LTL-Lite | D7-S9-A26-T06 + D7-S9-A50-T01..T08 + D5-S25-A01-T01..T02 + A02-T01 + A03-T01 | PR-A | 3.5d |
| **PR-C** | feat(mups): VerifyContract + verdict reason 透传 + Learn FeedbackMemory | D7-S10-A50-T01..T04 + D7-S2-A50-T07..T08 + D2-S15-A02-T13 | PR-B | 2.5d |
| **PR-D** | feat(mups): Filter v2 三维 + task kind 路由 + cross-consistency | D2-S15-A02-T02..T05, T15 | PR-C | 2d |

**总 PR 数：4 联动**

---

## 风险对照 T 点

| Risk | 缓解 T 点 |
|------|-----------|
| ToolSpec v3 breaking change | D2-S15-A02-T12 (struct literal 兼容性单测 + 6 字段位置在末尾) |
| Execute 4 ToolChannel 行为变更 | D7-S9-A50-T07 shadow mode + D7-S9-A26-T06 PlanChannel rename |
| emission_class cheap talk | D7-S2-A50-T08 Learn FeedbackMemory（Phase E 完整 drift defer） |
| Phase B false positive | D7-S9-A50-T07 shadow 1 周 |
| Filter workspace 过载 | OOS-10 defer；Phase D 仅三维 |
| LTL-Lite L4-L6 新概念 | D5-S25-A01-T01 + A02-T01 + A03-T01 (3 invariant 各 3+ 单元测试) |
| PR-B 与 DM-005 重叠 | D7-S10-A50-T01 (VerifyContract 实现 DM-005 协同) |
| 现有 19 工具 metadata 默认值 | **D2-S15-A02-T08 已 INCLUDE**（治本叙事不能留 Phase E 尾巴） |
| feishu render 签名 break | D7-S10-A50-T03 (RenderArgs struct param 避免 break 5-param) |

---

## T 点注册（PR 前置门禁 — S/A 必须先在域 spec.md 注册）

> ⚠️ **Critical 前置门禁**：本 change 引入的 **新 S/A**（D5-S25 / D7-S9-A50 / D7-S10-A50）必须在对应域的 `spec.md` 和 `a-registry.md` **先注册**，再开 PR-B/PR-C 的实现分支。S/A 未注册则对应 PR 的 review 阶段会被 codex 双模型自动判 FAIL。
>
> 流程：
> 1. PR-A 之前：先在 `openspec/specs/d5-observability/spec.md` 加 S25 (LTL-Lite termination) + `a-registry.md` 加 A01/A02/A03
> 2. PR-B 之前：先在 `openspec/specs/d7-orchestration/spec.md` 加 A50 (ToolChannel) + `a-registry.md` 加 A50
> 3. PR-C 之前：先在 `openspec/specs/d7-orchestration/spec.md` 加 A50 (VerifyContract) — 与 PR-B 共用 A50 namespace
> 4. T 点本身在每个 PR 的代码合入时再注册到 `t-registry.md`（DSAFT 规范）

每个 T 点需注册到对应域 t-registry.md：
- D2-S15-A02-T06..T12, T13, T14, T15 (Phase A T06..T12+T14 + Phase C T13 + Phase D T15) → `openspec/specs/d2-context-engine/t-registry.md`
- D5-S25-A01-T01 + A02-T01 + A03-T01 → `openspec/specs/d5-observability/t-registry.md`（**新 S25 需在 d5 spec.md 注册**）
- D7-S2-A50-T07..T08 → `openspec/specs/d7-orchestration/t-registry.md`
- D7-S9-A26-T06 + D7-S9-A50-T01..T08 → `openspec/specs/d7-orchestration/t-registry.md`（**新 A50 需在 d7 spec.md 注册**）
- D7-S10-A50-T01..T03 → `openspec/specs/d7-orchestration/t-registry.md`（**新 A50 复用 namespace，与 PR-B 共用**）

---

## 更新历史

- 2026-07-01：v1 创建 (23 T 点 / 4 PR 联动)
- 2026-07-01：v1.1 Codex Critical 9 项修复 + T 点全 DSAFT 化
  - T-A* → D2-S1-A01-T* (Phase A)
  - T-B* → D7-S9-A26-T* (Phase B) + D5-S3-A03..A05 (LTL-Lite)
  - T-C* → D7-S10-A50-T* + D7-S2-A50-T* + D2-S2-A05-T* (Phase C)
  - T-D* → D2-S15-A02-T* (Phase D)
- 2026-07-01：v1.2 Codex Re-Review 6 Critical 修复（T 编号重映射避 retired S + 占用 slot）
  - Phase A: D2-S1-A01-T01..T07 → **D2-S15-A02-T06..T12**（D2-S1 RETIRED, 跳 T01 RepairToolChain + T02..T05 Phase D）
  - Phase B ToolChannel: D7-S9-A26-T01..T06 → **D7-S9-A50-T01..T06**（A26 已被现有 4 PlanKind Channel 占）
  - Phase B LTL-Lite: D5-S3-A03..A05 → **D5-S25-A01-T01, A02-T01, A03-T01**（D5-S3 RETIRED, 开新 S25）
  - Phase C truncate: D2-S2-A05-T01 → **D2-S15-A02-T13**（D2-S2 RETIRED）
  - Phase C meta透传: D7-S2-A50-T01 → **D7-S2-A50-T07**（T01..T06 已被 turn merge + API error class 占）
  - Phase D Filter: D2-S15-A02-T01..T04 → **D2-S15-A02-T02..T05**（T01 被 RepairToolChain 占）
  - 总数: 23 → **25**（Phase B 因 LTL-Lite 拆 3 T 而非 1 T，+2 体现）
  - read_file ConvergenceContract: Quotient(0.7) → **None**（deterministic read-only, 不用 Quotient）
- 2026-07-01：v1.3 博弈论双 review 共识并入
  - grep/glob → Probe+Bounded(15)（H12）；T14 silent default gate；B-pre PlanChannel rename
  - T07 shadow mode；T08 L0-L3 cross-check；C T04 burden of proof；C T08 Learn FeedbackMemory
  - D T06 cross-consistency；Filter 三维无 workspace；总数 25 → **33**
- 2026-07-01：v1.3.1 S3-Gate Re-Review patch — T-ID 冲突修复（D2-S15-A02-T06 Phase D 改为 T15；tasks.md 7 处同步：§Phase D heading, T registry, AC table P1-AC-7, PR-D row, t-registry note；.openspec.yaml ds aft_activities + t_points；proposal.md AC24）；codex 复审 PASS（critical_count: 0）
- 2026-07-02：v1.4 S4 实现收口 + S5 验收（33/33 T IMPLEMENTED 4 PR 联动）：
  - **PR-A** ToolSpec v3 + 19 工具默认 metadata + silent default gate（commit `74fba9c5` 已合入 master #374）— D2-S15-A02-T06..T12+T14 8 T
  - **PR-B-pre** PlanChannel rename + 1-release alias — D7-S9-A26-T06 1 T
  - **PR-B** Execute 4 ToolChannel + LTL-Lite L4–L6 — D7-S9-A50-T01..T08 + D5-S25-A01-T01/T02 + A02-T01 + A03-T01 12 T
  - **PR-C** VerifyContract + Reason 透传 + Learn ReasonLog + TruncateWithMarker — D7-S10-A50-T01/T04 + D7-S2-A50-T07/T08 + D2-S15-A02-T13 7 T
  - **PR-D** Filter v2 三维 + cross-consistency — D2-S15-A02-T02..T05+T15 5 T
  - **Total 33/33 = 100% IMPLEMENTED**（含 30 P0）；详见 `acceptance-report.md`（verdict: ACCEPTED）
  - supersede 标记：DM-005 (`devrix-d7-verify-synthesize-enforce`) + DM-006 (`devrix-d2-tool-result-budget-for-review`) → `s1_cancelled 2026-07-02` + `replaced_by: devrix-mups-tool-classification-and-channel-autonomy (DM-20260701-007)`
  - 答 Cursor 8K token 问题：需 4 PR 全部合入才治本，PR-A 单 PR 不解决；三件套 (Bounded + TruncateMarker + Filter v2 task_kind 推) 全部就位
