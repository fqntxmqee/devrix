// Package executionflow is the D7-S4 ExecutionFlow + Verify layer (v6.0.0
// 6 S + 1 横切 layout). It aggregates FlowEvent → WorkPlan and runs the
// 5-node MUPS Pipeline Verify/Learn nodes.
//
// Architecture (v6.0.0):
//
//	S4 ExecutionFlow+Verify (Costly Signaler + Certifier) → THIS PACKAGE
//	  ├── session_queue.go        (Hub↔runtime command drain, in-package)
//	  ├── delegate_progress_test.go (render delegate-progress FlowEvent)
//	  ├── session_queue_test.go    (drain ordering tests)
//	  ├── bridge/                  (FlowEvent → WorkPlan aggregator)
//	  ├── hub/                     (ExecutionFlowHub implementation)
//	  ├── imsink/                  (IM message adapter)
//	  ├── verify/                  (Certifier role + VerdictKind + ExitReason)
//	  └── workplan/                (WorkPlan canonical model)
//
// Why session_queue.go lives at parent level (not in a sub-package):
//   - It's the shared drain contract used by both bridge/ and hub/, so a
//     sub-package import boundary would force circular refs.
//   - 1 prod + 2 test files only — no independent test value as a sub-package.
//
// Why verify/ is its own sub-package: 4 VerdictKind + 14 ExitReason +
// AggregateVerdicts deserve first-class citizenship in the directory tree
// (certifier role separation).
package executionflow