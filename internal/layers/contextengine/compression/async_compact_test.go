package compression_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/layers/contextengine/token"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type slowSummarizer struct {
	delay    time.Duration
	response string
	err      error
	calls    atomic.Int32
}

func (s *slowSummarizer) Summarize(ctx context.Context, model, prompt string, maxTokens int) (string, error) {
	s.calls.Add(1)
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if s.err != nil {
		return "", s.err
	}
	if s.response != "" {
		return s.response, nil
	}
	return `{"topics":["t"],"decisions":[],"open_items":[]}`, nil
}

type asyncObserver struct {
	mu        sync.Mutex
	complete  int
	degraded  int
	lastToken string
}

func (o *asyncObserver) OnStep(string, int, int) {}
func (o *asyncObserver) OnAutocompact(meta compression.AutocompactMeta) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if meta.Degraded {
		o.degraded++
	}
}
func (o *asyncObserver) OnAutocompactComplete(_ types.Message, _, asyncToken string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.complete++
	o.lastToken = asyncToken
}

func waitForAsyncDegraded(t *testing.T, obs *asyncObserver, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		obs.mu.Lock()
		done := obs.degraded > 0
		obs.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for async autocompact degraded")
}

func waitForAsyncComplete(t *testing.T, obs *asyncObserver, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		obs.mu.Lock()
		done := obs.complete > 0
		obs.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for async autocompact complete")
}

func autocompactTestMessages(n int) []types.Message {
	var msgs []types.Message
	for i := 0; i < n; i++ {
		role := types.MessageRoleUser
		if i%2 == 1 {
			role = types.MessageRoleAssistant
		}
		msgs = append(msgs, *types.NewMessage("m", "sess-async", role, strings.Repeat("word ", 80)))
	}
	return msgs
}

// Covers: L5-CTX-31
func TestAsyncAutocompact_should_return_placeholder_without_blocking(t *testing.T) {
	counter := &highTokenCounter{inner: token.NewCounter()}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	slow := &slowSummarizer{delay: 200 * time.Millisecond}
	async := compression.NewAsyncAutocompacter(slow)
	obs := &asyncObserver{}

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(counter),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(slow),
		compression.WithAsyncAutocompacter(async),
		compression.WithSessionID("sess-async"),
		compression.WithStepObserver(obs),
	)

	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100
	start := time.Now()
	out, report, err := p.RunForSession(context.Background(), "sess-async", autocompactTestMessages(12), "system", budget)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("expected fast placeholder return, took %v", elapsed)
	}
	hasAutocompact := false
	for _, step := range report.StepsApplied {
		if strings.HasPrefix(step, "autocompact") {
			hasAutocompact = true
			break
		}
	}
	if !hasAutocompact {
		t.Fatalf("expected autocompact step, got %v", report.StepsApplied)
	}
	foundPending := false
	for _, m := range out {
		if m.Metadata["status"] == "pending" {
			foundPending = true
		}
	}
	if !foundPending {
		t.Fatal("expected pending placeholder message")
	}

	waitForAsyncComplete(t, obs, 2*time.Second)

	if err := async.Shutdown(2 * time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if obs.complete != 1 {
		t.Fatalf("expected 1 async complete, got %d", obs.complete)
	}
}

// Covers: L5-CTX-33
func TestAsyncAutocompact_should_degrade_without_losing_head_tail_on_failure(t *testing.T) {
	counter := &highTokenCounter{inner: token.NewCounter()}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	failSum := &slowSummarizer{delay: time.Millisecond, err: context.DeadlineExceeded}
	async := compression.NewAsyncAutocompacter(failSum)
	obs := &asyncObserver{}

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(counter),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(failSum),
		compression.WithAsyncAutocompacter(async),
		compression.WithSessionID("sess-fail"),
		compression.WithStepObserver(obs),
	)

	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100
	out, _, err := p.RunForSession(context.Background(), "sess-fail", autocompactTestMessages(12), "system", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	foundPending := false
	for _, m := range out {
		if m.Metadata["status"] == "pending" {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Fatal("expected pending placeholder preserving head/tail structure")
	}

	waitForAsyncDegraded(t, obs, 2*time.Second)

	if err := async.Shutdown(2 * time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if obs.degraded != 1 {
		t.Fatalf("expected degraded event, got %d", obs.degraded)
	}
	if obs.complete != 0 {
		t.Fatalf("expected no complete event on failure")
	}
}

func TestAsyncAutocompact_should_deduplicate_pending_tasks(t *testing.T) {
	counter := &highTokenCounter{inner: token.NewCounter()}
	cfg := config.DefaultAutocompactConfig()
	cfg.Enabled = true
	cfg.MinMessagesForSummary = 4
	cfg.PreserveHeadTurns = 1
	cfg.PreserveTailTurns = 1

	slow := &slowSummarizer{delay: 300 * time.Millisecond}
	async := compression.NewAsyncAutocompacter(slow)
	obs := &asyncObserver{}

	p := compression.NewPipeline(
		compression.WithEnabled(true),
		compression.WithCounter(counter),
		compression.WithAutocompactConfig(cfg),
		compression.WithSummarizer(slow),
		compression.WithAsyncAutocompacter(async),
		compression.WithSessionID("sess-dedup"),
		compression.WithStepObserver(obs),
	)

	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 100
	msgs := autocompactTestMessages(12)
	_, _, _ = p.RunForSession(context.Background(), "sess-dedup", msgs, "system", budget)
	_, _, _ = p.RunForSession(context.Background(), "sess-dedup", msgs, "system", budget)
	waitForAsyncComplete(t, obs, 2*time.Second)

	if err := async.Shutdown(2 * time.Second); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if slow.calls.Load() < 1 {
		t.Fatal("expected at least one summarize call")
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if obs.complete != 1 {
		t.Fatalf("expected single complete after dedup, got %d", obs.complete)
	}
}
