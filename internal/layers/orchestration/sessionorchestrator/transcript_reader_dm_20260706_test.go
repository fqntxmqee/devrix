package sessionorchestrator

import (
	"strings"
	"testing"
)

// TestBuildPriorOutputSummary_should_strip_deliverable_template (DM-20260706-006):
// When the prior turn's text is a long findings_json template, the injected
// prior-output-summary should be (a) the XML envelope stripped and (b)
// truncated so the next turn's LLM cannot mimic the structure.
func TestBuildPriorOutputSummary_should_strip_deliverable_template(t *testing.T) {
	t.Parallel()
	r := NewTranscriptReader("")

	// Mimic a typical LLM finalText for a review task: prose intro +
	// <deliverable_schema> envelope + ```findings_json``` code block.
	turnText := "以下是对 PR-A1 的变更分析。\n" +
		"<deliverable_schema>legacy_contract</deliverable_schema>\n" +
		"```findings_json\n" +
		"{\"pr_id\":\"#163\",\"files\":[\"observation.go\",\"uncertainty_report.go\"]}\n" +
		"```\n" +
		strings.Repeat("x", 1500) // padding to trigger truncation

	got := r.BuildPriorOutputSummary([]string{turnText})

	// The envelope + code fence must be gone
	if strings.Contains(got, "<deliverable_schema>") {
		t.Fatalf("deliverable_schema tag leaked into prior summary: %q", got)
	}
	if strings.Contains(got, "findings_json") {
		t.Fatalf("findings_json code-fence leaked into prior summary: %q", got)
	}
	if strings.Contains(got, "observation.go\"") {
		t.Fatalf("JSON content leaked into prior summary: %q", got)
	}
	// The natural-language intro must survive
	if !strings.Contains(got, "以下是对 PR-A1 的变更分析") {
		t.Fatalf("natural-language intro lost: %q", got)
	}
	// The per-turn truncation must be enforced (capped at ~800 chars + ellipsis)
	if !strings.Contains(got, "…") {
		t.Fatalf("truncation ellipsis missing: %q", got)
	}
	// The wrapper must still be the <prior-output-summary> envelope
	if !strings.HasPrefix(got, "<prior-output-summary>") || !strings.HasSuffix(got, "</prior-output-summary>") {
		t.Fatalf("envelope missing: %q", got)
	}
}

// TestBuildPriorOutputSummary_should_keep_short_text_intact (DM-20260706-006):
// Short, plain natural-language turns must be passed through verbatim
// (no truncation, no envelope stripping). This is the happy-path case
// for casual conversations.
func TestBuildPriorOutputSummary_should_keep_short_text_intact(t *testing.T) {
	t.Parallel()
	r := NewTranscriptReader("")

	got := r.BuildPriorOutputSummary([]string{"好,这是个 PR 标题。", "继续看 spec_delta.md 文件。"})
	if !strings.Contains(got, "[turn 1] 好,这是个 PR 标题。") {
		t.Fatalf("turn 1 missing: %q", got)
	}
	if !strings.Contains(got, "[turn 2] 继续看 spec_delta.md 文件。") {
		t.Fatalf("turn 2 missing: %q", got)
	}
	if strings.Contains(got, "…") {
		t.Fatalf("truncation triggered on short text: %q", got)
	}
}

// TestBuildPriorOutputSummary_empty_input: empty input → empty output
// (regression for the legacy nil/empty short-circuits).
func TestBuildPriorOutputSummary_empty_input(t *testing.T) {
	t.Parallel()
	r := NewTranscriptReader("")
	if got := r.BuildPriorOutputSummary(nil); got != "" {
		t.Fatalf("nil input → %q, want empty", got)
	}
	if got := r.BuildPriorOutputSummary([]string{}); got != "" {
		t.Fatalf("empty input → %q, want empty", got)
	}
}
