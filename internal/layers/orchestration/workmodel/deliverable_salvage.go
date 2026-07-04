package workmodel

// SalvageDeliverablePayload extracts presentable findings from mixed execute
// output when strict verify rejected the artifact (CC-U2 presentation layer).
func SalvageDeliverablePayload(summary string, contract DeliverableContract) *DeliverablePayload {
	c := contract.Normalized()
	if c.Structure != DeliverableStructureFindingsJSON && !contractFindingsApplicable(c) {
		return nil
	}
	raw := extractDeliverableJSONObject(summary)
	if raw == nil {
		return nil
	}
	findings, ok := decodeFindingsFromJSONObject(raw, FindingParseSalvage)
	if !ok {
		return nil
	}
	return &DeliverablePayload{
		Schema:   DeliverableSchema(contract.CacheKey()),
		Findings: findings,
		Raw:      string(raw),
	}
}

func contractFindingsApplicable(c DeliverableContract) bool {
	n := c.Normalized()
	return n.Citation == DeliverableCitationFileLine && n.Severity == DeliverableSeverityP0P1
}

// SalvageSessionDeliverable walks session work items and salvages structured
// findings from artifact summaries (best-effort, task-agnostic).
func SalvageSessionDeliverable(tm *TaskManager, sessionID string) string {
	if tm == nil {
		return ""
	}
	for _, item := range tm.Tree().List(sessionID) {
		if item == nil || item.LastRound == nil {
			continue
		}
		lr := item.LastRound
		contract := lr.DeliverableContract
		if !contract.ContractApplicable() {
			contract = ExpandLegacySchemaToContract(lr.DeliverableSchema)
		}
		if !contract.ContractApplicable() {
			continue
		}
		payload := SalvageDeliverablePayload(lr.ArtifactSummary, contract)
		if formatted := FormatDeliverablePayloadForIM(payload); formatted != "" {
			return formatted
		}
	}
	return ""
}

// SalvageDeliverableFromRound returns IM-ready text from one pipeline round.
func SalvageDeliverableFromRound(round *WorkItemPipelineRound) string {
	if round == nil {
		return ""
	}
	contract := round.DeliverableContract
	if !contract.ContractApplicable() {
		contract = ExpandLegacySchemaToContract(round.DeliverableSchema)
	}
	payload := SalvageDeliverablePayload(round.ArtifactSummary, contract)
	return FormatDeliverablePayloadForIM(payload)
}
