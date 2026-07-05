package prompttags

import (
	"strings"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// SemanticBlock returns locale-neutral JSON-lines rules for phase output semantics.
func SemanticBlock(phase contracts.MUPSPhase) string {
	sem := SemanticsForPhase(phase)
	if len(sem.OutputRules) == 0 {
		return ""
	}
	var lines []string
	for _, rule := range sem.OutputRules {
		if line := rule.MachineLine(); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// SemanticConditionsForPhase returns glossary keys referenced by phase output rules.
func SemanticConditionsForPhase(phase contracts.MUPSPhase) []SemanticCondition {
	return uniqueConditions(SemanticsForPhase(phase).OutputRules)
}
