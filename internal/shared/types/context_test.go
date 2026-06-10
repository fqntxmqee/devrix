package types_test

import (
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestCompressionReport_Ratio_should_compute_token_ratio(t *testing.T) {
	r := types.CompressionReport{OriginalTokens: 100, CompressedTokens: 42}
	if got := r.Ratio(); got != 0.42 {
		t.Fatalf("Ratio() = %v want 0.42", got)
	}
	if (types.CompressionReport{}).Ratio() != 1 {
		t.Fatal("zero original should return 1")
	}
}
