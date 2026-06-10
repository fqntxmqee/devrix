package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/harness"
	"github.com/devrix/devrix/internal/shared/types"
)

type toolRegistryAdapter struct {
	reg IToolRegistry
}

func (a toolRegistryAdapter) ListTools(ctx context.Context, workDir string) ([]harness.ToolDesc, error) {
	tools, err := a.reg.ListTools(ctx, workDir)
	if err != nil {
		return nil, err
	}
	out := make([]harness.ToolDesc, 0, len(tools))
	for _, t := range tools {
		out = append(out, harness.ToolDesc{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out, nil
}

func toolDescsToSchemas(tools []harness.ToolDesc) []ToolSchema {
	out := make([]ToolSchema, 0, len(tools))
	for _, t := range tools {
		out = append(out, ToolSchema{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}

func visibleToolsToSchemas(state *types.HarnessSessionState) []ToolSchema {
	return toolDescsToSchemas(harness.VisibleToolsFromState(state))
}

func toolDescsToVisibleTools(tools []harness.ToolDesc) []types.VisibleTool {
	out := make([]types.VisibleTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, types.VisibleTool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}
