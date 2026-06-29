package interfaces

import (
	"errors"
	"sort"
	"strings"
	"unicode"

	sharederrors "github.com/devrix/devrix/internal/shared/errors"
)

// SimilarityConfig holds the Jaccard threshold configuration used by
// CheckSimilarity. PR-C uses a fixed default config; v7.0.1 may introduce
// env-driven overrides (e.g. D7_SIMILARITY_WARN_THRESHOLD).
type SimilarityConfig struct {
	InterceptThreshold float64 // > this → Similar=true, abort
	WarnThreshold      float64 // 0.7-0.85 boundary → Warn=true, slog.Warn
	LookbackN          int     // how many chain entries to compare
}

// Default thresholds (PR-C fixed; v7.0.1 will allow env override).
const (
	DefaultInterceptThreshold = 0.85
	DefaultWarnThreshold      = 0.70
	DefaultLookbackN          = 5
)

// NewDefaultSimilarityConfig returns the PR-C fixed default config.
func NewDefaultSimilarityConfig() SimilarityConfig {
	return SimilarityConfig{
		InterceptThreshold: DefaultInterceptThreshold,
		WarnThreshold:      DefaultWarnThreshold,
		LookbackN:          DefaultLookbackN,
	}
}

// Validate enforces the invariants on a SimilarityConfig.
//   - 0.0 < WarnThreshold < InterceptThreshold <= 1.0
//   - LookbackN > 0
func (c SimilarityConfig) Validate() error {
	if c.WarnThreshold <= 0 || c.WarnThreshold >= 1 {
		return NewSimilarityCheckConfigInvalidError()
	}
	if c.InterceptThreshold <= c.WarnThreshold || c.InterceptThreshold > 1 {
		return NewSimilarityCheckConfigInvalidError()
	}
	if c.LookbackN <= 0 {
		return NewSimilarityCheckConfigInvalidError()
	}
	return nil
}

// SimilarityResult is the outcome of a Jaccard comparison against the
// chain's last N entries.
//
// PR-C IV-6: Similarity 边界 0.7-0.85 不阻塞 (only Warn=true).
type SimilarityResult struct {
	Similar     bool    // > InterceptThreshold (>= once, may Abort downstream)
	Warn        bool    // in (WarnThreshold, InterceptThreshold) (slog.Warn)
	Score       float64 // max Jaccard across looked-back entries
	MatchedHash Hash    // most similar snapshot hash (EmptyHash if none)
}

// Jaccard computes |A ∩ B| / |A ∪ B| for two token sets.
//
// Algorithm: O(|A| + |B|) with map[string]struct{} backing. Tokens are
// already lowercased and stripped of single-character / punctuation noise
// by Tokenize; the caller is expected to invoke Tokenize first.
//
// Edge cases:
//   - both empty → 1.0 (vacuously identical)
//   - one empty → 0.0
//   - identical → 1.0
func Jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	setA := make(map[string]struct{}, len(a))
	for _, t := range a {
		setA[t] = struct{}{}
	}
	intersection := 0
	setB := make(map[string]struct{}, len(b))
	for _, t := range b {
		setB[t] = struct{}{}
		if _, ok := setA[t]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// Tokenize splits a string into lowercase word tokens, dropping single-
// character tokens and pure-punctuation tokens. Intended to feed Jaccard.
//
// The tokenizer uses unicode.IsLetter / IsDigit to keep multilingual
// content working (e.g. Chinese characters, accented Latin). It is a
// pragmatic "good enough" split for the v7.0 use case; v7.0.1 may swap
// in a CJK-aware segmenter (e.g. Jieba) if false-positive rates on
// Chinese text exceed 5%.
func Tokenize(s string) []string {
	s = strings.ToLower(s)
	tokens := make([]string, 0, 16)
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		// Drop single-character and pure-digit tokens (too noisy).
		if len(tok) < 2 {
			return
		}
		allDigit := true
		for _, r := range tok {
			if !unicode.IsDigit(r) {
				allDigit = false
				break
			}
		}
		if allDigit {
			return
		}
		tokens = append(tokens, tok)
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// CheckSimilarity compares current string against the last N entries in
// the supplied chain, returning a SimilarityResult. The chain is allowed
// to be nil or empty — in that case returns a default "no-match" result.
//
// PR-C IV-6: 0.7-0.85 boundary sets Warn=true but does NOT set Similar=true.
// PR-C IV-2: does not mutate the chain.
//
// Errors:
//   - nil chain or empty chain → no error, returns zero-value Similar=false
//   - cfg invalid → NewSimilarityCheckConfigInvalidError()
func CheckSimilarity(current string, chain *VersionChain, cfg SimilarityConfig) (SimilarityResult, error) {
	if err := cfg.Validate(); err != nil {
		return SimilarityResult{}, err
	}
	if chain == nil || chain.Len() == 0 {
		return SimilarityResult{}, nil
	}
	currentTokens := Tokenize(current)
	if len(currentTokens) == 0 {
		return SimilarityResult{}, nil
	}
	hashes := chain.LastN(cfg.LookbackN)
	// Walk from most-recent to least-recent so the "first hit" is the
	// most recent match. For deterministic test output we sort on score
	// descending, then by hash ascending.
	type scored struct {
		h     Hash
		score float64
	}
	scoredList := make([]scored, 0, len(hashes))
	for _, h := range hashes {
		entry, ok := chain.Get(h)
		if !ok {
			// Should not happen — race with GC. Skip defensively.
			continue
		}
		entryTokens := Tokenize(string(entry.Content))
		s := Jaccard(currentTokens, entryTokens)
		scoredList = append(scoredList, scored{h: h, score: s})
	}
	if len(scoredList) == 0 {
		return SimilarityResult{}, nil
	}
	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].h < scoredList[j].h
	})
	top := scoredList[0]
	res := SimilarityResult{
		Score:       top.score,
		MatchedHash: top.h,
	}
	switch {
	case top.score > cfg.InterceptThreshold:
		res.Similar = true
	case top.score > cfg.WarnThreshold:
		// IV-6: 边界不阻塞 — only Warn=true.
		res.Warn = true
	}
	return res, nil
}

// ErrSimilarityCheckConfigInvalid is returned when SimilarityConfig.Validate fails.
var ErrSimilarityCheckConfigInvalid = errors.New("interfaces: SimilarityConfig invalid (Warn must be in (0,1), Intercept must be in (Warn,1], LookbackN must be > 0)")

// NewSimilarityCheckConfigInvalidError is the canonical wrap helper for
// ORCH_SIMILARITY_INTERCEPTED_7121 (variant: config invalid).
func NewSimilarityCheckConfigInvalidError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_SIMILARITY_INTERCEPTED_7121",
		"SimilarityCheck config invalid",
		ErrSimilarityCheckConfigInvalid,
	)
}

// NewSimilarityCheckInterceptedError is the canonical wrap helper for
// ORCH_SIMILARITY_INTERCEPTED_7121 (the actual intercept event).
func NewSimilarityCheckInterceptedError() *sharederrors.SentinelError {
	return sharederrors.WithCode(
		"ORCH_SIMILARITY_INTERCEPTED_7121",
		"similarity check intercepted: Jaccard > InterceptThreshold",
		errors.New("interfaces: similarity check intercepted (Jaccard > threshold)"),
	)
}
