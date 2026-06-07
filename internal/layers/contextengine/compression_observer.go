package contextengine

import (
	"github.com/devrix/devrix/internal/layers/contextengine/compression"
)

type pipelineStepObserver struct {
	sessionID string
	observer  ICompressionObserver
}

func newPipelineStepObserver(sessionID string, observer ICompressionObserver) compression.StepObserver {
	if observer == nil {
		return nil
	}
	return &pipelineStepObserver{sessionID: sessionID, observer: observer}
}

func (o *pipelineStepObserver) OnStep(step string, before, after int) {
	o.observer.EmitCompressionStep(o.sessionID, step, before, after)
}

func (o *pipelineStepObserver) OnAutocompact(meta compression.AutocompactMeta) {
	o.observer.EmitAutocompact(o.sessionID, AutocompactMeta{
		Degraded:      meta.Degraded,
		SummaryTokens: meta.SummaryTokens,
		Model:         meta.Model,
	})
}
