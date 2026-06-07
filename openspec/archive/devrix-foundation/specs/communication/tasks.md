# Communication Layer Tasks (Layer 1)

**Change ID:** `devrix-foundation`
**Layer:** 1 - Communication
**Status:** Draft
**Version:** 1.0
**Based on:** design.md

---

## Task Overview

实现通信层的核心模块：CLI 适配器、会话存储、通信网关、命令处理器。

共 **58 个子任务**，分为 9 个阶段。

---

## Phase 1: Shared Types & Error Definitions

### 1.1 Type Definitions

**Owner:** internal/shared/types

- [ ] 1.1.1 Define `Session` struct
  - Location: `internal/shared/types/session.go`
  - Fields: SessionID, RequestID, AdapterID, UserID, UserName, WorkDir, Model, State, CurrentAgentID, CreatedAt, UpdatedAt, LastMessageAt, ContextSnapshot

- [ ] 1.1.2 Define `SessionState` type
  - Location: `internal/shared/types/session.go`
  - States: SessionStateIdle, SessionStateThinking, SessionStateStreaming, SessionStateToolExecuting, SessionStateWaitingPermission, SessionStateCompleted, SessionStateFailed

- [ ] 1.1.3 Define `Message` struct
  - Location: `internal/shared/types/message.go`
  - Fields: ID, SessionID, Role, Content, Attachments, Metadata, Timestamp

- [ ] 1.1.4 Define `Attachment` struct
  - Location: `internal/shared/types/message.go`
  - Fields: Type, Name, Path, Content

- [ ] 1.1.5 Define `PermissionRequest` struct
  - Location: `internal/shared/types/permission.go`
  - Fields: ID, SessionID, ToolName, Description, InputPreview, RiskLevel, CreatedAt, ExpiresAt, Status, RespondedAt, Response

- [ ] 1.1.6 Define `RiskLevel` type
  - Location: `internal/shared/types/permission.go`
  - Levels: RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical

- [ ] 1.1.7 Define `PermissionStatus` type
  - Location: `internal/shared/types/permission.go`
  - Status: PermissionStatusPending, PermissionStatusApproved, PermissionStatusDenied, PermissionStatusExpired

- [ ] 1.1.8 Define `CommandType` type
  - Location: `internal/shared/types/command.go`
  - Commands: CommandNew, CommandStop, CommandHelp, CommandUnknown

- [ ] 1.1.9 Define `DomainEvent` types
  - Location: `internal/shared/types/events.go`
  - Events: EventSessionCreated, EventSessionExpired, EventMessageReceived, EventMessageSent, EventPermissionRequested

- [ ] 1.1.10 Define `InboundMessage` struct
  - Location: `internal/shared/types/message.go`
  - From Adapter to Gateway

- [ ] 1.1.11 Define `OutboundMessage` struct
  - Location: `internal/shared/types/message.go`
  - From Gateway to Adapter

### 1.2 Error Definitions

**Owner:** internal/shared/errors

- [ ] 1.2.1 Define sentinel errors
  - Location: `internal/shared/errors/communication.go`
  - Errors: ErrSessionNotFound, ErrSessionExpired, ErrSessionCreateFailed, ErrSessionStore, ErrMessageEmpty, ErrMessageTooLong, ErrMessageInvalidFormat, ErrPermissionDenied, ErrPermissionTimeout, ErrPermissionInvalid, ErrGatewayRoute, ErrGatewayAdapt

- [ ] 1.2.2 Define `SentinelError` struct
  - Location: `internal/shared/errors/communication.go`

- [ ] 1.2.3 Define `ErrSessionNotFound`
  - Code: COMM_SESSION_NOT_FOUND_1001

- [ ] 1.2.4 Define `SessionExpiredError`
  - Code: COMM_SESSION_EXPIRED_1002

- [ ] 1.2.5 Define `MessageEmptyError`
  - Code: COMM_MESSAGE_EMPTY_2001

