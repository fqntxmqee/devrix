package tracer

import (
	"context"
	"strings"
)

// PropagationEnvVars returns W3C trace/baggage headers as uppercase env vars for subprocesses.
func PropagationEnvVars(ctx context.Context) []string {
	carrier := MapCarrier{}
	prop := NewPropagator()
	if err := prop.Inject(ctx, carrier); err != nil {
		return nil
	}

	var out []string
	for _, key := range []string{"traceparent", "tracestate", BaggageHeader} {
		if v := carrier.Get(key); v != "" {
			out = append(out, strings.ToUpper(key)+"="+v)
		}
	}
	return out
}
