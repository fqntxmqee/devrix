package gateway

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

// StubContextEngine is a stub implementation for testing and development
type StubContextEngine struct{}

// NewStubContextEngine creates a new stub context engine
func NewStubContextEngine() *StubContextEngine {
	return &StubContextEngine{}
}

// Process implements IContextEngine.Process
func (e *StubContextEngine) Process(ctx context.Context, session *types.Session, message string) <-chan *EngineEvent {
	ch := make(chan *EngineEvent, 10)

	go func() {
		defer close(ch)

		// Echo back the user's message
		ch <- &EngineEvent{
			Type:      "text",
			SessionID: session.SessionID,
			Content:   "Echo: " + message,
		}

		// Emit completion
		ch <- &EngineEvent{
			Type:      "complete",
			SessionID: session.SessionID,
		}
	}()

	return ch
}
