package surface

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// ToolSearchSurfaceName is the surface identifier and the name of the
// single tool this surface exposes (`tool_search`).
const ToolSearchSurfaceName = "tool_search"

// ToolSearchResult is one entry returned by tool_search. We marshal the
// matching ToolSpec as JSON so the LLM can read name + description + parameters.
type ToolSearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  string `json:"parameters,omitempty"`
	Risk        string `json:"risk,omitempty"`
	Category    string `json:"category,omitempty"`
}

// ToolSearchSurface provides the `tool_search` tool that lets the LLM
// discover deferred tool schemas on demand. It is intentionally itself
// NOT defer-loaded (the TurnAdapter Prepare path must always include it,
// otherwise the LLM would have no way to escape the deferred set).
//
// Algorithm (T28):
//  1. exact name match on the query, if any
//  2. glob match (filepath.Match style: `delegate_*`)
//  3. substring match on name (case-insensitive)
//  4. optional category prefix filter (e.g. "delegate")
//     Top-5 results; only DeferLoading=true specs are searched (non-deferred
//     tools are already in the prompt).
//
// DSAFT: TOOL-SURFACE-1-A02 (DM-20260618-003 devrix-surface-lazy-loading).
type ToolSearchSurface struct {
	allSpecs []contracts.ToolSpec
	locale   i18n.Locale
}

// NewToolSearchSurface builds the surface from the full tool catalog.
// The catalog is filtered at search time so callers don't have to
// rebuild when surfaces mutate.
func NewToolSearchSurface(specs []contracts.ToolSpec, locale i18n.Locale) *ToolSearchSurface {
	if locale == "" {
		locale = i18n.DefaultLocale
	}
	cp := make([]contracts.ToolSpec, len(specs))
	copy(cp, specs)
	return &ToolSearchSurface{allSpecs: cp, locale: locale}
}

// Name implements contracts.ToolSurface.
func (s *ToolSearchSurface) Name() string { return ToolSearchSurfaceName }

// Tools implements contracts.ToolSurface. Always returns a single spec
// for the `tool_search` tool. DeferLoading is forced false so callers
// can't accidentally filter it out.
func (s *ToolSearchSurface) Tools(_ context.Context, _, _ string) []contracts.ToolSpec {
		spec := contracts.ToolSpec{

			Name:        ToolSearchSurfaceName,
			Description: "Search the deferred tool catalog. Pass a query (substring or glob like 'delegate_*') and an optional category prefix. Returns up to 5 matching tool schemas that you can then call directly.",
			Parameters: `{
				"type": "object",
				"properties": {
					"query":    {"type": "string", "description": "Substring or glob to match tool names against"},
					"category": {"type": "string", "description": "Optional name prefix to filter (e.g. 'delegate' or 'task')"}
				},
				"required": ["query"]
			}`,
			Risk:            types.RiskLevelLow,
			ReadOnly:        true,
			OpenWorld:       false,
			ConcurrencySafe: true,
			DeferLoading:    false, // forced: this tool must always be in-pack
		
}
	ApplyV3Metadata(&spec, "tool_search")
	return []contracts.ToolSpec{spec}
}

// RiskLevel implements contracts.ToolSurface.
func (s *ToolSearchSurface) RiskLevel(name string) types.RiskLevel {
	if name == ToolSearchSurfaceName {
		return types.RiskLevelLow
	}
	return ""
}

