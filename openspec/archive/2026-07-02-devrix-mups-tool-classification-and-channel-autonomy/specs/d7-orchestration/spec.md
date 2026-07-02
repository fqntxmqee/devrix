# Delta: D7 Orchestration — ToolChannel Router + VerifyContract + PlanChannel rename

**Change ID:** `devrix-mups-tool-classification-and-channel-autonomy`
**Demand ID:** DM-20260701-007
**Affects:** D7-S9 (Execute Node) + D7-S10 (Verify Node, new section) + D7-S2 (Session Orchestrator)

---

## ADDED

### Requirement: D7-S9-A50 ToolChannel Router — per-EmissionClass termination

Execute 节点 SHALL 在 4 PlanKind Channel (D7-S9-A26) 之上叠加 4 per-EmissionClass ToolChannel (Fact/Action/Probe/Experiment) + Router。

#### Scenario: Router.Route by emission_class
- GIVEN tool.ToolSpec.EmissionClass = Probe + ReadOnly = false
- WHEN Router.Route(tool)
- THEN 路由到 ProbeToolChannel

#### Scenario: ProbeToolChannel Bounded(15) Hard Stop
- GIVEN iteration_bound = 15
- WHEN 第 16 次 call
- THEN 返 SynthesizeNowSignal
- WHEN 第 17 次 call
- THEN 返 ErrProbeToolChannelBoundExceeded (P0-AC-1)

#### Scenario: PromptPressure 3-stage
- GIVEN Bounded(15), task_kind = review
- WHEN @剩 5 iter
- THEN 软警告 (soft inject)
- WHEN @剩 2 iter
- THEN 硬警告 (hard inject)
- WHEN @剩 0 iter
- THEN 强制 synthesize-now (P1-AC-6)

#### Scenario: Shadow mode
- GIVEN Mode = Shadow
- WHEN bound 超限
- THEN 仅 log `would_reject=true` + wouldRejectCount++ metric, 不 block
- AND EnableMupsChannelsEnforce=true 后切 Enforce (P1-AC-5)

#### Scenario: Fact→Probe escalation
- GIVEN Fact tool + 同 query 5x
- WHEN trigger
- THEN 升级 Probe 行为 (L7-FACT-SAME-Q-5x)

### Requirement: D7-S10-A50 VerifyContract + BurdenOfProof + D1 Reason 透传

Verify 节点 SHALL 接受 4 元 input contract + 透传 verdict.Reason 到 D1。

#### Scenario: VerifyContract 4 元 + NewVerifyContract
- GIVEN taskKind = review, expectedEmissionClass = Probe
- WHEN NewVerifyContract(taskKind, expectedEmissionClass)
- THEN 显式构造, 零值陷阱 fail-fast
- AND Verify() 校验 deliverable / evidence / source_uncertainty / emission_class

#### Scenario: CalibratedConfidence 公式
- GIVEN source_uncertainty = [0.8, 0.5, 0.3]
- AND weight (Fact=0.50, Action=0.35, Probe=0.20, Experiment=0.10)
- WHEN compute
- THEN = Σ(su × w) / Σ(w)

#### Scenario: D1 EmitComplete 透传
- GIVEN verdict.Reason = "ProbeToolChannel: bound exceeded @ iter 16/15, source_uncertainty=0.5"
- WHEN EmitComplete
- THEN OutboundMessage.Metadata["verify_exit_reason"] = verdict.Reason (P0-AC-5)

#### Scenario: D1 feishu render reason 标签
- GIVEN RenderArgs struct param (避免 break PR #373 5-param 签名)
- WHEN render
- THEN title "任务失败 (ProbeToolChannel: <reason> @ iter X/Y, source_uncertainty=Z)"
- AND footer "任务未完成 (reason: <verdict_reason>)"

#### Scenario: BurdenOfProofForClass by EmissionClass
- GIVEN EmissionClass = Fact
- WHEN BurdenOfProofForClass
- THEN = text 自证 (P1-AC-3)
- AND Action = state change evidence
- AND Probe = source_quality
- AND Experiment = reproducibility

### Requirement: D7-S9-A26-T06 Channel → PlanChannel rename

`Channel` interface SHALL 重命名为 `PlanChannel`, 1-release alias `type Channel = PlanChannel` 保留 (P0-AC-8 门禁)。

#### Scenario: PlanChannel rename + alias 兼容
- GIVEN `type Channel interface` 已存在
- WHEN rename to `type PlanChannel interface`
- AND add `type Channel = PlanChannel` alias
- THEN 4 PlanKind channel implementations (commit/protocol/scenario/exploration) + 4 callers 全部更新
- AND `grep type Channel interface mups/execute/` = 0 命中

### Requirement: D7-S2-A50-T07/T08 meta 透传 + Learn ReasonLog

`session_complete.go` SHALL 透传 5 元 verdict 到 OutboundMessage.Metadata + Learn ReasonLog 记录跨 session。

#### Scenario: meta 透传
- GIVEN verdict = {Reason, Confidence, EmissionClass, ...}
- WHEN emit complete
- THEN meta["verify_exit_reason"] = verdict.Reason
- AND meta["emission_class"] = verdict.EmissionClass
- AND meta["source_uncertainty"] = verdict.Confidence (P0-AC-5)

#### Scenario: Learn ReasonLog
- GIVEN reason = "ProbeToolChannel: bound exceeded"
- WHEN ReasonLog.Record(sessionID, reason, emissionClass)
- THEN 跨 session 可读 (P1-AC-4)
- AND 8 unit tests 100% PASS: Record / RejectsEmptySessionID / RejectsEmptyReason / FIFOEviction / RecentByTool / DriftRate / DriftRate_Unknown / RecordFromVerdict
