package workmodel

import (
	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

// CheckSimilarityForSession is the workmodel-side convenience wrapper around
// interfaces.CheckSimilarity. It pulls the chain for sessionID from the
// provided registry and runs the check with the supplied config.
//
// PR-B/C design: provides a single entry point so decisionplanning/decomposer
// doesn't need to know about registry internals. Returns the same result
// shape as interfaces.CheckSimilarity. If registry is nil or the chain is
// empty, the result is the zero-value (no match).
func CheckSimilarityForSession(reg *VersionChainRegistry, sessionID, current string, cfg interfaces.SimilarityConfig) (interfaces.SimilarityResult, error) {
	if reg == nil {
		return interfaces.SimilarityResult{}, nil
	}
	chain := reg.ChainFor(sessionID)
	return interfaces.CheckSimilarity(current, chain, cfg)
}

// MostSimilarSessionID finds the session ID whose most recent chain entry has
// the highest Jaccard overlap with current, scanning up to lookbackSessions.
// Returns (sessionID, score, found). If no sessions have chains, returns
// ("", 0.0, false).
//
// This is a best-effort discovery helper used by decisionplanning when the
// caller does not already have a target session in mind. It does NOT mutate
// any chain (PR-C IV-2 immutability invariant).
func MostSimilarSessionID(reg *VersionChainRegistry, current string, cfg interfaces.SimilarityConfig, lookbackSessions int) (string, float64, bool) {
	if reg == nil || reg.SessionCount() == 0 {
		return "", 0.0, false
	}
	if err := cfg.Validate(); err != nil {
		return "", 0.0, false
	}
	if lookbackSessions <= 0 {
		lookbackSessions = cfg.LookbackN
	}
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	var bestSession string
	bestScore := -1.0
	currentTokens := interfaces.Tokenize(current)
	if len(currentTokens) == 0 {
		return "", 0.0, false
	}
	count := 0
	for sid, chain := range reg.chains {
		if count >= lookbackSessions {
			break
		}
		count++
		last := chain.LastN(cfg.LookbackN)
		if len(last) == 0 {
			continue
		}
		for _, h := range last {
			entry, ok := chain.Get(h)
			if !ok {
				continue
			}
			entryTokens := interfaces.Tokenize(string(entry.Content))
			s := interfaces.Jaccard(currentTokens, entryTokens)
			if s > bestScore {
				bestScore = s
				bestSession = sid
			}
		}
	}
	if bestSession == "" || bestScore <= 0 {
		return "", 0.0, false
	}
	return bestSession, bestScore, true
}
