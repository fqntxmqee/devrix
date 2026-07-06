package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
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

// stubObsMUPSWithPrepend extends stubObsMUPS with a UserContextPrepend payload
// so the proposer test can verify messagesForLLMInvoke is wired correctly.
// Used by DM-20260706-007 (Observe→Plan data flow).
type stubObsMUPSWithPrepend struct {
	prepend map[string]string
}

func (s *stubObsMUPSWithPrepend) MaterializeForMUPS(_ context.Context, _ contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	return contracts.MUPSPreparedContext{
		SystemPrompt:       "你是 Observe 助手。",
		UserContextPrepend: s.prepend,
	}, nil
}

type capturingObsLLM struct {
	lastMessages []types.Message
}

func (s *capturingObsLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.lastMessages = req.Messages
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: "[]"}
	close(ch)
	return ch, nil
}

// DM-20260706-007: when MUPS returns a UserContextPrepend (AGENTS.md /
// claudeMd), the Observe LLM call must prepend it to messages so the LLM
// sees the project structure table (D{N} → path mapping).
func TestLLMObservationProposer_PrependsUserContext(t *testing.T) {
	prepend := map[string]string{
		"claudeMd": "D7 → internal/layers/orchestration/",
	}
	mups := &stubObsMUPSWithPrepend{prepend: prepend}
	llm := &capturingObsLLM{}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleZH)
	if _, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: "wi_1",
		Directive:  "review d7 领域 plan目录下代码",
	}); err != nil {
		t.Fatal(err)
	}
	if len(llm.lastMessages) < 2 {
		t.Fatalf("expected at least 2 messages after prepend (was %d)", len(llm.lastMessages))
	}
	first := llm.lastMessages[0]
	if first.Role != types.MessageRoleUser {
		t.Fatalf("prepend msg role = %q want user", first.Role)
	}
	if !strings.Contains(first.Content, "claudeMd") || !strings.Contains(first.Content, "D7 → internal/layers/orchestration/") {
		t.Fatalf("prepend msg missing AGENTS.md content:\n%s", first.Content)
	}
	if !strings.Contains(first.Content, "<system-reminder>") {
		t.Fatalf("prepend msg missing <system-reminder> wrapper:\n%s", first.Content)
	}
}
