package adapter_test

import (
	"testing"

	"github.com/devrix/devrix/internal/layers/llmgateway/stream/adapter"
	sharedconfig "github.com/devrix/devrix/internal/shared/config"
)

// DSAFT: D3-S2-A01-T06 (IAdapter.Protocol() unit, v1.1 F5).
// Verifies all production adapters return the expected Protocol() identifier.
func TestIAdapter_Protocol_DeepSeek(t *testing.T) {
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL: "https://api.deepseek.com",
	}
	a := adapter.NewDeepSeekAdapter(cfg)
	if got := a.Protocol(); got != adapter.ProtocolOpenAICompatible {
		t.Errorf("DeepSeekAdapter.Protocol() = %q, want %q", got, adapter.ProtocolOpenAICompatible)
	}
	if got := a.Provider(); got != "deepseek" {
		t.Errorf("DeepSeekAdapter.Provider() = %q, want %q", got, "deepseek")
	}
}

func TestIAdapter_Protocol_MiniMax(t *testing.T) {
	cfg := sharedconfig.LLMProviderRuntimeConfig{
		BaseURL: "https://api.minimax.com",
	}
	a := adapter.NewMiniMaxAdapter(cfg)
	if got := a.Protocol(); got != adapter.ProtocolOpenAICompatible {
		t.Errorf("MiniMaxAdapter.Protocol() = %q, want %q", got, adapter.ProtocolOpenAICompatible)
	}
	if got := a.Provider(); got != "minimax" {
		t.Errorf("MiniMaxAdapter.Provider() = %q, want %q", got, "minimax")
	}
}

func TestIAdapter_Protocol_StubReturnsStub(t *testing.T) {
	sa := stubAdapter{provider: "test"}
	if got := sa.Protocol(); got != adapter.ProtocolStub {
		t.Errorf("stubAdapter.Protocol() = %q, want %q", got, adapter.ProtocolStub)
	}
}

func TestIAdapter_Protocol_ConstantsDistinct(t *testing.T) {
	// Reserved V3 protocol constant must not collide with v1.1 ones.
	if adapter.ProtocolAnthropicNative == adapter.ProtocolOpenAICompatible {
		t.Error("ProtocolAnthropicNative must differ from ProtocolOpenAICompatible")
	}
	if adapter.ProtocolStub == adapter.ProtocolOpenAICompatible {
		t.Error("ProtocolStub must differ from ProtocolOpenAICompatible")
	}
}