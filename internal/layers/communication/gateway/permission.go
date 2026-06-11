package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/metrics"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionManager handles permission request lifecycle
type PermissionManager struct {
	config   *config.PermissionConfig
	userCfg  *config.UserConfig
	mu       sync.RWMutex
	requests map[string]*types.PermissionRequest

	metricApproved metrics.Counter
	metricDenied   metrics.Counter
	metricTimeouts metrics.Counter
}

// NewPermissionManager creates a new PermissionManager
func NewPermissionManager(cfg *config.PermissionConfig) *PermissionManager {
	if cfg == nil {
		cfg = &config.PermissionConfig{
			DefaultTimeout: 60 * time.Second,
			MaxRetries:     3,
		}
	}
	return &PermissionManager{
		config:   cfg,
		userCfg:  config.DefaultUserConfig(),
		requests: make(map[string]*types.PermissionRequest),
	}
}

// SetObservability wires permission decision and timeout counters.
func (p *PermissionManager) SetObservability(obs *observability.Observability) {
	if obs == nil || obs.Meter() == nil {
		return
	}
	m := obs.Meter()
	p.metricApproved, _ = m.Int64Counter("permission_decisions_total",
		metrics.WithLabels(metrics.LabelMap{"decision": "approved"}))
	p.metricDenied, _ = m.Int64Counter("permission_decisions_total",
		metrics.WithLabels(metrics.LabelMap{"decision": "denied"}))
	p.metricTimeouts, _ = m.Int64Counter("permission_timeouts_total",
		metrics.WithLabels(metrics.LabelMap{}))
}

func (p *PermissionManager) recordDecision(approved bool) {
	if approved {
		if p.metricApproved != nil {
			p.metricApproved.Inc()
		}
		return
	}
	if p.metricDenied != nil {
		p.metricDenied.Inc()
	}
}

func (p *PermissionManager) recordTimeout() {
	if p.metricTimeouts != nil {
		p.metricTimeouts.Inc()
	}
}

// SetUserConfig sets the user configuration (for YOLO mode)
func (p *PermissionManager) SetUserConfig(userCfg *config.UserConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userCfg = userCfg
}

// Request creates a permission request and waits for user response
// The ctx is used for cancellation; if ctx is cancelled, request is denied
// In YOLO mode, requests are auto-approved based on configuration
func (p *PermissionManager) Request(ctx context.Context, sessionID, toolName, inputPreview string, riskLevel types.RiskLevel) bool {
	request := &types.PermissionRequest{
		ID:           generateRequestID(),
		SessionID:    sessionID,
		ToolName:     toolName,
		InputPreview: truncatePreview(inputPreview),
		RiskLevel:    riskLevel,
		Status:       types.PermissionStatusPending,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(p.config.DefaultTimeout),
	}

	p.mu.Lock()
	p.requests[request.ID] = request
	userCfg := p.userCfg
	p.mu.Unlock()

	// Check YOLO mode for auto-approve
	if userCfg != nil && userCfg.IsYOLOMode() {
		approved := p.shouldAutoApprove(toolName, riskLevel)
		if approved {
			p.resolveRequest(request, true)
			p.recordDecision(true)
			return true
		}
	}

	// Wait for response with timeout
	timeout := time.After(p.config.DefaultTimeout)

	select {
	case <-ctx.Done():
		p.resolveRequest(request, false)
		p.recordDecision(false)
		return false
	case <-timeout:
		p.resolveRequest(request, false)
		p.recordTimeout()
		p.recordDecision(false)
		return false
	}
}

// IsYOLOMode reports whether YOLO mode is enabled.
func (p *PermissionManager) IsYOLOMode() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.userCfg != nil && p.userCfg.YOLO.Enabled
}

// AutoApproveFiles reports whether YOLO allows workspace file writes without plan-mode restrictions.
func (p *PermissionManager) AutoApproveFiles() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.userCfg == nil {
		return false
	}
	return p.userCfg.ShouldAutoApproveFile()
}

// shouldAutoApprove checks if a tool should be auto-approved in YOLO mode
func (p *PermissionManager) shouldAutoApprove(toolName string, riskLevel types.RiskLevel) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.userCfg == nil || !p.userCfg.YOLO.Enabled {
		return false
	}

	// YOLO mode always approves for LOW and MEDIUM risk
	// HIGH and CRITICAL risk require specific auto-approve flags
	switch riskLevel {
	case types.RiskLevelLow, types.RiskLevelMedium:
		return true
	case types.RiskLevelHigh:
		return p.userCfg.YOLO.AutoApproveTools
	case types.RiskLevelCritical:
		// Critical risks never auto-approve in YOLO mode unless explicitly trusted
		return false
	}

	return false
}

// resolveRequest updates the request status (caller must hold mutex)
func (p *PermissionManager) resolveRequest(request *types.PermissionRequest, approved bool) {
	request.RespondedAt = time.Now()
	if approved {
		request.Status = types.PermissionStatusApproved
	} else {
		request.Status = types.PermissionStatusDenied
	}
}

// Resolve approves or denies a pending permission request
func (p *PermissionManager) Resolve(requestID string, approved bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	req, ok := p.requests[requestID]
	if !ok {
		return fmt.Errorf("permission request not found: %s", requestID)
	}

	if req.Status != types.PermissionStatusPending {
		return fmt.Errorf("permission request is not pending: %s", requestID)
	}

	req.Resolve(approved)
	return nil
}

// GetRequest returns a permission request by ID
func (p *PermissionManager) GetRequest(requestID string) (*types.PermissionRequest, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	req, ok := p.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("permission request not found: %s", requestID)
	}

	return req, nil
}

// ListPending returns all pending permission requests
func (p *PermissionManager) ListPending() []*types.PermissionRequest {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var pending []*types.PermissionRequest
	for _, req := range p.requests {
		if req.IsPending() {
			pending = append(pending, req)
		}
	}

	return pending
}

// CleanupExpired removes expired permission requests
func (p *PermissionManager) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, req := range p.requests {
		if req.IsExpired() {
			req.Expire()
			delete(p.requests, id)
		}
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%10000)
}

// truncatePreview truncates the input preview to a reasonable length
func truncatePreview(input string) string {
	const maxLen = 200
	if len(input) <= maxLen {
		return input
	}
	return input[:maxLen] + "..."
}
