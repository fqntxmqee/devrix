package capture

import "testing"

func TestBuildCompletionSummary_delegatesToPresent(t *testing.T) {
	got := buildCompletionSummary("7655", "1500", "claude-sonnet-4-6", "12")
	want := "用时: 8s, 消耗: 1500 tokens, ctx: 12%, 模型: claude-sonnet-4-6"
	if got != want {
		t.Fatalf("buildCompletionSummary() = %q, want %q", got, want)
	}
}

func TestComputeCtxPct(t *testing.T) {
	cases := []struct {
		name   string
		prompt int
		max    int
		want   int
	}{
		{"zero_max_returns_zero", 1000, 0, 0},
		{"quarter", 32000, 128000, 25},
		{"over_max_clamped", 200000, 128000, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ComputeCtxPct(c.prompt, c.max)
			if got != c.want {
				t.Fatalf("ComputeCtxPct(%d,%d) = %d, want %d", c.prompt, c.max, got, c.want)
			}
		})
	}
}
