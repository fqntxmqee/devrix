package metrics

import (
	"testing"
	"time"
)

// T: D2-S4-A01-T06 — LSP SLO 监控 (SUG-2 吸收)。
// 验证 5 个 method 都注册 latency histogram + call counter + timeout counter。
func TestRegisterLSPMetrics_AllMethodsRegistered(t *testing.T) {
	r := NewRegistry(nil, nil)
	m := RegisterLSPMetrics(r)
	if len(m.Latency) != len(LSPMethodNames) {
		t.Fatalf("latency map size = %d, want %d", len(m.Latency), len(LSPMethodNames))
	}
	if len(m.Calls) != len(LSPMethodNames) {
		t.Fatalf("calls map size = %d, want %d", len(m.Calls), len(LSPMethodNames))
	}
	if len(m.Timeouts) != len(LSPMethodNames) {
		t.Fatalf("timeouts map size = %d, want %d", len(m.Timeouts), len(LSPMethodNames))
	}
	for _, method := range LSPMethodNames {
		if _, ok := m.Latency[method]; !ok {
			t.Errorf("latency missing for %s", method)
		}
		if _, ok := m.Calls[method]; !ok {
			t.Errorf("calls missing for %s", method)
		}
		if _, ok := m.Timeouts[method]; !ok {
			t.Errorf("timeouts missing for %s", method)
		}
	}
}

// T: D2-S4-A01-T06 — LSPMethodTimer.Done 记录 latency + Inc calls counter。
func TestLSPMethodTimer_Done_RecordsLatency(t *testing.T) {
	r := NewRegistry(nil, nil)
	m := RegisterLSPMetrics(r)
	SetGlobalLSPMetrics(m)
	defer SetGlobalLSPMetrics(nil)

	timer := StartLSPMethodTimer(nil, "lsp_go_to_definition")
	time.Sleep(2 * time.Millisecond)
	timer.Done()

	if c := m.Calls["lsp_go_to_definition"].Value(); c != 1 {
		t.Errorf("calls count = %d, want 1", c)
	}
	if to := m.Timeouts["lsp_go_to_definition"].Value(); to != 0 {
		t.Errorf("timeout count = %d, want 0", to)
	}
}

// T: D2-S4-A01-T06 — LSPMethodTimer.Timeout 记录 latency + Inc calls + Inc timeouts。
func TestLSPMethodTimer_Timeout_RecordsTimeout(t *testing.T) {
	r := NewRegistry(nil, nil)
	m := RegisterLSPMetrics(r)
	SetGlobalLSPMetrics(m)
	defer SetGlobalLSPMetrics(nil)

	timer := StartLSPMethodTimer(nil, "lsp_hover")
	timer.Timeout()

	if c := m.Calls["lsp_hover"].Value(); c != 1 {
		t.Errorf("calls count = %d, want 1", c)
	}
	if to := m.Timeouts["lsp_hover"].Value(); to != 1 {
		t.Errorf("timeout count = %d, want 1", to)
	}
}

// T: D2-S4-A01-T06 — globalLSPMetrics 未注册时 StartLSPMethodTimer 返回 no-op
// timer (不 panic, 不记录)。
func TestLSPMethodTimer_NilMetrics_NoPanic(t *testing.T) {
	SetGlobalLSPMetrics(nil)
	timer := StartLSPMethodTimer(nil, "lsp_go_to_definition")
	timer.Done()
	timer.Fail()
	timer.Timeout()
	// 没 panic = 通过
}
