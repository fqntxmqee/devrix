package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestPermissionManager_Request(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 100 * time.Millisecond,
		MaxRetries:     3,
	}

	mgr := NewPermissionManager(cfg)
	ctx := context.Background()

	// This test will timeout quickly due to short timeout
	approved := mgr.Request(ctx, "sess_123", "bash", "ls -la", types.RiskLevelMedium)

	// Without a mock, this will timeout
	if approved {
		t.Error("expected permission to be denied (timeout)")
	}
}

func TestPermissionManager_Resolve(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 10 * time.Second,
		MaxRetries:     3,
	}

	mgr := NewPermissionManager(cfg)

	// Create a request directly for testing
	request := types.NewPermissionRequest("req_123", "sess_123", "bash", types.RiskLevelMedium, cfg.DefaultTimeout)

	mgr.mu.Lock()
	mgr.requests[request.ID] = request
	mgr.mu.Unlock()

	err := mgr.Resolve(request.ID, true)
	if err != nil {
		t.Fatalf("failed to resolve request: %v", err)
	}

	updated, _ := mgr.GetRequest(request.ID)
	if updated.Status != types.PermissionStatusApproved {
		t.Errorf("expected status 'approved', got '%s'", updated.Status)
	}
}

func TestPermissionManager_GetRequest_NotFound(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 60 * time.Second,
		MaxRetries:     3,
	}

	mgr := NewPermissionManager(cfg)

	_, err := mgr.GetRequest("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}

func TestPermissionManager_should_record_timeout_metrics(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 50 * time.Millisecond,
		MaxRetries:     3,
	}
	mgr := NewPermissionManager(cfg)
	obs, err := observability.New(observability.DefaultConfig())
	if err != nil {
		t.Fatalf("observability: %v", err)
	}
	mgr.SetObservability(obs)

	approved := mgr.Request(context.Background(), "sess_1", "bash", "ls", types.RiskLevelMedium)
	if approved {
		t.Fatal("expected timeout denial")
	}

	denied := findCounterValue(t, obs, "devrix_permission_decisions_total", metrics.LabelMap{"decision": "denied"})
	if denied != 1 {
		t.Fatalf("denied decisions=%d, want 1", denied)
	}
	timeouts := findCounterValue(t, obs, "devrix_permission_timeouts_total", metrics.LabelMap{})
	if timeouts != 1 {
		t.Fatalf("timeouts=%d, want 1", timeouts)
	}
}

func findCounterValue(t *testing.T, obs *observability.Observability, name string, labels metrics.LabelMap) int64 {
	t.Helper()
	for _, metric := range obs.Meter().Registry().List() {
		c, ok := metric.(metrics.Counter)
		if !ok || c.Name() != name {
			continue
		}
		if !labelSubset(c.Labels(), labels) {
			continue
		}
		return c.Value()
	}
	t.Fatalf("counter %s labels=%v not found", name, labels)
	return 0
}

func labelSubset(actual, want metrics.LabelMap) bool {
	for k, v := range want {
		if actual[k] != v {
			return false
		}
	}
	return true
}

// Covers: L5-TOOL-02
func TestPermissionManager_should_never_auto_approve_critical_in_yolo(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: time.Second,
		MaxRetries:     3,
	}
	mgr := NewPermissionManager(cfg)
	mgr.SetUserConfig(&config.UserConfig{
		YOLO: config.YOLOConfig{
			Enabled:          true,
			AutoApproveTools: true,
		},
	})

	if mgr.shouldAutoApprove("danger_tool", types.RiskLevelCritical) {
		t.Fatal("CRITICAL risk must never auto-approve in YOLO mode")
	}
	if !mgr.shouldAutoApprove("read_file", types.RiskLevelLow) {
		t.Fatal("LOW risk should auto-approve in YOLO mode")
	}
	if !mgr.shouldAutoApprove("write_file", types.RiskLevelMedium) {
		t.Fatal("MEDIUM risk should auto-approve in YOLO mode")
	}
}

func TestPermissionManager_ListPending(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 60 * time.Second,
		MaxRetries:     3,
	}

	mgr := NewPermissionManager(cfg)

	// Create some requests
	request1 := types.NewPermissionRequest("req_1", "sess_1", "bash", types.RiskLevelMedium, cfg.DefaultTimeout)
	request2 := types.NewPermissionRequest("req_2", "sess_2", "read", types.RiskLevelLow, cfg.DefaultTimeout)

	mgr.mu.Lock()
	mgr.requests[request1.ID] = request1
	mgr.requests[request2.ID] = request2
	mgr.mu.Unlock()

	pending := mgr.ListPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending requests, got %d", len(pending))
	}
}
