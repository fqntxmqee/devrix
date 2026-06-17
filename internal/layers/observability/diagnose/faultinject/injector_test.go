//go:build testbuild
// +build testbuild

package faultinject

import (
	"errors"
	"os"
	"testing"
	"time"
)

// TestInjector_DisabledByDefault — 无 env 时 injector 不启用。
func TestInjector_DisabledByDefault(t *testing.T) {
	os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if inj.Enabled() {
		t.Fatal("expected disabled when no env set")
	}
	if err := inj.Hook("any.target"); err != nil {
		t.Errorf("disabled hook should return nil, got %v", err)
	}
}

// TestInjector_EnvParse_Basic — env 解析后,Hook 返回 error。
func TestInjector_EnvParse_Basic(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "svc.dispatch=error:simulated_network_down")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if !inj.Enabled() {
		t.Fatal("expected enabled")
	}
	err := inj.Hook("svc.dispatch")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "injected: simulated_network_down" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestInjector_MultipleRules — 多个 target。
func TestInjector_MultipleRules(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "a=error:a_fail,b=truncate:b_trunc")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if err := inj.Hook("a"); err == nil {
		t.Error("expected error from a")
	}
	if err := inj.Hook("b"); err == nil || err.Error() != "truncated: b_trunc" {
		t.Errorf("expected truncated b_trunc, got %v", err)
	}
	if err := inj.Hook("c"); err != nil {
		t.Errorf("unknown target should pass, got %v", err)
	}
}

// TestInjector_Once_TriggersOnce — :once 后缀只触发一次。
func TestInjector_Once_TriggersOnce(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "x:once=error:once_fail")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if err := inj.Hook("x"); err == nil {
		t.Error("first call should error")
	}
	if err := inj.Hook("x"); err != nil {
		t.Errorf("second call should pass (once), got %v", err)
	}
}

// TestInjector_Reset — Reset 后 enabled=false, hook 失效。
func TestInjector_Reset(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "x=error:reset")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if !inj.Enabled() {
		t.Fatal("expected enabled")
	}
	inj.Reset()
	if inj.Enabled() {
		t.Error("expected disabled after Reset")
	}
	if err := inj.Hook("x"); err != nil {
		t.Errorf("after reset, hook should pass: %v", err)
	}
}

// TestInjector_AddRule_Programmatic — 编程式添加 rule。
func TestInjector_AddRule_Programmatic(t *testing.T) {
	os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	inj.AddRule(Rule{Target: "p", Mode: "error", Param: "p_fail"})
	if !inj.Enabled() {
		t.Error("AddRule should enable")
	}
	if err := inj.Hook("p"); err == nil {
		t.Error("expected error from p")
	}
}

// TestInjector_LatencyMode — mode=latency 时延迟 param ms。
func TestInjector_LatencyMode(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "slow=latency:50")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	start := time.Now()
	if err := inj.Hook("slow"); err != nil {
		t.Errorf("latency mode should not error, got %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected >=50ms delay, got %v", elapsed)
	}
}

// TestInjector_LatencyBadParam — 非法 latency 值 → 静默 pass(避免崩溃)。
func TestInjector_LatencyBadParam(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "bad=latency:notanumber")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if err := inj.Hook("bad"); err != nil {
		t.Errorf("bad latency param should pass, got %v", err)
	}
}

// TestInjector_NilSafe — nil receiver 不 panic。
func TestInjector_NilSafe(t *testing.T) {
	var i *Injector
	if i.Enabled() {
		t.Error("nil Enabled should be false")
	}
	if err := i.Hook("x"); err != nil {
		t.Errorf("nil Hook should be nil, got %v", err)
	}
}

// TestInjector_UnknownMode — 未知 mode → pass。
func TestInjector_UnknownMode(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "u=unknown_mode:foo")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	if err := inj.Hook("u"); err != nil {
		t.Errorf("unknown mode should pass, got %v", err)
	}
}

// TestInjector_ErrorEmptyParam — error 模式 param 空时仍返回错误。
func TestInjector_ErrorEmptyParam(t *testing.T) {
	os.Setenv("DEVRIX_FAULT_INJECT", "e=error:")
	defer os.Unsetenv("DEVRIX_FAULT_INJECT")
	inj := New()
	err := inj.Hook("e")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, err) { // 自身 Is 检查
		_ = err.Error()
	}
}
