package tracer

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// TraceID represents a 16-byte trace identifier
type TraceID [16]byte

// SpanID represents an 8-byte span identifier
type SpanID [8]byte

// TraceFlags represents trace flags (W3C standard)
type TraceFlags uint8

const (
	// FlagSampled indicates the span is sampled
	FlagSampled TraceFlags = 0x01
)

// TraceState represents the tracestate header value
type TraceState string

// SpanContext contains trace identification information
type SpanContext struct {
	TraceID    TraceID
	SpanID     SpanID
	TraceFlags TraceFlags
	TraceState TraceState
	Remote     bool
}

// IsValid checks if the span context is valid
func (sc SpanContext) IsValid() bool {
	return sc.TraceID.IsValid() && sc.SpanID.IsValid()
}

// IsSampled checks if the span is sampled
func (sc SpanContext) IsSampled() bool {
	return sc.TraceFlags&FlagSampled == FlagSampled
}

// String returns the W3C traceparent string representation
func (sc SpanContext) String() string {
	return fmt.Sprintf("00-%s-%s-%02x",
		sc.TraceID.String(),
		sc.SpanID.String(),
		sc.TraceFlags,
	)
}

// ParseTraceID parses a hex string into a TraceID
func ParseTraceID(s string) (TraceID, error) {
	var tid TraceID
	s = strings.TrimSpace(s)

	// Handle 32-char hex (W3C standard) or 16-char hex
	switch len(s) {
	case 32:
		// 16 bytes = 32 hex chars
		b, err := hex.DecodeString(s)
		if err != nil {
			return TraceID{}, fmt.Errorf("invalid trace ID hex: %w", err)
		}
		copy(tid[:], b)
		return tid, nil
	case 16:
		// Already 8 bytes = 16 hex chars (legacy format)
		b, err := hex.DecodeString(s)
		if err != nil {
			return TraceID{}, fmt.Errorf("invalid trace ID hex: %w", err)
		}
		// Pad to 16 bytes
		copy(tid[8:], b)
		return tid, nil
	default:
		return TraceID{}, fmt.Errorf("invalid trace ID length: %d (expected 32 or 16)", len(s))
	}
}

// ParseSpanID parses a hex string into a SpanID
func ParseSpanID(s string) (SpanID, error) {
	var sid SpanID
	s = strings.TrimSpace(s)

	if len(s) != 16 {
		return SpanID{}, fmt.Errorf("invalid span ID length: %d (expected 16)", len(s))
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return SpanID{}, fmt.Errorf("invalid span ID hex: %w", err)
	}
	copy(sid[:], b)
	return sid, nil
}

// IsValid checks if the trace ID is valid (non-zero)
func (tid TraceID) IsValid() bool {
	for _, b := range tid {
		if b != 0 {
			return true
		}
	}
	return false
}

// String returns the hex string representation (32 chars)
func (tid TraceID) String() string {
	return hex.EncodeToString(tid[:])
}

// IsValid checks if the span ID is valid (non-zero)
func (sid SpanID) IsValid() bool {
	for _, b := range sid {
		if b != 0 {
			return true
		}
	}
	return false
}

// String returns the hex string representation (16 chars)
func (sid SpanID) String() string {
	return hex.EncodeToString(sid[:])
}

// SpanStatusCode represents the status of a span
type SpanStatusCode int

const (
	// StatusCodeUnset indicates the span has not been set
	StatusCodeUnset SpanStatusCode = iota
	// StatusCodeOk indicates the span has completed successfully
	StatusCodeOk
	// StatusCodeError indicates the span has completed with an error
	StatusCodeError
)

// String returns the string representation
func (s SpanStatusCode) String() string {
	switch s {
	case StatusCodeUnset:
		return "Unset"
	case StatusCodeOk:
		return "Ok"
	case StatusCodeError:
		return "Error"
	default:
		return fmt.Sprintf("StatusCode(%d)", s)
	}
}

// SpanKind represents the kind of span
type SpanKind int

const (
	// SpanKindInternal indicates an internal span
	SpanKindInternal SpanKind = iota
	// SpanKindServer indicates a server span
	SpanKindServer
	// SpanKindClient indicates a client span
	SpanKindClient
	// SpanKindProducer indicates a producer span
	SpanKindProducer
	// SpanKindConsumer indicates a consumer span
	SpanKindConsumer
)

// String returns the string representation
func (k SpanKind) String() string {
	switch k {
	case SpanKindInternal:
		return "internal"
	case SpanKindServer:
		return "server"
	case SpanKindClient:
		return "client"
	case SpanKindProducer:
		return "producer"
	case SpanKindConsumer:
		return "consumer"
	default:
		return fmt.Sprintf("SpanKind(%d)", k)
	}
}
