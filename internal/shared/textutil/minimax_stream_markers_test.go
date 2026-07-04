package textutil

import "testing"

func TestStripMiniMaxStreamMarkers_should_remove_delimiter_tokens(t *testing.T) {
	t.Parallel()
	raw := "Hello ]<[minimax[>[ world ]<[minimax[>["
	got := StripMiniMaxStreamMarkers(raw)
	want := "Hello  world "
	if got != want {
		t.Fatalf("StripMiniMaxStreamMarkers() = %q, want %q", got, want)
	}
}

func TestStripMiniMaxStreamMarkers_should_preserve_plain_text(t *testing.T) {
	t.Parallel()
	raw := "plain visible answer"
	got := StripMiniMaxStreamMarkers(raw)
	if got != raw {
		t.Fatalf("StripMiniMaxStreamMarkers() = %q, want %q", got, raw)
	}
}

func TestStripAssistantInternalMarkers_should_strip_minimax_delimiters(t *testing.T) {
	t.Parallel()
	raw := "]<[minimax[>[visible text"
	got := StripAssistantInternalMarkers(raw)
	want := "visible text"
	if got != want {
		t.Fatalf("StripAssistantInternalMarkers() = %q, want %q", got, want)
	}
}
