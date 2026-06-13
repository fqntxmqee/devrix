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
	"github.com/devrix/devrix/internal/layers/contextengine/tasks"
	"github.com/devrix/devrix/internal/layers/observability"
	"github.com/devrix/devrix/internal/layers/observability/telemetry"
	"github.com/devrix/devrix/internal/layers/observability/tracer"
	"github.com/devrix/devrix/internal/shared/config"
	"github.com/devrix/devrix/internal/shared/types"
)

// CLIAdapter provides a command-line interface for the communication layer
type CLIAdapter struct {
	gateway      *gateway.CommunicationGateway
	renderer     *renderers.CLIRenderer
	cfg          *config.CommunicationConfig
	reader       *bufio.Reader
	writer       io.Writer
	obsBridge    *observability.Bridge
	taskCommands *tasks.CLICommands
	planMode     *tasks.PlanMode

	mu               sync.RWMutex
	running          bool
	currentSession   *types.Session
	messageHandler   func(*types.OutboundMessage)
	permissionHandler func(*types.PermissionRequest) bool
}

// NewCLIAdapter creates a new CLIAdapter
func NewCLIAdapter(
	gw *gateway.CommunicationGateway,
	cfg *config.CommunicationConfig,
) *CLIAdapter {
	return &CLIAdapter{
		gateway:      gw,
		renderer:     renderers.NewCLIRenderer(cfg.CLI.ANSI),
		cfg:          cfg,
		reader:       bufio.NewReader(os.Stdin),
		writer:       os.Stdout,
		taskCommands: tasks.NewCLICommands(tasks.GlobalTaskManager),
		planMode:     tasks.NewPlanMode(nil, nil), // LLM + ObsBridge injected later
	}
}

// SetPlanModeLLM sets the LLM for plan mode.
func (a *CLIAdapter) SetPlanModeLLM(llm tasks.LLMCompleter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.planMode != nil {
		a.planMode = tasks.NewPlanMode(llm, a.obsBridge)
	}
}

// SetObservability wires tracing/metrics into the CLI adapter.
func (a *CLIAdapter) SetObservability(obs *observability.Observability) {
	if obs == nil {
		a.obsBridge = nil
		return
	}
	a.obsBridge = observability.NewBridge(obs)
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
				slog.Info("CLI stdin closed, waiting for shutdown signal")
				<-ctx.Done()
				return a.Stop()
			}
			slog.Error("error reading input", "error", err)
		}
	}
}

// readInput reads a line of input and processes it.
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
		return a.handleNewSession()

	case types.CommandStop:
		a.mu.RLock()
		sessionID := ""
		if a.currentSession != nil {
			sessionID = a.currentSession.SessionID
		}
		a.mu.RUnlock()
		if sessionID == "" {
			a.writer.Write([]byte("\n⏸️ 没有正在运行的任务\n"))
			return nil
		}
		if err := a.gateway.StopProcess(sessionID); err != nil {
			a.writer.Write([]byte(fmt.Sprintf("\n❌ 停止失败: %v\n", err)))
			return nil
		}
		a.writer.Write([]byte("\n⏸️ 已停止当前任务\n"))

	case types.CommandHelp:
		a.showHelp()

	case types.CommandTask:
		a.handleTaskCommand(cmd.Args)

	case types.CommandPlan:
		a.handlePlanCommand(cmd.Args)

	default:
		a.writer.Write([]byte(fmt.Sprintf("%sUnknown command: %s%s\n", a.cfg.CLI.ANSI.Warning, input, a.cfg.CLI.ANSI.Reset)))
		a.showHelp()
	}

	return nil
}

// handleNewSession creates a new session
func (a *CLIAdapter) handleNewSession() error {
	a.mu.RLock()
	oldSession := a.currentSession
	a.mu.RUnlock()
	if oldSession != nil {
		if err := a.gateway.StopProcess(oldSession.SessionID); err != nil {
			slog.Warn("cli: failed to stop old session process", "sessionID", oldSession.SessionID, "error", err)
		}
	}

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
	return nil
}

// handleTaskCommand processes task-related commands
func (a *CLIAdapter) handleTaskCommand(args []string) {
	a.mu.RLock()
	sessionID := ""
	if a.currentSession != nil {
		sessionID = a.currentSession.SessionID
	}
	a.mu.RUnlock()

	raw := "/task " + strings.Join(args, " ")
	cmd := tasks.ParseCommand(raw)
	if cmd == nil {
		a.writer.Write([]byte("Invalid task command\n"))
		return
	}

	output := a.taskCommands.Handle(cmd, sessionID)
	a.writer.Write([]byte(output + "\n"))
}

// handlePlanCommand processes plan-related commands
func (a *CLIAdapter) handlePlanCommand(args []string) {
	a.mu.RLock()
	sessionID := ""
	workDir := ""
	if a.currentSession != nil {
		sessionID = a.currentSession.SessionID
		workDir = a.currentSession.WorkDir
	}
	a.mu.RUnlock()

	planCommands := tasks.NewPlanCLICommands(a.planMode)
	output := planCommands.Handle(args, sessionID, workDir, nil)
	a.writer.Write([]byte(output + "\n"))
}

// sendMessage sends a message to the gateway
func (a *CLIAdapter) sendMessage(ctx context.Context, content string) error {
	a.mu.RLock()
	session := a.currentSession
	a.mu.RUnlock()

	_, cliSpan := a.startCLISendSpan(ctx, session.SessionID, content)

	msg := &types.InboundMessage{
		SessionID: session.SessionID,
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
		if cliSpan != nil {
			cliSpan.End()
		}
		return err
	}
	if cliSpan != nil {
		cliSpan.SetStatus(tracer.StatusCodeOk, "")
		cliSpan.End()
	}

	return nil
}

func (a *CLIAdapter) startCLISendSpan(ctx context.Context, sessionID, content string) (context.Context, tracer.Span) {
	if a.obsBridge == nil || a.obsBridge.Tracer() == nil {
		return ctx, nil
	}
	opts := []tracer.SpanStartOption{
		tracer.WithSpanKind(tracer.SpanKindProducer),
		tracer.WithSpanAttributes(telemetry.SpanAttrs(telemetry.OpAdapterCLISend,
			tracer.Attribute{Key: "session.id", Value: sessionID},
			tracer.Attribute{Key: "message.len", Value: fmt.Sprintf("%d", len(content))},
		)...),
	}
	if parentSC := tracer.SpanContextFromContext(ctx); parentSC != nil {
		opts = append(opts, tracer.WithParent(*parentSC))
	}
	return a.obsBridge.Tracer().Start(ctx, telemetry.OpAdapterCLISend, opts...)
}

// showHelp displays the help message
func (a *CLIAdapter) showHelp() {
	help := `
Commands:
  /new        - Start a new session
  /stop       - Stop current generation
  /task       - Task management (see /task help)
  /plan       - Plan mode (see /plan help)
  /help       - Show this help

Task Commands:
  /task create <subject> [description]  - Create a new task
  /task list                          - List all tasks
  /task get <task_id>                - Get task details
  /task update <task_id> [status]   - Update task status
  /task ready                       - Show ready tasks
  /task dep <task_id> <blocked_by> - Add dependency

Plan Commands:
  /plan <goal>  - Enter plan mode with a goal
  /plan approve - Approve the plan
  /plan reject  - Reject the plan
  /plan status - Show plan status
  /plan show   - Show current plan

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
		return true
	default:
		return false
	}
}
