package legacy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/contextengine/persist"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/adapters"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SessionContext returns the cached session context for a session ID (test helper).
func (e *ContextEngine) SessionContext(sessionID string) (*types.SessionContext, bool) {
	return e.memory.Get(sessionID)
}

// ExportSessionSnapshot serializes the in-memory session context for persistence.
func (e *ContextEngine) ExportSessionSnapshot(sessionID string) ([]byte, error) {
	sc, ok := e.memory.Get(sessionID)
	if !ok || sc == nil {
		return nil, fmt.Errorf("session context not found: %s", sessionID)
	}
	return e.memory.PersistSnapshot(sc)
}

// PrepareForTurn is the synchronous Prepare entrypoint used by D7's
// turn.ContextPreparer (bootstrap/turn_adapter.go::contextEngineAdapter).
// Runs the full D2 PrepareOrchestrator (A01 Snapshot_Load / A02 Longterm_Recall
// / A03 Compression_Run / A04 SystemPrompt_Build) so the LLM call site sees
// compressed messages + assembled system prompt, and the D2_Context_Process
// span carries its 4 A-level sub-spans.
//
// Replaces contextEngineAdapter.Prepare's previous direct sc.Messages read,
// which bypassed compression (root cause of untrimmed 58-message LLM input)
// and skipped A01-A04 spans (root cause of the single-span trace).
//
// Returns the prepare.PrepareOutput; the D7 caller (contextEngineAdapter)
// maps Messages / SystemPrompt / Model / MaxContextTokens into its own
// turn.PreparedContext struct.
func (e *ContextEngine) PrepareForTurn(ctx context.Context, session *types.Session, message string) (*prepare.PrepareOutput, error) {
	e.wirePrepareOrchestrator()
	workerLocal := false
	if ov, ok := contracts.ProcessOverlayFromContext(ctx); ok && ov.IsWorker {
		workerLocal = true
	}
	return e.prepareOrchestrator.Prepare(ctx, prepare.PrepareInput{
		Session:         session,
		Message:         message,
		WorkerLocal:     workerLocal,
		CompressPerTurn: e.cfg.TurnRuntime.CompressPerTurn,
	}, e.startSpan)
}

// AppendAndTrimMessages writes a batch of messages into the session context's
// Messages and then trims to the configured token budget. Used by D7 turn
// orchestrator's SessionPersister.PersistTurn to commit a turn's transcript
// back into D2 memory so the next Prepare call can read it back.
//
// DM-20260617-003 (devrix-d7-turn-history-persist): bridges D7 turn bridge to
// D2 Memory. Lazily initializes the session if it does not yet exist (D7 path
// first-write scenario).
//
// P1-f (AC-P1-7): bootstrap path now goes through adapters.SessionLoaderAdapter
// (the orchestrator's session loader port) instead of the legacy
// engine_prepare.go::loadOrInitSession helper. The latter is now dead code
// and removed.
func (e *ContextEngine) AppendAndTrimMessages(sessionID string, msgs []types.Message) error {
	return persist.AppendAndTrimMessages(persist.CommitDeps{
		Store: e.memory,
		Bootstrap: func(sid string) (*types.SessionContext, error) {
			loader := adapters.NewSessionLoaderAdapter(e.memory)
			sc, _, err := loader.LoadOrInit(context.Background(), &types.Session{SessionID: sid}, "")
			if err != nil || sc == nil {
				return nil, fmt.Errorf("cannot init session %s", sid)
			}
			return sc, nil
		},
	}, sessionID, msgs)
}

// noopEmit is used by AppendAndTrimMessages when calling loadOrInitSession
// (which may invoke emit on snapshot corruption). We discard events silently
// because the D7 turn persist path does not own the engine event channel.
func noopEmit(*contracts.EngineEvent) {}

// Shutdown waits for background autocompact work to finish.
func (e *ContextEngine) Shutdown(timeout time.Duration) error {
	if e.asyncCompact == nil {
		return nil
	}
	return e.asyncCompact.Shutdown(timeout)
}

