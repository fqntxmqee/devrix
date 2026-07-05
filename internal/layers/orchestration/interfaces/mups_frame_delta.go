// Package interfaces — mups_frame_delta.go
//
// DM-20260705-010 (devrix-d7-mups-frame-delta-closure): MUPS 5 节点
// Observe→Plan→Execute LLM 帧 delta 显式契约。
//
// FrameDelta 是 D7 跨 S kernel 类型（与 TaskSpec / TaskReport / ResolutionStrategy
// 同层），0 import D7 子包（layout guard 守护）。
//
// 5 字段契约：
//   - PriorArtifactSummary (≤80 字符，人读摘要)
//   - KnownGaps           (machine-readable gap ID 数组，每项 ≤60 字符)
//   - ExecutionMode       (decompose / protocol / scenario / exploration)
//   - ChildSpecs          (plan child refs，NEW typed contract — 见 ChildSpecRef doc)
//   - DeliverableContract (期望产出 schema 摘要)
//
// 不变性：纯值对象，所有 method 走值接收器；零值 FrameDelta{} 是合法默认。
//
// v1.1 演进预留：TraceID 字段占位（json:"-" 暂不序列化），待 LP-1 反向追溯链
// 接入时启用。
package interfaces

import (
	"encoding/json"
	"hash/fnv"
)

// 注入 / FrameDelta 字段上限常量（单源 SoT — 所有判定走 const，避免 magic
// number 散落）。codex S3-Gate review H3 修复：阈值必须集中化。
const (
	// MaxPriorArtifactSummaryChars ≤80 字符 (人读摘要)。
	MaxPriorArtifactSummaryChars = 80

	// MaxKnownGapsItemChars ≤60 字符 / 项 (gap ID 或短字符串)。
	MaxKnownGapsItemChars = 60

	// MaxDeliverableContractChars ≤200 字符 (deliverable schema 摘要)。
	MaxDeliverableContractChars = 200

	// MaxPlanFrameDeltaInjectChars ≤200 字符 (Plan→Execute system_prompt
	// 注入绝对上限；不是 baseline-relative，超限降级走 baseline)。
	MaxPlanFrameDeltaInjectChars = 200

	// MaxChildSpecCount ≤5 项 (Plan ChildSpecs 数量上限)。
	MaxChildSpecCount = 5

	// MaxObserveFrameDeltaTotalChars ≤400 字符 (Observe→Plan user frame
	// 注入总量上限；summary + gaps 总长不超过此值，超限走 zero-value
	// fallback — cursor S3-Gate review M3 修复)。
	MaxObserveFrameDeltaTotalChars = 400
)

// FrameDelta is the machine-readable JSON delta passed between MUPS LLM nodes.
// All fields are snake_case JSON tagged. 5 fields: 2 from Observe→Plan side
// (PriorArtifactSummary + KnownGaps) + 3 from Plan→Execute side (ExecutionMode +
// ChildSpecs + DeliverableContract).
//
// v1.1 reserved TraceID field carries LP-1 reverse trace link (planned for
// devrix-d7-taskcontract-unification v1.1 follow-up); not serialized yet.
type FrameDelta struct {
	// PriorArtifactSummary is a ≤80-char human-readable summary of the
	// previous round's Execute convergence (built by BuildObservePriorDelta).
	PriorArtifactSummary string `json:"prior_artifact_summary,omitempty"`

	// KnownGaps is a machine-readable JSON array of gap IDs or short
	// strings (each ≤60 chars). Distinguishes "machine gap ID" entries
	// (e.g. "missing:ux_flow", "unresolved:a1b2c3") from prose text.
	KnownGaps []string `json:"known_gaps,omitempty"`

	// ExecutionMode is the Plan-decided channel/mode for Execute:
	// decompose / protocol / scenario / exploration (matches Strategy 抽象
	// DM-20260705-008 的 4 PlanKind).
	ExecutionMode string `json:"execution_mode,omitempty"`

	// ChildSpecs is the typed child ref list from Plan output. NEW typed
	// contract — distinct from the deprecated StrategicPlanFrame.ChildSpecs
	// carrier field (DM-20260704-006 Phase 5 deprecation). See ChildSpecRef
	// doc for the naming-collision clarification.
	ChildSpecs []ChildSpecRef `json:"child_specs,omitempty"`

	// DeliverableContract is the schema summary for expected output, ≤200
	// chars. Injected into Execute system_prompt alongside ChildSpecs to
	// constrain the LLM producer.
	DeliverableContract string `json:"deliverable_contract,omitempty"`

	// TraceID is reserved for v1.1 LP-1 reverse trace link. Not serialized
	// in v1.0 — the json:"-" tag ensures wire-format stability.
	// v1.1 will flip to json:"trace_id,omitempty" once AssetBuilder wires
	// SourceTraceID propagation (DM-20260705-010 §6.4 v1.1).
	TraceID string `json:"-"`
}

