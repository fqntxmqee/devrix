package types

import "time"

// BootstrapStage identifies a harness bootstrap phase.
type BootstrapStage string

const (
	BootstrapStagePrefetch     BootstrapStage = "prefetch"
	BootstrapStageGuards       BootstrapStage = "guards"
	BootstrapStageSetup          BootstrapStage = "setup"
	BootstrapStageDeferredInit   BootstrapStage = "deferred_init"
	BootstrapStageToolPool       BootstrapStage = "tool_pool"
)

// WorkspaceContext summarizes scanned workspace metadata.
type WorkspaceContext struct {
	WorkDir         string    `json:"workDir"`
	SourceRoots     []string  `json:"sourceRoots,omitempty"`
	GoFileCount     int       `json:"goFileCount"`
	TestFileCount   int       `json:"testFileCount"`
	AgentsMDPresent bool      `json:"agentsMdPresent"`
	ScannedAt       time.Time `json:"scannedAt"`
}

// DeferredInitResult records trust-gated deferred init flags (V5a stub).
type DeferredInitResult struct {
	PluginInit   bool `json:"pluginInit"`
	SkillInit    bool `json:"skillInit"`
	MCPPrefetch  bool `json:"mcpPrefetch"`
	SessionHooks bool `json:"sessionHooks"`
}

// VisibleTool describes a tool exposed to the LLM after ToolPool filtering.
type VisibleTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  string `json:"parameters,omitempty"`
}

// BootstrapReport summarizes a harness bootstrap run.
type BootstrapReport struct {
	StagesApplied []BootstrapStage  `json:"stagesApplied"`
	Workspace     WorkspaceContext  `json:"workspace"`
	ToolCount     int               `json:"toolCount"`
	VisibleTools  int               `json:"visibleTools"`
	VisibleToolList []VisibleTool   `json:"visibleToolList,omitempty"`
	Trusted       bool              `json:"trusted"`
	DeferredInit  DeferredInitResult `json:"deferredInit"`
	Duration      time.Duration     `json:"duration"`
}

// HarnessSessionState holds per-session harness runtime state.
type HarnessSessionState struct {
	Initialized bool            `json:"initialized"`
	Report      BootstrapReport `json:"report"`
}

// RoutingHint holds advisory routing matches for system prompt injection.
type RoutingHint struct {
	Commands []string       `json:"commands,omitempty"`
	Tools    []string       `json:"tools,omitempty"`
	Scores   map[string]int `json:"scores,omitempty"`
}

// PreflightScores holds rule-based preflight dimension scores.
type PreflightScores struct {
	Relevance    int `json:"relevance"`
	Completeness int `json:"completeness"`
	Safety       int `json:"safety"`
	TokenBudget  int `json:"tokenBudget"`
}

// ToolFilterDecision records preflight tool filtering outcome.
type ToolFilterDecision struct {
	Applied      bool     `json:"applied"`
	RemovedTools []string `json:"removedTools,omitempty"`
	KeptTools    []string `json:"keptTools,omitempty"`
}

// PreflightResult holds preflight evaluation output.
type PreflightResult struct {
	Scores     PreflightScores    `json:"scores"`
	Warnings   []string           `json:"warnings,omitempty"`
	ToolFilter ToolFilterDecision `json:"toolFilter"`
	Mode       string             `json:"mode"`
}

// TranscriptEntry is a single transcript turn.
type TranscriptEntry struct {
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
}

// TranscriptStore holds in-memory transcript entries (optional V4 compat).
type TranscriptStore struct {
	Entries []TranscriptEntry `json:"entries"`
	Flushed bool              `json:"flushed"`
}

// Append adds a transcript entry.
func (t *TranscriptStore) Append(role MessageRole, content string) {
	if t == nil {
		return
	}
	t.Entries = append(t.Entries, TranscriptEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// Compact retains only the last keepLast entries.
func (t *TranscriptStore) Compact(keepLast int) {
	if t == nil || keepLast <= 0 {
		return
	}
	if len(t.Entries) <= keepLast {
		return
	}
	t.Entries = append([]TranscriptEntry(nil), t.Entries[len(t.Entries)-keepLast:]...)
}

// Replay returns a copy of transcript entries.
func (t *TranscriptStore) Replay() []TranscriptEntry {
	if t == nil {
		return nil
	}
	out := make([]TranscriptEntry, len(t.Entries))
	copy(out, t.Entries)
	return out
}
