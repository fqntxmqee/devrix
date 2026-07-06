package textutil

import (
	"strings"
	"testing"
)

// TestStripStructuredDeliverable_should_remove_xml_envelope (DM-20260706-006):
// Each XML-style deliverable tag pair is stripped; natural-language text
// before/after is preserved (separator collapses via TrimSpace, so adjacent
// text fragments join with a single newline).
func TestStripStructuredDeliverable_should_remove_xml_envelope(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "deliverable_schema_paired",
			in:   "intro text\n<deliverable_schema>findings_json</deliverable_schema>\noutro",
			want: "intro text\noutro",
		},
		{
			name: "findings_json_paired",
			in:   "analysis below\n<findings_json>{\"a\":1}</findings_json>\nending",
			want: "analysis below\nending",
		},
		{
			name: "deliverable_contract_paired",
			in:   "before\n<deliverable_contract>{\"x\":1}</deliverable_contract>\nafter",
			want: "before\nafter",
		},
		{
			name: "all_tags_combined",
			in:   "A\n<deliverable_schema>s1</deliverable_schema>B\n<findings_json>f</findings_json>C\n<deliverable_contract>c</deliverable_contract>D",
			want: "A\nB\nC\nD",
		},
		{
			name: "no_tags_unchanged",
			in:   "just plain text, no tags at all",
			want: "just plain text, no tags at all",
		},
		{
			name: "case_insensitive",
			in:   "A\n<DELIVERABLE_SCHEMA>s1</DELIVERABLE_SCHEMA>B",
			want: "A\nB",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := StripStructuredDeliverable(c.in)
			if got != c.want {
				t.Fatalf("StripStructuredDeliverable(%q) =\n%q\nwant\n%q", c.in, got, c.want)
			}
		})
	}
}

// TestStripStructuredDeliverable_should_drop_unbalanced_open:
// Unbalanced markers (open without close) drop everything from the open
// tag onward — defensive fallback so half-tags never reach the LLM.
func TestStripStructuredDeliverable_should_drop_unbalanced_open(t *testing.T) {
	t.Parallel()
	in := "kept text\n<deliverable_schema>findings_json"
	got := StripStructuredDeliverable(in)
	want := "kept text"
	if got != want {
		t.Fatalf("StripStructuredDeliverable(%q) = %q, want %q", in, got, want)
	}
}

// TestStripStructuredDeliverable_should_handle_mixed_with_findings_fence:
// Combined with StripFindingsJSONBlocks, the result should contain only
// the natural-language text.
func TestStripStructuredDeliverable_should_handle_mixed_with_findings_fence(t *testing.T) {
	t.Parallel()
	raw := "以下是我对 PR-A1 的分析。\n" +
		"<deliverable_schema>legacy_contract</deliverable_schema>\n" +
		"```findings_json\n" +
		"[{\"pr_id\":\"#163\",\"files\":[\"a.go\"]}]\n" +
		"```\n" +
		"建议深入阅读 observation.go。"
	// StripStructuredDeliverable only handles XML tags; the ```fence
	// stays. StripFindingsJSONBlocks handles the ```fence. Composition
	// happens in TranscriptReader.BuildPriorOutputSummary.
	onlyXML := StripStructuredDeliverable(raw)
	if strings.Contains(onlyXML, "<deliverable_schema>") {
		t.Fatalf("XML tag not stripped: %q", onlyXML)
	}
	if !strings.Contains(onlyXML, "```findings_json") {
		t.Fatalf("fence unexpectedly stripped by StripStructuredDeliverable: %q", onlyXML)
	}
	// Then chain StripFindingsJSONBlocks for full effect
	full := StripFindingsJSONBlocks(onlyXML)
	if strings.Contains(full, "findings_json") || strings.Contains(full, "observation.go\",") {
		t.Fatalf("fenced JSON not stripped: %q", full)
	}
	if !strings.Contains(full, "建议深入阅读 observation.go") {
		t.Fatalf("natural-language tail lost: %q", full)
	}
}
