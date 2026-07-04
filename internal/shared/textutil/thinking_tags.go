package textutil

import (
	"strings"
)

type thinkTagPair struct {
	open  string
	close string
}

var thinkTagPairs = []thinkTagPair{
	{open: "<think>", close: "</think>"},
}

// ThinkTagSplitter separates embedded thinking tags from visible assistant text during streaming.
type ThinkTagSplitter struct {
	buf      strings.Builder
	thinkBuf strings.Builder
	inThink  bool
	closeTag string
}

// Push processes the next streaming delta and returns extracted thinking and visible text.
// Thinking is emitted only after a closing tag is seen.
func (s *ThinkTagSplitter) Push(delta string) (thinking string, content string) {
	if delta == "" {
		return "", ""
	}
	s.buf.WriteString(delta)
	raw := s.buf.String()
	s.buf.Reset()

	var contentOut strings.Builder
	var thinkOut strings.Builder

	for len(raw) > 0 {
		if s.inThink {
			closeIdx, closeLen := findCaseInsensitive(raw, s.closeTag)
			if closeIdx >= 0 {
				s.thinkBuf.WriteString(raw[:closeIdx])
				if s.thinkBuf.Len() > 0 {
					thinkOut.WriteString(s.thinkBuf.String())
					s.thinkBuf.Reset()
				}
				raw = raw[closeIdx+closeLen:]
				s.inThink = false
				s.closeTag = ""
				continue
			}
			keep := partialTagSuffix(raw, s.closeTag)
			if keep > 0 {
				s.thinkBuf.WriteString(raw[:len(raw)-keep])
				s.buf.WriteString(raw[len(raw)-keep:])
			} else {
				s.thinkBuf.WriteString(raw)
			}
			break
		}

		openIdx, openLen, closeTag := findEarliestOpenTag(raw)
		if openIdx >= 0 {
			contentOut.WriteString(raw[:openIdx])
			raw = raw[openIdx+openLen:]
			s.inThink = true
			s.closeTag = closeTag
			continue
		}

		keep := partialOpenTagSuffix(raw)
		if keep > 0 {
			contentOut.WriteString(raw[:len(raw)-keep])
			s.buf.WriteString(raw[len(raw)-keep:])
		} else {
			contentOut.WriteString(raw)
		}
		break
	}

	return thinkOut.String(), contentOut.String()
}

// Flush returns any buffered tail at the end of a stream.
func (s *ThinkTagSplitter) Flush() (thinking string, content string) {
	raw := s.buf.String()
	s.buf.Reset()
	if raw == "" {
		if s.inThink && s.thinkBuf.Len() > 0 {
			thinking = s.thinkBuf.String()
			s.thinkBuf.Reset()
			s.inThink = false
			s.closeTag = ""
		}
		return thinking, ""
	}
	if s.inThink {
		s.thinkBuf.WriteString(raw)
		thinking = s.thinkBuf.String()
		s.thinkBuf.Reset()
		s.inThink = false
		s.closeTag = ""
		return thinking, ""
	}
	return "", raw
}

// StripThinkingTags removes complete thinking tag blocks from static text.
func StripThinkingTags(text string) string {
	out := text
	for {
		next := stripOneThinkingPass(out)
		if next == out {
			return strings.TrimSpace(out)
		}
		out = next
	}
}

func stripOneThinkingPass(text string) string {
	lower := strings.ToLower(text)
	bestIdx := -1
	bestOpenLen := 0
	bestClose := ""
	for _, pair := range thinkTagPairs {
		openIdx := strings.Index(lower, strings.ToLower(pair.open))
		if openIdx < 0 {
			continue
		}
		if bestIdx < 0 || openIdx < bestIdx {
			bestIdx = openIdx
			bestOpenLen = len(pair.open)
			bestClose = pair.close
		}
	}
	if bestIdx < 0 {
		return text
	}
	rest := text[bestIdx+bestOpenLen:]
	closeIdx, closeLen := findCaseInsensitive(rest, bestClose)
	if closeIdx < 0 {
		return strings.TrimSpace(text[:bestIdx])
	}
	prefix := strings.TrimSpace(text[:bestIdx])
	suffix := strings.TrimSpace(rest[closeIdx+closeLen:])
	switch {
	case prefix == "":
		return suffix
	case suffix == "":
		return prefix
	default:
		return prefix + suffix
	}
}

