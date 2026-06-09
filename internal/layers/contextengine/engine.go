package contextengine

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// EngineDeps holds dependencies for ContextEngine.
type EngineDeps struct {
	LLM                 ILLMGateway
	TokenCounter        contracts.ITokenCounter
	Tools               IToolRunner
	ToolsReg            IToolRegistry
	Permission          IPermissionGate
	Observer            IObserver
	CompressionObserver ICompressionObserver
	PEVObserver         IPEVObserver
	VerifyRunner        IVerifyCommandRunner
	Planner             contracts.IMilestonePlanner
	LongTerm            memory.ILongTermMemory
	Config              *config.ContextEngineConfig
	ObsBridge           *observability.Bridge
}

// ContextEngine implements gateway.IContextEngine.
type ContextEngine struct {
	memory        *memory.Manager
	counter       contracts.ITokenCounter
	pev           *PEVEngine
	prompt        *prompt.Loader
	cfg           *config.ContextEngineConfig
	observer      IObserver
	compObserver  ICompressionObserver
	obsBridge     *observability.Bridge
	asyncCompact  *compression.AsyncAutocompacter
}

// NewContextEngine creates the Layer 2 context engine.
func NewContextEngine(deps EngineDeps) *ContextEngine {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.DefaultContextEngineConfig()
	}
	observer := deps.Observer
	if observer == nil {
		observer = NoOpObserver{}
	}
	compObserver := deps.CompressionObserver
	if compObserver == nil {
		compObserver = NoOpCompressionObserver{}
	}
	pevObserver := deps.PEVObserver
	if pevObserver == nil {
		pevObserver = NoOpPEVObserver{}
	}
	counter := ResolveTokenCounter(cfg, deps.TokenCounter)
	store := snapshot.NewStore(&cfg.Snapshot)
	var asyncCompact *compression.AsyncAutocompacter
	if cfg.Compression.Autocompact.Enabled {
		asyncCompact = compression.NewAsyncAutocompacter(&AutocompactSummarizer{
			LLM:     deps.LLM,
			Timeout: cfg.Compression.Autocompact.Timeout,
		})
	}
	return &ContextEngine{
		memory:  memory.NewManager(cfg, store, deps.LongTerm),
		counter: counter,
		pev: NewPEVEngine(
			deps.LLM,
			deps.Tools,
			deps.ToolsReg,
			deps.Permission,
			observer,
			&cfg.PEV,
			deps.ObsBridge,
			deps.VerifyRunner,
			pevObserver,
			deps.Planner,
			cfg.Plan,
		),
		prompt:    prompt.NewLoader(&cfg.SystemPrompt),
		cfg:               cfg,
		observer:          observer,
		compObserver:      compObserver,
		obsBridge:         deps.ObsBridge,
		asyncCompact:      asyncCompact,
	}
}

// Shutdown waits for background autocompact work to finish.
func (e *ContextEngine) Shutdown(timeout time.Duration) error {
	if e.asyncCompact == nil {
		return nil
	}
	return e.asyncCompact.Shutdown(timeout)
}

