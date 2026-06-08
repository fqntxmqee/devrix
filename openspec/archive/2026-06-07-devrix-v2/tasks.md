# Communication Layer V2 Tasks

**Change ID:** devrix-v2
**Layer:** 1 - Communication
**Status:** Completed
**Version:** 2.0
**Based on:** delta.md, devrix-foundation design

---

## Task Overview

实现 V2 可靠性增强功能：ShortId、Auth、IM Adapter (Feishu)、Heartbeat、完整事件、限流。

共 **45 个子任务**，分为 6 个阶段。

---

## Phase 1: ShortId Generator

### 1.1 ShortId Types

**Owner:** internal/shared/types

- [ ] 1.1.1 Add `ShortID` field to Session struct
  - Location: `internal/shared/types/session.go`
  - 5-character string

- [ ] 1.1.2 Create ShortId generator function
  - Location: `internal/shared/types/shortid.go`
  - Charset: `0123456789ABCDEFGHJKLMNPQRSTUVWXYZ`
  - No ambiguous characters (I, O)

- [ ] 1.1.3 Add unit tests for ShortId
  - Test uniqueness
  - Test charset validation

---

## Phase 2: Auth Middleware

### 2.1 Auth Types

**Owner:** internal/shared/auth

- [ ] 2.1.1 Define AuthConfig struct
  - Location: `internal/shared/config/auth.go`
  - Fields: Secret, TokenExpiry, Issuer

- [ ] 2.1.2 Define Token struct
  - Location: `internal/shared/types/auth.go`
  - Fields: AdapterID, IssuedAt, ExpiresAt, Token string

- [ ] 2.1.3 Define AuthResult struct
  - Location: `internal/shared/types/auth.go`
  - Fields: Success, Token, Error

### 2.2 Auth Service

**Owner:** internal/layers/communication/auth

- [ ] 2.2.1 Create IAuthService interface
  - Location: `internal/layers/communication/auth/service.go`
  - Methods: Register, Validate, Refresh

- [ ] 2.2.2 Create AuthService implementation
  - Location: `internal/layers/communication/auth/service.go`
  - JWT-based token management

- [ ] 2.2.3 Implement Register(adapterID, secret)
  - Validate shared secret
  - Issue JWT token
  - Return AuthResult

- [ ] 2.2.4 Implement Validate(token)
  - Parse and verify JWT
  - Return adapter ID if valid
  - Return error if invalid/expired

- [ ] 2.2.5 Add JWT middleware to gateway
  - Location: `internal/layers/communication/gateway/middleware.go`
  - Validate token on each request

### 2.3 Auth Tests

- [ ] 2.3.1 Test Register with valid secret
- [ ] 2.3.2 Test Register with invalid secret
- [ ] 2.3.3 Test Validate with valid token
- [ ] 2.3.4 Test Validate with expired token
- [ ] 2.3.5 Test token refresh

---

## Phase 3: IM Adapter (Feishu) - Completion

### 3.1 Feishu Adapter Refinement

**Owner:** internal/layers/communication/adapters

- [ ] 3.1.1 Complete FeishuAdapter implementation
  - Location: `internal/layers/communication/adapters/feishu.go`
  - Full WebSocket event handling

- [ ] 3.1.2 Implement SendMessage (text)
  - Send text messages to Feishu

- [ ] 3.1.3 Implement SendCard (interactive)
  - Send permission request cards

- [ ] 3.1.4 Implement ReplyMessage
  - Reply to specific messages

### 3.2 Feishu Event Handling

- [ ] 3.2.1 Handle message.received event
  - Extract text content
  - Route to gateway

- [ ] 3.2.2 Handle card.action.trigger callback
  - Process permission approval/denial

- [ ] 3.2.3 Handle message.recalled event
  - Mark message as recalled

### 3.3 Feishu Renderer

**Owner:** internal/layers/communication/renderers

- [ ] 3.3.1 Create FeishuCardRenderer
  - Location: `internal/layers/communication/renderers/feishu_card.go`
  - Render permission request cards

- [ ] 3.3.2 Implement thinking card
  - Show "思考中..." state

- [ ] 3.3.3 Implement streaming card
  - Show incremental text

- [ ] 3.3.4 Implement done card
  - Show completion state

---

## Phase 4: Heartbeat & Connection Events

### 4.1 Connection Manager

**Owner:** internal/layers/communication/connection

- [ ] 4.1.1 Create IConnectionManager interface
  - Location: `internal/layers/communication/connection/manager.go`
  - Methods: Register, Unregister, Heartbeat, GetStatus

