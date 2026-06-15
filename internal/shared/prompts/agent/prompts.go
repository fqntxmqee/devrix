// Package agent holds embedded worker role prompts shared across domains.
package agent

import (
	_ "embed"
)

//go:embed explore.md
var ExplorePrompt string

//go:embed plan.md
var PlanPrompt string

//go:embed implement.md
var ImplementPrompt string
