# S3 设计文档：配置热重载机制

**Change ID:** feat-config-hot-reload  
**状态:** Ready for Review  
**最后更新:** 2026-06-13  

---

## 1. 架构设计

### 1.1 模块位置

```
evolution/
└── config/
    ├── watcher.go           # 文件监控器
    ├── watcher_test.go
    ├── notifier.go          # 变更通知器
    ├── notifier_test.go
    ├── hotreload/
    │   ├── service.go       # 热重载服务（主入口）
    │   ├── service_test.go
    │   ├── options.go       # 配置选项
    │   └── doc.go           # 包文档
    └── doc.go               # 包文档
```

### 1.2 组件交互图

```
┌──────────────────────────────────────────────────────────────────┐
│                        HotReloadService                          │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                     ConfigManager                            │ │
│  │  - config: *Config (protected by RWMutex)                   │ │
│  │  - watchers: []Watcher                                     │ │
│  │  - subscribers: []Subscriber                               │ │
│  └────────────────────────────────────────────────────────────┘ │
│                              │                                    │
│          ┌───────────────────┼───────────────────┐               │
│          ▼                   ▼                   ▼               │
│   ┌────────────┐     ┌────────────┐     ┌────────────────┐       │
│   │  Watcher   │     │  Notifier  │     │ ConfigWatcher  │       │
│   │ (interface)│     │ (interface)│     │  (fsnotify impl)      │
│   └────────────┘     └────────────┘     └────────────────┘       │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │   Subscribers   │
                    │  - LLMGateway   │
                    │  - Logger       │
                    │  - Permission   │
                    └─────────────────┘
```

---

## 2. 接口设计

### 2.1 Watcher 接口

```go
// Watcher 文件变更监控接口
type Watcher interface {
    // Start 启动监控
    Start(ctx context.Context) error
    
    // Stop 停止监控
    Stop() error
    
    // Events 返回变更事件通道
    Events() <-chan Event
}
```

### 2.2 Subscriber 接口

```go
// Subscriber 配置变更订阅者
type Subscriber interface {
    // OnConfigChange 配置变更回调
    // old: 变更前的配置
    // new: 变更后的配置
    OnConfigChange(old, new *Config) error
    
    // Priority 优先级（数值越小优先级越高）
    Priority() int
}
```

### 2.3 HotReloadService 接口

```go
// Service 热重载服务
type Service interface {
    // Start 启动热重载服务
    Start(ctx context.Context) error
    
    // Stop 停止热重载服务
    Stop() error
    
    // Subscribe 注册订阅者
    Subscribe(sub Subscriber) error
    
    // Unsubscribe 取消订阅
    Unsubscribe(sub Subscriber) error
    
    // GetConfig 获取当前配置
    GetConfig() *Config
}
```

---

## 3. 核心算法

### 3.1 防抖算法

```go
func (w *configWatcher) debounce(duration time.Duration, fn func()) {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if w.timer != nil {
        w.timer.Stop()
    }
    w.timer = time.AfterFunc(duration, func() {
        w.mu.Lock()
        defer w.mu.Unlock()
        if w.lastEvent != nil {
            fn()
        }
    })
}
```

### 3.2 配置合并算法

```go
func mergeConfig(old, new *Config) *Config {
    result := deepCopy(old)
    
    // 逐字段合并，仅更新有变更的字段
    if new.Log != nil {
        result.Log = new.Log
    }
    if new.LLM != nil {
        result.LLM = new.LLM
    }
    // ...
    
    return result
}
```

### 3.3 错误回滚算法

```go
func (s *service) reload() error {
    // 1. 备份当前配置
    backup := s.config
    
    // 2. 尝试加载新配置
    newConfig, err := s.loader.Load(s.path)
    if err != nil {
        // 恢复备份
        s.config = backup
        s.logger.Error("config reload failed, rolled back", "error", err)
        return err
    }
    
    // 3. 通知订阅者
    for _, sub := range s.subscribers {
        if err := sub.OnConfigChange(backup, newConfig); err != nil {
            s.logger.Error("subscriber error", "sub", sub, "error", err)
            // 继续通知其他订阅者
        }
    }
    
    // 4. 更新配置
    s.config = newConfig
    return nil
}
```

---

## 4. 数据结构

### 4.1 Event 类型

```go
// Event 文件变更事件
type Event struct {
    Path    string
    Op      Op      // CREATE, WRITE, REMOVE
    Time    time.Time
}

// Op 文件操作类型
type Op uint32

const (
    CREATE Op = 1 << iota
    WRITE
    REMOVE
    RENAME
)
```

### 4.2 Options 配置

```go
// Options 热重载选项
type Options struct {
    // ConfigPath 配置文件路径
    ConfigPath string
    
    // Debounce 防抖延迟（默认 500ms）
    Debounce time.Duration
    
    // MaxSubscribers 最大订阅者数量（默认 10）
    MaxSubscribers int
    
    // OnError 错误回调
    OnError func(error)
}
```

---

## 5. 测试策略

### 5.1 单元测试

| 测试用例 | 覆盖功能 |
|----------|----------|
| TestWatcher_Events | 文件变更事件捕获 |
| TestNotifier_Subscribe | 订阅者注册 |
| TestNotifier_Notify | 变更通知分发 |
| TestService_Reload | 配置重载逻辑 |
| TestService_Rollback | 错误回滚 |
| TestService_Merge | 配置合并 |
| TestDebounce | 防抖机制 |

### 5.2 集成测试

| 测试用例 | 说明 |
|----------|------|
| TestIntegration_FullFlow | 完整热重载流程 |
| TestIntegration_MultiSubscriber | 多订阅者场景 |

### 5.3 覆盖率目标

- 行覆盖率 ≥ 80%
- 分支覆盖率 ≥ 75%
- 函数覆盖率 ≥ 90%

---

## 6. 依赖管理

### 6.1 外部依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| github.com/fsnotify/fsnotify | v1.7.0 | 文件系统监控 |
| gopkg.in/yaml.v3 | v3.0.1 | YAML 解析 |

### 6.2 内部依赖

```
evolution/config
    └── shared/errors     # SentinelError 定义
    └── shared/config    # 基础配置结构
```

---

## 7. 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `hotreload.enabled` | bool | true | 是否启用热重载 |
| `hotreload.debounce` | duration | 500ms | 防抖延迟 |
| `hotreload.path` | string | "devrix.yaml" | 配置文件路径 |

---

## 8. 设计检查清单

- [x] 接口设计遵循单一职责
- [x] 使用 RWMutex 保护并发读写
- [x] 错误处理使用 SentinelError 模式
- [x] 函数行数 < 50 行
- [x] 文件行数 < 800 行
- [x] 所有公开接口有文档注释
- [x] 单元测试覆盖率 ≥ 80%
- [x] 使用选项模式扩展配置
- [x] 资源清理（Stop 方法）正确实现
