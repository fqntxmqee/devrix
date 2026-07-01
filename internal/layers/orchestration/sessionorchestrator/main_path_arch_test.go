package sessionorchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/mups/learn"
	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// newMainPathTestOrchestrator wires the production-shape ingress:
// RunSessionTurnLoop + WorkTree + ItemPipelineRunner (MUPS), with a
// capturing WorkItemExecutor and a legacy TurnExecutor stub that must
// never be invoked for user messages.
func newMainPathTestOrchestrator(t *testing.T) (*SessionOrchestrator, *fakeD2, *capturingWorkItemExecutor, *workmodel.TaskManager) {
	t.Helper()
	exec := &fakeD2{}
	capt := &capturingWorkItemExecutor{}
	tm := workmodel.NewTaskManager()
	skill := learn.NewSkillMemory()
	feedback := learn.NewFeedbackMemory()
	scheduled := learn.NewScheduledMemory()
	rep := learn.NewInMemoryReputationStore()
	learner := learn.NewDefaultLearner(skill, feedback, scheduled, rep, learn.NewAssetBuilder())
	runner, err := NewItemPipelineRunner(ItemPipelineDeps{
		Executor: capt,
		Learner:  learner,
		Tasks:    tm,
	})
	if err != nil {
		t.Fatalf("NewItemPipelineRunner: %v", err)
	}
	orch := NewSessionOrchestrator(orchtypes.DefaultConfig(), exec,
		WithTaskManager(tm),
		WithItemPipelineRunner(runner),
	)
	return orch, exec, capt, tm
}

// Covers: D7 main-path architecture guard (TurnLoop + WorkTree + MUPS).
//
// Invariant: ProcessMessage for user instructions must:
//  1. NOT call legacy TurnExecutor.RunTurn (RunTurn is sub-agent only)
//  2. Seed WorkTree via EnsureGoal
//  3. Drive ItemPipelineRunner (MUPS Execute phase) via RunSessionTurnLoop
func TestD7MainPath_ProcessMessage_TurnLoopWorkTreeMUPS(t *testing.T) {
	const sessionID = "sess-d7-main-path"
	orch, legacyExec, mupsExec, tm := newMainPathTestOrchestrator(t)

	ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
		SessionID: sessionID,
		Message:   "review d2 domain code",
	})
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	events := drainEvents(ch)
	if !hasEventType(events, "complete") {
		t.Fatalf("expected complete, got %v", loopEventTypes(events))
	}
	if !hasEventType(events, "pipeline_round") {
		t.Fatalf("expected pipeline_round (MUPS), got %v", loopEventTypes(events))
	}

	if legacyExec.calls != 0 {
		t.Fatalf("legacy TurnExecutor.RunTurn calls = %d, want 0 (RunTurn is sub-agent only)", legacyExec.calls)
	}
	if len(mupsExec.calls) == 0 {
		t.Fatal("ItemPipelineRunner never invoked WorkItemExecutor.ExecuteWorkItem")
	}
	if mupsExec.calls[0].SessionID != sessionID {
		t.Fatalf("ExecuteWorkItem session = %q, want %q", mupsExec.calls[0].SessionID, sessionID)
	}

	roots := tm.Tree().ListRoots(sessionID)
	if len(roots) == 0 {
		t.Fatal("WorkTree has no root goal after ProcessMessage")
	}
	if roots[0].Kind != workmodel.WorkKindGoal {
		t.Fatalf("root kind = %q, want goal", roots[0].Kind)
	}
}

// IntentFast and IntentOrchestrate are classification labels only; both
// must route through the same RunSessionTurnLoop ingress.
func TestD7MainPath_IntentFastAndOrchestrate_SameIngress(t *testing.T) {
	kinds := []orchtypes.IntentKind{orchtypes.IntentFast, orchtypes.IntentOrchestrate}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			orch, legacyExec, mupsExec, _ := newMainPathTestOrchestrator(t)
			orch.classifier = &forcedKindClassifier{kind: kind}

			ch, err := orch.ProcessMessage(context.Background(), orchtypes.ProcessRequest{
				SessionID: "sess-" + string(kind),
				Message:   "implement feature",
			})
			if err != nil {
				t.Fatalf("ProcessMessage: %v", err)
			}
			for range ch {
			}
			if legacyExec.calls != 0 {
				t.Fatalf("%s: legacy RunTurn calls = %d, want 0", kind, legacyExec.calls)
			}
			if len(mupsExec.calls) == 0 {
				t.Fatalf("%s: MUPS Execute never ran", kind)
			}
		})
	}
}

