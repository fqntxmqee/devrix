// Package audit provides lightweight, in-band token-budget diagnostics
// for the prepare layer.
//
// token_audit.go implements DM-20260620-001 / AC4 + AC13: every turn
// iteration audits the candidate messages against the LLM context
// budget and decides whether a proactive fold is warranted before the
// LLM is invoked.
//
// DSAFT: D2-S15-A08 extension.
package audit

import (
	"github.com/devrix/devrix/internal/layers/contextengine/prepare/token"
	"github.com/devrix/devrix/internal/shared/types"
)

// DefaultProactiveFoldPercent is the fraction of the budget above
// which the orchestrator proactively folds the largest assistant
// message before invoking the LLM. 0.6 mirrors clawcode's
// pre-compact threshold.
const DefaultProactiveFoldPercent = 0.6

// AuditResult is a snapshot of the message budget for one turn.
type AuditResult struct {
	TotalTokens      int
	SystemTokens     int
	MessagesTokens   int
	OverBudget       bool
	BudgetPercent    float64
	LargestMsgTokens int
	LargestMsgIdx    int
}

// AuditMessages computes a token-budget snapshot for systemPrompt +
// msgs against budget. The counter is expected to be a token.Counter
// (heuristic char/4) — we keep it as an interface so callers can swap
// in a provider-specific encoding in future.
//
// budget <= 0 disables budget tracking (returns zeros for budget fields).
func AuditMessages(counter *token.Counter, systemPrompt string, msgs []types.Message, budget int) AuditResult {
	if counter == nil {
		counter = token.NewCounter()
	}
	res := AuditResult{}
	res.SystemTokens = counter.CountText(systemPrompt)
	for i, m := range msgs {
		t := counter.CountText(m.Content)
		res.MessagesTokens += t
		if t > res.LargestMsgTokens {
			res.LargestMsgTokens = t
			res.LargestMsgIdx = i
		}
	}
	res.TotalTokens = res.SystemTokens + res.MessagesTokens
	if budget > 0 {
		res.BudgetPercent = float64(res.TotalTokens) / float64(budget)
		res.OverBudget = res.TotalTokens > budget
	}
	return res
}

// ShouldFoldProactively reports whether the orchestrator should fold
// the largest in-band message before invoking the LLM. The decision
// combines two signals:
//
//   - OverBudget: total tokens exceed the budget (always fold).
//   - ProactiveFoldPercent: a tunable fraction (default 60%) so we
//     fold early enough to leave headroom for the response + tool
//     round, matching clawcode's pre-compact cadence.
//
// maxAssistantChars > 0 is required; folding a message shorter than the
// cap is wasted work.
func ShouldFoldProactively(r AuditResult, maxAssistantChars int, proactivePercent float64) bool {
	if maxAssistantChars <= 0 {
		return false
	}
	if r.LargestMsgTokens < maxAssistantChars {
		// Largest message is already within the fold threshold — no
		// proactive fold would actually shrink it.
		return false
	}
	if r.OverBudget {
		return true
	}
	if proactivePercent <= 0 {
		proactivePercent = DefaultProactiveFoldPercent
	}
	return r.BudgetPercent >= proactivePercent
}