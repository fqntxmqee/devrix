# Tool Security Enhancement Design

**Change ID:** devrix-tool-security
**Status:** S2 Design

---

## 一、Sandbox 详细设计

### 1.1 命令白名单

**文件：** `internal/layers/contextengine/tool_runner.go` → 新增 `sandbox.go`

```go
// sandbox.go
type CommandPolicy struct {
    Allowlist    []string          // 允许的命令名列表
    DenyPatterns []*regexp.Regexp  // 危险模式正则
    WorkDirLock  bool              // 锁定工作目录
}

var defaultAllowlist = []string{
    "ls", "cat", "head", "tail", "wc", "grep", "find",
    "git", "go", "python", "python3", "node", "npm",
    "echo", "printf", "date", "env", "pwd", "which",
    "mkdir", "cp", "mv", "touch", "chmod", "chown",
    "diff", "sort", "uniq", "cut", "tr", "sed", "awk",
}

var defaultDenyPatterns = []string{
    `\brm\s+(-[a-zA-Z]+\s+)*[/~]`,             // rm with flags targeting / or ~
    `\bsudo\b`,                                  // privilege escalation
    `\bcurl\b.*\|.*\b(?:sh|bash|python|perl)\b`,  // curl pipe interpreter
    `\bwget\b.*\|.*\b(?:sh|bash|python|perl)\b`,  // wget pipe interpreter
    `>[>]?\s*/dev/[a-z]`,                        // write to /dev/*
    `\bmkfifo\b`,                                 // named pipe
    `\bnc\s+-[lL]`,                               // netcat listen mode
    `\bchmod\s+.*[0-7]*7[0-7]*[0-7]*`,           // world-writable permission (any digit=7)
    `:\(\)\s*\{\s*:`,                            // fork bomb
    `\breboot\b`,                                 // system reboot
    `\bshutdown\b`,                               // system shutdown
    `\bdd\s+if=`,                                // raw disk read
    `\bchroot\b`,                                 // chroot escape
    `\$\(`,                                       // command substitution (potential bypass)
    "`[^`]+`",                                   // backtick command substitution
}
```

### 1.2 命令验证流程

```go
// extractCommandName 提取管道/链中的第一个命令名
// 输入: "ls -la | grep foo" → 输出: "ls"
// 输入: "sudo rm -rf /" → 输出: "sudo"（随后被 deny patterns 拦截）
func extractCommandName(command string) string {
    cmd := strings.TrimSpace(command)
    // 切割管道和链操作符
    for _, sep := range []string{"|", "&&", "||", ";"} {
        if idx := strings.Index(cmd, sep); idx >= 0 {
            cmd = cmd[:idx]
        }
    }
    cmd = strings.TrimSpace(cmd)
    // 取第一个空格前的 token
    if idx := strings.Index(cmd, " "); idx >= 0 {
        return cmd[:idx]
    }
    return cmd
}

func (p *CommandPolicy) Validate(command string) error {
    cmdName := extractCommandName(command)
    if cmdName == "" {
        return fmt.Errorf("empty command")
    }

    // Step 1: 白名单检查
    if !p.isAllowed(cmdName) {
        return fmt.Errorf("command not allowed: %s (add to tool.allowlist in config)", cmdName)
    }

    // Step 2: 危险模式检查（对完整命令字符串，不仅是命令名）
    for _, pattern := range p.DenyPatterns {
        if pattern.MatchString(command) {
            return fmt.Errorf("dangerous command pattern detected: %s", pattern.String())
        }
    }

    // Step 3: 工作目录外路径检测
    if p.WorkDirLock {
        if containsAbsPath(command) {
            return fmt.Errorf("absolute paths are not allowed")
        }
    }

    return nil
}
```

### 1.3 工作目录锁定

bash 命令执行前，通过环境变量限制作用域：

```go
func (r *BuiltinToolRunner) runBash(ctx context.Context, workDir, input string) (*ToolResult, error) {
    command := toolInputString(input, "command", "cmd")
    if command == "" {
        command = strings.TrimSpace(input)
    }

    // 安全校验（新增）
    if err := r.policy.Validate(command); err != nil {
        return &ToolResult{Error: err.Error()}, nil
    }

    // ... 现有 timeout 逻辑 ...

    cmd := exec.CommandContext(runCtx, "sh", "-c", command)
    cmd.Dir = workDir
    cmd.Env = []string{
        "HOME=" + workDir,
        "PATH=/usr/local/bin:/usr/bin:/bin",
        "PWD=" + workDir,
        "USER=devrix",
    }
    cmd.WaitDelay = 2 * time.Second

    // 审计日志
    if r.auditLogger != nil {
        r.auditLogger.LogCommand(ctx, command, workDir)
    }

    // ... 现有 stdout/stderr 收集逻辑 ...
}
```

**安全边界说明：** 环境变量限制（HOME/PATH/PWD）是纵深防御的一层，不是独立的沙箱。它依赖：
- 命令白名单阻止执行未授权二进制
- 危险模式拒绝阻止破坏性操作
- 工作目录路径穿越防护（resolveWorkspacePath）

三层共同构成防御体系。单层被绕过不会导致完全失控。

### 1.4 配置扩展

```yaml
# devrix.yaml
tool:
  sandbox:
    enabled: true
    allowlist_extra: ["docker", "kubectl"]  # 额外允许的命令
    deny_patterns_extra: []                  # 额外拦截模式
  concurrent_max: 10  # 并发工具执行上限
```

---

## 二、Plugin Registry 详细设计

### 2.1 ToolRunner 接口

**新增文件：** `internal/layers/contextengine/tool_plugin.go`

```go
// ToolRunner defines a pluggable tool executor.
type ToolRunner interface {
    Name() string
    Schema() ToolSchema
    Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
}

// ToolRegistry manages tool registration and execution.
type ToolRegistry struct {
    mu      sync.RWMutex
    runners map[string]ToolRunner
}
```

### 2.2 注册与分发

```go
func NewToolRegistry() *ToolRegistry {
    return &ToolRegistry{
        runners: make(map[string]ToolRunner),
    }
}

func (r *ToolRegistry) Register(runner ToolRunner) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, ok := r.runners[runner.Name()]; ok {
        return fmt.Errorf("tool already registered: %s", runner.Name())
    }
    r.runners[runner.Name()] = runner
    return nil
}

func (r *ToolRegistry) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
    r.mu.RLock()
    runner, ok := r.runners[call.Name]
    r.mu.RUnlock()

    if !ok {
        return &ToolResult{Error: fmt.Sprintf("unknown tool: %s", call.Name)}, nil
    }

    workDir, err := ResolveToolWorkDir(ctx)
    if err != nil {
        return &ToolResult{Error: err.Error()}, nil
    }

    return runner.Execute(ctx, workDir, call.Input)
}
```

### 2.3 内置工具适配

现有的 `BuiltinToolRunner` 通过实现 `ToolRunner` 接口适配：

```go
// bashRunner 实现 ToolRunner 接口
type bashRunner struct {
    policy *CommandPolicy
    cfg    *ToolConfig
}

