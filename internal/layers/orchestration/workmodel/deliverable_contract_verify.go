package workmodel

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	fileLineCitationRE = regexp.MustCompile(`\w[\w./-]*\.(go|py|ts|tsx|js|rs):\d+`)
	p0p1SeverityRE     = regexp.MustCompile(`(?i)\bP[01]\b|"severity"\s*:\s*"P[01]"`)
)

type deliverableVerifyResult struct {
	Status  DeliverableStatus
	Reason  string
	Payload *DeliverablePayload
}

// VerifyDeliverableContract runs dimension-composed verification.
func VerifyDeliverableContract(contract DeliverableContract, summary, stopReason string) deliverableVerifyResult {
	if !contract.ContractApplicable() {
		return deliverableVerifyResult{Status: DeliverableStatusNotApplicable}
	}
	c := contract.Normalized()
	summary = strings.TrimSpace(summary)
	payload := parseDeliverablePayload(c, summary)

	for _, r := range c.Reject {
		if r != DeliverableRejectPlanningMeta {
			continue
		}
		planningScope := summary
		if c.Structure == DeliverableStructureFindingsJSON {
			if raw := extractDeliverableJSONObject(summary); raw != nil {
				planningScope = string(raw)
			}
		}
		if DetectPlanningMeta(planningScope) {
			return deliverableVerifyResult{
				Status:  DeliverableStatusIncomplete,
				Payload: payload,
				Reason:  "planning_meta",
			}
		}
	}
	if c.MinRunes > 0 && utf8.RuneCountInString(summary) < c.MinRunes {
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "min_runes",
		}
	}

	if c.Structure == DeliverableStructureFindingsJSON {
		if payload == nil || len(payload.Findings) == 0 {
			return deliverableVerifyResult{
				Status:  DeliverableStatusIncomplete,
				Payload: payload,
				Reason:  "findings_json_required",
			}
		}
		if contractFindingsComplete(c, payload) {
			return deliverableVerifyResult{Status: DeliverableStatusComplete, Payload: payload}
		}
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "findings_json_incomplete",
		}
	}

	hasCitation := c.Citation != DeliverableCitationFileLine || fileLineCitationRE.MatchString(summary)
	hasSeverity := c.Severity != DeliverableSeverityP0P1 || p0p1SeverityRE.MatchString(summary)

	if c.Citation == DeliverableCitationFileLine && c.Severity == DeliverableSeverityP0P1 {
		if hasCitation && hasSeverity {
			return deliverableVerifyResult{Status: DeliverableStatusComplete, Payload: payload}
		}
		if stopReason == "max_iters" && !hasCitation {
			return deliverableVerifyResult{
				Status:  DeliverableStatusIncomplete,
				Payload: payload,
				Reason:  "max_iters without file:line deliverable",
			}
		}
		if hasCitation || hasSeverity {
			return deliverableVerifyResult{
				Status:  DeliverableStatusIncomplete,
				Payload: payload,
				Reason:  "partial deliverable",
			}
		}
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "missing deliverable dimensions",
		}
	}

	if c.Citation == DeliverableCitationFileLine && !hasCitation {
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "missing file:line citation",
		}
	}
	if c.Severity == DeliverableSeverityP0P1 && !hasSeverity {
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "missing severity tags",
		}
	}
	if summary == "" {
		return deliverableVerifyResult{
			Status:  DeliverableStatusIncomplete,
			Payload: payload,
			Reason:  "empty deliverable",
		}
	}
	return deliverableVerifyResult{Status: DeliverableStatusComplete, Payload: payload}
}

func parseDeliverablePayload(contract DeliverableContract, summary string) *DeliverablePayload {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil
	}
	c := contract.Normalized()
	if c.Structure == DeliverableStructureFindingsJSON {
		if p, ok := parseFindingsJSONPayload(summary); ok {
			p.Schema = DeliverableSchema(contract.CacheKey())
			return p
		}
		return &DeliverablePayload{
			Schema: DeliverableSchema(contract.CacheKey()),
			Raw:    summary,
		}
	}
	start := strings.Index(summary, "{")
	end := strings.LastIndex(summary, "}")
	if start >= 0 && end > start {
		var payload DeliverablePayload
		if err := json.Unmarshal([]byte(summary[start:end+1]), &payload); err == nil && len(payload.Findings) > 0 {
			payload.Schema = DeliverableSchema(contract.CacheKey())
			return &payload
		}
	}
	var findings []DeliverableFinding
	for _, m := range fileLineCitationRE.FindAllString(summary, -1) {
		file, line := splitFileLineCitation(m)
		sev := "P1"
		if strings.Contains(strings.ToUpper(summary), "P0") {
			sev = "P0"
		}
		findings = append(findings, DeliverableFinding{
			Severity: sev,
			File:     file,
			Line:     line,
			Message:  strings.TrimSpace(summary),
		})
		if len(findings) >= 8 {
			break
		}
	}
	if len(findings) == 0 {
		return &DeliverablePayload{
			Schema: DeliverableSchema(contract.CacheKey()),
			Raw:    summary,
		}
	}
	return &DeliverablePayload{
		Schema:   DeliverableSchema(contract.CacheKey()),
		Findings: findings,
		Raw:      summary,
	}
}

func splitFileLineCitation(citation string) (file string, line int) {
	parts := strings.Split(citation, ":")
	if len(parts) < 2 {
		return citation, 0
	}
	file = strings.Join(parts[:len(parts)-1], ":")
	var n int
	for _, ch := range parts[len(parts)-1] {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return file, n
}

// AcceptanceCriteriaForContract returns i18n-ready machine summary of dimensions.
func AcceptanceCriteriaForContract(c DeliverableContract) string {
	if !c.ContractApplicable() {
		return ""
	}
	n := c.Normalized()
	var parts []string
	if n.Citation == DeliverableCitationFileLine {
		parts = append(parts, "citation=file_line")
	}
	if n.Severity == DeliverableSeverityP0P1 {
		parts = append(parts, "severity=p0_p1")
	}
	if n.MinRunes > 0 {
		parts = append(parts, "min_runes="+strconv.Itoa(n.MinRunes))
	}
	for _, r := range n.Reject {
		parts = append(parts, "reject="+string(r))
	}
	if n.Structure == DeliverableStructureFindingsJSON {
		parts = append(parts, "structure=findings_json")
	}
	return "Acceptance: " + strings.Join(parts, "; ")
}

func contractFindingsComplete(c DeliverableContract, payload *DeliverablePayload) bool {
	if payload == nil || len(payload.Findings) == 0 {
		return false
	}
	normalized := normalizeDeliverableFindings(payload.Findings)
	if len(normalized) == 0 {
		return false
	}
	payload.Findings = normalized
	for _, f := range normalized {
		if c.Citation == DeliverableCitationFileLine && strings.TrimSpace(f.File) == "" {
			return false
		}
		if c.Severity == DeliverableSeverityP0P1 {
			sev := strings.ToUpper(strings.TrimSpace(f.Severity))
			if sev != "P0" && sev != "P1" {
				return false
			}
		}
	}
	return true
}
