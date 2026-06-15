package query

import (
	"reflect"
	"testing"
)

// T: D2-S16-A01-T04 — DM-020 D2 thin closure: Loop must not carry
// orchestration fields. The four removed slots — Hooks (with sub-fields
// BeforeComplete / AfterToolRound), Attachments, SessionQueue — were D7
// orchestration surface that violated D2 = "pure execution primitive".
// This test guards against re-introduction. If you intentionally need
// one back, also update openspec/specs/d7-orchestration/d7-domain.md
// §Requirement: D2 Thin (line ~460) and §layer-delta.md.
func TestQueryLoop_ForbidsOrchestrationFields(t *testing.T) {
	banned := []string{
		"Hooks",        // contains BeforeComplete + AfterToolRound
		"Attachments",  // D7 caller collection (D2-S15 Prepare)
		"SessionQueue", // D7-S4 Hub-Spoke drain
	}
	rt := reflect.TypeOf(Loop{})
	for _, name := range banned {
		if _, ok := rt.FieldByName(name); ok {
			t.Errorf("D2 Thin violation: query.Loop.%s must not exist (DM-020 D2 thin closure)", name)
		}
	}
}

// T: D2-S16-A01-T05 — sub-fields of the removed Hooks struct must also
// not re-surface as standalone fields. (Compile-time guard: if a future
// refactor reintroduces these names, this reflection check catches it.)
func TestQueryLoop_ForbidsLegacyHookSubFields(t *testing.T) {
	banned := []string{
		"BeforeComplete",  // D7-S2-A06 lifecycle (RunTurnLoop)
		"AfterToolRound",  // D7-S2-A06 tool-round hook
		"EnsureParallelAsyncBatch",
		"WaitPendingAsyncBatch",
	}
	rt := reflect.TypeOf(Loop{})
	for _, name := range banned {
		if _, ok := rt.FieldByName(name); ok {
			t.Errorf("D2 Thin violation: query.Loop.%s must not exist (DM-020 D2 thin closure)", name)
		}
	}
}
