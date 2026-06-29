package external

import (
	"sync"
	"time"
)

// CursorConfig holds configuration for a Cursor Agent tool.
type CursorConfig struct {
	Name         string
	DisplayName  string
	Description  string
	Capabilities []string
	Role         string // LLM role description for tool decision
	Command      string   // CLI binary name, default "cursor"
	Args         []string // optional extra args (for testing with bash etc.)
	Model        string
	Mode         string // "force" | "plan" | "ask" | "default"
	WorkDir      string
	Timeout      time.Duration
}

// CursorAgentTool implements AgentTool for Cursor Agent CLI using
// one-shot processes with --resume for multi-turn conversations.
type CursorAgentTool struct {
	cfg     CursorConfig
	info    Info
	chatIDs map[string]string // sessionID → cursor chatID for --resume
	mu      sync.RWMutex
}

// NewCursorAgentTool creates a Cursor Agent tool.
func NewCursorAgentTool(cfg CursorConfig) *CursorAgentTool {
	if cfg.Command == "" {
		cfg.Command = "cursor"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	info := Info{
		Name:         cfg.Name,
		DisplayName:  cfg.DisplayName,
		Description:  cfg.Description,
		Capabilities: cfg.Capabilities,
		Role:         cfg.Role,
	}
	return &CursorAgentTool{
		cfg:     cfg,
		info:    info,
		chatIDs: make(map[string]string),
	}
}

// Info returns the tool's identity metadata.
func (t *CursorAgentTool) Info() Info { return t.info }

// ExecutionTimeout returns the configured per-call timeout for this agent tool.
func (t *CursorAgentTool) ExecutionTimeout() time.Duration {
	return t.cfg.Timeout
}

// Stop cleans up all tracked sessions.
func (t *CursorAgentTool) Stop() {
	t.mu.Lock()
	t.chatIDs = make(map[string]string)
	t.mu.Unlock()
}

// CloseSession forgets the cursor chatID for the given session.
func (t *CursorAgentTool) CloseSession(sessionID string) {
	t.mu.Lock()
	delete(t.chatIDs, sessionID)
	t.mu.Unlock()
}

// CleanupBySessionID removes all sessions for the given D1 Session ID.
func (t *CursorAgentTool) CleanupBySessionID(sessionID string) {
	t.CloseSession(sessionID)
}

// lookupChatID returns the cached cursor chatID for resumption under the read lock.
func (t *CursorAgentTool) lookupChatID(sessionID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.chatIDs[sessionID]
}

// storeChatID records the cursor chatID returned in a "system" event for
// subsequent --resume calls under the write lock.
func (t *CursorAgentTool) storeChatID(sessionID, chatID string) {
	t.mu.Lock()
	t.chatIDs[sessionID] = chatID
	t.mu.Unlock()
}
