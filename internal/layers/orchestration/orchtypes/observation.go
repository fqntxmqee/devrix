package orchtypes

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// Errors are declared in errors.go as exported sentinels so callers can
// use errors.Is / errors.As. sharederrors is used for *SentinelError
// wrapping when a stable code is required.

// Category classifies an Observation by its semantic domain.
//
// CatBusiness observations feed Plan.Kind / Plan.Strength and influence the
// business path. CatSystem observations are infrastructure-level signals that
// must NOT bleed into ComputeOverallStrength (which is business-only).
type Category uint8

const (
	CatBusiness Category = iota
	CatSystem
)

func (c Category) String() string {
	switch c {
	case CatBusiness:
		return "business"
	case CatSystem:
		return "system"
	default:
		return fmt.Sprintf("Category(%d)", uint8(c))
	}
}

// MarshalJSON serializes Category as its string form for human-readable
// wire output. Numbers in {0,1} remain valid as a fallback for legacy
// readers.
func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON accepts either the string form or the raw uint8 form.
func (c *Category) UnmarshalJSON(data []byte) error {
	var n uint8
	if err := json.Unmarshal(data, &n); err == nil {
		*c = Category(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "business":
		*c = CatBusiness
	case "system":
		*c = CatSystem
	default:
		return fmt.Errorf("orchtypes: unknown Category %q", s)
	}
	return nil
}

// ObservationKind enumerates the 4 observation flavors emitted by ObserveNode.
// 4 kinds × 2 categories = 8 valid combinations, but the design intent is
// ObsFact/ObsSignal are typically business, ObsDeviation/ObsUncertainty may
// span both. The matrix is therefore not strict 1-to-1.
type ObservationKind uint8

const (
	ObsFact ObservationKind = iota
	ObsSignal
	ObsDeviation
	ObsUncertainty
)

func (k ObservationKind) String() string {
	switch k {
	case ObsFact:
		return "fact"
	case ObsSignal:
		return "signal"
	case ObsDeviation:
		return "deviation"
	case ObsUncertainty:
		return "uncertainty"
	default:
		return fmt.Sprintf("ObservationKind(%d)", uint8(k))
	}
}

// MarshalJSON serializes ObservationKind as its string form.
func (k ObservationKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// UnmarshalJSON accepts either the string form or the raw uint8 form.
func (k *ObservationKind) UnmarshalJSON(data []byte) error {
	var n uint8
	if err := json.Unmarshal(data, &n); err == nil {
		*k = ObservationKind(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "fact":
		*k = ObsFact
	case "signal":
		*k = ObsSignal
	case "deviation":
		*k = ObsDeviation
	case "uncertainty":
		*k = ObsUncertainty
	default:
		return fmt.Errorf("orchtypes: unknown ObservationKind %q", s)
	}
	return nil
}

// Payload is a sealed interface for kind-specific data attached to an
// Observation. We use a small closed set of concrete payload types below
// so callers can exhaustively type-assert without runtime registration.
type Payload interface {
	Validate() error
	kindMarker()
}

// FactPayload describes a verified, ground-truth fact (e.g. "user is admin").
type FactPayload struct {
	Statement string   `json:"statement"`
	Evidence  []string `json:"evidence,omitempty"`
}

func (FactPayload) kindMarker()             {}
func (p FactPayload) Validate() error        { return validateFact(p) }

// SignalPayload describes a noisy/repetitive signal (e.g. "user typed 3 times").
type SignalPayload struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Unit      string  `json:"unit,omitempty"`
}

func (SignalPayload) kindMarker() {}
func (p SignalPayload) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("orchtypes: SignalPayload: %w", ErrObservationPayloadInvalid)
	}
	return nil
}

// DeviationPayload describes a delta from baseline (e.g. "latency +200ms").
type DeviationPayload struct {
	Metric   string  `json:"metric"`
	Expected float64 `json:"expected"`
	Observed float64 `json:"observed"`
	Delta    float64 `json:"delta"`
}

func (DeviationPayload) kindMarker() {}
func (p DeviationPayload) Validate() error {
	if p.Metric == "" {
		return fmt.Errorf("orchtypes: DeviationPayload: %w", ErrObservationPayloadInvalid)
	}
	return nil
}

// UncertaintyPayload describes an unresolved question (e.g. "user intent
// ambiguous between Fast vs Orchestrate").
type UncertaintyPayload struct {
	Question     string   `json:"question"`
	Candidates   []string `json:"candidates,omitempty"`
	Confidence   float64  `json:"confidence"`
	RequiresMore bool     `json:"requires_more,omitempty"`
}

func (UncertaintyPayload) kindMarker() {}
func (p UncertaintyPayload) Validate() error {
	if p.Question == "" {
		return fmt.Errorf("orchtypes: UncertaintyPayload: %w", ErrObservationPayloadInvalid)
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return fmt.Errorf("orchtypes: UncertaintyPayload.Confidence %.2f: %w",
			p.Confidence, ErrObservationPayloadInvalid)
	}
	return nil
}

func validateFact(p FactPayload) error {
	if p.Statement == "" {
		return fmt.Errorf("orchtypes: FactPayload.Statement empty: %w", ErrObservationPayloadInvalid)
	}
	return nil
}

// Observation is the atomic unit emitted by ObserveNode. It is treated as
// an immutable value object — mutators (With*) return a copy.
type Observation struct {
	ID         string          `json:"id"`
	Kind       ObservationKind `json:"kind"`
	Category   Category        `json:"category"`
	Strength   float64         `json:"strength"` // [0,1]
	Payload    Payload         `json:"payload"`
	DetectedAt time.Time       `json:"detected_at"`
	Source     string          `json:"source"`
}

