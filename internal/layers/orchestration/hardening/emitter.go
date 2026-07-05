// Package hardening (emitter.go) provides v6.0.0 5 new P0/P1 Span emit helpers for D7 6 S 精简.
// Each function is a thin wrapper around observability.Bridge.Tracer().Start
// that no-ops when the bridge is nil or disabled.
//
// Why in hardening package: avoids each consumer (execute / learn / decisionplanning /
// wavescheduler / verify) re-implementing the same start/end dance. Centralised
// attribute naming keeps the Jaeger UI consistent.
//
// Why no constructor changes: a package-level setter is set once at bootstrap,
// then any caller in any orchestration sub-package can emit a Span without
// threading an obsBridge through every constructor.
package hardening

import (
	"context"
	"sync"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
)

// bridge is the package-level observability bridge. Set via SetBridge at
// bootstrap. Nil means tracing is disabled and all emitters no-op.
var (
	bridgeMu sync.RWMutex
	bridge   *observability.Bridge

	locatorAttrsMu sync.RWMutex
	locatorAttrs   func(context.Context) []tracer.Attribute
)

// SetLocatorAttrsProvider wires semantic locator span attrs from ctx.
// Called from bootstrap to avoid hardening → workmodel import cycle.
func SetLocatorAttrsProvider(fn func(context.Context) []tracer.Attribute) {
	locatorAttrsMu.Lock()
	defer locatorAttrsMu.Unlock()
	locatorAttrs = fn
}

// SetBridge wires the observability bridge for v6.0.0 5 new Span ops.
// Idempotent; safe to call multiple times. Called from
// bootstrap/wire_coordinator.go after WithObservability is applied.
func SetBridge(b *observability.Bridge) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	bridge = b
}

// start begins a Span. Returns (ctx, span) where span may be nil if bridge
// is nil or disabled. Caller MUST defer endSpan to ensure End() fires.
func start(ctx context.Context, operation string, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	bridgeMu.RLock()
	b := bridge
	bridgeMu.RUnlock()
	if b == nil || !b.IsEnabled() {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindInternal),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return b.Tracer().Start(ctx, operation, opts...)
}

// endSpan fires End() if span is non-nil. Mirrors sessionorchestrator.tracing.endSpan.
func endSpan(span tracer.Span) {
	if span != nil {
		span.End()
	}
}

