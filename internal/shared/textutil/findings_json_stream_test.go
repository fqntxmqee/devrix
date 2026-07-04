package textutil

import "testing"

func TestLooksLikeFindingsJSONStream_should_detect_json_chunks(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"scope": "internal/layers/contextengine/kernel/"`,
		`"findings": [`,
		`  "severity": "p1",`,
	}
	for _, c := range cases {
		if !LooksLikeFindingsJSONStream(c) {
			t.Fatalf("LooksLikeFindingsJSONStream(%q) = false, want true", c)
		}
	}
	if LooksLikeFindingsJSONStream("Reviewing kernel package structure.") {
		t.Fatal("plain prose should not match")
	}
}

func TestStripFindingsJSONBlocks_should_remove_fenced_json(t *testing.T) {
	t.Parallel()
	raw := "Intro text\n```json\n{\"findings\":[{\"severity\":\"p1\"}]}\n```\nTail"
	got := StripFindingsJSONBlocks(raw)
	want := "Intro text\n\nTail"
	if got != want {
		t.Fatalf("StripFindingsJSONBlocks() = %q, want %q", got, want)
	}
}
