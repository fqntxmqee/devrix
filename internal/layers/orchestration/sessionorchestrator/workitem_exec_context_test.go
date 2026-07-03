package sessionorchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

func TestWorkItemExecutor_AppendsAcceptanceCriteria(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ack", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	exec.Materializer = nil

	contract := workmodel.DefaultTestDeliverableContract()
	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{
		Item:                &workmodel.WorkItem{ID: "wi_x", Kind: workmodel.WorkKindImplement},
		Tasks:               workmodel.NewTaskManager(),
		DeliverableContract: contract,
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
	directive := llm.messages[0][0].Content
	tag := workmodel.DeliverableContractTag(contract)
	if !strings.Contains(directive, tag) {
		t.Errorf("directive missing contract tag, got: %q", directive)
	}
	for _, sub := range []string{"citation=file_line", "severity=p0_p1", "reject=planning_meta"} {
		if !strings.Contains(directive, sub) {
			t.Errorf("directive missing acceptance-criteria substring %q, got: %q", sub, directive)
		}
	}
}

func TestWorkItemExecutor_AppendsPriorVerifyReasonOnRetry(t *testing.T) {
	llm := &scriptedLLM{script: [][]llmgateway.Chunk{
		{{Content: "ack", FinishReason: "stop"}},
	}}
	exec := NewWorkItemExecutor(llm, stubCtxPreparer{}, nil)
	exec.Materializer = nil

	ctx := WithWorkItemExecContext(context.Background(), WorkItemExecContext{
		Item:                &workmodel.WorkItem{ID: "wi_x", Kind: workmodel.WorkKindImplement},
		Tasks:               workmodel.NewTaskManager(),
		DeliverableContract: workmodel.DefaultTestDeliverableContract(),
		PriorVerifyReason:   "rollup summary too short",
	})
	if _, err := exec.ExecuteWorkItem(ctx, "sess_x", "wi_x", "review d2 code"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	directive := llm.messages[0][0].Content
	if !strings.Contains(directive, "PriorVerifyReason: rollup summary too short") {
		t.Errorf("directive missing PriorVerifyReason section, got: %q", directive)
	}
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
	})
	if _, err := exec.ExecuteWorkItem(ctx, "sess_x", "wi_x", "review d2 code"); err != nil {
		t.Fatalf("ExecuteWorkItem: %v", err)
	}
	directive := llm.messages[0][0].Content
	if strings.Contains(directive, "PriorVerifyReason:") {
		t.Errorf("first-pass directive must not contain PriorVerifyReason section, got: %q", directive)
	}
	if strings.Contains(directive, "<deliverable_contract>") {
		t.Errorf("baseline directive must not contain contract tag, got: %q", directive)
	}
}

func TestAcceptanceCriteriaFor_ContainsVerifyBar(t *testing.T) {
	crit := workmodel.AcceptanceCriteriaForContract(workmodel.DefaultTestDeliverableContract())
	if crit == "" {
		t.Fatal("AcceptanceCriteriaForContract returned empty")
	}
	for _, want := range []string{"citation=file_line", "severity=p0_p1", "reject=planning_meta"} {
		if !strings.Contains(crit, want) {
			t.Errorf("criteria missing %q, got: %q", want, crit)
		}
	}
	if unknown := workmodel.AcceptanceCriteriaForContract(workmodel.DeliverableContract{}); unknown != "" {
		t.Errorf("empty contract must return empty criteria, got %q", unknown)
	}
}

var _ orchtypes.LLMInvoker = (*scriptedLLM)(nil)
