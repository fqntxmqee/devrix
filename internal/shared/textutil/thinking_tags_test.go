package textutil

import "testing"

func TestStripPriorOutputSummary_should_remove_complete_block(t *testing.T) {
	t.Parallel()
	raw := `some visible text
<prior-output-summary>
first 800 chars of prior turn
...[middle 12345 chars truncated; see /path/to/file]
last 200 chars of prior turn
</prior-output-summary>
trailing visible text`
	got := StripPriorOutputSummary(raw)
	want := "some visible text\ntrailing visible text"
	if got != want {
		t.Fatalf("StripPriorOutputSummary() = %q, want %q", got, want)
	}
}

func TestStripPriorOutputSummary_should_remove_multiple_blocks(t *testing.T) {
	t.Parallel()
	raw := `A<prior-output-summary>fold 1</prior-output-summary>B<prior-output-summary>fold 2</prior-output-summary>C`
	got := StripPriorOutputSummary(raw)
	want := "A\nB\nC"
	if got != want {
		t.Fatalf("StripPriorOutputSummary() = %q, want %q", got, want)
	}
}

func TestStripPriorOutputSummary_should_drop_unbalanced_open_tag(t *testing.T) {
	t.Parallel()
	// Pathological case: LLM emitted the open tag but not the close. We
	// drop the open tag and everything after it — losing a tail fragment
	// is better than rendering half a tag to the user.
	raw := `good prefix <prior-output-summary>
fold content that has no closing tag and just keeps going`
	got := StripPriorOutputSummary(raw)
	want := "good prefix"
	if got != want {
		t.Fatalf("StripPriorOutputSummary() = %q, want %q", got, want)
	}
}

func TestStripPriorOutputSummary_should_preserve_plain_text(t *testing.T) {
	t.Parallel()
	raw := "plain text without any markers"
	got := StripPriorOutputSummary(raw)
	if got != raw {
		t.Fatalf("StripPriorOutputSummary() = %q, want %q", got, raw)
	}
}

func TestStripPriorOutputSummary_should_be_case_insensitive(t *testing.T) {
	t.Parallel()
	raw := "prefix<PRIOR-OUTPUT-SUMMARY>fold</prior-output-summary>suffix"
	got := StripPriorOutputSummary(raw)
	want := "prefix\nsuffix"
	if got != want {
		t.Fatalf("StripPriorOutputSummary() = %q, want %q", got, want)
	}
}

func TestStripAssistantInternalMarkers_should_strip_think_and_prior_output(t *testing.T) {
	t.Parallel()
	raw := `<think>internal reasoning</think>visible A
<prior-output-summary>fold</prior-output-summary>
visible B`
	got := StripAssistantInternalMarkers(raw)
	want := "visible A\nvisible B"
	if got != want {
		t.Fatalf("StripAssistantInternalMarkers() = %q, want %q", got, want)
	}
}

func TestThinkTagSplitter_should_split_complete_block(t *testing.T) {
	t.Parallel()

	var splitter ThinkTagSplitter
	thinking, content := splitter.Push("<think>plan reply</think>Hello")
	if thinking != "plan reply" {
		t.Fatalf("thinking = %q, want %q", thinking, "plan reply")
	}
	if content != "Hello" {
		t.Fatalf("content = %q, want %q", content, "Hello")
	}
}

func TestThinkTagSplitter_should_split_across_chunks(t *testing.T) {
	t.Parallel()

	var splitter ThinkTagSplitter
	thinking, content := splitter.Push("<think>part ")
	if thinking != "" || content != "" {
		t.Fatalf("partial chunk should not emit output yet, got thinking=%q content=%q", thinking, content)
	}
	thinking, content = splitter.Push("two</think>Hi")
	if thinking != "part two" {
		t.Fatalf("thinking = %q, want %q", thinking, "part two")
	}
	if content != "Hi" {
		t.Fatalf("content = %q, want %q", content, "Hi")
	}
}

