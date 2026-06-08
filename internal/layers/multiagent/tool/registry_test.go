package tool

import (
	"context"
	"sync"
	"testing"
)

// stubTool implements AgentTool for testing.
type stubTool struct {
	name         string
	displayName  string
	description  string
	capabilities []string
}

func (s *stubTool) Info() Info {
	return Info{
		Name:         s.name,
		DisplayName:  s.displayName,
		Description:  s.description,
		Capabilities: s.capabilities,
	}
}
func (s *stubTool) Execute(_ context.Context, _ string, _ Request) (<-chan Event, error) {
	ch := make(chan Event)
	close(ch)
	return ch, nil
}

func TestRegistry_RegisterAndList(t *testing.T) {
	reg := NewRegistry()
	tool := &stubTool{name: "claude-code", displayName: "Claude Code"}
	if err := reg.Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if list[0].Name != "claude-code" {
		t.Errorf("List()[0].Name = %q, want %q", list[0].Name, "claude-code")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "claude-code"})
	err := reg.Register(&stubTool{name: "claude-code"})
	if err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "claude-code"})
	reg.Register(&stubTool{name: "gemini"})

	got, err := reg.Get("claude-code")
	if err != nil {
		t.Fatalf("Get('claude-code') err = %v", err)
	}
	if got.Info().Name != "claude-code" {
		t.Errorf("got.Name = %q", got.Info().Name)
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_FindByCapability(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "coder", capabilities: []string{"coding", "review"}})
	reg.Register(&stubTool{name: "gemini", capabilities: []string{"research"}})

	tools := reg.FindByCapability("coding")
	if len(tools) != 1 {
		t.Fatalf("FindByCapability('coding') = %d, want 1", len(tools))
	}
	if tools[0].Info().Name != "coder" {
		t.Errorf("got %q, want 'coder'", tools[0].Info().Name)
	}

	tools = reg.FindByCapability("research")
	if len(tools) != 1 {
		t.Fatalf("FindByCapability('research') = %d, want 1", len(tools))
	}

	tools = reg.FindByCapability("nonexistent")
	if len(tools) != 0 {
		t.Fatalf("FindByCapability('nonexistent') = %d, want 0", len(tools))
	}
}

func TestRegistry_ConcurrentSafe(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "tool-" + string(rune('A'+i))
			_ = reg.Register(&stubTool{name: name})
			_, _ = reg.Get(name)
			reg.List()
			reg.FindByCapability("coding")
		}(i)
	}
	wg.Wait()
}

func TestRegistry_RegisterNil(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(nil); err == nil {
		t.Fatal("expected error for nil tool")
	}
}

func TestRegistry_RegisterEmptyName(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(&stubTool{name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegistry_ListSorted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubTool{name: "zebra"})
	reg.Register(&stubTool{name: "alpha"})
	reg.Register(&stubTool{name: "beta"})

	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "beta" || list[2].Name != "zebra" {
		t.Errorf("order: %v", []string{list[0].Name, list[1].Name, list[2].Name})
	}
}
