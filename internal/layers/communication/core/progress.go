package core

// ProgressEntryKind 进度条目类型
type ProgressEntryKind string

const (
	ProgressEntryThinking   ProgressEntryKind = "thinking"
	ProgressEntryToolUse   ProgressEntryKind = "tool_use"
	ProgressEntryToolResult ProgressEntryKind = "tool_result"
	ProgressEntryError     ProgressEntryKind = "error"
	ProgressEntryInfo      ProgressEntryKind = "info"
)

// ProgressEntry 进度条目
type ProgressEntry struct {
	Kind     ProgressEntryKind
	Text     string
	Tool     string // 工具名称
	Status   string
	ExitCode *int
	Success  *bool
}

// ToolStepKind 工具步骤类型
type ToolStepKind string

const (
	ToolStepKindTool     ToolStepKind = "tool"
	ToolStepKindThinking ToolStepKind = "thinking"
)

// ToolStep 是显示在富进度卡片中的一个步骤
type ToolStep struct {
	Kind    ToolStepKind // 步骤类型
	Name    string       // 工具名称（如 "Bash", "Read"）
	Summary string       // 显示的摘要
	Result  string       // 工具输出/结果摘要
	Status  string       // 状态（如 "completed"/"failed"）
	Done    bool         // 是否完成
}

// CardStatus 卡片状态
type CardStatus string

const (
	CardStatusThinking   CardStatus = "thinking"
	CardStatusRunning   CardStatus = "running"
	CardStatusDone     CardStatus = "done"
	CardStatusFailed   CardStatus = "failed"
)
