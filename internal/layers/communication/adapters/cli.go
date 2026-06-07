package adapters

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/layers/communication/gateway"
	"github.com/devrix/devrix/internal/layers/communication/renderers"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// CLIAdapter provides a command-line interface for the communication layer
type CLIAdapter struct {
	gateway  *gateway.CommunicationGateway
	renderer *renderers.CLRenderer
	cfg      *config.CommunicationConfig
	reader   *bufio.Reader
	writer   io.Writer

	mu             sync.RWMutex
	running        bool
	currentSession *types.Session
	messageHandler   func(*types.OutboundMessage)
	permissionHandler func(*types.PermissionRequest) bool
}

// NewCLIAdapter creates a new CLIAdapter
func NewCLIAdapter(
	gw *gateway.CommunicationGateway,
	cfg *config.CommunicationConfig,
) *CLIAdapter {
	return &CLIAdapter{
		gateway:  gw,
		renderer: renderers.NewCLIRenderer(cfg.CLI.ANSI),
		cfg:      cfg,
		reader:   bufio.NewReader(os.Stdin),
		writer:   os.Stdout,
	}
}

// Start begins the interactive CLI session
func (a *CLIAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("CLI adapter already running")
	}
	a.running = true
	a.mu.Unlock()

	// Show welcome message
	a.writer.Write([]byte(a.cfg.CLI.WelcomeMessage))
	a.writer.Write([]byte("\n\n"))

	// Create initial session
	workDir, _ := os.Getwd()
	session, err := a.gateway.CreateSession("cli", workDir)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	a.currentSession = session

	// Main input loop
	slog.Info("CLI adapter started", "sessionID", session.SessionID)

	for {
		if err := ctx.Err(); err != nil {
			return a.Stop()
		}
		if err := a.readInput(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				return a.Stop()
			}
			if err == io.EOF {
				// Non-interactive stdin (e.g. IM-only mode): wait for shutdown signal.
				slog.Info("CLI stdin closed, waiting for shutdown signal")
				<-ctx.Done()
				return a.Stop()
			}
			slog.Error("error reading input", "error", err)
		}
	}
}

// readInput reads a line of input and processes it.
// The read is interruptible: Ctrl+C cancels ctx and returns without waiting for Enter.
func (a *CLIAdapter) readInput(ctx context.Context) error {
	input, err := a.readLine(ctx)
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	// Check for commands
	if strings.HasPrefix(input, a.cfg.Commands.Prefix) {
		return a.handleCommand(ctx, input)
	}

	// Send to gateway
	return a.sendMessage(ctx, input)
}

func (a *CLIAdapter) readLine(ctx context.Context) (string, error) {
	type readResult struct {
		line string
		err  error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		a.writer.Write([]byte(a.cfg.CLI.Prompt))
		line, err := a.reader.ReadString('\n')
		resultCh <- readResult{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", context.Canceled
	case res := <-resultCh:
		return res.line, res.err
	}
}

// handleCommand processes a CLI command
func (a *CLIAdapter) handleCommand(ctx context.Context, input string) error {
	cmd := types.ParseCommand(input, a.cfg.Commands.Prefix)

	switch cmd.Type {
	case types.CommandNew:
		// Create new session
		workDir, _ := os.Getwd()
		newSession, err := a.gateway.CreateSession("cli", workDir)
		if err != nil {
			return fmt.Errorf("failed to create new session: %w", err)
		}
		a.mu.Lock()
		a.currentSession = newSession
		a.mu.Unlock()

		a.writer.Write([]byte("\n--- New session started ---\n\n"))
		slog.Info("new session created", "sessionID", newSession.SessionID)

	case types.CommandStop:
		a.writer.Write([]byte("\n--- Stop requested ---\n"))
		// TODO: Cancel current LLM call

	case types.CommandHelp:
		a.showHelp()

	default:
		a.writer.Write([]byte(fmt.Sprintf("%sUnknown command: %s%s\n", a.cfg.CLI.ANSI.Warning, input, a.cfg.CLI.ANSI.Reset)))
		a.showHelp()
	}

	return nil
}

// sendMessage sends a message to the gateway
func (a *CLIAdapter) sendMessage(ctx context.Context, content string) error {
	a.mu.RLock()
	session := a.currentSession
	a.mu.RUnlock()

	msg := &types.InboundMessage{
		SessionID:  session.SessionID,
		ChatID:    "cli",
		Content:   content,
		MessageID: fmt.Sprintf("cli_%s", session.SessionID),
		AdapterID: "cli",
		Metadata: map[string]string{
			"work_dir": session.WorkDir,
		},
	}

	if err := a.gateway.RouteInbound(ctx, msg); err != nil {
		a.renderer.RenderError(err)
		return err
	}

	return nil
}

// showHelp displays the help message
func (a *CLIAdapter) showHelp() {
	help := `
Commands:
  /new    - Start a new session
  /stop   - Stop current generation
  /help   - Show this help

`
	a.writer.Write([]byte(help))
}

// Stop stops the CLI adapter
func (a *CLIAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.running = false
	slog.Info("CLI adapter stopped")
	return nil
}

// OnMessage registers a callback for incoming messages
func (a *CLIAdapter) OnMessage(handler func(*types.OutboundMessage)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messageHandler = handler
}

// OnPermissionRequest registers a callback for permission requests
func (a *CLIAdapter) OnPermissionRequest(handler func(*types.PermissionRequest) bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.permissionHandler = handler
}

// HandlePermission handles a permission request by prompting the user
func (a *CLIAdapter) HandlePermission(req *types.PermissionRequest) bool {
	a.renderer.RenderPermissionRequest(req)

	a.writer.Write([]byte("\n"))
	a.writer.Write([]byte(fmt.Sprintf("%s[Risk: %s]%s ", a.cfg.CLI.ANSI.Warning, req.RiskLevel, a.cfg.CLI.ANSI.Reset)))
	a.writer.Write([]byte("Allow execution of " + req.ToolName + "? (yes/no/all): "))

	input, err := a.reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "yes", "y":
		return true
	case "all", "a":
		// TODO: Implement allow-all logic
		return true
	default:
		return false
	}
}
