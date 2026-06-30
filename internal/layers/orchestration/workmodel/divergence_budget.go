package workmodel

// Divergence budget constants — single source of truth for spawn-side
// limits (RH-MUPS-07, DM-20260701-001, T-P1-1). Prompts and the cap
// function in spawn_apply.go both read from these; the LLM-strategic
// proposer surfaces them in the Plan user prompt so it can self-bound
// proposals instead of being silently truncated post-hoc.
//
// Previously scattered: the per-parent children cap lived in
// work_tree.go (DefaultMaxChildren), the per-session-kind daily limit
// was a magic number in decompose.go:144, and the per-WorkItem ReAct
// iter cap lived in sessionorchestrator.workitem_executor.go as
// DefaultWorkItemMaxIters. Prompts had no way to see any of these.
const (
	// DefaultMaxChildren caps how many non-ephemeral children a single
	// parent may decompose to. Mirrors DefaultMaxChildren in work_tree.go
	// (kept in sync intentionally; the canonical name is here).
	DefaultMaxChildrenDiv = 7

	// DefaultMaxDecomposePerDay caps the 24h rolling per-(session, kind)
	// decompose count. The 5 was a magic number inside
	// checkDailyDecomposeLimit before this constant existed.
	DefaultMaxDecomposePerDay = 5

	// DefaultMaxReactIters caps the per-WorkItem ReAct loop. Mirrors
	// DefaultWorkItemMaxIters in sessionorchestrator.workitem_executor.go.
	// We keep the orchestrator value (it is a runtime safety cap) and
	// expose this constant so the LLM strategic proposer can advertise
	// the bound to the LLM.
	DefaultMaxReactIters = 5
)

// DivergenceBudget snapshots all spawn-side limits visible to one Plan
// round. The runner (item_pipeline.go) builds a fresh snapshot per
// pipeline round and threads it through StrategicPlanInput so the LLM
// proposer sees the same numbers the cap function will use.
type DivergenceBudget struct {
	// Depth / MaxDepth are the work-item's current depth and the
	// configured tree ceiling.
	Depth    int
	MaxDepth int

	// ExistingChildren is the parent's current non-ephemeral child
	// count; RemainingChildren = MaxChildrenDiv - ExistingChildren.
	ExistingChildren int
	MaxChildren      int

	// DecomposeUsedToday is the 24h rolling count for this
	// (session, kind); RemainingDaily = MaxDecomposePerDay -
	// DecomposeUsedToday.
	DecomposeUsedToday int
	MaxDaily           int

	// MaxIters is the per-WorkItem ReAct loop cap.
	MaxIters int
}

// RemainingChildren returns the headroom for new children (clamped ≥ 0).
func (b DivergenceBudget) RemainingChildren() int {
	rem := b.MaxChildren - b.ExistingChildren
	if rem < 0 {
		return 0
	}
	return rem
}

// RemainingDaily returns the headroom for new decompositions today (clamped ≥ 0).
func (b DivergenceBudget) RemainingDaily() int {
	rem := b.MaxDaily - b.DecomposeUsedToday
	if rem < 0 {
		return 0
	}
	return rem
}

// AsMap renders the budget as a key→string map for embedding in the
// Plan user prompt. Stable ordering so the snapshot test can assert
// exact output (T-P1-4).
func (b DivergenceBudget) AsMap() map[string]string {
	return map[string]string{
		"depth":               fmtInt(b.Depth),
		"max_depth":           fmtInt(b.MaxDepth),
		"existing_children":   fmtInt(b.ExistingChildren),
		"remaining_children":  fmtInt(b.RemainingChildren()),
		"max_children":        fmtInt(b.MaxChildren),
		"decompose_used_today": fmtInt(b.DecomposeUsedToday),
		"remaining_daily":     fmtInt(b.RemainingDaily()),
		"max_daily":           fmtInt(b.MaxDaily),
		"max_iters":           fmtInt(b.MaxIters),
	}
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	// strconv.Itoa would be cleaner; kept manual to avoid an extra import
	// in this hot file. Negative values emit the leading minus.
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// StrategicPlanBudget returns a snapshot for the given (parent, tm).
// Reading from tm directly means the values reflect live tree state
// (existing-children count, today's decompose counter) at the moment
// the Plan round runs.
func StrategicPlanBudget(sessionID string, item *WorkItem, tm *TaskManager) DivergenceBudget {
	b := DivergenceBudget{
		MaxChildren: DefaultMaxChildrenDiv,
		MaxDaily:    DefaultMaxDecomposePerDay,
		MaxIters:    DefaultMaxReactIters,
	}
	if tm == nil || item == nil {
		return b
	}
	b.Depth = tm.Tree().Depth(sessionID, item.ID)
	if md := tm.Tree().MaxDecomposeDepth(); md > 0 {
		b.MaxDepth = md
	} else {
		b.MaxDepth = DefaultMaxDecomposeDepth
	}
	b.ExistingChildren = countDecomposableChildren(tm, sessionID, item.ID)
	if cnt, ok := decomposeCountFor(sessionID, item.Kind); ok {
		b.DecomposeUsedToday = cnt
	}
	return b
}
