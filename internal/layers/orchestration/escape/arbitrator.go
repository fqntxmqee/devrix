// ChainedArbitrator + 3 层仲裁 (DM-20260625-003, PR-V5.3)
//
// 关键设计 (doc 38 §21.3.4, design.md §5.3):
//
//   3 层链式调度 (LLM → Rule → Human):
//   - LLMArbitrator  (5s timeout, 1 次 retry, panic recover, ctx.Done() 优先)
//   - RuleArbitrator (规则检查, hasUnrecoverableFailure)
//   - HumanArbitrator (异步注册 + 立即返回 EscapePendingHuman)
//
//   ChainedArbitrator.Arbitrate 关键不变式:
//   1. 返回值 Action ∈ {EscapeContinue, EscapePendingHuman, EscapeForceExit, EscapeAbortWithAudit}
//   2. EscalateToRule / EscalateToHuman 中间态绝不返回给 caller
//   3. 任何一层 panic / error → 降级到下一层（不阻塞主链路）
//
//   HumanArbitrator 异步化（采纳 Phase 7 Auto-Close 模式）:
//   - Arbitrate 立即返回 EscapePendingHuman（ProcessMessage 同步返回）
//   - 后台 goroutine 等待 user 响应 / 10s timeout / ctx.Done()
//   - ResumeSession 由 EscapeEngine.ResumeSession 委托调用
package escape

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// LLMClient is the minimal interface LLMArbitrator needs for a one-shot
// prompt → response call (no streaming). Production wires this to the
// D3 LLM gateway; tests inject a mock.
type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// EscapeArbitrator is the common interface for all 3 layers.
// Each layer inspects the upstream decisions and emits its own
// EscapeDecision (Continue / EscalateTo* / ForceExit / AbortWithAudit).
type EscapeArbitrator interface {
	Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) (EscapeDecision, error)
}

// ChainedArbitrator chains the 3 layers in fixed order (LLM → Rule → Human).
//
// Layer responsibilities (see design.md §5.3 for the full flow):
//   - LLMArbitrator: ask the LLM whether to Continue or Exit (5s timeout).
//     Continue → return Continue; Exit → EscalateToRule; timeout/error → EscalateToRule.
//   - RuleArbitrator: check hasUnrecoverableFailure.
//     Unrecoverable → AbortWithAudit; recoverable → EscalateToHuman.
//   - HumanArbitrator: async, returns EscapePendingHuman immediately.
//
// The chain DECIDES which layer's decision is final; intermediate
// EscalateTo* values are NEVER returned to the caller.
type ChainedArbitrator struct {
	llm   EscapeArbitrator
	rule  EscapeArbitrator
	human EscapeArbitrator
}

// NewChainedArbitrator constructs the 3-layer chain.
func NewChainedArbitrator(llm, rule, human EscapeArbitrator) *ChainedArbitrator {
	return &ChainedArbitrator{
		llm:   llm,
		rule:  rule,
		human: human,
	}
}

