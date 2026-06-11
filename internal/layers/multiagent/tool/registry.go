package tool

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is a thread-safe registry for agent tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]AgentTool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]AgentTool),
	}
}

// Register adds a tool. Returns an error if a tool with the same name already exists.
func (r *Registry) Register(tool AgentTool) error {
	if tool == nil {
		return fmt.Errorf("agent tool is nil")
	}
	name := tool.Info().Name
	if name == "" {
		return fmt.Errorf("agent tool name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; ok {
		return fmt.Errorf("agent tool already registered: %s", name)
	}
	r.tools[name] = tool
	return nil
}

// Get returns a tool by name. Returns an error if not found.
func (r *Registry) Get(name string) (AgentTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("agent tool not found: %s", name)
	}
	return tool, nil
}

// List returns info for all registered tools, sorted by name.
func (r *Registry) List() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Info, 0, len(names))
	for _, name := range names {
		out = append(out, r.tools[name].Info())
	}
	return out
}
