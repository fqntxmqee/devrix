package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// ErrWorkerCardClosed indicates the renderer has been closed and no further
// events should be processed.
var ErrWorkerCardClosed = errors.New("worker_card: renderer closed")

// WorkerCardOptions configures card creation (presentation DTO).
type WorkerCardOptions = contracts.WorkerCardOpts

// WorkerCardRenderer renders a per-worker double-block (thinking + output)
// Feishu card. One session gets N independent cards, one per Wave worker.
type WorkerCardRenderer struct {
	cardkit  CardkitSurface
	mu       sync.Mutex
	sessions map[string]*WorkerCardSession
	closed   bool
}

// CardkitSurface is the subset of CardkitClient used by the worker card
// renderer. Defined as an interface so tests can inject a fake without
// pulling in the full Feishu client.
type CardkitSurface interface {
	CreateCard(ctx context.Context, cardJSON string) (string, error)
	StreamElementContent(ctx context.Context, cardID, elementID, content string, sequence int) error
	UpdateCard(ctx context.Context, cardID, cardJSON string, sequence int) error
}

// WorkerCardSession tracks the streaming state for a single worker card.
type WorkerCardSession struct {
	SessionID   string
	TaskID      string
	WorkerID    string
	WorkerKind  contracts.WorkerKind
	CardID      string
	Title       string
	thinkingSeq int
	outputSeq   int
	thinkingBuf strings.Builder
	outputBuf   strings.Builder
	status      string
	mu          sync.Mutex
	created     bool
	createdAt   time.Time
}

// NewWorkerCardRenderer returns a renderer.
func NewWorkerCardRenderer(cardkit CardkitSurface) *WorkerCardRenderer {
	return &WorkerCardRenderer{
		cardkit:  cardkit,
		sessions: make(map[string]*WorkerCardSession),
	}
}

// Close marks the renderer as closed; subsequent events are no-ops.
func (r *WorkerCardRenderer) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
}

func workerCardKey(sessionID, taskID string) string {
	return sessionID + ":" + taskID
}

// GetSession returns the session for the (sessionID, taskID) pair, creating
// it on first use.
func (r *WorkerCardRenderer) GetSession(opts WorkerCardOptions) *WorkerCardSession {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	key := workerCardKey(opts.SessionID, opts.TaskID)
	s, ok := r.sessions[key]
	if !ok {
		s = &WorkerCardSession{
			SessionID:  opts.SessionID,
			TaskID:     opts.TaskID,
			WorkerID:   opts.WorkerID,
			WorkerKind: opts.WorkerKind,
			Title:      opts.Title,
			createdAt:  time.Now(),
		}
		r.sessions[key] = s
	}
	return s
}

// EmitWorkerEvent drives the card from a presentation WorkerStreamEvent.
func (r *WorkerCardRenderer) EmitWorkerEvent(ctx context.Context, opts WorkerCardOptions, ev contracts.WorkerStreamEvent) error {
	if r == nil {
		return errWaveNil
	}
	if r.cardkit == nil {
		return errWaveNoCardkit
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrWorkerCardClosed
	}
	r.mu.Unlock()

	sess := r.GetSession(opts)
	if sess == nil {
		return ErrWorkerCardClosed
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()

	switch ev.Type {
	case "thinking":
		sess.thinkingBuf.WriteString(ev.Content)
		if !sess.created {
			if err := r.createCardLocked(ctx, sess); err != nil {
				return err
			}
		}
		sess.thinkingSeq++
		if err := r.cardkit.StreamElementContent(ctx, sess.CardID, "thinking", sess.thinkingBuf.String(), sess.thinkingSeq); err != nil {
			return err
		}
	case "text", "tool_use":
		sess.outputBuf.WriteString(ev.Content)
		if !sess.created {
			if err := r.createCardLocked(ctx, sess); err != nil {
				return err
			}
		}
		sess.outputSeq++
		if err := r.cardkit.StreamElementContent(ctx, sess.CardID, "output", sess.outputBuf.String(), sess.outputSeq); err != nil {
			return err
		}
	case "error", "complete", "cancelled":
		sess.status = ev.Type
		if !sess.created {
			return nil
		}
		cardJSON := buildWorkerCardJSON(sess)
		seq := sess.thinkingSeq + sess.outputSeq
		if err := r.cardkit.UpdateCard(ctx, sess.CardID, cardJSON, seq+1); err != nil {
			return err
		}
	}
	return nil
}

func (r *WorkerCardRenderer) createCardLocked(ctx context.Context, sess *WorkerCardSession) error {
	cardJSON := buildWorkerCardJSON(sess)
	cardID, err := r.cardkit.CreateCard(ctx, cardJSON)
	if err != nil {
		return fmt.Errorf("create card: %w", err)
	}
	sess.CardID = cardID
	sess.created = true
	return nil
}

// Snapshot returns the current buffer state of a session (test helper).
func (r *WorkerCardRenderer) Snapshot(sessionID, taskID string) (thinking string, output string, status string, ok bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	s, exists := r.sessions[workerCardKey(sessionID, taskID)]
	r.mu.Unlock()
	if !exists {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.thinkingBuf.String(), s.outputBuf.String(), s.status, true
}

// ActiveSessions returns the count of currently-tracked sessions.
func (r *WorkerCardRenderer) ActiveSessions() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sessions)
}

func buildWorkerCardJSON(sess *WorkerCardSession) string {
	status := sess.status
	if status == "" {
		status = "running"
	}
	emoji := workerEmoji(sess.WorkerKind)
	title := sess.Title
	if title == "" {
		title = string(sess.WorkerKind) + " / " + sess.TaskID
	}
	thinking := sess.thinkingBuf.String()
	if thinking == "" {
		thinking = "_waiting for thinking…_"
	}
	output := sess.outputBuf.String()
	if output == "" {
		output = "_waiting for output…_"
	}
	card := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": emoji + " " + title,
			},
			"status": workerStatusColor(status),
		},
		"elements": []any{
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "💭 Thinking",
				},
			},
			map[string]any{
				"tag":        "markdown",
				"element_id": "thinking",
				"content":    thinking,
			},
			map[string]any{
				"tag": "hr",
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "📤 Output",
				},
			},
			map[string]any{
				"tag":        "markdown",
				"element_id": "output",
				"content":    output,
			},
			map[string]any{
				"tag": "hr",
			},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "Status: " + status,
				},
			},
		},
	}
	b, _ := json.Marshal(card)
	return string(b)
}

func workerEmoji(kind contracts.WorkerKind) string {
	switch kind {
	case contracts.WorkerKindCursor:
		return "🖱️"
	case contracts.WorkerKindClaudeCode:
		return "🛠️"
	case contracts.WorkerKindSubAgent:
		return "🤖"
	default:
		return "⚙️"
	}
}

func workerStatusColor(status string) string {
	switch status {
	case "complete":
		return "green"
	case "failed":
		return "red"
	case "cancelled":
		return "grey"
	default:
		return "blue"
	}
}

var (
	errWaveNil       = errors.New("worker_card: nil renderer")
	errWaveNoCardkit = errors.New("worker_card: no cardkit client")
)
