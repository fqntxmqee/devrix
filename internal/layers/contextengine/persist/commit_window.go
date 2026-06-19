// Package persist — D2-S17-A04 CommitWindow.
//
// CommitWindow trims the session's active message window when message count
// or token count exceeds configured thresholds. The trimmed result is written
// back via SetActiveMessages + TrimMessages so the next Prepare call sees a
// bounded working set.
//
// DSAFT: D2-S17-A04 (CommitWindow)
package persist

import (
	"context"
	"fmt"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

// CommitWindowDeps holds dependencies for the A04 CommitWindow port.
type CommitWindowDeps struct {
	// Store mutates the session context (SetActiveMessages + TrimMessages).
	Store CommitWindowStore
	// Pipeline runs the compression pipeline (typically facade.compressionPipeline).
	Pipeline CommitWindowPipeline
	// MaxMessages caps the active window; messages beyond this trigger compression.
	MaxMessages int
	// ShouldCompress is an optional extra predicate (token-budget aware). When
	// nil, only message-count is checked.
	ShouldCompress func(msgs []types.Message, budget types.TokenBudget) bool
}

// CommitWindowStore mutates the session context after a CommitWindow.
type CommitWindowStore interface {
	SetActiveMessages(sc *types.SessionContext, msgs []types.Message)
	TrimMessages(sc *types.SessionContext)
}

// CommitWindowPipeline runs the compression pipeline for the active window.
type CommitWindowPipeline interface {
	Run(ctx context.Context, msgs []types.Message, systemPrompt string, budget types.TokenBudget) ([]types.Message, types.CompressionReport, error)
}

// RunCommitWindow trims the session's active window when the working set
// exceeds the configured MaxMessages threshold or the optional token-budget
// predicate. Returns the CompressionReport (empty when no compression ran).
func RunCommitWindow(ctx context.Context, deps CommitWindowDeps, sc *types.SessionContext) (types.CompressionReport, error) {
	if sc == nil {
		return types.CompressionReport{}, fmt.Errorf("persist: CommitWindow: session context is nil")
	}
	if deps.Store == nil || deps.Pipeline == nil {
		return types.CompressionReport{}, fmt.Errorf("persist: CommitWindow: missing Store or Pipeline")
	}
	max := deps.MaxMessages
	if max <= 0 {
		max = 50
	}
	active := conversation.RepairToolMessageChain(conversation.MessagesAfterCompactBoundary(sc.Messages))
	overMessages := len(active) > max
	overTokens := deps.ShouldCompress != nil && deps.ShouldCompress(active, sc.TokenBudget)
	if !overMessages && !overTokens {
		return types.CompressionReport{}, nil
	}
	compressed, report, err := deps.Pipeline.Run(ctx, active, "", sc.TokenBudget)
	if err != nil || len(report.StepsApplied) == 0 {
		return report, err
	}
	committed := conversation.RepairToolMessageChain(stripSystemMessageForCommitWindow(compressed))
	deps.Store.SetActiveMessages(sc, committed)
	deps.Store.TrimMessages(sc)
	return report, nil
}

// stripSystemMessageForCommitWindow removes the leading system role from a
// compressed slice. Mirrors facade/engine_compression.go::stripSystemMessage.
func stripSystemMessageForCommitWindow(msgs []types.Message) []types.Message {
	if len(msgs) > 0 && msgs[0].Role == types.MessageRoleSystem {
		return msgs[1:]
	}
	return msgs
}
