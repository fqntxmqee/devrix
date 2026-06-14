package kernel

import (
	"strconv"

	"github.com/devrix/devrix/internal/shared/contracts"
)

// OutboundMetadata builds base metadata for an engine event type.
func OutboundMetadata(eventType string, meta map[string]string) map[string]string {
	out := map[string]string{"event_type": eventType}
	if len(meta) == 0 {
		return out
	}
	for k, v := range meta {
		if k == "event_type" || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// EnrichMetadata attaches canonical signal anchor fields (DM-20260614-006 AC9).
func EnrichMetadata(meta map[string]string, sig contracts.IMOutboundSignal) map[string]string {
	if sig.SourceEventID == "" && sig.Sequence == 0 && sig.InboundTurnID == "" {
		return meta
	}
	if meta == nil {
		meta = make(map[string]string)
	}
	meta["signal_kind"] = string(sig.Kind)
	meta["signal_sequence"] = strconv.FormatUint(sig.Sequence, 10)
	meta["source_event_id"] = sig.SourceEventID
	meta["elapsed_ms"] = strconv.FormatInt(sig.ElapsedMs, 10)
	meta["inbound_turn_id"] = sig.InboundTurnID
	return meta
}

func SigOrEmpty(hasSig bool, sig contracts.IMOutboundSignal) contracts.IMOutboundSignal {
	if hasSig {
		return sig
	}
	return contracts.IMOutboundSignal{}
}

// MetaField reads a metadata key safely.
func MetaField(meta map[string]string, key string) string {
	if meta == nil {
		return ""
	}
	return meta[key]
}
