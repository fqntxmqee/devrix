package wave

import (
	"path"
	"strings"
	"sync"
)

// RunningTask is a TaskNode paired with its acquired SlotID; tracked by the
// ConflictGuard during its in-flight window.
type RunningTask struct {
	Node   TaskNode
	SlotID SlotID
}

// ConflictGuard enforces upper-layer write-conflict avoidance (design §5).
// It does NOT replace worktree isolation — it is a scheduling-time guard so
// that two write tasks targeting the same file scope do not run in parallel.
//
// Rules (all must pass to Allow):
//  1. conflict_group: at most one Task per group can be running.
//  2. file_scope: if any running Task shares a non-read-only path with the
//     candidate AND the candidate is a writer, deny.
//  3. read_only running tasks never block writers (per design §5.3 hint).
type ConflictGuard struct {
	mu      sync.Mutex
	running map[SlotID]RunningTask
}

// NewConflictGuard creates an empty guard.
func NewConflictGuard() *ConflictGuard {
	return &ConflictGuard{running: make(map[SlotID]RunningTask)}
}

// Allow reports whether the candidate TaskNode can be dispatched given the
// currently running tasks.
func (g *ConflictGuard) Allow(candidate TaskNode, running []RunningTask) bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.running) == 0 && len(running) == 0 {
		return true
	}

	effective := make([]RunningTask, 0, len(g.running)+len(running))
	for _, r := range g.running {
		effective = append(effective, r)
	}
	effective = append(effective, running...)

	for _, r := range effective {
		if conflictBetween(candidate, r.Node) {
			return false
		}
	}
	return true
}

// Register records a running task so subsequent Allow checks see it.
func (g *ConflictGuard) Register(t RunningTask) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running[t.SlotID] = t
}

// Unregister removes a running task by slot id (idempotent).
func (g *ConflictGuard) Unregister(slotID SlotID) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.running, slotID)
}

// Running returns a snapshot of registered running tasks.
func (g *ConflictGuard) Running() []RunningTask {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]RunningTask, 0, len(g.running))
	for _, r := range g.running {
		out = append(out, r)
	}
	return out
}

func conflictBetween(a, b TaskNode) bool {
	// Group: same non-empty group blocks.
	if a.ConflictGroup != "" && a.ConflictGroup == b.ConflictGroup {
		return true
	}
	// File scope intersection: writers block each other, readers never block.
	if a.ReadOnly && b.ReadOnly {
		return false
	}
	if !a.ReadOnly && !b.ReadOnly {
		if scopesIntersect(a.FileScope, b.FileScope) {
			return true
		}
	}
	// Mixed: writer vs read-only is OK; read-only vs writer is also OK
	// (read-only is safe to coexist). No need to check.
	return false
}

func scopesIntersect(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for _, pa := range a {
		for _, pb := range b {
			if globOverlap(pa, pb) {
				return true
			}
		}
	}
	return false
}

// globOverlap reports whether two glob patterns can match the same concrete
// file. We use a conservative approximation: each pattern is expanded against
// representative "stem" strings to see if a common path could exist.
func globOverlap(p1, p2 string) bool {
	if p1 == "" || p2 == "" {
		return false
	}
	if p1 == p2 {
		return true
	}
	// Try to match p1 against a representative path of p2 (and vice versa).
	stem := globStem(p2)
	if matchGlob(p1, stem) {
		return true
	}
	stem = globStem(p1)
	if matchGlob(p2, stem) {
		return true
	}
	return false
}

// globStem extracts a representative concrete path from a glob pattern,
// suitable for cross-pattern overlap testing. It strips the trailing "/**"
// and leading "**/".
func globStem(p string) string {
	stem := p
	if strings.HasSuffix(stem, "/**") {
		stem = strings.TrimSuffix(stem, "/**")
	}
	if strings.HasPrefix(stem, "**/") {
		stem = strings.TrimPrefix(stem, "**/")
	}
	// If still contains glob meta, use a canonical sample.
	if strings.ContainsAny(stem, "*?[") {
		// Replace any remaining wildcard with a stable token "x".
		stem = strings.ReplaceAll(stem, "*", "x")
	}
	return stem
}

// matchGlob does a small subset of path-pattern matching sufficient for the
// conflict guard. It supports the doubled-star "**" wildcard, which Go's
// path.Match does not handle ("**" only matches a sequence of non-"/" chars
// there). We split on "/**" or "**/" boundaries and recurse on each segment.
//
// Pattern semantics:
//   - "*" matches any sequence of non-"/" chars
//   - "**" matches any sequence including "/"
//   - "?" matches a single non-"/" char
//   - trailing "/**" on a pattern means "anything under this dir"
//   - leading "**/" means "any depth" (matches as if the prefix were absent)
func matchGlob(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	name = strings.TrimSpace(name)
	if pattern == "" || name == "" {
		return false
	}
	// Trailing "/**" with literal prefix: anything under prefix matches.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if name == prefix {
			return true
		}
		if strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	// Leading "**/": drop it and re-try (also try matching against suffixes).
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		if matchGlob(suffix, name) {
			return true
		}
		// Match a tail of the name against the suffix.
		idx := -1
		for {
			j := strings.Index(name[idx+1:], "/")
			if j < 0 {
				break
			}
			idx = idx + 1 + j
			if matchGlob(suffix, name[idx+1:]) {
				return true
			}
		}
	}
	// Fall back to path.Match (treat ** as *): over-approximates safely.
	pat := strings.ReplaceAll(pattern, "**", "*")
	if ok, err := path.Match(pat, name); err == nil && ok {
		return true
	}
	// Segment-by-segment match where each "**" segment matches >= 0 segments.
	patSegs := strings.Split(pattern, "/")
	nameSegs := strings.Split(name, "/")
	return matchSegments(patSegs, nameSegs)
}

// matchSegments returns true if pattern segments match name segments, where
// "**" segments may match zero or more name segments.
func matchSegments(pat, name []string) bool {
	pi, ni := 0, 0
	starPi, starNi := -1, -1
	for ni < len(name) {
		if pi < len(pat) && pat[pi] == "**" {
			starPi = pi
			starNi = ni
			pi++
			continue
		}
		if pi < len(pat) {
			ok, _ := path.Match(pat[pi], name[ni])
			if ok {
				pi++
				ni++
				continue
			}
		}
		if starPi >= 0 {
			pi = starPi + 1
			starNi++
			ni = starNi
			continue
		}
		return false
	}
	// Trailing "**" segments match zero name segments.
	for pi < len(pat) && pat[pi] == "**" {
		pi++
	}
	return pi == len(pat)
}

// globMatch exposes globMatch as a package-level helper for tests.
func globMatch(name, pattern string) bool { return matchGlob(pattern, name) }
