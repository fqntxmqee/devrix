//go:build integration && d6

// Package d6integration: D6-S12-A03 rename verification (DM-20260621-011 PR-B).
//
// Asserts that the deprecated alias declarations are wired correctly:
//   - RuntimeOrchestrationValidator = RuntimeGuardValidator
//   - OrchestrationObserver = GuardObserver
//   - OrchestrationConfig = GuardConfig (via guard/config.go)
// and that the legacy constructors still produce the same underlying value
// (i.e. type alias semantics, not type definition).
package d6integration

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/evolution/guard"
	"github.com/devrix/devrix/internal/shared/config"
)

// T: D6-S12-A03-T01 (alias 类型同一性)
// Deprecated aliases must be true type aliases (`type X = Y`), so an instance
// constructed via the old constructor is assignable to the new type without
// conversion. If anyone accidentally switches to `type X Y`, this test breaks.
func TestD6Rename_AliasesAreTypeAliases(t *testing.T) {
	var _ guard.RuntimeGuardValidator = guard.RuntimeOrchestrationValidator{}
	var _ guard.GuardObserver = guard.OrchestrationObserver{}
	var _ guard.GuardConfig = guard.OrchestrationConfig{}

	_ = guard.NewRuntimeGuardValidator // compile-time presence
	_ = guard.NewRuntimeOrchestrationValidator
	_ = guard.NewGuardObserver
	_ = guard.NewOrchestrationObserver
}

// T: D6-S12-A03-T03 (deprecated constructors 行为等价)
// Both constructors must produce a validator with the same underlying type
// and equal config (we cannot trivially inspect private state, but a non-nil
// return from both confirms the wiring is intact).
func TestD6Rename_OldNewConstructorsEquivalent(t *testing.T) {
	cfg := guard.GuardConfig{Enabled: true}
	judge := guard.NewRuntimeJudge(nil, cfg)
	exec := guard.NewInterventionExecutor(nil, nil, nil)

	v1 := guard.NewRuntimeGuardValidator(cfg, judge, exec)
	v2 := guard.NewRuntimeOrchestrationValidator(cfg, judge, exec)

	if v1 == nil || v2 == nil {
		t.Fatalf("constructors returned nil: v1=%v v2=%v", v1, v2)
	}
	if v1 == v2 {
		t.Errorf("expected distinct validator instances (each constructor allocates fresh)")
	}
}

// T: D6-S12-A03-T04 (deprecated observer constructor 行为等价)
func TestD6Rename_OldNewObserverConstructorsEquivalent(t *testing.T) {
	cfg := guard.GuardConfig{Enabled: true}
	judge := guard.NewRuntimeJudge(nil, cfg)
	exec := guard.NewInterventionExecutor(nil, nil, nil)
	v := guard.NewRuntimeGuardValidator(cfg, judge, exec)

	o1 := guard.NewGuardObserver(v, nil, nil)
	o2 := guard.NewOrchestrationObserver(v, nil, nil)

	if o1 == nil || o2 == nil {
		t.Fatalf("observer constructors returned nil: o1=%v o2=%v", o1, o2)
	}
}

// T: D6-S12-A03-T05 (shared/config 默认值可作为 guard.GuardConfig 使用)
// shared/config.OrchestrationConfig 是 guard.GuardConfig 的底层类型 (type alias),
// 所以从 shared/config 工厂返回的实例可直接用作 guard.GuardConfig 参数.
// 这是 cmd/devrix/main.go 的 initOrchestration 实际调用方式.
func TestD6Rename_SharedConfigCompatibleWithGuardConfig(t *testing.T) {
	cfg := config.DefaultOrchestrationConfig()
	judge := guard.NewRuntimeJudge(nil, cfg)
	exec := guard.NewInterventionExecutor(nil, nil, nil)

	v := guard.NewRuntimeGuardValidator(cfg, judge, exec)
	if v == nil {
		t.Fatal("NewRuntimeGuardValidator with shared/config cfg returned nil")
	}
}
