package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// PermissionManager handles permission request lifecycle
type PermissionManager struct {
	config   *config.PermissionConfig
	mu       sync.RWMutex
	requests map[string]*types.PermissionRequest
}

// NewPermissionManager creates a new PermissionManager
func NewPermissionManager(cfg *config.PermissionConfig) *PermissionManager {
	return &PermissionManager{
		config:   cfg,
		requests: make(map[string]*types.PermissionRequest),
	}
}

// Request creates a permission request and waits for user response
// The ctx is used for cancellation; if ctx is cancelled, request is denied
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
	p.mu.Unlock()

	// Wait for response with timeout
	timeout := time.After(p.config.DefaultTimeout)

	select {
	case <-ctx.Done():
		p.resolveRequest(request, false)
		return false
	case <-timeout:
		p.resolveRequest(request, false)
		return false
	}
}

// resolveRequest updates the request status (caller must hold mutex)
func (p *PermissionManager) resolveRequest(request *types.PermissionRequest, approved bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
