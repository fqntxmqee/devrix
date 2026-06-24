package escape

import (
	"encoding/json"
	"strings"
	"testing"
)

// L1-Extra: EscapeAction 6 类 String() 互不相同
func TestEscapeAction_String_AllUnique(t *testing.T) {
	actions := []EscapeAction{
		EscapeContinue,
		EscalateToRule,
		EscalateToHuman,
		EscapeForceExit,
		EscapeAbortWithAudit,
		EscapePendingHuman,
	}
	seen := make(map[string]bool)
	for _, a := range actions {
		s := a.String()
		if seen[s] {
			t.Errorf("duplicate string for EscapeAction(%d): %q", uint8(a), s)
		}
		seen[s] = true
	}
	if len(seen) != 6 {
		t.Errorf("got %d unique strings, want 6", len(seen))
	}
}

// L1-Extra: EscapeAction MarshalJSON 输出 string
func TestEscapeAction_MarshalJSON(t *testing.T) {
	tests := []struct {
		action EscapeAction
		want   string
	}{
		{EscapeContinue, `"continue"`},
		{EscalateToRule, `"escalate_to_rule"`},
		{EscalateToHuman, `"escalate_to_human"`},
		{EscapeForceExit, `"force_exit"`},
		{EscapeAbortWithAudit, `"abort_with_audit"`},
		{EscapePendingHuman, `"pending_human"`},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			data, err := json.Marshal(tc.action)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(data) != tc.want {
				t.Errorf("Marshal(%d) = %s, want %s", uint8(tc.action), string(data), tc.want)
			}
		})
	}
}

// L1-Extra: EscapeAction UnmarshalJSON 接受 string
func TestEscapeAction_UnmarshalJSON(t *testing.T) {
	var a EscapeAction
	if err := json.Unmarshal([]byte(`"force_exit"`), &a); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if a != EscapeForceExit {
		t.Errorf("Unmarshal: got %d, want EscapeForceExit=%d", uint8(a), uint8(EscapeForceExit))
	}
}

// L1-Extra: EscapeAction 未知值返回 error
func TestEscapeAction_UnmarshalJSON_Unknown(t *testing.T) {
	var a EscapeAction
	err := json.Unmarshal([]byte(`"unknown_action"`), &a)
	if err == nil {
		t.Errorf("Unmarshal unknown should fail")
	}
	if !strings.Contains(err.Error(), "unknown EscapeAction") {
		t.Errorf("error should mention unknown EscapeAction, got: %v", err)
	}
}

// L1-Extra: EscapeDecision 9 字段构造
func TestEscapeDecision_Construction(t *testing.T) {
	d := EscapeDecision{
		Action:     EscapePendingHuman,
		Reason:     "test",
		AuditLevel: 1,
		Depth:      3,
		PendingID:  "pid-123",
	}
	if d.Action != EscapePendingHuman {
		t.Errorf("Action = %d, want %d", uint8(d.Action), uint8(EscapePendingHuman))
	}
	if d.PendingID != "pid-123" {
		t.Errorf("PendingID = %q, want pid-123", d.PendingID)
	}
}

// L1-Extra: NewPendingID 返回 UUID v4 格式
func TestNewPendingID_UUIDFormat(t *testing.T) {
	id := NewPendingID()
	if len(id) != 36 {
		t.Errorf("PendingID length = %d, want 36 (UUID v4)", len(id))
	}
	// UUID v4 format: 8-4-4-4-12
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Errorf("PendingID format: expected 5 dash-separated parts, got %d", len(parts))
	}
}