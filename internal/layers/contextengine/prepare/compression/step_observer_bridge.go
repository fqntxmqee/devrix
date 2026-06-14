package compression

import (
	"context"

	"github.com/devrix/devrix/internal/shared/types"
)

type pipelineStepObserver struct {
	sessionID string
	sink      CompressionEventSink
}

// NewPipelineStepObserver bridges pipeline steps to a CompressionEventSink.
func NewPipelineStepObserver(sessionID string, sink CompressionEventSink) StepObserver {
	if sink == nil {
		return nil
	}
	return &pipelineStepObserver{sessionID: sessionID, sink: sink}
}

func (o *pipelineStepObserver) OnStep(_ context.Context, step string, before, after int) {
	o.sink.EmitCompressionStep(o.sessionID, step, before, after)
}

func (o *pipelineStepObserver) OnAutocompact(meta AutocompactMeta) {
	o.sink.EmitAutocompact(o.sessionID, meta)
}

func (o *pipelineStepObserver) OnAutocompactComplete(summaryMsg types.Message, sessionID, asyncToken string) {
	if sessionID == "" {
		sessionID = o.sessionID
	}
	o.sink.EmitAutocompactComplete(sessionID, summaryMsg, asyncToken)
}
