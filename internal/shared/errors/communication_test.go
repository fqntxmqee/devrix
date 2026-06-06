package errors

import (
	"errors"
	"testing"
)

func TestErrorCode(t *testing.T) {
	err := NewSessionNotFoundError("sess_123")
	code := ErrorCode(err)

	if code != "COMM_SESSION_NOT_FOUND_1001" {
		t.Errorf("expected error code 'COMM_SESSION_NOT_FOUND_1001', got '%s'", code)
	}
}

func TestErrorCode_NoCode(t *testing.T) {
	err := errors.New("plain error")
	code := ErrorCode(err)

	if code != "" {
		t.Errorf("expected empty error code, got '%s'", code)
	}
}

func TestSentinelError(t *testing.T) {
	err := NewSessionNotFoundError("sess_123")

	if err.Error() != "session not found: sess_123" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if !errors.Is(err, ErrSessionNotFound) {
		t.Error("expected errors.Is to return true for ErrSessionNotFound")
	}
}

func TestNewPermissionTimeoutError(t *testing.T) {
	err := NewPermissionTimeoutError("req_123")

	if err.Code != "COMM_PERMISSION_TIMEOUT_3002" {
		t.Errorf("expected error code 'COMM_PERMISSION_TIMEOUT_3002', got '%s'", err.Code)
	}

	if !errors.Is(err, ErrPermissionTimeout) {
		t.Error("expected errors.Is to return true for ErrPermissionTimeout")
	}
}