// Process implements gateway.IContextEngine.
func (e *ContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *gateway.EngineEvent {
	ch := make(chan *gateway.EngineEvent, 32)
	go e.runProcess(ctx, session, message, ch)
	return ch
}

func (e *ContextEngine) runProcess(ctx context.Context, session *types.Session, message string, ch chan<- *gateway.EngineEvent) {
	defer close(ch)
	start := time.Now()
	slog.Info("context engine: Process", "sessionID", session.SessionID, "messageLen", len(message))

	emit := func(ev *gateway.EngineEvent) {
		select {
		case <-ctx.Done():
			return
		case ch <- ev:
		}
	}

	// Create "context_engine.process" span as child of gateway span.
	ctx, processSpan := e.startSpan(ctx, telemetry.OpContextProcess, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(message))},
	)
	if processSpan != nil {
		defer processSpan.End()
	}

	systemPrompt := e.prompt.Load(session.WorkDir)

	// Load or init snapshot — with child span.
	loadCtx, loadSpan := e.startSpan(ctx, telemetry.OpContextSnapshotLoad, tracer.SpanKindInternal)
	sc, err := e.memory.LoadOrInit(session, systemPrompt)
	if err != nil {
		emit(infoEvent(session.SessionID, "快照已重置，开始新上下文"))
		session.ContextSnapshot = nil
		sc, err = e.memory.LoadOrInit(session, systemPrompt)
		if err != nil {
			if loadSpan != nil {
				loadSpan.RecordError(err)
				loadSpan.End()
			}
			emit(errorEvent(session.SessionID, errors.NewSnapshotCorruptError(err), false))
			return
		}
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	}
	if loadSpan != nil {
		loadSpan.End()
	}
	_ = loadCtx

	recallCtx, recallSpan := e.startSpan(ctx, telemetry.OpContextLongTermRecall, tracer.SpanKindInternal)
	recallErr := e.memory.EnrichWithLongTermRecall(recallCtx, sc, message)
	if recallSpan != nil {
		if recallErr != nil {
			recallSpan.RecordError(recallErr)
		}
		recallSpan.End()
	}
	if recallErr != nil {
		if processSpan != nil {
			processSpan.RecordError(recallErr)
		}
		var se *errors.SentinelError
		if stderrors.As(recallErr, &se) {
			emit(errorEvent(session.SessionID, se, false))
		} else {
			emit(errorEvent(session.SessionID, errors.NewLongTermDBError(recallErr), false))
		}
		return
	}

	requestID := session.RequestID
	if !e.memory.AppendUserMessage(sc, requestID, message) {
		slog.Debug("contextengine: duplicate request skipped", "sessionID", session.SessionID, "requestID", requestID)
	}

	msgs := sc.Messages
	if e.shouldCompress(msgs, sc.TokenBudget) {
		compCtx, compSpan := e.startSpan(ctx, telemetry.OpContextCompressionRun, tracer.SpanKindInternal,
			tracer.Attribute{Key: "context.tokens_before", Value: fmt.Sprintf("%d", len(msgs))},
		)
		compressed, report, compErr := e.compressionPipeline(session.SessionID).Run(compCtx, msgs, sc.SystemPrompt, sc.TokenBudget)
		if compErr != nil {
			if compSpan != nil {
				compSpan.RecordError(compErr)
				compSpan.End()
			}
			if se, ok := compErr.(*errors.SentinelError); ok {
				emit(errorEvent(session.SessionID, se, false))
			}
			return
		}
		if compSpan != nil {
			compSpan.SetAttributes(
				tracer.Attribute{Key: "context.tokens_after", Value: fmt.Sprintf("%d", report.CompressedTokens)},
			)
			compSpan.End()
		}
		e.observer.EmitContextCompressed(report)
		e.memory.SetCompressedView(sc, compressed)
		emit(infoEvent(session.SessionID, fmt.Sprintf("上下文已压缩 (%d→%d tokens)", report.OriginalTokens, report.CompressedTokens)))
	} else {
		view := append([]types.Message{}, msgs...)
		if sc.SystemPrompt != "" {
			view = append([]types.Message{{Role: types.MessageRoleSystem, Content: sc.SystemPrompt}}, view...)
		}
		e.memory.SetCompressedView(sc, view)
	}

	working := memory.NewWorkingMemory()
	result, runErr := e.pev.Run(ctx, sc, sc.CompressedView, message, func(ev *gateway.EngineEvent) {
		if ev.Type == "text" && ev.Metadata["is_complete"] == "false" {
			working.AppendStream(ev.Content)
		}
		emit(ev)
	})

	var assistantSummary string
	if runErr == nil {
		// 方案 2: 同步工具调用历史到 sc.Messages
		// 这样下一轮对话时，LLM 能感知到之前的工具调用和结果
		if result != nil && len(result.ToolCallHistory) > 0 {
			for _, tc := range result.ToolCallHistory {
				// 构建工具调用的 assistant 消息（包含 tool_calls）
				tcJSON, _ := json.Marshal([]struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				}{{
					ID:       "call_" + tc.ToolName,
					Type:     "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tc.ToolName, Arguments: tc.Input},
				}})
				tcMsg := types.Message{
					Role:      types.MessageRoleAssistant,
					Content:   "",
					Metadata:  map[string]string{"tool_calls": string(tcJSON)},
				}
				e.memory.AppendMessage(sc, types.MessageRoleAssistant, tcMsg.Content)
				// 直接追加到 sc.Messages（通过 Metadata 保存 tool_calls）
				sc.Messages = append(sc.Messages, tcMsg)
				
				// 构建工具结果的 tool 消息
				resultContent := tc.Output
				if tc.Error != "" {
					resultContent = "Error: " + tc.Error
				}
				resultMsg := types.Message{
					Role:     types.MessageRoleTool,
					Content:  resultContent,
					Metadata: map[string]string{"tool_call_id": "call_" + tc.ToolName},
				}
				e.memory.AppendMessage(sc, types.MessageRoleTool, resultContent)
				// 直接追加到 sc.Messages
				sc.Messages = append(sc.Messages, resultMsg)
			}
		}
		
		if text := working.FlushStream(); text != "" {
			e.memory.AppendMessage(sc, types.MessageRoleAssistant, text)
			assistantSummary = text
		}
		if assistantSummary == "" {
			assistantSummary = lastAssistantContent(sc.Messages)
		}
		storeCtx, storeSpan := e.startSpan(ctx, telemetry.OpContextLongTermStore, tracer.SpanKindInternal)
		storeErr := e.memory.AutoStoreLongTerm(storeCtx, sc, message, assistantSummary)
		if storeSpan != nil {
			if storeErr != nil {
				storeSpan.RecordError(storeErr)
			}
			storeSpan.End()
		}
		if storeErr != nil {
			slog.Warn("contextengine: longterm auto_store failed", "error", storeErr)
		}
	} else {
		if processSpan != nil {
			processSpan.RecordError(runErr)
			processSpan.SetStatus(tracer.StatusCodeError, runErr.Error())
		}
		emit(mapProcessError(session.SessionID, runErr))
	}

	if data, persistErr := e.memory.PersistSnapshot(sc); persistErr == nil {
		session.ContextSnapshot = data
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	} else {
		slog.Warn("contextengine: persist snapshot failed", "error", persistErr)
	}

	slog.Debug("contextengine: process done", "sessionID", session.SessionID, "duration", time.Since(start))
}

