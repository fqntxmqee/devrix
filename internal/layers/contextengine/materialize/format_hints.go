package materialize

// WorkItemOutputFormatHints documents machine-readable output blocks for WorkItem
// Execute. I/O shape only — no business tactics (how to review, which tools, etc.).
// Loaded into system prompts via buildSystemPrompt / deliveryHintBlock.
const WorkItemOutputFormatHints = `
## Work item output blocks (machine-readable)
- Verifiable conclusion: <conclusion>...</conclusion>
- Residual uncertainty: <open_questions>one question per line</open_questions>
- Scope boundary (optional JSON): <scope_contract>{"goal_statement":"","in_scope":[],"out_of_scope":[],"assumptions":[],"open_questions":[],"success_criteria":[]}</scope_contract>
- Deliverable schema (only when ExpectedReturn or directive already names one): <deliverable_schema>{registered_schema}</deliverable_schema>
- Do not label observations as ObsFact/ObsSignal/ObsDeviation/ObsUncertainty; Observe classifies signals.
`
