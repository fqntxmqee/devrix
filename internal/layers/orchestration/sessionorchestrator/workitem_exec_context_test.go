package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// T: D7-S9-A91-T01 (DM-20260701-001 RH-MUPS-10 AcceptanceCriteriaVisibility)
//
// Snapshot: when the WorkItemExecContext carries a DeliverableSchema, the
// first user message sent to the LLM must include BOTH the machine-readable
// schema tag AND the human-readable acceptance bar (criteria) so the
// producer can self-correct before verify fails. Pre-RH-MUPS-10 only the
// tag was injected; the criteria text lived only in the verify regex and
// was invisible to the LLM.
func TestWorkItemExecutor_AppendsAcceptanceCriteria(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ack", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	exec.Materializer = nil // keep tests off the Materialize path

	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{
		Item:              &workmodel.WorkItem{ID: "wi_x", Kind: workmodel.WorkKindImplement},
		Tasks:             workmodel.NewTaskManager(),
		DeliverableSchema: workmodel.DeliverableSchemaP0P1FileLine,
		// PriorVerifyReason empty: first-pass execute.
	})
	res, err := exec.ExecuteWorkItem(ctx, "sess_x", "wi_x", "review d2 code")
	if err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if !res.Done {
		t.Fatalf("res.Done = false, want true")
	}
	if len(llm.messages) != 1 {
		t.Fatalf("expected 1 LLM call, got %d", len(llm.messages))
	}
	if len(llm.messages[0]) == 0 {
		t.Fatal("LLM request has no messages")
	}
	directive := llm.messages[0][0].Content
	if !strings.Contains(directive, "<deliverable_schema>p0_p1_file_line</deliverable_schema>") {
		t.Errorf("directive missing schema tag, got: %q", directive)
	}
	// The acceptance bar must surface a file:line-citation example AND the
	// P0/P1 severity tag requirement AND a planning-meta denylist phrase —
	// all three are what verify checks for, and the producer can't follow
	// rules it can't see.
	mustContain := []string{
		"file:line",
		"P0",
		"P1",
		"我将要", // planning meta
		"parallel explore", // planning meta
		"500 runes", // rollup length threshold
	}
	for _, sub := range mustContain {
		if !strings.Contains(directive, sub) {
			t.Errorf("directive missing acceptance-criteria substring %q, got: %q", sub, directive)
		}
	}
}

// T: D7-S9-A91-T02 (DM-20260701-001 RH-MUPS-10 AcceptanceCriteriaVisibility)
//
// Snapshot: when the previous round's verify produced a non-Pass verdict
// (SpawnInline retry), the executor's directive must include a
// "PriorVerifyReason:" section so the LLM can self-correct. Empty reason
// (first-pass or pass-on-prior) must NOT inject the section — we don't
// want the producer to second-guess a clean history.
func TestWorkItemExecutor_AppendsPriorVerifyReasonOnRetry(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ack", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	exec.Materializer = nil

	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{
		Item:              &workmodel.WorkItem{ID: "wi_x", Kind: workmodel.WorkKindImplement},
		Tasks:             workmodel.NewTaskManager(),
		DeliverableSchema: workmodel.DeliverableSchemaP0P1FileLine,
		PriorVerifyReason: "rollup summary too short",
	})
	if _, err := exec.ExecuteWorkItem(ctx, "sess_x", "wi_x", "review d2 code"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	if len(llm.messages) == 0 || len(llm.messages[0]) == 0 {
		t.Fatal("LLM request has no messages")
	}
	directive := llm.messages[0][0].Content
	if !strings.Contains(directive, "PriorVerifyReason: rollup summary too short") {
		t.Errorf("directive missing PriorVerifyReason section, got: %q", directive)
	}
	// The "Adjust your approach" hint rides along with the reason so the
	// LLM treats it as actionable context, not metadata noise.
	if !strings.Contains(directive, "Adjust your approach") {
		t.Errorf("directive missing the action hint that accompanies PriorVerifyReason, got: %q", directive)
	}
}

func TestWorkItemExecutor_NoPriorReasonSectionWhenFirstPass(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ack", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	exec.Materializer = nil

	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{
		Item:  &workmodel.WorkItem{ID: "wi_x", Kind: workmodel.WorkKindImplement},
		Tasks: workmodel.NewTaskManager(),
		// No schema, no prior reason — baseline first-pass case.
	})
	if _, err := exec.ExecuteWorkItem(ctx, "sess_x", "wi_x", "review d2 code"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	directive := llm.messages[0][0].Content
	if strings.Contains(directive, "PriorVerifyReason:") {
		t.Errorf("first-pass directive must not contain PriorVerifyReason section, got: %q", directive)
	}
	if strings.Contains(directive, "<deliverable_schema>") {
		t.Errorf("baseline directive must not contain schema tag, got: %q", directive)
	}
}

// T: D7-S9-A91-T03 (DM-20260701-001 RH-MUPS-10 AcceptanceCriteriaVisibility)
//
// AcceptanceCriteriaFor contract: the criteria text contains the
// plan-meta denylist (let me continue / 我将要 / parallel explore) and the
// ≥500-runes threshold that verify applies. If the LLM doesn't see these
// it has no way to comply on the first pass.
func TestAcceptanceCriteriaFor_ContainsVerifyBar(t *testing.T) {
	crit := workmodel.AcceptanceCriteriaFor(workmodel.DeliverableSchemaP0P1FileLine)
	if crit == "" {
		t.Fatal("AcceptanceCriteriaFor returned empty for P0P1FileLine")
	}
	for _, want := range []string{"file:line", "P0", "P1", "我将要", "parallel explore", "500"} {
		if !strings.Contains(crit, want) {
			t.Errorf("criteria missing %q, got: %q", want, crit)
		}
	}
	if unknown := workmodel.AcceptanceCriteriaFor("nonexistent_schema"); unknown != "" {
		t.Errorf("AcceptanceCriteriaFor for unknown schema must return empty (forward-compat), got %q", unknown)
	}
	if none := workmodel.AcceptanceCriteriaFor(""); none != "" {
		t.Errorf("AcceptanceCriteriaFor for empty schema must return empty, got %q", none)
	}
}

// Compile-time guard: the LLM mock satisfies the orchtypes.LLMInvoker
// contract so this test file's helpers compose with other tests in the
// package without triggering an interface mismatch.
var _ orchtypes.LLMInvoker = (*scriptedLLM)(nil)
