package auth

import (
	"testing"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestAuthService_Register_and_Validate(t *testing.T) {
	svc := NewAuthService(&types.AuthConfig{
		Secret:      "test-secret",
		TokenExpiry: time.Hour,
		Issuer:      "devrix-test",
	})

	reg, err := svc.Register("feishu", "test-secret")
	if err != nil {
		t.Fatalf("Register() err = %v", err)
	}
	if !reg.Success || reg.Token == nil {
		t.Fatalf("Register() = %+v", reg)
	}

	adapterID, err := svc.Validate(reg.Token.Token)
	if err != nil {
		t.Fatalf("Validate() err = %v", err)
	}
	if adapterID != "feishu" {
		t.Fatalf("adapterID = %q, want feishu", adapterID)
	}
}

func TestAuthService_Register_rejects_invalid_secret(t *testing.T) {
	svc := NewAuthService(&types.AuthConfig{Secret: "expected"})

	reg, err := svc.Register("cli", "wrong")
	if err != nil {
		t.Fatalf("Register() err = %v", err)
	}
	if reg.Success {
		t.Fatal("expected Register failure for invalid secret")
	}
}

func TestAuthService_Validate_rejects_empty_token(t *testing.T) {
	svc := NewAuthService(&types.AuthConfig{Secret: "expected"})
	if _, err := svc.Validate(""); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestAuthService_Refresh_rotates_token(t *testing.T) {
	svc := NewAuthService(&types.AuthConfig{
		Secret:      "rotate-secret",
		TokenExpiry: time.Hour,
		Issuer:      "devrix-test",
	})
	reg, err := svc.Register("dingtalk", "rotate-secret")
	if err != nil || !reg.Success {
		t.Fatalf("Register() = %+v, err = %v", reg, err)
	}
	oldToken := reg.Token.Token
	time.Sleep(time.Second)

	refreshed, err := svc.Refresh(oldToken)
	if err != nil || !refreshed.Success {
		t.Fatalf("Refresh() = %+v, err = %v", refreshed, err)
	}
	if refreshed.Token.Token == oldToken {
		t.Fatal("expected new token after refresh")
	}
	if _, err := svc.Validate(oldToken); err == nil {
		t.Fatal("old token should be invalid after refresh")
	}
	if adapterID, err := svc.Validate(refreshed.Token.Token); err != nil || adapterID != "dingtalk" {
		t.Fatalf("Validate(new) = %q, err = %v", adapterID, err)
	}
}
