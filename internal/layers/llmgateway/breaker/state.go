package breaker

import (
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

type circuitRecord struct {
	state            llmgateway.CircuitState
	failureCount     int
	halfOpenSuccesses int
	openedAt         time.Time
}

func newCircuitRecord() *circuitRecord {
	return &circuitRecord{state: llmgateway.CircuitClosed}
}
