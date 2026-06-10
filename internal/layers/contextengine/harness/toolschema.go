package harness

import "context"

// ToolDesc describes a tool for harness filtering and routing.
type ToolDesc struct {
	Name        string
	Description string
	Parameters  string
}

// ToolLister lists available tools during bootstrap.
type ToolLister interface {
	ListTools(ctx context.Context, workDir string) ([]ToolDesc, error)
}

// ToToolDescs converts visible tool records to tool descriptions.
func ToToolDescs(tools []VisibleToolRecord) []ToolDesc {
	out := make([]ToolDesc, 0, len(tools))
	for _, t := range tools {
		out = append(out, ToolDesc{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}

// VisibleToolRecord is a serializable tool record stored on harness session state.
type VisibleToolRecord struct {
	Name        string
	Description string
	Parameters  string
}
