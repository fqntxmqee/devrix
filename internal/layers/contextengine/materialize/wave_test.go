package materialize

import (
	"testing"
)

func TestPolicyFromWaveContext(t *testing.T) {
	tests := []struct {
		in   string
		want Mode
	}{
		{WavePolicyFresh, ModeFresh},
		{WavePolicyResume, ModeResume},
		{WavePolicyUpstream, ModeUpstream},
	}
	for _, tc := range tests {
		got := PolicyFromWaveContext(tc.in)
		if got.Mode != tc.want {
			t.Errorf("PolicyFromWaveContext(%q).Mode = %q, want %q", tc.in, got.Mode, tc.want)
		}
	}
}

func TestMaterializeWave_Fresh(t *testing.T) {
	m := NewDefaultMaterializer(nil, "")
	res, err := m.Materialize(t.Context(), Request{
		Partition: Partition{SessionID: "s1", Kind: PartitionWave},
		Policy:    PolicyFromWaveContext(WavePolicyFresh),
		SystemPrompt: "base",
		Signals: InboundSignals{
			Directive:       "do X",
			WaveExtraPrompt: "be terse",
			WaveFileScope:   []string{"src/api/**"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 1 || res.Messages[0].Content != "do X" {
		t.Fatalf("messages = %+v", res.Messages)
	}
	if !contains(res.SystemPrompt, "base") || !contains(res.SystemPrompt, "be terse") || !contains(res.SystemPrompt, "src/api/**") {
		t.Fatalf("system prompt = %q", res.SystemPrompt)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
