# Delta: Domain D1 (COMM)

**Change ID:** devrix-foundation → current
**Affects:** communication layer, session management, all adapters, event bus, milestone, renderers

---

## Current State Summary

D1 通信域已从 V1 基础版本演进为完整的 V2 实现能力。以下记录所有变更：

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

File-based session persistence.

#### Scenario: Create new session
- GIVEN no existing session for requestId
- WHEN CreateSession is called
- THEN new Session object is created with: requestId, createdAt, lastMessageAt, messages[]
- AND status is 'active'
- AND session is persisted to file via atomic write (temp file + rename)

#### Scenario: Restore existing session
- GIVEN session file exists for requestId
- WHEN ResolveSessionByChatID is called
- THEN session data is loaded from file with recency-weighted scoring

#### Scenario: Session idle timeout
- GIVEN session.lastMessageAt is older than configured timeout
- WHEN CleanupRoutine runs
- THEN session is marked as 'expired' and deleted

---

### Requirement: Communication Gateway

Central message routing hub.

#### Scenario: Route inbound message
- GIVEN message received from adapter
- WHEN RouteInbound is called
- THEN message is validated (non-empty, string)
- AND session is created/restored
- AND message is forwarded to Context Engine (via Agent when AgentFactory is set)

#### Scenario: Route outbound streaming
- GIVEN streaming chunk from LLM
- WHEN RouteOutbound is called
- THEN chunk is sent to appropriate adapter (CLI/Feishu/DingTalk)
- AND adapter renders the chunk according to its event handling

#### Scenario: Route permission request
- GIVEN tool execution requires user permission
- WHEN RoutePermission is called
- THEN permission request is sent to adapter
- AND execution pauses until user response (signal-based resolution)

---

### Requirement: Command Handler

CLI and IM command parsing.

#### Scenario: Parse /help command
- GIVEN user input starts with "/help"
- WHEN ParseCommand is called
- THEN help text is returned with all available commands

#### Scenario: Parse /new command
- GIVEN user input starts with "/new"
- WHEN ParseCommand is called
- THEN current session state is preserved
- AND new session is created for subsequent messages

#### Scenario: Parse /stop command
- GIVEN user input starts with "/stop"
- WHEN ParseCommand is called
- THEN current LLM process is cancelled
- AND session mapping is preserved for reuse

---

### Requirement: Feishu Adapter (V2)

Full-featured Feishu/Lark IM adapter.

#### Scenario: WebSocket mode
- GIVEN FeishuConfig with valid AppID/AppSecret
- WHEN FeishuAdapter.Start is called
- THEN WebSocket connection is established via Lark WS SDK
- AND bot receives real-time messages

#### Scenario: Webhook mode (fallback)
- GIVEN FeishuConfig with UseWebhook=true or WebSocket unavailable
- WHEN adapter starts
- THEN HTTP webhook server is started
- AND messages are received via POST callbacks

#### Scenario: CardKit streaming (typewriter effect)
- GIVEN streaming is enabled in config
- WHEN LLM produces text output
- THEN CardKit card is created with CreateCard
- AND content is streamed element-by-element via StreamElementContent
- AND throttling is applied per streamThrottleConfig
- AND on CardKit failure, system falls back to Im.Message.Patch

#### Scenario: Structured progress cards
- GIVEN progressStyle is "structured"
- WHEN engine produces thinking/tool_call/tool_result events
- THEN separate progress cards are maintained (thinking card, tools card, agent output card)

---

### Requirement: DingTalk Adapter (V2)

DingTalk chatbot integration.

#### Scenario: Webhook message routing
- GIVEN DingTalk webhook receives a message
- WHEN webhookHandler processes the payload
- THEN session is resolved via session_resolve
- AND message is deduplicated via dedupMap
- AND outbound replies use DingTalkCardRenderer for milestone rendering

---

### Requirement: Backpressure EventBus (V2)

Priority-aware event bus with lifecycle management.

#### Scenario: Normal event flow
- GIVEN events are published via Publish
- WHEN backlog is below HighWatermark
- THEN events are delivered to all subscribers in order

#### Scenario: Critical event guaranteed delivery
- GIVEN a "complete" or "error" event
- WHEN PublishCritical is called
- THEN event is synchronously fanned out to all subscribers
- AND event is NEVER dropped regardless of backlog

#### Scenario: Drain under backpressure
- GIVEN backlog exceeds HighWatermark
- WHEN bus state transitions to Draining
- THEN Normal/Low events are shed until backlog reaches LowWatermark
- AND Critical events continue to be delivered

