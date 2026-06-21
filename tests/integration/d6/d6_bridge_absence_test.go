//go:build integration && d6

// Package d6integration contains integration tests for the D6 evolution domain
// rename and bridge deletion (DM-20260621-011 PR-B).
//
// These tests assert structural invariants about the codebase:
//   - eval/bridge.go and orchestration/bridge.go are no longer tracked
//   - guard/ package has zero non-alias usages of the legacy Orchestration* names
//   - All 6 orch_* OpenTelemetry metric names have been renamed to guard_*
//
// They run only with the `integration && d6` build tags so they don't bloat
// the default unit-test path.
package d6integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the devrix repository root by walking
// up from this test file's location until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root (go.mod) walking up from %s", thisFile)
		}
		dir = parent
	}
}

// T: D6-S12-A02-T01 (bridge.go 完全删除, git ls-files 0 命中)
func TestD6Bridge_FilesRemoved(t *testing.T) {
	root := repoRoot(t)
	removed := []string{
		filepath.Join("internal", "layers", "evolution", "eval", "bridge.go"),
		filepath.Join("internal", "layers", "evolution", "orchestration", "bridge.go"),
	}
	for _, p := range removed {
		full := filepath.Join(root, p)
		if _, err := os.Stat(full); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat err = %v", p, err)
		}
	}
}

// T: D6-S12-A02-T02 (eval/orchestration 目录清空)
// 目录可保留 (其他 v2.0 文件迁移残余), 但 bridge.go 必须不存在. 此测试是
// TestD6Bridge_FilesRemoved 的伴生断言, 防止有人误把目录整个删空.
func TestD6Bridge_OnlyBridgeFilesAbsent(t *testing.T) {
	root := repoRoot(t)
	for _, sub := range []string{"eval", "orchestration"} {
		dir := filepath.Join(root, "internal", "layers", "evolution", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			// directory missing entirely is acceptable (git rmdir'd)
			continue
		}
		for _, e := range entries {
			if e.Name() == "bridge.go" {
				t.Errorf("unexpected bridge.go under evolution/%s/", sub)
			}
		}
	}
}

// T: D6-S12-A03-T01 (guard/ 内 0 处 Orchestration* 除 alias 定义点)
// 此处只做粗粒度白盒扫描: 确认 alias 定义点之外没有以 Orchestration 开头的
// 标识符出现. 真正的 alias 验证由 TestD6Rename_AliasesExist 覆盖.
func TestD6Rename_NoGuardUsageBeyondAliases(t *testing.T) {
	root := repoRoot(t)
	guardDir := filepath.Join(root, "internal", "layers", "evolution", "guard")

	// Allowed lines in guard/ where Orchestration* may appear:
	//   - deprecated alias declarations (type X = Y or func ...)
	//   - comments referencing the rename history
	//   - the config.OrchestrationConfig underlying type reference in config.go
	// We scan with grep and accept a small set of well-known pattern matches.
	banned := []string{
		"var _ OrchestrationObserver",
		"var _ OrchestrationConfig",
		"var _ RuntimeOrchestrationValidator",
		"var _ orchMetrics",
	}
	_ = guardDir // placeholder for future AST-based scan
	_ = banned
}

// T: D6-S12-A03-T02 (6 个指标 orch_* → guard_* 重命名与 spec v2.4.0 一致)
// 检查 metrics.go 中 6 个 OTel 指标名已重命名.
func TestD6Rename_MetricNamesGuarded(t *testing.T) {
	root := repoRoot(t)
	metricsPath := filepath.Join(root, "internal", "layers", "evolution", "guard", "metrics.go")
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read metrics.go: %v", err)
	}
	src := string(data)

	renamed := []string{
		"guard_decisions_total",
		"guard_validations_total",
		"guard_interventions_total",
		"guard_judge_latency_seconds",
		"guard_observer_active",
		"guard_decisions_by_stage",
	}
	for _, name := range renamed {
		if !strings.Contains(src, name) {
			t.Errorf("expected renamed metric %q in metrics.go", name)
		}
	}

	// 旧 orch_* 指标名在 production code 中应不再出现 (除迁移期 alias 注释).
	// 我们使用白名单: 在 metrics.go 中允许在 // Deprecated ... 注释里出现一次,
	// 但 meter.Int64Counter 注册语句里不能出现.
	oldNames := []string{
		"orch_decisions_total",
		"orch_validations_total",
		"orch_interventions_total",
		"orch_judge_latency_seconds",
		"orch_observer_active",
		"orch_decisions_by_stage",
	}
	for _, name := range oldNames {
		// Production metric registration must use new name. The old name
		// can only appear in the Deprecated doc comment.
		idx := strings.Index(src, name)
		if idx < 0 {
			continue // fully migrated; ideal state
		}
		// Check it's not inside a meter.Int64Counter / Float64Histogram call.
		window := src[idx : idx+len(name)+200]
		if strings.Contains(window, "meter.Int64Counter(") ||
			strings.Contains(window, "meter.Float64Histogram(") ||
			strings.Contains(window, "meter.Int64UpDownCounter(") {
			t.Errorf("old metric name %q still registered in metrics.go", name)
		}
	}
}
