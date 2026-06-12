package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/layers/multiagent"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/contracts"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

func sharedTerminated(agentID string) error {
	return sharederrors.NewAgentAlreadyTerminatedError(agentID)
}

// Run executes the agent main loop.
func (a *Impl) Run(ctx context.Context) (*multiagent.AgentResult, error) {
	if a.State() == multiagent.AgentStateTerminated {
		return nil, sharedTerminated(a.id)
	}
	if a.State() != multiagent.AgentStateCreated {
		return nil, sharederrors.NewAgentInvalidTransitionError(
			a.State().String(), multiagent.AgentStateRunning.String(),
		)
	}
	if err := a.setState(multiagent.AgentStateRunning); err != nil {
		return nil, err
	}
	a.emit("agent.started", nil)

	ctx, runSpan := a.startSpan(ctx, telemetry.OpAgentRun, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
		tracer.Attribute{Key: "agent.mode", Value: string(a.cfg.Mode)},
		tracer.Attribute{Key: "session.id", Value: a.cfg.SessionID},
	)

	runCtx, cancel := context.WithCancel(ctx)
	if a.cfg.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, a.cfg.Timeout)
	}
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	defer cancel()

	start := time.Now()
	result, err := a.runLoop(runCtx)
	duration := time.Since(start)

	if err != nil {
		a.emit("agent.error", map[string]any{"error": err})
		if runSpan != nil {
			runSpan.RecordError(err)
			runSpan.SetAttributes(tracer.Attribute{Key: "agent.duration_ms", Value: fmt.Sprintf("%d", duration.Milliseconds())})
			runSpan.End()
		}
		a.finishResult(&multiagent.AgentResult{
			Messages: a.GetMessages(),
			ExitCode: 1,
			Error:    err,
			Duration: duration,
		})
		return nil, err
	}
	result.Messages = append(a.GetMessages(), result.Messages...)
	result.Duration = duration
	a.emit("agent.terminated", nil)
	if runSpan != nil {
		runSpan.SetAttributes(tracer.Attribute{Key: "agent.duration_ms", Value: fmt.Sprintf("%d", duration.Milliseconds())})
		runSpan.SetStatus(tracer.StatusCodeOk, "")
		runSpan.End()
	}
	a.finishResult(result)
	return result, nil
}

func (a *Impl) runLoop(ctx context.Context) (*multiagent.AgentResult, error) {
	if err := a.setState(multiagent.AgentStateIterating); err != nil {
		return nil, err
	}
	a.emit("agent.iterating", nil)

	input := a.cfg.InitialInput
	if input == "" {
		input = "continue"
	}

	engine := a.processEngine()
	if engine == nil {
		return nil, sharederrors.WithCode("AGT_ENGINE_ERROR", "engine not configured", nil)
	}
	eventCh := engine.Process(ctx, a.session, input)
	var finalText string

	for ev := range eventCh {
		if ev == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, sharederrors.NewAgentTimeoutError(a.id, a.cfg.Timeout.String())
			}
			return nil, sharederrors.NewAgentContextCancelledError(a.id)
		}

		if a.engineEventSink != nil {
			a.engineEventSink(ev)
		}

		switch ev.Type {
		case "permission", "tool_call":
			_ = a.setState(multiagent.AgentStateIterating)
			// Capture tool_call events as messages so Join can dedup by
			// call_id (DM-20260611-005). The Metadata map on the message
			// preserves the engine's call_id key.
			if callID, ok := ev.Metadata["call_id"]; ok && callID != "" {
				a.appendMessages(types.Message{
					Role:    types.MessageRoleAssistant,
					Content: ev.Content,
					Metadata: map[string]string{
						"call_id": callID,
						"tool":    ev.ToolName,
						"event":   "tool_call",
					},
				})
			}
		case "text":
			if ev.Metadata["is_complete"] == "true" || ev.Metadata["is_complete"] == "" {
				finalText = ev.Content
			}
		case "complete":
			if ev.Content != "" {
				finalText = ev.Content
			}
			if finalText != "" {
				a.appendMessages(types.Message{
					Role:    types.MessageRoleAssistant,
					Content: finalText,
				})
			}
			return &multiagent.AgentResult{Messages: a.GetMessages(), ExitCode: 0}, nil
		case "error":
			return nil, sharederrors.WithCode("AGT_ENGINE_ERROR", ev.Content, nil)
		}
	}

	if err := ctx.Err(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, sharederrors.NewAgentTimeoutError(a.id, a.cfg.Timeout.String())
		}
		return nil, sharederrors.NewAgentContextCancelledError(a.id)
	}
	return &multiagent.AgentResult{ExitCode: 0, Messages: a.GetMessages()}, nil
}

// Wait blocks until the agent terminates.
func (a *Impl) Wait(ctx context.Context) (*multiagent.AgentResult, error) {
	select {
	case <-ctx.Done():
		return nil, sharederrors.NewAgentContextCancelledError(a.id)
	case <-a.done:
		a.mu.RLock()
		defer a.mu.RUnlock()
		if a.result == nil {
			return &multiagent.AgentResult{ExitCode: 0}, nil
		}
		return a.result, a.result.Error
	}
}

// Terminate force-stops the agent and its children.
func (a *Impl) Terminate(ctx context.Context) error {
	if a.State() == multiagent.AgentStateTerminated {
		return sharedTerminated(a.id)
	}
	_, termSpan := a.startSpan(ctx, telemetry.OpAgentTerminate, tracer.SpanKindInternal,
		tracer.Attribute{Key: "agent.id", Value: a.id},
	)
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.terminateChildren(ctx)
	a.emit("agent.terminated", map[string]any{"forced": true})
	a.finishResult(&multiagent.AgentResult{
		ExitCode: 130,
		Error:    sharederrors.NewAgentContextCancelledError(a.id),
	})
	if termSpan != nil {
		termSpan.SetStatus(tracer.StatusCodeOk, "")
		termSpan.End()
	}
	return nil
}

// StubEngine is a test double for contracts.IEngine.
type StubEngine struct {
	Events []*contracts.EngineEvent
	Err    error
}

func (s *StubEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *contracts.EngineEvent {
	ch := make(chan *contracts.EngineEvent, len(s.Events)+1)
	go func() {
		defer close(ch)
		if s.Err != nil {
			ch <- &contracts.EngineEvent{Type: "error", Content: s.Err.Error(), SessionID: session.SessionID}
			return
		}
		for _, ev := range s.Events {
			select {
			case <-ctx.Done():
				return
			case ch <- ev:
			}
		}
	}()
	return ch
}
