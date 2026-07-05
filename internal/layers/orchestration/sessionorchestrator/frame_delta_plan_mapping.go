// Package sessionorchestrator — frame_delta_plan_mapping.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure) Phase 1 T2 helper:
// map StrategicPlanProposal → interfaces.FrameDelta (Plan→Execute delta).
//
// StrategicPlanProposal (LLM 解析后的强类型) 与 FrameDelta (machine-readable
// LLM 帧 delta) 是两层：Proposal 是 domain payload；FrameDelta 是注入 Execute
// system_prompt 的 wire-format 字段集。本文件做纯 transform，0 LLM 调用，
// 0 副作用，符合 idempotency 承诺。
//
// 边界约束（单源 SoT = interfaces.Mux* const，codex H3 修复）：
//   - ChildSpecs 数量 ≤ MaxChildSpecCount (5)
//   - 每个 ChildSpecRef.ID ≤ MaxKnownGapsItemChars (60) — 来自 Title 截断
//   - 每个 ChildSpecRef.DirectiveSuffix ≤ MaxKnownGapsItemChars (60) — 来自 Directive 截断
//   - DeliverableContract ≤ MaxDeliverableContractChars (200)
//
// 命名冲突澄清（与 DM-20260704-006 deprecate 的 StrategicPlanFrame.ChildSpecs
// 不互通 — 见 interfaces/mups_frame_delta.go ChildSpecRef doc）：
// FrameDelta.ChildSpecs 是本 Change 新引入的 machine-readable 列表，承载
// 从 workmodel.ChildSpec（domain）转换来的 ref 列表。
package sessionorchestrator

import (
	"encoding/json"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/interfaces"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
)

// FrameDeltaFromPlanProposal constructs a FrameDelta from a validated
// StrategicPlanProposal. Pure function; same input → same output (idempotency).
//
// Bounds applied (in order, codex H3 + cursor M3' constraints):
//  1. ChildSpecs truncated to first MaxChildSpecCount entries (drop overflow).
//  2. Each ChildSpecRef ID/DirectiveSuffix truncated to MaxKnownGapsItemChars.
//  3. DeliverableContract JSON-encoded then truncated to MaxDeliverableContractChars.
//
// Returns zero-value FrameDelta{} if prop is nil — the fallback is intentional
// so callers can safely pass through a nil proposal without nil-checks.
func FrameDeltaFromPlanProposal(prop *StrategicPlanProposal) interfaces.FrameDelta {
	if prop == nil {
		return interfaces.FrameDelta{}
	}
	return interfaces.FrameDelta{
		ExecutionMode:       strings.TrimSpace(prop.ExecutionMode),
		ChildSpecs:          mapWorkmodelChildSpecsToFrameDelta(prop.ChildSpecs),
		DeliverableContract: deriveDeliverableContractForFrameDelta(prop.DeliverableContract),
	}
}

// buildPlanFrameDeltaForExecCtx is the WorkItemExecContext binder for the
// Plan→Execute FrameDelta (DM-20260705-010 Phase 1 T4). Returns:
//   - nil when prop is nil OR when the constructed FrameDelta is zero-value
//     (no signal to inject) → Execute's legacy baseline path + emit
//     PlanFrameDeltaInjectEmpty span via InjectPlanFrameDelta no-op branch.
//   - non-nil *interfaces.FrameDelta when there is at least one bindable
//     signal (ExecutionMode / ChildSpecs / DeliverableContract).
//
// The pointer indirection lets the WorkItemExecContext zero-value path stay
// backward-compatible: consumers (subturn_materialize.go, future trace
// consumers) can `if ec.PlanFrameDelta != nil` cheaply.
func buildPlanFrameDeltaForExecCtx(prop *StrategicPlanProposal) *interfaces.FrameDelta {
	if prop == nil {
		return nil
	}
	fd := FrameDeltaFromPlanProposal(prop)
	if fd.IsZero() {
		return nil
	}
	return &fd
}

// mapWorkmodelChildSpecsToFrameDelta converts domain ChildSpec list to
// machine-readable ChildSpecRef list, bounded by MaxChildSpecCount. Each
// ref is truncated per MaxKnownGapsItemChars.
//
// nil input → nil output (preserves "no children planned" semantics).
func mapWorkmodelChildSpecsToFrameDelta(specs []workmodel.ChildSpec) []interfaces.ChildSpecRef {
	if len(specs) == 0 {
		return nil
	}
	bounded := specs
	if len(bounded) > interfaces.MaxChildSpecCount {
		bounded = bounded[:interfaces.MaxChildSpecCount]
	}
	out := make([]interfaces.ChildSpecRef, 0, len(bounded))
	for _, s := range bounded {
		out = append(out, interfaces.ChildSpecRef{
			ID:              truncateForFrameDelta(s.Title, interfaces.MaxKnownGapsItemChars),
			DirectiveSuffix: truncateForFrameDelta(s.Directive, interfaces.MaxKnownGapsItemChars),
		})
	}
	return out
}

// deriveDeliverableContractForFrameDelta serializes DeliverableContract to
// a compact one-line JSON, then truncates to MaxDeliverableContractChars.
// Returns "" if contract is zero-value (preserves FrameDelta.IsZero()).
func deriveDeliverableContractForFrameDelta(contract workmodel.DeliverableContract) string {
	if isEmptyDeliverableContract(contract) {
		return ""
	}
	b, err := json.Marshal(contract)
	if err != nil {
		return ""
	}
	return truncateForFrameDelta(string(b), interfaces.MaxDeliverableContractChars)
}

// isEmptyDeliverableContract reports whether the contract carries no
// machine-readable signal. Implemented as a defensive helper since
// DeliverableContract has many optional fields.
func isEmptyDeliverableContract(c workmodel.DeliverableContract) bool {
	return c.Citation == "" &&
		c.Severity == "" &&
		c.Structure == "" &&
		c.MinRunes == 0 &&
		len(c.Reject) == 0
}

// truncateForFrameDelta returns s clipped to at most max bytes, appending
// an ellipsis marker ("...") when truncated so downstream readers can see
// the budget was hit. ASCII-safe (3 bytes for "...").
func truncateForFrameDelta(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}