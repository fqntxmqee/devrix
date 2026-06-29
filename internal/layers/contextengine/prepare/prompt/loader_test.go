package prompt

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
)

func TestLoaderLoadAsSectionsEnglish(t *testing.T) {
	loader := NewLoader(nil, i18n.LocaleEN)
	sections := loader.LoadAsSections("/tmp")

	if len(sections) != 11 {
		t.Errorf("expected 11 sections, got %d", len(sections))
	}

	if len(sections) > 0 && !contains(sections[0], "You are an interactive agent") {
		t.Error("first section should be intro")
	}
}

func TestLoaderLoadAsSectionsChineseDefault(t *testing.T) {
	loader := NewLoader(nil, i18n.DefaultLocale)
	sections := loader.LoadAsSections("/tmp")
	if len(sections) != 11 {
		t.Fatalf("expected 11 sections, got %d", len(sections))
	}
	if !contains(sections[0], "软件工程") {
		t.Fatalf("expected Chinese intro, got: %.80q", sections[0])
	}
}

func TestLoaderLoadWithDynamic(t *testing.T) {
	loader := NewLoader(nil, i18n.LocaleEN)

	dynamic := []string{"# Dynamic Section\nDynamic content here"}
	sections := loader.LoadWithDynamic("/tmp", dynamic)

	if len(sections) != 13 {
		t.Errorf("expected 13 sections, got %d", len(sections))
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

func TestLoaderClearCache(t *testing.T) {
	loader := NewLoader(nil, i18n.LocaleEN)
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