- [ ] 1.2.6 Define `PermissionDeniedError`
  - Code: COMM_PERMISSION_DENIED_3001

- [ ] 1.2.7 Define `PermissionTimeoutError`
  - Code: COMM_PERMISSION_TIMEOUT_3002

**Quality Gate:** All type definitions compile without errors

---

## Phase 2: Configuration

### 2.1 Communication Config

**Owner:** shared/config

- [ ] 2.1.1 Define communication config interface
  - Location: `internal/shared/config/communication.go`
  - Session: idleTimeoutMs (30min), storageDir (~/.devrix/sessions), maxSessions (1000)

- [ ] 2.1.2 Define permission config
  - Location: `internal/shared/config/communication.go`
  - DefaultTimeoutMs (60s), maxRetries (3)

- [ ] 2.1.3 Define CLI config
  - Location: `internal/shared/config/communication.go`
  - WelcomeMessage, prompt, ANSI colors

- [ ] 2.1.4 Define commands config
  - Location: `internal/shared/config/communication.go`
  - Prefix: '/', list: ['new', 'stop', 'help']

- [ ] 2.1.5 Implement ConfigLoader
  - Location: `internal/shared/config/config.go`
  - Load from JSON + env override

**Quality Gate:** Config loads correctly in tests

---

## Phase 3: Session Store

### 3.1 FileSessionStore Implementation

**Owner:** layers/communication/gateway

- [ ] 3.1.1 Create ISessionStore interface
  - Location: `internal/layers/communication/gateway/store.go`
  - Methods: create, get, update, delete, list, getIdleSessions

- [ ] 3.1.2 Create FileSessionStore class
  - Location: `internal/layers/communication/gateway/store.go`
  - Implements ISessionStore

- [ ] 3.1.3 Implement `create(session)` method
  - Persist to ~/.devrix/sessions/{sessionId}.json
  - Emit 'session.created' event

- [ ] 3.1.4 Implement `get(sessionId)` method
  - Load from file if exists
  - Return null if not found

- [ ] 3.1.5 Implement `update(session)` method
  - Update lastMessageAt
  - Re-persist to file

- [ ] 3.1.6 Implement `delete(sessionId)` method
  - Remove session file

- [ ] 3.1.7 Implement `list()` method
  - List all session files in storageDir

- [ ] 3.1.8 Implement `getIdleSessions(timeoutMs)` method
  - Find sessions idle longer than timeout

### 3.2 Session State Machine

**Owner:** layers/communication/gateway

- [ ] 3.2.1 Implement session state transitions
  - States: idle, thinking, streaming, tool_executing, waiting_permission, completed, failed
  - Transitions managed by explicit SetState() method

- [ ] 3.2.2 Implement idle timeout detection
  - Check on each message
  - Mark session as 'expired' if idle > 30 minutes
  - Emit 'session.expired' event

**Quality Gate:** Unit tests pass for all session store operations

---

## Phase 4: Communication Gateway

### 4.1 Gateway Core

**Owner:** layers/communication/gateway

- [ ] 4.1.1 Create ICommunicationGateway interface
  - Location: `internal/layers/communication/gateway/gateway.go`
  - Methods: routeInbound, routeOutbound, routePermission, routeError, createSession, getSession, expireSession

- [ ] 4.1.2 Create CommunicationGateway class
  - Location: `internal/layers/communication/gateway/gateway.go`
  - Constructor: takes ISessionStore, ITraceEmitter, IPermissionManager, IContextEngine

- [ ] 4.1.3 Implement `routeInbound(message)` method
  - Validate message (non-empty string)
  - Get or create session
  - Emit 'message.received' event
  - Forward to context engine

- [ ] 4.1.4 Implement `routeOutbound(event)` method
  - Emit 'message.sent' event
  - Send to CLI adapter renderer