// ToolRunner exposes the engine's IToolRunner for D7 TurnOrchestrator adapters (DM-020 D-c).
func (e *ContextEngine) ToolRunner() IToolRunner { return e.tools }

// PermissionGate exposes the engine's IPermissionGate for D7 TurnOrchestrator adapters (DM-020 D-c).
func (e *ContextEngine) PermissionGate() contracts.IPermissionGate { return e.permission }

// Process implements contracts.IEngine.
//
// Deprecated: P5 legacy retirement. New code MUST go through the D7
// SessionOrchestrator (orchestration/sessionorchestrator) or the turn
// adapter (D7-S2-A06 in bootstrap/turn_adapter.go). This entry point is
// kept only to give the 8 known production callers (cmd/llm-smoke,
// multiagent/run/lifecycle.go, integration tests, etc.) time to migrate
// during the deprecation window. Removal is gated by AC-P5-4 (all
// callers migrated + integration tests green ≥7 days).
//
// Each call emits a slog.Warn so the deprecation is visible in
// production logs without breaking behavior.
func (e *ContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent {
	slog.Warn("contextengine.legacy.Process called (deprecated)",
		"sessionID", session.SessionID,
		"migration", "D7 SessionOrchestrator or turn_adapter.ExecuteRound",
	)
	ch := make(chan *contracts.EngineEvent, 32)
	go e.runProcess(ctx, session, message, ch)
	return ch
}

func (e *ContextEngine) runProcess(ctx context.Context, session *types.Session, message string, ch chan<- *contracts.EngineEvent) {
	defer close(ch)
	start := time.Now()
	slog.Info("context engine: Process", "sessionID", session.SessionID, "messageLen", len(message))

	emit := func(ev *contracts.EngineEvent) {
		select {
		case <-ctx.Done():
			return
		case ch <- ev:
		}
	}

	e.initMetrics()

	// Worker-local is resolved BEFORE the Process span is started so the
	// `session.worker_local` attribute reflects the actual decision that
	// gates snapshot persistence, permission init, and tier resolution.
	workerLocal := false
	if ov, ok := contracts.ProcessOverlayFromContext(ctx); ok && ov.IsWorker {
		workerLocal = true
	}

	ctx, processSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(message))},
		tracer.Attribute{Key: "session.worker_local", Value: boolStr(workerLocal)},
		tracer.Attribute{Key: "session.compress_per_turn", Value: boolStr(e.cfg.TurnRuntime.CompressPerTurn)},
		tracer.Attribute{Key: "session.request_id", Value: session.RequestID},
		tracer.Attribute{Key: "context.caller", Value: "d7"},
		tracer.Attribute{Key: "context.runtime_path", Value: string(obsruntime.PathD7Turn)},
	)
	if processSpan != nil {
		defer processSpan.End()
	}

	obsruntime.Record(obsruntime.PathD7Turn)
	agentsRaw := e.prompt.Load(session.WorkDir)
	{
		_, spSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_SystemPrompt_Load, tracer.SpanKindInternal,
			tracer.Attribute{Key: "agents_raw.length", Value: fmt.Sprintf("%d", len(agentsRaw))},
			tracer.Attribute{Key: "system_prompt.sources_count", Value: fmt.Sprintf("%d", len(e.cfg.SystemPrompt.Sources))},
			tracer.Attribute{Key: "session.work_dir", Value: session.WorkDir},
			tracer.Attribute{Key: "session.request_id", Value: session.RequestID},
		)
		if spSpan != nil {
			spSpan.End()
		}
	}

	// P1-d: scenario orchestrator is the production wired path for
	// A01-A04. Facade retains responsibility for worker fork, permission
	// init, tier resolution (AfterLoad hook) and the final
	// prompt → CompressedView wrap (AfterPrepare hook).
	e.wirePrepareOrchestrator()

	// P1-e: persist orchestrator handles A01-A04 persistence (snapshot,
	// transcript, long-term store, CommitWindow). Wired lazily so unit tests
	// that exercise Process() without going through D7 InitOrchestration
	// still get a working persist path.
	e.wirePersistOrchestrator()

	// workerLocal is resolved earlier (above the Process span) so the span
	// attribute reflects the same value used for snapshot persistence.
	output, err := e.prepareOrchestrator.Prepare(ctx, prepare.PrepareInput{
		Session:         session,
		Message:         message,
		WorkerLocal:     workerLocal,
		CompressPerTurn: e.cfg.TurnRuntime.CompressPerTurn,
	}, e.startSpan)
	if err != nil {
		emit(mapProcessError(session.SessionID, err))
		return
	}

	sc := output.SessionContext
	requestID := session.RequestID
	{
		preCount := len(sc.Messages)
		appended := e.memory.AppendUserMessage(sc, requestID, message)
		_, memSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Memory_Append, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "session.request_id", Value: requestID},
			tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(message))},
			tracer.Attribute{Key: "memory.appended", Value: boolStr(appended)},
			tracer.Attribute{Key: "memory.pre_count", Value: fmt.Sprintf("%d", preCount)},
			tracer.Attribute{Key: "memory.post_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if memSpan != nil {
			memSpan.End()
		}
		if !appended {
			slog.Debug("contextengine: duplicate request skipped", "sessionID", session.SessionID, "requestID", requestID)
		}
	}
	transcriptFrom := len(sc.Messages)

	// Worker fork happens AFTER Prepare (facade retains responsibility for
	// the fork because ForkWorkerSessionContext is a D7-aware concept).
	if workerLocal {
		ov, hasOverlay := contracts.ProcessOverlayFromContext(ctx)
		_, forkSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Worker_Fork, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "worker.is_worker", Value: boolStr(workerLocal)},
			tracer.Attribute{Key: "worker.overlay_present", Value: boolStr(hasOverlay)},
			tracer.Attribute{Key: "worker.overlay_is_worker", Value: boolStr(ov.IsWorker)},
			tracer.Attribute{Key: "worker.agent_id", Value: ov.AgentID},
			tracer.Attribute{Key: "worker.role", Value: ov.WorkerRole},
			tracer.Attribute{Key: "worker.model_tier", Value: ov.ModelTier},
			tracer.Attribute{Key: "session.messages_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		forkDidFork := false
		if hasOverlay && ov.IsWorker {
			sc = conversation.ForkWorkerSessionContext(sc, ov)
			forkDidFork = true
		}
		if forkSpan != nil {
			forkSpan.SetAttributes(
				tracer.Attribute{Key: "worker.forked", Value: boolStr(forkDidFork)},
				tracer.Attribute{Key: "session.messages_count_post", Value: fmt.Sprintf("%d", len(sc.Messages))},
			)
			forkSpan.End()
		}
	}

	if !workerLocal {
		preMode := sc.PermissionMode
		prePlan := sc.PlanFilePath
		permission.InitSessionPermission(sc, e.cfg.Permission)
		_, permSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Permission_Init, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "permission.pre_mode", Value: string(preMode)},
			tracer.Attribute{Key: "permission.post_mode", Value: string(sc.PermissionMode)},
			tracer.Attribute{Key: "permission.default", Value: e.cfg.Permission.DefaultMode},
			tracer.Attribute{Key: "permission.pre_plan_file", Value: prePlan},
			tracer.Attribute{Key: "permission.post_plan_file", Value: sc.PlanFilePath},
			tracer.Attribute{Key: "permission.mode_changed", Value: boolStr(string(preMode) != string(sc.PermissionMode))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if permSpan != nil {
			permSpan.End()
		}
	}
	if sc.Model == "" && e.defaultModel != "" {
		sc.Model = e.defaultModel
	}
	if sc.ModelTier != "" && e.tierResolver != nil {
		preModel := sc.Model
		if resolved, err := e.tierResolver.ResolveTier(sc.ModelTier); err == nil && resolved != "" {
			sc.Model = resolved
		}
		_, tierSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Tier_Resolve, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "tier.source", Value: "sc.ModelTier"},
			tracer.Attribute{Key: "tier.input", Value: sc.ModelTier},
			tracer.Attribute{Key: "tier.pre_model", Value: preModel},
			tracer.Attribute{Key: "tier.post_model", Value: sc.Model},
			tracer.Attribute{Key: "tier.has_resolver", Value: boolStr(e.tierResolver != nil)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if tierSpan != nil {
			tierSpan.End()
		}
	}
	if sc.Model == "" && e.tierResolver != nil && e.defaultModel != "" {
		preModel := sc.Model
		if resolved, err := e.tierResolver.ResolveTier(e.defaultModel); err == nil && resolved != "" {
			sc.Model = resolved
		}
		_, tierSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Tier_Resolve, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "tier.source", Value: "defaultModel"},
			tracer.Attribute{Key: "tier.input", Value: e.defaultModel},
			tracer.Attribute{Key: "tier.pre_model", Value: preModel},
			tracer.Attribute{Key: "tier.post_model", Value: sc.Model},
			tracer.Attribute{Key: "tier.has_resolver", Value: boolStr(e.tierResolver != nil)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if tierSpan != nil {
			tierSpan.End()
		}
	}

	// System prompt is now in output.SystemPrompt (built by orchestrator's
	// AssemblerAdapter). Wrap it into a System-role Message for the LLM view.
	view := append([]types.Message{}, output.Messages...)
	if output.SystemPrompt != "" {
		view = append([]types.Message{{Role: types.MessageRoleSystem, Content: output.SystemPrompt}}, view...)
	}
	{
		_, cvSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_CompressedView_Set, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "view.has_system_prompt", Value: boolStr(output.SystemPrompt != "")},
			tracer.Attribute{Key: "view.system_prompt_len", Value: fmt.Sprintf("%d", len(output.SystemPrompt))},
			tracer.Attribute{Key: "view.messages_count", Value: fmt.Sprintf("%d", len(view))},
			tracer.Attribute{Key: "view.user_messages", Value: fmt.Sprintf("%d", len(output.Messages))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		e.memory.SetCompressedView(sc, view)
		if cvSpan != nil {
			cvSpan.End()
		}
	}
	sc.SystemPrompt = output.SystemPrompt

	working := memory.NewWorkingMemory()
	var pendingComplete *contracts.EngineEvent
	messages := sc.CompressedView
	if len(messages) > 0 && messages[0].Role == types.MessageRoleSystem {
		messages = messages[1:]
	}

	if e.attachReg != nil && !workerLocal {
		inputCount := len(messages)
		payloads := e.attachReg.Collect(ctx, sc, messages, 0)
		rendered := attachments.Render(payloads)
		messages = append(messages, rendered...)
		_, attSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Attachments_Collect, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "attach.work_dir", Value: sc.WorkDir},
			tracer.Attribute{Key: "attach.input_count", Value: fmt.Sprintf("%d", inputCount)},
			tracer.Attribute{Key: "attach.payload_count", Value: fmt.Sprintf("%d", len(payloads))},
			tracer.Attribute{Key: "attach.rendered_count", Value: fmt.Sprintf("%d", len(rendered))},
			tracer.Attribute{Key: "attach.permission_mode", Value: string(sc.PermissionMode)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if attSpan != nil {
			attSpan.End()
		}
	}
	if e.sessionQueue != nil && !workerLocal {
		mainThread := sc.AgentID == ""
		drained := e.sessionQueue.Drain(sc.SessionID, sc.AgentID, mainThread)
		notifications := contracts.RenderQueueNotifications(sc.SessionID, drained)
		messages = append(messages, notifications...)
		_, qSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Queue_Drain, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "queue.agent_id", Value: sc.AgentID},
			tracer.Attribute{Key: "queue.main_thread", Value: boolStr(mainThread)},
			tracer.Attribute{Key: "queue.drained_count", Value: fmt.Sprintf("%d", len(drained))},
			tracer.Attribute{Key: "queue.notification_count", Value: fmt.Sprintf("%d", len(notifications))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if qSpan != nil {
			qSpan.End()
		}
	}

	tools, _ := e.toolsReg.ListTools(ctx, sc.WorkDir)
	{
		_, tSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Tools_List, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "tools.work_dir", Value: sc.WorkDir},
			tracer.Attribute{Key: "tools.count", Value: fmt.Sprintf("%d", len(tools))},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if tSpan != nil {
			tSpan.End()
		}
	}
	{
		preCount := len(tools)
		tools = enforce.FilterToolsByPermissionMode(sc.PermissionMode, tools, sc.PlanFilePath)
		postCount := len(tools)
		_, fpSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Tools_Filter_Permission, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "filter.input_count", Value: fmt.Sprintf("%d", preCount)},
			tracer.Attribute{Key: "filter.output_count", Value: fmt.Sprintf("%d", postCount)},
			tracer.Attribute{Key: "filter.removed_count", Value: fmt.Sprintf("%d", preCount-postCount)},
			tracer.Attribute{Key: "filter.permission_mode", Value: string(sc.PermissionMode)},
			tracer.Attribute{Key: "filter.plan_file_set", Value: boolStr(sc.PlanFilePath != "")},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if fpSpan != nil {
			fpSpan.End()
		}
	}
	if e.agentRoleToolFilter != nil {
		preCount := len(tools)
		tools = e.agentRoleToolFilter.Filter(sc, tools)
		postCount := len(tools)
		_, frSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Tools_Filter_AgentRole, tracer.SpanKindInternal,
			tracer.Attribute{Key: "session.id", Value: session.SessionID},
			tracer.Attribute{Key: "filter.input_count", Value: fmt.Sprintf("%d", preCount)},
			tracer.Attribute{Key: "filter.output_count", Value: fmt.Sprintf("%d", postCount)},
			tracer.Attribute{Key: "filter.removed_count", Value: fmt.Sprintf("%d", preCount-postCount)},
			tracer.Attribute{Key: "filter.agent_id", Value: sc.AgentID},
			tracer.Attribute{Key: "filter.permission_mode", Value: string(sc.PermissionMode)},
			tracer.Attribute{Key: "context.caller", Value: "d7"},
		)
		if frSpan != nil {
			frSpan.End()
		}
	}

	if e.preparedTurnRunner == nil {
		emit(mapProcessError(session.SessionID, fmt.Errorf("contextengine: PreparedTurnRunner not wired (use D7 InitOrchestration)")))
		return
	}

	toolSchemas := make([]contracts.ToolSchema, len(tools))
	loc := e.PromptLocale()
	for i, t := range tools {
		desc, params := i18n.LocalizeTool(t.Name, t.Description, t.Parameters, loc)
		toolSchemas[i] = contracts.ToolSchema{
			Name:        t.Name,
			Description: desc,
			Parameters:  params,
		}
	}

	res, runErr := e.preparedTurnRunner.RunPreparedTurn(ctx, contracts.PreparedTurnRequest{
		SessionID:    session.SessionID,
		SystemPrompt: sc.SystemPrompt,
		Messages:     messages,
		Tools:        toolSchemas,
		MaxTurns:     e.cfg.TurnRuntime.MaxTurns,
		Emit: func(ev *contracts.EngineEvent) {
			if ev.Type == "complete" {
				pendingComplete = ev
				return
			}
			if ev.Type == "text" && ev.Metadata["is_complete"] == "false" {
				working.AppendStream(ev.Content)
			}
			emit(ev)
		},
	})

	var loopResult *preparedTurnLoopResult
	if res != nil {
		loopResult = &preparedTurnLoopResult{
			AssistantText:   res.AssistantText,
			Usage:           res.Usage,
			TurnCount:       res.TurnCount,
			ToolCallHistory: res.ToolCallHistory,
		}
	}

	e.persistTurn(ctx, session, sc, loopResult, runErr, working, message, workerLocal, transcriptFrom, pendingComplete, ch, emit, processSpan, start)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
