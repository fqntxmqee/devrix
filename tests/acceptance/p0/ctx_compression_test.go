//go:build acceptance

package p0_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/compression"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

// Covers: L5-CTX-03, L5-CTX-04
func TestAcceptance_CompressionPipelineP0(t *testing.T) {
	p := compression.NewPipelineEnabled(true)
	budget := types.DefaultTokenBudget()
	budget.CompressionTarget = 20

	var msgs []types.Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, *types.NewMessage("m", "s", types.MessageRoleUser, strings.Repeat("data ", 100)))
	}

	_, report, err := p.Run(context.Background(), msgs, "system", budget)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}
	if len(report.StepsApplied) == 0 {
		t.Fatal("expected compression steps")
	}
}

// Covers: L5-CTX-04
func TestAcceptance_TokenBlockP0(t *testing.T) {
	p := compression.NewPipelineEnabled(true)
	budget := types.TokenBudget{MaxContextTokens: 30, ReservedOutput: 5, CompressionTarget: 5}

	_, _, err := p.Run(context.Background(),
		[]types.Message{*types.NewMessage("m", "s", types.MessageRoleUser, strings.Repeat("z", 5000))},
		strings.Repeat("s", 5000), budget)
	if errors.ErrorCode(err) != errors.CodeContextExceeded {
		t.Fatalf("expected context exceeded, got %v", err)
	}
}
