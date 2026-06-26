// DM-20260626-009 follow-up: dedup_test.go was rewritten to drop the
// 3 deleted dedup helpers (DedupRepeatedText / DedupRepeatedTextIterative
// / DedupAdjacentRepeats) and retain only StripToolCallXML tests. The
// deleted helpers' tests lived in this file alongside StripToolCallXML;
// the helpers themselves were deleted in the same change because they
// were a minimax M2.7 streaming-replay band-aid at the wrong layer
// (D1 adapter instead of D3 gateway). See
// internal/shared/textutil/tool_call_xml.go for StripToolCallXML.
package textutil

import (
	"strings"
	"testing"
)

// TestStripToolCallXML_removes_function_calls_block (DM-20260626-009
// preserved): minimax M2.7 streaming emits tool calls as inline XML
// inside the text channel rather than as structured tool_use events.
// Without stripping, the user sees <function_calls>, <invoke>, and
// <parameter> blocks rendered on the reply card. Real example from a
// 2026-06-26 streaming run:
//
//	"让我先\n\n<function_calls>\n<invoke name=\"bash\">\n<parameter
//	name=\"command\">pwd</parameter>\n</invoke>\n</function_calls>"
//	→ "让我先"
func TestStripToolCallXML_removes_function_calls_block(t *testing.T) {
	input := "让我先\n\n<function_calls>\n<invoke name=\"bash\">\n<parameter name=\"command\">pwd ls -la</parameter>\n</invoke>\n</function_calls>\n\n好的，我看看。"
	got := StripToolCallXML(input)
	if strings.Contains(got, "<") || strings.Contains(got, "function_calls") {
		t.Errorf("StripToolCallXML left XML in result: %q", got)
	}
	if !strings.Contains(got, "让我先") {
		t.Errorf("StripToolCallXML lost surrounding text: %q", got)
	}
	if !strings.Contains(got, "好的") {
		t.Errorf("StripToolCallXML lost tail text: %q", got)
	}
}

// TestStripToolCallXML_handles_orphan_closing_tag asserts that an
// orphan closing tag (no matching open tag, or where the open tag was
// already removed) is also stripped. minimax M2.7 streaming sometimes
// leaves a stray `</markdown>` at the tail of a chunk that the user
// sees as `-markdown>` on the live card.
func TestStripToolCallXML_handles_orphan_closing_tag(t *testing.T) {
	input := "好的，我来</markdown>看看代码"
	got := StripToolCallXML(input)
	if strings.Contains(got, "markdown") {
		t.Errorf("StripToolCallXML left orphan closing tag: %q", got)
	}
	if !strings.Contains(got, "好的") || !strings.Contains(got, "看看代码") {
		t.Errorf("StripToolCallXML lost surrounding text: %q", got)
	}
}

// TestStripToolCallXML_preserves_natural_repetition (DM-20260626-009
// regression): after the dedup removal, StripToolCallXML must NOT
// collapse legitimate Chinese repetition. Real failure mode before
// the dedup removal: an LLM wrote "先看一下代码。先看一下代码的结构。",
// DedupAdjacentRepeats flagged "先看一下代码" as an echo and stripped
// the second occurrence, leaving "先看一下代码。的结构。" — readable but
// semantically mangled. Pin natural repetition flows through untouched.
func TestStripToolCallXML_preserves_natural_repetition(t *testing.T) {
	input := "先看一下代码。先看一下代码的结构。"
	got := StripToolCallXML(input)
	if got != input {
		t.Errorf("StripToolCallXML altered natural repetition:\n  in:  %q\n  out: %q", input, got)
	}
}