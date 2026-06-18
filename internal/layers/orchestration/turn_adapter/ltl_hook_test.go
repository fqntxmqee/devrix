package turn_adapter

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/ltllite"
)

// T: W15 — HookRegistry 注册 + 计数。
func TestHookRegistry_Register(t *testing.T) {
	r := NewHookRegistry()
	if r.Count() != 0 {
		t.Errorf("new registry count = %d, want 0", r.Count())
	}
	r.Register(SurfaceHook{Name: "test"})
	if r.Count() != 1 {
		t.Errorf("after register count = %d, want 1", r.Count())
	}
}

// T: W15 — Prepare 全部满足时返回 nil。
func TestHookRegistry_Prepare_AllSatisfied_NoError(t *testing.T) {
	r := NewHookRegistry()
	r.Register(SurfaceHook{
		Name: "s1",
		InvSet: mustParseHook("a => a_holds"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"a": true, "a_holds": true}
		},
	})
	if err := r.Prepare(); err != nil {
		t.Errorf("Prepare: %v", err)
	}
}

// T: W15 — Prepare 一条违规时返回 wrapped ErrInvariantViolation。
func TestHookRegistry_Prepare_OneViolated_ReturnsErrInvariantViolation(t *testing.T) {
	r := NewHookRegistry()
	r.Register(SurfaceHook{
		Name: "s1",
		InvSet: mustParseHook("destructive => permission_gate"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"destructive": true, "permission_gate": false}
		},
	})
	err := r.Prepare()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvariantViolation) {
		t.Errorf("expected ErrInvariantViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("error should mention 'destructive', got %q", err.Error())
	}
}

// T: W15 — BeforeExecute 只评估指定 surface。
func TestHookRegistry_BeforeExecute_TargetedCheck(t *testing.T) {
	r := NewHookRegistry()
	r.Register(SurfaceHook{
		Name: "lsp",
		InvSet: mustParseHook("is_typed => typed_only"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"is_typed": true, "typed_only": false} // violation
		},
	})
	r.Register(SurfaceHook{
		Name: "bash",
		InvSet: mustParseHook("has_rules => rules_nonempty"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"has_rules": true, "rules_nonempty": true} // OK
		},
	})

	// 调用 lsp 应失败
	if err := r.BeforeExecute("lsp"); err == nil {
		t.Error("expected error for lsp")
	}
	// 调用 bash 应成功
	if err := r.BeforeExecute("bash"); err != nil {
		t.Errorf("bash should pass: %v", err)
	}
	// 调用不存在的 surface 应成功 (no-op)
	if err := r.BeforeExecute("nonexistent"); err != nil {
		t.Errorf("nonexistent should pass: %v", err)
	}
}

// T: W15 — PrepareTimed latency < 5ms (1000 invariants)。
func TestHookRegistry_PrepareTimed_LatencyBound(t *testing.T) {
	type Big struct {
		A string `invariant:"a => a_holds"`
	}
	bigSet, _ := ltllite.ParseStruct(Big{})
	// 复制到 1000 条
	big := ltllite.InvariantSet{Invariants: make([]ltllite.Invariant, 0, 1000)}
	for i := 0; i < 1000; i++ {
		big.Invariants = append(big.Invariants, bigSet.Invariants[0])
	}
	r := NewHookRegistry()
	r.Register(SurfaceHook{
		Name:   "stress",
		InvSet: big,
		Provider: func() ltllite.State {
			return ltllite.MapState{"a": true, "a_holds": true}
		},
	})
	elapsed, err := r.PrepareTimed()
	if err != nil {
		t.Fatalf("PrepareTimed: %v", err)
	}
	if elapsed > 5*time.Millisecond {
		t.Errorf("PrepareTimed took %v, want <= 5ms (spec §LTL-Lite)", elapsed)
	}
}

// T: W15 — Nil Provider 跳过 (graceful degradation)。
func TestHookRegistry_NilProvider_Skips(t *testing.T) {
	r := NewHookRegistry()
	r.Register(SurfaceHook{Name: "nil", InvSet: ltllite.InvariantSet{}, Provider: nil})
	if err := r.Prepare(); err != nil {
		t.Errorf("nil provider should not error: %v", err)
	}
}

// T: W15 — 多 surface violation 全部报告 (非 first-fail)。
func TestHookRegistry_Prepare_MultipleSurfaces_AllReported(t *testing.T) {
	r := NewHookRegistry()
	r.Register(SurfaceHook{
		Name: "s1",
		InvSet: mustParseHook("a => a_holds"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"a": true, "a_holds": false}
		},
	})
	r.Register(SurfaceHook{
		Name: "s2",
		InvSet: mustParseHook("b => b_holds"),
		Provider: func() ltllite.State {
			return ltllite.MapState{"b": true, "b_holds": false}
		},
	})
	err := r.Prepare()
	if err == nil {
		t.Fatal("expected error")
	}
	// 应同时报告 s1 和 s2 违规
	if !strings.Contains(err.Error(), "a_holds") {
		t.Errorf("error missing a_holds, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "b_holds") {
		t.Errorf("error missing b_holds, got %q", err.Error())
	}
}

type hookSample struct {
	F string `invariant:"x => y"`
}

func mustParseHook(tag string) ltllite.InvariantSet {
	// 解析 tag 字符串生成 single-invariant set
	type T struct {
		F string `invariant:""`
	}
	_ = T{}
	set := ltllite.InvariantSet{Invariants: []ltllite.Invariant{{
		Name: "F", Pre: strings.Split(tag, " => ")[0], Post: strings.Split(tag, " => ")[1], Source: "test",
	}}}
	return set
}