// Execute implements contracts.ToolSurface. The `name` arg MUST be
// ToolSearchSurfaceName (the only tool the surface exposes); other names
// return an error envelope.
func (s *ToolSearchSurface) Execute(_ context.Context, name, input, _ string) (*contracts.ToolResult, error) {
	if name != ToolSearchSurfaceName {
		return &contracts.ToolResult{
			Error: fmt.Sprintf("tool_search surface: unknown tool %q", name),
		}, nil
	}
	var req struct {
		Query    string `json:"query"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return &contracts.ToolResult{
			Error: "tool_search: invalid input JSON: " + err.Error(),
		}, nil
	}
	matches := s.search(req.Query, req.Category)
	out, err := json.Marshal(matches)
	if err != nil {
		return &contracts.ToolResult{Error: "tool_search: marshal: " + err.Error()}, nil
	}
	return &contracts.ToolResult{Output: string(out)}, nil
}

// InterruptBehavior implements contracts.ToolSurface. tool_search is
// short-run, so it blocks on ctx cancellation like every other read-only
// tool.
func (s *ToolSearchSurface) InterruptBehavior(_ string) contracts.InterruptMode {
	return contracts.InterruptBlock
}

// CheckPermission implements contracts.ToolSurface. tool_search is a
// pure read-only discovery tool; default Allow.
func (s *ToolSearchSurface) CheckPermission(_ context.Context, _ contracts.ToolSpec, _ json.RawMessage) contracts.Decision {
	return contracts.DecisionAllow
}

// IsConcurrencySafe implements contracts.ToolSurface v4. tool_search is
// a pure read-only discovery — concurrency safe (DSAFT: D2-S15-A02-T17).
func (s *ToolSearchSurface) IsConcurrencySafe(_ json.RawMessage) bool {
	return true
}

// ToAutoClassifierInput implements contracts.ToolSurface v4. P2 stub
// default — returns "" to skip in classifier transcript.
func (s *ToolSearchSurface) ToAutoClassifierInput(input json.RawMessage) string {
	return DefaultToAutoClassifierInputFor(ToolSearchSurfaceName, input)
}

// search returns up to 5 deferred tools matching (query, category).
// Algorithm order: exact > glob > substring (case-insensitive).
func (s *ToolSearchSurface) search(query, category string) []ToolSearchResult {
	const maxResults = 5
	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.TrimSpace(category)
	seen := map[string]bool{}
	var out []ToolSearchResult

	// Pass 1: exact name match.
	if q != "" {
		for _, sp := range s.allSpecs {
			if !sp.DeferLoading || sp.Name == ToolSearchSurfaceName || seen[sp.Name] {
				continue
			}
			if cat != "" && !strings.HasPrefix(sp.Name, cat) {
				continue
			}
			if strings.ToLower(sp.Name) == q {
				out = append(out, toResult(sp, s.locale))
				seen[sp.Name] = true
				if len(out) >= maxResults {
					return out
				}
			}
		}
	}

	// Pass 2: glob match.
	if q != "" && len(out) < maxResults {
		for _, sp := range s.allSpecs {
			if !sp.DeferLoading || sp.Name == ToolSearchSurfaceName || seen[sp.Name] {
				continue
			}
			if cat != "" && !strings.HasPrefix(sp.Name, cat) {
				continue
			}
			if !strings.Contains(q, "*") && !strings.Contains(q, "?") {
				continue
			}
			if ok, _ := filepath.Match(q, strings.ToLower(sp.Name)); ok {
				out = append(out, toResult(sp, s.locale))
				seen[sp.Name] = true
				if len(out) >= maxResults {
					return out
				}
			}
		}
	}

	// Pass 3: substring match.
	if q != "" && len(out) < maxResults {
		for _, sp := range s.allSpecs {
			if !sp.DeferLoading || sp.Name == ToolSearchSurfaceName || seen[sp.Name] {
				continue
			}
			if cat != "" && !strings.HasPrefix(sp.Name, cat) {
				continue
			}
			if strings.Contains(strings.ToLower(sp.Name), q) {
				out = append(out, toResult(sp, s.locale))
				seen[sp.Name] = true
				if len(out) >= maxResults {
					return out
				}
			}
		}
	}

	// Pass 4: category-only listing (when query empty).
	if q == "" && cat != "" {
		for _, sp := range s.allSpecs {
			if !sp.DeferLoading || sp.Name == ToolSearchSurfaceName || seen[sp.Name] {
				continue
			}
			if strings.HasPrefix(sp.Name, cat) {
				out = append(out, toResult(sp, s.locale))
				seen[sp.Name] = true
				if len(out) >= maxResults {
					return out
				}
			}
		}
	}

	return out
}

func toResult(sp contracts.ToolSpec, loc i18n.Locale) ToolSearchResult {
	cat := ""
	if i := strings.Index(sp.Name, "_"); i > 0 {
		cat = sp.Name[:i]
	}
	desc, params := i18n.LocalizeTool(sp.Name, sp.Description, sp.Parameters, loc)
	return ToolSearchResult{
		Name:        sp.Name,
		Description: desc,
		Parameters:  params,
		Risk:        string(sp.Risk),
		Category:    cat,
	}
}
