// LoopDepthTracker v2 (DM-20260625-003, PR-V5.1) — 按"模式 hash"计数回路深度。
//
// 关键设计:
//   - 按 hash(SessionID + 5 字段) 计数而非"按轮数"计数
//     → LLM 切换 Plan.Kind 算作新回路 (depth=1), 但同模式重复会被累积计数
//   - MaxDepth=3, 严格按 `depth >= MaxDepth` 触发 EscapeForceExit
//     (depth=1/2 → EscapeContinue, depth=3 → EscapeForceExit)
//   - 按 SessionID 维度保留 History map (不随 ProcessMessage 结束清空)
//     → T2 续跑机制 (V5.5) 的前提: depth 跨 ProcessMessage 边界不重置
//   - 唯一清空时机: tracker.Reset() (仅在 session 彻底结束 / admin reset 调用)
//   - 并发安全: sync.RWMutex (多 goroutine 同时调用 ShouldContinue)
//
// SoT: doc 38 §21.3.2 (line 3706-3780)
package escape

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/plan"
)

// LoopContext is the input to LoopDepthTracker.ShouldContinue.
//
// 7 fields total: 5 hash inputs (deterministic mode identification) +
// 2 state fields (LoopBudgetState + ExitReason, carried by Evaluate but
// not part of the hash). PrevPlanKind / PlanKindSwitchCount are NOT in
// LoopContext — they live in PlanKindSwitchPolicy (V5.2) and would
// otherwise pollute the hash (see review-r3 ISSUE-3).
type LoopContext struct {
	// 5 hash inputs (participate in hashLoopContext, mode identification).
	SessionID        string
	PlanKind         PlanKind         // 4-class: Commitment/Protocol/Scenario/Exploration
	ObservationKind  ObservationKind  // 4-class: ObsFact/ObsSignal/ObsDeviation/ObsUncertainty
	FailureCriterion string           // Plan failure criterion
	ArtifactType     ArtifactType     // 4-class: StateChangeCert/ResponseRecord/ProbeReport/ExperimentData

	// 2 state fields (do NOT participate in hash; passed to Evaluate for
	// downstream consumers like EscapeEngine and CircuitBreaker).
	LoopBudgetState LoopBudgetState
	ExitReason      ExitReason
}

// PlanKind is a typed enum re-exported from plan/plan.go so that escape
// package can consume PlanKind values without circular imports. The
// concrete enum + String/Parse live in plan/plan.go (Phase 2 PR-B1).
type PlanKind = plan.PlanKind

// ObservationKind is a typed enum re-exported from shared/types (Phase 2
// PR-A1).
type ObservationKind = uint8

// ArtifactType is a typed enum re-exported from shared/types (Phase 3
// PR-C1).
type ArtifactType = uint8

// ExitReason is a typed enum re-exported from shared/types (Phase 4
// 14-class ExitReason, see spec v4.5.0+ D7-S10-A33-T03). The concrete
// enum lives in shared/types/exit.go.
type ExitReason = uint8

// LoopBudgetState carries DenialBudget累计状态 (doc 38 §19.2):
//   - ConsecutiveFails: 连续失败次数 (达到 3 触发 ForceExit)
//   - TotalFails: 累计失败次数 (达到 20 触发 AbortWithAudit)
//
// V5.4 introduces LoopBudget evaluation; V5.1 only carries the value
// inside LoopContext (no logic yet).
type LoopBudgetState struct {
	ConsecutiveFails int
	TotalFails       int
}

// DefaultMaxDepth is the canonical MaxDepth value. ShouldContinue returns
// EscapeContinue while depth < MaxDepth and EscapeForceExit when
// depth >= MaxDepth.
const DefaultMaxDepth = 3

// LoopDepthTracker counts loop depth by mode-hash for a single orchestrator
// instance. Safe for concurrent use; the History map is keyed by
// SessionID so multiple sessions can share one tracker.
//
// History is preserved across ProcessMessage boundaries (depth carries
// over from T1 to T2 via T2 ResumeSession in V5.5). The only way to
// clear history is Reset() (typically called at session end / admin reset).
type LoopDepthTracker struct {
	mu       sync.RWMutex
	maxDepth int
	history  map[string]int    // sessionID → current depth
	modes    map[string]string // sessionID → last mode-hash (for reset detection)
}

// NewLoopDepthTracker constructs a tracker with the given MaxDepth.
// MaxDepth < 1 returns nil + error (fail-fast).
func NewLoopDepthTracker(maxDepth int) (*LoopDepthTracker, error) {
	if maxDepth < 1 {
		return nil, ErrInvalidMaxDepth
	}
	return &LoopDepthTracker{
		maxDepth: maxDepth,
		history:  make(map[string]int),
		modes:    make(map[string]string),
	}, nil
}

