package contextengine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/memory"
	"github.com/devrix/devrix/internal/layers/contextengine/prompt"
	"github.com/devrix/devrix/internal/layers/contextengine/snapshot"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// EngineDeps holds dependencies for ContextEngine.
type EngineDeps struct {
	LLM        ILLMGateway
	Tools      IToolRunner
	ToolsReg   IToolRegistry
	Permission IPermissionGate
	Observer   IObserver
	Config     *config.ContextEngineConfig
}

// ContextEngine implements gateway.IContextEngine.
type ContextEngine struct {
	memory     *memory.Manager
	compression *compression.Pipeline
	pev        *PEVEngine
	prompt     *prompt.Loader
	cfg        *config.ContextEngineConfig
	observer   IObserver
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
	store := snapshot.NewStore(&cfg.Snapshot)
	return &ContextEngine{
		memory:      memory.NewManager(cfg, store),
		compression: compression.NewPipeline(cfg.CompressionEnabled),
		pev: NewPEVEngine(
			deps.LLM,
			deps.Tools,
			deps.ToolsReg,
			deps.Permission,
			observer,
			&cfg.PEV,
		),
		prompt:   prompt.NewLoader(&cfg.SystemPrompt),
		cfg:      cfg,
		observer: observer,
	}
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

	emit := func(ev *gateway.EngineEvent) {
		select {
		case <-ctx.Done():
			return
		case ch <- ev:
		}
	}

	systemPrompt := e.prompt.Load(session.WorkDir)
	sc, err := e.memory.LoadOrInit(session, systemPrompt)
	if err != nil {
		emit(infoEvent(session.SessionID, "快照已重置，开始新上下文"))
		session.ContextSnapshot = nil
		sc, err = e.memory.LoadOrInit(session, systemPrompt)
		if err != nil {
			emit(errorEvent(session.SessionID, errors.NewSnapshotCorruptError(err), false))
			return
		}
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	}

	requestID := session.RequestID
	if !e.memory.AppendUserMessage(sc, requestID, message) {
		slog.Debug("contextengine: duplicate request skipped", "sessionID", session.SessionID, "requestID", requestID)
	}

	msgs := sc.Messages
	if e.compression.ShouldCompress(msgs, sc.TokenBudget) {
		compressed, report, compErr := e.compression.Run(ctx, msgs, sc.SystemPrompt, sc.TokenBudget)
		if compErr != nil {
			if se, ok := compErr.(*errors.SentinelError); ok {
				emit(errorEvent(session.SessionID, se, false))
			}
			return
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
	_, runErr := e.pev.Run(ctx, sc, sc.CompressedView, func(ev *gateway.EngineEvent) {
		if ev.Type == "text" && ev.Metadata["is_complete"] == "false" {
			working.AppendStream(ev.Content)
		}
		emit(ev)
	})

	if runErr == nil {
		if text := working.FlushStream(); text != "" {
			e.memory.AppendMessage(sc, types.MessageRoleAssistant, text)
		}
	}

	if data, persistErr := e.memory.PersistSnapshot(sc); persistErr == nil {
		session.ContextSnapshot = data
		e.observer.EmitSnapshotRestored(session.SessionID, false)
	} else {
		slog.Warn("contextengine: persist snapshot failed", "error", persistErr)
	}

	slog.Debug("contextengine: process done", "sessionID", session.SessionID, "duration", time.Since(start))
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
