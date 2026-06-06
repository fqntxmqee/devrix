package gateway

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

func TestPermissionManager_Request(t *testing.T) {
	cfg := &config.PermissionConfig{
		DefaultTimeout: 100 * time.Millisecond,
		MaxRetries:     3,
	}

	mgr := NewPermissionManager(cfg)

	// This test will timeout quickly due to short timeout
	approved := mgr.Request("sess_123", "bash", "ls -la", types.RiskLevelMedium)

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
