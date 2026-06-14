package conversation

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

const snippedMessagesFmt = "[snipped %d messages]"

// HeadTailTrim keeps early user-turn anchors and a recent tail when over maxMessages.
// A system placeholder marks removed middle segments so the model knows history was compressed.
func HeadTailTrim(msgs []types.Message, maxMessages, headTurns, tailMessages int) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	if maxMessages <= 0 {
		maxMessages = 50
	}
	if headTurns <= 0 {
		headTurns = 1
	}
	if tailMessages <= 0 {
		tailMessages = maxMessages - 2
		if tailMessages < 1 {
			tailMessages = 1
		}
	}
	if len(msgs) <= maxMessages {
		out := make([]types.Message, len(msgs))
		copy(out, msgs)
		return out
	}

	headEnd := headCutIndex(msgs, headTurns)
	tailStart := len(msgs) - tailMessages
	if tailStart < headEnd {
		tailStart = headEnd
	}

	for {
		snipped := tailStart - headEnd
		size := headEnd + (len(msgs) - tailStart)
		if snipped > 0 {
			size++
		}
		if size <= maxMessages {
			break
		}
		if tailStart < len(msgs)-1 {
			tailStart++
			continue
		}
		firstUser := firstUserIndex(msgs)
		if headEnd > firstUser+1 {
			headEnd--
			continue
		}
		return append([]types.Message(nil), msgs[len(msgs)-maxMessages:]...)
	}

	snipped := tailStart - headEnd
	if snipped <= 0 {
		return append([]types.Message(nil), msgs[len(msgs)-maxMessages:]...)
	}

	out := make([]types.Message, 0, headEnd+1+(len(msgs)-tailStart))
	out = append(out, msgs[:headEnd]...)
	snip := types.Message{
		Role:    types.MessageRoleSystem,
		Content: fmt.Sprintf(snippedMessagesFmt, snipped),
	}
	MarkSnipAsCompactBoundary(&snip, snipped)
	out = append(out, snip)
	out = append(out, msgs[tailStart:]...)
	return out
}

func headCutIndex(msgs []types.Message, headTurns int) int {
	firstUser := firstUserIndex(msgs)
	if firstUser < 0 {
		return 0
	}
	usersSeen := 0
	for i, m := range msgs {
		if m.Role == types.MessageRoleUser {
			usersSeen++
			if usersSeen >= headTurns {
				return i + 1
			}
		}
	}
	return firstUser + 1
}

func firstUserIndex(msgs []types.Message) int {
	for i, m := range msgs {
		if m.Role == types.MessageRoleUser {
			return i
		}
	}
	return -1
}