func findEarliestOpenTag(raw string) (idx int, openLen int, closeTag string) {
	lower := strings.ToLower(raw)
	bestIdx := -1
	for _, pair := range thinkTagPairs {
		openIdx := strings.Index(lower, strings.ToLower(pair.open))
		if openIdx < 0 {
			continue
		}
		if bestIdx < 0 || openIdx < bestIdx {
			bestIdx = openIdx
			openLen = len(pair.open)
			closeTag = pair.close
		}
	}
	return bestIdx, openLen, closeTag
}

func findCaseInsensitive(haystack, needle string) (idx int, length int) {
	if needle == "" {
		return -1, 0
	}
	lowerHaystack := strings.ToLower(haystack)
	lowerNeedle := strings.ToLower(needle)
	idx = strings.Index(lowerHaystack, lowerNeedle)
	if idx < 0 {
		return -1, 0
	}
	return idx, len(needle)
}

// StripPriorOutputSummary removes <prior-output-summary>…</prior-output-summary>
// blocks from assistant text. The marker is an internal D2 context-budget
// fold artifact (see internal/layers/contextengine/prepare/persist/
// turn_output_store.go FoldAssistantOutput) used to bound oversized prior
// turns in the LLM's context window. When the LLM echoes it back into its
// own reply (which it does — the marker is part of its own conversation
// history), the echo leaks to the user via the streaming card, the thinking
// card, and the standalone "任务总结" card.
//
// We strip it at the D1 IM adapter boundary so all downstream renderers see
// clean text. Unbalanced markers (LLM emitted the open tag but not the
// close) are dropped entirely — better to lose a tail fragment than to
// render half a tag to the user.
//
// This is a static-text stripper. For streaming delta splitting, the
// ThinkTagSplitter is the source of truth for <think> blocks; if a future
// change needs to split <prior-output-summary> at the stream layer, follow
// the same splitter pattern.
func StripPriorOutputSummary(text string) string {
	const openTag = "<prior-output-summary>"
	const closeTag = "</prior-output-summary>"
	out := text
	for {
		lower := strings.ToLower(out)
		openIdx := strings.Index(lower, openTag)
		if openIdx < 0 {
			return strings.TrimSpace(out)
		}
		rest := out[openIdx+len(openTag):]
		closeIdx, closeLen := findCaseInsensitive(rest, closeTag)
		var rebuilt string
		if closeIdx < 0 {
			// Unbalanced — drop the open tag and everything after it.
			rebuilt = strings.TrimSpace(out[:openIdx])
		} else {
			prefix := strings.TrimSpace(out[:openIdx])
			suffix := strings.TrimSpace(rest[closeIdx+closeLen:])
			switch {
			case prefix == "":
				rebuilt = suffix
			case suffix == "":
				rebuilt = prefix
			default:
				rebuilt = prefix + "\n" + suffix
			}
		}
		if rebuilt == out {
			// No progress — avoid infinite loop on pathological input.
			return strings.TrimSpace(out)
		}
		out = rebuilt
	}
}

// StripAssistantInternalMarkers removes both <think> blocks and
// <prior-output-summary> markers in one pass. Use this at the D1 IM
// adapter boundary so neither thinking content nor context-budget fold
// artifacts leak to the user. Order matters: thinking blocks may contain
// embedded <prior-output-summary> markers when the LLM quotes its own
// prior reasoning, so we strip thinking first.
func StripAssistantInternalMarkers(text string) string {
	return StripMiniMaxStreamMarkers(StripPriorOutputSummary(StripThinkingTags(text)))
}

