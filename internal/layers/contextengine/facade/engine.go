package facade

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/permission"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/prompt"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/configure/runtime"
	"github.com/devrix/devrix/internal/layers/observability/instrument/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/instrument/tracer"
	"github.com/devrix/devrix/internal/shared/buildinfo"
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

// AppendAndTrimMessages writes a batch of messages into the session context's
// Messages and then trims to the configured token budget. Used by D7 turn
// orchestrator's SessionPersister.PersistTurn to commit a turn's transcript
// back into D2 memory so the next Prepare call can read it back.
//
// DM-20260617-003 (devrix-d7-turn-history-persist): bridges D7 turn bridge to
// D2 Memory. Lazily initializes the session if it does not yet exist (D7 path
// first-write scenario).
func (e *ContextEngine) AppendAndTrimMessages(sessionID string, msgs []types.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	sc, ok := e.memory.Get(sessionID)
	if !ok || sc == nil {
		stub := &types.Session{SessionID: sessionID}
		var initOK bool
		sc, initOK = e.loadOrInitSession(context.Background(), stub, noopEmit)
		if !initOK || sc == nil {
			return fmt.Errorf("append+trim: cannot init session %s", sessionID)
		}
	}
	for i := range msgs {
		e.memory.AppendFullMessage(sc, msgs[i])
	}
	e.memory.TrimMessages(sc)
	return nil
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
func (e *ContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent {
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

	ctx, processSpan := e.startSpan(ctx, telemetry.OpD2_S2_Context_Process, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(message))},
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
		)
		if spSpan != nil {
			spSpan.End()
		}
	}

	sc, ok := e.loadOrInitSession(ctx, session, emit)
	if !ok {
		return
	}
	workerLocal := false
	if ov, ok := contracts.ProcessOverlayFromContext(ctx); ok && ov.IsWorker {
		sc = conversation.ForkWorkerSessionContext(sc, ov)
		workerLocal = true
	}
	permission.InitSessionPermission(sc, e.cfg.Permission)
	if sc.Model == "" && e.defaultModel != "" {
		sc.Model = e.defaultModel
	}
	if sc.ModelTier != "" && e.tierResolver != nil {
		if resolved, err := e.tierResolver.ResolveTier(sc.ModelTier); err == nil && resolved != "" {
			sc.Model = resolved
		}
	}
	if sc.Model == "" && e.tierResolver != nil && e.defaultModel != "" {
		if resolved, err := e.tierResolver.ResolveTier(e.defaultModel); err == nil && resolved != "" {
			sc.Model = resolved
		}
	}

	memoryEntries, ok := e.recallLongTermMemory(ctx, session.SessionID, message, workerLocal, processSpan, emit)
	if !ok {
		return
	}

	requestID := session.RequestID
	if !e.memory.AppendUserMessage(sc, requestID, message) {
		slog.Debug("contextengine: duplicate request skipped", "sessionID", session.SessionID, "requestID", requestID)
	}
	transcriptFrom := len(sc.Messages)

	msgs, ok := e.prepareMessages(ctx, sc, session.SessionID, workerLocal, emit)
	if !ok {
		return
	}

	if !workerLocal {
		omitAgents := e.cfg.UserContext.Mode == "prepend"
		buildInput := prompt.SystemPromptBuildInput{
			WorkDir: session.WorkDir,
			Session: session,
			Runtime: prompt.ProcessRuntimeContext{
				SessionID: session.SessionID,
				RequestID: session.RequestID,
				UserID:    session.UserID,
			},
			AgentsRaw:            agentsRaw,
			MemoryEntries:        memoryEntries,
			OmitAgentsFromSystem: omitAgents,
			RecallMaxTokens:      e.cfg.LongTerm.RecallMaxTokens,
		}
		_, buildSpan := e.startSpan(ctx, telemetry.OpD2_S5_Context_Harness_SystemPrompt_Build, tracer.SpanKindInternal)
		builtPrompt, buildReport := e.assembler.Build(buildInput)
		if buildSpan != nil {
			buildSpan.SetAttributes(
				tracer.Attribute{Key: "system_prompt.total_tokens", Value: fmt.Sprintf("%d", buildReport.TotalTokens)},
				tracer.Attribute{Key: "system_prompt.memory_truncated", Value: fmt.Sprintf("%t", buildReport.MemoryTruncated)},
			)
			buildSpan.SetAttributes(telemetry.GenAIPromptAttrs(
				buildinfo.Version,
				buildReport.TemplateHash,
				buildReport.AgentsMDHash,
			)...)
			buildSpan.End()
		}
		sc.SystemPrompt = builtPrompt
	}

	view := append([]types.Message{}, msgs...)
	if sc.SystemPrompt != "" {
		view = append([]types.Message{{Role: types.MessageRoleSystem, Content: sc.SystemPrompt}}, view...)
	}
	e.memory.SetCompressedView(sc, view)

	working := memory.NewWorkingMemory()
	var pendingComplete *contracts.EngineEvent
	messages := sc.CompressedView
	if len(messages) > 0 && messages[0].Role == types.MessageRoleSystem {
		messages = messages[1:]
	}

	if e.attachReg != nil && !workerLocal {
		payloads := e.attachReg.Collect(ctx, sc, messages, 0)
		messages = append(messages, attachments.Render(payloads)...)
	}
	if e.sessionQueue != nil && !workerLocal {
		mainThread := sc.AgentID == ""
		drained := e.sessionQueue.Drain(sc.SessionID, sc.AgentID, mainThread)
		messages = append(messages, contracts.RenderQueueNotifications(sc.SessionID, drained)...)
	}

	tools, _ := e.toolsReg.ListTools(ctx, sc.WorkDir)
	tools = enforce.FilterToolsByPermissionMode(sc.PermissionMode, tools, sc.PlanFilePath)
	if e.agentRoleToolFilter != nil {
		tools = e.agentRoleToolFilter.Filter(sc, tools)
	}

	if e.preparedTurnRunner == nil {
		emit(mapProcessError(session.SessionID, fmt.Errorf("contextengine: PreparedTurnRunner not wired (use D7 InitOrchestration)")))
		return
	}

	toolSchemas := make([]contracts.ToolSchema, len(tools))
	for i, t := range tools {
		toolSchemas[i] = contracts.ToolSchema{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
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

	e.finalizeTurn(ctx, session, sc, loopResult, runErr, working, message, workerLocal, transcriptFrom, pendingComplete, ch, emit, processSpan, start)
}
