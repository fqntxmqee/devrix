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
	PermissionStatusDenied  PermissionStatus = "denied"
	PermissionStatusExpired PermissionStatus = "expired"
)

// PermissionRequest represents a request for user permission to execute a tool
type PermissionRequest struct {
	ID           string           // 内部 ID (UUID)
	SessionID    string           // 所属会话
	ToolName     string           // 工具名称
	Description  string           // 工具描述
	InputPreview string           // 输入预览（截断）

	RiskLevel RiskLevel // LOW | MEDIUM | HIGH | CRITICAL

	// 生命周期
	CreatedAt   time.Time        // 创建时间
	ExpiresAt   time.Time        // 过期时间（创建 + 60s）
	Status      PermissionStatus  // pending | approved | denied | expired
	RespondedAt time.Time        // 响应时间
	Response    *bool            // 响应结果（nil=未响应）
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
		ID:           id,
		SessionID:    sessionID,
		ToolName:     toolName,
		RiskLevel:    riskLevel,
		Status:       PermissionStatusPending,
		CreatedAt:    now,
		ExpiresAt:    now.Add(timeout),
		RespondedAt:  time.Time{},
	}
}
