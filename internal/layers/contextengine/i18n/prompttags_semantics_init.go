package i18n

import (
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/prompttags"
)

// init() registers the (frame, tag) pairs that have a when-use entry in
// prompttagsSemantics_{en,zh}.go with prompttags.RegisterFrameGuide.
// Runs at package load; subsequent prompttags.MustRegisterFrame[T] calls
// in other packages' init() will see this registry populated.
//
// When a new tag is added to a FrameSpec, the matching i18n entry MUST be
// added here (or directly to prompttagsSemantics_{en,zh}.go) or
// MustRegisterFrame will panic at init time (DM-20260705-003 invariant 5).
func init() {
	phaseToFrame := map[contracts.MUPSPhase]prompttags.FrameName{
		contracts.MUPSPhaseObserve: prompttags.FrameObserveUser,
		contracts.MUPSPhasePlan:    prompttags.FramePlanUser,
	}
	for phase, frame := range phaseToFrame {
		for _, rule := range prompttags.SemanticsForPhase(phase).InputRules {
			prompttags.RegisterFrameFieldGuide(frame, prompttags.TagName(rule.Name))
		}
	}
}
