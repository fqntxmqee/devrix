package ltllite

import (
	"strings"
	"testing"
	"time"
)

// T: W14 — Check 全部成立时返回空 violations。
func TestCheck_AllHold_NoViolation(t *testing.T) {
	type S struct {
		A string `invariant:"is_read_only => no_destructive"`
		B string `invariant:"destructive => permission_gate"`
	}
	set, _ := ParseStruct(S{})
	state := MapState{
		"is_read_only":    false, // pre=false → 不触发
		"no_destructive":  true,  // 仍需满足 (但 pre=false 跳过)
		"destructive":     true,  // pre=true
		"permission_gate": true,  // post=true → 成立
	}
	vs := Check(set, state)
	if len(vs) != 0 {
		t.Errorf("expected 0 violations, got %d: %+v", len(vs), vs)
	}
}

// T: W14 — 一条 invariant 违规时返回 1 个 Violation。
func TestCheck_OneViolated_ReturnsViolation(t *testing.T) {
	type S struct {
		A string `invariant:"destructive => permission_gate"`
	}
	set, _ := ParseStruct(S{})
	state := MapState{
		"destructive":     true,  // pre=true
		"permission_gate": false, // post=false → 违规
	}
	vs := Check(set, state)
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(vs))
	}
	v := vs[0]
	if v.PreVal != true || v.PostVal != false {
		t.Errorf("violation Pre/Post = (%v,%v), want (true,false)", v.PreVal, v.PostVal)
	}
	if !strings.Contains(v.Reason, "destructive") {
		t.Errorf("reason should mention 'destructive', got %q", v.Reason)
	}
}

// T: W14 — 未知命题 (state.Eval 返回 false) 视为 false — pre=false 不触发, 不违规。
// 这与"保守默认值"一致: 缺 state 映射时, 不会因 missing key 误报。
func TestCheck_UnknownProp_NotViolationWhenPre(t *testing.T) {
	type S struct {
		A string `invariant:"missing_prop => something"`
	}
	set, _ := ParseStruct(S{})
	vs := Check(set, MapState{}) // 空 state
	if len(vs) != 0 {
		t.Errorf("unknown pre-prop should not trigger violation, got %d", len(vs))
	}
}

// T: W14 — Check latency bound (≤ 5ms) for 1000 invariants (spec §LTL-Lite Self-Invariants)。
// 实测单 Eval 是 map lookup, 1000 次应远小于 5ms。
func TestCheck_LatencyBound_5ms(t *testing.T) {
	type Big struct {
		A string `invariant:"a => a_satisfied"`
	}
	set, _ := ParseStruct(Big{})
	// 复制到 1000 条
	big := InvariantSet{Invariants: make([]Invariant, 0, 1000)}
	for i := 0; i < 1000; i++ {
		big.Invariants = append(big.Invariants, set.Invariants[0])
	}
	state := MapState{"a": true, "a_satisfied": true}
	start := time.Now()
	vs := Check(big, state)
	elapsed := time.Since(start)
	if elapsed > 5*time.Millisecond {
		t.Errorf("Check 1000 invariants took %v, want <= 5ms (spec §LTL-Lite Self-Invariants)", elapsed)
	}
	if len(vs) != 0 {
		t.Errorf("expected 0 violations, got %d", len(vs))
	}
}

// T: W14 — Violation.String 含 invariant name + pre/post。
func TestViolation_String(t *testing.T) {
	type S struct {
		F string `invariant:"destructive => permission_gate"`
	}
	set, _ := ParseStruct(S{})
	vs := Check(set, MapState{"destructive": true, "permission_gate": false})
	if len(vs) != 1 {
		t.Fatal("expected 1 violation")
	}
	s := vs[0].String()
	for _, want := range []string{"S.F", "destructive", "permission_gate"} {
		if !strings.Contains(s, want) {
			t.Errorf("String missing %q: %s", want, s)
		}
	}
}

// T: W14 — MapState 实现 State 接口。
func TestMapState_ImplementsState(t *testing.T) {
	var _ State = MapState{}
}
