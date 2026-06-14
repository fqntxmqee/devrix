// Package hubspoke — D7 Hub-Spoke orchestration: dispatch, bridge, and flow publishing.
//
// SpokeDispatcher is the single entry point for delegate_* tool calls.
// It selects the appropriate Spoke (D4 Worker or D2 SubQuery), wires the
// corresponding FlowBridge, and publishes terminal FlowEvents through
// the single ExecutionFlowHub export.
//
// DSAFT: D7-S2 (DispatchWorker) + D7-S4 (AggregateFlow/SpokeBridge)
package hubspoke