// ChildSpecRef is the typed child reference for FrameDelta.ChildSpecs.
//
// 命名冲突澄清：DM-20260704-006 Phase 5 deprecate 了 StrategicPlanFrame.ChildSpecs
// 字段（carrier 字段，Decide 实际不读 — 见 d7_child_specs_deprecation_test.go
// CI guard）。FrameDelta.ChildSpecs 是本 Change 新引入的字段，与 deprecated
// carrier **字段名同但语义不同**：本 type 是 FrameDelta 上的 NEW typed
// machine-readable contract，承载 Plan→Execute 子节点 ref。两个字段互不
// 影响，命名冲突已记录。
type ChildSpecRef struct {
	ID              string `json:"id"`
	DirectiveSuffix string `json:"directive_suffix,omitempty"`
}

// NewFrameDelta constructs an immutable FrameDelta. All fields optional.
// Default value (FrameDelta{}) is valid for first-round zero-value injection.
func NewFrameDelta() *FrameDelta {
	return &FrameDelta{}
}

// SchemaHash returns a stable FNV-1a 64-bit hash of the FrameDelta JSON
// representation. Used for machine-readable delta tracking (Jaeger span tag
// plan_frame_delta_schema_hash).
//
// Idempotency guarantee:
//   - Value receiver (H1 cursor review fix): safe on zero-value FrameDelta{},
//     no nil-pointer panic risk from BuildObservePriorDelta returning the
//     zero value.
//   - Field order in JSON is deterministic (struct field declaration order).
//   - Same field values → same hash across processes.
//
// v1.0 wire format: PriorArtifactSummary + KnownGaps + ExecutionMode +
// ChildSpecs + DeliverableContract. TraceID is excluded by json:"-".
func (f FrameDelta) SchemaHash() string {
	b, err := json.Marshal(f)
	if err != nil {
		// Marshal on a struct with omitempty + json:"-" fields cannot fail
		// in practice, but guard explicitly to keep SchemaHash total.
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return hash64ToHex(h.Sum64())
}

// TotalChars returns the total character length of the FrameDelta when
// injected as a JSON string. Used by BuildObservePriorDelta + inject budget
// guards to enforce the AC1/AC3 ≤200-char ceiling (cursor review M3 fix:
// bounds total injection length to prevent Observe prompt pollution).
func (f FrameDelta) TotalChars() int {
	b, err := json.Marshal(f)
	if err != nil {
		return 0
	}
	return len(b)
}

// IsZero reports whether the FrameDelta carries no signal (all fields empty).
// Used as a fast-path gate in BuildObservePriorDelta to skip injection when
// the previous round produced no useful deltas.
func (f FrameDelta) IsZero() bool {
	return f.PriorArtifactSummary == "" &&
		len(f.KnownGaps) == 0 &&
		f.ExecutionMode == "" &&
		len(f.ChildSpecs) == 0 &&
		f.DeliverableContract == "" &&
		f.TraceID == ""
}

// hash64ToHex formats a uint64 as a 16-char lowercase hex string.
// Stable, machine-readable, no encoding/binary tags.
func hash64ToHex(h uint64) string {
	const hex = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = hex[h&0xf]
		h >>= 4
	}
	return string(buf[:])
}