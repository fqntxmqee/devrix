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

// DM-20260706-007: verify the Plan proposer routes messages through
// messagesForLLMInvoke so the AGENTS.md prepend (D{N} → path mapping) reaches
// the LLM. Observed gap in sess_1783333760211_6000 where Plan LLM emitted
// scope_in=["d7领域/plan/"] because it had no project structure context.

type stubPlanMUPSWithPrepend struct {
	prepend map[string]string
}

func (s *stubPlanMUPSWithPrepend) MaterializeForMUPS(_ context.Context, _ contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	return contracts.MUPSPreparedContext{
		SystemPrompt:       "你是 Plan 助手。",
		UserContextPrepend: s.prepend,
	}, nil
}

type capturingPlanLLM struct {
	lastMessages []types.Message
}

func (s *capturingPlanLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.lastMessages = req.Messages
	ch := make(chan llmgateway.Chunk, 1)
	// Minimal valid Plan JSON output so parseStrategicPlanJSON doesn't error.
	ch <- llmgateway.Chunk{Content: `{"execution_mode":"single","scope_in":["internal/layers/orchestration/"],"child_specs":[],"deliverable_contract":{"citation":"none","severity":"none","reject":[],"min_runes":0},"react_iters_hint":3,"rationale":"ok"}`}
	close(ch)
	return ch, nil
}

func TestLLMStrategicPlanProposer_PrependsUserContext(t *testing.T) {
	prepend := map[string]string{
		"claudeMd": "D7 → internal/layers/orchestration/",
	}
	mups := &stubPlanMUPSWithPrepend{prepend: prepend}
	llm := &capturingPlanLLM{}
	proposer := NewLLMStrategicPlanProposer(llm, mups, i18n.LocaleZH)
	in := StrategicPlanInput{
		SessionID:  "sess_test",
		WorkItemID: "wi_1",
		Directive:  "review d7 领域 plan目录下代码",
	}
	if _, err := proposer.ProposeStrategicPlan(context.Background(), in); err != nil {
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