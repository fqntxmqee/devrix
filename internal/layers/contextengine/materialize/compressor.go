package materialize

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

type messageSegment struct {
	msgs   []types.Message
	tokens int
}

// compressMessages keeps the tail of msgs within budget while preserving
// intact assistant+tool_call rounds. Naive tail truncation drops assistant
// headers but leaves tool results, which RepairToolMessageChain then removes
// entirely — the LLM sees no read_file evidence on synthesis / rollup reload.
func compressMessages(msgs []types.Message, budget int) []types.Message {
	if budget <= 0 || len(msgs) == 0 {
		return msgs
	}
	counter := token.NewCounter()
	if counter.CountMessages(msgs) <= budget {
		return msgs
	}
	segs := toolRoundSegments(msgs, counter)
	out := make([]types.Message, 0, len(msgs))
	used := 0
	for i := len(segs) - 1; i >= 0; i-- {
		seg := segs[i]
		if len(out) == 0 || used+seg.tokens <= budget {
			out = append(seg.msgs, out...)
			used += seg.tokens
			continue
		}
		break
	}
	if len(out) == 0 {
		last := segs[len(segs)-1].msgs
		return truncateMessagesToBudget(last, budget, counter)
	}
	return out
}

func toolRoundSegments(msgs []types.Message, counter *token.Counter) []messageSegment {
	if len(msgs) == 0 {
		return nil
	}
	var segs []messageSegment
	i := 0
	for i < len(msgs) {
		m := msgs[i]
		if m.Role == types.MessageRoleAssistant && len(conversation.ToolCallsFromAssistant(m)) > 0 {
			seg := []types.Message{m}
			pending := map[string]struct{}{}
			for _, c := range conversation.ToolCallsFromAssistant(m) {
				if id := c.ID; id != "" {
					pending[id] = struct{}{}
				}
			}
			i++
			for i < len(msgs) && len(pending) > 0 {
				tm := msgs[i]
				if tm.Role != types.MessageRoleTool {
					break
				}
				id := conversation.ToolCallIDFromResult(tm)
				if id == "" {
					break
				}
				if _, ok := pending[id]; !ok {
					break
				}
				delete(pending, id)
				seg = append(seg, tm)
				i++
			}
			segs = append(segs, messageSegment{msgs: seg, tokens: counter.CountMessages(seg)})
			continue
		}
		one := []types.Message{m}
		segs = append(segs, messageSegment{msgs: one, tokens: counter.CountMessages(one)})
		i++
	}
	return segs
}

func truncateMessagesToBudget(msgs []types.Message, budget int, counter *token.Counter) []types.Message {
	if budget <= 0 || len(msgs) == 0 {
		return msgs
	}
	if counter.CountMessages(msgs) <= budget {
		return append([]types.Message(nil), msgs...)
	}
	last := msgs[len(msgs)-1]
	last.Content = counter.TruncateToTokens(last.Content, budget)
	return []types.Message{last}
}

// toolsForProfile returns the readonly ToolDescriptor set when the WorkItem's
// ToolProfile is "readonly"; otherwise nil (executor uses the full tool set
// from the ContextPreparer fallback merge).
//
// DM-20260629-002 PR-3: extracted from materializer.go (was 13 LOC).
func toolsForProfile(profile string) []ToolDescriptor {
	switch profile {
	case "rollup_synth":
		return []ToolDescriptor{} // synthesis-only: no tools
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
