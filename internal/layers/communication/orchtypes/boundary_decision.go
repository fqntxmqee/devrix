package orchtypes

const (
	BoundaryD1ToD7OrchestrationEntry      = "boundary-debt:d1-to-d7-orchestration-entry-v1.0"
	BoundaryD1ToD4PermissionGate          = "boundary-debt:d1-to-d4-permission-gate-v1.0"
	BoundaryD1ForbiddenOrchestrationImport = "boundary-debt:d1-forbidden-orchestration-import-v2.0"
)

func AllBoundaryDecisions() [3]string {
	return [3]string{
		BoundaryD1ToD7OrchestrationEntry,
		BoundaryD1ToD4PermissionGate,
		BoundaryD1ForbiddenOrchestrationImport,
	}
}