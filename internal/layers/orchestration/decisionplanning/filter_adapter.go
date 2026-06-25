package decisionplanning

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// AsToolFilter adapts the existing toolpolicy policy (FilterToolsForAgentRole)
// to the contracts.ToolFilter contract used by the new surface system.
//
// The existing filter takes a *types.SessionContext because it is invoked
// from inside the D2 context engine where the SessionContext is in scope.
// The contracts.ToolFilter API is intentionally neutral (no D1/D2 types
// on FilterCtx) so the SessionContext fields are recovered from
// FilterCtx.AgentType:
//
//   - "main"  → AgentID="" && !IsWorker  (leader, no filter)
//   - "fix"   → same as main              (no filter)
//   - "explore" / "plan" → IsWorker + WorkerRole
//   - "worker" → IsWorker (full worker set, not explore/plan restricted)
//   - "delegate" → hide delegate_* (no leader)
//   - anything else → conservative: hide delegate_* + apply worker read-only if IsWorker
//
// This means the adapter is fully self-contained — no SessionContext
// wiring is needed at the call site. Wires used in composition with
// PerAgentFilter / PerRiskFilter should put this filter BEFORE
// PerAgentFilter so the worker/explore read-only is applied first.
//
// DSAFT: TOOL-SURFACE-1-A03-F03 (DM-20260617-007)
func AsToolFilter() contracts.ToolFilter {
	return &toolpolicyFilterAdapter{}
}

type toolpolicyFilterAdapter struct{}

// Apply implements contracts.ToolFilter.
//
// Mapping rules (mirrors FilterToolsForAgentRole):
//  1. main / fix / "" → return all
//  2. non-leader (anything else) → hide delegate_*
//  3. explore / plan → also restrict to read-only worker set
//  4. worker → return what's left after step 2
//  5. delegate → return just delegate_*
func (a *toolpolicyFilterAdapter) Apply(specs []contracts.ToolSpec, ctx contracts.FilterCtx) []contracts.ToolSpec {
	switch ctx.AgentType {
	case "", "main", "fix":
		return specs
	case "delegate":
		out := make([]contracts.ToolSpec, 0, len(specs))
		for _, s := range specs {
			if DelegateToolNames[s.Name] {
				out = append(out, s)
			}
		}
		return out
	case "explore", "plan":
		return a.applyWorkerReadOnly(a.hideDelegate(specs))
	case "worker":
		return a.hideDelegate(specs)
	default:
		// Unknown: conservative — hide delegate_* + apply read-only worker set
		return a.applyWorkerReadOnly(a.hideDelegate(specs))
	}
}

func (a *toolpolicyFilterAdapter) hideDelegate(specs []contracts.ToolSpec) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if DelegateToolNames[s.Name] {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (a *toolpolicyFilterAdapter) applyWorkerReadOnly(specs []contracts.ToolSpec) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if readOnlyWorkerTools[strings.ToLower(s.Name)] {
			out = append(out, s)
		}
	}
	return out
}

// Compile-time check that AsToolFilter returns a contracts.ToolFilter.
var _ contracts.ToolFilter = AsToolFilter()
