package coordinator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wave"
)

// LLMTaskDecomposer defines the interface for LLM-based task decomposition.
// Implementations should use an LLM to analyze the goal and produce task nodes.
type LLMTaskDecomposer interface {
	// Decompose uses an LLM to decompose a goal into task nodes.
	Decompose(ctx context.Context, sessionID, goal string) ([]wave.TaskNode, error)
}

// TaskDecomposer implements D7-S5-A02 SynthesizeTaskGraph.
// Default path is rule-based decomposition. LLM-based decomposition is
// available via SetLLMDecomposer (used when a planning LLM is wired).
type TaskDecomposer struct {
	llmDecomposer LLMTaskDecomposer
	timeout       time.Duration
}

// NewTaskDecomposer creates a new TaskDecomposer.
func NewTaskDecomposer() *TaskDecomposer {
	return &TaskDecomposer{
		timeout: 5 * time.Second,
	}
}

// SetLLMDecomposer wires an LLM-based decomposer for enhanced task synthesis.
func (d *TaskDecomposer) SetLLMDecomposer(llm LLMTaskDecomposer) {
	d.llmDecomposer = llm
}

// DecompositionResult contains the output of SynthesizeTaskGraph.
type DecompositionResult struct {
	Nodes      []wave.TaskNode
	Validation *ValidationReport
}

// ValidationReport describes the validity of a synthesized task graph.
type ValidationReport struct {
	Valid   bool
	Errors  []string
	Warnings []string
}

// SynthesizeTaskGraph implements D7-S5-A02. It decomposes a goal into a DAG
// of TaskNode.
//
// If an LLM decomposer is configured, it uses LLM-based decomposition.
// Otherwise, it falls back to rule-based heuristics.
func (d *TaskDecomposer) SynthesizeTaskGraph(ctx context.Context, sessionID, goal string) (*DecompositionResult, error) {
	if goal == "" {
		return nil, fmt.Errorf("SynthesizeTaskGraph: goal is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("SynthesizeTaskGraph: sessionID is required")
	}

	var nodes []wave.TaskNode

	// Try LLM decomposition first if available
	if d.llmDecomposer != nil {
		ctx, cancel := context.WithTimeout(ctx, d.timeout)
		defer cancel()

		var err error
		nodes, err = d.llmDecomposer.Decompose(ctx, sessionID, goal)
		if err != nil {
			// LLM decomposition failed, fall back to rule-based
			nodes = nil
		}
	}

	// Fall back to rule-based decomposition
	if nodes == nil {
		subGoals := d.decomposeGoal(goal)
		nodes = d.buildNodes(sessionID, subGoals)
	}

	// Validate the graph
	validation := d.validateGraph(nodes)

	return &DecompositionResult{
		Nodes:      nodes,
		Validation: validation,
	}, nil
}

// decomposeGoal splits a goal into sub-goals using simple rules.
// v1.1 uses keyword-based heuristics. Future versions will use LLM.
func (d *TaskDecomposer) decomposeGoal(goal string) []string {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil
	}

	// Simple heuristic: split by common separators
	// In production, this would use LLM-based decomposition
	var subGoals []string

	// Check for explicit step markers
	if strings.Contains(goal, "&&") {
		for _, part := range strings.Split(goal, "&&") {
			part = strings.TrimSpace(part)
			if part != "" {
				subGoals = append(subGoals, part)
			}
		}
	} else if strings.Contains(goal, "→") {
		for _, part := range strings.Split(goal, "→") {
			part = strings.TrimSpace(part)
			if part != "" {
				subGoals = append(subGoals, part)
			}
		}
	} else if strings.Contains(goal, "|") {
		for _, part := range strings.Split(goal, "|") {
			part = strings.TrimSpace(part)
			if part != "" {
				subGoals = append(subGoals, part)
			}
		}
	}

	// If no clear separation, treat as single task
	if len(subGoals) == 0 {
		subGoals = []string{goal}
	}

	return subGoals
}

// buildNodes creates TaskNode list from sub-goals.
// v1.1 assigns WorkerSubAgent (SubQuery) so the default wired scheduler can execute.
func (d *TaskDecomposer) buildNodes(sessionID string, subGoals []string) []wave.TaskNode {
	nodes := make([]wave.TaskNode, 0, len(subGoals))

	for i, sg := range subGoals {
		nodeID := fmt.Sprintf("task_%d", i+1)

		node := wave.TaskNode{
			ID:            nodeID,
			Title:         truncateTitle(sg),
			Directive:     sg,
			WorkerType:    wave.WorkerSubAgent,
			ContextPolicy: wave.ContextFresh,
			Metadata: map[string]any{
				"session_id": sessionID,
				"step":       i + 1,
			},
		}

		// First task gets the session context; subsequent tasks depend on previous
		if i > 0 {
			node.DependsOn = []string{fmt.Sprintf("task_%d", i)}
		}

		nodes = append(nodes, node)
	}

	return nodes
}

// validateGraph checks the task graph for basic validity.
func (d *TaskDecomposer) validateGraph(nodes []wave.TaskNode) *ValidationReport {
	report := &ValidationReport{Valid: true, Errors: []string{}, Warnings: []string{}}

	if len(nodes) == 0 {
		report.Valid = false
		report.Errors = append(report.Errors, "graph contains no nodes")
		return report
	}

	// Check for duplicate IDs
	ids := make(map[string]bool)
	for _, n := range nodes {
		if ids[n.ID] {
			report.Valid = false
			report.Errors = append(report.Errors, fmt.Sprintf("duplicate node ID: %s", n.ID))
		}
		ids[n.ID] = true
	}

	// Check dependency references exist
	for _, n := range nodes {
		for _, dep := range n.DependsOn {
			if !ids[dep] {
				report.Valid = false
				report.Errors = append(report.Errors, fmt.Sprintf("node %s depends on unknown ID: %s", n.ID, dep))
			}
		}
	}

	// Check for circular dependencies (simple DFS)
	if hasCycle(nodes) {
		report.Valid = false
		report.Errors = append(report.Errors, "graph contains a cycle")
	}

	// Warn about missing directives
	for _, n := range nodes {
		if n.Directive == "" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("node %s has empty directive", n.ID))
		}
	}

	return report
}

// hasCycle detects if the graph has a cycle using DFS.
func hasCycle(nodes []wave.TaskNode) bool {
	nodeMap := make(map[string]wave.TaskNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = true
		recStack[id] = true

		node, ok := nodeMap[id]
		if !ok {
			return false
		}

		for _, dep := range node.DependsOn {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	for _, n := range nodes {
		if !visited[n.ID] {
			if dfs(n.ID) {
				return true
			}
		}
	}

	return false
}

// truncateTitle truncates a title to a reasonable length.
func truncateTitle(s string) string {
	const maxLen = 50
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
