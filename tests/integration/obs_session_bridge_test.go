//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/shared/config"
)

// Covers: L5-OBS-18
func TestGateway_should_track_active_sessions_via_session_bridge(t *testing.T) {
	dir := t.TempDir()
	store, err := gateway.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	obsCfg := observability.DefaultConfig()
	obs, err := observability.New(obsCfg)
	if err != nil {
		t.Fatalf("observability: %v", err)
	}

	gw := gateway.NewCommunicationGateway(store, nil, nil, nil, config.DefaultConfig())
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
