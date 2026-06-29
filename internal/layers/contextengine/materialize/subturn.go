package materialize

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/prepare/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	SubTurnBrief = "brief"
	SubTurnFork  = "fork"
	SubTurnFull  = "full"
)

// ModeFork selects cache-friendly fork prefix composition (D7-S16-A65).
const ModeFork Mode = "fork"

// PolicyFromSubTurnMode maps context-budget SubTurn modes to Materialize policy (DM-20260627-003 Phase 3 T33).
func PolicyFromSubTurnMode(mode string, tokenBudget int) Policy {
	p := Policy{TokenBudget: tokenBudget}
	switch strings.TrimSpace(mode) {
	case SubTurnFork:
		p.Mode = ModeFork
	case SubTurnFull:
		p.Mode = ModeResume
	default:
		p.Mode = ModeFresh
	}
	return p
}

// ComposeSubTurnMessages mirrors SubTurnRunner.applyMode for the Materialize path.
func ComposeSubTurnMessages(mode string, parent []types.Message) (preloaded []types.Message, lastUser types.Message) {
	lastUser = lastUserMessage(parent)
	switch strings.TrimSpace(mode) {
	case SubTurnFork:
		directive := lastUser.Content
		return conversation.BuildForkedMessages(directive, messagesWithoutLastUser(parent)), lastUser
	case SubTurnFull:
		return messagesWithoutLastUser(parent), lastUser
	default:
		return nil, lastUser
	}
}

func lastUserMessage(msgs []types.Message) types.Message {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleUser {
			return msgs[i]
		}
	}
	if len(msgs) > 0 {
		return msgs[len(msgs)-1]
	}
	return types.Message{}
}

func messagesWithoutLastUser(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return nil
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.MessageRoleUser {
			if i == 0 {
				return nil
			}
			out := make([]types.Message, i)
			copy(out, msgs[:i])
			return out
		}
	}
	return msgs
}
