package workmodel

import (
	"errors"
	"fmt"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// PrepareDecomposeSpecs ensures round carries capped child specs and passes I4.
func PrepareDecomposeSpecs(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if round == nil {
		return errSpawnRoundRequired
	}
	if len(round.ChildSpecs) == 0 {
		round.ChildSpecs = DefaultDecomposeProposer(item, round)
	}
	workDir := ""
	if tm != nil {
		workDir = tm.SessionWorkDir(sessionID)
	}
	round.ChildSpecs = FilterValidatedChildSpecs(item, round.ChildSpecs, workDir)
	if len(round.ChildSpecs) == 0 {
		round.ChildSpecs = DefaultDecomposeProposer(item, round)
		round.ChildSpecs = FilterValidatedChildSpecs(item, round.ChildSpecs, workDir)
	}
	round.ChildSpecs = CapChildSpecs(round.ChildSpecs)
	return ValidateSpawnDecompose(round)
}

// ApplySpawnPolicy executes spawn side effects after SpawnPolicyEvaluator (Phase C).
func ApplySpawnPolicy(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if round == nil || tm == nil {
		return nil
	}
	switch round.SpawnPolicy {
	case SpawnDecompose:
		if item != nil && !CanDecompose(item.Kind) {
			round.SpawnPolicy = SpawnInline
			round.SpawnRationale = "spawn guard: kind " + string(item.Kind) + " cannot decompose → inline retry"
			return nil
		}
		if err := PrepareDecomposeSpecs(sessionID, item, round, tm); err != nil {
			return err
		}
		_, err := tm.DecomposeChildren(sessionID, item.ID, round.ChildSpecs)
		if err == nil {
			_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
			return nil
		}
		// DM-20260704-006 Phase 4 (D7-S15-A109-T02): budget gate
		// degradation. When DecomposeChildren fails because the
		// parent is at max depth / max children / daily decompose
		// limit, RC-4a-driven decompose degrades gracefully to
		// SpawnInline rather than aborting the session loop. This
		// matches the legacy `execution_mode: "decompose"` behavior
		// (silent inline retry) so callers see no observable
		// regression. Other errors (parent not found, scope
		// validation, etc.) are returned to the caller.
		if isBudgetGateError(err) {
			round.SpawnPolicy = SpawnInline
			round.SpawnRationale = "budget gate: " + err.Error() + " → inline retry (RC-4a graceful degradation)"
			_ = tm.Tree().SetRoundPhase(sessionID, item.ID, RoundPhaseIdle)
			return nil
		}
		return err
	case SpawnEscalateHuman:
		if err := createHumanReviewWorkItem(sessionID, item, round, tm); err != nil {
			return err
		}
		_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
		return nil
	case SpawnUserGate:
		// DM-20260704-006 RC-4b: opens a verify child with
		// ToolFilter=["ask_user_question"] so the LLM cannot bypass the
		// gate via free-form tools (read_file / delegate_explore). Falls
		// through to human review creation on any failure so the user
		// always sees something rather than a silent round.
		if err := createUserGateWorkItem(sessionID, item, round, tm); err != nil {
			if herr := createHumanReviewWorkItem(sessionID, item, round, tm); herr != nil {
				return err
			}
			round.SpawnRationale = round.SpawnRationale + " (user-gate creation failed; escalated to human review)"
		}
		_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
		return nil
	default:
		if round.RollupSynthRequested && item != nil {
			_ = tm.Tree().SetNeedsRollup(sessionID, item.ID, true)
			_ = tm.Tree().ResetInlineRetriesAtMaxDepth(sessionID, item.ID)
			if item.Status == TaskStatusCompleted || item.Status == TaskStatusFailed {
				_ = tm.Tree().ReopenForRollup(sessionID, item.ID)
			}
		}
		return nil
	}
}

// createHumanReviewWorkItem opens a verify child for human gate (TD-WT-05).
func createHumanReviewWorkItem(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if tm == nil || item == nil || round == nil {
		return nil
	}
	directive := round.SpawnRationale
	if item.Directive != "" {
		directive = round.SpawnRationale + "\n\nContext: " + item.Directive
	}
	_, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID:  item.ID,
		Kind:      WorkKindVerify,
		Title:     HumanReviewItemTitle,
		Directive: directive,
		Policy:    ExecPolicyReadonly,
	})
	if err != nil {
		return err
	}
	_ = tm.Tree().SetRoundPhase(sessionID, item.ID, RoundPhaseAwaitChild)
	return nil
}

// UserGateItemTitle (DM-20260704-006 RC-4b) marks WorkItems created by
// SpawnUserGate. Distinct from HumanReviewItemTitle so dashboards can
// distinguish "user-facing question needs answering" from "verifier
// abstained, escalate to operator review".
const UserGateItemTitle = "User gate required"

// DefaultUserGateToolFilter is the tool whitelist applied to user-gate
// WorkItems. ask_user_question is the only tool that can satisfy the
// gate; all other tools are dropped at the executor's contract.ToolFilter
// step (see bootstrap/surfaces.go + decisionplanning/filter_adapter.go).
//
// "ask_user_question" matches the tool name used in
// internal/layers/contextengine/enforce/tools/surface/orthogonal_flags.go
// (the canonical surface registry). If that name ever changes, update
// this constant in the same commit and re-run the tool-surface contract
// tests (TestAskUserQuestionSurface in
// internal/layers/contextengine/enforce/tools/surface/).
var DefaultUserGateToolFilter = []string{"ask_user_question"}

