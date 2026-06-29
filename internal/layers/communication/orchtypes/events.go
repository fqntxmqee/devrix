package orchtypes

const (
	EventOrchestrationEntryProcess = "orchestration.entry.process"
	EventPermissionRequired        = "permission.required"
	EventOrchestrationOrphanEvent  = "orchestration.orphan_event"
)

func AllEvents() [3]string {
	return [3]string{
		EventOrchestrationEntryProcess,
		EventPermissionRequired,
		EventOrchestrationOrphanEvent,
	}
}