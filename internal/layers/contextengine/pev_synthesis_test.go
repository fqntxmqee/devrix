package contextengine_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/contextengine"
	mockctx "github.com/devrix/devrix/internal/layers/contextengine/mock"
	"github.com/devrix/devrix/internal/layers/contextengine/registry"
	"github.com/devrix/devrix/internal/shared/config"
	sharederrors "github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

type sequentialLLM struct {
	mu        sync.Mutex
	responses []func(*contextengine.LLMRequest) contextengine.LLMChunk
	call      int
}

func (s *sequentialLLM) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	ch := make(chan contextengine.LLMChunk, 1)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			return
		default:
		}
		s.mu.Lock()
		idx := s.call
		s.call++
		s.mu.Unlock()
		if idx >= len(s.responses) {
			return
		}
		ch <- s.responses[idx](req)
	}()
	return ch, nil
}

func (s *sequentialLLM) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.call
}

type synthesisFailLLM struct {
	inner  *sequentialLLM
	failOn int
	err    error
}

func (s *synthesisFailLLM) ChatStream(ctx context.Context, req *contextengine.LLMRequest) (<-chan contextengine.LLMChunk, error) {
	s.inner.mu.Lock()
	idx := s.inner.call
	s.inner.mu.Unlock()
	if idx == s.failOn {
		s.inner.mu.Lock()
		s.inner.call++
		s.inner.mu.Unlock()
		if len(req.Tools) != 0 {
			return nil, fmt.Errorf("synthesis LLM tools = %d, want 0", len(req.Tools))
		}
		return nil, s.err
	}
	return s.inner.ChatStream(ctx, req)
}

func newSynthesisTestEngine(t *testing.T, llm contextengine.ILLMGateway, tools contextengine.IToolRunner) *contextengine.PEVEngine {
	t.Helper()
	cfg := config.DefaultContextEngineConfig()
	cfg.PEV.MaxIterations = 3
	cfg.PEV.VerifyMode = config.VerifyModeBasic
	cfg.Plan.Enabled = false
	return contextengine.NewPEVEngine(
		llm,
		tools,
		registry.NewBuiltinRegistry(),
		mockctx.AllowAllPermission{},
		contextengine.NoOpObserver{},
		&cfg.PEV,
		nil,
		contextengine.NewBuiltinVerifyRunner(t.TempDir()),
		contextengine.NoOpPEVObserver{},
		nil,
		cfg.Plan,
	)
}

func TestPEVEngine_should_force_synthesis_after_failed_tools_in_basic_mode(t *testing.T) {
	callIdx := 0
	llm := &sequentialLLM{
		responses: []func(*contextengine.LLMRequest) contextengine.LLMChunk{
			func(req *contextengine.LLMRequest) contextengine.LLMChunk {
				callIdx++
				if callIdx > 2 {
					t.Fatalf("unexpected LLM call #%d with %d tools", callIdx, len(req.Tools))
				}
				if callIdx == 1 {
					return contextengine.LLMChunk{
						ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
						Done:      true,
					}
				}
				if len(req.Tools) != 0 {
					t.Fatalf("synthesis call tools = %d, want 0", len(req.Tools))
				}
				return contextengine.LLMChunk{Content: "summary after tool failure", Done: true}
			},
		},
	}
	engine := newSynthesisTestEngine(t, llm, &failingToolRunner{})
	sc := &types.SessionContext{SessionID: "sess_basic_fail", Model: "test", WorkDir: t.TempDir()}

	_, err := engine.Run(context.Background(), sc, nil, "hello", func(*gateway.EngineEvent) {})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if llm.calls() != 2 {
		t.Fatalf("LLM calls = %d, want 2 (tool round + synthesis)", llm.calls())
	}
}

