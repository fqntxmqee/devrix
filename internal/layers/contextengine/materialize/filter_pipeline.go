package materialize

import (
	"context"
	"encoding/json"

	"github.com/devrix/devrix/internal/layers/contextengine/enforce"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/filter"
	"github.com/devrix/devrix/internal/layers/contextengine/enforce/tools/surface"
	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
)

// FilterPipelineInput carries session context for the 7-step tool filter pipeline.
type FilterPipelineInput struct {
	Phase        contracts.MUPSPhase
	TaskKind     string
	ToolProfile  string
	AgentProfile string
	WorkDir      string
	SessionCtx   *types.SessionContext
}

// FilterPipelineDeps supplies registry, permission, and agent-role filters.
type FilterPipelineDeps struct {
	ToolsReg    tools.IToolRegistry
	Surfaces    []contracts.ToolSurface
	AgentFilter enforce.AgentRoleToolFilter
	Locale      i18n.Locale
}

var mupsBlockedTools = map[string]struct{}{
	"ask_user_question": {},
}

var readonlyBlockedNamePrefixes = []string{"write", "edit", "bash", "delegate"}

// RunFilterPipeline executes the 7-step MUPS tool filter (DM-20260704-001).
func RunFilterPipeline(ctx context.Context, deps FilterPipelineDeps, in FilterPipelineInput) []ToolDescriptor {
	if in.Phase == contracts.MUPSPhaseObserve || in.Phase == contracts.MUPSPhasePlan {
		return nil
	}
	if in.ToolProfile == "rollup_synth" {
		return nil
	}

	specs := listToolSpecs(ctx, deps, in.WorkDir)
	if len(specs) == 0 {
		return nil
	}

	// Step 2: permission filter (on ToolSchema, then re-map to surviving specs).
	if in.SessionCtx != nil {
		schemas := specsToToolSchemas(specs)
		filtered := enforce.FilterToolsByPermissionMode(
			in.SessionCtx.PermissionMode, schemas, in.SessionCtx.PlanFilePath,
		)
		specs = filterSpecsByNames(specs, schemaNames(filtered))
	}

	// Step 3: agent role filter.
	if deps.AgentFilter != nil && in.SessionCtx != nil {
		schemas := specsToToolSchemas(specs)
		filtered := deps.AgentFilter.Filter(in.SessionCtx, schemas)
		specs = filterSpecsByNames(specs, schemaNames(filtered))
	}

	// Step 4: per emission class.
	classes := allowedEmissionClasses(in.Phase, in.AgentProfile)
	ecFilter := filter.NewPerEmissionClassFilter(classes)
	specs = ecFilter.Apply(specs)

	// Step 5: per task kind hints.
	tkFilter := filter.NewPerTaskKindFilter(in.TaskKind)
	specs = tkFilter.Apply(specs)

	// Step 6: tool profile filter.
	specs = profileFilter(in.ToolProfile, specs)

	// Step 7: localize to descriptors.
	return specsToDescriptors(deps.Locale, specs)
}

func allowedEmissionClasses(phase contracts.MUPSPhase, agentProfile string) []contracts.EmissionClass {
	switch phase {
	case contracts.MUPSPhaseObserve, contracts.MUPSPhasePlan:
		return nil
	case contracts.MUPSPhaseExecute:
		return filter.AllowedEmissionClassesForAgent(agentProfile)
	default:
		return nil
	}
}

