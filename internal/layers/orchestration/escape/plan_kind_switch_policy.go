// PlanKindSwitchPolicy 3 档策略 (DM-20260625-003, PR-V5.2).
//
// 关键设计 (doc 38 §21.4.2, design.md §5.2):
//   - 3 档: Allowed (无上限) / Constrained (≤4) / Forbidden (任何切换 → ForceExit)
//   - 按 PlanKind 决定 policy:
//     * ExplorationPlan → Constrained (避免 LLM 反复尝试不同"探索"角度)
//     * ScenarioPlan    → Allowed    (并行多假设合法)
//     * ProtocolPlan    → Constrained (防止规则层震荡)
//     * CommitmentPlan  → Forbidden  (一旦承诺就要执行到底)
//   - 计数语义: PlanKindSwitchCount = "实际发生切换的次数"
//     * 首次建立 (KindUnset → X) 不算切换 → count=0
//     * 同 Kind 重选 (X → X)        不算切换 → count 不变
//     * 异 Kind (X → Y, X≠KindUnset) 算切换 → count++
//   - 累计跨 ProcessMessage 边界保留 (类似 LoopDepthTracker), 仅 Reset() 清空
//   - 并发安全: sync.RWMutex
//
// 与 LoopDepthTracker 关系:
//   - LoopDepthTracker 按"模式 hash"计数
//   - PlanKindSwitchTracker 按"PlanKind 切换"计数
//   - 两者正交: 切换 PlanKind 会 reset LoopDepth (新模式 = depth=1), 但
//     PlanKindSwitchCount 不受影响 (这是切换计数, 不是深度计数)
package escape

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// PlanKindSwitchPolicy is the 3-tier policy governing how many times
// PlanKind may switch within a single session (doc 38 §21.4.2).
//
// Mapping (from design §5.2 table):
//
//	ExplorationPlan → Constrained (≤4)
//	ScenarioPlan    → Allowed (no limit)
//	ProtocolPlan    → Constrained (≤4)
//	CommitmentPlan  → Forbidden (any switch → ForceExit)
type PlanKindSwitchPolicy uint8

const (
	// SwitchAllowed: free switching, no upper bound. Currently only
	// ScenarioPlan (parallel hypothesis exploration).
	SwitchAllowed PlanKindSwitchPolicy = iota

	// SwitchConstrained: bounded switching, up to MaxConstrainedSwitches.
	// Applies to ExplorationPlan and ProtocolPlan.
	SwitchConstrained

	// SwitchForbidden: any switch is rejected → EscapeForceExit.
	// Applies to CommitmentPlan ("一旦承诺就要执行到底").
	SwitchForbidden
)

// String returns the snake_case wire form for logging / debug.
func (p PlanKindSwitchPolicy) String() string {
	switch p {
	case SwitchAllowed:
		return "allowed"
	case SwitchConstrained:
		return "constrained"
	case SwitchForbidden:
		return "forbidden"
	default:
		return fmt.Sprintf("PlanKindSwitchPolicy(%d)", uint8(p))
	}
}

// MaxConstrainedSwitches is the upper bound for SwitchConstrained policy.
// Matches design §5.2 SoT: 4 switches → OK, 5th → ForceExit.
//
// Forbidden is also checked against this constant as a sanity bound —
// since Forbidden rejects at count=1 (any switch), the constant itself
// never causes Forbidden to exceed.
const MaxConstrainedSwitches = 4

// PlanKind re-export alias declared in loop_depth_tracker.go (kept here
// for compile-time access by tests in this file). The numeric constants
// mirror plan.PlanKind from plan/plan.go (Phase 2 PR-B1):

// DetermineSwitchPolicy maps a PlanKind to its switch policy (doc 38 §21.4.2).
//
// Unknown PlanKind (numeric values outside the 4 known kinds) defaults to
// SwitchConstrained (conservative: cap at MaxConstrainedSwitches rather than
// allow unlimited or outright forbid).
func DetermineSwitchPolicy(planKind PlanKind) PlanKindSwitchPolicy {
	switch planKind {
	case plan.ExplorationPlan:
		return SwitchConstrained
	case plan.ScenarioPlan:
		return SwitchAllowed
	case plan.ProtocolPlan:
		return SwitchConstrained
	case plan.CommitmentPlan:
		return SwitchForbidden
	default:
		return SwitchConstrained
	}
}

