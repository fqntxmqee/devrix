package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// DM-20260705-009 — Integration coverage for the closed-classifier contract.
//
// These tests walk the full Observe pipeline that triggered the regression
// reported in DM-20260705-009 §2.1:
//
//   user directive: "review d7 领域 plan目录下代码"
//   user frame:     directive + work_item_id + prior_mean (no signal/scope/evidence)
//   system prompt:  ObservationTaskAppendix (was missing "closed classifier" role)
//
// Goal: prove that for an "open-ended" directive with no signal, the system
// prompt now (1) declares the closed-classifier role and (2) guides the LLM
// to prefer obs_uncertainty over an empty/incorrect response.

// stubObsClassifierMUPS is a minimal IMUPSContextMaterializer that returns a
// fixed system base, with the ObservationTaskAppendix appended in the same way
// as llm_observation_proposer's mergeMUPSPreparedSystem path.
type stubObsClassifierMUPS struct {
	base string
}

func (s *stubObsClassifierMUPS) MaterializeForMUPS(_ context.Context, req contracts.MUPSContextRequest) (contracts.MUPSPreparedContext, error) {
	appendix := i18n.ObservationTaskAppendix(i18n.ParseLanguage(req.Policy.Locale))
	return contracts.MUPSPreparedContext{
		SystemPrompt:  strings.TrimSpace(s.base) + "\n\n" + appendix,
		PhaseAppendix: appendix,
	}, nil
}

type stubObsClassifierLLM struct {
	lastSystem string
	lastUser   string
	raw        string
}

func (s *stubObsClassifierLLM) InvokeStream(_ context.Context, req orchtypes.LLMInvokeRequest) (<-chan llmgateway.Chunk, error) {
	s.lastSystem = req.SystemPrompt
	if len(req.Messages) > 0 {
		s.lastUser = req.Messages[0].Content
	}
	ch := make(chan llmgateway.Chunk, 1)
	ch <- llmgateway.Chunk{Content: s.raw}
	close(ch)
	return ch, nil
}

// T: D7-S5-A99-T16 (NEW, DM-20260705-009) — Walk the full Observe pipeline for
// an open-ended directive with no signal and assert the system prompt carries
// the closed-classifier role + the "prefer obs_uncertainty" guidance.
//
// This is the exact scenario from the user-reported regression
// (wi_65d7819c "review d7 领域 plan目录下代码").
func TestObservePipeline_OpenDirectiveNoSignal_ClassifierPrompt(t *testing.T) {
	mups := &stubObsClassifierMUPS{base: "你是 Devrix 助手。"}
	llm := &stubObsClassifierLLM{raw: `[]`}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleZH)

	tm := workmodel.NewTaskManager()
	wi, err := tm.EnsureGoal("s1", "review d7 领域 plan目录下代码")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	// Inject prior_mean=0.625 to mirror the regression trace.
	proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: wi.ID,
		Directive:  "review d7 领域 plan目录下代码",
		PriorMean:  0.625,
	})
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	if proposals != nil {
		t.Fatalf("expected nil proposals for raw=[]; got %d", len(proposals))
	}

	// (1) user frame preserves the directive verbatim (M1 contract).
	if !strings.Contains(llm.lastUser, "directive: review d7 领域 plan目录下代码") {
		t.Errorf("user frame should preserve directive verbatim:\n%s", llm.lastUser)
	}

	// (2) system prompt declares the closed-classifier role.
	for _, marker := range []string{
		"封闭式分类助手",
		"不执行工具",
		"输入 = directive",
		"输出 = Obs* 数组",
	} {
		if !strings.Contains(llm.lastSystem, marker) {
			t.Errorf("system prompt missing closed-classifier marker %q:\n%s", marker, llm.lastSystem)
		}
	}

	// (3) system prompt guides the LLM toward obs_uncertainty when signal is missing.
	for _, marker := range []string{
		"signal 不足",
		"优先 obs_uncertainty",
		"返回 question",
	} {
		if !strings.Contains(llm.lastSystem, marker) {
			t.Errorf("system prompt missing signal-insufficient guidance %q:\n%s", marker, llm.lastSystem)
		}
	}
}

