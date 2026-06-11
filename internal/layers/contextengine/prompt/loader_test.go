package prompt

import (
	"testing"
)

func TestLoaderLoadAsSections(t *testing.T) {
	loader := NewLoader(nil)
	sections := loader.LoadAsSections("/tmp")

	if len(sections) != 7 {
		t.Errorf("expected 7 sections, got %d", len(sections))
	}

	if len(sections) > 0 && !contains(sections[0], "You are an interactive agent") {
		t.Error("first section should be intro")
	}
}

func TestLoaderLoadWithDynamic(t *testing.T) {
	loader := NewLoader(nil)

	dynamic := []string{"# Dynamic Section\nDynamic content here"}
	sections := loader.LoadWithDynamic("/tmp", dynamic)

	if len(sections) != 9 {
		t.Errorf("expected 9 sections, got %d", len(sections))
	}

	found := false
	for _, s := range sections {
		if s == DynamicBoundary {
			found = true
			break
		}
	}
	if !found {
		t.Error("should contain dynamic boundary")
	}
}

func TestCacheOperations(t *testing.T) {
	cache := GetCache()

	cache.Set("test", "value")
	if v, ok := cache.Get("test"); !ok || v != "value" {
		t.Error("cache Set/Get failed")
	}

	cache.Set("test", "")
	if v, ok := cache.Get("test"); !ok || v != "" {
		t.Error("cache overwrite failed")
	}
}

func TestDynamicBoundary(t *testing.T) {
	if DynamicBoundary != "<!-- DYNAMIC_CONTENT_BOUNDARY -->" {
		t.Error("dynamic boundary has unexpected value")
	}
}

func TestDefaultSectionDefinitions(t *testing.T) {
	defs := DefaultSectionDefinitions()

	expected := []string{
		"intro", "system", "doing_tasks", "actions",
		"using_tools", "output_efficiency", "tone_and_style",
	}

	if len(defs) != len(expected) {
		t.Fatalf("expected %d definitions, got %d", len(expected), len(defs))
	}

	for i, name := range expected {
		if defs[i] != name {
			t.Errorf("definition %d: expected %s, got %s", i, name, defs[i])
		}
	}
}

func TestLoaderClearCache(t *testing.T) {
	loader := NewLoader(nil)
	loader.cache.Set("intro", "mutated")
	loader.ClearCache()
	if v, _ := loader.cache.Get("intro"); v == "mutated" {
		t.Fatal("ClearCache should restore static intro content")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
