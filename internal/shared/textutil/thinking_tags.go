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

func partialOpenTagSuffix(raw string) int {
	maxKeep := 0
	for _, pair := range thinkTagPairs {
		if keep := partialTagSuffix(raw, pair.open); keep > maxKeep {
			maxKeep = keep
		}
	}
	return maxKeep
}
