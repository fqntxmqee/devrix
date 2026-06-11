package gateway

import "testing"

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0s"},
		{"negative", -1, "0s"},
		{"sub_second_round_down", 400, "0s"},
		{"sub_second_round_up", 600, "1s"},
		{"exact_second", 1000, "1s"},
		{"user_example_7655", 7655, "8s"},
		{"30_seconds", 30000, "30s"},
		{"just_under_minute", 59400, "59s"},
		{"rounds_up_to_minute", 59500, "1m"},
		{"exact_minute", 60000, "1m"},
		{"two_minutes_two_seconds", 122000, "2m2s"},
		{"five_minutes", 300000, "5m"},
		{"complex_long", 3661000, "61m1s"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formatDuration(c.in)
			if got != c.want {
				t.Fatalf("formatDuration(%d) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0 tokens"},
		{"negative_clamped", -10, "0 tokens"},
		{"small", 100, "100 tokens"},
		{"under_threshold", 9999, "9999 tokens"},
		{"exact_threshold", 10000, "1.0w tokens"},
		{"user_example_22000", 22000, "2.2w tokens"},
		{"large", 123456, "12.3w tokens"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formatTokens(c.in)
			if got != c.want {
				t.Fatalf("formatTokens(%d) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseInt64Safe(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"abc", 0},
		{"123", 123},
		{"-5", -5},
	}
	for _, c := range cases {
		got := parseInt64Safe(c.in)
		if got != c.want {
			t.Fatalf("parseInt64Safe(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBuildCompletionSummary(t *testing.T) {
	cases := []struct {
		name     string
		duration string
		usage    string
		model    string
		ctxPct   string
		want     string
	}{
		{
			name:     "with_model",
			duration: "122000",
			usage:    "22000",
			model:    "claude-sonnet-4-6",
			ctxPct:   "",
			want:     "用时: 2m2s, 消耗: 2.2w tokens, 模型: claude-sonnet-4-6",
		},
		{
			name:     "without_model",
			duration: "7655",
			usage:    "1500",
			model:    "",
			ctxPct:   "",
			want:     "用时: 8s, 消耗: 1500 tokens",
		},
		{
			name:     "all_empty",
			duration: "",
			usage:    "",
			model:    "",
			ctxPct:   "",
			want:     "用时: 0s, 消耗: 0 tokens",
		},
		{
			name:     "garbage_inputs",
			duration: "abc",
			usage:    "def",
			model:    "gpt-4",
			ctxPct:   "",
			want:     "用时: 0s, 消耗: 0 tokens, 模型: gpt-4",
		},
		// DM-20260611-008：ctx 比例注入
		{
			name:     "with_model_with_ctx",
			duration: "7655",
			usage:    "1500",
			model:    "claude-sonnet-4-6",
			ctxPct:   "12",
			want:     "用时: 8s, 消耗: 1500 tokens, ctx: 12%, 模型: claude-sonnet-4-6",
		},
		{
			name:     "with_model_ctx_zero_omitted",
			duration: "7655",
			usage:    "1500",
			model:    "claude-sonnet-4-6",
			ctxPct:   "0",
			want:     "用时: 8s, 消耗: 1500 tokens, 模型: claude-sonnet-4-6",
		},
		{
			name:     "without_model_with_ctx",
			duration: "7655",
			usage:    "1500",
			model:    "",
			ctxPct:   "12",
			want:     "用时: 8s, 消耗: 1500 tokens, ctx: 12%",
		},
		{
			name:     "ctx_hundred_clamp_kept",
			duration: "7655",
			usage:    "1500",
			model:    "claude-sonnet-4-6",
			ctxPct:   "100",
			want:     "用时: 8s, 消耗: 1500 tokens, ctx: 100%, 模型: claude-sonnet-4-6",
		},
		{
			name:     "ctx_garbage_omitted",
			duration: "7655",
			usage:    "1500",
			model:    "claude-sonnet-4-6",
			ctxPct:   "abc",
			want:     "用时: 8s, 消耗: 1500 tokens, 模型: claude-sonnet-4-6",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildCompletionSummary(c.duration, c.usage, c.model, c.ctxPct)
			if got != c.want {
				t.Fatalf("buildCompletionSummary(%q,%q,%q,%q) = %q, want %q",
					c.duration, c.usage, c.model, c.ctxPct, got, c.want)
			}
		})
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
		{"negative_max_returns_zero", 1000, -1, 0},
		{"zero_prompt_returns_zero", 0, 128000, 0},
		{"both_zero_returns_zero", 0, 0, 0},
		{"exact_full", 128000, 128000, 100},
		{"quarter", 32000, 128000, 25},
		{"under_quarter_truncates", 100, 128000, 0},
		{"over_max_clamped", 200000, 128000, 100},
		{"tiny_max", 100, 50, 100},
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
