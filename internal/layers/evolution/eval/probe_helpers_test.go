package eval

import "testing"

func TestWordJaccard_Identical(t *testing.T) {
	if got := wordJaccard("hello world", "hello world"); got != 1.0 {
		t.Errorf("wordJaccard = %v, want 1.0", got)
	}
}

func TestWordJaccard_PartialOverlap(t *testing.T) {
	got := wordJaccard("user service order service", "user order notification")
	if got <= 0 || got >= 1 {
		t.Errorf("wordJaccard = %v, want between 0 and 1", got)
	}
}

func TestIsolationRate_NoLeaks(t *testing.T) {
	if got := isolationRate("only agent A context", []string{"agent B secret"}); got != 1.0 {
		t.Errorf("isolationRate = %v, want 1.0", got)
	}
}

func TestIsolationRate_WithLeak(t *testing.T) {
	if got := isolationRate("contains agent B secret here", []string{"agent B secret"}); got != 0.0 {
		t.Errorf("isolationRate = %v, want 0.0", got)
	}
}
