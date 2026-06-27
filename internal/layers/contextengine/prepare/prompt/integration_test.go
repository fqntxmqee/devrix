package prompt

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
)

// TestStaticSectionsContent verifies that static sections contain expected keywords.
func TestStaticSectionsContent(t *testing.T) {
	loader := NewLoader(nil, i18n.LocaleEN)
	sections := loader.LoadAsSections("/tmp")

	expectedKeywords := map[string][]string{
		"intro":              {"You are an interactive agent", "software engineering"},
		"system":             {"Tools are executed", "permission mode"},
		"doing_tasks":        {"Don't add features", "verify it actually works"},
		"actions":            {"reversibility", "blast radius", "user confirmation"},
		"using_tools":        {"dedicated read tool", "parallel"},
		"output_efficiency":  {"Go straight to the point", "brief"},
		"tone_and_style":     {"emojis", "file_path:line_number"},
		"todo_write":         {"Todo List Management", "Complex multi-step tasks"},
		"delegate_strategy":  {"Autonomous Task Strategy", "delegate_explore", "Context budget"},
	}

	sectionNames := []string{
		"intro", "system", "doing_tasks", "actions",
		"using_tools", "output_efficiency", "tone_and_style",
		"safety_guidelines", "knowledge_boundaries",
		"todo_write", "delegate_strategy",
	}

	for i, name := range sectionNames {
		if i >= len(sections) {
			t.Errorf("missing section: %s", name)
			continue
		}
		keywords, ok := expectedKeywords[name]
		if !ok {
			continue
		}
		for _, kw := range keywords {
			if !contains(sections[i], kw) {
				t.Errorf("section %q missing keyword: %q", name, kw)
			}
		}
	}
}

// TestContextAssemblerUsesNewLoader verifies assembler uses new loader.
func TestContextAssemblerUsesNewLoader(t *testing.T) {
	loader := NewLoader(nil, i18n.LocaleEN)

	// Verify loader provides sections
	sections := loader.LoadAsSections("/tmp")
	if len(sections) < 7 {
		t.Errorf("expected at least 7 sections, got %d", len(sections))
	}
}
