package adapters

import (
	"strconv"
	"strings"

	"github.com/devrix/devrix/internal/layers/communication/renderers"
	"github.com/devrix/devrix/internal/shared/types"
)

func renderDingTalkOutboundContent(msg *types.OutboundMessage) string {
	if msg == nil {
		return ""
	}

	render := msg.Metadata["render"]
	if render == "" && msg.Metadata["event_type"] == "milestone_progress" {
		render = "milestone"
	}

	switch render {
	case "milestone":
		if m := milestoneFromOutboundMetadata(msg); m != nil {
			return renderers.NewDingTalkCardRenderer().RenderMilestone(m)
		}
	case "taskflow":
		// TaskFlow snapshots can be wired when gateway emits taskflow metadata.
	}

	return msg.Content
}

func milestoneFromOutboundMetadata(msg *types.OutboundMessage) *types.Milestone {
	name := msg.Metadata["milestone_name"]
	if name == "" {
		name = msg.Metadata["task"]
	}
	if name == "" {
		name = strings.TrimSpace(msg.Content)
	}
	if name == "" {
		return nil
	}

	id := msg.Metadata["milestone_id"]
	if id == "" {
		id = "milestone"
	}
	taskID := msg.Metadata["milestone_task_id"]
	if taskID == "" {
		taskID = "task"
	}

	m := types.NewMilestone(id, taskID, name)
	status := msg.Metadata["milestone_status"]
	if status == "" {
		status = string(types.MilestoneStatusInProgress)
	}
	m.SetStatus(types.MilestoneStatus(status))

	progress := parseMetadataProgress(msg.Metadata["milestone_progress"])
	if progress == 0 {
		progress = parseMetadataProgress(msg.Metadata["progress"])
	}
	m.SetProgress(progress)
	return m
}

func parseMetadataProgress(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	raw = strings.TrimSuffix(raw, "%")
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		if v > 1 {
			return v / 100
		}
		return v
	}
	return 0
}
