// CommandHandler is the D7-S2 orchtypes.IntentCommand explicit-dispatch path.
//
// Dispatches commands directly to D7-internal CLI/PlanMode handlers
// (zero LLM cost on the command path; command semantics owned by D7 —
// PlanMode state machine + TaskManager CRUD — not by an LLM guessing).
//
// Wiring: bootstrap injects CLICommands and PlanCLICommands via
// NewCommandHandler. The handler is then attached to SessionOrchestrator
// via WithCommandHandler. ProcessMessage's orchtypes.IntentCommand case calls
// Handle directly.
package sessionorchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/devrix/devrix/internal/layers/orchestration/orchtypes"
	"github.com/devrix/devrix/internal/layers/orchestration/workmodel"
	"github.com/devrix/devrix/internal/shared/contracts"
)

// CommandHandler dispatches orchtypes.IntentCommand to D7-internal command processors.
// It is the v1.1+ replacement for the v1.0 FastPath hint passthrough.
type CommandHandler struct {
	cli             *workmodel.CLICommands
	plan            *workmodel.PlanCLICommands
	sink            EventPublisher
	interruptHandle func(ctx context.Context, sessionID string) error
}

// NewCommandHandler builds the handler. cli and plan are required; sink
// and interruptHandle are optional (nil-safe at call sites).
func NewCommandHandler(
	cli *workmodel.CLICommands,
	plan *workmodel.PlanCLICommands,
	sink EventPublisher,
) *CommandHandler {
	return &CommandHandler{cli: cli, plan: plan, sink: sink}
}

// SetInterruptHandle lets bootstrap install a /stop handler. When set,
// /stop calls interruptHandle.Handle(ctx, sessionID). nil → /stop returns
// a textual acknowledgement without cancelling the active process.
func (h *CommandHandler) SetInterruptHandle(f func(ctx context.Context, sessionID string) error) {
	h.interruptHandle = f
}

// Handle processes an orchtypes.IntentCommand. The classifier has already extracted
// intent.Command as the first whitespace-separated token; the rest of the
// message is the argument list.
//
// Returned channel emits exactly: command_reply (to sink only, not to
// caller) → text (the reply string) → complete. The channel is closed
// after complete.
//
// Handle does NOT call the LLM. The command path has zero LLM cost.
func (h *CommandHandler) Handle(ctx context.Context, req orchtypes.ProcessRequest, intent orchtypes.IntentClassification) (<-chan *contracts.EngineEvent, error) {
	if h == nil {
		return nil, fmt.Errorf("orchestrator: CommandHandler is nil (bootstrap missing wiring)")
	}
	if h.cli == nil {
		return nil, fmt.Errorf("orchestrator: CommandHandler.cli is nil (bootstrap missing CLICommands)")
	}
	if h.plan == nil {
		return nil, fmt.Errorf("orchestrator: CommandHandler.plan is nil (bootstrap missing PlanCLICommands)")
	}

	// Parse command + args from the full user message. The classifier stores
	// only the first token in intent.Command; args live in req.Message.
	raw := strings.TrimSpace(req.Message)
	if raw == "" {
		raw = strings.TrimSpace(intent.Command)
	}
	cmd, args := splitCommand(raw)

	reply, err := h.dispatch(ctx, req, cmd, args)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: CommandHandler.%s: %w", cmd, err)
	}

	out := make(chan *contracts.EngineEvent, 4)
	go func() {
		defer close(out)
		if h.sink != nil {
			h.sink.Publish(ctx, &contracts.EngineEvent{
				Type:      "command_reply",
				Content:   reply,
				SessionID: req.SessionID,
				Metadata:  map[string]string{"command": cmd},
			})
		}
		// A5 (DM-20260622-001): select-default guards against consumer stalls.
		// The Handle goroutine must not block indefinitely when the consumer is
		// slow or wedged; the user-facing /stop, /help responses are
		// best-effort and slog.Warn preserves an audit trail. See design.md §2.5.
		emit := func(ev *contracts.EngineEvent) {
			select {
			case out <- ev:
			default:
				slog.Warn("command_handler: out channel full, drop event",
					"type", ev.Type,
					"session", ev.SessionID,
					"channel_size", cap(out),
				)
			}
		}
		emit(&contracts.EngineEvent{Type: "text", Content: reply, SessionID: req.SessionID})
		emit(&contracts.EngineEvent{Type: "complete", SessionID: req.SessionID})
	}()
	return out, nil
}

// dispatch routes the command to the right processor. The set of
// recognized commands matches coordinator.DefaultConfig.CommandWhitelist
// (see config.go); commands outside this set produce an error.
func (h *CommandHandler) dispatch(ctx context.Context, req orchtypes.ProcessRequest, cmd string, args []string) (string, error) {
	switch cmd {
	case "/plan":
		// PlanMode trigger (read-only exploration → pending_approval).
		// workDir + tools are not exposed at the D1 boundary yet; pass
		// empty values that the PlanMode defaults to a no-tools agent
		// when args is empty (mirrors PlanCLICommands.help behavior).
		return h.plan.Handle(args, req.SessionID, "", workmodel.PlanAgentReadOnlyTools), nil
	case "/task":
		// Task CRUD. ParseCommand expects a "/task ..." raw input; we
		// reconstruct it from cmd + args since the classifier already
		// stripped the command token.
		parsed := workmodel.ParseCommand("/task " + strings.Join(args, " "))
		if parsed == nil {
			parsed = &workmodel.Command{Name: "help"}
		}
		return h.cli.Handle(parsed, req.SessionID), nil
	case "/help":
		return h.cli.Help(), nil
	case "/stop":
		if h.interruptHandle != nil {
			_ = h.interruptHandle(ctx, req.SessionID)
			return "stopped", nil
		}
		return "stopped (no interrupt handler wired)", nil
	default:
		return "", fmt.Errorf("unknown command %q (not in whitelist)", cmd)
	}
}

// splitCommand extracts the command token and the rest of the message as
// args. The classifier pre-trims and stores the first token in
// intent.Command; this function handles the "command with args" case
// where the classifier stored the whole "/plan add auth" string.
func splitCommand(raw string) (string, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	idx := strings.IndexAny(raw, " \t\n")
	if idx < 0 {
		return raw, nil
	}
	cmd := raw[:idx]
	rest := strings.TrimSpace(raw[idx+1:])
	if rest == "" {
		return cmd, nil
	}
	return cmd, strings.Fields(rest)
}

// newDefaultCommandHandler builds a CommandHandler bound to the supplied
// TaskManager and a fresh PlanMode. It is the v1.1.0+ default for
// NewSessionOrchestrator when no WithCommandHandler option is supplied.
//
// The default is intentionally minimal — production callers that need a
// shared PlanMode state across sessions should still wire explicitly.
func newDefaultCommandHandler(_ WorkModel, sink EventPublisher, tm *workmodel.TaskManager) *CommandHandler {
	// Use the injected TaskManager for /task commands. For /plan, each
	// default-constructed PlanMode maintains its own state, which is
	// correct for v1.1.0 (PlanMode is per-session; the injected
	// TaskManager stores the underlying tasks).
	cli := workmodel.NewCLICommands(tm)
	plan := workmodel.NewPlanCLICommands(workmodel.NewPlanMode(nil, nil))
	plan.SetTaskManager(tm)
	return NewCommandHandler(cli, plan, sink)
}
