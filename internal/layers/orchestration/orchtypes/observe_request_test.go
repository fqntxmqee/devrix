package orchtypes

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
)

func TestObserveRequest_New_Success(t *testing.T) {
	r, err := NewObserveRequest("sess-1", "hello", nil)
	if err != nil {
		t.Fatalf("NewObserveRequest: unexpected error: %v", err)
	}
	if r.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", r.SessionID, "sess-1")
	}
	if r.Message != "hello" {
		t.Errorf("Message = %q, want %q", r.Message, "hello")
	}
	if r.Prior != nil {
		t.Errorf("Prior = %v, want nil", r.Prior)
	}
}

func TestObserveRequest_New_EmptySessionID(t *testing.T) {
	_, err := NewObserveRequest("", "hello", nil)
	if err == nil {
		t.Fatal("NewObserveRequest with empty SessionID: expected error, got nil")
	}
}

func TestObserveRequest_New_EmptyMessage(t *testing.T) {
	_, err := NewObserveRequest("sess-1", "", nil)
	if err == nil {
		t.Fatal("NewObserveRequest with empty Message: expected error, got nil")
	}
}

func TestObserveRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ObserveRequest
		wantErr bool
	}{
		{
			name:    "valid_with_nil_prior",
			req:     ObserveRequest{SessionID: "sess-1", Message: "hello"},
			wantErr: false,
		},
		{
			name:    "valid_with_prior",
			req:     ObserveRequest{SessionID: "sess-1", Message: "hello", Prior: learn.BuildAdaptivePrior(nil, learn.TrackModeDeveloper)},
			wantErr: false,
		},
		{
			name:    "empty_session",
			req:     ObserveRequest{SessionID: "", Message: "hello"},
			wantErr: true,
		},
		{
			name:    "empty_message",
			req:     ObserveRequest{SessionID: "sess-1", Message: ""},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Validate: expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate: unexpected error: %v", err)
			}
		})
	}
}

func TestObserveRequest_EffectivePrior_NilPrior_DefaultDeveloper(t *testing.T) {
	r := ObserveRequest{SessionID: "sess-1", Message: "hello", Prior: nil}
	p := r.EffectivePrior()
	if p == nil {
		t.Fatal("EffectivePrior with nil Prior: expected non-nil default")
	}
	// DefaultDeveloperPrior is Beta(5,3) → Mean = 5/8 = 0.625
	want := learn.BetaPrior{Alpha: 5, Beta: 3}
	if p.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", p.PriorBeta, want)
	}
}

func TestObserveRequest_EffectivePrior_NonNilPrior_ReturnAsIs(t *testing.T) {
	rep, err := learn.NewReputationEvidence("sess-1", learn.TrackModeDeveloper)
	if err != nil {
		t.Fatalf("NewReputationEvidence: %v", err)
	}
	rep.Alpha = 3
	rep.Beta = 0
	prior := learn.BuildAdaptivePrior(rep, learn.TrackModeDeveloper)

	r := ObserveRequest{SessionID: "sess-1", Message: "hello", Prior: prior}
	got := r.EffectivePrior()
	if got != prior {
		t.Errorf("EffectivePrior with non-nil Prior: should return the same pointer")
	}
	// Verify it's Beta(8,3) = Developer(5,3) + rep(3,0)
	want := learn.BetaPrior{Alpha: 8, Beta: 3}
	if got.PriorBeta != want {
		t.Errorf("PriorBeta = %+v, want %+v", got.PriorBeta, want)
	}
}
