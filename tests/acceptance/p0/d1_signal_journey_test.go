//go:build acceptance && d1

package p0

import (
	"context"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/contracts"
	"github.com/devrix/devrix/internal/shared/types"
	"github.com/devrix/devrix/tests/testutil"
)

// T: D1-S13-A02-T01, D1-S14-A01-F01-T01, D1-S15-A01-F01-T01, D1-S16-A02-T01
func TestL5_D1_SignalJourney_CaptureToConclusion(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	handler := testutil.NewMockEventHandler()
	engine := &testutil.MockContextEngine{
		Events: []*capture.EngineEvent{
			{Type: "thinking", Content: "planning", SessionID: ""},
			{Type: "tool_call", ToolName: "grep", SessionID: ""},
			{Type: "text", Content: "answer", SessionID: ""},
			{Type: "complete", SessionID: ""},
		},
	}

	gw := capture.NewCommunicationGateway(store, handler, nil, config.DefaultConfig())
	testutil.WireGatewayOrchestration(gw, engine)
	session, err := gw.CreateSession("cli", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for _, ev := range engine.Events {
		ev.SessionID = session.SessionID
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		MessageID: "turn-1",
		Content:   "explain signals",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	if !handler.WaitForMessages(3, 3*time.Second) {
		t.Fatalf("expected outbound messages, got %d", handler.MessageCount())
	}

	msgs := handler.OutboundMessages()

	var sawThinking, sawTask, sawComplete bool
	for _, msg := range msgs {
		kind := msg.Metadata["signal_kind"]
		switch kind {
		case string(contracts.SignalThinking):
			sawThinking = true
		case string(contracts.SignalTask):
			sawTask = true
		case string(contracts.SignalConclusion):
			if msg.Metadata["event_type"] == "complete" {
				sawComplete = true
			}
		}
		if msg.Metadata["source_event_id"] == "" {
			t.Fatalf("missing source_event_id on %v", msg.Metadata["event_type"])
		}
		if msg.Metadata["inbound_turn_id"] != "turn-1" {
			t.Fatalf("inbound_turn_id=%q want turn-1", msg.Metadata["inbound_turn_id"])
		}
	}
	if !sawThinking || !sawTask || !sawComplete {
		t.Fatalf("signal chain incomplete: thinking=%v task=%v complete=%v", sawThinking, sawTask, sawComplete)
	}
}

// T: D1-S16-A02-T02, D1-S18-A01-F02-T01
func TestL5_D1_SignalJourney_ErrorConclusion(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	handler := testutil.NewMockEventHandler()
	engine := &testutil.MockContextEngine{
		Events: []*capture.EngineEvent{
			{Type: "error", Content: "engine failed", SessionID: ""},
		},
	}

	gw := capture.NewCommunicationGateway(store, handler, nil, config.DefaultConfig())
	testutil.WireGatewayOrchestration(gw, engine)
	session, err := gw.CreateSession("cli", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	for _, ev := range engine.Events {
		ev.SessionID = session.SessionID
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		MessageID: "turn-err",
		Content:   "trigger error",
	}); err != nil {
		t.Fatalf("RouteInbound: %v", err)
	}

	if !handler.WaitForMessages(1, 3*time.Second) {
		t.Fatalf("expected error outbound, got %d", handler.MessageCount())
	}
	msg := handler.OutboundMessages()[0]
	if msg.Metadata["event_type"] != "error" {
		t.Fatalf("event_type=%q want error", msg.Metadata["event_type"])
	}
	if msg.Metadata["signal_kind"] != string(contracts.SignalConclusion) {
		t.Fatalf("signal_kind=%q want conclusion", msg.Metadata["signal_kind"])
	}
	if msg.Metadata["source_event_id"] == "" {
		t.Fatal("missing source_event_id on error conclusion")
	}
}

// T: D1-S13-A04-T01 hook — feedback capture without dispatch
func TestL5_D1_ConclusionFeedbackCapture(t *testing.T) {
	dir := t.TempDir()
	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	handler := testutil.NewMockEventHandler()
	engine := &testutil.MockContextEngine{
		Events: []*capture.EngineEvent{{Type: "complete"}},
	}
	gw := capture.NewCommunicationGateway(store, handler, nil, config.DefaultConfig())
	testutil.WireGatewayOrchestration(gw, engine)
	session, err := gw.CreateSession("cli", dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := gw.RouteInbound(context.Background(), &types.InboundMessage{
		SessionID: session.SessionID,
		ChatID:    "cli",
		Content:   contracts.ConclusionFeedbackPrefix + "wrong summary",
	}); err != nil {
		t.Fatalf("RouteInbound feedback: %v", err)
	}
	if handler.MessageCount() != 0 {
		t.Fatalf("feedback should not dispatch engine, got %d messages", handler.MessageCount())
	}
}