// Arbitrate chains the 3 layers and returns the final decision.
//
// Chain logic (matches design.md §5.3 + review-r3 ISSUE-2):
//   - LLM Continue       → return immediately (depth-budget check passed)
//   - LLM Exit/timeout   → escalate to Rule
//   - Rule AbortWithAudit→ return immediately (irrecoverable)
//   - Rule Human         → escalate to Human (always async path)
//   - Human              → returns EscapePendingHuman synchronously
//
// Errors from any layer are logged and the chain proceeds to the next
// layer (or ForceExit if it's the last layer).
func (c *ChainedArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) EscapeDecision {
	chain := append([]EscapeDecision{}, decisions...)

	// Layer 1: LLM
	llmDecision, err := c.safeArbitrate(ctx, c.llm, loopCtx, chain, "llm")
	if err == nil {
		switch llmDecision.Action {
		case EscapeContinue:
			return llmDecision
		case EscapeForceExit, EscapeAbortWithAudit:
			return llmDecision
		case EscalateToRule:
			chain = append(chain, llmDecision)
		case EscalateToHuman:
			// LLM shouldn't escalate directly to Human; defensive fallback.
			slog.Warn("chained_arbitrator_llm_escalated_to_human",
				"session_id", loopCtx.SessionID)
			chain = append(chain, llmDecision)
		case EscapePendingHuman:
			// LLM shouldn't return pending; defensive fallback.
			slog.Warn("chained_arbitrator_llm_returned_pending",
				"session_id", loopCtx.SessionID)
			chain = append(chain, llmDecision)
		default:
			slog.Warn("chained_arbitrator_unknown_llm_action",
				"action", llmDecision.Action.String(),
				"session_id", loopCtx.SessionID)
		}
	} else {
		// LLM error → log and force-exit (per design §5.3 兜底)
		slog.Warn("chained_arbitrator_llm_error_force_exit",
			"session_id", loopCtx.SessionID,
			"error", err.Error(),
		)
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "llm_error_" + err.Error(),
			AuditLevel: 1,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	// Layer 2: Rule
	ruleDecision, err := c.safeArbitrate(ctx, c.rule, loopCtx, chain, "rule")
	if err == nil {
		switch ruleDecision.Action {
		case EscapeContinue, EscapeForceExit, EscapeAbortWithAudit:
			return ruleDecision
		case EscalateToHuman:
			chain = append(chain, ruleDecision)
		case EscalateToRule:
			slog.Warn("chained_arbitrator_rule_escalated_to_rule",
				"session_id", loopCtx.SessionID)
		case EscapePendingHuman:
			slog.Warn("chained_arbitrator_rule_returned_pending",
				"session_id", loopCtx.SessionID)
		default:
			slog.Warn("chained_arbitrator_unknown_rule_action",
				"action", ruleDecision.Action.String(),
				"session_id", loopCtx.SessionID)
		}
	} else {
		slog.Warn("chained_arbitrator_rule_error_force_exit",
			"session_id", loopCtx.SessionID,
			"error", err.Error(),
		)
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "rule_error_" + err.Error(),
			AuditLevel: 1,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	// Layer 3: Human (always async, returns EscapePendingHuman synchronously)
	humanDecision, err := c.safeArbitrate(ctx, c.human, loopCtx, chain, "human")
	if err != nil {
		slog.Warn("chained_arbitrator_human_error_force_exit",
			"session_id", loopCtx.SessionID,
			"error", err.Error(),
		)
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "human_error_" + err.Error(),
			AuditLevel: 1,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}
	return humanDecision
}

// safeArbitrate runs a layer with panic recovery.
func (c *ChainedArbitrator) safeArbitrate(ctx context.Context, layer EscapeArbitrator, loopCtx LoopContext, decisions []EscapeDecision, name string) (decision EscapeDecision, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("chained_arbitrator_panic",
				"layer", name,
				"panic", r,
				"session_id", loopCtx.SessionID,
			)
			err = fmt.Errorf("panic in %s arbitrator: %v", name, r)
		}
	}()
	return layer.Arbitrate(ctx, loopCtx, decisions)
}

// --- LLM Arbitrator ---------------------------------------------------------

// LLMArbitrator prompts the LLM and parses {action: Continue|Exit, reason}.
//
// Failure modes (per design.md §5.3.3):
//   - panic → recover + log + return ForceExit (caller escalates)
//   - ctx cancelled → ForceExit with reason="ctx_cancelled"
//   - 5s timeout → ForceExit with reason="llm_timeout_5s"
//   - 6s double-safety timer → ForceExit with reason="llm_stuck_force_exit"
//   - JSON parse fail → 1 retry with format hint, then ForceExit
//   - action != "Continue"|"Exit" → ForceExit (rejected early, no escalation)
type LLMArbitrator struct {
	llmClient LLMClient
	timeout   time.Duration
	promptFn  func(loopCtx LoopContext, decisions []EscapeDecision) string
	parseFn   func(resp string) (action string, reason string, err error)
}

// NewLLMArbitrator constructs an LLMArbitrator with default 5s timeout.
// promptFn / parseFn may be nil to use the defaults.
func NewLLMArbitrator(client LLMClient) *LLMArbitrator {
	return &LLMArbitrator{
		llmClient: client,
		timeout:   5 * time.Second,
	}
}

// SetTimeout overrides the default 5s LLM timeout (test hook).
func (a *LLMArbitrator) SetTimeout(d time.Duration) {
	a.timeout = d
}

// SetPromptFn injects a custom prompt builder (test hook). nil → default.
func (a *LLMArbitrator) SetPromptFn(fn func(loopCtx LoopContext, decisions []EscapeDecision) string) {
	a.promptFn = fn
}

// SetParseFn injects a custom response parser (test hook). nil → default.
func (a *LLMArbitrator) SetParseFn(fn func(resp string) (string, string, error)) {
	a.parseFn = fn
}