#### Scenario: Compact during recovery
- GIVEN backlog is at LowWatermark after Drain
- WHEN Compact is called
- THEN adjacent same-type events are merged into single aggregate
- AND Critical events are never compacted

#### Scenario: Reconnect full cycle
- GIVEN bus needs to be rebuilt
- WHEN Reconnect is called
- THEN Drain → Compact → ChannelRebuilt lifecycle executes
- AND bus returns to Running state

---

### Requirement: Worker Card System (V2)

Per-worker independent streaming cards for orchestration visualization.

#### Scenario: Create worker card
- GIVEN a worker starts executing
- WHEN EmitWorkerEvent is called with WorkerCardOptions
- THEN a new card is created with worker emoji and title
- AND card updates are streamed independently per worker

#### Scenario: Double-block streaming
- GIVEN a worker produces thinking and output
- WHEN thinking and output chunks arrive
- THEN thinking block and output block are updated independently
- AND Snapshot can retrieve current state at any time

---

### Requirement: Agent Routing (V2)

Gateway-level agent binding for L4 multi-agent integration.

#### Scenario: Route via agent
- GIVEN AgentFactory is set on CommunicationGateway
- WHEN RouteInbound is called
- THEN agent is created and bound to session
- AND messages are processed through agent instead of direct engine path
- AND gatewayAgentObserver bridges agent events back to gateway

---

### Requirement: Connection Manager

WebSocket/Webhook connection lifecycle with heartbeat.

#### Scenario: Connection heartbeat timeout
- GIVEN a connection is registered with ConnectionManager
- WHEN heartbeat is not received within timeout
- THEN exponential backoff reconnect is attempted (1s → 60s, max 10 attempts)
- AND OnLost/OnRestored callbacks are triggered

---

## MODIFIED

### Session Resolve Strategy (updated from simple create to 3-tier fallback)

Originally sessions were always created fresh. Now uses:
1. Memory cache lookup (sessionMap)
2. Disk recovery via Gateway.ResolveSessionByChatID
3. Create new session as fallback

### Milestone Service (expanded from basic CRUD)

Added: AddDependency with cycle detection, GetExecutionOrder (topological sort), CalculateOverallProgress, GetMilestonesByTaskID.

### Card System (expanded from basic card)

Added: CardkitClient for streaming, WorkerCardRenderer for orchestration, structured progress cards, DingTalk card renderer, Component interface with 5 implementations.

---

## Current Feature Matrix

| Feature | V1 (Foundation) | V2 (Current) | Notes |
|---------|-----------------|--------------|-------|
| CLI Adapter | ANSI rendering | + task/plan commands | `adapters/cli.go` |
| Feishu Adapter | Webhook only | + WebSocket + CardKit streaming + Worker cards | `adapters/feishu.go` |
| DingTalk Adapter | — | Webhook + Card renderer | `adapters/dingtalk.go` |
| Session Store | File-based (JSON) | + atomic write + ResolveByChatID + CleanupRoutine | `gateway/store.go` |
| Session idle timeout | 30min (configurable) | unchanged | `config.CommunicationConfig.Session` |
| Command Handler | /new, /stop, /help | + /task, /plan | CLI + Feishu |
| Permission System | Basic YOLO | + CRITICAL never auto-approve + timeout metrics + signal-based resolution | `gateway/permission.go` |
| EventBus | — | BackpressureEventBus (Drain/Compact/Reconnect) + Critical guarantee | `eventbus/bus.go` |
| CardKit Streaming | — | Element-level streaming + throttle + fallback to Patch | `adapters/feishu_cardkit.go` |
| Worker Cards | — | Per-worker independent cards + double-block streaming | `adapters/feishu_worker_card.go` |
| Agent Routing | — | Gateway-to-L4 bridge + permission bridging | `gateway/agent_route.go` |
| Connection Manager | — | Heartbeat + exponential backoff reconnect | `connection/manager.go` |
| Rate Limiter | — | Token bucket + HTTP middleware | `ratelimit/limiter.go` |
| Instance Registry | — | In-memory Register/Unregister/HealthCheck | `instance/registry.go` |
| Milestone DAG | Basic CRUD | + cycle detection + topological sort + task flow orchestration | `milestone/` |
| Metrics | Legacy Counter/Gauge | DEPRECATED → D5 observability.Bridge | `metrics/collector.go` |

## REMOVED

(None — deprecated metrics package retained for reference, not removed)
