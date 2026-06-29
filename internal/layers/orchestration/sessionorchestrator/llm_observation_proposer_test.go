package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
)

type stubObsCtxPreparer struct {
	system string
	calls  int
}

func (s *stubObsCtxPreparer) Prepare(_ context.Context, _ PrepareRequest) (PreparedContext, error) {
	s.calls++
	return PreparedContext{SystemPrompt: s.system}, nil
}

type stubObsLLM struct {
	lastSystem string
	raw        string
}

func (s *stubObsLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.lastSystem = req.SystemPrompt
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: s.raw}
	close(ch)
	return ch, nil
}

func TestLLMObservationProposer_CallsD2BeforeD3(t *testing.T) {
	ctxPrep := &stubObsCtxPreparer{system: "你是 Devrix 助手。"}
	llm := &stubObsLLM{raw: `[{"kind":"obs_uncertainty","strength":0.6,"question":"需要 API 版本？","evidence":["wi_1"]}]`}
	proposer := NewLLMObservationProposer(llm, ctxPrep, i18n.LocaleZH)
	got, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: "wi_1",
		Directive:  "你好",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ctxPrep.calls != 1 {
		t.Fatalf("Prepare calls = %d, want 1", ctxPrep.calls)
	}
	if !strings.Contains(llm.lastSystem, "你是 Devrix") {
		t.Fatalf("system prompt missing D2 base: %q", llm.lastSystem)
	}
	if !strings.Contains(llm.lastSystem, "Observe 节点") {
		t.Fatalf("system prompt missing zh observation appendix: %q", llm.lastSystem)
	}
	if len(got) != 1 || got[0].Kind != orchtypes.ObsUncertainty {
		t.Fatalf("got = %+v", got)
	}
}

func TestLLMObservationProposer_EnglishAppendix(t *testing.T) {
	ctxPrep := &stubObsCtxPreparer{system: "You are Devrix."}
	llm := &stubObsLLM{raw: "[]"}
	proposer := NewLLMObservationProposer(llm, ctxPrep, i18n.LocaleEN)
	if _, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "s1", WorkItemID: "wi_1", Directive: "hi",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(llm.lastSystem, "orchestration Observe node") {
		t.Fatalf("system = %q", llm.lastSystem)
	}
}

func TestParseObservationProposalsJSON(t *testing.T) {
	raw := `[{"kind":"obs_fact","strength":0.7,"statement":"ok","evidence":["wi_1"]}]`
	got, err := parseObservationProposalsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != orchtypes.ObsFact {
		t.Fatalf("got = %+v", got)
	}
}