// endSpanWithError fires RecordError + End if span is non-nil and err is non-nil.
func endSpanWithError(span tracer.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

// locatorAttrsFromCtx appends Jaeger breadcrumb attrs when a LocatorFrame
// provider is wired (D7 semantic locator).
func locatorAttrsFromCtx(ctx context.Context) []tracer.Attribute {
	locatorAttrsMu.RLock()
	fn := locatorAttrs
	locatorAttrsMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// --- S6 MUPS Pipeline ---

// EmitMUPSPipeline is the v6.0.0 root span for the 5-node MUPS pipeline
// (DM-20260626-009 hotfix: previously only OrchestratePath emitted this; the
// per-WorkItem ItemPipelineRunner path was missing it, so Jaeger showed
// the 5-node span tree on the legacy route only). Returns the enriched
// context so callers can chain the 5 sub-spans as children.
func EmitMUPSPipeline(ctx context.Context, sessionID, workItemID, pipelineIntent string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "pipeline.work_item_id", Value: workItemID},
		{Key: "pipeline.intent", Value: pipelineIntent},
		{Key: "pipeline.nodes", Value: "observe,plan,wave,execute,verify,learn"},
	}
	attrs = append(attrs, locatorAttrsFromCtx(ctx)...)
	c, span := start(ctx, telemetry.OpD7_S6_MUPS_Pipeline, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitMUPSPhase wraps one MUPS node (observe/plan/execute/verify/learn/decide).
// Returns enriched ctx so D2/D3 children inherit the phase locator breadcrumb.
func EmitMUPSPhase(ctx context.Context, sessionID, workItemID, phase string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "pipeline.work_item_id", Value: workItemID},
		{Key: "pipeline.phase", Value: phase},
	}
	attrs = append(attrs, locatorAttrsFromCtx(ctx)...)
	c, span := start(ctx, telemetry.OpD7_S6_MUPS_Phase, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitChannelRoute wraps ChannelRouter.Route. v6.0.0 S6-A48 P0.
func EmitChannelRoute(ctx context.Context, sessionID, planKind, channelKind, score, fallback string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "channel.kind", Value: channelKind},
		{Key: "plan.kind", Value: planKind},
		{Key: "score", Value: score},
		{Key: "fallback", Value: fallback},
	}
	c, span := start(ctx, telemetry.OpD7_S6_Channel_Route, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitMemoryPersist wraps Memory.Persist. v6.0.0 S6-A49 P0.
func EmitMemoryPersist(ctx context.Context, sessionID, channel, assetKind string, ttlMs int, payloadSize int) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "channel", Value: channel},
		{Key: "asset.kind", Value: assetKind},
		{Key: "ttl_ms", Value: intToString(ttlMs)},
		{Key: "payload_size", Value: intToString(payloadSize)},
	}
	c, span := start(ctx, telemetry.OpD7_S6_Memory_Persist, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S5 DecisionPlanning + Observe ---

// EmitTaskGraphSynthesize wraps DecisionPlanning.SynthesizeTaskGraph. v6.0.0 S5-A33 P1.
func EmitTaskGraphSynthesize(ctx context.Context, sessionID string, nodeCount, edgeCount, dagDepth int, cycleDetected bool) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "taskgraph.node_count", Value: intToString(nodeCount)},
		{Key: "taskgraph.edge_count", Value: intToString(edgeCount)},
		{Key: "taskgraph.dag_depth", Value: intToString(dagDepth)},
		{Key: "taskgraph.cycle_detected", Value: boolToString(cycleDetected)},
	}
	c, span := start(ctx, telemetry.OpD7_S5_TaskGraph_Synthesize, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S4 ExecutionFlow + Verify ---

// EmitSystemAnomalyDetect wraps verify.DetectSystemAnomaly. v6.0.0 S4-A47 P0.
func EmitSystemAnomalyDetect(ctx context.Context, sessionID, anomalyKind, severity, threshold, evidenceID string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "anomaly.kind", Value: anomalyKind},
		{Key: "severity", Value: severity},
		{Key: "threshold", Value: threshold},
		{Key: "evidence_id", Value: evidenceID},
	}
	c, span := start(ctx, telemetry.OpD7_S4_System_Anomaly_Detect, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// EmitResolutionCoverage (DM-20260704-006 Phase 5) emits the
// Verify→Decide handoff span with the CoverageRatio + unresolved
// count metrics. The ItemPipelineRunner calls this once per round
// when the round carries a non-nil ResolutionReport.
//
// total_strategies / total_claims / unresolved_count are integer-typed
// for Grafana panel compatibility; coverage_ratio is float64.
// session_id + work_item_id round_no disambiguate the trace tree.
//
// D7-S4 P0 (Phase 5). 0-behavior-change on legacy rounds (the
// runner checks round.ResolutionReport != nil before calling).
func EmitResolutionCoverage(ctx context.Context, sessionID, workItemID string, roundNo, totalStrategies, totalClaims, unresolvedLen int, coverageRatio float64) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "work_item_id", Value: workItemID},
		{Key: "round_no", Value: intToString(roundNo)},
		{Key: "total_strategies", Value: intToString(totalStrategies)},
		{Key: "total_claims", Value: intToString(totalClaims)},
		{Key: "unresolved_count", Value: intToString(unresolvedLen)},
		{Key: "coverage_ratio", Value: floatToString(coverageRatio)},
	}
	c, span := start(ctx, telemetry.OpD7_S4_Resolution_Coverage, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- S3 WaveScheduler ---

// EmitExecutorSelect wraps WaveScheduler.ExecutorSelector.Select. v6.0.0 S5-A34 P1.
func EmitExecutorSelect(ctx context.Context, sessionID string, candidatesCount int, selectedKind, score, policy string) (context.Context, func(error)) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "candidates_count", Value: intToString(candidatesCount)},
		{Key: "selected_kind", Value: selectedKind},
		{Key: "score", Value: score},
		{Key: "policy", Value: policy},
	}
	c, span := start(ctx, telemetry.OpD7_S3_Executor_Select, attrs...)
	return c, func(err error) { endSpanWithError(span, err) }
}

// --- DM-20260626-009 follow-up inner observability spans ---
//
// The 5-node MUPS spans cover the top-level pipeline; the three below
// cover the inner layers that were invisible in Jaeger until this fix.
// Without them, debugging "why did this WorkItem take 16s?" meant reading
// code instead of inspecting traces. All three follow the same package-
// level bridge pattern as the 5-node emitters (no-op when bridge nil).

