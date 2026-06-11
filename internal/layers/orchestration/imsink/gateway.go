package imsink

import (
	"github.com/devrix/devrix/internal/shared/contracts"
)

// EngineEventSink receives engine events for outbound IM delivery.
type EngineEventSink interface {
	Emit(ev *contracts.EngineEvent)
}

// GatewaySink adapts FlowEvent to worker_progress EngineEvents.
type GatewaySink struct {
	sink EngineEventSink
}

// NewGatewaySink creates an IM sink targeting Gateway event stream.
func NewGatewaySink(sink EngineEventSink) *GatewaySink {
	return &GatewaySink{sink: sink}
}

// EmitWorkerProgress implements flow.IMSink.
func (g *GatewaySink) EmitWorkerProgress(ev contracts.FlowEvent) {
	if g == nil || g.sink == nil || ev.SessionID == "" {
		return
	}
	meta := map[string]string{
		"event_type": "worker_progress",
		"render":     "worker_tree",
		"flow_id":    ev.FlowID,
		"worker_id":  ev.WorkerID,
		"task_id":    ev.TaskID,
		"source":     string(ev.Source),
		"role":       ev.Role,
		"kind":       string(ev.Kind),
	}
	for k, v := range ev.Metadata {
		meta[k] = v
	}
	content := ev.Summary
	if content == "" {
		content = string(ev.Kind)
	}
	g.sink.Emit(&contracts.EngineEvent{
		Type:      "worker_progress",
		Content:   content,
		SessionID: ev.SessionID,
		Metadata:  meta,
	})
}
