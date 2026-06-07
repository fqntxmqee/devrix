package compression_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-03, L5-CTX-04
func TestPipeline_should_compress_when_over_target(t *testing.T) {
	p := compression.NewPipeline(true)
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 10

	var msgs []types.Message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, *types.NewMessage("m", "s", types.MessageRoleUser, strings.Repeat("word ", 50)))
	}

	out, report, err := p.Run(context.Background(), msgs, "sys", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.StepsApplied) == 0 {
		t.Error("expected compression steps applied")
	}
	if len(out) == 0 {
		t.Error("expected output messages")
	}
}

// Covers: L5-CTX-04
func TestPipeline_should_return_context_exceeded_when_still_over_budget(t *testing.T) {
	p := compression.NewPipeline(true)
	budget := types.TokenBudget{
		MaxContextTokens:  50,
		ReservedOutput:    10,
		ToolResultBudget:  800,
		CompressionTarget: 5,
	}

	msgs := []types.Message{
		*types.NewMessage("m1", "s", types.MessageRoleUser, strings.Repeat("x", 1000)),
	}

	_, _, err := p.Run(context.Background(), msgs, strings.Repeat("y", 1000), budget)
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.ErrorCode(err) != errors.CodeContextExceeded {
		t.Errorf("expected CTX_EXCEEDED, got %v", err)
	}
}

// Covers: L5-CTX-08
func TestPipeline_should_skip_autocompact_in_v1(t *testing.T) {
	p := compression.NewPipeline(true)
	budget := types.DefaultTokenBudget()
	msgs := []types.Message{*types.NewMessage("m", "s", types.MessageRoleUser, "hi")}
	_, report, err := p.Run(context.Background(), msgs, "", budget)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, s := range report.StepsApplied {
		if strings.Contains(s, "autocompact") {
			found = true
		}
	}
	if !found {
		t.Error("expected autocompact skip logged")
	}
}
