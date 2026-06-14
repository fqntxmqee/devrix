package capture

import (
	"github.com/devrix/devrix/internal/layers/communication/conclusion"
	"github.com/devrix/devrix/internal/layers/communication/kernel"
	"github.com/devrix/devrix/internal/layers/communication/taskprogress"
	"github.com/devrix/devrix/internal/layers/communication/thinking"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// SignalRouter dispatches engine events to S14/S15/S16 presenters.
//
// DSAFT: D1-S14 PresentThinking, D1-S15 PresentTaskProgress, D1-S16 DeliverConclusion
type SignalRouter struct{}

// SignalInput carries session context and optional signal anchors for one engine event.
type SignalInput struct {
	Session   *types.Session
	Event     *contracts.EngineEvent
	Signal    contracts.IMOutboundSignal
	HasSignal bool
}

// SignalResult reports session state side-effects for the gateway.
type SignalResult struct {
	SessionState types.SessionState
	SetState     bool
	Suppressed   bool
}

// Dispatch routes a single engine event to the canonical signal presenter.
func (SignalRouter) Dispatch(in SignalInput, emit kernel.Emitter) SignalResult {
	if in.Session == nil || in.Event == nil {
		return SignalResult{}
	}
	switch in.Event.Type {
	case "thinking":
		in.Session.SetState(types.SessionStateThinking)
		thinking.EmitThinking(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{SessionState: types.SessionStateThinking, SetState: true}

	case "text":
		in.Session.SetState(types.SessionStateStreaming)
		conclusion.EmitText(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{SessionState: types.SessionStateStreaming, SetState: true}

	case "tool_call":
		taskprogress.EmitToolCall(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{}

	case "tool_result":
		in.Session.SetState(types.SessionStateToolExecuting)
		taskprogress.EmitToolResult(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{SessionState: types.SessionStateToolExecuting, SetState: true}

	case "milestone_progress":
		taskprogress.EmitMilestoneProgress(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{}

	case "worker_progress":
		taskprogress.EmitWorkerProgress(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{}

	case "info":
		conclusion.EmitInfo(in.Session, in.Event, emit)
		return SignalResult{}

	case "complete":
		in.Session.SetState(types.SessionStateCompleted)
		conclusion.EmitComplete(in.Session, in.Event, in.Signal, in.HasSignal, emit)
		return SignalResult{SessionState: types.SessionStateCompleted, SetState: true}

	case "error":
		return SignalResult{Suppressed: false}

	default:
		return SignalResult{}
	}
}

// DispatchError handles S16-A02 error terminal signals with optional suppression.
func (SignalRouter) DispatchError(in SignalInput, emit kernel.Emitter, suppress bool) SignalResult {
	if suppress {
		in.Session.SetState(types.SessionStateCompleted)
		return SignalResult{SessionState: types.SessionStateCompleted, SetState: true, Suppressed: true}
	}
	in.Session.SetState(types.SessionStateFailed)
	conclusion.EmitError(in.Session, in.Event, in.Signal, in.HasSignal, emit)
	return SignalResult{SessionState: types.SessionStateFailed, SetState: true}
}

// ApplyNilHandlerState updates session state when no EventHandler is wired (tests).
func ApplyNilHandlerState(session *types.Session, eventType string) {
	if session == nil {
		return
	}
	switch eventType {
	case "thinking":
		session.SetState(types.SessionStateThinking)
	case "tool", "tool_result":
		session.SetState(types.SessionStateToolExecuting)
	case "text":
		session.SetState(types.SessionStateStreaming)
	case "complete":
		session.SetState(types.SessionStateCompleted)
	case "error":
		session.SetState(types.SessionStateFailed)
	}
}
