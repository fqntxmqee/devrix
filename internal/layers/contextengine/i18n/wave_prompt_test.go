package i18n

import (
	"strings"
	"testing"
)

func TestWavePromptLabelsFor_ZH(t *testing.T) {
	got := WavePromptLabelsFor(LocaleZH)
	if strings.Contains(got.AllowedFileScope, "Allowed file") {
		t.Fatalf("ZH wave labels must not be English: %+v", got)
	}
	if got.AllowedFileScope == "" || got.FilesChangedUpstream == "" || got.UpstreamErrorPrefix == "" {
		t.Fatalf("ZH wave labels incomplete: %+v", got)
	}
}

func TestWavePromptLabelsFor_EN(t *testing.T) {
	got := WavePromptLabelsFor(LocaleEN)
	if !strings.Contains(got.AllowedFileScope, "Allowed file scope") {
		t.Fatalf("EN wave labels: %+v", got)
	}
}
