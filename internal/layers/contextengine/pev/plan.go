package pev

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/errors"
	"github.com/devrix/devrix/internal/shared/types"
)

var milestoneIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// PlanDocument is the LLM structured plan output.
type PlanDocument struct {
	TaskID     string              `json:"task_id"`
	Milestones []PlanMilestoneSpec `json:"milestones"`
}

// PlanMilestoneSpec is a single milestone in plan JSON.
type PlanMilestoneSpec struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"`
}

// PlanResult holds a validated plan outcome.
type PlanResult struct {
	TaskID     string
	Milestones []*types.Milestone
	Degraded   bool
	Err        error
}

// PlanLLMRequest is the input for plan-phase LLM completion.
type PlanLLMRequest struct {
	Model        string
	SystemPrompt string
	UserPrompt   string
}

// LLMCompleter performs non-streaming completion for planning.
type LLMCompleter interface {
	Complete(ctx context.Context, req PlanLLMRequest) (string, error)
}

// PlanEngine generates and persists milestone DAGs.
type PlanEngine struct {
	llm     LLMCompleter
	planner contracts.IMilestonePlanner
	cfg     config.PlanConfig
}

// NewPlanEngine creates a plan engine.
func NewPlanEngine(
	llm LLMCompleter,
	planner contracts.IMilestonePlanner,
	cfg config.PlanConfig,
) *PlanEngine {
	return &PlanEngine{llm: llm, planner: planner, cfg: cfg}
}

// Plan runs LLM planning, validates DAG, and persists via planner.
func (e *PlanEngine) Plan(ctx context.Context, userGoal string) (*PlanResult, error) {
	if e.llm == nil {
		return &PlanResult{Degraded: true, Err: errors.NewPlanValidationFailedError("llm not configured")}, nil
	}
	timeout := e.cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	raw, err := e.callPlanLLM(runCtx, userGoal)
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return nil, errors.NewPlanLLMTimeoutError(err)
		}
		return &PlanResult{Degraded: true, Err: err}, nil
	}

	doc, err := ParsePlanJSON(raw)
	if err != nil {
		return &PlanResult{Degraded: true, Err: errors.NewPlanValidationFailedError(err.Error())}, nil
	}

	maxMS := e.cfg.MaxMilestones
	if maxMS <= 0 {
		maxMS = 10
	}
	milestones, err := ValidatePlanDocument(doc, maxMS)
	if err != nil {
		return &PlanResult{Degraded: true, Err: errors.NewPlanValidationFailedError(err.Error())}, nil
	}

	if e.planner != nil {
		if err := e.planner.CreateBatch(doc.TaskID, milestones); err != nil {
			return &PlanResult{Degraded: true, Err: errors.NewPlanValidationFailedError(err.Error())}, nil
		}
	}

	return &PlanResult{TaskID: doc.TaskID, Milestones: milestones}, nil
}

// ParsePlanJSON extracts and unmarshals plan JSON from LLM text.
func ParsePlanJSON(text string) (*PlanDocument, error) {
	jsonText := extractJSONObject(text)
	if jsonText == "" {
		return nil, fmt.Errorf("no JSON object found")
	}
	var doc PlanDocument
	if err := json.Unmarshal([]byte(jsonText), &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if doc.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if !milestoneIDPattern.MatchString(doc.TaskID) {
		return nil, fmt.Errorf("task_id %q invalid format", doc.TaskID)
	}
	return &doc, nil
}

// ValidatePlanDocument validates milestone specs and returns types.Milestone slice.
func ValidatePlanDocument(doc *PlanDocument, maxMilestones int) ([]*types.Milestone, error) {
	if doc == nil {
		return nil, fmt.Errorf("plan document is nil")
	}
	if len(doc.Milestones) == 0 {
		return nil, fmt.Errorf("at least one milestone required")
	}
	if len(doc.Milestones) > maxMilestones {
		return nil, fmt.Errorf("milestone count %d exceeds max %d", len(doc.Milestones), maxMilestones)
	}

	ids := make(map[string]struct{}, len(doc.Milestones))
	for _, spec := range doc.Milestones {
		if spec.ID == "" {
			return nil, fmt.Errorf("milestone id is required")
		}
		if !milestoneIDPattern.MatchString(spec.ID) {
			return nil, fmt.Errorf("milestone id %q invalid format", spec.ID)
		}
		if spec.Name == "" {
			return nil, fmt.Errorf("milestone %s name is required", spec.ID)
		}
		if _, exists := ids[spec.ID]; exists {
			return nil, fmt.Errorf("duplicate milestone id %s", spec.ID)
		}
		ids[spec.ID] = struct{}{}
	}

	for _, spec := range doc.Milestones {
		for _, dep := range spec.Dependencies {
			if dep == spec.ID {
				return nil, fmt.Errorf("milestone %s cannot depend on itself", spec.ID)
			}
			if _, ok := ids[dep]; !ok {
				return nil, fmt.Errorf("milestone %s references unknown dependency %s", spec.ID, dep)
			}
		}
	}

	milestones := make([]*types.Milestone, 0, len(doc.Milestones))
	for _, spec := range doc.Milestones {
		m := types.NewMilestone(spec.ID, doc.TaskID, spec.Name)
		m.Description = spec.Description
		for _, dep := range spec.Dependencies {
			m.AddDependency(dep)
		}
		milestones = append(milestones, m)
	}

	if _, err := TopologicalSort(milestones); err != nil {
		return nil, err
	}
	return milestones, nil
}

// TopologicalSort returns milestones in dependency order (Kahn's algorithm).
func TopologicalSort(milestones []*types.Milestone) ([]*types.Milestone, error) {
	if len(milestones) == 0 {
		return nil, nil
	}
	byID := make(map[string]*types.Milestone, len(milestones))
	inDegree := make(map[string]int, len(milestones))
	dependents := make(map[string][]string, len(milestones))

	for _, m := range milestones {
		byID[m.ID] = m
		inDegree[m.ID] = len(m.Dependencies)
		for _, dep := range m.Dependencies {
			dependents[dep] = append(dependents[dep], m.ID)
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	order := make([]*types.Milestone, 0, len(milestones))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, byID[id])
		for _, child := range dependents[id] {
			inDegree[child]--
			if inDegree[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if len(order) != len(milestones) {
		return nil, fmt.Errorf("cycle detected in milestone DAG")
	}
	return order, nil
}

func (e *PlanEngine) callPlanLLM(ctx context.Context, userGoal string) (string, error) {
	model := e.cfg.Model
	if model == "" {
		model = config.DefaultPlanConfig().Model
	}
	prompt := buildPlanPrompt(userGoal, e.cfg.MaxMilestones)
	return e.llm.Complete(ctx, PlanLLMRequest{
		Model:        model,
		SystemPrompt: "You are a software task planner. Output only valid JSON.",
		UserPrompt:   prompt,
	})
}

func buildPlanPrompt(userGoal string, maxMilestones int) string {
	if maxMilestones <= 0 {
		maxMilestones = 10
	}
	return fmt.Sprintf(`Decompose the following goal into at most %d milestones.

Goal:
%s

Respond with JSON only:
{
  "task_id": "task_<short_id>",
  "milestones": [
    {
      "id": "ms_1",
      "name": "short title",
      "description": "what to do",
      "dependencies": []
    }
  ]
}

Rules:
- id and task_id must match [a-z0-9_-]+
- dependencies must reference existing milestone ids
- no cycles`, maxMilestones, userGoal)
}

func extractJSONObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return ""
	}
	return text[start : end+1]
}