// Arbitrate runs the LLM and parses the response.
func (a *LLMArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) (EscapeDecision, error) {
	llmCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	prompt := a.buildPrompt(loopCtx, decisions)

	// Channel for goroutine result (buffered=1 to prevent leak).
	type result struct {
		resp string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := a.llmClient.Generate(llmCtx, prompt)
		resCh <- result{resp: resp, err: err}
	}()

	var rawResp string
	var llmErr error
	select {
	case r := <-resCh:
		rawResp, llmErr = r.resp, r.err
	case <-llmCtx.Done():
		if ctx.Err() != nil {
			return EscapeDecision{
				Action:     EscapeForceExit,
				Reason:     "ctx_cancelled",
				AuditLevel: 2,
				SessionID:  loopCtx.SessionID,
				CreatedAt:  nowFunc(),
			}, ctx.Err()
		}
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "llm_timeout_5s",
			AuditLevel: 2,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, llmCtx.Err()
	case <-time.After(a.timeout + time.Second):
		// Double-safety: ctx already done but LLM didn't honor it.
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "llm_stuck_force_exit",
			AuditLevel: 2,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, nil
	}

	if llmErr != nil {
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "llm_error",
			AuditLevel: 1,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, llmErr
	}

	// Parse response.
	action, reason, parseErr := a.parseResponse(rawResp)
	if parseErr != nil {
		// 1 retry with format hint.
		retryPrompt := prompt + "\n\n必须返回 JSON: {\"action\":\"Continue|Exit\",\"reason\":\"...\"}"
		var retryResp string
		var retryErr error
		done := make(chan struct{})
		go func() {
			retryResp, retryErr = a.llmClient.Generate(llmCtx, retryPrompt)
			close(done)
		}()
		select {
		case <-done:
		case <-llmCtx.Done():
			return EscapeDecision{
				Action:     EscapeForceExit,
				Reason:     "ctx_cancelled_during_retry",
				AuditLevel: 2,
				SessionID:  loopCtx.SessionID,
				CreatedAt:  nowFunc(),
			}, llmCtx.Err()
		}
		if retryErr != nil {
			return EscapeDecision{
				Action:     EscapeForceExit,
				Reason:     "llm_non_json_after_retry",
				AuditLevel: 1,
				SessionID:  loopCtx.SessionID,
				CreatedAt:  nowFunc(),
			}, retryErr
		}
		action, reason, parseErr = a.parseResponse(retryResp)
		if parseErr != nil {
			return EscapeDecision{
				Action:     EscapeForceExit,
				Reason:     "llm_invalid_format",
				AuditLevel: 1,
				SessionID:  loopCtx.SessionID,
				CreatedAt:  nowFunc(),
			}, parseErr
		}
	}

	// Validate action.
	if action != "Continue" && action != "Exit" {
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "llm_invalid_action_" + action,
			AuditLevel: 1,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, nil
	}

	if action == "Continue" {
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     reason,
			AuditLevel: 0,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, nil
	}
	// action == "Exit" → escalate to Rule layer.
	return EscapeDecision{
		Action:     EscalateToRule,
		Reason:     reason,
		AuditLevel: 1,
		SessionID:  loopCtx.SessionID,
		CreatedAt:  nowFunc(),
	}, nil
}

// buildPrompt constructs the LLM prompt (default impl; injectable).
func (a *LLMArbitrator) buildPrompt(loopCtx LoopContext, decisions []EscapeDecision) string {
	if a.promptFn != nil {
		return a.promptFn(loopCtx, decisions)
	}
	return defaultLLMPrompt(loopCtx, decisions)
}

// parseResponse parses the LLM JSON response (default impl; injectable).
func (a *LLMArbitrator) parseResponse(resp string) (string, string, error) {
	if a.parseFn != nil {
		return a.parseFn(resp)
	}
	return defaultParseLLMResponse(resp)
}

func defaultLLMPrompt(loopCtx LoopContext, decisions []EscapeDecision) string {
	return fmt.Sprintf(`回路深度已超过上限。请判断:

LoopContext: %+v
Decisions: %+v

返回 JSON 格式: {"action":"Continue|Exit","reason":"..."}`, loopCtx, decisions)
}

func defaultParseLLMResponse(resp string) (string, string, error) {
	var parsed struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return "", "", err
	}
	return parsed.Action, parsed.Reason, nil
}

// --- Rule Arbitrator ---------------------------------------------------------