- [ ] 4.1.5 Implement `routePermission(request)` method
  - Emit 'permission.requested' event
  - Wait for user response (async)
  - Return boolean

- [ ] 4.1.6 Implement `routeError(error, sessionId)` method
  - Format error message
  - Send to CLI adapter

- [ ] 4.1.7 Implement `createSession(chatId, workDir)` method
  - Delegate to sessionStore.create()
  - Emit 'session.created' event

- [ ] 4.1.8 Implement `getSession(sessionId)` method
  - Delegate to sessionStore.get()

- [ ] 4.1.9 Implement `expireSession(sessionId)` method
  - Mark session as expired
  - Emit 'session.expired' event

### 4.2 Permission Manager

**Owner:** layers/communication/gateway

- [ ] 4.2.1 Create IPermissionManager interface
  - Location: `internal/layers/communication/gateway/permission.go`

- [ ] 4.2.2 Create PermissionManager class
  - Location: `internal/layers/communication/gateway/permission.go`
  - Handle permission request lifecycle

- [ ] 4.2.3 Implement `request(sessionId, toolName, args, riskLevel)` method
  - Create PermissionRequest
  - Set expiresAt = now + 60s
  - Emit 'permission.requested' event
  - Wait for user response

- [ ] 4.2.4 Implement `resolve(requestId, allowed)` method
  - Update request status
  - Emit 'permission.responded' (internal)

- [ ] 4.2.5 Implement timeout handling
  - Auto-deny after 60s
  - Emit 'permission.expired' (internal)

### 4.3 Input Validator

**Owner:** layers/communication/gateway

- [ ] 4.3.1 Create IInputValidator interface
  - Location: `internal/layers/communication/gateway/validate.go`

- [ ] 4.3.2 Implement message validation
  - Non-empty string
  - Max length check (64000 chars)

**Quality Gate:** Gateway unit tests pass

---

## Phase 5: CLI Adapter

### 5.1 CLI Adapter Core

**Owner:** layers/communication/adapters

- [ ] 5.1.1 Create ICLIAdapter interface
  - Location: `internal/layers/communication/adapters/cli.go`
  - Methods: start, send, onMessage, onPermissionRequest, stop

- [ ] 5.1.2 Create CLIAdapter class
  - Location: `internal/layers/communication/adapters/cli.go`
  - Implements ICLIAdapter

- [ ] 5.1.3 Implement `start()` method
  - Initialize readline interface
  - Show welcome message (from config)
  - Create initial session
  - Start listening for input

- [ ] 5.1.4 Implement `send(message)` method
  - Send user message to gateway

- [ ] 5.1.5 Implement `onMessage(handler)` method
  - Register callback for incoming messages

- [ ] 5.1.6 Implement `onPermissionRequest(handler)` method
  - Register permission request handler

- [ ] 5.1.7 Implement `stop()` method
  - Close readline
  - Clean up resources

### 5.2 CLI Renderer

**Owner:** layers/communication/renderers

- [ ] 5.2.1 Create ICLIRenderer interface
  - Location: `internal/layers/communication/renderers/message.go`

- [ ] 5.2.2 Create CLIRenderer class
  - Location: `internal/layers/communication/renderers/message.go`