// EmitWorktreeOp wraps a single r.Tasks.Tree().Xxx() mutation. v6.0.0 S1
// worktree.op P1. The op attribute (set_round_phase / apply_pipeline_round
// / update_status / list_children) names the mutation; itemID + phase /
// status give the trace enough context to reconstruct the round sequence
// without re-reading the workmodel types.
func EmitWorktreeOp(ctx context.Context, sessionID, op, itemID, phaseOrStatus string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "worktree.op", Value: op},
		{Key: "worktree.item_id", Value: itemID},
		{Key: "worktree.phase_or_status", Value: phaseOrStatus},
	}
	attrs = append(attrs, locatorAttrsFromCtx(ctx)...)
	_, span := start(ctx, telemetry.OpD7_S1_Worktree_Op, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitSubWorktreeRun wraps a child WorkItem run from RunParallelExplore
// or SpawnDecompose. v6.0.0 S1 subworktree.run P2. parentID + childID let
// the trace show the parent → child relationship; spawned_by attribute
// names the caller (parallel_explore / spawn_decompose) so dashboards can
// filter by spawn path.
func EmitSubWorktreeRun(ctx context.Context, sessionID, parentID, childID, spawnedBy string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "subworktree.parent_id", Value: parentID},
		{Key: "subworktree.child_id", Value: childID},
		{Key: "subworktree.spawned_by", Value: spawnedBy},
	}
	_, span := start(ctx, telemetry.OpD7_S1_SubWorktree_Run, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitSubTurnIteration wraps one iteration of the per-WorkItem ReAct loop
// in DefaultWorkItemExecutor.ExecuteWorkItem. v6.0.0 S5 subturn.iteration
// P1. iter is the 1-based iteration number; finishReason captures the LLM
// finish reason ("stop" / "tool_calls" / "length" / ...). The ReAct loop
// emits one span per iteration, all siblings under the surrounding
// EmitMUPSPipeline Execute phase — Jaeger shows the iter-level latency
// distribution directly, which is what was missing when "16s session" had
// no per-iter breakdown.
// EmitContextMaterialize wraps D2 Materialize for a WorkItem partition (DM-20260627-003).
//
// DM-20260630-011 (devrix-session-conclusion-completeness) — messageCount/tokenEst
// are emitted at span start (may be 0 pre-Materialize), then OVERWRITTEN
// at end-of-span with the actual mat.Messages/mat.TokenEstimate values
// via the returned end func. The end func accepts an optional override
// closure so callers can back-fill real numbers without losing the original
// start-time attributes (for OTel span protocol compatibility).
func EmitContextMaterialize(ctx context.Context, sessionID, wiID, policy string, messageCount, tokenEst int) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "materialize.wi_id", Value: wiID},
		{Key: "materialize.policy", Value: policy},
		{Key: "materialize.message_count", Value: intToString(messageCount)},
		{Key: "materialize.token_est", Value: intToString(tokenEst)},
	}
	_, span := start(ctx, telemetry.OpD2_S16_Context_Materialize, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitMaterializeEmptyYield — DM-20260630-011 AC3. Emitted when the
// D2 Materializer returns 0 messages AND 0 token estimate. Indicates
// the WorkItem partition materialized with no content; downstream
// Execute should still proceed but the user-facing summary may not
// reflect real findings.
//
// Emitted as a sibling to EmitContextMaterialize so Jaeger shows the
// empty-yield condition without confusing it with the regular materialize span.
func EmitMaterializeEmptyYield(ctx context.Context, sessionID, wiID, policy string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "materialize.wi_id", Value: wiID},
		{Key: "materialize.policy", Value: policy},
		{Key: "materialize.kind", Value: "empty_yield"},
	}
	_, span := start(ctx, telemetry.OpD2_S16_Context_Materialize_EmptyYield, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitLastTextQualityGate — DM-20260630-011 AC1. Emitted at D7
// finalizeLoop phase. Captures the structural quality classification of
// the LLM's last-turn text (which becomes the IM "任务总结" card
// content). kind ∈ {valid, thin, too_short, inconclusive}; length is
// rune count of the original text; exit_reason is the terminal exit
// reason (natural / max_turns / etc.) for downstream correlation.
func EmitLastTextQualityGate(ctx context.Context, sessionID, kind string, length int, exitReason string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "summary.kind", Value: kind},
		{Key: "summary.length", Value: intToString(length)},
		{Key: "summary.exit_reason", Value: exitReason},
	}
	_, span := start(ctx, telemetry.OpD7_S2_LastText_Quality_Gate, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitEmitCompleteFallback — DM-20260630-011 AC2. Emitted when
// conclusion.EmitComplete falls back from `summary` to `event.Content`
// or to `stats` because the summary was empty/blank or marked
// inconclusive by D7's LastTextQualityGate. Surfaces the fallback
// chain so dashboards can alert on "abnormal fallback rate".
//
// fallback.source ∈ {event.Content, stats, event.Content_redacted};
// content_length is the rune count of the eventual OutboundMessage.Content;
// summary_quality ∈ {valid, thin, too_short, inconclusive} (mirrors
// LastTextQualityGate kind) for easy Jaeger correlation.
func EmitEmitCompleteFallback(ctx context.Context, sessionID, fallbackSource, summaryQuality string, contentLength int) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "fallback.source", Value: fallbackSource},
		{Key: "fallback.content_length", Value: intToString(contentLength)},
		{Key: "summary_quality", Value: summaryQuality},
	}
	_, span := start(ctx, telemetry.OpD1_S16_EmitComplete_Fallback, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

func EmitSubTurnIteration(ctx context.Context, sessionID, itemID string, iter int, finishReason, stopReason string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "subturn.item_id", Value: itemID},
		{Key: "subturn.iter", Value: intToString(iter)},
		{Key: "subturn.finish_reason", Value: finishReason},
		{Key: "subturn.stop_reason", Value: stopReason},
	}
	attrs = append(attrs, locatorAttrsFromCtx(ctx)...)
	_, span := start(ctx, telemetry.OpD7_S5_SubTurn_Iteration, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// --- DM-20260629-001 PR-6 t-span-coverage 5 emit helpers (T38-T42) ---

// EmitResumeDecisionPath wraps the 3 决策路径 in applyResumeSession
// (T38, A fall-through / B user_accept→ForceExit / C user_cancel→
// AbortWithAudit). The span is emitted whenever a decision is reached so
// dashboards can filter traces by route independently of sessionSpan.
func EmitResumeDecisionPath(ctx context.Context, sessionID, userChoice, decision string, auditLevel, depth int) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "resume.user_choice", Value: userChoice},
		{Key: "resume.decision", Value: decision},
		{Key: "resume.audit_level", Value: intToString(auditLevel)},
		{Key: "resume.depth", Value: intToString(depth)},
	}
	_, span := start(ctx, telemetry.OpD7_S2_Resume_Decision_Path, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitAdaptivePriorInject wraps learner.Inject in buildObserveRequest (T41).
// priorKind labels the inject target (Observe / Plan); alpha/beta are the
// Beta prior parameters being injected (clamp [0.001, 1000] upstream so
// downstream Jaeger filters get a stable numeric range).
func EmitAdaptivePriorInject(ctx context.Context, sessionID, priorKind string, alpha, beta float64) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "prior.adaptive_kind", Value: priorKind},
		{Key: "prior.beta_alpha", Value: floatToString(alpha)},
		{Key: "prior.beta_beta", Value: floatToString(beta)},
	}
	_, span := start(ctx, telemetry.OpD7_S5_AdaptivePrior_Inject, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitAnomalyTrigger wraps SystemAnomalyDetector.Trigger (T40).
// kind (CatSystem) and severity (High/Medium/Low) come from the anomaly
// envelope; threshold is the value that tripped the detector; evidenceID
// lets downstream traces correlate back to the Verdict.SourceID that
// produced the anomaly.
func EmitAnomalyTrigger(ctx context.Context, sessionID, kind, severity, threshold, evidenceID string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "anomaly.kind", Value: kind},
		{Key: "anomaly.severity", Value: severity},
		{Key: "anomaly.threshold", Value: threshold},
		{Key: "anomaly.evidence_id", Value: evidenceID},
	}
	_, span := start(ctx, telemetry.OpD7_S4_Anomaly_Trigger, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitLongTermReputationUpdate wraps BayesianUpdate in
// mups/learn/reputation/store.go (T39). Both prior and posterior Beta
// parameters are recorded so the trace shows the actual update delta;
// wilson95 lower/upper provide the 95% Wilson score confidence interval.
// trackMode (developer/operator) is propagated so dashboards can
// separate operator vs developer reputation streams.
func EmitLongTermReputationUpdate(
	ctx context.Context,
	sessionID string,
	priorAlpha, priorBeta, posteriorAlpha, posteriorBeta float64,
	wilsonLower, wilsonUpper float64,
	verifierFailureCount int,
	trackMode string,
) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "reputation.prior_alpha", Value: floatToString(priorAlpha)},
		{Key: "reputation.prior_beta", Value: floatToString(priorBeta)},
		{Key: "reputation.posterior_alpha", Value: floatToString(posteriorAlpha)},
		{Key: "reputation.posterior_beta", Value: floatToString(posteriorBeta)},
		{Key: "reputation.wilson_lower", Value: floatToString(wilsonLower)},
		{Key: "reputation.wilson_upper", Value: floatToString(wilsonUpper)},
		{Key: "reputation.verifier_failure_count", Value: intToString(verifierFailureCount)},
		{Key: "reputation.track_mode", Value: trackMode},
	}
	_, span := start(ctx, telemetry.OpD7_S6_LongTerm_Reputation_Update, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitFeishuCardRender wraps finalizeReplyCardStreaming in
// communication/feishu/progress.go (T42). cardType (initial / update /
// final) and updateMethod (add / update / patch) describe the IM card
// lifecycle; lastVerdict + lastExitReason (from ProcessAutoClose) let
// dashboards correlate the rendered card with the orchestration verdict
// that produced the conclusion text shown in the card.
func EmitFeishuCardRender(
	ctx context.Context,
	sessionID, cardType, updateMethod, lastVerdict, lastExitReason string,
) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "feishu.card_type", Value: cardType},
		{Key: "feishu.update_method", Value: updateMethod},
		{Key: "d7.last_verdict", Value: lastVerdict},
		{Key: "d7.last_exit_reason", Value: lastExitReason},
	}
	_, span := start(ctx, telemetry.OpD7_Feishu_Card_Render, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

func floatToString(f float64) string {
	// 4-decimal precision keeps Beta parameters in a stable numeric
	// range for Jaeger filter ranges without losing meaningful digits.
	if f == 0 {
		return "0"
	}
	neg := f < 0
	if neg {
		f = -f
	}
	intPart := int64(f)
	frac := f - float64(intPart)
	fracPart := int64(frac*10000 + 0.5)
	if fracPart >= 10000 {
		intPart++
		fracPart -= 10000
	}
	s := intToString(int(intPart)) + "." + intToString(int(fracPart))
	if neg {
		s = "-" + s
	}
	return s
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// --- DM-20260629-009 PR-C: 3 inner-layer spans (AC13/14/15) ---
//
// The 3 PR-C spans cover the new CoW VersionChain (AC13), Similarity Check
// gate (AC14), and Hard Evidence gate (AC15). All three follow the same
// package-level bridge pattern (no-op when bridge nil) so calls are safe
// even when the corresponding feature flag is off.

// EmitHardEvidenceReject wraps verify.GateVerdictPass when the gate rejects
// a Pass verdict. kind labels the rejected verdict (code/chat/unknown);
// minimumField identifies which kind-specific minimum failed (coverage /
// log_excerpt / artifact_hash / coherence / entity_hash) so dashboards
// can stratify by reason.
func EmitHardEvidenceReject(ctx context.Context, sessionID, verdictID, kind, minimumField string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "verdict.id", Value: verdictID},
		{Key: "evidence.kind", Value: kind},
		{Key: "evidence.minimum_failed", Value: minimumField},
	}
	_, span := start(ctx, telemetry.OpD7_S18_Hard_Evidence_Reject, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitWorktreeVersionChainAppend wraps workmodel.VersionChainRegistry.Append
// (AC13). reason (commit / rollback / replan / init) and hash let the trace
// reconstruct the chain without re-reading the registry.
func EmitWorktreeVersionChainAppend(ctx context.Context, sessionID, reason, hash string, contentBytes int) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "versionchain.reason", Value: reason},
		{Key: "versionchain.hash", Value: hash},
		{Key: "versionchain.content_bytes", Value: intToString(contentBytes)},
	}
	_, span := start(ctx, telemetry.OpD7_S18_Worktree_VersionChain_Append, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}

// EmitSimilarityCheckIntercept wraps decisionplanning.CheckDecomposeSimilarity
// when Jaccard > InterceptThreshold (AC14). bucket (intercept / warn / pass)
// plus score and matched hash give enough to reproduce the decision.
func EmitSimilarityCheckIntercept(ctx context.Context, sessionID, bucket string, score float64, matchedHash string) func(error) {
	attrs := []tracer.Attribute{
		{Key: "session_id", Value: sessionID},
		{Key: "similarity.bucket", Value: bucket},
		{Key: "similarity.score", Value: floatToString(score)},
		{Key: "similarity.matched_hash", Value: matchedHash},
	}
	_, span := start(ctx, telemetry.OpD7_S18_Similarity_Check_Intercept, attrs...)
	return func(err error) { endSpanWithError(span, err) }
}
