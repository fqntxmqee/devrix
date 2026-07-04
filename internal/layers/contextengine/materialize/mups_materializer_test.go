package materialize

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
)

type stubMUPSPrepare struct {
	system string
}

func (s *stubMUPSPrepare) prepare(_ context.Context, _, _ string) (string, map[string]string, error) {
	return s.system, nil, nil
}

// T: D2-S15-A90-T01 — observe → empty tools + obs schema appendix.
func TestMaterializeForMUPS_Observe(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{
		PrepareBase: func(ctx context.Context, sessionID, userMessage string) (string, map[string]string, error) {
			return "base prompt", nil, nil
		},
		FilterDeps: FilterPipelineDeps{Locale: i18n.LocaleEN},
	})
	got, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{
		Phase: contracts.MUPSPhaseObserve,
		Turn:  &contracts.MUPSTurnContext{SessionID: "s1"},
		UserMessage: "directive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("tools = %v, want empty", got.Tools)
	}
	if !strings.Contains(got.PhaseAppendix, "obs_fact") {
		t.Fatalf("appendix = %q", got.PhaseAppendix)
	}
	if !strings.Contains(got.SystemPrompt, "obs_fact") {
		t.Fatalf("system = %q", got.SystemPrompt)
	}
}

// T: D2-S15-A90-T02 — plan → empty tools + strategic plan appendix.
func TestMaterializeForMUPS_Plan(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{
		PrepareBase: func(ctx context.Context, sessionID, userMessage string) (string, map[string]string, error) {
			return "base", nil, nil
		},
		ContractDoc: `{"citation":["file_line"]}`,
		FilterDeps:  FilterPipelineDeps{Locale: i18n.LocaleEN},
	})
	got, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{
		Phase:       contracts.MUPSPhasePlan,
		Turn:        &contracts.MUPSTurnContext{SessionID: "s1"},
		UserMessage: "plan directive",
		Policy:      contracts.MUPSContextPolicy{Locale: "en"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("tools = %v", got.Tools)
	}
	if !strings.Contains(got.SystemPrompt, "execution_mode") {
		t.Fatalf("system = %q", got.SystemPrompt)
	}
}

// T: D2-S15-A90-T03 — execute implement includes read_file and edit_file.
func TestMaterializeForMUPS_ExecuteImplement(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{
		PartitionMat: NewDefaultMaterializer(nil, ""),
		FilterDeps: FilterPipelineDeps{
			Surfaces: []contracts.ToolSurface{&stubFilterSurface{}},
			Locale:   i18n.LocaleEN,
		},
	})
	got, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{
		Phase:       contracts.MUPSPhaseExecute,
		ToolProfile: "implement",
		Turn:        &contracts.MUPSTurnContext{SessionID: "s1", WorkDir: "/tmp"},
		WorkItem: &contracts.MUPSWorkItemSnapshot{
			ID:        "wi1",
			Directive: "do work",
			Partition: contracts.MUPSPartition{SessionID: "s1", WorkItemID: "wi1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(got.Tools)
	if !containsAll(names, "read_file", "edit_file") {
		t.Fatalf("tools = %v", names)
	}
}

// T: D2-S15-A90-T04 — execute readonly excludes write/bash.
func TestMaterializeForMUPS_ExecuteReadonly(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{
		PartitionMat: NewDefaultMaterializer(nil, ""),
		FilterDeps: FilterPipelineDeps{
			Surfaces: []contracts.ToolSurface{&stubFilterSurface{}},
			Locale:   i18n.LocaleEN,
		},
	})
	got, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{
		Phase:       contracts.MUPSPhaseExecute,
		ToolProfile: "readonly",
		Turn:        &contracts.MUPSTurnContext{SessionID: "s1", WorkDir: "/tmp"},
		WorkItem: &contracts.MUPSWorkItemSnapshot{
			Directive: "review",
			Partition: contracts.MUPSPartition{SessionID: "s1", WorkItemID: "wi1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range got.Tools {
		if tool.Name == "bash" || tool.Name == "edit_file" || strings.HasPrefix(tool.Name, "delegate") {
			t.Fatalf("readonly leaked %q", tool.Name)
		}
	}
}

// T: D2-S15-A90-T05 — rollup_synth → empty tools.
func TestMaterializeForMUPS_RollupSynth(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{
		PartitionMat: NewDefaultMaterializer(nil, ""),
		FilterDeps:   FilterPipelineDeps{Locale: i18n.LocaleEN},
	})
	got, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{
		Phase:       contracts.MUPSPhaseExecute,
		ToolProfile: "rollup_synth",
		Turn:        &contracts.MUPSTurnContext{SessionID: "s1"},
		WorkItem: &contracts.MUPSWorkItemSnapshot{
			Directive: "synth",
			Partition: contracts.MUPSPartition{SessionID: "s1", WorkItemID: "wi1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("tools = %v", got.Tools)
	}
}

// T: D2-S15-A90-T06 — verify/learn/decide rejected.
func TestMaterializeForMUPS_NonMaterializablePhases(t *testing.T) {
	mat := NewMUPSMaterializer(MUPSMaterializerDeps{})
	for _, phase := range []contracts.MUPSPhase{
		contracts.MUPSPhaseVerify, contracts.MUPSPhaseLearn, contracts.MUPSPhaseDecide,
	} {
		_, err := mat.MaterializeForMUPS(context.Background(), contracts.MUPSContextRequest{Phase: phase})
		if !errors.Is(err, contracts.ErrPhaseNotMaterializable) {
			t.Fatalf("phase %s err = %v, want ErrPhaseNotMaterializable", phase, err)
		}
	}
}

func toolNames(tools []contracts.MUPSToolDescriptor) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Name
	}
	return out
}

func containsAll(haystack []string, needles ...string) bool {
	set := map[string]bool{}
	for _, h := range haystack {
		set[h] = true
	}
	for _, n := range needles {
		if !set[n] {
			return false
		}
	}
	return true
}
