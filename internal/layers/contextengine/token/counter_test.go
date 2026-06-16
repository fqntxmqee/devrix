package token_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/token"
)

// T: D2-S4-A01-T01
func TestCounter_should_implement_shared_contract(t *testing.T) {
	var _ interface{ CountText(string) int } = token.NewCounter()
}
