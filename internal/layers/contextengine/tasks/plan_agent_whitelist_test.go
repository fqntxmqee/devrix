package tasks

import (
	"strings"
	"testing"
)

// T: D7-S5-T02 AC1 — 白名单非空且不含 write/edit/bash/delete。
func TestPlanAgent_Whitelist_NoWriteTools(t *testing.T) {
	if len(PlanAgentReadOnlyTools) == 0 {
		t.Fatalf("PlanAgentReadOnlyTools must be non-empty")
	}
	banned := map[string]bool{
		"write":  true,
		"edit":   true,
		"bash":   true,
		"delete": true,
	}
	for _, tool := range PlanAgentReadOnlyTools {
		if banned[tool] {
			t.Fatalf("read-only whitelist contains forbidden tool %q", tool)
		}
	}
}

// T: D7-S5-T02 AC2 — 黑名单非空且至少含 write/edit/bash。
func TestPlanAgent_Blacklist_NonEmpty(t *testing.T) {
	if len(PlanAgentForbiddenTools) == 0 {
		t.Fatalf("PlanAgentForbiddenTools must be non-empty")
	}
	required := []string{"write", "edit", "bash"}
	have := map[string]bool{}
	for _, t1 := range PlanAgentForbiddenTools {
		have[t1] = true
	}
	for _, r := range required {
		if !have[r] {
			t.Fatalf("PlanAgentForbiddenTools missing required entry %q", r)
		}
	}
}

// T: D7-S5-T02 AC3 — AllowedTools 与包级常量内容与顺序一致。
func TestPlanAgent_AllowedTools_Consistent(t *testing.T) {
	a := &PlanAgent{}
	got := a.AllowedTools()
	if len(got) != len(PlanAgentReadOnlyTools) {
		t.Fatalf("len(AllowedTools)=%d, want %d", len(got), len(PlanAgentReadOnlyTools))
	}
	for i := range PlanAgentReadOnlyTools {
		if got[i] != PlanAgentReadOnlyTools[i] {
			t.Fatalf("AllowedTools[%d]=%q, want %q", i, got[i], PlanAgentReadOnlyTools[i])
		}
	}
}

// T: D7-S5-T02 AC4 — IsReadOnlyTool 正确：白名单内 true，外部 false。
func TestPlanAgent_IsReadOnlyTool_Allowed(t *testing.T) {
	a := &PlanAgent{}
	for _, tool := range PlanAgentReadOnlyTools {
		if !a.IsReadOnlyTool(tool) {
			t.Fatalf("IsReadOnlyTool(%q) = false, want true (in whitelist)", tool)
		}
	}
}

func TestPlanAgent_IsReadOnlyTool_Forbidden(t *testing.T) {
	a := &PlanAgent{}
	for _, tool := range PlanAgentForbiddenTools {
		if a.IsReadOnlyTool(tool) {
			t.Fatalf("IsReadOnlyTool(%q) = true, want false (forbidden)", tool)
		}
	}
}

func TestPlanAgent_IsReadOnlyTool_Unknown(t *testing.T) {
	a := &PlanAgent{}
	for _, name := range []string{"", "unknown_tool", "Write", "READ"} {
		if a.IsReadOnlyTool(name) {
			t.Fatalf("IsReadOnlyTool(%q) = true, want false (case-sensitive)", name)
		}
	}
}

// T: D7-S5-T02 AC5 — buildPlanPrompt 注入白名单全部条目。
func TestPlanAgent_PromptInjectsWhitelist(t *testing.T) {
	prompt := buildPlanPrompt(PlanRequest{UserGoal: "explore", WorkDir: "/tmp"})
	if !strings.Contains(prompt, "Available tools") {
		t.Fatalf("prompt missing 'Available tools' marker; got: %q", prompt[:min(200, len(prompt))])
	}
	for _, tool := range PlanAgentReadOnlyTools {
		if !strings.Contains(prompt, tool) {
			t.Fatalf("prompt missing whitelist tool %q", tool)
		}
	}
}

// T: D7-S5-T02 AC5b — req.Tools 与白名单去重合并。
func TestPlanAgent_PromptMergesCallerTools(t *testing.T) {
	// caller supplies a write tool — kept in merged list (transparency),
	// but whitelist prefix still signals read-only intent.
	prompt := buildPlanPrompt(PlanRequest{
		UserGoal: "x",
		Tools:    []string{"bash", "read"}, // "read" is dup with whitelist
	})
	if !strings.Contains(prompt, "bash") {
		t.Fatalf("prompt should include caller-supplied tool 'bash' for transparency")
	}
	if !strings.Contains(prompt, "read") {
		t.Fatalf("prompt should include whitelist tool 'read'")
	}
	// "read" should appear at least once (whitelist position); we don't
	// assert exact count, but ensure no obvious duplication "read, read".
	if strings.Contains(prompt, "read, read") {
		t.Fatalf("prompt should de-duplicate 'read' between whitelist and caller")
	}
}

// T: D7-S5-T02 AC6 — nil receiver 不 panic，返回 false。
func TestPlanAgent_NilReceiver_NoPanic(t *testing.T) {
	var a *PlanAgent
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil receiver panicked: %v", r)
		}
	}()
	if a.IsReadOnlyTool("read") {
		t.Fatalf("nil receiver IsReadOnlyTool = true, want false")
	}
	if a.IsReadOnlyTool("write") {
		t.Fatalf("nil receiver IsReadOnlyTool(write) = true, want false")
	}
	// AllowedTools on nil receiver is OK (no method body access to a).
	tools := a.AllowedTools()
	if len(tools) == 0 {
		t.Fatalf("nil receiver AllowedTools should still return package constant")
	}
}

// T: D7-S5-T02 AC7 — 黑白名单不相交。
func TestPlanAgent_ListsDisjoint(t *testing.T) {
	forbidden := map[string]bool{}
	for _, t1 := range PlanAgentForbiddenTools {
		forbidden[t1] = true
	}
	for _, tool := range PlanAgentReadOnlyTools {
		if forbidden[tool] {
			t.Fatalf("tool %q appears in BOTH read-only whitelist and forbidden list", tool)
		}
	}
}

// T: 边界 — 极小白名单（仅 read）仍合法。
func TestPlanAgent_MinimalWhitelist_StillValid(t *testing.T) {
	// Save & restore to avoid mutating package state across tests.
	orig := PlanAgentReadOnlyTools
	t.Cleanup(func() { PlanAgentReadOnlyTools = orig })

	PlanAgentReadOnlyTools = []string{"read"}
	a := &PlanAgent{}
	if !a.IsReadOnlyTool("read") {
		t.Fatalf("IsReadOnlyTool(read) = false on minimal whitelist")
	}
	if a.IsReadOnlyTool("ls") {
		t.Fatalf("IsReadOnlyTool(ls) = true; minimal whitelist should reject it")
	}
	// Disjointness still holds with minimal whitelist.
	for _, fb := range PlanAgentForbiddenTools {
		for _, w := range PlanAgentReadOnlyTools {
			if fb == w {
				t.Fatalf("minimal whitelist %q collides with forbidden %q", w, fb)
			}
		}
	}
}
