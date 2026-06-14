package conversation

import (
	"fmt"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

const (
	// MetaCompactBoundary marks a system message as a compaction boundary.
	MetaCompactBoundary = "compact_boundary"
	// MetaMessagesSummarized stores how many messages were folded at this boundary.
	MetaMessagesSummarized = "messages_summarized"
)

// IsCompactBoundary reports whether m is a compaction boundary marker.
func IsCompactBoundary(m types.Message) bool {
	if m.Role != types.MessageRoleSystem || m.Metadata == nil {
		return false
	}
	return m.Metadata[MetaCompactBoundary] != ""
}

// MessagesAfterCompactBoundary returns messages from the last compaction boundary onward.
// When no boundary exists, the full slice is returned.
func MessagesAfterCompactBoundary(msgs []types.Message) []types.Message {
	if len(msgs) == 0 {
		return msgs
	}
	last := -1
	for i, m := range msgs {
		if IsCompactBoundary(m) {
			last = i
		}
	}
	if last < 0 {
		out := make([]types.Message, len(msgs))
		copy(out, msgs)
		return out
	}
	out := make([]types.Message, len(msgs)-last)
	copy(out, msgs[last:])
	return out
}

// NewCompactBoundaryMessage creates a system marker for a compaction event.
func NewCompactBoundaryMessage(sessionID, trigger string, messagesSummarized int) types.Message {
	meta := map[string]string{
		MetaCompactBoundary:      trigger,
		MetaMessagesSummarized: fmt.Sprintf("%d", messagesSummarized),
	}
	return types.Message{
		ID:        fmt.Sprintf("compact_%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Role:      types.MessageRoleSystem,
		Content:   "Conversation compacted",
		Metadata:  meta,
		Timestamp: time.Now(),
	}
}

// MarkSnipAsCompactBoundary tags a head+tail snip placeholder as a boundary.
func MarkSnipAsCompactBoundary(m *types.Message, messagesSummarized int) {
	if m == nil || m.Role != types.MessageRoleSystem {
		return
	}
	if m.Metadata == nil {
		m.Metadata = map[string]string{}
	}
	m.Metadata[MetaCompactBoundary] = "snip"
	m.Metadata[MetaMessagesSummarized] = fmt.Sprintf("%d", messagesSummarized)
}
