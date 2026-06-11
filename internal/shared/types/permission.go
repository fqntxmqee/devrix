package types

import "time"

// RiskLevel represents the risk level of a tool execution
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// PermissionStatus represents the status of a permission request
type PermissionStatus string

const (
	PermissionStatusPending  PermissionStatus = "pending"
	PermissionStatusApproved PermissionStatus = "approved"
	PermissionStatusDenied   PermissionStatus = "denied"
	PermissionStatusExpired  PermissionStatus = "expired"
)

// PermissionRequest represents a request for user permission to execute a tool
type PermissionRequest struct {
	ID           string
	SessionID    string
	ToolName     string
	Description  string
	InputPreview string
	RiskLevel    RiskLevel
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Status       PermissionStatus
	RespondedAt  time.Time
	Response     *bool
}

// IsExpired returns true if the permission request has expired
func (p *PermissionRequest) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsPending returns true if the permission request is still pending
func (p *PermissionRequest) IsPending() bool {
	return p.Status == PermissionStatusPending && !p.IsExpired()
}

// Resolve approves or denies the permission request
func (p *PermissionRequest) Resolve(allowed bool) {
	p.Response = &allowed
	if allowed {
		p.Status = PermissionStatusApproved
	} else {
		p.Status = PermissionStatusDenied
	}
	p.RespondedAt = time.Now()
}

// Expire marks the permission request as expired
func (p *PermissionRequest) Expire() {
	p.Status = PermissionStatusExpired
	p.RespondedAt = time.Now()
}

// NewPermissionRequest creates a new permission request with default timeout
func NewPermissionRequest(id, sessionID, toolName string, riskLevel RiskLevel, timeout time.Duration) *PermissionRequest {
	now := time.Now()
	return &PermissionRequest{
		ID:          id,
		SessionID:   sessionID,
		ToolName:    toolName,
		RiskLevel:   riskLevel,
		Status:      PermissionStatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(timeout),
		RespondedAt: time.Time{},
	}
}

// PermissionMode controls tool visibility and approval semantics (Claude Code aligned).
type PermissionMode string

const (
	PermissionDefault     PermissionMode = "default"
	PermissionPlan        PermissionMode = "plan"
	PermissionAcceptEdits PermissionMode = "accept_edits"
	PermissionBypass      PermissionMode = "bypass"
	PermissionBubble      PermissionMode = "bubble"
)

// DefaultPermissionMode is the session default when unset.
func DefaultPermissionMode() PermissionMode {
	return PermissionDefault
}

// IsPlanMode reports whether the session is in read-only plan mode.
func (m PermissionMode) IsPlanMode() bool {
	return m == PermissionPlan
}
