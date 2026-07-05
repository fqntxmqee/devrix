package sessionorchestrator

import (
	"errors"
	"strings"

	"github.com/devrix/devrix/internal/shared/prompttags"
)

func parseRejectFromStrategicPlan(reject *StrategicPlanReject) prompttags.ParseRejectRecord {
	if reject == nil {
		return prompttags.ParseRejectRecord{}
	}
	code := prompttags.RejectBudgetCap
	switch strings.TrimSpace(reject.Reason) {
	case BudgetFieldUncertainty:
		code = prompttags.RejectUncertaintyGate
	case BudgetFieldScope:
		code = prompttags.RejectScopeGate
	}
	field := strings.TrimSpace(reject.Field)
	if field == "" {
		field = strings.TrimSpace(reject.Reason)
	}
	return prompttags.NewPlanParseReject(code, field, reject.Error(), reject.Requested, reject.MaxAllowed)
}

func parseRejectFromPlanError(err error) prompttags.ParseRejectRecord {
	if err == nil {
		return prompttags.ParseRejectRecord{}
	}
	var reject *StrategicPlanReject
	if errors.As(err, &reject) {
		return parseRejectFromStrategicPlan(reject)
	}
	return prompttags.NewPlanParseReject(prompttags.RejectParseFail, "", err.Error(), 0, 0)
}

func parseRejectFromObserveError(err error, snippet string) prompttags.ParseRejectRecord {
	if err == nil {
		return prompttags.ParseRejectRecord{}
	}
	return prompttags.NewObserveParseReject(prompttags.RejectParseFail, err.Error(), snippet)
}
