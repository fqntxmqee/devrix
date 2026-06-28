package materialize

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

// compressMessages keeps the tail of `msgs` such that the total token count
// fits within `budget`. When even a single message overflows, the last
// message is truncated to fit. Pure tail-preserve strategy (no semantic
// summarization) — the WorkItem partition store already contains the
// full prior turns.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from
// materializer.go (was 24 LOC).
func compressMessages(msgs []types.Message, budget int) []types.Message {
	if budget <= 0 || len(msgs) == 0 {
		return msgs
	}
	counter := token.NewCounter()
	total := counter.CountMessages(msgs)
	if total <= budget {
		return msgs
	}
	// Keep last messages until budget — simple tail preserve.
	out := make([]types.Message, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		trial := append([]types.Message{msgs[i]}, out...)
		if counter.CountMessages(trial) > budget && len(out) > 0 {
			break
		}
		out = trial
	}
	if len(out) == 0 && len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		last.Content = counter.TruncateToTokens(last.Content, budget)
		out = []types.Message{last}
	}
	return out
}

// toolsForProfile returns the readonly ToolDescriptor set when the WorkItem's
// ToolProfile is "readonly"; otherwise nil (executor uses the full tool set
// from the ContextPreparer fallback merge).
//
// DM-20260629-002 PR-3: extracted from materializer.go (was 13 LOC).
func toolsForProfile(profile string) []ToolDescriptor {
	switch profile {
	case "implement", "":
		return nil // executor uses full tool set from ContextPreparer fallback merge
	case "readonly":
		return []ToolDescriptor{
			{Name: "read_file", Description: "Read file contents"},
			{Name: "grep", Description: "Search codebase"},
		}
	default:
		return nil
	}
}