func profileFilter(profile string, specs []contracts.ToolSpec) []contracts.ToolSpec {
	switch profile {
	case "rollup_synth":
		return nil
	case "readonly":
		out := make([]contracts.ToolSpec, 0, len(specs))
		for _, s := range specs {
			if s.EmissionClass != contracts.EC_Fact && s.EmissionClass != contracts.EC_Probe {
				continue
			}
			if isReadonlyBlockedName(s.Name) {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return removeBlockedSpecs(specs)
	}
}

func removeBlockedSpecs(specs []contracts.ToolSpec) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if _, blocked := mupsBlockedTools[s.Name]; blocked {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isReadonlyBlockedName(name string) bool {
	for _, prefix := range readonlyBlockedNamePrefixes {
		if name == prefix || hasPrefix(name, prefix+"_") {
			return true
		}
	}
	if name == "bash" || name == "edit_file" || name == "write_file" {
		return true
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func listToolSpecs(ctx context.Context, deps FilterPipelineDeps, workDir string) []contracts.ToolSpec {
	seen := map[string]bool{}
	var specs []contracts.ToolSpec
	for _, s := range deps.Surfaces {
		if s == nil {
			continue
		}
		for _, sp := range s.Tools(ctx, workDir, "") {
			if seen[sp.Name] {
				continue
			}
			seen[sp.Name] = true
			specs = append(specs, sp)
		}
	}
	if deps.ToolsReg != nil {
		schemas, err := deps.ToolsReg.ListTools(ctx, workDir)
		if err == nil {
			for _, ts := range schemas {
				if seen[ts.Name] {
					continue
				}
				seen[ts.Name] = true
				spec := contracts.ToolSpec{
					Name:        ts.Name,
					Description: ts.Description,
					Parameters:  ts.Parameters,
				}
				surface.ApplyV3Metadata(&spec, ts.Name)
				specs = append(specs, spec)
			}
		}
	}
	return specs
}

func specsToToolSchemas(specs []contracts.ToolSpec) []tools.ToolSchema {
	out := make([]tools.ToolSchema, len(specs))
	for i, s := range specs {
		out[i] = tools.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			Parameters:  s.Parameters,
		}
	}
	return out
}

func schemaNames(schemas []tools.ToolSchema) map[string]struct{} {
	m := make(map[string]struct{}, len(schemas))
	for _, s := range schemas {
		m[s.Name] = struct{}{}
	}
	return m
}

func filterSpecsByNames(specs []contracts.ToolSpec, allowed map[string]struct{}) []contracts.ToolSpec {
	out := make([]contracts.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if _, ok := allowed[s.Name]; ok {
			out = append(out, s)
		}
	}
	return out
}

func specsToDescriptors(loc i18n.Locale, specs []contracts.ToolSpec) []ToolDescriptor {
	out := make([]ToolDescriptor, 0, len(specs))
	for _, s := range specs {
		desc, paramsRaw := i18n.LocalizeTool(s.Name, s.Description, s.Parameters, loc)
		params := map[string]any{}
		if paramsRaw != "" {
			_ = json.Unmarshal([]byte(paramsRaw), &params)
		}
		out = append(out, ToolDescriptor{
			Name:        s.Name,
			Description: desc,
			Schema:      params,
		})
	}
	return out
}

// PipelineStep records one filter step for order-invariant tests.
type PipelineStep struct {
	Name  string
	Count int
}

// TraceFilterPipelineSteps returns tool counts after each step (testing only).
func TraceFilterPipelineSteps(ctx context.Context, deps FilterPipelineDeps, in FilterPipelineInput) []PipelineStep {
	var trace []PipelineStep
	if in.Phase == contracts.MUPSPhaseObserve || in.Phase == contracts.MUPSPhasePlan || in.ToolProfile == "rollup_synth" {
		return trace
	}
	specs := listToolSpecs(ctx, deps, in.WorkDir)
	trace = append(trace, PipelineStep{Name: "list", Count: len(specs)})

	if in.SessionCtx != nil {
		schemas := specsToToolSchemas(specs)
		filtered := enforce.FilterToolsByPermissionMode(
			in.SessionCtx.PermissionMode, schemas, in.SessionCtx.PlanFilePath,
		)
		specs = filterSpecsByNames(specs, schemaNames(filtered))
	}
	trace = append(trace, PipelineStep{Name: "permission", Count: len(specs)})

	if deps.AgentFilter != nil && in.SessionCtx != nil {
		schemas := specsToToolSchemas(specs)
		filtered := deps.AgentFilter.Filter(in.SessionCtx, schemas)
		specs = filterSpecsByNames(specs, schemaNames(filtered))
	}
	trace = append(trace, PipelineStep{Name: "agent_role", Count: len(specs)})

	classes := allowedEmissionClasses(in.Phase, in.AgentProfile)
	specs = filter.NewPerEmissionClassFilter(classes).Apply(specs)
	trace = append(trace, PipelineStep{Name: "emission_class", Count: len(specs)})

	specs = filter.NewPerTaskKindFilter(in.TaskKind).Apply(specs)
	trace = append(trace, PipelineStep{Name: "task_kind", Count: len(specs)})

	specs = profileFilter(in.ToolProfile, specs)
	trace = append(trace, PipelineStep{Name: "profile", Count: len(specs)})

	trace = append(trace, PipelineStep{Name: "descriptors", Count: len(specsToDescriptors(deps.Locale, specs))})
	return trace
}
