//go:build integration && d5

package integration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/instrument/metrics"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D5-S1-A01-T01
func TestGateway_should_track_active_sessions_via_session_bridge(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	obsCfg := observability.DefaultConfig()
	obs, err := observability.New(obsCfg)
	if err != nil {
		t.Fatalf("observability: %v", err)
	}

	gw := capture.NewCommunicationGateway(store, nil, nil, config.DefaultConfig(), nil)
	gw.SetObservability(obs)

	session, err := gw.CreateSession("feishu_chat", "/tmp")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	gaugeValue := activeSessionsGaugeValue(t, obs)
	if gaugeValue != 1 {
		t.Fatalf("expected active_sessions=1 after create, got %g", gaugeValue)
	}

	if err := gw.ExpireSession(session.SessionID); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	gaugeValue = activeSessionsGaugeValue(t, obs)
	if gaugeValue != 0 {
		t.Fatalf("expected active_sessions=0 after expire, got %g", gaugeValue)
	}
}

func activeSessionsGaugeValue(t *testing.T, obs *observability.Observability) float64 {
	t.Helper()
	reg := obs.Meter().Registry()
	for _, metric := range reg.List() {
		g, ok := metric.(metrics.Gauge)
		if !ok {
			continue
		}
		if g.Name() != "devrix_active_sessions" {
			continue
		}
		if g.Labels()["adapter"] != "all" {
			continue
		}
		return g.Value()
	}
	t.Fatal("devrix_active_sessions gauge not found")
	return 0
}
