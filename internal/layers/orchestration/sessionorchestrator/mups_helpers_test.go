package sessionorchestrator

import (
	"strings"
	"testing"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/contracts"
)

func TestMergeMUPSPreparedSystem_doesNotDuplicateObserveAppendix(t *testing.T) {
	appendix := i18n.ObservationTaskAppendix(i18n.LocaleZH)
	prepared := contracts.MUPSPreparedContext{
		SystemPrompt:  "devrix_core\n\n" + appendix,
		PhaseAppendix: appendix,
	}
	got := mergeMUPSPreparedSystem(prepared)
	if strings.Count(got, appendix) != 1 {
		t.Fatalf("appendix count = %d, want 1\n%s", strings.Count(got, appendix), got)
	}
}

func TestMergeMUPSPreparedSystem_doesNotDuplicatePlanAppendix(t *testing.T) {
	dims := `{"citation":["file_line"]}`
	appendix := i18n.StrategicPlanAppendix(i18n.LocaleZH, dims)
	prepared := contracts.MUPSPreparedContext{
		SystemPrompt:  "devrix_core\n\n" + appendix,
		PhaseAppendix: appendix,
	}
	got := mergeMUPSPreparedSystem(prepared)
	if strings.Count(got, appendix) != 1 {
		t.Fatalf("appendix count = %d, want 1\n%s", strings.Count(got, appendix), got)
	}
}
