# Delta: Communication Layer V2 - Reliability Enhancement

**Change ID:** devrix-v2
**Affects:** communication layer, session management, adapters, auth
**Based on:** devrix-foundation (V1)

---

## ADDED

### Requirement: ShortId Generator

5位短ID生成，用于友好展示，防脏话。

#### Scenario: Generate ShortId for session
- GIVEN new session is created
- WHEN GenerateShortId is called
- THEN 5-character string is returned
- AND charset excludes ambiguous characters (I, O)

#### Scenario: ShortId uniqueness
- GIVEN multiple sessions created
- WHEN ShortIds are generated
- THEN each ShortId is unique (collision probability < 0.001%)

```
ShortId Format: [0-9][A-Z][A-Z][0-9][A-Z]
Example: 7K2T9, 3MWB7
```

---

### Requirement: Auth Middleware

Adapter 身份验证机制。

#### Scenario: Adapter registration with secret
- GIVEN adapter connects to gateway
- WHEN adapter sends register request with shared secret
- THEN secret is validated
- AND JWT token is issued if valid
- AND token is returned to adapter

#### Scenario: Authenticated request
- GIVEN adapter has valid token
- WHEN adapter sends message with token
- THEN token is validated
- AND request is processed if valid
- AND 401 is returned if invalid

#### Scenario: Token expiration
- GIVEN adapter uses expired token
- WHEN request is made
- THEN 401 Unauthorized is returned
- AND adapter must re-authenticate

```
Token Format: JWT
Claims: adapter_id, issued_at, expires_at
Expiry: 24 hours
```

---

### Requirement: IM Adapter (Feishu)

飞书消息平台适配器，支持远程控制。

#### Scenario: Feishu WebSocket connection
- GIVEN Feishu adapter starts
- WHEN Connect is called
- THEN WebSocket connection is established
- AND bot info is fetched
- AND event dispatcher is registered

#### Scenario: Receive Feishu message
- GIVEN user sends message in Feishu
- WHEN message event is received
- THEN message content is extracted
- AND session is created/restored
- AND message is routed to gateway

#### Scenario: Send response to Feishu
- GIVEN gateway sends outbound message
- WHEN SendMessage is called
- THEN message is sent to Feishu user
- AND message ID is returned

#### Scenario: Feishu permission card
- GIVEN permission request from engine
- WHEN permission card is rendered
- THEN interactive card is sent to Feishu
- AND user approval triggers callback

---

### Requirement: Heartbeat & Connection Events

WebSocket 连接保活和断开检测。

#### Scenario: Send heartbeat ping
- GIVEN WebSocket connection is active
- WHEN heartbeat interval (30s) elapses
- THEN ping is sent to remote
- AND pong response is expected

#### Scenario: Detect connection lost
- GIVEN WebSocket connection is active
- WHEN no pong received within 60s
- THEN connection is considered lost
- AND connection.lost event is emitted
- AND reconnection is attempted

#### Scenario: Connection restored
- GIVEN connection.lost was emitted
- WHEN reconnection succeeds
- THEN connection.restored event is emitted
- AND adapter resumes normal operation

---

### Requirement: Complete Domain Events

V2 完整领域事件定义。

#### Scenario: Emit connection.lost
- GIVEN heartbeat times out
- WHEN connection is detected as lost
- THEN event 'connection.lost' is emitted
- AND reconnection timer starts

#### Scenario: Emit connection.restored
- GIVEN connection.lost was emitted
- WHEN reconnection succeeds
- THEN event 'connection.restored' is emitted
- AND normal operation resumes

#### Scenario: Emit permission.responded
- GIVEN user responds to permission request
- WHEN response is received
- THEN event 'permission.responded' is emitted
- AND request status is updated

#### Scenario: Emit permission.expired
- GIVEN permission request is pending
- WHEN 60s timeout elapses
- THEN event 'permission.expired' is emitted
- AND request is auto-denied

---

### Requirement: Rate Limiting

请求限流，防止滥用。

#### Scenario: Rate limit exceeded
- GIVEN adapter sends too many requests
- WHEN request rate > threshold (100/min)
- THEN 429 Too Many Requests is returned
- AND Retry-After header is included

#### Scenario: Rate limit reset
- GIVEN rate limit was exceeded
- WHEN time window passes
- THEN rate limit counter resets
- AND new requests are accepted

---

## MODIFIED

### Modified: Session Entity

新增 ShortId 字段。

```go
type Session struct {
    SessionID    string      // 内部会话 ID
    ShortID      string      // 5位短 ID (V2 新增)
    RequestID    string      // 请求关联 ID
    AdapterID    string      // 所属 Adapter
    // ... existing fields
}
```

### Modified: PermissionRequest Entity

新增完整状态。

```go
type PermissionRequest struct {
    ID          string
    SessionID   string
    ToolName    string
    InputPreview string
    RiskLevel   RiskLevel
    Status      PermissionStatus // V2: 包含 expired 状态
    // ... existing fields
}
```

---

## REMOVED

(None)

---

## V1/V2/V3 Feature Matrix

| Feature | V1 | V2 | V3 |
|---------|-----|-----|-----|
| CLI Adapter | ✅ | ✅ | ✅ |
| **IM Adapter (飞书/钉钉/Telegram)** | ❌ | ✅ | ✅ |
| **WebSocket** | ❌ | ✅ | ✅ |
| **ShortId (5-char)** | ❌ | ✅ | ✅ |
| **Auth (JWT)** | ❌ | ✅ | ✅ |
| **Heartbeat** | ❌ | ✅ | ✅ |
| **connection.lost/restored** | ❌ | ✅ | ✅ |
| FileSessionStore | ✅ | ✅ | ✅ |
| Session idle timeout | ✅ | ✅ | ✅ |
| Command Handler | ✅ | ✅ | ✅ |
| Permission pipeline | ✅ | ✅ | ✅ |
| **Milestone DAG** | ❌ | ❌ | ✅ |
| Task Flow | ✅ | ✅ | ✅ |
| 钉钉 Adapter | ❌ | ❌ | ✅ |
| 限流设计 | ❌ | ✅ | ✅ |

---

## Dependencies

- V2 依赖 V1 的 FileSessionStore、Gateway 核心
- V2 的 Auth 需要 JWT 库：`github.com/golang-jwt/jwt/v5`
- V2 的 Feishu 需要：`github.com/larksuite/oapi-sdk-go/v3`

---

## Backward Compatibility

V2 保持向后兼容：
- V1 Session 仍可使用（ShortID 字段可选）
- CLI Adapter 继续工作，无需 Auth
- API 保持不变
