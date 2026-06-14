package connection

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
)

// Connection 代表一个活动连接
type Connection struct {
	ID        string
	AdapterID string
	Type      string // "websocket" | "webhook"
	Status    string // "connected" | "disconnected"
	LastSeen  time.Time
	Heartbeat *time.Timer
	OnLost    func(*Connection)    // 连接断开回调
	OnRestored func(*Connection)   // 连接恢复回调
}

// ConnectionManager 管理所有活动连接
type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	timeout     time.Duration // 心跳超时时间，默认 60s
	interval    time.Duration // 心跳检查间隔，默认 10s
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewConnectionManager 创建新的连接管理器
func NewConnectionManager(timeout, interval time.Duration) *ConnectionManager {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	if interval == 0 {
		interval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectionManager{
		connections: make(map[string]*Connection),
		timeout:      timeout,
		interval:     interval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Register 注册一个新连接
func (m *ConnectionManager) Register(conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn.Status = "connected"
	conn.LastSeen = time.Now()
	m.connections[conn.ID] = conn

	slog.Info("connection registered",
		"connection_id", conn.ID,
		"adapter_id", conn.AdapterID,
		"type", conn.Type,
	)

	// 启动心跳检测
	m.startHeartbeatMonitor(conn)
}

// Unregister 注销一个连接
func (m *ConnectionManager) Unregister(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[connID]
	if !exists {
		return
	}

	// 停止心跳
	if conn.Heartbeat != nil {
		conn.Heartbeat.Stop()
	}

	delete(m.connections, connID)

	slog.Info("connection unregistered",
		"connection_id", connID,
	)
}

// Heartbeat 更新连接的最近活动时间
func (m *ConnectionManager) Heartbeat(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[connID]
	if !exists {
		return
	}

	conn.LastSeen = time.Now()

	// 重置心跳定时器
	if conn.Heartbeat != nil {
		conn.Heartbeat.Stop()
	}
	m.resetHeartbeat(conn)
}

// Get 获取连接
func (m *ConnectionManager) Get(connID string) (*Connection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[connID]
	return conn, exists
}

// List 返回所有连接
func (m *ConnectionManager) List() []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		result = append(result, conn)
	}
	return result
}

// Count 返回连接数量
func (m *ConnectionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// startHeartbeatMonitor 启动心跳监控
func (m *ConnectionManager) startHeartbeatMonitor(conn *Connection) {
	conn.Heartbeat = time.AfterFunc(m.timeout, func() {
		m.mu.RLock()
		lastSeen := conn.LastSeen
		exists := m.connections[conn.ID] != nil
		m.mu.RUnlock()

		if !exists {
			return
		}

		// 检查是否真的超时
		if time.Since(lastSeen) >= m.timeout {
			m.handleConnectionLost(conn)
		}
	})
}

// resetHeartbeat 重置心跳定时器
func (m *ConnectionManager) resetHeartbeat(conn *Connection) {
	if conn.Heartbeat != nil {
		conn.Heartbeat.Stop()
	}
	conn.Heartbeat = time.AfterFunc(m.timeout, func() {
		m.mu.RLock()
		lastSeen := conn.LastSeen
		exists := m.connections[conn.ID] != nil
		m.mu.RUnlock()

		if !exists {
			return
		}

		if time.Since(lastSeen) >= m.timeout {
			m.handleConnectionLost(conn)
		}
	})
}

// handleConnectionLost 处理连接断开
func (m *ConnectionManager) handleConnectionLost(conn *Connection) {
	m.mu.Lock()
	conn.Status = "disconnected"
	m.mu.Unlock()

	slog.Warn("connection lost",
		"connection_id", conn.ID,
		"adapter_id", conn.AdapterID,
		"last_seen", conn.LastSeen,
	)

	// 触发断开回调
	if conn.OnLost != nil {
		conn.OnLost(conn)
	}

	// 发出领域事件
	event := types.NewDomainEvent(
		types.EventConnectionLost,
		"",
		&types.EventConnectionLostData{
			ConnectionID: conn.ID,
			AdapterID:    conn.AdapterID,
			Reason:       "heartbeat timeout",
		},
	)
	m.emitEvent(event)

	// 尝试重连
	go m.attemptReconnect(conn)
}

// attemptReconnect 尝试重连
func (m *ConnectionManager) attemptReconnect(conn *Connection) {
	const (
		initialInterval = 1 * time.Second
		maxInterval     = 60 * time.Second
		maxAttempts     = 10
	)

	interval := initialInterval

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(interval):
		}

		slog.Info("attempting reconnection",
			"connection_id", conn.ID,
			"attempt", i+1,
		)

		// 模拟重连尝试
		// 实际实现中，这里会调用 adapter 的重连方法
		if m.tryReconnect(conn) {
			m.handleConnectionRestored(conn)
			return
		}

		// 增加间隔
		interval *= 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}

	slog.Error("reconnection failed after max attempts",
		"connection_id", conn.ID,
		"max_attempts", maxAttempts,
	)
}

// tryReconnect 尝试单个重连
// 在实际实现中，这里会调用具体 adapter 的重连逻辑
func (m *ConnectionManager) tryReconnect(conn *Connection) bool {
	// 这里是模拟实现
	// 实际应该调用具体 adapter 的 Connect 方法
	return true
}

// handleConnectionRestored 处理连接恢复
func (m *ConnectionManager) handleConnectionRestored(conn *Connection) {
	m.mu.Lock()
	conn.Status = "connected"
	conn.LastSeen = time.Now()
	m.mu.Unlock()

	slog.Info("connection restored",
		"connection_id", conn.ID,
		"adapter_id", conn.AdapterID,
	)

	// 触发恢复回调
	if conn.OnRestored != nil {
		conn.OnRestored(conn)
	}

	// 发出领域事件
	event := types.NewDomainEvent(
		types.EventConnectionRestored,
		"",
		&types.EventConnectionRestoredData{
			ConnectionID: conn.ID,
			AdapterID:    conn.AdapterID,
		},
	)
	m.emitEvent(event)

	// 重启心跳监控
	m.startHeartbeatMonitor(conn)
}

// emitEvent 发出事件（可以通过事件总线发送）
func (m *ConnectionManager) emitEvent(event *types.DomainEvent) {
	switch data := event.Data.(type) {
	case *types.EventConnectionLostData:
		slog.Debug("emitting event",
			"type", event.Type,
			"connection_id", data.ConnectionID,
		)
	case *types.EventConnectionRestoredData:
		slog.Debug("emitting event",
			"type", event.Type,
			"connection_id", data.ConnectionID,
		)
	default:
		slog.Warn("emitting unknown event type",
			"type", event.Type,
			"data_type", fmt.Sprintf("%T", event.Data),
		)
	}
}

// Stop 停止连接管理器
func (m *ConnectionManager) Stop() {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		if conn.Heartbeat != nil {
			conn.Heartbeat.Stop()
		}
	}

	m.connections = make(map[string]*Connection)
}
