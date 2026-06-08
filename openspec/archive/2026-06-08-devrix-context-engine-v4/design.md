# Context Engine Performance Optimization Design

**Change ID:** devrix-context-engine-v4
**Parent:** devrix-context-engine-v3
**Status:** S2 Design

---

## 一、Autocompact 异步化

### 1.1 当前问题

**文件：** `compression/autocompact.go:54-61`

```go
// 同步阻塞：整个压缩管道等待 LLM 摘要返回
timeout := cfg.Timeout  // 默认 10s
runCtx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()
summary, err := summarizeWithRetry(runCtx, summarizer, cfg, middle)
// 用户在此等待 10s ...
```

### 1.2 异步方案

**文件：** `compression/autocompact.go`

新增 `AsyncAutocompacter` 包装，支持同步降级 + 异步完成：

```go
type asyncResult struct {
    summary string
    err     error
    token   string  // 去重 token
}

type AsyncAutocompacter struct {
    summarizer Summarizer
    observer   StepObserver
    mu         sync.Mutex
    pending    map[string]map[string]context.CancelFunc  // sessionID → token → cancel
    latest     map[string]string                          // sessionID → 最新完成的摘要 token
    wg         sync.WaitGroup
}

func NewAsyncAutocompacter(summarizer Summarizer, observer StepObserver) *AsyncAutocompacter {
    return &AsyncAutocompacter{
        summarizer: summarizer,
        observer:   observer,
        pending:    make(map[string]map[string]context.CancelFunc),
        latest:     make(map[string]string),
    }
}
```

**runAutocompact 修改：**

```go
func runAutocompact(...) ([]types.Message, string, error) {
    if !shouldAutocompact(msgs, budget, counter, cfg) {
        return msgs, stepAutocompact + ":skipped", nil
    }

    turns := splitTurns(msgs)
    head := cfg.PreserveHeadTurns  // 默认 2
    tail := cfg.PreserveTailTurns  // 默认 2

    // 同步返回：占位摘要（head + 占位 + tail）
    placeholder := []types.Message{}
    for i := 0; i < head; i++ {
        placeholder = append(placeholder, turns[i]...)
    }
    placeholder = append(placeholder, types.Message{
        Role:    types.MessageRoleAssistant,
        Content: "[compressing conversation... keeping 4 most recent exchanges]",
        Metadata: map[string]string{
            "compressed_by": "autocompact",
            "status":        "pending",
        },
    })
    for i := len(turns) - tail; i < len(turns); i++ {
        placeholder = append(placeholder, turns[i]...)
    }

    // 异步生成真实摘要
    if a := asyncAutocompacter; a != nil {
        asyncToken := uuid.New().String()
        a.startAsync(ctx, sessionID, cfg, turns, head, tail, asyncToken)
    }

    return placeholder, stepAutocompact, nil
}

func (a *AsyncAutocompacter) startAsync(
    ctx context.Context,
    sessionID string,
    cfg config.AutocompactConfig,
    turns [][]types.Message,
    head, tail int,
    asyncToken string,
) {
    // 取消同一 session 之前的 pending 任务，标记 latest
    a.mu.Lock()
    if _, ok := a.pending[sessionID]; !ok {
        a.pending[sessionID] = make(map[string]context.CancelFunc)
    }
    for token, cancel := range a.pending[sessionID] {
        cancel()
        delete(a.pending[sessionID], token)
    }
    a.latest[sessionID] = asyncToken // 触发时标记，完成时对比去重
    runCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
    a.pending[sessionID][asyncToken] = cancel
    a.wg.Add(1)
    a.mu.Unlock()

    go func() {
        defer a.wg.Done()
        defer func() {
            a.mu.Lock()
            delete(a.pending[sessionID], asyncToken)
            if len(a.pending[sessionID]) == 0 {
                delete(a.pending, sessionID)
            }
            a.mu.Unlock()
        }()

        middle := flattenTurns(turns[head : len(turns)-tail])
        summary, err := summarizeWithRetry(runCtx, a.summarizer, cfg, middle)
        if err != nil {
            if a.observer != nil {
                a.observer.OnAutocompact(AutocompactMeta{Degraded: true, Model: cfg.Model})
            }
            return
        }

        // 去重：仅当 myToken 仍是 session 最新 token 时才写入
        a.mu.Lock()
        isLatest := a.latest[sessionID] == asyncToken
        a.mu.Unlock()

        if isLatest {
            if a.observer != nil {
                a.observer.OnAutocompactComplete(
                    buildSummaryMessage(summary), sessionID, asyncToken,
                )
            }
        }
    }()
}
```

### 1.3 Shutdown 清理

