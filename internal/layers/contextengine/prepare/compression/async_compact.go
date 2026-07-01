package compression

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/google/uuid"
)

// AsyncAutocompacter runs autocompact LLM summarization in the background.
type AsyncAutocompacter struct {
	summarizer Summarizer
	mu         sync.Mutex
	pending    map[string]map[string]context.CancelFunc
	latest     map[string]string
	wg         sync.WaitGroup
}

// NewAsyncAutocompacter creates an async autocompact coordinator.
func NewAsyncAutocompacter(summarizer Summarizer) *AsyncAutocompacter {
	return &AsyncAutocompacter{
		summarizer: summarizer,
		pending:    make(map[string]map[string]context.CancelFunc),
		latest:     make(map[string]string),
	}
}

// StartAsync schedules background summarization; duplicate triggers cancel prior work.
func (a *AsyncAutocompacter) StartAsync(
	sessionID string,
	asyncToken string,
	cfg config.AutocompactConfig,
	turns [][]types.Message,
	head, tail int,
	observer StepObserver,
	loc i18n.Locale,
) {
	if a == nil || a.summarizer == nil || sessionID == "" {
		return
	}

	if asyncToken == "" {
		asyncToken = uuid.NewString()
	}

	a.mu.Lock()
	if _, ok := a.pending[sessionID]; !ok {
		a.pending[sessionID] = make(map[string]context.CancelFunc)
	}
	for token, cancel := range a.pending[sessionID] {
		cancel()
		delete(a.pending[sessionID], token)
	}
	a.latest[sessionID] = asyncToken

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	runCtx, cancel := context.WithTimeout(context.Background(), timeout)
	a.pending[sessionID][asyncToken] = cancel
	a.wg.Add(1)
	a.mu.Unlock()

	go func() {
		defer a.wg.Done()
		defer cancel()
		defer func() {
			a.mu.Lock()
			delete(a.pending[sessionID], asyncToken)
			if len(a.pending[sessionID]) == 0 {
				delete(a.pending, sessionID)
			}
			a.mu.Unlock()
		}()

		middle := flattenTurns(turns[head : len(turns)-tail])
		summary, err := summarizeWithRetry(runCtx, a.summarizer, cfg, middle, loc)
		if err != nil {
			if observer != nil {
				a.mu.Lock()
				isLatest := a.latest[sessionID] == asyncToken
				a.mu.Unlock()
				if isLatest {
					observer.OnAutocompactFailed(sessionID, middle, asyncToken)
				}
				observer.OnAutocompact(AutocompactMeta{Degraded: true, Model: cfg.Model})
			}
			return
		}

		a.mu.Lock()
		isLatest := a.latest[sessionID] == asyncToken
		a.mu.Unlock()
		if !isLatest || observer == nil {
			return
		}

		observer.OnAutocompactComplete(buildSummaryMessage(summary, len(middle), asyncToken, loc), sessionID, asyncToken)
		observer.OnAutocompact(AutocompactMeta{
			Degraded: false,
			Model:    cfg.Model,
		})
	}()
}

// Shutdown cancels pending work and waits up to timeout for goroutines to exit.
func (a *AsyncAutocompacter) Shutdown(timeout time.Duration) error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	for _, sessionTasks := range a.pending {
		for _, cancel := range sessionTasks {
			cancel()
		}
	}
	a.mu.Unlock()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("async autocompact shutdown timeout")
	}
}

func buildAutocompactPlaceholder(turns [][]types.Message, head, tail int, asyncToken string) []types.Message {
	if head <= 0 {
		head = 2
	}
	if tail <= 0 {
		tail = 2
	}
	var out []types.Message
	for i := 0; i < head; i++ {
		out = append(out, turns[i]...)
	}
	out = append(out, types.Message{
		Role:    types.MessageRoleAssistant,
		Content: fmt.Sprintf("[compressing conversation... keeping %d most recent exchanges]", head+tail),
		Metadata: map[string]string{
			"compressed_by": "autocompact",
			"status":        "pending",
			"async_token":   asyncToken,
		},
	})
	for i := len(turns) - tail; i < len(turns); i++ {
		out = append(out, turns[i]...)
	}
	return out
}

func buildSummaryMessage(summary string, middleCount int, asyncToken string, loc i18n.Locale) types.Message {
	return types.Message{
		Role:    types.MessageRoleAssistant,
		Content: formatSummaryContent(summary, loc),
		Metadata: map[string]string{
			"compressed_by":  "autocompact",
			"original_count": fmt.Sprintf("%d", middleCount),
			"status":         "complete",
			"async_token":    asyncToken,
		},
	}
}
