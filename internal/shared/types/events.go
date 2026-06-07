package types

import "time"

// DomainEventType represents the type of a domain event
type DomainEventType string

const (
	EventSessionCreated         DomainEventType = "session.created"
	EventSessionExpired        DomainEventType = "session.expired"
	EventMessageReceived       DomainEventType = "message.received"
	EventMessageSent          DomainEventType = "message.sent"
	EventPermissionRequested  DomainEventType = "permission.requested"
	EventPermissionResponded  DomainEventType = "permission.responded"
	EventPermissionExpired    DomainEventType = "permission.expired"
	EventToolCalled           DomainEventType = "tool.called"
	EventToolResult          DomainEventType = "tool.result"
	EventConnectionLost      DomainEventType = "connection.lost"
	EventConnectionRestored  DomainEventType = "connection.restored"
)

// DomainEvent represents a domain event in the system
type DomainEvent struct {
	Type      DomainEventType // 事件类型
	SessionID string          // 关联会话 ID
	Timestamp time.Time       // 事件时间
	Data      interface{}     // 事件数据
	Metadata  map[string]string // 额外元数据
}

// NewDomainEvent creates a new domain event
func NewDomainEvent(eventType DomainEventType, sessionID string, data interface{}) *DomainEvent {
	return &DomainEvent{
		Type:      eventType,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]string),
	}
}

// WithMetadata adds metadata to the event
func (e *DomainEvent) WithMetadata(key, value string) *DomainEvent {
	e.Metadata[key] = value
	return e
}

// EventConnectionLostData 连接断开事件数据
type EventConnectionLostData struct {
	ConnectionID string // 连接 ID
	AdapterID    string // 适配器 ID
	Reason       string // 断开原因
}

// EventConnectionRestoredData 连接恢复事件数据
type EventConnectionRestoredData struct {
	ConnectionID string // 连接 ID
	AdapterID    string // 适配器 ID
}

// EventPermissionRespondedData 权限响应事件数据
type EventPermissionRespondedData struct {
	RequestID    string    // 权限请求 ID
	SessionID    string    // 会话 ID
	Approved     bool      // 是否批准
	ResponseTime time.Time // 响应时间
}

// EventPermissionExpiredData 权限过期事件数据
type EventPermissionExpiredData struct {
	RequestID string    // 权限请求 ID
	SessionID string    // 会话 ID
	ExpiredAt time.Time // 过期时间
}

