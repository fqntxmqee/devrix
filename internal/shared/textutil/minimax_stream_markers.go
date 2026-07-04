package textutil

import "regexp"

// minimaxStreamMarkerRE matches MiniMax M2.7 streaming delimiter tokens that
// leak into delta.Content when the model multiplexes thinking/tool-call markup
// on the text channel. Observed in sess_1783133995187_7000 as repeated
// "]<[minimax[>[" fragments between tool-call XML shards.
var minimaxStreamMarkerRE = regexp.MustCompile(`\]\<\[minimax\[\>\[`)

// StripMiniMaxStreamMarkers removes MiniMax streaming delimiter tokens from
// assistant-visible text. Stateless — safe on individual streaming chunks.
func StripMiniMaxStreamMarkers(text string) string {
	if text == "" {
		return ""
	}
	return minimaxStreamMarkerRE.ReplaceAllString(text, "")
}
