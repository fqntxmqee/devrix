// Package turn implements the D7 Turn Leader — the canonical LLM↔Tool turn loop
// owned by the Orchestration Domain (D7-S2-A06 RunTurnLoop, D7-S2-A07 InvokeLLM).
//
// DM-020 (D7 Turn 编排上移): TurnOrchestrator replaces the legacy D2-S16 turn loop
// as the single source of truth for turn execution. D7 directly calls D3 for LLM
// inference; D2 provides ContextPreparer, ToolRoundExecutor, and SessionPersister
// as pure execution primitives (Context Follower).
//
// v2.0 Slice plan:
//
//	a: skeleton + interfaces (this package)
//	b: bootstrap WireContextLLM → D7
//	c: FastPath calls TurnOrchestrator
//	d: D2 removes ILLMGateway + import lint
//	e: Autocompact D7→D3
//	f: Legacy adapter + all T green
package sessionorchestrator
