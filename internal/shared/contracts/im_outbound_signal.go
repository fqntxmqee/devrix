package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// SignalKind classifies IM outbound signals (切法 A: Thinking / Task / Conclusion).
//
// DSAFT: D1-S14 / D1-S15 / D1-S16
type SignalKind string

const (
	SignalThinking   SignalKind = "thinking"
	SignalTask       SignalKind = "task"
	SignalConclusion SignalKind = "conclusion"
)

// IMOutboundSignal is the canonical D1 outbound signal contract.
// Objective anchor fields are filled by D1; Agent must not self-report Confidence as SoT.
//
// DSAFT: DM-20260614-006 AC9
type IMOutboundSignal struct {
	Kind          SignalKind
	SessionID     string
	Sequence      uint64
	Delta         string
	IsTerminal    bool
	SourceEventID string
	ElapsedMs     int64
	InboundTurnID string
	Metadata      map[string]string
}

// MapEngineEventToSignal maps a context-engine event to an IMOutboundSignal Kind.
// seq and anchors are assigned by the D1 turn tracker at emit time.
func MapEngineEventToSignal(ev *EngineEvent, seq uint64, inboundTurnID string, turnStarted time.Time) (IMOutboundSignal, bool) {
	if ev == nil || ev.SessionID == "" {
		return IMOutboundSignal{}, false
	}
	kind, ok := engineEventSignalKind(ev.Type)
	if !ok {
		return IMOutboundSignal{}, false
	}
	elapsed := time.Since(turnStarted).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	meta := cloneStringMap(ev.Metadata)
	sig := IMOutboundSignal{
		Kind:          kind,
		SessionID:     ev.SessionID,
		Sequence:      seq,
		Delta:         ev.Content,
		IsTerminal:    kind == SignalConclusion && (ev.Type == "complete" || ev.Type == "error"),
		SourceEventID: SourceEventID(ev),
		ElapsedMs:     elapsed,
		InboundTurnID: inboundTurnID,
		Metadata:      meta,
	}
	if ev.ToolName != "" {
		if sig.Metadata == nil {
			sig.Metadata = map[string]string{}
		}
		sig.Metadata["tool_name"] = ev.ToolName
	}
	return sig, true
}

func engineEventSignalKind(eventType string) (SignalKind, bool) {
	switch eventType {
	case "thinking":
		return SignalThinking, true
	case "tool_call", "tool_result", "tool", "milestone_progress", "worker_progress", "progress":
		return SignalTask, true
	case "text", "complete", "error":
		return SignalConclusion, true
	default:
		return "", false
	}
}

// SourceEventID builds a stable, D1-assigned id for trace correlation (not Agent self-report).
func SourceEventID(ev *EngineEvent) string {
	if ev == nil {
		return ""
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s", ev.SessionID, ev.Type, ev.Content, ev.ToolName)))
	return hex.EncodeToString(h[:8])
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// ConclusionFeedbackPrefix is the inbound hook for user rejection capture (S13 → D5 span).
const ConclusionFeedbackPrefix = "/feedback "

// ParseConclusionFeedback reports whether content is a user conclusion-feedback turn.
func ParseConclusionFeedback(content string) (feedback bool, reason string) {
	if len(content) <= len(ConclusionFeedbackPrefix) {
		return false, ""
	}
	if content[:len(ConclusionFeedbackPrefix)] != ConclusionFeedbackPrefix {
		return false, ""
	}
	reason = content[len(ConclusionFeedbackPrefix):]
	return reason != "", reason
}
