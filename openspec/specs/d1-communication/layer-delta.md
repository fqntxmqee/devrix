# Delta: Domain D1 (COMM)

**Change ID:** devrix-foundation
**Affects:** communication layer, session management, CLI adapter

---

## ADDED

### Requirement: CLI Adapter

V1 CLI adapter for command-line interaction with ANSI rendering.

#### Scenario: Start CLI session
- GIVEN user runs `devrix` command
- WHEN CLI adapter initializes
- THEN session is created with unique requestId
- AND welcome message is displayed

#### Scenario: Send message via CLI
- GIVEN user is in active CLI session
- WHEN user types message and presses Enter
- THEN message is sent to Communication Gateway
- AND session.lastMessageAt is updated

#### Scenario: Receive streaming response
- GIVEN CLI session is active
- WHEN LLM streams a response
- THEN text is rendered incrementally via ANSI
- AND final response replaces streaming text

---

### Requirement: Session Store

File-based session persistence for V1.

#### Scenario: Create new session
- GIVEN no existing session for requestId
- WHEN CreateSession is called
- THEN new Session object is created with:
  - requestId, createdAt, lastMessageAt, messages[]
  - status: 'active'
- AND session is persisted to file

#### Scenario: Restore existing session
- GIVEN session file exists for requestId
- WHEN RestoreSession is called
- THEN session data is loaded from file
- AND session status is 'active'

#### Scenario: Session idle timeout
- GIVEN session.lastMessageAt is older than 30 minutes
- WHEN any message is received
- THEN session is marked as 'expired'
- AND new session is created

---

### Requirement: Communication Gateway

Central message routing hub.

#### Scenario: Route inbound message
- GIVEN message received from CLI
- WHEN RouteInbound is called
- THEN message is validated (non-empty, string)
- AND session is created/restored
- AND message is forwarded to Context Engine

#### Scenario: Route outbound streaming
- GIVEN streaming chunk from LLM
- WHEN RouteOutbound is called
- THEN chunk is sent to appropriate adapter (CLI)
- AND CLI adapter renders the chunk

#### Scenario: Route permission request
- GIVEN tool execution requires user permission
- WHEN RoutePermission is called
- THEN permission request is sent to CLI
- AND execution pauses until user response

---

### Requirement: Command Handler

CLI command parsing (/new, /stop, /help).

#### Scenario: Parse /help command
- GIVEN user input starts with "/help"
- WHEN ParseCommand is called
- THEN help text is returned
- AND no session interaction occurs

#### Scenario: Parse /new command
- GIVEN user input starts with "/new"
- WHEN ParseCommand is called
- THEN current session is terminated
- AND new session is created

#### Scenario: Parse /stop command
- GIVEN user input starts with "/stop"
- WHEN ParseCommand is called
- THEN current LLM call is cancelled
- AND partial response is preserved

---

### Requirement: Four Flow Support

V1 implements instruction, event, and information flows.

#### Scenario: Instruction flow
- GIVEN user sends "/new" command
- WHEN command is parsed
- THEN instruction flow handles it
- AND new session is created

#### Scenario: Event flow - streaming text
- GIVEN LLM streams text response
- WHEN RouteOutbound processes chunk
- THEN event flow delivers to CLI adapter
- AND CLI adapter renders incrementally

#### Scenario: Information flow - error display
- GIVEN error occurs in processing
- WHEN error is caught
- THEN error message is sent via info flow
- AND CLI adapter displays error badge

---

### Requirement: Domain Events (V1 Simplified)

Core events for layer coordination.

#### Scenario: Emit session.created
- GIVEN new session is created
- WHEN CreateSession completes
- THEN event 'session.created' is emitted
- AND traceId is generated

#### Scenario: Emit session.expired
- GIVEN session.idleTime > 30 minutes
- WHEN ExpireSession is triggered
- THEN event 'session.expired' is emitted
- AND session is marked 'expired'

#### Scenario: Emit message.received
- GIVEN user message arrives
- WHEN RouteInbound processes it
- THEN event 'message.received' is emitted
- AND lastMessageAt is updated

#### Scenario: Emit message.sent
- GIVEN response is about to be sent
- WHEN RouteOutbound delivers it
- THEN event 'message.sent' is emitted
- AND adapter renders it

#### Scenario: Emit permission.requested
- GIVEN tool requires permission
- WHEN RoutePermission is called
- THEN event 'permission.requested' is emitted
- AND consumer waits for user response

---

### Requirement: V1/V2/V3 Version Differences

| Feature | V1 | V2 | V3 |
|---------|-----|-----|-----|
| CLI Adapter | ✅ | ✅ | ✅ |
| IM Adapter (飞书/钉钉/Telegram) | ❌ | ✅ | ✅ |
| WebSocket | ❌ | ✅ | ✅ |
| FileSessionStore | ✅ | ✅ | ✅ |
| Session idle timeout (30min) | ✅ | ✅ | ✅ |
| Command Handler (/new, /stop, /help) | ✅ | ✅ | ✅ |
| ShortId (5-char permission) | ❌ | ✅ | ✅ |
| Milestone DAG | ✅ | ✅ | ✅ |
| Task Flow (simplified) | ✅ | ✅ | ✅ |
| Instruction Flow | ✅ | ✅ | ✅ |
| Event Flow | ✅ | ✅ | ✅ |
| Information Flow | ✅ | ✅ | ✅ |

---

## MODIFIED

(None - initial layer specification)

---

## REMOVED

(None - V1 intentionally excludes future features, no removals)
