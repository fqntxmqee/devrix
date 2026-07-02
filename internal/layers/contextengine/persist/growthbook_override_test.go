// T05 + T08 tests for growthbook override (ThresholdOverride +
// GetPersistenceThreshold).
// DM-20260702-008 / D2-S15-A02-T05.
package persist

import (
	"testing"
)

func TestThresholdOverride_Empty_NoEffect(t *testing.T) {
	// Empty override map → declared value wins for all tools.
	override := NewThresholdOverride(nil)
	if got := GetPersistenceThreshold("read_file", 8192, override); got != 8192 {
		t.Errorf("nil override + read_file declared 8192: got %d, want 8192", got)
	}
	if got := GetPersistenceThreshold("bash", 30_000, override); got != 30_000 {
		t.Errorf("nil override + bash declared 30000: got %d, want 30000", got)
	}
}

func TestThresholdOverride_NilOverride_DeclaredWins(t *testing.T) {
	// nil override (no growthbook client wired in) → declared wins.
	if got := GetPersistenceThreshold("read_file", 8192, nil); got != 8192 {
		t.Errorf("nil override must let declared win, got %d", got)
	}
}

func TestThresholdOverride_PerToolOverrideWins(t *testing.T) {
	// GB operator shifted read_file from 8K → 16K as a canary. The
	// override value must be used verbatim, bypassing the declared.
	override := NewThresholdOverride(map[string]int{
		"read_file": 16 * 1024,
		"bash":      50 * 1024,
	})
	if got := GetPersistenceThreshold("read_file", 8*1024, override); got != 16*1024 {
		t.Errorf("override should win, got %d, want %d", got, 16*1024)
	}
	if got := GetPersistenceThreshold("bash", 30*1024, override); got != 50*1024 {
		t.Errorf("override should win, got %d, want %d", got, 50*1024)
	}
	// Tool not in override map → declared wins
	if got := GetPersistenceThreshold("grep", 20*1024, override); got != 20*1024 {
		t.Errorf("non-overridden tool must use declared, got %d, want %d", got, 20*1024)
	}
}

func TestThresholdOverride_HardOptOut_NonPositive(t *testing.T) {
	// declaredMaxResultSizeChars <= 0 is the "hard opt-out" clawcode
	// uses for Read (Infinity sentinel). We use 0/-1 because Go has no
	// infinity int. The override must NOT override the opt-out.
	cases := []int{0, -1, -100}
	for _, declared := range cases {
		override := NewThresholdOverride(map[string]int{
			"read_file": 16 * 1024, // would normally win
		})
		if got := GetPersistenceThreshold("read_file", declared, override); got != declared {
			t.Errorf("hard opt-out (declared=%d) must NOT be overridden, got %d", declared, got)
		}
	}
}

func TestThresholdOverride_DefensiveAgainstBadGBValues(t *testing.T) {
	// Growthbook caches can leak bad values (null, NaN, negative).
	// NewThresholdOverride does the only filtering we apply; the GB
	// client is responsible for not handing us NaN/Inf. We test the
	// "negative override" case explicitly: a bad value must fall
	// through to the declared.
	override := NewThresholdOverride(map[string]int{
		"read_file": -5, // operator fat-finger
	})
	if got := GetPersistenceThreshold("read_file", 8192, override); got != 8192 {
		t.Errorf("negative override must fall through to declared, got %d, want 8192", got)
	}
}

func TestNewThresholdOverride_DefensiveCopy(t *testing.T) {
	// Caller mutates the source map after construction; the override
	// MUST NOT see the mutation (defense-in-depth against shared
	// growthbook-client reuse).
	src := map[string]int{"read_file": 16 * 1024}
	override := NewThresholdOverride(src)
	src["read_file"] = 32 * 1024 // mutate after construction
	if got := GetPersistenceThreshold("read_file", 8192, override); got != 16*1024 {
		t.Errorf("override must be a copy, got %d, want %d", got, 16*1024)
	}
}

func TestWithOverrides_NilGetter_ReturnsNil(t *testing.T) {
	// WithOverrides(nil) → nil override → declared values win.
	override := WithOverrides(nil)
	if override != nil {
		t.Errorf("WithOverrides(nil) must return nil, got %v", override)
	}
}

func TestWithOverrides_LazyGetter_CalledOnce(t *testing.T) {
	// The getter is called once at WithOverrides time, not per-tool.
	// This is the contract the compression pipeline relies on (one
	// config snapshot per run, not per-message).
	calls := 0
	getter := func() map[string]int {
		calls++
		return map[string]int{"read_file": 16 * 1024}
	}
	override := WithOverrides(getter)
	if calls != 1 {
		t.Errorf("getter must be called once at WithOverrides, got %d", calls)
	}
	// Subsequent GetPersistenceThreshold calls must NOT re-invoke the getter.
	_ = GetPersistenceThreshold("read_file", 8192, override)
	_ = GetPersistenceThreshold("bash", 30720, override)
	if calls != 1 {
		t.Errorf("getter must not be called per-tool, got %d total calls", calls)
	}
}

func TestPersistThresholdOverrideFlag_NameMatchesSpec(t *testing.T) {
	// Sanity check on the flag name. The ops team uses this string in
	// their dashboards; if it changes, dashboards break.
	if PersistThresholdOverrideFlag != "devrix_persist_threshold_override" {
		t.Errorf("PersistThresholdOverrideFlag = %q, want devrix_persist_threshold_override",
			PersistThresholdOverrideFlag)
	}
}
