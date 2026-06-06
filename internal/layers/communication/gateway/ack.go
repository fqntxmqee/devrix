package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Acknowledger handles message acknowledgment
type Acknowledger struct {
	mu         sync.RWMutex
	pending    map[string]*PendingAck
	maxRetries int
	timeout    time.Duration
}

// PendingAck represents a pending acknowledgment
type PendingAck struct {
	MessageID  string
	CreatedAt time.Time
	Retries   int
	Done      chan<- bool
}

// NewAcknowledger creates a new Acknowledger
func NewAcknowledger(maxRetries int, timeout time.Duration) *Acknowledger {
	return &Acknowledger{
		pending:    make(map[string]*PendingAck),
		maxRetries: maxRetries,
		timeout:    timeout,
	}
}

// SendWithAck sends a message and waits for acknowledgment
func (a *Acknowledger) SendWithAck(ctx context.Context, messageID string) error {
	done := make(chan bool, 1)

	a.mu.Lock()
	a.pending[messageID] = &PendingAck{
		MessageID:  messageID,
		CreatedAt: time.Now(),
		Retries:   0,
		Done:      done,
	}
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.pending, messageID)
		a.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("acknowledgment cancelled")
	case <-done:
		return nil
	case <-time.After(a.timeout):
		a.mu.Lock()
		pending := a.pending[messageID]
		a.mu.Unlock()

		if pending != nil && pending.Retries < a.maxRetries {
			pending.Retries++
			// Retry logic would be handled by the caller
			return fmt.Errorf("acknowledgment timeout, retry %d/%d", pending.Retries, a.maxRetries)
		}
		return fmt.Errorf("acknowledgment timeout after %d retries", a.maxRetries)
	}
}

// Ack marks a message as acknowledged
func (a *Acknowledger) Ack(messageID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if pending, ok := a.pending[messageID]; ok {
		close(pending.Done)
	}
}

// Cleanup removes stale pending acknowledgments
func (a *Acknowledger) Cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	for id, pending := range a.pending {
		if now.Sub(pending.CreatedAt) > a.timeout*time.Duration(a.maxRetries+1) {
			close(pending.Done)
			delete(a.pending, id)
		}
	}
}