// createUserGateWorkItem opens a verify child gated on ask_user_question.
// Mirrors createHumanReviewWorkItem but stamps the WorkItem.ToolFilter so
// the LLM cannot bypass the gate via free-form tools. The Directive is
// built from the round's ResolutionReport.UnresolvedObs so the child LLM
// sees which ObsIDs it must surface to the user.
func createUserGateWorkItem(sessionID string, item *WorkItem, round *WorkItemPipelineRound, tm *TaskManager) error {
	if tm == nil || item == nil || round == nil {
		return nil
	}
	directive := round.SpawnRationale
	if round.ResolutionReport != nil && len(round.ResolutionReport.UnresolvedObs) > 0 {
		obsList := buildUserGateObsList(round.ResolutionReport.UnresolvedObs)
		directive = directive + "\n\nUnresolved ObsIDs:\n" + obsList
	}
	if item.Directive != "" {
		directive = directive + "\n\nOriginal directive:\n" + item.Directive
	}
	_, err := tm.CreateWorkItem(sessionID, CreateWorkItemInput{
		ParentID:   item.ID,
		Kind:       WorkKindVerify,
		Title:      UserGateItemTitle,
		Directive:  directive,
		Policy:     ExecPolicyReadonly,
		ToolFilter: DefaultUserGateToolFilter,
	})
	if err != nil {
		return err
	}
	_ = tm.Tree().SetRoundPhase(sessionID, item.ID, RoundPhaseAwaitChild)
	return nil
}

// buildUserGateObsList renders the UnresolvedObs slice as a human-readable
// bullet list for the user-gate child's directive. Strength + Reason are
// included so the LLM knows the priority order. Truncated to the top-10
// unresolved ObsIDs to keep the directive well within token budget.
func buildUserGateObsList(unresolved []interfaces.UnresolvedObs) string {
	if len(unresolved) == 0 {
		return "  (none)"
	}
	const cap = 10
	out := ""
	for i, uo := range unresolved {
		if i >= cap {
			out += fmt.Sprintf("  ... +%d more\n", len(unresolved)-cap)
			break
		}
		out += fmt.Sprintf("  - %s (strength=%.3f, reason=%s)\n", uo.ObsID, uo.Strength, uo.Reason)
	}
	return out
}

// isBudgetGateError reports whether err is one of the three decompose
// budget gates (depth / children / daily). When true, RC-4a-driven
// SpawnDecompose degrades to SpawnInline rather than aborting the
// session loop.
//
// DM-20260704-006 Phase 4 (D7-S15-A109-T02): the legacy
// `execution_mode: "decompose"` + `child_specs[]` carrier silently
// dropped these errors (the LLM proposal was narrative intent only),
// so callers observed "no children spawned" without a session abort.
// Phase 4 preserves that observable behavior by degrading to inline
// retry when the budget is exhausted.
//
// Non-budget errors (parent not found, scope validation, ...) still
// propagate so callers see real failures.
func isBudgetGateError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrDecomposeDepthExceeded) ||
		errors.Is(err, ErrTooManyChildren) ||
		errors.Is(err, ErrDecomposeDailyLimit)
}

// pipelineItemNeedsContinuation reports whether the session loop should schedule
// another MUPS round on this WorkItem (DM-20260703-001 CC-1 / D7-S2-A86-T02).
func pipelineItemNeedsContinuation(item *WorkItem) bool {
	if item == nil || item.LastRound == nil || item.Status != TaskStatusInProgress {
		return false
	}
	if item.LastRound.SpawnPolicy == SpawnInline {
		return DeliverableContinuationRequired(item.LastRound)
	}
	return false
}

// GetPipelineFocus selects the next WorkItem for RunSessionTurnLoop (Phase C).
// Pending ready items win; otherwise in_progress items needing inline retry
// or deliverable continuation after SpawnNone stagnation.
func (t *WorkTree) GetPipelineFocus(sessionID string) (*WorkItem, error) {
	if t == nil {
		return nil, nil
	}
	if focus, err := t.GetFocus(sessionID); focus != nil || err != nil {
		return focus, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.ensureSessionLocked(sessionID)
	for _, item := range t.items[sessionID] {
		if item == nil || item.Ephemeral || isTerminalStatus(item.Status) {
			continue
		}
		if pipelineItemNeedsContinuation(item) {
			return item, nil
		}
	}
	return nil, nil
}

// HasOpenWork reports whether the session still has non-terminal work items.
func (t *WorkTree) HasOpenWork(sessionID string) bool {
	if t == nil {
		return false
	}
	if focus, _ := t.GetPipelineFocus(sessionID); focus != nil {
		return true
	}
	for _, item := range t.List(sessionID) {
		if item == nil || item.Ephemeral {
			continue
		}
		if item.Kind == WorkKindGoal && item.NeedsRollup && item.ParentID == "" {
			return true
		}
		if !isTerminalStatus(item.Status) {
			return true
		}
	}
	return false
}