// RuleArbitrator checks for unrecoverable failures. If found → AbortWithAudit;
// otherwise → EscalateToHuman.
//
// hasUnrecoverableFailure is the only decision function; injected at
// construction so the test can simulate any rule outcome.
type RuleArbitrator struct {
	hasUnrecoverableFailure func(loopCtx LoopContext, decisions []EscapeDecision) bool
}

// NewRuleArbitrator constructs a RuleArbitrator with the given failure check.
func NewRuleArbitrator(hasUnrecoverableFailure func(loopCtx LoopContext, decisions []EscapeDecision) bool) *RuleArbitrator {
	return &RuleArbitrator{hasUnrecoverableFailure: hasUnrecoverableFailure}
}

// Arbitrate runs the rule check.
func (r *RuleArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) (EscapeDecision, error) {
	if r.hasUnrecoverableFailure != nil && r.hasUnrecoverableFailure(loopCtx, decisions) {
		return EscapeDecision{
			Action:     EscapeAbortWithAudit,
			Reason:     "rule_unrecoverable_failure",
			AuditLevel: 2,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}, nil
	}
	return EscapeDecision{
		Action:     EscalateToHuman,
		Reason:     "rule_recoverable_escalate_human",
		AuditLevel: 1,
		SessionID:  loopCtx.SessionID,
		CreatedAt:  nowFunc(),
	}, nil
}

// --- Human Arbitrator --------------------------------------------------------

// HumanArbitrator runs the async user-decision flow.
//
// ProcessMessage semantics:
//   - Arbitrate returns EscapePendingHuman immediately (synchronous, unblocks caller)
//   - Background goroutine waits for: user choice, 10s timeout, or ctx.Done()
//   - Final decision is persisted via PendingResolutionStore
//   - T2 ProcessMessage calls ResumeSession to consume the saved decision
//
// UI consistency (design §5.3.3):
//   - ctx.Done() path calls notifier.SubmitOverrideCard to overwrite the card
//   - SubmitUserChoice after expiry still writes audit (late-response record)
type HumanArbitrator struct {
	timeout  time.Duration
	notifier EscapeDecisionNotifier
	audit    *EscapeAuditLog
	resume   PendingResolutionStore

	mu      sync.Mutex
	pending map[string]chan UserChoice // pendingID → buffered channel
}

// NewHumanArbitrator constructs a HumanArbitrator.
// audit and resume may be nil for tests; passed nil → no-op.
func NewHumanArbitrator(notifier EscapeDecisionNotifier, audit *EscapeAuditLog, resume PendingResolutionStore) *HumanArbitrator {
	return &HumanArbitrator{
		timeout:  10 * time.Second,
		notifier: notifier,
		audit:    audit,
		resume:   resume,
		pending:  make(map[string]chan UserChoice),
	}
}

// SetTimeout overrides the default 10s human timeout (test hook).
func (a *HumanArbitrator) SetTimeout(d time.Duration) {
	a.timeout = d
}

// Arbitrate registers a pending decision and returns EscapePendingHuman.
//
// The pendingID is a UUID v4 (consistent with NewPendingID in types.go).
// Returns immediately to avoid blocking ProcessMessage (feishu card UX).
func (a *HumanArbitrator) Arbitrate(ctx context.Context, loopCtx LoopContext, decisions []EscapeDecision) (EscapeDecision, error) {
	pendingID := NewPendingID()
	userInputCh := make(chan UserChoice, 1) // buffered=1 prevents goroutine leak

	a.mu.Lock()
	a.pending[pendingID] = userInputCh
	a.mu.Unlock()

	// Async notify user (best-effort; errors logged via ChainedNotifier fallback).
	if a.notifier != nil {
		go func() {
			nCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := a.notifier.Notify(nCtx, loopCtx, pendingID, decisions); err != nil {
				slog.Warn("human_arbitrator_notify_failed",
					"pending_id", pendingID,
					"session_id", loopCtx.SessionID,
					"error", err.Error(),
				)
			}
		}()
	}

	// Background goroutine: wait for user input / timeout / ctx cancellation.
	go a.waitForUserResponse(ctx, pendingID, userInputCh, loopCtx, decisions)

	return EscapeDecision{
		Action:     EscapePendingHuman,
		Reason:     "human_review_required",
		AuditLevel: 1,
		PendingID:  pendingID,
		SessionID:  loopCtx.SessionID,
		CreatedAt:  nowFunc(),
	}, nil
}

