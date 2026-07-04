package i18n

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/prompttags"
)

// WorkItemExecuteFieldLabels are machine-readable field headings for Execute-phase
// WorkItem system prompts. Always English — values (directive text) follow user locale.
var WorkItemExecuteFieldLabels = WorkItemExecuteLabels{
	Directive:      "Directive",
	ScopeIn:        "ScopeIn",
	ScopeOut:       "ScopeOut",
	ExpectedReturn: "ExpectedReturn",
}

// WorkItemExecuteLabels holds English field keys for WorkItem execute prompts.
type WorkItemExecuteLabels struct {
	Directive      string
	ScopeIn        string
	ScopeOut       string
	ExpectedReturn string
}

// WorkItemExecuteIntro is the opening line for Execute WorkItem system prompts.
func WorkItemExecuteIntro(loc Locale) string {
	if loc == LocaleEN {
		return "You are executing one WorkItem in a layered work tree."
	}
	return "你正在分层工作树中执行一个 WorkItem。"
}

// WorkItemExecuteOutputHints documents machine-readable output blocks for Execute.
// Locale prose + prompttags.ExecuteOutputTagDoc machine syntax (P2 DocBlock).
func WorkItemExecuteOutputHints(loc Locale) string {
	tagDoc := prompttags.ExecuteOutputTagDoc()
	if loc == LocaleEN {
		return workItemOutputFormatHeaderEN + "\n" + tagDoc + workItemOutputFormatFooterEN
	}
	return workItemOutputFormatHeaderZH + "\n" + tagDoc + workItemOutputFormatFooterZH
}

const workItemOutputFormatHeaderEN = `
## Work item output blocks (machine-readable)`

const workItemOutputFormatFooterEN = `
- Do not label observations as ObsFact/ObsSignal/ObsDeviation/ObsUncertainty; Observe classifies signals.
`

const workItemOutputFormatHeaderZH = `
## WorkItem 输出块（机器可读）`

const workItemOutputFormatFooterZH = `
- 不要自行标注 ObsFact/ObsSignal/ObsDeviation/ObsUncertainty；Observe 节点负责分类。
`

// WorkItemOutputFormatHintsEN is the English Execute output-hints block (tests / default).
// Deprecated: prefer WorkItemExecuteOutputHints(LocaleEN) which composes DocBlock tag syntax.
var WorkItemOutputFormatHintsEN = strings.TrimSpace(
	workItemOutputFormatHeaderEN + "\n" + prompttags.ExecuteOutputTagDoc() + workItemOutputFormatFooterEN,
)

// WorkItemOutputFormatHintsZH is the Chinese Execute output-hints block.
var WorkItemOutputFormatHintsZH = strings.TrimSpace(
	workItemOutputFormatHeaderZH + "\n" + prompttags.ExecuteOutputTagDoc() + workItemOutputFormatFooterZH,
)
