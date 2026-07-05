package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type stubObsMUPS struct {
	system string
	calls  int
}

func (s *stubObsMUPS) MaterializeForMUPS(_ context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	s.calls++
	appendix := i18n.ObservationTaskAppendix(i18n.ParseLanguage(req.Policy.Locale))
	return contracts.MUPSPreparedContext{
		SystemPrompt:  strings.TrimSpace(s.system) + "\n\n" + appendix,
		PhaseAppendix: appendix,
	}, nil
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

// T: D7-S2-A90-T01 — LLMObservationProposer uses D2 MaterializeForMUPS appendix.
func TestLLMObservationProposer_CallsD2BeforeD3(t *testing.T) {
	mups := &stubObsMUPS{system: "你是 Devrix 助手。"}
	llm := &stubObsLLM{raw: `[{"kind":"obs_uncertainty","strength":0.6,"question":"需要 API 版本？","evidence":["wi_1"]}]`}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleZH)
	got, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: "wi_1",
		Directive:  "你好",
	})
	if err != nil {
		t.Fatal(err)
	}
	if mups.calls != 1 {
		t.Fatalf("MaterializeForMUPS calls = %d, want 1", mups.calls)
	}
	if !strings.Contains(llm.lastSystem, "你是 Devrix") {
		t.Fatalf("system prompt missing D2 base: %q", llm.lastSystem)
	}
	if !strings.Contains(llm.lastSystem, "Observe 节点") {
		t.Fatalf("system prompt missing zh observation appendix: %q", llm.lastSystem)
	}
	appendix := i18n.ObservationTaskAppendix(i18n.LocaleZH)
	if strings.Count(llm.lastSystem, appendix) != 1 {
		t.Fatalf("observation appendix duplicated: count=%d", strings.Count(llm.lastSystem, appendix))
	}
	if len(got) != 1 || got[0].Kind != orchtypes.ObsUncertainty {
		t.Fatalf("got = %+v", got)
	}
}

// T: D7-S5-A97-T01 — prepared Observe system prompt includes semantic appendix markers.
func TestLLMObservationProposer_SystemIncludesSemanticMarkers(t *testing.T) {
	mups := &stubObsMUPS{system: "你是 Devrix 助手。"}
	llm := &stubObsLLM{raw: "[]"}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleZH)
	if _, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID: "s1", WorkItemID: "wi_1", Directive: "hi",
	}); err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"obs_uncertainty", "语义规则（机器可读）", "范围/目标不清"} {
		if !strings.Contains(llm.lastSystem, marker) {
			t.Fatalf("system missing semantic marker %q: %q", marker, llm.lastSystem)
		}
	}
}

func TestLLMObservationProposer_EnglishAppendix(t *testing.T) {
	mups := &stubObsMUPS{system: "You are Devrix."}
	llm := &stubObsLLM{raw: "[]"}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleEN)
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
	if len(got) != 1 {
		t.Fatalf("got = %+v", got)
	}
}
