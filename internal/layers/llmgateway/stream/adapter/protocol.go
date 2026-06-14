package adapter

// Protocol identifiers returned by IAdapter.Protocol().
//
// DSAFT: D3-S2-A01-F04 (AdapterProtocolMethod, v1.1 BREAKING).
const (
	// ProtocolOpenAICompatible identifies adapters that speak the
	// OpenAI-compatible /chat/completions SSE wire format.
	// Currently used by DeepSeekAdapter and MiniMaxAdapter.
	ProtocolOpenAICompatible = "openai-compatible"

	// ProtocolAnthropicNative is reserved for V3 Anthropic adapter.
	// Not implemented in v1.1.
	ProtocolAnthropicNative = "anthropic-native"

	// ProtocolStub is returned by stubAdapter in tests.
	ProtocolStub = "stub"
)