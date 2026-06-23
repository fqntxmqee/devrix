package orchtypes

import (
	"errors"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// T: D7-S13-A48-T04 — ProcessRequest.TrackMode field + validation
// ─────────────────────────────────────────────────────────────────────────

// TestProcessRequest_ZeroValue_TrackModeEmpty verifies that the zero value
// of ProcessRequest has TrackMode="" (semantic: treat as developer).
func TestProcessRequest_ZeroValue_TrackModeEmpty(t *testing.T) {
	req := ProcessRequest{}
	if req.TrackMode != "" {
		t.Errorf("zero value TrackMode = %q, want \"\"", req.TrackMode)
	}
}

func TestNewProcessRequest_EmptySession_FailFast(t *testing.T) {
	_, err := NewProcessRequest("", "hello", "")
	if !errors.Is(err, ErrProcessRequestSessionIDEmpty) {
		t.Errorf("err = %v, want ErrProcessRequestSessionIDEmpty", err)
	}
}

func TestNewProcessRequest_EmptyMessage_FailFast(t *testing.T) {
	_, err := NewProcessRequest("sess_1", "", "")
	if !errors.Is(err, ErrProcessRequestMessageEmpty) {
		t.Errorf("err = %v, want ErrProcessRequestMessageEmpty", err)
	}
}

func TestNewProcessRequest_TrackModeEmpty_Accepts(t *testing.T) {
	req, err := NewProcessRequest("sess_1", "hi", "")
	if err != nil {
		t.Fatalf("NewProcessRequest: %v", err)
	}
	if req.TrackMode != "" {
		t.Errorf("TrackMode = %q, want \"\"", req.TrackMode)
	}
}

func TestNewProcessRequest_TrackModeDeveloper_Accepts(t *testing.T) {
	req, err := NewProcessRequest("sess_1", "hi", TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewProcessRequest: %v", err)
	}
	if req.TrackMode != TrackModeDeveloper {
		t.Errorf("TrackMode = %q, want %q", req.TrackMode, TrackModeDeveloper)
	}
}

func TestNewProcessRequest_TrackModeOperator_Accepts(t *testing.T) {
	req, err := NewProcessRequest("sess_1", "hi", TrackModeOperator)
	if err != nil {
		t.Fatalf("NewProcessRequest: %v", err)
	}
	if req.TrackMode != TrackModeOperator {
		t.Errorf("TrackMode = %q, want %q", req.TrackMode, TrackModeOperator)
	}
}

func TestNewProcessRequest_TrackModeInvalid_FailFast(t *testing.T) {
	_, err := NewProcessRequest("sess_1", "hi", "garbage")
	if !errors.Is(err, ErrProcessRequestInvalidTrackMode) {
		t.Errorf("err = %v, want ErrProcessRequestInvalidTrackMode", err)
	}
}

// TestNewProcessRequest_RoundTripAllFields verifies that the constructor
// populates all fields correctly (SessionID + Message + TrackMode).
func TestNewProcessRequest_RoundTripAllFields(t *testing.T) {
	req, err := NewProcessRequest("sess_1", "hi there", TrackModeOperator)
	if err != nil {
		t.Fatalf("NewProcessRequest: %v", err)
	}
	if req.SessionID != "sess_1" {
		t.Errorf("SessionID = %q, want %q", req.SessionID, "sess_1")
	}
	if req.Message != "hi there" {
		t.Errorf("Message = %q, want %q", req.Message, "hi there")
	}
	if req.TrackMode != TrackModeOperator {
		t.Errorf("TrackMode = %q, want %q", req.TrackMode, TrackModeOperator)
	}
}
