package materialize

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/types"
)

// buildWaveSystemPrompt formats the per-wave system prompt, optionally
// appending the WaveExtraPrompt hint, WaveFileScope list, ModeUpstream
// signals (line-by-line), WaveUpstreamFiles, and WaveUpstreamError.
//
// DM-20260629-002 devrix-d2-dsaft-restructuring PR-3: extracted from
// materializer.go (was 45 LOC; total Materialize area was 30+34+32 LOC).
func buildWaveSystemPrompt(req Request) string {
	loc := i18n.ParseLanguage(req.Policy.Locale)
	labels := i18n.WavePromptLabelsFor(loc)

	var b strings.Builder
	if base := strings.TrimSpace(req.SystemPrompt); base != "" {
		b.WriteString(base)
	}
	if extra := strings.TrimSpace(req.Signals.WaveExtraPrompt); extra != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(extra)
	}
	if len(req.Signals.WaveFileScope) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(labels.AllowedFileScope)
		b.WriteString("\n- ")
		b.WriteString(strings.Join(req.Signals.WaveFileScope, "\n- "))
	}
	if req.Policy.Mode == ModeUpstream {
		for _, line := range req.Signals.SignalLines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(line)
		}
		if len(req.Signals.WaveUpstreamFiles) > 0 {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(labels.FilesChangedUpstream)
			b.WriteString("\n- ")
			b.WriteString(strings.Join(req.Signals.WaveUpstreamFiles, "\n- "))
		}
		if errMsg := strings.TrimSpace(req.Signals.WaveUpstreamError); errMsg != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(labels.UpstreamErrorPrefix)
			b.WriteString(errMsg)
		}
	}
	return b.String()
}

// buildWorkItemSystemBody formats the WorkItem context block (intro, directive,
// scopes, signals) without output format hints. MUPS Execute prepends devrix_core
// via PrepareBase and appends hints separately to avoid duplication.
func buildWorkItemSystemBody(req Request) string {
	loc := i18n.ParseLanguage(req.Policy.Locale)
	labels := i18n.WorkItemExecuteFieldLabels

	var b strings.Builder
	b.WriteString(i18n.WorkItemExecuteIntro(loc))
	b.WriteByte('\n')
	if req.Signals.Directive != "" {
		b.WriteString("\n")
		b.WriteString(labels.Directive)
		b.WriteString(": ")
		b.WriteString(req.Signals.Directive)
		b.WriteByte('\n')
	}
	if len(req.Signals.ScopeIn) > 0 {
		b.WriteString("\n")
		b.WriteString(labels.ScopeIn)
		b.WriteString(":\n")
		for _, p := range req.Signals.ScopeIn {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	if len(req.Signals.ScopeOut) > 0 {
		b.WriteString("\n")
		b.WriteString(labels.ScopeOut)
		b.WriteString(":\n")
		for _, p := range req.Signals.ScopeOut {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteByte('\n')
		}
	}
	if req.Signals.ExpectedReturn != "" {
		b.WriteString("\n")
		b.WriteString(labels.ExpectedReturn)
		b.WriteString(": ")
		b.WriteString(req.Signals.ExpectedReturn)
		b.WriteByte('\n')
	}
	for _, line := range req.Signals.SignalLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(line)
	}
	return b.String()
}

// buildSystemPrompt formats the per-WorkItem system prompt including
// directive, scope-in/scope-out lists, expected return, and signal lines,
// followed by locale-aware output format hints.
//
// DM-20260629-002 PR-3: extracted from materializer.go (was 39 LOC).
func buildSystemPrompt(req Request) string {
	loc := i18n.ParseLanguage(req.Policy.Locale)
	body := buildWorkItemSystemBody(req)
	if body == "" {
		return workItemOutputHints(loc)
	}
	return body + workItemOutputHints(loc)
}

func workItemOutputHints(loc i18n.Locale) string {
	return i18n.WorkItemExecuteOutputHints(loc)
}

// buildInitialMessages builds the user-role opening message for the partition.
// Returns nil when the directive is empty (caller treats nil as "no opening
// user message, defer to system prompt only").
//
// DM-20260629-002 PR-3: extracted from materializer.go (was 9 LOC).
func buildInitialMessages(req Request) []types.Message {
	if strings.TrimSpace(req.Signals.Directive) == "" {
		return nil
	}
	return []types.Message{{
		SessionID: req.Partition.SessionID,
		Role:      types.MessageRoleUser,
		Content:   req.Signals.Directive,
	}}
}

// mergeInitialWithPrivateChain joins the fresh user directive with persisted
// private-chain turns without duplicating the opening user message when a
// prior ExecuteWorkItem round already persisted it.
func mergeInitialWithPrivateChain(initial, priv []types.Message) []types.Message {
	if len(priv) == 0 {
		return initial
	}
	if len(initial) == 1 &&
		initial[0].Role == types.MessageRoleUser &&
		priv[0].Role == types.MessageRoleUser &&
		initial[0].Content == priv[0].Content {
		out := make([]types.Message, 0, len(priv))
		out = append(out, initial[0])
		out = append(out, priv[1:]...)
		return out
	}
	out := make([]types.Message, 0, len(initial)+len(priv))
	out = append(out, initial...)
	out = append(out, priv...)
	return out
}
