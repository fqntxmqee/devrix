package decisionplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/devrix/devrix/internal/layers/llmgateway"
	"github.com/devrix/devrix/internal/layers/orchestration/turn"
	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/shared/types"
)

// LLMDecomposerDeps wires the LLM-based task decomposer (D7-S5-A03 — LLM
// augmentation of SynthesizeTaskGraph). It uses the D7 turn.LLMInvoker
// interface so coordinator does not import the D3 gateway directly to
// invoke LLMs; the gateway.Chunk struct is imported only to consume the
// streaming contract value returned by InvokeStream.
type LLMDecomposerDeps struct {
	// LLM is the D7-S2-A07 streaming LLM entry point. Required.
	LLM turn.LLMInvoker
	// DefaultTier is used when Decompose is invoked without an explicit
	// tier in the goal context. Optional; defaults to "default".
	DefaultTier string
	// SystemPromptExtra is appended to the decomposition system prompt
	// to inject project-specific guidance. Optional.
	SystemPromptExtra string
}

// LLMDecomposer implements LLMTaskDecomposer by asking an LLM to
// decompose a goal into a JSON array of task nodes, then validating
// and mapping the result into []wavescheduler.TaskNode.
//
// v1.1: this is the LLM-augmented path; TaskDecomposer.SynthesizeTaskGraph
// falls back to the rule-based decomposeGoal when this returns an error
// or empty result.
type LLMDecomposer struct {
	llm              turn.LLMInvoker
	defaultTier      string
	systemPromptBase string
}

// NewLLMDecomposer constructs an LLMDecomposer bound to deps.LLM.
// Returns nil if deps.LLM is nil so callers can defensively skip wiring.
func NewLLMDecomposer(deps LLMDecomposerDeps) *LLMDecomposer {
	if deps.LLM == nil {
		return nil
	}
	tier := deps.DefaultTier
	if tier == "" {
		tier = "default"
	}
	base := decompositionSystemPrompt
	if deps.SystemPromptExtra != "" {
		base = base + "\n\nAdditional guidance:\n" + deps.SystemPromptExtra
	}
	return &LLMDecomposer{
		llm:              deps.LLM,
		defaultTier:      tier,
		systemPromptBase: base,
	}
}

// Decompose satisfies LLMTaskDecomposer.
func (d *LLMDecomposer) Decompose(ctx context.Context, sessionID, goal string) ([]wavescheduler.TaskNode, error) {
	if d == nil || d.llm == nil {
		return nil, fmt.Errorf("LLMDecomposer: not initialized")
	}
	if goal == "" {
		return nil, fmt.Errorf("LLMDecomposer: goal is required")
	}

	messages := []types.Message{
		{
			ID:      "decomp_user",
			Role:    types.MessageRoleUser,
			Content: buildDecomposeUserPrompt(goal),
		},
	}

	stream, err := d.llm.InvokeStream(ctx, turn.LLMInvokeRequest{
		SessionID:    sessionID,
		Tier:         d.defaultTier,
		SystemPrompt: d.systemPromptBase,
		Messages:     messages,
	})
	if err != nil {
		return nil, fmt.Errorf("LLMDecomposer: invoke: %w", err)
	}

	raw := collectChunkContent(ctx, stream)
	raw = extractJSON(raw)
	if raw == "" {
		return nil, fmt.Errorf("LLMDecomposer: no JSON payload in LLM response")
	}

	nodes, err := parseDecomposedTasks(raw, sessionID)
	if err != nil {
		return nil, fmt.Errorf("LLMDecomposer: parse: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("LLMDecomposer: empty task list")
	}
	return nodes, nil
}

// decompositionSystemPrompt is the system prompt fed to the LLM. It pins
// the JSON schema and the worker_type/context_policy vocabulary so the
// output can be parsed deterministically.
const decompositionSystemPrompt = `You are a task planner for a multi-agent orchestrator.

Decompose the user's goal into a DAG of tasks. Output ONLY a JSON array — no prose, no markdown fence.

Each task object MUST have exactly these fields:
- id (string, lowercase snake_case, e.g. "design_api")
- title (string, ≤ 60 chars)
- directive (string, what the worker should do)
- worker_type (one of: "cursor", "claude_code", "subagent")
- context_policy (one of: "fresh", "resume", "upstream")
- depends_on (array of task ids; [] if none)

Defaults if unspecified:
- worker_type: "cursor"
- context_policy: "fresh"
- depends_on: []

Rules:
1. Order tasks so dependencies form a DAG (no cycles).
2. First task should have empty depends_on unless the goal explicitly references prior work.
3. Prefer 2–6 tasks; never produce more than 12.
4. Keep directives specific and actionable.`

func buildDecomposeUserPrompt(goal string) string {
	return "Goal: " + strings.TrimSpace(goal) + "\n\nReturn the JSON array now."
}

// collectChunkContent reads a D3 chunk stream into a single string.
// It respects ctx cancellation and stops at the first Done chunk.
func collectChunkContent(ctx context.Context, stream <-chan llmgateway.Chunk) string {
	var sb strings.Builder
	for {
		select {
		case <-ctx.Done():
			return sb.String()
		case chunk, ok := <-stream:
			if !ok {
				return sb.String()
			}
			sb.WriteString(chunk.Content)
			if chunk.Done {
				return sb.String()
			}
		}
	}
}

// extractJSON pulls the first balanced JSON array out of a possibly
// verbose LLM response. It is tolerant of code fences, leading prose,
// and trailing commentary.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Strip code fences: ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		if end := strings.LastIndex(s, "```"); end > 3 {
			inner := s[3:end]
			inner = strings.TrimPrefix(inner, "json")
			inner = strings.TrimPrefix(inner, "JSON")
			s = strings.TrimSpace(inner)
		}
	}
	start := strings.Index(s, "[")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(s, "]")
	if end <= start {
		return ""
	}
	return s[start : end+1]
}

