package fallback

import (
	"strings"
	"unicode"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PromptRouter scores tools against the user prompt for advisory routing hints.
type PromptRouter struct {
	cfg config.RoutingConfig
}

// NewPromptRouter creates a prompt router from config.
func NewPromptRouter(cfg config.RoutingConfig) *PromptRouter {
	return &PromptRouter{cfg: cfg}
}

// Route returns advisory routing hints based on keyword overlap scoring.
func (r *PromptRouter) Route(prompt string, tools []ToolDesc, limit int) types.RoutingHint {
	if r == nil || !r.cfg.Enabled {
		return types.RoutingHint{}
	}
	if limit <= 0 {
		limit = r.cfg.MaxMatches
	}
	if limit <= 0 {
		limit = 5
	}
	tokens := tokenize(prompt)
	if len(tokens) == 0 {
		return types.RoutingHint{}
	}

	scores := make(map[string]int)
	for _, tool := range tools {
		score := scoreTokens(tokens, tool.Name+" "+tool.Description)
		if score > 0 {
			scores[tool.Name] = score
		}
	}

	toolsOut := topScored(scores, limit)
	return types.RoutingHint{
		Tools:  toolsOut,
		Scores: scores,
	}
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) >= 2 {
			out = append(out, f)
		}
	}
	return out
}

func scoreTokens(tokens []string, haystack string) int {
	lower := strings.ToLower(haystack)
	score := 0
	for _, tok := range tokens {
		if strings.Contains(lower, tok) {
			score++
		}
	}
	return score
}

func topScored(scores map[string]int, limit int) []string {
	type pair struct {
		name  string
		score int
	}
	pairs := make([]pair, 0, len(scores))
	for name, score := range scores {
		pairs = append(pairs, pair{name: name, score: score})
	}
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].score > pairs[i].score {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.name)
	}
	return out
}