// SwitchDecision is the outcome of a RecordSwitch call.
type SwitchDecision struct {
	// Allowed reports whether the switch is permitted under the active policy.
	Allowed bool

	// Count is the new PlanKindSwitchCount after this call.
	Count int

	// Exceeded reports whether the switch triggered EscapeForceExit.
	// (Allowed=false always pairs with Exceeded=true; the inverse is not
	// guaranteed because Allowed=false can also occur when prev==newKind
	// is impossible — see implementation.)
	Exceeded bool

	// Policy is the resolved policy (for logging/audit). For first-call
	// (prev=KindUnset) this reflects the new-kind policy as fallback.
	Policy PlanKindSwitchPolicy
}

// PlanKindSwitchTracker tracks per-session PlanKind switch count and the
// most-recent PlanKind. The state is preserved across ProcessMessage
// boundaries (T2 ResumeSession in V5.5), so a switch that happened in
// ProcessMessage N is still counted in N+1.
//
// Concurrency: safe for concurrent use via sync.RWMutex.
type PlanKindSwitchTracker struct {
	mu          sync.RWMutex
	prevKind    map[string]PlanKind
	switchCount map[string]int
}

// NewPlanKindSwitchTracker constructs an empty tracker. No constructor
// validation needed (no knobs to misconfigure).
func NewPlanKindSwitchTracker() *PlanKindSwitchTracker {
	return &PlanKindSwitchTracker{
		prevKind:    make(map[string]PlanKind),
		switchCount: make(map[string]int),
	}
}

// RecordSwitch records a new PlanKind for the session and reports whether
// the transition is allowed under the active policy.
//
// Semantics (matches design §5.2):
//
//	prev == KindUnset (first call):    always allowed, count = 0
//	prev == newKind  (same kind):      always allowed, count unchanged
//	prev != newKind, prev != KindUnset: switch → count++, then check policy
//	  - policy = DetermineSwitchPolicy(prev)  (the kind we're leaving)
//	  - Allowed:        always allowed
//	  - Constrained:    allowed iff count <= MaxConstrainedSwitches
//	  - Forbidden:      never allowed (any switch exceeds)
//
// The state (prevKind, switchCount) is updated atomically with the
// decision. If Allowed=false, the caller should typically emit
// EscapeForceExit; the tracker itself does not emit EscapeDecisions.
func (t *PlanKindSwitchTracker) RecordSwitch(sessionID string, newKind PlanKind) SwitchDecision {
	t.mu.Lock()
	defer t.mu.Unlock()

	prev := t.prevKind[sessionID]
	count := t.switchCount[sessionID]

	// Case 1: same kind → no switch, no count change.
	if prev == newKind {
		return SwitchDecision{
			Allowed: true,
			Count:   count,
			Policy:  DetermineSwitchPolicy(newKind),
		}
	}

	// Case 2: first call (prev == plan.KindUnset) → not a real switch.
	if prev == plan.KindUnset {
		t.prevKind[sessionID] = newKind
		return SwitchDecision{
			Allowed: true,
			Count:   0,
			Policy:  DetermineSwitchPolicy(newKind),
		}
	}

	// Case 3: actual switch — prev != KindUnset && prev != newKind.
	newCount := count + 1
	t.switchCount[sessionID] = newCount
	t.prevKind[sessionID] = newKind

	policy := DetermineSwitchPolicy(prev)
	switch policy {
	case SwitchAllowed:
		return SwitchDecision{
			Allowed: true,
			Count:   newCount,
			Policy:  policy,
		}
	case SwitchConstrained:
		if newCount <= MaxConstrainedSwitches {
			return SwitchDecision{
				Allowed: true,
				Count:   newCount,
				Policy:  policy,
			}
		}
		return SwitchDecision{
			Allowed:  false,
			Count:    newCount,
			Exceeded: true,
			Policy:   policy,
		}
	case SwitchForbidden:
		// Any switch is rejected — even the first one.
		return SwitchDecision{
			Allowed:  false,
			Count:    newCount,
			Exceeded: true,
			Policy:   policy,
		}
	default:
		// Unknown policy value → conservative failure.
		return SwitchDecision{
			Allowed:  false,
			Count:    newCount,
			Exceeded: true,
			Policy:   policy,
		}
	}
}

