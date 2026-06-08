# 编码规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** S4
**适用范围:** `internal/` 下所有 Go 源码

---

## 1. 继承的全局规则

本规范以下列全局规则为基础，补充项目特有规则：

| 全局规则 | 路径 |
|---------|------|
| Go 编码风格 | `~/.claude/rules/golang/coding-style.md` |
| Go 安全 | `~/.claude/rules/golang/security.md` |
| Go 模式 | `~/.claude/rules/golang/patterns.md` |
| 通用编码风格 | `~/.claude/rules/common/coding-style.md` |
| 通用安全 | `~/.claude/rules/common/security.md` |

在全局规则与本规范冲突时，**本规范胜出**。

---

## 2. 包组织

### 2.1 目录映射

```
internal/
├── bootstrap/     # 依赖注入、启动组装
├── bridges/       # 层间桥接器
├── layers/
│   ├── communication/   # D1
│   ├── contextengine/   # D2
│   ├── llmgateway/      # D3
│   ├── multiagent/      # D4
│   └── observability/   # D5
└── shared/
    ├── types/      # 共享类型定义
    ├── config/     # 配置结构与加载
    ├── errors/     # 分层错误定义
    ├── contracts/  # 跨层接口契约
    └── textutil/   # 文本工具
```

### 2.2 包命名规则

- 全小写，单数，无下划线
- 与目录名一致
- 禁止 `util`、`common`、`base` 等无意义名称

---

## 3. 错误处理

### 3.1 Sentinel Error 模式

项目使用 `internal/shared/errors/` 下的 SentinelError 体系：

```go
// 定义 sentinel
var ErrAgentTimeout = errors.New("agent execution timeout")

// 定义错误码
const CodeAgentTimeout = "AGT_LIFECYCLE_5005"

// 工厂函数
func NewAgentTimeoutError(agentID, timeout string) *SentinelError {
    return WithCode(CodeAgentTimeout,
        fmt.Sprintf("Agent %s 执行超时 (timeout=%s)", agentID, timeout), ErrAgentTimeout)
}
```

### 3.2 错误包装

```go
// 跨层传播时包装
if err != nil {
    return fmt.Errorf("failed to create agent: %w", err)
}

// 使用 errors.Is 做类型判断
if errors.Is(err, ErrAgentTimeout) { ... }
```

### 3.3 禁止

- `panic` 用于业务错误（仅限初始化阶段不可恢复错误）
- `_ = err` 忽略错误（除非有注释说明原因）
- 返回 nil error 时同时返回有效值

---

## 4. 配置模式

### 4.1 配置分层

```
用户级: ~/.claude/     →  项目级: ~/.devrix/  →  环境变量  →  CLI flag
```

### 4.2 Config 结构模式

```go
// FileConfig — YAML 反序列化
type MultiAgentFileConfig struct {
    MaxChildren int `yaml:"max_children"`
}

// Config — 运行时配置（已验证、类型安全）
type MultiAgentConfig struct {
    MaxChildren int
}

// DefaultConfig — 返回 V1 默认值
func DefaultMultiAgentConfig() *MultiAgentConfig { ... }

// BuildConfig — 合并 file 覆盖 default
func BuildMultiAgentConfig(file *MultiAgentFileConfig) *MultiAgentConfig { ... }
```

---

## 5. 不可变性

必须创建新对象，禁止原地修改：

```go
// WRONG
session.Messages = append(session.Messages, msg)

// CORRECT — 使用写锁保护的追加方法
session.AppendMessage(msg)  // 内部处理并发安全
```

---

## 6. 文件规模限制

| 类型 | 上限 | 说明 |
|------|------|------|
| 函数 | 50 行 | 超过时拆分 |
| 文件 | 800 行 | 超过时按职责拆分 |
| 嵌套 | 4 层 | 超过时提取函数或使用 early return |

---

## 7. 导入组织

```go
import (
    // 标准库
    "context"
    "fmt"

    // 外部依赖
    "github.com/xxx/yyy"

    // 内部包
    "github.com/devrix/devrix/internal/shared/errors"
)
```

使用 `goimports` 自动格式化。

---

## 8. 检查清单

每次提交前：

- [ ] `go vet ./...` 通过
- [ ] `go build ./...` 通过
- [ ] 无 `panic` 用于业务错误
- [ ] 错误不被静默忽略
- [ ] 新文件在正确 D-S 目录下
- [ ] 函数 < 50 行，文件 < 800 行
- [ ] 无硬编码密钥/凭证

---

## 9. 代码完整性

### 9.1 不可变性分层策略

| 类型类别 | 允许变异 | 要求 | 示例 |
|---------|---------|------|------|
| 值对象 (Value Object) | 不可变 | `With*()` 返回新副本 | Attachment, AuthConfig |
| 聚合根/实体 (Entity) | 受控可变 | method + 内部锁，禁止外部直接改字段 | Session, Milestone |
| Service/Manager | 内部可变 | 对外暴露只读视图或副本 | AuthService, SessionStore |
| 基础设施 | 自由 | Timer、sync.Mutex | — |

### 9.2 值对象不可变模式

```go
// CORRECT
func (a *Attachment) WithName(name string) *Attachment {
    return &Attachment{Type: a.Type, Name: name, Path: a.Path, Content: a.Content}
}
```

### 9.3 实体受控可变模式

```go
// CORRECT
func (s *Session) SetState(state SessionState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.State = state
    s.UpdatedAt = time.Now()
}
```

### 9.4 不安全类型断言

禁止 `.(*ConcreteType)` 裸断言；使用 type switch 或 `ok` 模式。

### 9.5 命令查询分离 (CQS)

读方法（Get/List/Count）不得修改状态；状态刷新使用独立写方法（如 `HealthCheck`）。