// startSpan creates a child span if observability is configured.
func (e *ContextEngine) startSpan(ctx context.Context, operation string, kind tracer.SpanKind, attrs ...tracer.Attribute) (context.Context, tracer.Span) {
	if e.obsBridge == nil || e.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(kind),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(operation, attrs...)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return e.obsBridge.Tracer().Start(ctx, operation, opts...)
}

func (e *ContextEngine) shouldCompress(msgs []types.Message, budget types.TokenBudget) bool {
	return e.compressionPipeline("").ShouldCompress(msgs, budget)
}

func (e *ContextEngine) compressionPipeline(sessionID string) *compression.Pipeline {
	opts := []compression.Option{
		compression.WithEnabled(e.cfg.CompressionEnabled),
		compression.WithCounter(e.counter),
		compression.WithAutocompactConfig(e.cfg.Compression.Autocompact),
		compression.WithSummarizer(&AutocompactSummarizer{
			LLM:     e.pev.llm,
			Timeout: e.cfg.Compression.Autocompact.Timeout,
		}),
	}
	if sessionID != "" {
		opts = append(opts,
			compression.WithStepObserver(newPipelineStepObserver(sessionID, e.compObserver)),
			compression.WithSessionID(sessionID),
		)
	}
	if e.asyncCompact != nil {
		opts = append(opts, compression.WithAsyncAutocompacter(e.asyncCompact))
	}
	return compression.NewPipeline(opts...)
}

func infoEvent(sessionID, content string) *gateway.EngineEvent {
	return &gateway.EngineEvent{
		Type:      "info",
		Content:   content,
		SessionID: sessionID,
		Metadata:  map[string]string{"category": "context"},
	}
}

func errorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *gateway.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &gateway.EngineEvent{
		Type:      "error",
		Content:   err.Error(),
		SessionID: sessionID,
		Metadata: map[string]string{
			"code":        err.Code,
			"recoverable": rec,
		},
	}
}

func lastAssistantContent(msgs []types.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleAssistant && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return ""
}

func mapProcessError(sessionID string, err error) *gateway.EngineEvent {
	if err == nil {
		return nil
	}
	var se *errors.SentinelError
	if stderrors.As(err, &se) {
		return errorEvent(sessionID, se, false)
	}
	return errorEvent(sessionID, errors.WithCode("CTX_PROCESS_FAILED", err.Error(), err), false)
}
