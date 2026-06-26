package textutil

import (
	"regexp"
	"strings"
)

// StripToolCallXML removes leaked tool-call XML markup from assistant
// text. minimax M2.7 streaming emits tool calls as inline XML inside
// the text channel rather than as structured tool_use events; without
// this strip the user sees `<function_calls>`, `<invoke name="bash">`,
// and `<parameter name="command">…</parameter>` blocks rendered on the
// reply card. The closing tag is sometimes split across chunks
// (e.g. `</markdown` in one chunk, `>` in the next), so we also strip
// incomplete tag fragments.
//
// Strips in priority order:
//  1. Complete `<function_calls>…</function_calls>` blocks (greedy, multiline).
//  2. Complete `<invoke …>…</invoke>` blocks.
//  3. Complete `<parameter …>…</parameter>` blocks.
//  4. Any remaining `<tag>` or `</tag>` constructs (self-closing or
//     open — also catches malformed markup where the `>` arrived in a
//     later chunk, so the tag is no longer complete in the buffer).
//
// Apply at the D1 IM adapter boundary after chunk accumulation, never
// on individual chunks (chunks can split a tag). The function is a
// pure string transform with no allocation beyond the output buffer.
var (
	toolCallBlockRe = regexp.MustCompile(`(?is)<function_calls[^>]*>.*?</function_calls>`)
	invokeBlockRe   = regexp.MustCompile(`(?is)<invoke\b[^>]*>.*?</invoke>`)
	paramBlockRe    = regexp.MustCompile(`(?is)<parameter\b[^>]*>.*?</parameter>`)
	tagRe           = regexp.MustCompile(`<[/a-zA-Z][^>]*>?`)
)

// StripToolCallXML returns text with leaked tool-call XML removed.
func StripToolCallXML(text string) string {
	out := text
	out = toolCallBlockRe.ReplaceAllString(out, "")
	out = invokeBlockRe.ReplaceAllString(out, "")
	out = paramBlockRe.ReplaceAllString(out, "")
	out = tagRe.ReplaceAllString(out, "")
	return strings.TrimSpace(out)
}