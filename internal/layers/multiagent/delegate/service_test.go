package delegate_test

import (
	"context"
	"testing"

	"github.com/devrix/devrix/internal/layers/multiagent/delegate"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type stubFallback struct {
	called bool
}

func (s *stubFallback) RunSubQuery(_ context.Context, _ *types.SessionContext, _ delegate.WorkerSpec) (string, error) {
	s.called = true
	return "subquery summary", nil
}

// T: D4-S10-A01-T01 (fallback path when D4 disabled)
func TestService_should_fallback_to_subquery_when_delegate_disabled(t *testing.T) {
	fallback := &stubFallback{}
	svc := delegate.NewService(config.DelegateConfig{Enabled: false}, fallback, nil, nil)
	parent := &types.SessionContext{SessionID: "sess_1", WorkDir: "/tmp"}
	res, err := svc.DelegateOrFallback(context.Background(), nil, parent, delegate.WorkerSpec{
		Role:      delegate.WorkerRoleExplore,
		Directive: "find auth flow",
	})
	if err != nil {
		t.Fatalf("DelegateOrFallback: %v", err)
	}
	if !fallback.called {
		t.Fatal("expected subquery fallback to run")
	}
	if res.Summary != "subquery summary" {
		t.Fatalf("summary = %q, want subquery summary", res.Summary)
	}
}

// T: D4-S10-A01-T01
func TestService_should_report_disabled_when_not_configured(t *testing.T) {
	svc := delegate.NewService(config.DelegateConfig{Enabled: false}, nil, nil, nil)
	if svc.Enabled() {
		t.Fatal("expected delegate disabled")
	}
}
