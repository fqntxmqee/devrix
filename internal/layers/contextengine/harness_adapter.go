// Package contextengine — harness adapter (facade bridge to #deprecated harness package).

package contextengine

import (
	"context"

	"github.com/devrix/devrix/internal/layers/contextengine/fallback"
	"github.com/devrix/devrix/internal/shared/types"
)

type toolRegistryAdapter struct {
	reg IToolRegistry
}

func (a toolRegistryAdapter) ListTools(ctx context.Context, workDir string) ([]fallback.ToolDesc, error) {
	tools, err := a.reg.ListTools(ctx, workDir)
	if err != nil {
		return nil, err
	}
	out := make([]fallback.ToolDesc, 0, len(tools))
	for _, t := range tools {
		out = append(out, fallback.ToolDesc{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out, nil
}

func toolDescsToSchemas(tools []fallback.ToolDesc) []ToolSchema {
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
	return toolDescsToSchemas(fallback.VisibleToolsFromState(state))
}

func toolDescsToVisibleTools(tools []fallback.ToolDesc) []types.VisibleTool {
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
