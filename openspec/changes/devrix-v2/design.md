# Communication Layer V2 Design - Reliability Enhancement

**Change ID:** devrix-v2
**Layer:** 1 - Communication
**Status:** Draft
**Version:** 2.0

---

## 一、架构目标

### V2 业务目标

| 业务目标 | V1 | V2 |
|---------|-----|-----|
| **ShortId** | ❌ | ✅ 5位短ID |
| **Auth** | ❌ | ✅ JWT认证 |
| **IM Adapter** | CLI only | ✅ 飞书/Telegram |
| **Heartbeat** | ❌ | ✅ 60s检测 |
| **完整事件** | 简化版 | ✅ 全部11个事件 |

---

## 二、ShortId 设计

### 2.1 字符集

```
Charset: 0123456789ABCDEFGHJKLMNPQRSTUVWXYZ
Length: 5 characters
Excluded: I, O (易混淆)
```

### 2.2 生成算法

```go
// internal/shared/types/shortid.go

const charset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func GenerateShortId() string {
    b := make([]byte, 5)
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    for i := range b {
        b[i] = charset[r.Intn(len(charset))]
    }
    return string(b)
}
```

### 2.3 碰撞概率

5位字符集 32^5 = 33,554,432 可能的 ShortId
碰撞概率 < 0.001% (1000 并发会话)

---

## 三、Auth 设计

### 3.1 Token 格式

```go
type Token struct {
    AdapterID  string    // 适配器 ID
    IssuedAt   time.Time // 签发时间
    ExpiresAt  time.Time // 过期时间
    Token      string    // JWT token
}

type Claims struct {
    AdapterID  string `json:"adapter_id"`
    jwt.RegisteredClaims
}
```

### 3.2 JWT 配置

```go
type AuthConfig struct {
    Secret       string        // 共享密钥
    TokenExpiry  time.Duration // 24 hours
    Issuer       string        // "devrix"
    SigningKey   []byte        // HMAC-SHA256 key
}
```

### 3.3 认证流程

```
Adapter → Gateway: POST /auth/register { adapter_id, secret }
Gateway: 验证 secret → 签发 JWT
Gateway → Adapter: { token, expires_at }

Adapter → Gateway: 请求 + Authorization: Bearer {token}
Gateway: 验证 JWT → 处理请求 / 返回 401
```

### 3.4 中间件

```go
// gateway/middleware.go

func AuthMiddleware(gw *Gateway) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if token == "" {
                http.Error(w, "Unauthorized", 401)
                return
            }
            // Validate token...
            // Set adapter_id in context
        })
    }
}
```

---

## 四、IM Adapter (Feishu) 设计

### 4.1 Feishu Adapter 架构

```
┌─────────────────────────────────────────────────────┐
│                   FeishuAdapter                       │
├─────────────────────────────────────────────────────┤
│  client: *lark.Client                               │
│  wsClient: *larkws.Client                          │
│  dispatcher: *dispatcher.EventDispatcher           │
│  sessionMap: sync.Map (sessionKey → sessionID)      │
├─────────────────────────────────────────────────────┤
│  Start(ctx)                                        │
│    ├─ fetchBotInfo() → 获取 bot open_id            │
│    ├─ createEventDispatcher() → 注册事件处理器      │
│    └─ startWebSocketMode() / startWebhookMode()    │
│                                                      │
│  onMessage(ctx, event)                            │
│    ├─ extractMessageContent() → 解析 text          │
│    ├─ buildSessionKey() → feishu_{chat}_{user}    │
│    ├─ getOrCreateSession()                         │
│    └─ gateway.RouteInbound()                      │
│                                                      │
│  SendMessage(ctx, chatID, content)                │
│    └─ client.Im.Message.Create()                   │
│                                                      │
│  SendCard(ctx, chatID, card)                      │
│    └─ client.Im.Message.Create(msgType=interactive)│
└─────────────────────────────────────────────────────┘
```

### 4.2 事件流

```
Feishu User → 飞书服务器 → WebSocket → FeishuAdapter.onMessage()
    → gateway.RouteInbound() → Context Engine

Context Engine → gateway.RouteOutbound() → FeishuAdapter
    → FeishuCardRenderer → Feishu User
```

### 4.3 权限卡片

```json
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": {"tag": "plain_text", "content": "权限请求"},
      "template": "warning"
    },
    "elements": [
      {
        "tag": "div",
        "content": "**工具**: bash\n**操作**: ls -la\n**风险等级**: MEDIUM"
      },
      {
        "actions": [
          {"tag": "button", "text": {"tag": "plain_text", "content": "允许"}, "type": "primary"},
          {"tag": "button", "text": {"tag": "plain_text", "content": "拒绝"}, "type": "danger"}
        ]
      }
    ]
  }
}
```

---

## 五、Heartbeat 设计

### 5.1 连接管理器

```go
// connection/manager.go

type Connection struct {
    ID        string
    AdapterID string
    Type      string // "websocket" | "webhook"
    Status    string // "connected" | "disconnected"
    LastSeen  time.Time
    heartbeat *time.Timer
}

type ConnectionManager struct {
    mu          sync.RWMutex
    connections map[string]*Connection
    timeout     time.Duration // 60s
}

func (m *ConnectionManager) Register(conn *Connection) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.connections[conn.ID] = conn
    m.startHeartbeat(conn)
}

func (m *ConnectionManager) Heartbeat(connID string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if conn, ok := m.connections[connID]; ok {
        conn.LastSeen = time.Now()
        m.resetHeartbeat(conn)
    }
}
```

### 5.2 断连检测

