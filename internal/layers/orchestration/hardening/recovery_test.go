package hardening

import (
	"errors"
	"testing"

	sherrors "github.com/devrix/devrix/internal/shared/errors"
)

func TestIsContextLengthError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"code", sherrors.WithCode(sherrors.CodeContextExceeded, "ctx exceeded", errors.New("x")), true},
		{"413 text", errors.New("HTTP 413 prompt too long"), true},
		{"other", errors.New("500 internal"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsContextLengthError(tc.err); got != tc.want {
				t.Fatalf("IsContextLengthError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsOverloadOr5xx(t *testing.T) {
	if !IsOverloadOr5xx(errors.New("503 service unavailable")) {
		t.Fatal("expected overload detection")
	}
	if IsOverloadOr5xx(sherrors.WithCode(sherrors.CodeContextExceeded, "ctx exceeded", errors.New("x"))) {
		t.Fatal("context length must not count as overload")
	}
}

func TestNeedsMaxOutputTokenRecovery(t *testing.T) {
	if !NeedsMaxOutputTokenRecovery("length") {
		t.Fatal("finish_reason=length should trigger recovery")
	}
	if NeedsMaxOutputTokenRecovery("stop") {
		t.Fatal("finish_reason=stop should not trigger recovery")
	}
}