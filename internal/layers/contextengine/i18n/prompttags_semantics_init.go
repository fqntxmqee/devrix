package i18n

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// init() registers (frame, tag) pairs that have glossary entries for InputRulesForFrame
// with prompttags.RegisterFrameFieldGuide. Runs at package load; subsequent
// prompttags.MustRegisterFrame calls in other packages' init() will see this registry populated.
//
// When a new tag is added to a FrameSpec, the matching SemanticCondition glossary entry MUST be
// added to semanticGlossary{ZH,EN} or MustRegisterFrame will panic at init time.
func init() {
	phaseToFrame := map[contracts.MUPSPhase]prompttags.FrameName{
		contracts.MUPSPhaseObserve: prompttags.FrameObserveUser,
		contracts.MUPSPhasePlan:    prompttags.FramePlanUser,
	}
	for _, frame := range phaseToFrame {
		for _, rule := range prompttags.InputRulesForFrame(frame) {
			prompttags.RegisterFrameFieldGuide(frame, prompttags.TagName(rule.Target))
		}
	}
}