// decomposedTaskDTO is the wire format we expect from the LLM. Fields
// are intentionally permissive (optional slices, defaults filled by
// the parser) so a well-meaning LLM that adds or omits optional fields
// doesn't fail parsing outright.
type decomposedTaskDTO struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Directive     string   `json:"directive"`
	WorkerType    string   `json:"worker_type"`
	ContextPolicy string   `json:"context_policy"`
	DependsOn     []string `json:"depends_on"`
}

// parseDecomposedTasks decodes raw JSON into validated wavescheduler.TaskNodes.
// It enforces: id uniqueness, non-empty directive, valid worker_type,
// valid context_policy, and drops DependsOn edges that reference
// unknown ids so downstream validators don't reject an otherwise-sound
// graph.
func parseDecomposedTasks(raw, sessionID string) ([]wavescheduler.TaskNode, error) {
	var dtos []decomposedTaskDTO
	if err := json.Unmarshal([]byte(raw), &dtos); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	if len(dtos) == 0 {
		return nil, fmt.Errorf("empty task list")
	}
	if len(dtos) > 12 {
		return nil, fmt.Errorf("too many tasks: %d (max 12)", len(dtos))
	}

	seenIDs := make(map[string]bool, len(dtos))
	nodes := make([]wavescheduler.TaskNode, 0, len(dtos))
	for i, dto := range dtos {
		id := strings.TrimSpace(dto.ID)
		if id == "" {
			id = fmt.Sprintf("task_%d", i+1)
		}
		if seenIDs[id] {
			// Disambiguate duplicate ids so the validator never sees them.
			id = fmt.Sprintf("%s_%d", id, i+1)
		}
		seenIDs[id] = true

		workerType := wavescheduler.WorkerType(strings.ToLower(strings.TrimSpace(dto.WorkerType)))
		if !workerType.Valid() {
			workerType = wavescheduler.WorkerCursor
		}

		contextPolicy := wavescheduler.ContextPolicy(strings.ToLower(strings.TrimSpace(dto.ContextPolicy)))
		if !contextPolicy.Valid() {
			contextPolicy = wavescheduler.ContextFresh
		}

		title := strings.TrimSpace(dto.Title)
		if title == "" {
			title = truncateTitle(dto.Directive)
		}

		directive := strings.TrimSpace(dto.Directive)
		if directive == "" {
			directive = id
		}

		node := wavescheduler.TaskNode{
			ID:            id,
			Title:         title,
			Directive:     directive,
			WorkerType:    workerType,
			ContextPolicy: contextPolicy,
			DependsOn:     dto.DependsOn,
			Metadata: map[string]any{
				"session_id": sessionID,
				"source":     "llm_decomposer",
				"raw_index":  i,
			},
		}
		nodes = append(nodes, node)
	}

	// Drop DependsOn edges that reference unknown ids.
	for i := range nodes {
		kept := nodes[i].DependsOn[:0]
		for _, dep := range nodes[i].DependsOn {
			if seenIDs[dep] {
				kept = append(kept, dep)
			}
		}
		nodes[i].DependsOn = kept
	}

	slog.Info("LLMDecomposer: parsed tasks",
		"session_id", sessionID, "count", len(nodes))
	return nodes, nil
}
