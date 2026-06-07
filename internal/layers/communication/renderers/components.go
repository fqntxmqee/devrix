package renderers

import (
	"fmt"

	"github.com/devrix/devrix/internal/shared/types"
)

// Component represents a UI component
type Component interface {
	// Render renders the component to a string
	Render() string
}

// MilestoneCard renders a milestone as a card
type MilestoneCard struct {
	Milestone *types.Milestone
	ShowProgress bool
}

// NewMilestoneCard creates a new milestone card
func NewMilestoneCard(m *types.Milestone) *MilestoneCard {
	return &MilestoneCard{
		Milestone:    m,
		ShowProgress: true,
	}
}

// Render renders the milestone card
func (c *MilestoneCard) Render() string {
	m := c.Milestone
	progress := int(m.Progress * 100)

	var statusIcon string
	switch m.Status {
	case types.MilestoneStatusPending:
		statusIcon = "[ ]"
	case types.MilestoneStatusInProgress:
		statusIcon = "[~]"
	case types.MilestoneStatusCompleted:
		statusIcon = "[✓]"
	case types.MilestoneStatusFailed:
		statusIcon = "[✗]"
	default:
		statusIcon = "[?]"
	}

	if c.ShowProgress {
		return fmt.Sprintf("%s Milestone: %s (%d%%)", statusIcon, m.Name, progress)
	}
	return fmt.Sprintf("%s Milestone: %s", statusIcon, m.Name)
}

// ProgressBar renders a progress bar
type ProgressBar struct {
	Progress float64
	Width    int
	ShowText bool
}

// NewProgressBar creates a new progress bar
func NewProgressBar(progress float64, width int) *ProgressBar {
	if width <= 0 {
		width = 20
	}
	return &ProgressBar{
		Progress: progress,
		Width:    width,
		ShowText: true,
	}
}

// Render renders the progress bar
func (p *ProgressBar) Render() string {
	progress := p.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(progress * float64(p.Width))
	empty := p.Width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	percent := int(progress * 100)
	if p.ShowText {
		return fmt.Sprintf("[%s] %d%%", bar, percent)
	}
	return fmt.Sprintf("[%s]", bar)
}

// StatusBadge renders a status badge
type StatusBadge struct {
	Status string
}

// NewStatusBadge creates a new status badge
func NewStatusBadge(status string) *StatusBadge {
	return &StatusBadge{Status: status}
}

// Render renders the status badge
func (b *StatusBadge) Render() string {
	var color string
	var icon string

	switch b.Status {
	case "pending":
		color = "\x1b[33m" // yellow
		icon = "○"
	case "in_progress":
		color = "\x1b[34m" // blue
		icon = "◐"
	case "completed":
		color = "\x1b[32m" // green
		icon = "●"
	case "failed":
		color = "\x1b[31m" // red
		icon = "✗"
	default:
		color = "\x1b[0m"
		icon = "?"
	}

	return fmt.Sprintf("%s%s%s", color, icon, "\x1b[0m")
}

// TaskFlowSummary renders a task flow summary
type TaskFlowSummary struct {
	TaskFlow *types.TaskFlow
}

// NewTaskFlowSummary creates a new task flow summary
func NewTaskFlowSummary(tf *types.TaskFlow) *TaskFlowSummary {
	return &TaskFlowSummary{TaskFlow: tf}
}

// Render renders the task flow summary
func (s *TaskFlowSummary) Render() string {
	tf := s.TaskFlow

	progressBar := NewProgressBar(tf.OverallProgress, 30)
	barStr := progressBar.Render()

	var statusStr string
	switch tf.Status {
	case types.TaskFlowStatusPending:
		statusStr = "PENDING"
	case types.TaskFlowStatusRunning:
		statusStr = "RUNNING"
	case types.TaskFlowStatusCompleted:
		statusStr = "COMPLETED"
	case types.TaskFlowStatusFailed:
		statusStr = "FAILED"
	default:
		statusStr = "UNKNOWN"
	}

	summary := tf.GetStatusSummary()

	return fmt.Sprintf("TaskFlow: %s\nProgress: %s\nStatus: %s\n%s",
		tf.Name, barStr, statusStr, summary)
}

// PermissionCard renders a permission request as a card
type PermissionCard struct {
	Request *types.PermissionRequest
}

// NewPermissionCard creates a new permission card
func NewPermissionCard(req *types.PermissionRequest) *PermissionCard {
	return &PermissionCard{Request: req}
}

// Render renders the permission card
func (c *PermissionCard) Render() string {
	req := c.Request

	riskColor := "\x1b[33m" // yellow
	switch req.RiskLevel {
	case types.RiskLevelHigh:
		riskColor = "\x1b[31m" // red
	case types.RiskLevelCritical:
		riskColor = "\x1b[35m" // magenta
	case types.RiskLevelLow:
		riskColor = "\x1b[32m" // green
	}

	return fmt.Sprintf(`Permission Request
═══════════════════════════════
Tool: %s
Risk: %s%s%s
───────────────────────────────
%s

Approve? (yes/no)`,
		req.ToolName,
		riskColor, req.RiskLevel, "\x1b[0m",
		truncate(req.InputPreview, 100),
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