func (r *bashRunner) Name() string { return "bash" }
func (r *bashRunner) Schema() ToolSchema {
    return ToolSchema{Name: "bash", Description: "Execute a shell command (sandboxed)"}
}
```

初始化时注册三个内置工具：

```go
func NewBuiltinToolRegistry(cfg *ToolConfig) *ToolRegistry {
    reg := NewToolRegistry()
    reg.Register(newBashRunner(cfg))
    reg.Register(newReadFileRunner(cfg))
    reg.Register(newWriteFileRunner(cfg))
    return reg
}
```

### 2.4 PEV 引擎集成

```go
// pev_engine.go — Execute 阶段替换硬编码 switch
func (e *PEVEngine) executeToolCall(ctx context.Context, call ToolCall) (*ToolResult, error) {
    // 权限检查（不变）
    if e.permissionGate != nil {
        if !e.permissionGate.Request(ctx, sessionID, call.Name, call.Input, riskLevel) {
            return &ToolResult{Error: "permission denied"}, nil
        }
    }

    // 通过注册表执行（替代 switch）
    return e.toolRegistry.Execute(ctx, call)
}
```

---

## 三、并发控制

**新增文件：** `internal/layers/contextengine/tool_limiter.go`

```go
type ToolLimiter struct {
    semaphore chan struct{}
}

func NewToolLimiter(maxConcurrent int) *ToolLimiter {
    if maxConcurrent <= 0 {
        maxConcurrent = 10
    }
    return &ToolLimiter{
        semaphore: make(chan struct{}, maxConcurrent),
    }
}

func (l *ToolLimiter) Acquire(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case l.semaphore <- struct{}{}:
        return nil
    }
}

func (l *ToolLimiter) Release() {
    <-l.semaphore
}
```

---

## 四、受影响的文件

```
internal/layers/contextengine/
├── sandbox.go              # NEW: CommandPolicy, 命令验证
├── tool_plugin.go          # NEW: ToolRunner 接口 + ToolRegistry
├── tool_limiter.go         # NEW: 并发工具执行限制
├── tool_runner.go          # MODIFIED: 重构为 ToolRunner 实现
├── pev_engine.go           # MODIFIED: 替换 switch 为 toolRegistry.Execute
├── registry/builtin.go     # MODIFIED: NewBuiltinToolRegistry
├── tool_runner_test.go     # MODIFIED: 增加安全测试
├── sandbox_test.go         # NEW: 命令验证测试
└── tool_plugin_test.go     # NEW: 插件注册测试

internal/shared/config/
└── tool_config.go          # NEW: ToolSandboxConfig

internal/layers/contextengine/contracts.go
                            # MODIFIED: 新增 ToolRunner 接口
```

---

## 五、回归风险评估

| 变更 | 回归风险 | 缓解措施 |
|------|---------|---------|
| 命令白名单 | 中 — 可能拦截合法命令 | 配置化 allowlist，提供审计模式（只记录不拦截） |
| 危险模式检测 | 低 — 仅拦截已知危险模式 | 白名单优先，误拦截可通过配置 bypass |
| 插件化注册 | 低 — 保持现有工具行为 | 已有集成测试覆盖 |
| 并发限制 | 低 — 默认 10，足够大 | 可配置关闭 |