// MaxDepth returns the configured maximum loop depth (read-only).
func (t *LoopDepthTracker) MaxDepth() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.maxDepth
}

// Depth returns the current depth for a given sessionID. 0 if the
// session has not been observed yet.
func (t *LoopDepthTracker) Depth(sessionID string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.history[sessionID]
}

// ShouldContinue increments the depth counter for the mode-hash
// corresponding to ctx and returns an EscapeDecision. Behavior:
//   - depth < MaxDepth → EscapeContinue (depth incremented)
//   - depth >= MaxDepth → EscapeForceExit (depth NOT incremented past MaxDepth)
//
// Mode-hash = SHA-256(SessionID + PlanKind + ObservationKind + FailureCriterion + ArtifactType).
// Same inputs across calls increment the counter; any input change
// resets the counter (new mode = new loop).
//
// On internal panic (should be impossible in practice but defensive):
// recovers → returns EscapeContinue + slog.Warn (不阻塞主链路, design §9).
func (t *LoopDepthTracker) ShouldContinue(ctx LoopContext) (decision EscapeDecision) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("loop_depth_tracker_panic_recovered",
				"panic", r,
				"session_id", ctx.SessionID,
				"max_depth", t.MaxDepth(),
			)
			// NOTE: avoid calling nowFunc() here — if panic was caused
			// by nowFunc, the deferred body would re-panic. Use a
			// zero-value CreatedAt instead.
			decision = EscapeDecision{
				Action:     EscapeContinue,
				Reason:     "panic_recovered_failsafe",
				AuditLevel: 1,
				SessionID:  ctx.SessionID,
			}
		}
	}()

	return t.shouldContinueLocked(ctx)
}

// shouldContinueLocked is the non-deferred inner ShouldContinue. Caller
// must NOT hold t.mu (we acquire it here).
func (t *LoopDepthTracker) shouldContinueLocked(ctx LoopContext) EscapeDecision {
	if t == nil {
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "tracker_nil_failsafe",
			AuditLevel: 0,
			SessionID:  ctx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	if ctx.SessionID == "" {
		// Defensive: callers should validate, but fail-safe to Continue
		// so the orchestrator can surface a structured error elsewhere.
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "session_id_empty",
			AuditLevel: 0,
			SessionID:  ctx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	mode := hashLoopContext(ctx)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Mode-based reset detection: if a different mode is observed for the
	// same session, reset the counter (LLM switched Plan.Kind = new loop).
	if lastMode, ok := t.modes[ctx.SessionID]; ok && lastMode != mode {
		t.modes[ctx.SessionID] = mode
		t.history[ctx.SessionID] = 1
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "mode_changed_reset",
			AuditLevel: 0,
			Depth:      1,
			SessionID:  ctx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	// First observation of this session OR same mode → increment counter.
	t.modes[ctx.SessionID] = mode
	nextDepth := t.history[ctx.SessionID] + 1
	t.history[ctx.SessionID] = nextDepth

	if nextDepth < t.maxDepth {
		return EscapeDecision{
			Action:     EscapeContinue,
			Reason:     "loop_depth_under_max",
			AuditLevel: 0,
			Depth:      nextDepth,
			SessionID:  ctx.SessionID,
			CreatedAt:  nowFunc(),
		}
	}

	return EscapeDecision{
		Action:     EscapeForceExit,
		Reason:     "loop_depth_exceeded",
		AuditLevel: 2,
		Depth:      nextDepth,
		SessionID:  ctx.SessionID,
		CreatedAt:  nowFunc(),
	}
}

// hashLoopContext returns a stable SHA-256 hex string identifying the
// "mode" of a loop iteration. 5 hash inputs participate; 2 state fields
// (LoopBudgetState + ExitReason) deliberately do NOT.
//
// Format: "sessionID:planKind:observationKind:failureCriterion:artifactType"
// → SHA-256 → hex (64 chars).
func hashLoopContext(ctx LoopContext) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%d:%d:%s:%d",
		ctx.SessionID,
		ctx.PlanKind,
		ctx.ObservationKind,
		ctx.FailureCriterion,
		ctx.ArtifactType,
	)))
	return hex.EncodeToString(h.Sum(nil))
}

// Reset clears the entire history (all sessions) AND mode map. Typically
// called at session end / admin reset. To reset a single session, use
// ResetSession.
func (t *LoopDepthTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.history = make(map[string]int)
	t.modes = make(map[string]string)
}

// ResetSession clears the history AND mode for a single session. Returns
// true if the session existed and was reset.
func (t *LoopDepthTracker) ResetSession(sessionID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, hadDepth := t.history[sessionID]
	_, hadMode := t.modes[sessionID]
	delete(t.history, sessionID)
	delete(t.modes, sessionID)
	return hadDepth || hadMode
}

// nowFunc is overridable for tests.
var nowFunc = func() time.Time { return time.Now() }