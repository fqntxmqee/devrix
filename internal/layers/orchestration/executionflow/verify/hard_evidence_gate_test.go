package verify

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
)

func TestGateVerdictPass_FlagOffAlwaysAdmits(t *testing.T) {
	defer SetHardEvidenceEnabledForTest(false)()
	admit, err := GateVerdictPass(nil, "code")
	if err != nil || !admit {
		t.Fatalf("flag-off should always admit; got (%v,%v)", admit, err)
	}
	admit, err = GateVerdictPass(nil, "chat")
	if err != nil || !admit {
		t.Fatalf("flag-off chat should admit; got (%v,%v)", admit, err)
	}
}

func TestGateVerdictPass_FlagOnCodeWithLog(t *testing.T) {
	defer SetHardEvidenceEnabledForTest(true)()
	ev := &interfaces.Evidence{LogExcerpt: "some log"}
	admit, err := GateVerdictPass(ev, "code")
	if err != nil || !admit {
		t.Fatalf("code with log should admit; got (%v,%v)", admit, err)
	}
}

func TestGateVerdictPass_FlagOnCodeEmptyRejects(t *testing.T) {
	defer SetHardEvidenceEnabledForTest(true)()
	admit, err := GateVerdictPass(nil, "code")
	if err == nil {
		t.Fatalf("flag-on empty code should reject with error")
	}
	if admit {
		t.Fatalf("flag-on empty code should NOT admit")
	}
}

func TestGateVerdictPass_FlagOnChatRejectsWithoutCoherence(t *testing.T) {
	defer SetHardEvidenceEnabledForTest(true)()
	admit, err := GateVerdictPass(nil, "chat")
	if err == nil {
		t.Fatalf("nil-evidence chat should reject; chat requires CoherenceScore or EntityHash")
	}
	if admit {
		t.Fatalf("chat without coherence/entity should NOT admit")
	}
}

func TestGateVerdictPass_DefaultKindIsCode(t *testing.T) {
	defer SetHardEvidenceEnabledForTest(true)()
	// Empty kind → defaults to "code" → empty code rejects.
	admit, err := GateVerdictPass(nil, "")
	if err == nil {
		t.Fatalf("default-kind empty should reject (code rule)")
	}
	if admit {
		t.Fatalf("default-kind empty should NOT admit")
	}
}
