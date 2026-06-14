//go:build integration && d1

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// T: D1-S1-A01-T04
func TestIntegration_PermissionRequestTimeout(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Permission.DefaultTimeout = 100 * time.Millisecond

	permMgr := capture.NewPermissionManager(&cfg.Permission)
	gw := capture.NewCommunicationGateway(store, nil, nil, permMgr, cfg)

	session, err := gw.CreateSession("cli", "/tmp")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	approved := permMgr.Request(context.Background(), session.SessionID, "bash", "ls -la", types.RiskLevelMedium)
	if approved {
		t.Error("expected permission to be denied after timeout")
	}
}
