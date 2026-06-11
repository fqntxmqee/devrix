package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/contextengine/conversation"
	"github.com/devrix/devrix/internal/shared/types"
)

const (
	// ForkPlaceholderResult is identical for all fork children (prompt cache sharing).
	ForkPlaceholderResult = "Fork started — processing in background"
	forkBoilerplateTag    = "fork-worker-context"
	forkDirectivePrefix   = "Your assigned directive:\n"
)

// BuildForkedMessages constructs cache-friendly fork prefix messages (CC forkSubagent aligned).
// Returns [assistant(all tool_uses), user(placeholder tool_results + directive)].
func BuildForkedMessages(directive string, parentMessages []types.Message) []types.Message {
	assistant, refs, ok := findLastAssistantWithToolCalls(parentMessages)
	if !ok || len(refs) == 0 {
		return []types.Message{buildForkDirectiveUser(directive, nil)}
	}

	cloned := cloneAssistantMessage(assistant)
	out := []types.Message{cloned, buildForkDirectiveUser(directive, refs)}
	return out
}

// BuildChildDirective renders the per-child fork directive block.
func BuildChildDirective(directive string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<%s>\n", forkBoilerplateTag)
	b.WriteString("You are a forked worker process. Execute the directive directly; do not spawn sub-agents.\n")
	fmt.Fprintf(&b, "</%s>\n\n", forkBoilerplateTag)
	b.WriteString(forkDirectivePrefix)
	b.WriteString(strings.TrimSpace(directive))
	return b.String()
}

// IsInForkChild reports whether messages contain fork boilerplate (blocks recursive fork).
func IsInForkChild(msgs []types.Message) bool {
	tag := fmt.Sprintf("<%s>", forkBoilerplateTag)
	for _, m := range msgs {
		if m.Role == types.MessageRoleUser && strings.Contains(m.Content, tag) {
			return true
		}
	}
	return false
}

func findLastAssistantWithToolCalls(msgs []types.Message) (types.Message, []conversation.ToolCallRef, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != types.MessageRoleAssistant {
			continue
		}
		refs := conversation.ToolCallsFromAssistant(m)
		if len(refs) > 0 {
			return m, refs, true
		}
	}
	return types.Message{}, nil, false
}

func cloneAssistantMessage(m types.Message) types.Message {
	out := m
	out.ID = fmt.Sprintf("fork_asst_%d", time.Now().UnixNano())
	if out.Metadata != nil {
		meta := make(map[string]string, len(out.Metadata))
		for k, v := range out.Metadata {
			meta[k] = v
		}
		out.Metadata = meta
	}
	return out
}

func buildForkDirectiveUser(directive string, refs []conversation.ToolCallRef) types.Message {
	var b strings.Builder
	for _, ref := range refs {
		fmt.Fprintf(&b, "[tool_result id=%s]\n%s\n", ref.ID, ForkPlaceholderResult)
	}
	b.WriteString(BuildChildDirective(directive))
	msg := types.Message{
		ID:        fmt.Sprintf("fork_user_%d", time.Now().UnixNano()),
		Role:      types.MessageRoleUser,
		Content:   b.String(),
		Timestamp: time.Now(),
		Metadata:  map[string]string{conversation.MetaIsMeta: "true"},
	}
	return msg
}

// ForkPrefixFingerprint returns stable prefix bytes shared across fork children (for cache tests).
func ForkPrefixFingerprint(msgs []types.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range msgs {
		if m.Role == types.MessageRoleAssistant {
			b.WriteString(m.Content)
			if raw := m.Metadata[conversation.MetaToolCalls]; raw != "" {
				b.WriteString(raw)
			}
		}
		if m.Role == types.MessageRoleTool || (m.Role == types.MessageRoleUser && strings.Contains(m.Content, ForkPlaceholderResult)) {
			b.WriteString(ForkPlaceholderResult)
		}
	}
	return b.String()
}
