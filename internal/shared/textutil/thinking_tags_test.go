package textutil

import "testing"

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
	if got != "plain answer" {
		t.Fatalf("StripThinkingTags() = %q, want %q", got, "plain answer")
	}
}
