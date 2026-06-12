package contextengine

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/attachments"
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/query"
	"github.com/devrix/devrix/internal/layers/contextengine/queue"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/layers/contextengine/usercontext"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	obsruntime "github.com/devrix/devrix/internal/layers/observability/runtime"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/buildinfo"
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
	// DefaultModel 由 L3 LLM 网关解析后的全局默认模型名（来自
	// llm_gateway.default_model）。仅用于在 SessionContext.Model 为空时
	// 回填展示用字段（例：飞书任务完成卡片"模型: xxx"）。LLM 路由仍由
	// 网关自身处理，与该字段无关。
	DefaultModel string
}

// ContextEngine implements contracts.IEngine.
type ContextEngine struct {
	memory       *memory.Manager
	counter      contracts.ITokenCounter
	pev          *PEVEngine
	prompt       *prompt.Loader
	cfg          *config.ContextEngineConfig
	observer     IObserver
	compObserver ICompressionObserver
	obsBridge    *observability.Bridge
	asyncCompact *compression.AsyncAutocompacter
	toolsReg     IToolRegistry
	harnessBoot  *harness.Bootstrap
	assembler    *harness.SystemPromptAssembler
	preflight    *harness.PreflightEvaluator
	router       *harness.PromptRouter
	transcript   *harness.TranscriptManager
	defaultModel string

	metricsOnce      sync.Once
	compressionRatio metrics.Histogram
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
	toolsReg := deps.ToolsReg
	harnessBoot := harness.NewBootstrap(harness.BootstrapDeps{
		Config:    cfg.Harness,
		ToolsReg:  toolRegistryAdapter{reg: toolsReg},
		ObsBridge: deps.ObsBridge,
	})
	ucProvider := usercontext.NewProvider(prompt.NewLoader(&cfg.SystemPrompt), cfg.UserContext)
	attachReg := attachments.NewRegistry(cfg.Attachments)
	pevEngine := NewPEVEngine(
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
	)
	pevEngine.setQueryLoopSupport(QueryLoopSupport{
		Enabled:        cfg.QueryLoop.Enabled,
		MaxTurns:       cfg.QueryLoop.MaxTurns,
		Compress:       cfg.QueryLoop.CompressPerTurn,
		StreamingTools: cfg.QueryLoop.StreamingTools,
		Attachments:    attachReg,
		UserContext:    ucProvider,
		SessionQueue:   queue.GlobalSessionQueue,
		Background:     query.SetGlobalBackgroundRegistry(),
		CompressFn: newCompressFn(
			cfg.QueryLoop.CompressPerTurn,
			cfg,
			counter,
			deps.LLM,
			asyncCompact,
			compObserver,
		),
	})
	return &ContextEngine{
		memory:       memory.NewManager(cfg, store, deps.LongTerm),
		counter:      counter,
		pev:          pevEngine,
		prompt:       prompt.NewLoader(&cfg.SystemPrompt),
		cfg:          cfg,
		observer:     observer,
		compObserver: compObserver,
		obsBridge:    deps.ObsBridge,
		asyncCompact: asyncCompact,
		toolsReg:     toolsReg,
		harnessBoot:  harnessBoot,
		assembler:    harness.NewSystemPromptAssembler(cfg.Workspace),
		preflight:    harness.NewPreflightEvaluator(cfg.Preflight, harness.NewToolPoolFilter(cfg.Harness.ToolPool)),
		router:       harness.NewPromptRouter(cfg.Harness.Routing),
		transcript:   harness.NewTranscriptManager(cfg.Harness.Transcript),
		defaultModel: deps.DefaultModel,
	}
}

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

	// Create "context_engine.process" span as child of gateway span.
	ctx, processSpan := e.startSpan(ctx, telemetry.OpContextProcess, tracer.SpanKindInternal,
		tracer.Attribute{Key: "session.id", Value: session.SessionID},
		tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(message))},
	)
	if processSpan != nil {
		defer processSpan.End()
	}

	// #deprecated: legacy fallback, will be removed in v2.0 (DM-20260611-004).
	// QueryLoop (`cfg.QueryLoop.Enabled`) is now the sole primary LLM↔Tool
	// path. The `harnessEnabled && !workerLocal` branches below are kept as
	// legacy fallback only when `query_loop.enabled: false` is explicitly set.
	harnessEnabled := e.cfg.Harness.Enabled

	// Record the resolved path before any LLM call. The D6
	// PathRegressionProbe uses these counters to assert that the legacy
	// harness path never fires in production (query_loop.enabled=true is
	// the default after DM-20260611-004).
	if e.cfg.QueryLoop.Enabled {
		obsruntime.Record(obsruntime.PathQueryLoop)
	} else {
		obsruntime.Record(obsruntime.PathLegacyHarness)
	}
	basePrompt := e.prompt.Load(session.WorkDir)
	systemPrompt := basePrompt
	// System prompt load observability.
	{
		_, spSpan := e.startSpan(ctx, telemetry.OpContextSystemPromptLoad, tracer.SpanKindInternal,
			tracer.Attribute{Key: "system_prompt.length", Value: fmt.Sprintf("%d", len(systemPrompt))},
			tracer.Attribute{Key: "system_prompt.sources_count", Value: fmt.Sprintf("%d", len(e.cfg.SystemPrompt.Sources))},
			tracer.Attribute{Key: "harness.enabled", Value: fmt.Sprintf("%t", harnessEnabled)},
		)
		if spSpan != nil {
			spSpan.End()
		}
	}

	// Load or init snapshot — with child span.
	_, loadSpan := e.startSpan(ctx, telemetry.OpContextSnapshotLoad, tracer.SpanKindInternal)
	hadSnapshot := session.ContextSnapshot != nil
	sc, err := e.memory.LoadOrInit(session, basePrompt)
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
		loadSpan.SetAttributes(
			tracer.Attribute{Key: "snapshot.message_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
			tracer.Attribute{Key: "snapshot.restored", Value: fmt.Sprintf("%t", hadSnapshot)},
		)
		loadSpan.End()
	}
	workerLocal := false
	if ov, ok := ProcessOverlayFromContext(ctx); ok && ov.IsWorker {
		sc = forkWorkerSessionContext(sc, ov)
		workerLocal = true
	}
	InitSessionPermission(sc, e.cfg.Permission)
	if sc.Model == "" && e.defaultModel != "" {
		sc.Model = e.defaultModel
	}

	// #deprecated: legacy fallback (see harnessEnabled declaration above)
	if harnessEnabled && !session.HarnessInitialized {
		harnessState, bootErr := e.harnessBoot.Run(ctx, session)
		if bootErr != nil {
			emit(errorEvent(session.SessionID, errors.WithCode("CTX_HARNESS_BOOTSTRAP", bootErr.Error(), bootErr), false))
			return
		}
		sc.Harness = harnessState
		session.HarnessInitialized = true
		emit(infoEvent(session.SessionID, fmt.Sprintf("Harness bootstrap 完成 (%d→%d 工具)",
			harnessState.Report.ToolCount, harnessState.Report.VisibleTools)))
	}

	var memoryEntries []memory.MemoryEntry
	recallCtx, recallSpan := e.startSpan(ctx, telemetry.OpContextLongTermRecall, tracer.SpanKindInternal,
		tracer.Attribute{Key: "longterm.enabled", Value: fmt.Sprintf("%t", e.cfg.LongTerm.Enabled)},
		tracer.Attribute{Key: "longterm.recall_topics", Value: strings.Join(e.cfg.LongTerm.Topics, ",")},
	)
	var recallErr error
	// #deprecated: legacy fallback (QueryLoop path uses EnrichWithLongTermRecall unconditionally)
	if harnessEnabled && !workerLocal {
		memoryEntries, recallErr = e.memory.RecallLongTermEntries(recallCtx, message)
	} else {
		recallErr = e.memory.EnrichWithLongTermRecall(recallCtx, sc, message)
	}
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
	compSystemPrompt := sc.SystemPrompt
	// #deprecated: legacy fallback (QueryLoop path keeps system prompt during compression)
	if harnessEnabled && !workerLocal {
		compSystemPrompt = ""
	}
	skipEntryCompress := e.cfg.QueryLoop.Enabled && e.cfg.QueryLoop.CompressPerTurn
	if !skipEntryCompress && e.shouldCompress(msgs, sc.TokenBudget) {
		compCtx, compSpan := e.startSpan(ctx, telemetry.OpContextCompressionRun, tracer.SpanKindInternal,
			tracer.Attribute{Key: "context.tokens_before", Value: fmt.Sprintf("%d", len(msgs))},
		)
		compressed, report, compErr := e.compressionPipeline(session.SessionID).Run(compCtx, msgs, compSystemPrompt, sc.TokenBudget)
		if compErr != nil {
			if compSpan != nil {
				telemetry.RecordSpanError(compSpan, compErr)
				compSpan.End()
			}
			if se, ok := compErr.(*errors.SentinelError); ok {
				emit(errorEvent(session.SessionID, se, false))
			}
			return
		}
		if compSpan != nil {
			ratio := report.Ratio()
			compSpan.SetAttributes(
				tracer.Attribute{Key: "context.tokens_after", Value: fmt.Sprintf("%d", report.CompressedTokens)},
				tracer.Attribute{Key: "context.messages_before", Value: fmt.Sprintf("%d", len(msgs))},
				tracer.Attribute{Key: "context.messages_after", Value: fmt.Sprintf("%d", len(compressed))},
				tracer.Attribute{Key: "context.steps_applied", Value: strings.Join(report.StepsApplied, ",")},
				tracer.Attribute{Key: "compression.trigger_reason", Value: "token_budget_exceeded"},
				tracer.Attribute{Key: "compression.ratio", Value: fmt.Sprintf("%.4f", ratio)},
			)
			compSpan.End()
		}
		if e.compressionRatio != nil {
			e.compressionRatio.Observe(report.Ratio())
		}
		e.observer.EmitContextCompressed(report)
		// #deprecated: legacy harness path skipped SetCompressedView (QueryLoop always sets it below)
		if !harnessEnabled {
			e.memory.SetCompressedView(sc, compressed)
		}
		emit(infoEvent(session.SessionID, fmt.Sprintf("上下文已压缩 (%d→%d tokens)", report.OriginalTokens, report.CompressedTokens)))
		msgs = stripSystemMessage(compressed)
	}

	var routingHint *types.RoutingHint
	var preflightResult *types.PreflightResult
	visibleTools := harness.VisibleToolsFromState(sc.Harness)
	// #deprecated: legacy fallback (preflight + routing + system prompt build are QueryLoop-irrelevant)
	if harnessEnabled && !workerLocal {
		provisionalContext := basePrompt
		if len(memoryEntries) > 0 {
			memCtx, _ := memory.FormatMemoryContext(memoryEntries, e.cfg.LongTerm.RecallMaxTokens)
			provisionalContext += "\n" + memCtx
		}
		if e.cfg.Preflight.Enabled {
			pfCtx, pfSpan := e.startSpan(ctx, telemetry.OpContextHarnessPreflight, tracer.SpanKindInternal)
			result := e.preflight.Evaluate(sc, message, visibleTools, provisionalContext)
			preflightResult = &result
			filtered, _ := e.preflight.FilterVisibleTools(message, visibleTools)
			visibleTools = filtered
			if sc.Harness != nil {
				sc.Harness.Report.VisibleToolList = toolDescsToVisibleTools(visibleTools)
				sc.Harness.Report.VisibleTools = len(visibleTools)
			}
			if pfSpan != nil {
				pfSpan.SetAttributes(tracer.Attribute{Key: "preflight.warning_count", Value: fmt.Sprintf("%d", len(result.Warnings))})
				pfSpan.End()
			}
			_ = pfCtx
		}
		if e.cfg.Harness.Routing.Enabled {
			routeCtx, routeSpan := e.startSpan(ctx, telemetry.OpContextHarnessRoute, tracer.SpanKindInternal)
			hint := e.router.Route(message, visibleTools, e.cfg.Harness.Routing.MaxMatches)
			if len(hint.Tools) > 0 {
				routingHint = &hint
			}
			if routeSpan != nil {
				routeSpan.SetAttributes(tracer.Attribute{Key: "matched_tools", Value: strings.Join(hint.Tools, ",")})
				routeSpan.End()
			}
			_ = routeCtx
		}

		var bootstrapReport *types.BootstrapReport
		var workspace *types.WorkspaceContext
		if sc.Harness != nil {
			bootstrapReport = &sc.Harness.Report
			workspace = &sc.Harness.Report.Workspace
		}
		buildInput := harness.SystemPromptBuildInput{
			WorkDir: session.WorkDir,
			Session: session,
			Runtime: harness.ProcessRuntimeContext{
				SessionID: session.SessionID,
				RequestID: session.RequestID,
				UserID:    session.UserID,
			},
			AgentsRaw:            basePrompt,
			MemoryEntries:        memoryEntries,
			Bootstrap:            bootstrapReport,
			Workspace:            workspace,
			Routing:              routingHint,
			Preflight:            preflightResult,
			HarnessEnabled:       true,
			OmitAgentsFromSystem: e.cfg.UserContext.Mode == "prepend",
		}
		_, buildSpan := e.startSpan(ctx, telemetry.OpContextSystemPromptBuild, tracer.SpanKindInternal)
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
		view := append([]types.Message{}, msgs...)
		if sc.SystemPrompt != "" {
			view = append([]types.Message{{Role: types.MessageRoleSystem, Content: sc.SystemPrompt}}, view...)
		}
		e.memory.SetCompressedView(sc, view)
	} else {
		view := append([]types.Message{}, msgs...)
		if sc.SystemPrompt != "" {
			view = append([]types.Message{{Role: types.MessageRoleSystem, Content: sc.SystemPrompt}}, view...)
		}
		e.memory.SetCompressedView(sc, view)
	}

	working := memory.NewWorkingMemory()
	result, runErr := e.pev.Run(ctx, sc, sc.CompressedView, message, func(ev *contracts.EngineEvent) {
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
			for i, tc := range result.ToolCallHistory {
				callID := strings.TrimSpace(tc.CallID)
				if callID == "" {
					callID = fmt.Sprintf("call_%s_%d", tc.ToolName, i)
				}
				// 构建工具调用的 assistant 消息（包含 tool_calls）
				tcJSON, _ := json.Marshal([]struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				}{{
					ID:   callID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tc.ToolName, Arguments: tc.Input},
				}})
				tcMsg := types.Message{
					Role:     types.MessageRoleAssistant,
					Content:  "",
					Metadata: map[string]string{"tool_calls": string(tcJSON)},
				}
				e.memory.AppendFullMessage(sc, tcMsg)

				resultContent := tc.Output
				if tc.Error != "" {
					resultContent = "Error: " + tc.Error
				}
				resultMsg := types.Message{
					Role:     types.MessageRoleTool,
					Content:  resultContent,
					Metadata: map[string]string{"tool_call_id": callID},
				}
				e.memory.AppendFullMessage(sc, resultMsg)
			}
		}

		if text := working.FlushStream(); text != "" {
			e.memory.AppendMessage(sc, types.MessageRoleAssistant, text)
			assistantSummary = text
		}
		if assistantSummary == "" {
			assistantSummary = lastAssistantContent(sc.Messages)
		}
		// #deprecated: legacy fallback (QueryLoop manages its own transcript via sidechain)
		if harnessEnabled && !workerLocal {
			e.transcript.AppendTurn(sc, message, assistantSummary)
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

	_, saveSpan := e.startSpan(ctx, telemetry.OpContextMemorySnapshotSave, tracer.SpanKindInternal,
		tracer.Attribute{Key: "snapshot.message_count", Value: fmt.Sprintf("%d", len(sc.Messages))},
	)
	if !workerLocal {
		if data, persistErr := e.memory.PersistSnapshot(sc); persistErr == nil {
			session.ContextSnapshot = data
			e.observer.EmitSnapshotRestored(session.SessionID, false)
			if saveSpan != nil {
				saveSpan.SetAttributes(tracer.Attribute{Key: "snapshot.size_bytes", Value: fmt.Sprintf("%d", len(data))})
				saveSpan.End()
			}
		} else {
			slog.Warn("contextengine: persist snapshot failed", "error", persistErr)
			if saveSpan != nil {
				saveSpan.RecordError(persistErr)
				saveSpan.End()
			}
		}
	} else if saveSpan != nil {
		saveSpan.End()
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

func infoEvent(sessionID, content string) *contracts.EngineEvent {
	return &contracts.EngineEvent{
		Type:      "info",
		Content:   content,
		SessionID: sessionID,
		Metadata:  map[string]string{"category": "context"},
	}
}

func errorEvent(sessionID string, err *errors.SentinelError, recoverable bool) *contracts.EngineEvent {
	rec := "false"
	if recoverable {
		rec = "true"
	}
	return &contracts.EngineEvent{
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

func stripSystemMessage(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	if msgs[0].Role == types.MessageRoleSystem {
		return append([]types.Message(nil), msgs[1:]...)
	}
	return msgs
}

func (e *ContextEngine) initMetrics() {
	e.metricsOnce.Do(func() {
		if e.obsBridge == nil || e.obsBridge.Meter() == nil {
			return
		}
		e.compressionRatio, _ = e.obsBridge.Meter().Float64Histogram("compression_ratio",
			metrics.WithBounds(metrics.CompressionRatioBounds()))
	})
}

func mapProcessError(sessionID string, err error) *contracts.EngineEvent {
	if err == nil {
		return nil
	}
	var se *errors.SentinelError
	if stderrors.As(err, &se) {
		return errorEvent(sessionID, se, false)
	}
	return errorEvent(sessionID, errors.WithCode("CTX_PROCESS_FAILED", err.Error(), err), false)
}