```go
func (a *AsyncAutocompacter) Shutdown(timeout time.Duration) error {
    a.mu.Lock()
    for _, sessionTasks := range a.pending {
        for _, cancel := range sessionTasks {
            cancel()
        }
    }
    a.mu.Unlock()

    done := make(chan struct{})
    go func() {
        a.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("async autocompact shutdown timeout")
    }
}
```

### 1.4 上层集成

PEV 引擎通过 Observer 接收异步完成事件，决定是否更新对话视图：

```go
// Observer 新增方法（contracts.go）
type IAutocompactObserver interface {
    OnAutocompactComplete(summaryMsg types.Message, sessionID, asyncToken string)
}

// buildSummaryMessage 将 LLM 摘要字符串转换为 Message（compression/async_compact.go）
func buildSummaryMessage(summary string) types.Message {
    return types.Message{
        Role:    types.MessageRoleAssistant,
        Content: summary,
        Metadata: map[string]string{
            "compressed_by": "autocompact",
            "status":        "complete",
        },
    }
}
```

---

## 二、快照 Snappy 压缩

### 2.1 当前问题

**文件：** `snapshot/store.go:41`

```go
func (s *Store) Serialize(sc *types.SessionContext) ([]byte, error) {
    snap := ... // toSnapshot
    return json.Marshal(snap)  // 无压缩
}
```

### 2.2 Snappy 压缩方案

```go
import "github.com/golang/snappy"

const snappyMagic = "\xfe\x53"  // 文件头标识，区分新旧格式

// toSnapshot 见 snapshot/store.go:22 — 将 SessionContext 转为 ContextSnapshotV1
// 此处省略重复引用，仅修改 Serialize/Deserialize 的序列化方式

func (s *Store) Serialize(sc *types.SessionContext) ([]byte, error) {
    snap := toSnapshot(sc)
    raw, err := json.Marshal(snap)
    if err != nil {
        return nil, err
    }

    if !s.cfg.Compression {
        return raw, nil
    }

    compressed := snappy.Encode(nil, raw)
    // 添加魔数头部以区分格式
    out := make([]byte, len(snappyMagic)+len(compressed))
    copy(out, snappyMagic)
    copy(out[len(snappyMagic):], compressed)
    return out, nil
}

func (s *Store) Deserialize(data []byte) (*types.SessionContext, error) {
    if len(data) == 0 {
        return nil, errors.NewSnapshotCorruptError(fmt.Errorf("empty snapshot"))
    }

    // 检测格式
    if len(data) >= 2 && string(data[:2]) == snappyMagic {
        raw, err := snappy.Decode(nil, data[2:])
        if err != nil {
            return nil, errors.NewSnapshotCorruptError(err)
        }
        data = raw
    }
    // 否则执行 legacy 路径（未压缩 JSON 或原始 snappy）

    var snap types.ContextSnapshotV1
    if err := json.Unmarshal(data, &snap); err != nil {
        return nil, errors.NewSnapshotCorruptError(err)
    }
    // ... 现有 deserialize 逻辑 ...
}
```

### 2.3 压缩配置

```yaml
# devrix.yaml
context_engine:
  snapshot:
    compression: true           # 启用 snappy 压缩
    compression_threshold: 4096 # 仅压缩 > 4KB 的快照
```

### 2.4 向后兼容

- 旧格式（无魔数、无压缩 JSON）直接 json.Unmarshal → 正常读取
- 新格式（含魔数 snappy）先 decode 再 json.Unmarshal
- WriteBackup 使用与主存储相同的压缩格式

---

## 三、受影响的文件

```
internal/layers/contextengine/
├── compression/
│   ├── autocompact.go          # MODIFIED: runAutocompact 返回占位 + 异步触发
│   ├── async_compact.go        # NEW: AsyncAutocompacter
│   └── contracts.go            # MODIFIED: IAutocompactObserver 新增方法
├── snapshot/
│   ├── store.go                # MODIFIED: snappy 压缩 + 格式检测
│   └── store_test.go           # MODIFIED: 压缩/解压 + 兼容性测试
├── pev_engine.go               # MODIFIED: AutocompactObserver 集成
├── pev_engine_test.go          # MODIFIED: 异步摘要事件测试
└── config/
    └── snapshot_config.go      # MODIFIED: compression 字段

go.mod                          # MODIFIED: + github.com/golang/snappy + github.com/google/uuid
```

---

## 四、回归风险评估

| 变更 | 回归风险 | 缓解措施 |
|------|---------|---------|
| Autocompact 异步化 | 中 — 改变压缩行为时序 | 同步降级路径不变：摘要失败时占位消息包含 head/tail |
| Snappy 压缩 | 低 — 格式兼容旧数据 | 魔数检测 + JSON fallback |
| 新增依赖 | 低 — golang/snappy 是 Go 官方子仓库 | 无第三方依赖 |
| 异步 goroutine | 中 — 泄漏风险 | WaitGroup + context 传播 + Shutdown |