```go
func (m *ConnectionManager) startHeartbeat(conn *Connection) {
    conn.heartbeat = time.AfterFunc(m.timeout, func() {
        m.mu.Lock()
        defer m.mu.Unlock()
        if time.Since(conn.LastSeen) > m.timeout {
            m.emitConnectionLost(conn)
            m.scheduleReconnect(conn)
        }
    })
}
```

### 5.3 重连策略

```go
// 指数退避重连
const (
    initialInterval = 1 * time.Second
    maxInterval     = 60 * time.Second
    maxAttempts     = 10
)

func (m *ConnectionManager) scheduleReconnect(conn *Connection) {
    interval := initialInterval
    for i := 0; i < maxAttempts; i++ {
        time.Sleep(interval)
        if m.attemptReconnect(conn) {
            m.emitConnectionRestored(conn)
            return
        }
        interval *= 2
        if interval > maxInterval {
            interval = maxInterval
        }
    }
}
```

---

## 六、完整领域事件

### 6.1 V2 事件列表

| 事件 | 触发条件 | V1 | V2 |
|------|---------|-----|-----|
| session.created | 新建会话 | ✅ | ✅ |
| session.expired | 30min 空闲 | ✅ | ✅ |
| message.received | 收到消息 | ✅ | ✅ |
| message.sent | 发送消息 | ✅ | ✅ |
| permission.requested | 请求权限 | ✅ | ✅ |
| permission.responded | 用户响应 | ⚠️ | ✅ |
| permission.expired | 60s 超时 | ⚠️ | ✅ |
| tool.called | 工具调用 | ❌ | ❌ | V3 |
| connection.lost | 连接断开 | ❌ | ✅ |
| connection.restored | 连接恢复 | ❌ | ✅ |
| milestone.created | 里程碑创建 | ❌ | ❌ | V3 |

### 6.2 事件定义

```go
// events.go

type EventConnectionLost struct {
    BaseEvent
    ConnectionID string
    AdapterID   string
    Reason      string
}

type EventConnectionRestored struct {
    BaseEvent
    ConnectionID string
    AdapterID   string
}

type EventPermissionResponded struct {
    BaseEvent
    RequestID  string
    SessionID  string
    Approved   bool
    ResponseTime time.Time
}

type EventPermissionExpired struct {
    BaseEvent
    RequestID  string
    SessionID  string
    ExpiredAt time.Time
}
```

---

## 七、限流设计

### 7.1 Token Bucket 算法

```go
// ratelimit/limiter.go

type RateLimiter struct {
    mu       sync.Mutex
    tokens   float64
    maxTokens float64
    rate     float64 // tokens per second
    lastUpdate time.Time
}

func (l *RateLimiter) Allow() bool {
    l.mu.Lock()
    defer l.mu.Unlock()

    l.replenish()

    if l.tokens >= 1 {
        l.tokens--
        return true
    }
    return false
}

func (l *RateLimiter) replenish() {
    now := time.Now()
    elapsed := now.Sub(l.lastUpdate).Seconds()
    l.tokens = math.Min(l.maxTokens, l.tokens+elapsed*l.rate)
    l.lastUpdate = now
}
```

### 7.2 限流配置

```go
type RateLimitConfig struct {
    RequestsPerMinute int              // 100
    BurstSize        int              // 10
    Enabled          bool             // true
}
```

### 7.3 HTTP 响应

```go
// 429 Too Many Requests
w.Header().Set("Retry-After", "60")
w.Header().Set("X-RateLimit-Limit", "100")
w.Header().Set("X-RateLimit-Remaining", "0")
http.Error(w, "Rate limit exceeded", 429)
```

---

## 八、项目结构

```
devrix/
├── internal/
│   ├── shared/
│   │   ├── types/
│   │   │   ├── shortid.go        # ShortId 生成器
│   │   │   ├── auth.go          # Auth 类型
│   │   │   └── events.go        # 完整事件定义
│   │   └── config/
│   │       ├── auth.go          # Auth 配置
│   │       └── ratelimit.go     # 限流配置
│   └── layers/
│       └── communication/
│           ├── auth/
│           │   ├── service.go   # Auth 服务实现
│           │   └── middleware.go # JWT 中间件
│           ├── adapters/
│           │   ├── feishu.go    # Feishu 适配器
│           │   └── feishu_test.go
│           ├── renderers/
│           │   └── feishu_card.go # Feishu 卡片渲染器
│           ├── connection/
│           │   ├── manager.go   # 连接管理器
│           │   ├── heartbeat.go # 心跳实现
│           │   └── reconnect.go # 重连实现
│           └── ratelimit/
│               ├── limiter.go   # 限流器
│               └── middleware.go # 限流中间件
```

---

## 九、错误处理

### 9.1 新增错误码

| 错误码 | 说明 |
|--------|------|
| COMM_AUTH_INVALID_SECRET | 无效的共享密钥 |
| COMM_AUTH_TOKEN_EXPIRED | Token 已过期 |
| COMM_AUTH_TOKEN_INVALID | Token 无效 |
| COMM_CONNECTION_LOST | 连接断开 |
| COMM_CONNECTION_TIMEOUT | 连接超时 |
| COMM_RATE_LIMIT_EXCEEDED | 限流触发 |

---

## 十、测试策略

### 10.1 单元测试

- ShortId 生成唯一性测试
- Auth 注册/验证测试
- Heartbeat 超时测试
- RateLimiter Allow/Deny 测试

### 10.2 集成测试

- Feishu WebSocket 连接测试
- Auth 中间件拦截测试
- Connection lost/restored 事件测试

### 10.3 Mock

- Mock Feishu API responses
- Mock WebSocket connections
