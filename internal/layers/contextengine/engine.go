package contextengine

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
	"github.com/devrix/devrix/internal/layers/contextengine/query"
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

	obsruntime.Record(obsruntime.PathQueryLoop)
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

	res, runErr := e.queryLoop.Run(ctx, sc, query.Params{
		SystemPrompt: sc.SystemPrompt,
		Messages:     messages,
		Tools:        query.ToolSchemasFromRunner(tools),
		MaxTurns:     e.cfg.QueryLoop.MaxTurns,
	}, func(ev *contracts.EngineEvent) {
		if ev.Type == "complete" {
			pendingComplete = ev
			return
		}
		if ev.Type == "text" && ev.Metadata["is_complete"] == "false" {
			working.AppendStream(ev.Content)
		}
		emit(ev)
	})

	e.finalizeTurn(ctx, session, sc, res, runErr, working, message, workerLocal, transcriptFrom, pendingComplete, ch, emit, processSpan, start)
}
