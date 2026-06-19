package configure

import (
	"strings"
	"testing"
)

func TestDefaultCatalog_LoadsEmbedded(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog: %v", err)
	}
	if c.Size() == 0 {
		t.Fatal("embedded catalog should not be empty")
	}
	if c.Source() != "<embedded:data/models.yaml>" {
		t.Errorf("Source = %q, want <embedded:data/models.yaml>", c.Source())
	}
}

func TestCatalog_LookupKnownModel(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatalf("DefaultCatalog: %v", err)
	}
	caps := c.Lookup("MiniMax-M3")
	if caps == nil {
		t.Fatal("MiniMax-M3 should be in catalog")
	}
	if !caps.NativeThinking {
		t.Error("MiniMax-M3 NativeThinking = false, want true")
	}
	if caps.ContextWindow != 1000000 {
		t.Errorf("MiniMax-M3 ContextWindow = %d, want 1000000", caps.ContextWindow)
	}
	if caps.Provider != "minimax" {
		t.Errorf("MiniMax-M3 Provider = %q, want minimax (auto-filled)", caps.Provider)
	}
}

func TestCatalog_LookupInlineThinkingModel(t *testing.T) {
	c, _ := DefaultCatalog()
	caps := c.Lookup("MiniMax-M2.7-highspeed")
	if caps == nil {
		t.Fatal("MiniMax-M2.7-highspeed should be in catalog")
	}
	if caps.NativeThinking {
		t.Error("MiniMax-M2.7-highspeed NativeThinking = true, want false (inline <think>)")
	}
}

func TestCatalog_LookupUnknown(t *testing.T) {
	c, _ := DefaultCatalog()
	if caps := c.Lookup("nonexistent-model-xyz"); caps != nil {
		t.Errorf("unknown model should return nil, got %+v", caps)
	}
}

func TestCatalog_LookupEmpty(t *testing.T) {
	c, _ := DefaultCatalog()
	if caps := c.Lookup(""); caps != nil {
		t.Errorf("empty model ID should return nil, got %+v", caps)
	}
}

func TestCatalog_NilSafe(t *testing.T) {
	var c *ModelCatalog
	if got := c.Lookup("anything"); got != nil {
		t.Errorf("nil catalog should return nil for Lookup")
	}
	if got := c.Size(); got != 0 {
		t.Errorf("nil catalog Size = %d, want 0", got)
	}
	if got := c.ListIDs(); got != nil {
		t.Errorf("nil catalog ListIDs = %v, want nil", got)
	}
}

func TestCatalog_ListIDsSorted(t *testing.T) {
	c, _ := DefaultCatalog()
	ids := c.ListIDs()
	if len(ids) < 2 {
		t.Skip("need at least 2 models")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Errorf("ListIDs not sorted at %d: %q >= %q", i, ids[i-1], ids[i])
			break
		}
	}
}

func TestCatalog_StringContainsCount(t *testing.T) {
	c, _ := DefaultCatalog()
	s := c.String()
	if !strings.Contains(s, "ModelCatalog{") {
		t.Errorf("String() missing prefix: %s", s)
	}
	if !strings.Contains(s, "entries=") {
		t.Errorf("String() missing entry count: %s", s)
	}
}

func TestCatalog_LoadFromFile_Override(t *testing.T) {
	// User can fork the catalog and load a private model.
	yaml := []byte(`
providers:
  custom:
    - id: my-private-model
      context_window: 32000
      native_thinking: true
`)
	c, err := parseCatalog(yaml)
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	if c.Lookup("my-private-model") == nil {
		t.Fatal("private model should load")
	}
	if c.Lookup("MiniMax-M3") != nil {
		t.Fatal("custom catalog should NOT have public models")
	}
}

func TestCatalog_MergedCatalog(t *testing.T) {
	// Simulate: start with default, add private models on top.
	def, _ := DefaultCatalog()
	private, _ := parseCatalog([]byte(`
providers:
  custom:
    - id: my-private-model
      native_thinking: true
`))
	merged := &ModelCatalog{byID: map[string]*ModelCapabilities{}}
	for _, id := range def.ListIDs() {
		merged.byID[id] = def.Lookup(id)
	}
	merged.byID["my-private-model"] = private.Lookup("my-private-model")

	if merged.Lookup("MiniMax-M3") == nil {
		t.Error("merged catalog should preserve public models")
	}
	if merged.Lookup("my-private-model") == nil {
		t.Error("merged catalog should include private models")
	}
}

func TestCatalog_SkipsEntriesWithEmptyID(t *testing.T) {
	c, err := parseCatalog([]byte(`
providers:
  minimax:
    - id: ""
      native_thinking: true
    - id: real-model
`))
	if err != nil {
		t.Fatalf("parseCatalog: %v", err)
	}
	if c.Size() != 1 {
		t.Errorf("expected 1 model (empty ID should be skipped), got %d", c.Size())
	}
	if c.Lookup("real-model") == nil {
		t.Error("real-model should be present")
	}
}