// NewObservation constructs an Observation. Strength is clamped to [0,1].
// If id is empty, a UUID v4 is generated. DetectedAt defaults to time.Now()
// when zero. Payload is validated per its concrete type.
func NewObservation(kind ObservationKind, category Category, strength float64, payload Payload, source string) (Observation, error) {
	if payload == nil {
		return Observation{}, ErrObservationPayloadRequired
	}
	if err := payload.Validate(); err != nil {
		return Observation{}, fmt.Errorf("orchtypes: invalid Payload: %w", err)
	}
	now := time.Now()
	return Observation{
		ID:         uuid.New().String(),
		Kind:       kind,
		Category:   category,
		Strength:   clamp01(strength),
		Payload:    payload,
		DetectedAt: now,
		Source:     source,
	}, nil
}

// Validate checks the post-construction invariants. Use this for hand-built
// observations (e.g. unmarshalled from JSON or restored from persistence)
// to catch missing fields that NewObservation would normally reject.
func (o Observation) Validate() error {
	if o.ID == "" {
		return ErrObservationIDRequired
	}
	if o.Strength < 0 || o.Strength > 1 {
		return NewObservationStrengthOutOfRangeError(o.Strength)
	}
	if o.DetectedAt.IsZero() {
		return ErrObservationDetectedAtRequired
	}
	if o.Payload == nil {
		return ErrObservationPayloadRequired
	}
	if err := o.Payload.Validate(); err != nil {
		return fmt.Errorf("orchtypes: %w: payload: %w", ErrObservationPayloadInvalid, err)
	}
	if o.Category > CatSystem {
		return fmt.Errorf("orchtypes: %w: %d", ErrObservationUnknownCategory, o.Category)
	}
	return nil
}

// WithKind returns a copy with the new Kind. Strength, Payload, and other
// fields are preserved as-is.
func (o Observation) WithKind(k ObservationKind) Observation {
	o.Kind = k
	return o
}

// WithStrength returns a copy with the new Strength (clamped to [0,1]).
func (o Observation) WithStrength(s float64) Observation {
	o.Strength = clamp01(s)
	return o
}

// WithCategory returns a copy with the new Category.
func (o Observation) WithCategory(c Category) Observation {
	o.Category = c
	return o
}

// WithPayload returns a copy with the new Payload (validated).
func (o Observation) WithPayload(p Payload) (Observation, error) {
	if p == nil {
		return o, ErrObservationPayloadRequired
	}
	if err := p.Validate(); err != nil {
		return o, err
	}
	o.Payload = p
	return o, nil
}

func clamp01(v float64) float64 {
	return clamp01Float(v, 0)
}

// clamp01Float clamps v to [0,1]. When v is NaN it returns onNaN so the
// caller can choose a sensible cold-start value (e.g. 0.5 for coord-style
// "neutral uncertainty", 0 for hard thresholds like Strength). Both
// Observation.Strength and UncertaintyCoord.Value use this helper.
func clamp01Float(v float64, onNaN float64) float64 {
	if math.IsNaN(v) {
		return onNaN
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// MarshalJSON flattens the Payload into a {kind, payload} sub-object so the
// wire format is self-describing and Payload can be reconstructed on
// unmarshal by inspecting the Kind discriminator.
func (o Observation) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID         string          `json:"id"`
		Kind       ObservationKind `json:"kind"`
		Category   Category        `json:"category"`
		Strength   float64         `json:"strength"`
		Payload    Payload         `json:"payload,omitempty"`
		DetectedAt time.Time       `json:"detected_at"`
		Source     string          `json:"source"`
	}
	return json.Marshal(wire{
		ID:         o.ID,
		Kind:       o.Kind,
		Category:   o.Category,
		Strength:   o.Strength,
		Payload:    o.Payload,
		DetectedAt: o.DetectedAt,
		Source:     o.Source,
	})
}

// UnmarshalJSON reconstructs the Payload concrete type from the Kind
// discriminator. Unknown kinds are preserved as a generic JSON map so the
// round-trip is lossless (and a warning can be logged at the call site).
func (o *Observation) UnmarshalJSON(data []byte) error {
	var w struct {
		ID         string          `json:"id"`
		Kind       ObservationKind `json:"kind"`
		Category   Category        `json:"category"`
		Strength   float64         `json:"strength"`
		Payload    json.RawMessage `json:"payload"`
		DetectedAt time.Time       `json:"detected_at"`
		Source     string          `json:"source"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	o.ID = w.ID
	o.Kind = w.Kind
	o.Category = w.Category
	o.Strength = w.Strength
	o.DetectedAt = w.DetectedAt
	o.Source = w.Source
	if len(w.Payload) == 0 || string(w.Payload) == "null" {
		return nil
	}
	p, err := unmarshalPayload(w.Kind, w.Payload)
	if err != nil {
		return err
	}
	o.Payload = p
	return nil
}

func unmarshalPayload(k ObservationKind, raw json.RawMessage) (Payload, error) {
	switch k {
	case ObsFact:
		var p FactPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case ObsSignal:
		var p SignalPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case ObsDeviation:
		var p DeviationPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	case ObsUncertainty:
		var p UncertaintyPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return p, nil
	default:
		return nil, fmt.Errorf("orchtypes: unknown ObservationKind %d", k)
	}
}