- [ ] 5.2.3 Implement ANSI colors
  - User messages: blue (\x1b[34m)
  - Assistant messages: green (\x1b[32m)
  - Errors: red (\x1b[31m)
  - Warning: yellow (\x1b[33m)
  - Reset: \x1b[0m

- [ ] 5.2.4 Implement streaming renderer
  - Show streaming text incrementally
  - Replace partial with final
  - Clear line on update

- [ ] 5.2.5 Implement status renderer
  - Location: `internal/layers/communication/renderers/status.go`
  - Show session state: idle, thinking, streaming, etc.

- [ ] 5.2.6 Implement permission card renderer
  - Location: `internal/layers/communication/renderers/permission.go`
  - Show tool name and risk level
  - Show command to execute
  - Prompt for yes/no

**Quality Gate:** CLI works interactively in terminal

---

## Phase 6: Command Handler

### 6.1 Command Parser

**Owner:** layers/communication

- [ ] 6.1.1 Create ICommandHandler interface
  - Location: `internal/layers/communication/commands/handler.go`
  - Methods: parse, execute

- [ ] 6.1.2 Create CommandHandler class
  - Location: `internal/layers/communication/commands/handler.go`
  - Implements ICommandHandler

- [ ] 6.1.3 Implement `parse(input)` method
  - Check if input starts with '/'
  - Extract command name
  - Return CommandType

- [ ] 6.1.4 Implement `isCommand(input)` method
  - Returns true if input starts with '/'

### 6.2 Command Execution

**Owner:** layers/communication

- [ ] 6.2.1 Implement `execute(command, context)` method
  - Route to specific command handler

- [ ] 6.2.2 Implement `/help` command
  - Return help text
  - Location: `internal/layers/communication/commands/help.go`
  - Commands: /new, /stop, /help

- [ ] 6.2.3 Implement `/new` command
  - Terminate current session
  - Create new session
  - Location: `internal/layers/communication/commands/new.go`

- [ ] 6.2.4 Implement `/stop` command
  - Cancel current LLM call
  - Preserve partial response
  - Location: `internal/layers/communication/commands/stop.go`

### 6.3 Command Integration

**Owner:** layers/communication

- [ ] 6.3.1 Integrate command handler with CLI adapter
  - Parse commands before sending to gateway
  - Execute command if valid command string

- [ ] 6.3.2 Create command registry
  - Location: `internal/layers/communication/commands/commands.go`
  - Register all commands

**Quality Gate:** All commands work correctly in CLI

---

## Phase 7: Four Flow Implementation

### 7.1 Instruction Flow

**Owner:** layers/communication

- [ ] 7.1.1 Implement instruction flow handler
  - Route /new, /stop, /help commands
  - Emit relevant events

### 7.2 Event Flow

**Owner:** layers/communication

- [ ] 7.2.1 Implement event emission
  - thinking → loading animation
  - text → streaming text
  - tool_call → tool block
  - tool_result → result display
  - permission → permission card
  - status → status badge
  - complete → completion summary
  - error → error display

### 7.3 Information Flow

**Owner:** layers/communication

- [ ] 7.3.1 Implement help information
  - Display help on /help command
  - Show available commands

- [ ] 7.3.2 Implement error display
  - Show formatted error messages
  - Show recovery suggestions

- [ ] 7.3.3 Implement status display
  - Show connection status
  - Show session state

### 7.4 Task Flow (Placeholder)

**Owner:** layers/communication

- [ ] 7.4.1 Add V3 placeholder
  - Log: "Task flow (Milestone DAG) not implemented in V1"

**Quality Gate:** All flows work end-to-end

---

## Phase 8: Message Reliability (V1 Simplified)

> **Note:** Heartbeat is not needed for CLI request-response mode. Heartbeat is V2 feature for IM adapters with persistent WebSocket connections.

### 8.1 Message Acknowledgment

**Owner:** layers/communication/gateway

- [ ] 8.1.1 Implement ack for inbound messages
  - Send ack after message received
  - Retry up to 3 times if no ack

**Quality Gate:** Message reliability tests pass

---

## Phase 9: Tests & Documentation

### 9.1 Unit Tests

**Owner:** layers/communication/tests

- [ ] 9.1.1 Test FileSessionStore
  - create, get, update, delete, list
  - Idle timeout detection

- [ ] 9.1.2 Test CommunicationGateway
  - routeInbound validation
  - routeOutbound streaming
  - routePermission flow

- [ ] 9.1.3 Test PermissionManager
  - request creation
  - resolve (grant/deny)
  - timeout handling

- [ ] 9.1.4 Test CommandHandler
  - parse /help, /new, /stop
  - Execute each command

- [ ] 9.1.5 Test CLIAdapter (mocked readline)
  - start, send, onMessage
  - Permission request handling

- [ ] 9.1.6 Test CLIRenderer
  - ANSI color output
  - Streaming render

- [ ] 9.1.7 Test InputValidator
  - Empty message rejection
  - Max length validation

### 9.2 Integration Tests

**Owner:** layers/communication/tests

- [ ] 9.2.1 Test CLI → Gateway → Session flow
- [ ] 9.2.2 Test message → context engine → response flow
- [ ] 9.2.3 Test permission request → user response → tool execution flow
- [ ] 9.2.4 Test /new command creates new session
- [ ] 9.2.5 Test /stop command cancels LLM call
- [ ] 9.2.6 Test /help command displays help
- [ ] 9.2.7 Test idle timeout expires session

### 9.3 Documentation

**Owner:** layers/communication

- [ ] 9.3.1 Add JSDoc to all public classes
- [ ] 9.3.2 Add JSDoc to all public methods
- [ ] 9.3.3 Document error codes in README
- [ ] 9.3.4 Document CLI usage

**Quality Gate:**
- All tests pass
- Coverage ≥ 80%
- No critical code analysis issues
- JSDoc complete

---

## Completion Checklist

- [ ] All 58 tasks complete
- [ ] All tests pass
- [ ] Coverage ≥ 80%
- [ ] No critical code analysis issues
- [ ] JSDoc complete
- [ ] Ready for layer integration

---

## File Checklist

```
devrix/
├── cmd/
│   └── devrix/
│       └── main.go                      # Entry point
│
├── internal/
│   └── layers/
│       └── communication/
│           ├── gateway/
│           │   ├── gateway.go           # CommunicationGateway
│           │   ├── gateway_test.go
│           │   ├── store.go           # FileSessionStore
│           │   ├── store_test.go
│           │   ├── permission.go      # PermissionManager
│           │   ├── permission_test.go
│           │   └── validate.go        # InputValidator
│           ├── adapters/
│           │   ├── cli.go             # CLIAdapter
│           │   └── cli_test.go
│           ├── renderers/
│           │   ├── message.go        # MessageRenderer
│           │   ├── status.go         # StatusRenderer
│           │   └── permission.go      # PermissionRenderer
│           └── commands/
│               ├── help.go            # /help command
│               ├── new.go              # /new command
│               └── stop.go            # /stop command
│           └── shared/
│               ├── types/
│               │   ├── session.go
│               │   ├── message.go
│               │   ├── permission.go
│               │   ├── command.go
│               │   └── events.go
│               ├── errors/
│               │   └── communication.go
│               └── config/
│                   └── communication.go
│
└── pkg/
    └── i18n/
        ├── i18n.go
        └── i18n_test.go
```

**Note:** Go 测试文件使用 `_test.go` 后缀，与源文件同目录

**Quality Gate:**
- `go test ./...` passes
- Coverage ≥ 80%
- `go vet` and `staticcheck` clean
- `gofmt` formatted

---

## V1 Feature Summary

| Feature | Status | Notes |
|---------|--------|-------|
| Session management | ✅ | FileSessionStore, 30min idle timeout |
| CLI Adapter | ✅ | readline + ANSI |
| Command handler | ✅ | /new, /stop, /help |
| Permission pipeline | ✅ | 60s timeout |
| Streaming response | ✅ | ANSI incremental render |
| Instruction flow | ✅ | Commands handled |
| Event flow | ✅ | Core events emitted |
| Information flow | ✅ | Help, error, status |
| Task flow | ❌ | V3 feature |
| ShortId | ❌ | V2 feature |
| IM Adapter | ❌ | V2 feature |
| Heartbeat | ❌ | V2 feature (IM Adapter WebSocket) |
| Auth | ❌ | V2 feature |
