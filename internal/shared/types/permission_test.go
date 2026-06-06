package types

import (
	"testing"
	"time"
)

func TestPermissionRequest_IsExpired(t *testing.T) {
	req := &PermissionRequest{
		ID:        "req_1",
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}

	if !req.IsExpired() {
		t.Error("expected IsExpired to return true for past expiry time")
	}
}

func TestPermissionRequest_IsPending(t *testing.T) {
	req := &PermissionRequest{
		ID:        "req_1",
		Status:    PermissionStatusPending,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	if !req.IsPending() {
		t.Error("expected IsPending to return true")
	}
}

func TestPermissionRequest_Resolve(t *testing.T) {
	req := &PermissionRequest{
		ID:     "req_1",
		Status: PermissionStatusPending,
	}

	req.Resolve(true)
	if req.Status != PermissionStatusApproved {
		t.Errorf("expected status 'approved', got '%s'", req.Status)
	}
	if req.Response == nil || !*req.Response {
		t.Error("expected response to be true")
	}

	req.Resolve(false)
	if req.Status != PermissionStatusDenied {
		t.Errorf("expected status 'denied', got '%s'", req.Status)
	}
}

func TestPermissionRequest_Expire(t *testing.T) {
	req := &PermissionRequest{
		ID:     "req_1",
		Status: PermissionStatusPending,
	}

	req.Expire()
	if req.Status != PermissionStatusExpired {
		t.Errorf("expected status 'expired', got '%s'", req.Status)
	}
}

func TestNewPermissionRequest(t *testing.T) {
	req := NewPermissionRequest("req_1", "sess_1", "bash", RiskLevelMedium, 60*time.Second)

	if req.ID != "req_1" {
		t.Errorf("expected ID 'req_1', got '%s'", req.ID)
	}
	if req.SessionID != "sess_1" {
		t.Errorf("expected SessionID 'sess_1', got '%s'", req.SessionID)
	}
	if req.Status != PermissionStatusPending {
		t.Errorf("expected status 'pending', got '%s'", req.Status)
	}
	if time.Until(req.ExpiresAt) < 59*time.Second {
		t.Error("expected expiresAt to be approximately 60 seconds from now")
	}
}