func TestThinkTagSplitter_should_handle_redacted_thinking_before_answer(t *testing.T) {
	t.Parallel()

	var splitter ThinkTagSplitter
	thinking, content := splitter.Push("<think>reason</think>Answer")
	if thinking != "reason" {
		t.Fatalf("thinking = %q, want %q", thinking, "reason")
	}
	if content != "Answer" {
		t.Fatalf("content = %q, want %q", content, "Answer")
	}
}

func TestStripThinkingTags_should_remove_embedded_blocks(t *testing.T) {
	t.Parallel()

	raw := "<think>hidden</think> visible"
	got := StripThinkingTags(raw)
	if got != "visible" {
		t.Fatalf("StripThinkingTags() = %q, want %q", got, "visible")
	}
}

func TestStripThinkingTags_should_preserve_plain_text(t *testing.T) {
	t.Parallel()

	raw := "plain answer"
	got := StripThinkingTags(raw)
	if got != raw {
		t.Fatalf("StripThinkingTags() = %q, want %q", got, raw)
	}
}

func TestPriorOutputSummarySplitter_should_split_complete_block(t *testing.T) {
	t.Parallel()

	var splitter PriorOutputSummarySplitter
	summary, visible := splitter.Push("prefix<prior-output-summary>fold content</prior-output-summary>suffix")
	if summary != "fold content" {
		t.Fatalf("summary = %q, want %q", summary, "fold content")
	}
	if visible != "prefixsuffix" {
		t.Fatalf("visible = %q, want %q", visible, "prefixsuffix")
	}
}

func TestPriorOutputSummarySplitter_should_split_across_chunks(t *testing.T) {
	t.Parallel()

	var splitter PriorOutputSummarySplitter
	summary, visible := splitter.Push("Hello <prior-output-summa")
	if summary != "" {
		t.Fatalf("chunk 1 should not emit summary yet, got %q", summary)
	}
	if visible != "Hello " {
		t.Fatalf("chunk 1 should emit visible prefix only, got %q", visible)
	}

	summary, visible = splitter.Push("ry>fold body</prior-output-summary>world")
	if summary != "fold body" {
		t.Fatalf("summary = %q, want %q", summary, "fold body")
	}
	if visible != "world" {
		t.Fatalf("visible = %q, want %q", visible, "world")
	}
}

func TestPriorOutputSummarySplitter_should_drop_unbalanced_open(t *testing.T) {
	t.Parallel()

	var splitter PriorOutputSummarySplitter
	_, visible := splitter.Push("good prefix<prior-output-summary>orphaned fold body")
	if visible != "good prefix" {
		t.Fatalf("mid-stream visible = %q, want %q", visible, "good prefix")
	}

	summary, visible := splitter.Flush()
	if summary != "orphaned fold body" {
		t.Fatalf("Flush summary = %q, want %q", summary, "orphaned fold body")
	}
	if visible != "" {
		t.Fatalf("Flush visible = %q, want empty (unbalanced fold body must not surface)", visible)
	}
}

func TestPriorOutputSummarySplitter_should_handle_chained_blocks(t *testing.T) {
	t.Parallel()

	var splitter PriorOutputSummarySplitter
	_, visible := splitter.Push("A<prior-output-summary>fold 1</prior-output-summary>")
	summary, visible := splitter.Push("B<prior-output-summary>fold 2</prior-output-summary>C")
	if summary != "fold 2" {
		t.Fatalf("summary = %q, want %q", summary, "fold 2")
	}
	if visible != "BC" {
		t.Fatalf("visible = %q, want %q", visible, "BC")
	}
}

func TestPriorOutputSummarySplitter_should_preserve_plain_text(t *testing.T) {
	t.Parallel()

	var splitter PriorOutputSummarySplitter
	summary, visible := splitter.Push("just plain visible text with no markers")
	if summary != "" {
		t.Fatalf("summary = %q, want empty", summary)
	}
	if visible != "just plain visible text with no markers" {
		t.Fatalf("visible = %q, want verbatim input", visible)
	}
}