- [ ] 4.1.2 Create ConnectionManager implementation
  - Track all active connections
  - Monitor heartbeat status

- [ ] 4.1.3 Implement Register(connection)
  - Add connection to tracking
  - Start heartbeat monitor

- [ ] 4.1.4 Implement Heartbeat(connectionID)
  - Reset timeout counter
  - Update last_seen timestamp

- [ ] 4.1.5 Implement connection lost detection
  - 60s timeout triggers lost event
  - Start reconnection attempts

### 4.2 Connection Events

- [ ] 4.2.1 Emit connection.lost event
  - When heartbeat timeout detected
  - Include connection ID and reason

- [ ] 4.2.2 Emit connection.restored event
  - When reconnection succeeds
  - Include connection ID

### 4.3 Reconnection Logic

- [ ] 4.3.1 Implement exponential backoff
  - Initial: 1s, Max: 60s
  - Max attempts: 10

- [ ] 4.3.2 Implement reconnection handler
  - Attempt reconnect on connection.lost
  - Notify adapter on success/failure

---

## Phase 5: Complete Domain Events

### 5.1 Event Types

**Owner:** internal/shared/types

- [ ] 5.1.1 Define EventConnectionLost
  - Location: `internal/shared/types/events.go`
  - Fields: ConnectionID, AdapterID, Reason, Timestamp

- [ ] 5.1.2 Define EventConnectionRestored
  - Location: `internal/shared/types/events.go`
  - Fields: ConnectionID, AdapterID, Timestamp

- [ ] 5.1.3 Define EventPermissionResponded
  - Location: `internal/shared/types/events.go`
  - Fields: RequestID, SessionID, Approved, ResponseTime

- [ ] 5.1.4 Define EventPermissionExpired
  - Location: `internal/shared/types/events.go`
  - Fields: RequestID, SessionID, ExpiredAt

### 5.2 Event Emission

**Owner:** internal/layers/communication/gateway

- [ ] 5.2.1 Emit connection.lost on heartbeat timeout
- [ ] 5.2.2 Emit connection.restored on successful reconnect
- [ ] 5.2.3 Emit permission.responded on user decision
- [ ] 5.2.4 Emit permission.expired on timeout

---

## Phase 6: Rate Limiting

### 6.1 Rate Limiter

**Owner:** internal/layers/communication/ratelimit

- [ ] 6.1.1 Create IRateLimiter interface
  - Location: `internal/layers/communication/ratelimit/limiter.go`
  - Methods: Allow, Reset

- [ ] 6.1.2 Create RateLimiter implementation
  - Token bucket algorithm
  - Per-adapter limits

- [ ] 6.1.3 Configure rate limits
  - Default: 100 requests/minute
  - Configurable per adapter

### 6.2 Rate Limit Response

- [ ] 6.2.1 Return 429 Too Many Requests
  - When rate limit exceeded
  - Include Retry-After header

- [ ] 6.2.2 Implement rate limit middleware
  - Check before processing request
  - Increment counter on each request

---

## Quality Gates

- [ ] All 45 tasks complete
- [ ] All tests pass
- [ ] Coverage ≥ 80%
- [ ] No critical code analysis issues
- [ ] go vet and staticcheck clean

---

## File Checklist

```
devrix/
├── internal/
│   ├── shared/
│   │   ├── types/
│   │   │   ├── shortid.go        # V2: ShortId generator
│   │   │   ├── auth.go          # V2: Auth types
│   │   │   └── events.go        # V2: Complete events
│   │   └── config/
│   │       └── auth.go          # V2: Auth config
│   └── layers/
│       └── communication/
│           ├── auth/
│           │   ├── service.go   # V2: Auth service
│           │   └── middleware.go # V2: JWT middleware
│           ├── adapters/
│           │   ├── feishu.go    # V2: Complete Feishu adapter
│           │   └── feishu_test.go
│           ├── renderers/
│           │   └── feishu_card.go # V2: Feishu card renderer
│           ├── connection/
│           │   ├── manager.go   # V2: Connection manager
│           │   └── events.go    # V2: Connection events
│           └── ratelimit/
│               ├── limiter.go   # V2: Rate limiter
│               └── middleware.go # V2: Rate limit middleware
```

---

## Completion Checklist

- [ ] All 45 tasks complete
- [ ] All tests pass
- [ ] Coverage ≥ 80%
- [ ] No critical code analysis issues
- [ ] Ready for V3 development