func partialTagSuffix(raw, tag string) int {
	if tag == "" || raw == "" {
		return 0
	}
	max := len(tag) - 1
	if max > len(raw) {
		max = len(raw)
	}
	for size := max; size > 0; size-- {
		suffix := raw[len(raw)-size:]
		if strings.HasPrefix(strings.ToLower(tag), strings.ToLower(suffix)) {
			return size
		}
	}
	return 0
}

// PriorOutputSummarySplitter separates embedded
// <prior-output-summary>...</prior-output-summary> blocks from visible
// assistant text during streaming. Unlike <think>...</think> which is
// shown in the thinking card, this marker is an internal D2 fold
// artifact (see persist/turn_output_store.go FoldAssistantOutput) that
// must NEVER reach the user. It is also handled statically by
// StripPriorOutputSummary at the D1 IM boundary, but adding the
// streaming-time split here prevents chunk-boundary artifacts from
// interacting badly with downstream dedup logic and provides a defense
// in depth if a future change bypasses the static stripper.
//
// The splitter is stateful across calls so chunks straddling an open
// or close tag boundary still split correctly. Unbalanced markers
// (open without close) are dropped entirely — better to lose the fold
// summary than to render half a tag to the user.
type PriorOutputSummarySplitter struct {
	buf        strings.Builder
	summaryBuf strings.Builder
	inSummary  bool
}

// Push processes the next streaming delta and returns extracted fold
// summary content (to be discarded) and visible assistant text (to be
// rendered).
func (s *PriorOutputSummarySplitter) Push(delta string) (summary string, visible string) {
	if delta == "" {
		return "", ""
	}
	s.buf.WriteString(delta)
	raw := s.buf.String()
	s.buf.Reset()

	const openTag = "<prior-output-summary>"
	const closeTag = "</prior-output-summary>"

	var visibleOut strings.Builder
	var summaryOut strings.Builder

	for len(raw) > 0 {
		if s.inSummary {
			closeIdx, closeLen := findCaseInsensitive(raw, closeTag)
			if closeIdx >= 0 {
				s.summaryBuf.WriteString(raw[:closeIdx])
				if s.summaryBuf.Len() > 0 {
					summaryOut.WriteString(s.summaryBuf.String())
					s.summaryBuf.Reset()
				}
				raw = raw[closeIdx+closeLen:]
				s.inSummary = false
				continue
			}
			keep := partialTagSuffix(raw, closeTag)
			if keep > 0 {
				s.summaryBuf.WriteString(raw[:len(raw)-keep])
				s.buf.WriteString(raw[len(raw)-keep:])
			} else {
				s.summaryBuf.WriteString(raw)
			}
			break
		}

		openIdx := strings.Index(strings.ToLower(raw), strings.ToLower(openTag))
		if openIdx >= 0 {
			visibleOut.WriteString(raw[:openIdx])
			raw = raw[openIdx+len(openTag):]
			s.inSummary = true
			continue
		}

		keep := partialTagSuffix(raw, openTag)
		if keep > 0 {
			visibleOut.WriteString(raw[:len(raw)-keep])
			s.buf.WriteString(raw[len(raw)-keep:])
		} else {
			visibleOut.WriteString(raw)
		}
		break
	}

	return summaryOut.String(), visibleOut.String()
}

// Flush returns any buffered tail at the end of a stream.
func (s *PriorOutputSummarySplitter) Flush() (summary string, visible string) {
	raw := s.buf.String()
	s.buf.Reset()
	if raw == "" {
		if s.inSummary && s.summaryBuf.Len() > 0 {
			summary = s.summaryBuf.String()
			s.summaryBuf.Reset()
			s.inSummary = false
		}
		return summary, ""
	}
	if s.inSummary {
		s.summaryBuf.WriteString(raw)
		summary = s.summaryBuf.String()
		s.summaryBuf.Reset()
		s.inSummary = false
		return summary, ""
	}
	return "", raw
}

func partialOpenTagSuffix(raw string) int {
	maxKeep := 0
	for _, pair := range thinkTagPairs {
		if keep := partialTagSuffix(raw, pair.open); keep > maxKeep {
			maxKeep = keep
		}
	}
	return maxKeep
}
