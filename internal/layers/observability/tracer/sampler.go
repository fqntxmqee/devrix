package tracer

import (
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/devrix/devrix/internal/layers/observability/settings"
)

// Sampler determines whether a span should be sampled
type Sampler interface {
	ShouldSample(traceID TraceID) bool
	Description() string
}

// samplerConfig holds sampler configuration
type samplerConfig struct {
	Type string
	Rate float64
}

// NewSampler creates a new sampler based on configuration
func NewSampler(cfg *settings.SamplingConfig) Sampler {
	switch cfg.Type {
	case "always_on":
		return &AlwaysOnSampler{}
	case "always_off":
		return &AlwaysOffSampler{}
	case "trace_id_ratio":
		return &TraceIdRatioSampler{rate: cfg.Rate}
	default:
		return &AlwaysOnSampler{}
	}
}

// AlwaysOnSampler always samples all spans
type AlwaysOnSampler struct{}

func (s *AlwaysOnSampler) ShouldSample(traceID TraceID) bool {
	return true
}

func (s *AlwaysOnSampler) Description() string {
	return "AlwaysOnSampler"
}

// AlwaysOffSampler never samples spans
type AlwaysOffSampler struct{}

func (s *AlwaysOffSampler) ShouldSample(traceID TraceID) bool {
	return false
}

func (s *AlwaysOffSampler) Description() string {
	return "AlwaysOffSampler"
}

// TraceIdRatioSampler samples spans based on trace ID hash
type TraceIdRatioSampler struct {
	rate float64
}

func (s *TraceIdRatioSampler) ShouldSample(traceID TraceID) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0.0 {
		return false
	}

	// Use trace ID as the sampling key
	hasher := fnv.New64a()
	hasher.Write(traceID[:])
	hash := hasher.Sum64()

	// Compare against rate (0.0-1.0)
	sampledValue := float64(hash%10000) / 10000.0
	return sampledValue < s.rate
}

func (s *TraceIdRatioSampler) Description() string {
	return "TraceIdRatioSampler{" + formatRate(s.rate) + "}"
}

func formatRate(rate float64) string {
	return time.Duration(rate * 1e11).String() // approximate
}

// DeterministicSampler samples based on a deterministic function of trace ID
// Useful for testing
type DeterministicSampler struct {
	threshold uint64
}

func NewDeterministicSampler(rate float64) *DeterministicSampler {
	return &DeterministicSampler{
		threshold: uint64(rate * float64(1<<63)),
	}
}

func (s *DeterministicSampler) ShouldSample(traceID TraceID) bool {
	hasher := fnv.New64a()
	hasher.Write(traceID[:])
	hash := hasher.Sum64()
	return hash < s.threshold
}

func (s *DeterministicSampler) Description() string {
	return "DeterministicSampler"
}

// RandomSampler uses random sampling
type RandomSampler struct {
	rng *rand.Rand
	rate float64
}

func NewRandomSampler(rate float64) *RandomSampler {
	return &RandomSampler{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		rate: rate,
	}
}

func (s *RandomSampler) ShouldSample(traceID TraceID) bool {
	return s.rng.Float64() < s.rate
}

func (s *RandomSampler) Description() string {
	return "RandomSampler"
}
