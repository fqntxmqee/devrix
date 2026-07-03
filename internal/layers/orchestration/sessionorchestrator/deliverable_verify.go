package sessionorchestrator

import (
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// DeliverableVerifyResult is the output of VerifyDeliverableContract.
type DeliverableVerifyResult struct {
	Status  workmodel.DeliverableStatus
	Payload *workmodel.DeliverablePayload
	Reason  string
}

// VerifyDeliverableContract checks artifact summary against dimension-composed contract.
func VerifyDeliverableContract(
	contract workmodel.DeliverableContract,
	art *wavescheduler.Artifact,
) DeliverableVerifyResult {
	if !contract.ContractApplicable() {
		return DeliverableVerifyResult{Status: workmodel.DeliverableStatusNotApplicable}
	}
	if art == nil {
		return DeliverableVerifyResult{
			Status: workmodel.DeliverableStatusIncomplete,
			Reason: "missing artifact",
		}
	}
	summary := strings.TrimSpace(art.Summary)
	stopReason, _ := art.Metadata["stop_reason"].(string)
	got := workmodel.VerifyDeliverableContract(contract, summary, stopReason)
	return DeliverableVerifyResult{
		Status:  got.Status,
		Payload: got.Payload,
		Reason:  got.Reason,
	}
}

// VerifyDeliverable is a legacy wrapper over schema → contract expansion.
func VerifyDeliverable(
	schema workmodel.DeliverableSchema,
	art *wavescheduler.Artifact,
) DeliverableVerifyResult {
	return VerifyDeliverableContract(workmodel.ExpandLegacySchemaToContract(schema), art)
}
