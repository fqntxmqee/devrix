// Package agent holds embedded agent role prompts organized by role.
//
// Each prompt follows a structured format:
// identity → tone_and_formatting → doing_tasks → safety_and_boundaries → examples.
//
// To update a prompt, edit the corresponding .md file — no Go code change needed.
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