// waitForUserResponse blocks until one of three paths resolves.
func (a *HumanArbitrator) waitForUserResponse(ctx context.Context, pendingID string, ch chan UserChoice, loopCtx LoopContext, decisions []EscapeDecision) {
	defer a.cleanupPending(pendingID)

	timer := time.NewTimer(a.timeout)
	defer timer.Stop()

	var finalDecision EscapeDecision
	select {
	case choice := <-ch:
		// Path 1: user choice received
		finalDecision = mapToEscapeDecision(choice, pendingID, loopCtx.SessionID)
	case <-timer.C:
		// Path 2: 10s timeout → ForceExit
		finalDecision = EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "human_timeout_10s",
			AuditLevel: 2,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
	case <-ctx.Done():
		// Path 3: ctx cancelled → ForceExit + UI override
		finalDecision = EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "ctx_cancelled",
			AuditLevel: 2,
			SessionID:  loopCtx.SessionID,
			CreatedAt:  nowFunc(),
		}
		// Overwrite the user-facing card (UI consistency, design §5.3.3).
		if a.notifier != nil {
			if override, ok := a.notifier.(OverrideCardNotifier); ok {
				_ = override.SubmitOverrideCard(context.Background(), pendingID, "已强制退出（客户端断开）", nil)
			}
		}
	}

	// Persist decision.
	if a.audit != nil {
		a.audit.Record(loopCtx, decisions, finalDecision)
	}
	if a.resume != nil {
		_ = a.resume.Save(loopCtx.SessionID, finalDecision)
	}
}

// cleanupPending removes the pending entry (idempotent).
func (a *HumanArbitrator) cleanupPending(pendingID string) {
	a.mu.Lock()
	delete(a.pending, pendingID)
	a.mu.Unlock()
}

// SubmitUserChoice is the entry point for the user-facing button callback.
// Called by feishu card callback / CLI handler / etc.
func (a *HumanArbitrator) SubmitUserChoice(pendingID string, choice UserChoice) {
	a.mu.Lock()
	ch, ok := a.pending[pendingID]
	a.mu.Unlock()

	if !ok {
		// Late response: still write audit (design §5.3.3 H4 fix).
		if a.audit != nil {
			a.audit.Record(LoopContext{}, nil, EscapeDecision{
				Action:     EscapePendingHuman,
				Reason:     "user_late_response",
				PendingID:  pendingID,
				AuditLevel: 1,
				CreatedAt:  nowFunc(),
			})
		}
		return
	}

	// Non-blocking send (channel buffered=1).
	select {
	case ch <- choice:
	default:
		// pending already consumed (timeout/cancel); still write audit.
		if a.audit != nil {
			a.audit.Record(LoopContext{}, nil, EscapeDecision{
				Action:     EscapePendingHuman,
				Reason:     "user_late_response_after_consume",
				PendingID:  pendingID,
				AuditLevel: 1,
				CreatedAt:  nowFunc(),
			})
		}
	}
}

// ResumeSession is the T2 entry point. Loads the saved decision (if any)
// and deletes it (one-shot consumption).
func (a *HumanArbitrator) ResumeSession(sessionID string) (EscapeDecision, bool, error) {
	if a.resume == nil {
		return EscapeDecision{}, false, nil
	}
	decision, found, err := a.resume.Load(sessionID)
	if err != nil || !found {
		return EscapeDecision{}, false, err
	}
	// One-shot consumption.
	_ = a.resume.Delete(sessionID)
	return decision, true, nil
}

// mapToEscapeDecision converts a UserChoice into the final EscapeDecision.
func mapToEscapeDecision(choice UserChoice, pendingID string, sessionID string) EscapeDecision {
	switch choice.Value {
	case "A":
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "user_continue",
			AuditLevel: 1,
			PendingID:  pendingID,
			SessionID:  sessionID,
			CreatedAt:  nowFunc(),
		}
	case "B":
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "user_accept",
			AuditLevel: 1,
			PendingID:  pendingID,
			SessionID:  sessionID,
			CreatedAt:  nowFunc(),
		}
	case "C":
		return EscapeDecision{
			Action:     EscapeAbortWithAudit,
			Reason:     "user_cancel",
			AuditLevel: 2,
			PendingID:  pendingID,
			SessionID:  sessionID,
			CreatedAt:  nowFunc(),
		}
	default:
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     "user_invalid_choice",
			AuditLevel: 2,
			PendingID:  pendingID,
			SessionID:  sessionID,
			CreatedAt:  nowFunc(),
		}
	}
}