// Retired ingress files must stay deleted (FastPath / OrchestratePath).
func TestD7MainPath_RetiredIngressFilesAbsent(t *testing.T) {
	root := findModuleRoot(t)
	retired := []string{
		"internal/layers/orchestration/sessionorchestrator/fastpath.go",
		"internal/layers/orchestration/sessionorchestrator/orchestrate_path.go",
	}
	for _, rel := range retired {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Fatalf("retired ingress file still present: %s", rel)
		}
	}
}

// T: D7-SN-T02 / D7-SN-T03 / D7-HC-T02 / D7-HC-T03 (DM-20260701-002 / DM-20260701-003)
//
// D7 current canonical S must stay normalized to S1-S6. Historical MUPS
// nodes (former S7-S14) and TaskContract IDs (former S20/S21) may remain
// in mapping sections or historical-s-mapping.md, but must not be reintroduced
// as current canonical scenario rows or registry headings.
func TestD7MainPath_CanonicalSLayerNormalized(t *testing.T) {
	root := findModuleRoot(t)
	specPath := filepath.Join(root, "openspec/specs/d7-orchestration/spec.md")
	specData, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}
	spec := string(specData)
	for _, needle := range []string{
		"| D7-S6 | MUPS Governance |",
		"### Historical / Contract Mapping",
		"historical-s-mapping.md",
	} {
		if !strings.Contains(spec, needle) {
			t.Fatalf("spec.md missing normalized S-layer marker %q", needle)
		}
	}
	for _, retiredCurrent := range []string{
		"| D7-S8 | Observation + UncertaintyReport |",
		"| D7-S20 | TaskSpec 下行 |",
		"| D7-S21 | TaskReport 上行 |",
	} {
		if strings.Contains(spec, retiredCurrent) {
			t.Fatalf("spec.md reintroduced former S as current scenario: %q", retiredCurrent)
		}
	}
	if !strings.Contains(spec, "explicit wave") {
		t.Fatalf("spec.md missing S3 explicit wave/background positioning")
	}
	for _, retiredArch := range []string{
		"FastPath.Run",
		"OrchestratePath.Run",
	} {
		if strings.Contains(spec, retiredArch) {
			t.Fatalf("spec.md Architecture still shows retired ingress: %q", retiredArch)
		}
	}

	registryPath := filepath.Join(root, "openspec/specs/d7-orchestration/a-registry.md")
	registryData, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read a-registry.md: %v", err)
	}
	registry := string(registryData)
	for _, retiredHeading := range []string{
		"### D7-S7: MUPS 5 节点管道入口",
		"### D7-S8: Observe 节点 ✅ Canonical",
		"### Historical: D7-S7",
		"## D7-S20 / S21: TaskContract 统一",
	} {
		if strings.Contains(registry, retiredHeading) {
			t.Fatalf("a-registry.md reintroduced retired current heading: %q", retiredHeading)
		}
	}
	if !strings.Contains(registry, "historical-s-mapping.md") {
		t.Fatalf("a-registry.md missing pointer to historical-s-mapping.md")
	}

	fRegistryPath := filepath.Join(root, "openspec/specs/d7-orchestration/f-registry.md")
	fRegistryData, err := os.ReadFile(fRegistryPath)
	if err != nil {
		t.Fatalf("read f-registry.md: %v", err)
	}
	fRegistry := string(fRegistryData)
	for _, retiredFHeading := range []string{
		"## D7-S8-A15 ObserveQuantize",
		"## D7-S14-A50..A52 EscapeEngine",
		"fastpath.go",
	} {
		if strings.Contains(fRegistry, retiredFHeading) {
			t.Fatalf("f-registry.md reintroduced retired F heading or path: %q", retiredFHeading)
		}
	}

	histPath := filepath.Join(root, "openspec/specs/d7-orchestration/historical-s-mapping.md")
	if _, err := os.Stat(histPath); err != nil {
		t.Fatalf("historical-s-mapping.md missing: %v", err)
	}
}

// Bootstrap must wire ItemPipelineRunner so ProcessMessage can reach MUPS.
func TestD7MainPath_BootstrapWiresItemPipeline(t *testing.T) {
	root := findModuleRoot(t)
	wire := filepath.Join(root, "internal/bootstrap/wire_coordinator.go")
	data, err := os.ReadFile(wire)
	if err != nil {
		t.Fatalf("read wire_coordinator: %v", err)
	}
	body := string(data)
	for _, needle := range []string{
		"WireItemPipeline(",
		"WithItemPipelineRunner(itemRunner)",
		"WithTaskManager(tm)",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("wire_coordinator.go missing %q — main path not wired", needle)
		}
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