func TestPEVEngine_should_force_synthesis_llm_when_tools_only(t *testing.T) {
	llm := &sequentialLLM{
		responses: []func(*contextengine.LLMRequest) contextengine.LLMChunk{
			func(_ *contextengine.LLMRequest) contextengine.LLMChunk {
				return contextengine.LLMChunk{
					ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls -la"}},
					Done:      true,
				}
			},
			func(req *contextengine.LLMRequest) contextengine.LLMChunk {
				if len(req.Tools) != 0 {
					t.Fatalf("synthesis LLM tools = %d, want 0", len(req.Tools))
				}
				last := req.Messages[len(req.Messages)-1]
				if last.Role != types.MessageRoleUser {
					t.Fatalf("synthesis last role = %q, want user", last.Role)
				}
				if !strings.Contains(last.Content, "工具执行结果") {
					t.Fatalf("synthesis prompt = %q", last.Content)
				}
				return contextengine.LLMChunk{
					Content: "The directory listing shows the devrix project folder.",
					Done:    true,
				}
			},
		},
	}
	engine := newSynthesisTestEngine(t, llm, &mockctx.ToolRunner{Output: "total 48\ndrwxr-xr-x devrix"})
	sc := &types.SessionContext{SessionID: "sess_test", Model: "test", WorkDir: t.TempDir()}

	var events []gateway.EngineEvent
	result, err := engine.Run(context.Background(), sc, nil, "list files", func(ev *gateway.EngineEvent) {
		events = append(events, *ev)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if llm.calls() != 2 {
		t.Fatalf("LLM calls = %d, want 2", llm.calls())
	}

	var textContent string
	for _, ev := range events {
		if ev.Type == "text" && ev.Metadata["source"] != "tool_fallback" {
			textContent += ev.Content
		}
	}
	if !strings.Contains(textContent, "devrix") {
		t.Fatalf("synthesis text = %q", textContent)
	}
	if len(result.Messages) != 1 {
		t.Fatalf("result messages = %+v", result.Messages)
	}
}

func TestPEVEngine_should_force_synthesis_llm_when_preamble_text_with_tools(t *testing.T) {
	llm := &sequentialLLM{
		responses: []func(*contextengine.LLMRequest) contextengine.LLMChunk{
			func(_ *contextengine.LLMRequest) contextengine.LLMChunk {
				return contextengine.LLMChunk{
					Content:   "让我查一下未完成的项目。",
					ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls openspec/changes"}},
					Done:      true,
				}
			},
			func(req *contextengine.LLMRequest) contextengine.LLMChunk {
				if len(req.Tools) != 0 {
					t.Fatalf("synthesis LLM tools = %d, want 0", len(req.Tools))
				}
				return contextengine.LLMChunk{
					Content: "当前还有 devrix-layering-standard 变更未完成。",
					Done:    true,
				}
			},
		},
	}
	engine := newSynthesisTestEngine(t, llm, &mockctx.ToolRunner{Output: "devrix-layering-standard/"})
	sc := &types.SessionContext{SessionID: "sess_test", Model: "test", WorkDir: t.TempDir()}

	_, err := engine.Run(context.Background(), sc, nil, "未完成项目?", func(*gateway.EngineEvent) {})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if llm.calls() != 2 {
		t.Fatalf("LLM calls = %d, want 2 (preamble must not skip synthesis)", llm.calls())
	}
}

func TestPEVEngine_should_emit_tool_fallback_when_synthesis_llm_fails(t *testing.T) {
	llmErr := sharederrors.NewProviderUnavailableError(
		fmt.Errorf("provider minimax status 400: {\"error\":\"bad request\"}"),
	)
	llm := &sequentialLLM{
		responses: []func(*contextengine.LLMRequest) contextengine.LLMChunk{
			func(_ *contextengine.LLMRequest) contextengine.LLMChunk {
				return contextengine.LLMChunk{
					ToolCalls: []contextengine.ToolCall{{ID: "tc1", Name: "bash", Input: "ls"}},
					Done:      true,
				}
			},
		},
	}
	failingLLM := &synthesisFailLLM{inner: llm, failOn: 1, err: llmErr}
	engine := newSynthesisTestEngine(t, failingLLM, &mockctx.ToolRunner{Output: "devrix output"})
	sc := &types.SessionContext{SessionID: "sess_test", Model: "test", WorkDir: t.TempDir()}

	var events []gateway.EngineEvent
	result, err := engine.Run(context.Background(), sc, nil, "hello", func(ev *gateway.EngineEvent) {
		events = append(events, *ev)
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var fallbackText string
	for _, ev := range events {
		if ev.Type == "text" && ev.Metadata["source"] == "tool_fallback" {
			fallbackText = ev.Content
		}
	}
	if fallbackText == "" {
		t.Fatal("expected tool_fallback after synthesis failure")
	}
	if len(result.Messages) != 1 || !strings.Contains(result.Messages[0].Content, "devrix") {
		t.Fatalf("result messages = %+v", result.Messages)
	}
}
