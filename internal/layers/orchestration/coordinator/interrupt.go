package coordinator

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// HandleInterrupt cancels an in-flight orchestration for the given session
// and emits a "stopped" EngineEvent.
//
// Per R2 命题 E 反驳：Plan Engine in wave/scheduler.go 刻意
// context.WithCancel(context.Background()) 脱离 parentCtx，/stop 不能
// 指望 ctx 传播，必须显式 CancelAll。契约顺序：
//
//	Wave → D4 → Process → stopped Event → TaskCancel→WorkerCancel 反向链路
//
// 与「正常 Process 结束」区分（正常结束 Wave 自行 ctx 触发收尾）。
//
// v1.0 实现：
//  1. wave.CancelAll(sessionID)  （D7-S3）— 若有 Wave 在跑
//  2. d4.CancelAll(sessionID)    （D4）— 若有 delegate worker 在跑
//  3. orchestrator.cancel(sessionID)  — 取消 D7 RunTurn
//  4. 发射 "stopped" EngineEvent 到 sink
//  5. 任务状态联动：running → cancelled （由 caller / WorkModel 触发）
//
// 子能力 5 (TaskCancel→WorkerCancel 反向链路) 在 v1.0 由 HandleInterrupt
// 的 d4 step 隐式覆盖；v1.1 可显式拆出。
type InterruptOptions struct {
	WaveCanceler     func(sessionID string) error
	DelegateCanceler func(sessionID string) error
	ProcessCanceler  func(sessionID string) error
	Sink             EventPublisher
}

// InterruptHandler is the v1.0 HandleInterrupt entry point.
type InterruptHandler struct {
	mu           sync.Mutex
	opts         InterruptOptions
	orchestrator *SessionOrchestrator
}

// NewInterruptHandler builds the handler. waveCancel and DelegateCanceler are
// optional (v1.0 may not have a running Wave/D4 for a simple FastPath).
func NewInterruptHandler(orch *SessionOrchestrator, opts InterruptOptions) *InterruptHandler {
	return &InterruptHandler{opts: opts, orchestrator: orch}
}

// Handle runs the cancel sequence. It is idempotent: calling it on a
// session with no active orchestration is a no-op.
//
// Returns when the cancel sequence has completed (best-effort). Emits a
// "stopped" EngineEvent with kind=stopped to the sink (if configured).
func (h *InterruptHandler) Handle(ctx context.Context, sessionID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Step 1: Wave CancelAll. wave/scheduler.go deliberately detaches from
	// the parent ctx, so we must call its public CancelAll.
	if h.opts.WaveCanceler != nil {
		if err := h.opts.WaveCanceler(sessionID); err != nil {
			slog.Warn("orchestrator: HandleInterrupt wave cancel failed", "sessionID", sessionID, "err", err)
		}
	}
	// Step 2: D4 delegate cancel. Best-effort; d4 may have no worker.
	if h.opts.DelegateCanceler != nil {
		if err := h.opts.DelegateCanceler(sessionID); err != nil {
			slog.Warn("orchestrator: HandleInterrupt delegate cancel failed", "sessionID", sessionID, "err", err)
		}
	}
	// Step 3: orchestrator (D7 RunTurn) cancel.
	if h.opts.ProcessCanceler != nil {
		if err := h.opts.ProcessCanceler(sessionID); err != nil {
			slog.Warn("orchestrator: HandleInterrupt process cancel failed", "sessionID", sessionID, "err", err)
		}
	}
	// Step 4: emit stopped event. EngineEvent.Type is an open string in the
	// contracts package; "stopped" is the v1.0 D7 value (per d7-domain.md).
	if h.opts.Sink != nil {
		ev := &contracts.EngineEvent{
			Type:      "stopped",
			SessionID: sessionID,
			Content:   "orchestration stopped by interrupt",
			Metadata: map[string]string{
				"reason":     "user_interrupt",
				"stopped_at": time.Now().UTC().Format(time.RFC3339Nano),
			},
		}
		h.opts.Sink.Publish(ctx, ev)
	}
	return nil
}
