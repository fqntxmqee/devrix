// Package hardening provides cross-cutting discipline keeping concerns for
// D7 orchestration: observability counters (metrics) and LLM error recovery
// helpers (recovery). It implements the "Discipline Keeper" 横切 (cross-cutting)
// role in the v6.0.0 6 S + 1 横切 layout.
//
// Architecture (v6.0.0):
//
//	6 S + 1 横切:
//	  S1 WorkModel           (State Authority)
//	  S2 SessionOrchestrator (Mediator + Turn Leader + Error Recovery)
//	  S3 WaveScheduler       (Mechanism Designer)
//	  S4 ExecutionFlow+Verify (Costly Signaler + Certifier)
//	  S5 DecisionPlanning+Observe (Info Producer + Quantizer)
//	  S6 MUPS Pipeline       (Pipeline Coordinator + Memory Curator)
//	  横切 Hardening         (Discipline Keeper) → THIS PACKAGE
//
// hardening/ contents:
//   - metrics.go: InterruptMetrics cross-canceller failure counters
//   - recovery.go: LLM error recovery helpers (context length, 5xx, output truncation)
//
// What hardening/ does NOT contain (intentional):
//   - escape.CircuitBreaker (5-Layer CB is core to EscapeEngine, see Decision 1)
//   - turn.Orchestrator (Mediator+Turn Leader is S2, see Decision 2)
package hardening