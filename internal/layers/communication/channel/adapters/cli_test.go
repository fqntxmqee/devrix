package adapters

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/devrix/devrix/internal/layers/communication/capture"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

type cliMockEventHandler struct{}

func (cliMockEventHandler) OnMessage(*types.OutboundMessage)                       {}
func (cliMockEventHandler) OnPermissionRequest(*types.PermissionRequest) bool       { return true }
func (cliMockEventHandler) OnError(error, string)                                 {}
func (cliMockEventHandler) OnStatus(string, types.SessionState)                   {}

type cliMockContextEngine struct {
	events []*capture.EngineEvent
}

func (m *cliMockContextEngine) Process(_ context.Context, _ *types.Session, _ string) <-chan *capture.EngineEvent {
	ch := make(chan *capture.EngineEvent, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch
}

type cliOrchestrationEntry struct {
	engine *cliMockContextEngine
}

func (e *cliOrchestrationEntry) ProcessMessage(ctx context.Context, sessionID, message string) (<-chan *capture.EngineEvent, error) {
	session := types.NewSession(sessionID, "cli", "")
	return e.engine.Process(ctx, session, message), nil
}

func (e *cliOrchestrationEntry) Cancel(context.Context, string) error { return nil }

func newTestGateway(t *testing.T) *capture.CommunicationGateway {
	t.Helper()
	return newTestGatewayInDir(t, t.TempDir())
}

func newTestGatewayInDir(t *testing.T, dir string) *capture.CommunicationGateway {
	t.Helper()

	store, err := capture.NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	engine := &cliMockContextEngine{
		events: []*capture.EngineEvent{
			{Type: "text", Content: "ok"},
			{Type: "complete", Content: ""},
		},
	}
	gw := capture.NewCommunicationGateway(store, cliMockEventHandler{}, nil, config.DefaultConfig(), nil)
	gw.SetOrchestrationEntry(&cliOrchestrationEntry{engine: engine})
	return gw
}

// waitGatewayAsync lets RouteInbound background goroutines finish before t.TempDir cleanup.
func waitGatewayAsync(t *testing.T) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
}

func newTestCLIAdapter(t *testing.T, gw *capture.CommunicationGateway, stdin string) (*CLIAdapter, *bytes.Buffer) {
	t.Helper()

	out := &bytes.Buffer{}
	a := NewCLIAdapter(gw, config.DefaultConfig())
	a.reader = bufio.NewReader(strings.NewReader(stdin))
	a.writer = out
	return a, out
}

func TestCLIAdapter_should_handle_help_command(t *testing.T) {
	gw := newTestGateway(t)
	a, out := newTestCLIAdapter(t, gw, "")

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	a.currentSession = session

	if err := a.handleCommand(context.Background(), "/help"); err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if !strings.Contains(out.String(), "/new") {
		t.Fatalf("expected help text, got %q", out.String())
	}
}

func TestCLIAdapter_should_create_new_session_on_new_command(t *testing.T) {
	gw := newTestGateway(t)
	a, out := newTestCLIAdapter(t, gw, "")

	first, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	a.currentSession = first

	if err := a.handleCommand(context.Background(), "/new"); err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if a.currentSession.SessionID == first.SessionID {
		t.Fatal("expected new session id")
	}
	if !strings.Contains(out.String(), "New session started") {
		t.Fatalf("expected new session message, got %q", out.String())
	}
}

func TestCLIAdapter_should_route_message_to_gateway(t *testing.T) {
	dir := t.TempDir()
	gw := newTestGatewayInDir(t, dir)
	a, _ := newTestCLIAdapter(t, gw, "")

	session, err := gw.CreateSession("cli", dir)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	a.currentSession = session

	if err := a.sendMessage(context.Background(), "hello gateway"); err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	waitGatewayAsync(t)
}

func TestCLIAdapter_should_ignore_empty_input(t *testing.T) {
	gw := newTestGateway(t)
	a, out := newTestCLIAdapter(t, gw, "\n")

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	a.currentSession = session

	if err := a.readInput(context.Background()); err != nil {
		t.Fatalf("readInput: %v", err)
	}
	if !strings.Contains(out.String(), config.DefaultConfig().CLI.Prompt) {
		t.Fatalf("expected prompt for empty input, got %q", out.String())
	}
}

func TestCLIAdapter_should_handle_unknown_command(t *testing.T) {
	gw := newTestGateway(t)
	a, out := newTestCLIAdapter(t, gw, "")

	session, err := gw.CreateSession("cli", t.TempDir())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	a.currentSession = session

	if err := a.handleCommand(context.Background(), "/unknown"); err != nil {
		t.Fatalf("handleCommand: %v", err)
	}
	if !strings.Contains(out.String(), "Unknown command") {
		t.Fatalf("expected unknown command message, got %q", out.String())
	}
}

func TestCLIAdapter_HandlePermission_should_accept_yes_and_reject_no(t *testing.T) {
	gw := newTestGateway(t)

	yesAdapter, _ := newTestCLIAdapter(t, gw, "yes\n")
	if !yesAdapter.HandlePermission(&types.PermissionRequest{ToolName: "bash", RiskLevel: types.RiskLevelHigh}) {
		t.Fatal("expected yes to allow")
	}

	noAdapter, _ := newTestCLIAdapter(t, gw, "no\n")
	if noAdapter.HandlePermission(&types.PermissionRequest{ToolName: "bash", RiskLevel: types.RiskLevelHigh}) {
		t.Fatal("expected no to deny")
	}

	allAdapter, _ := newTestCLIAdapter(t, gw, "all\n")
	if !allAdapter.HandlePermission(&types.PermissionRequest{ToolName: "bash", RiskLevel: types.RiskLevelHigh}) {
		t.Fatal("expected all to allow")
	}
}

func TestCLIAdapter_should_register_handlers_and_stop(t *testing.T) {
	gw := newTestGateway(t)
	a, _ := newTestCLIAdapter(t, gw, "")

	called := false
	a.OnMessage(func(*types.OutboundMessage) { called = true })
	a.OnPermissionRequest(func(*types.PermissionRequest) bool { return true })

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	if err := a.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if called {
		t.Fatal("handler should not have been invoked")
	}
	if err := a.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

func TestCLIAdapter_should_reject_double_start(t *testing.T) {
	gw := newTestGateway(t)
	a, _ := newTestCLIAdapter(t, gw, "")

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	if err := a.Start(context.Background()); err == nil {
		t.Fatal("expected error when already running")
	}
}
