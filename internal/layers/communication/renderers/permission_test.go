package renderers

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/shared/types"
)

func TestPermissionRenderer_should_render_card(t *testing.T) {
	r := NewPermissionRenderer(testANSI())

	out := captureStdout(t, func() {
		r.RenderCard(&types.PermissionRequest{
			ToolName:     "write_file",
			Description:  "write config file with sensitive data",
			InputPreview: "path=/etc/app\ncontent=secret",
			RiskLevel:    types.RiskLevelHigh,
		})
	})

	if !strings.Contains(out, "Permission Required") {
		t.Fatalf("expected card header, got %q", out)
	}
	if !strings.Contains(out, "write_file") || !strings.Contains(out, "[yes]") {
		t.Fatalf("expected tool and actions, got %q", out)
	}
}

func TestPermissionRenderer_formatRiskLevel_should_colorize(t *testing.T) {
	r := NewPermissionRenderer(testANSI())

	critical := r.formatRiskLevel(types.RiskLevelCritical)
	medium := r.formatRiskLevel(types.RiskLevelMedium)
	low := r.formatRiskLevel(types.RiskLevelLow)

	if !strings.Contains(critical, "[ERR]") {
		t.Fatalf("expected error color for critical, got %q", critical)
	}
	if !strings.Contains(medium, "[WARN]") {
		t.Fatalf("expected warning color for medium, got %q", medium)
	}
	if !strings.Contains(low, "[ASST]") {
		t.Fatalf("expected assistant color for low, got %q", low)
	}
}

func TestPadCenter_should_center_short_strings(t *testing.T) {
	got := padCenter("hi", 10)
	if len(got) != 10 {
		t.Fatalf("expected width 10, got len %d", len(got))
	}
	if !strings.HasPrefix(got, " ") || !strings.HasSuffix(got, " ") {
		t.Fatalf("expected padding on both sides, got %q", got)
	}
}

func TestPadCenter_should_truncate_long_strings(t *testing.T) {
	got := padCenter("hello world", 5)
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestWrapText_should_wrap_long_lines(t *testing.T) {
	text := "one two three four five six seven eight nine ten"
	got := wrapText(text, 12)
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected wrapped output, got %q", got)
	}
}
