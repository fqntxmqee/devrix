package workmodel

import (
	"fmt"
	"sync"
	"time"
)

// DefaultUncertaintyDecomposeThreshold triggers decompose hints when exceeded.
const DefaultUncertaintyDecomposeThreshold = 0.6

// ErrDecomposeDepthExceeded is returned when creating a child beyond max depth.
var ErrDecomposeDepthExceeded = fmt.Errorf("max decompose depth exceeded")

// ErrTooManyChildren is returned when a parent already has max children.
var ErrTooManyChildren = fmt.Errorf("max children per work item exceeded")

// ErrDecomposeDailyLimit is returned when session kind decompose limit is hit.
var ErrDecomposeDailyLimit = fmt.Errorf("daily decompose limit exceeded for kind")

// ChildSpec describes one decomposed child work item.
type ChildSpec struct {
	Kind           WorkKind
	Title          string
	Directive      string
	ScopeIn        []string
	ScopeOut       []string
	ExpectedReturn string
}

// DecomposeChildren creates persistent child work items under parentID.
func (m *TaskManager) DecomposeChildren(sessionID, parentID string, children []ChildSpec) ([]*WorkItem, error) {
	if m == nil || parentID == "" {
		return nil, fmt.Errorf("parent id required")
	}
	parent, ok := m.GetWorkItem(sessionID, parentID)
	if !ok {
		return nil, fmt.Errorf("parent not found: %s", parentID)
	}
	if !CanDecompose(parent.Kind) {
		return nil, fmt.Errorf("kind %s cannot be decomposed", parent.Kind)
	}
	if err := m.checkDecomposeLimits(sessionID, parentID, parent.Kind, len(children)); err != nil {
		return nil, err
	}
	if err := validateChildSpecsAgainstParentScope(parent, children); err != nil {
		return nil, err
	}

	out := make([]*WorkItem, 0, len(children))
	for _, c := range children {
		if trimScopeField(c.ExpectedReturn) == "" {
			return out, fmt.Errorf("child spec expected_return required (DM-20260627-003)")
		}
		kind := c.Kind
		if kind == "" {
			kind = WorkKindImplement
		}
		item, err := m.CreateWorkItem(sessionID, CreateWorkItemInput{
			ParentID:  parentID,
			Kind:      kind,
			Title:     c.Title,
			Directive: c.Directive,
		})
		if err != nil {
			return out, err
		}
		out = append(out, item)
		dl := DefaultChildDownlink(parent, item, c)
		m.storeChildDownlink(sessionID, dl)
		_ = m.EnsureCohortScope(sessionID, parentID)
	}
	recordDecompose(sessionID, parent.Kind, len(children))
	return out, nil
}

func validateChildSpecsAgainstParentScope(parent *WorkItem, children []ChildSpec) error {
	if parent == nil || parent.ScopeContract == nil || len(children) == 0 {
		return nil
	}
	tmp := make([]*WorkItem, 0, len(children))
	for i, c := range children {
		tmp = append(tmp, &WorkItem{
			ID:    fmt.Sprintf("child_spec_%d", i),
			Title: c.Title,
			ScopeContract: &ScopeContract{
				InScope: append([]string(nil), c.ScopeIn...),
			},
		})
	}
	res := ValidateChildScopes(parent, tmp)
	if res.OK {
		return nil
	}
	if len(res.Violations) == 0 {
		return fmt.Errorf("child scope validation failed")
	}
	return fmt.Errorf("child scope validation failed: %s", res.Violations[0].Message)
}

// CanDecompose reports whether a kind supports child decomposition.
func CanDecompose(kind WorkKind) bool {
	switch kind {
	case WorkKindGoal, WorkKindPlan, WorkKindImplement:
		return true
	default:
		return false
	}
}

func (m *TaskManager) checkDecomposeLimits(sessionID, parentID string, kind WorkKind, add int) error {
	depth := m.Tree().Depth(sessionID, parentID)
	if depth+1 > m.Tree().MaxDecomposeDepth() {
		return ErrDecomposeDepthExceeded
	}
	existing := countDecomposableChildren(m, sessionID, parentID)
	if existing+add > DefaultMaxChildren {
		return ErrTooManyChildren
	}
	if err := checkDailyDecomposeLimit(sessionID, kind, add); err != nil {
		return err
	}
	return nil
}

func countDecomposableChildren(m *TaskManager, sessionID, parentID string) int {
	n := 0
	for _, c := range m.Tree().ListChildren(sessionID, parentID) {
		if c == nil || (c.Kind == WorkKindChecklist && c.Ephemeral) {
			continue
		}
		n++
	}
	return n
}

var (
	decomposeMu     sync.Mutex
	decomposeCounts = map[string]decomposeBucket{}
)

type decomposeBucket struct {
	count int
	since time.Time
}

func sessionKindKey(sessionID string, kind WorkKind) string {
	return sessionID + "|" + string(kind)
}

func recordDecompose(sessionID string, kind WorkKind, n int) {
	decomposeMu.Lock()
	defer decomposeMu.Unlock()
	key := sessionKindKey(sessionID, kind)
	b := decomposeCounts[key]
	if b.since.IsZero() || time.Since(b.since) > 24*time.Hour {
		b = decomposeBucket{since: time.Now()}
	}
	b.count += n
	decomposeCounts[key] = b
}

func checkDailyDecomposeLimit(sessionID string, kind WorkKind, add int) error {
	decomposeMu.Lock()
	defer decomposeMu.Unlock()
	key := sessionKindKey(sessionID, kind)
	b := decomposeCounts[key]
	if !b.since.IsZero() && time.Since(b.since) > 24*time.Hour {
		b = decomposeBucket{since: time.Now()}
	}
	if b.count+add > DefaultMaxDecomposePerDay {
		return ErrDecomposeDailyLimit
	}
	return nil
}

// decomposeCountFor returns the live 24h rolling count for a (session,
// kind) pair, and whether the entry exists. Used by StrategicPlanBudget
// to surface remaining_daily in the Plan user prompt (RH-MUPS-07). Locked
// against the package-internal decomposeMu; resets naturally on >24h
// via the same path as checkDailyDecomposeLimit.
func decomposeCountFor(sessionID string, kind WorkKind) (int, bool) {
	decomposeMu.Lock()
	defer decomposeMu.Unlock()
	key := sessionKindKey(sessionID, kind)
	b, ok := decomposeCounts[key]
	if !ok {
		return 0, false
	}
	if !b.since.IsZero() && time.Since(b.since) > 24*time.Hour {
		return 0, false
	}
	return b.count, true
}

// ResetDecomposeLimits clears rate-limit state (tests).
func ResetDecomposeLimits() {
	decomposeMu.Lock()
	defer decomposeMu.Unlock()
	decomposeCounts = map[string]decomposeBucket{}
}