// GetCount returns the current switch count for a session. 0 if the
// session has not been recorded yet (or after Reset).
func (t *PlanKindSwitchTracker) GetCount(sessionID string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.switchCount[sessionID]
}

// GetPrevKind returns the most-recently recorded PlanKind for a session.
// KindUnset (0) if the session has not been recorded yet.
func (t *PlanKindSwitchTracker) GetPrevKind(sessionID string) PlanKind {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.prevKind[sessionID]
}

// Reset clears the state for a single session. Returns true if the
// session had any prior state (debug aid).
func (t *PlanKindSwitchTracker) Reset(sessionID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, hadKind := t.prevKind[sessionID]
	_, hadCount := t.switchCount[sessionID]
	delete(t.prevKind, sessionID)
	delete(t.switchCount, sessionID)
	return hadKind || hadCount
}

// ResetAll clears all sessions (typically called at process shutdown or
// admin reset — same pattern as LoopDepthTracker.Reset).
func (t *PlanKindSwitchTracker) ResetAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prevKind = make(map[string]PlanKind)
	t.switchCount = make(map[string]int)
}

// ToEscapeDecision converts a SwitchDecision into an EscapeDecision for
// the escape pipeline. Exceed=true maps to EscapeForceExit with
// AuditLevel=2 (full audit trail). This is the integration seam called
// from V5.5 (5-node wiring); kept here so V5.2's API surface is complete.
//
// On Allowed=true, returns EscapeContinue (the policy check passed and
// the caller may proceed with the new PlanKind).
func (d SwitchDecision) ToEscapeDecision(sessionID string, planKind PlanKind) EscapeDecision {
	if d.Exceeded {
		return EscapeDecision{
			Action:     EscapeForceExit,
			Reason:     fmt.Sprintf("plan_kind_switch_exceeded_kind=%s_count=%d_policy=%s", planKindName(planKind), d.Count, d.Policy),
			AuditLevel: 2,
			SessionID:  sessionID,
			CreatedAt:  nowFunc(),
		}
	}
	return EscapeDecision{
		Action:     EscapeContinue,
		Reason:     fmt.Sprintf("plan_kind_switch_allowed_kind=%s_count=%d_policy=%s", planKindName(planKind), d.Count, d.Policy),
		AuditLevel: 0,
		SessionID:  sessionID,
		CreatedAt:  nowFunc(),
	}
}

// planKindName returns a human-readable PlanKind name for logging.
// Kept private — external callers should use plan.PlanKind.String().
func planKindName(k PlanKind) string {
	switch k {
	case plan.KindUnset:
		return "unset"
	case plan.CommitmentPlan:
		return "commitment"
	case plan.ProtocolPlan:
		return "protocol"
	case plan.ScenarioPlan:
		return "scenario"
	case plan.ExplorationPlan:
		return "exploration"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// slogPlanKindSwitchExceeded emits a structured warn log when a switch
// exceeds policy. Called by V5.5's orchestrator integration (kept here
// so V5.2 owns the canonical log shape for the event).
func slogPlanKindSwitchExceeded(sessionID string, planKind PlanKind, decision SwitchDecision) {
	slog.Warn("plan_kind_switch_exceeded",
		"session_id", sessionID,
		"plan_kind", planKindName(planKind),
		"policy", decision.Policy.String(),
		"count", decision.Count,
		"max_constrained", MaxConstrainedSwitches,
	)
}