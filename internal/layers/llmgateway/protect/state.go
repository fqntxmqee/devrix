package protect

import (
	"time"

	"github.com/devrix/devrix/internal/layers/llmgateway"
)

type circuitRecord struct {
	providerKey       string
	state             llmgateway.CircuitState
	failureCount      int
	halfOpenSuccesses int
	halfOpenInFlight  int
	openedAt          time.Time
}

func newCircuitRecord(providerKey string) *circuitRecord {
	return &circuitRecord{providerKey: providerKey, state: llmgateway.CircuitClosed}
}
