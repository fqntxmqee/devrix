package i18n

// RollupPlanningDenylist (RH-MUPS-12, DM-20260701-001 T-P1-6) is the
// canonical list of phrases the rollup verifier rejects as "planning
// meta" instead of an actual rollup deliverable. Previously hard-coded
// as a Go slice in sessionorchestrator/rollup_verify.go — the
// tactical-hardcoding rule from PR-A of devrix-mups-deliverable-
// convergence (DM-20260630-012) flagged it; this file is the fix.
//
// The list is locale-independent (the phrases are marker patterns the
// LLM emits, not language-specific user copy). ZH phrases are included
// alongside EN so a Chinese-language session still catches the
// "我将要" / "我将" tell.
func RollupPlanningDenylist() []string {
	return []string{
		"parallel explore",
		"我将要",
		"我将",
		"todo_write",
	}
}
