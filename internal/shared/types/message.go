package types

import "time"

// MessageRole represents the role of a message sender
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleTool      MessageRole = "tool"
)

// AttachmentType represents the type of an attachment
type AttachmentType string

const (
	AttachmentTypeFile  AttachmentType = "file"
	AttachmentTypeImage AttachmentType = "image"
	AttachmentTypeCode  AttachmentType = "code"
)

// Message represents a chat message (Entity)
type Message struct {
	ID          string            // 消息 ID
	SessionID   string            // 所属会话
	Role        MessageRole       // user | assistant | system
	Content     string            // 内容
	Attachments []Attachment      // 附件
	Metadata    map[string]string // 元数据
	Timestamp   time.Time         // 时间戳
}

// Attachment represents a file or resource attached to a message
type Attachment struct {
	Type    AttachmentType // file | image | code
	Name    string
	Path    string
	Content string
}

// NewMessage creates a new message with default values
func NewMessage(id, sessionID string, role MessageRole, content string) *Message {
	return &Message{
		ID:        id,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// InboundMessage represents a message from Adapter to Gateway
type InboundMessage struct {
	SessionID  string            // 会话 ID（空则创建新会话）
	ChatID     string            // IM 平台的 chat ID
	UserID     string            // 用户 ID
	UserName   string            // 用户名
	Content    string            // 消息内容
	MessageID  string            // 客户端消息 ID（用于去重）
	AdapterID  string            // 来源 Adapter
	ReceivedAt time.Time         // 接收时间
	Metadata   map[string]string // 额外元数据
}

// OutboundMessage represents a message from Gateway to Adapter
type OutboundMessage struct {
	MessageID  string            // 消息 ID
	SessionID  string            // 会话 ID
	ChatID     string            // IM 平台的 chat ID
	Content    string            // 消息内容
	IsComplete bool              // 是否为完整消息（用于流式）
	Role       MessageRole       // 发送者角色
	Metadata   map[string]string // 额外元数据
	SentAt     time.Time         // 发送时间
}
