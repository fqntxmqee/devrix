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