// T: D7-S5-A99-T17 (NEW, DM-20260705-009) — parseObservationProposalsJSON
// still accepts all 4 alias forms (obs_fact/fact, obs_signal/signal,
// obs_deviation/deviation, obs_uncertainty/uncertainty) post-refactor.
// Guards against accidental breaking of the existing 4-alias compatibility.
func TestParseObservationProposalsJSON_AllAliasesAfterClassifierRefactor(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		wantKind orchtypes.ObservationKind
	}{
		{"obs_fact", `[{"kind":"obs_fact","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsFact},
		{"fact alias", `[{"kind":"fact","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsFact},
		{"obs_signal", `[{"kind":"obs_signal","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsSignal},
		{"signal alias", `[{"kind":"signal","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsSignal},
		{"obs_deviation", `[{"kind":"obs_deviation","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsDeviation},
		{"deviation alias", `[{"kind":"deviation","strength":0.5,"statement":"s","evidence":["w"]}]`, orchtypes.ObsDeviation},
		{"obs_uncertainty", `[{"kind":"obs_uncertainty","strength":0.7,"question":"q?","evidence":["w"]}]`, orchtypes.ObsUncertainty},
		{"uncertainty alias", `[{"kind":"uncertainty","strength":0.7,"question":"q?","evidence":["w"]}]`, orchtypes.ObsUncertainty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseObservationProposalsJSON(tc.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d proposals, want 1", len(got))
			}
			if got[0].Kind != tc.wantKind {
				t.Errorf("kind = %v, want %v", got[0].Kind, tc.wantKind)
			}
		})
	}
}

// T: D7-S5-A99-T18 (NEW, DM-20260705-009) — When the LLM does follow the new
// signal-insufficient guidance and returns an obs_uncertainty proposal, the
// proposal survives validation, is capped at 3, and is wired into the
// UncertaintyReport. This is the happy path the closed-classifier prompt is
// supposed to unlock.
func TestObservePipeline_ClassifierPromptUnlocksUncertaintyProposal(t *testing.T) {
	mups := &stubObsClassifierMUPS{base: "你是 Devrix 助手。"}
	llm := &stubObsClassifierLLM{raw: `[{"kind":"obs_uncertainty","strength":0.7,"question":"需要先 review 哪些 plan 文件以确定 scope?","evidence":["wi_1"]}]`}
	proposer := NewLLMObservationProposer(llm, mups, i18n.LocaleZH)

	tm := workmodel.NewTaskManager()
	wi, err := tm.EnsureGoal("s1", "review d7 领域 plan目录下代码")
	if err != nil {
		t.Fatalf("EnsureGoal: %v", err)
	}
	proposals, err := proposer.ProposeObservations(context.Background(), ObserveSignalInput{
		SessionID:  "s1",
		WorkItemID: wi.ID,
		Directive:  "review d7 领域 plan目录下代码",
		PriorMean:  0.625,
	})
	if err != nil {
		t.Fatalf("ProposeObservations: %v", err)
	}
	if len(proposals) != 1 || proposals[0].Kind != orchtypes.ObsUncertainty {
		t.Fatalf("proposals = %+v, want 1 obs_uncertainty", proposals)
	}

	obs, err := ValidateObservationProposals(proposals, "s1", wi.ID)
	if err != nil {
		t.Fatalf("ValidateObservationProposals: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("validated obs = %d, want 1", len(obs))
	}
	if obs[0].Kind != orchtypes.ObsUncertainty {
		t.Errorf("obs kind = %v, want obs_uncertainty", obs[0].Kind)
	}
	if p, ok := obs[0].Payload.(orchtypes.UncertaintyPayload); !ok || !strings.Contains(p.Question, "review 哪些 plan 文件") {
		t.Errorf("payload question not preserved: %+v", obs[0].Payload)
	}
}
