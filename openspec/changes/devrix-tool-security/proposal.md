# Proposal: 工具执行安全增强

**Change ID:** devrix-tool-security
**Demand ID:** DM-20260608-001
**Status:** S2 Design
**Author:** Architecture
**Date:** 2026-06-08

---

## 1. Background

Devrix 的 PEV 引擎通过 `BuiltinToolRunner` 执行用户请求的工具调用。当前实现存在两个结构性问题：

1. **bash 工具无沙箱**：`exec.CommandContext(runCtx, "sh", "-c", command)` 直接执行任意 shell 命令，无命令白名单、无危险模式检测、无资源限制。
2. **工具硬编码 switch-case**：`tool_runner.go:41-50` 中 `bash/read_file/write_file` 硬编码，新增工具需修改核心代码。

虽然 `read_file` 和 `write_file` 已有 `resolveWorkspacePath` 路径穿越防护，但 bash 工具完全绕过了这一防护——用户可以通过 `bash` 执行任意文件操作、网络访问、进程管理。

## 2. Problem Statement

### 2.1 安全问题

| 风险 | 当前行为 | 影响 |
|------|---------|------|
| 路径穿越 | bash 可访问工作目录外文件 | 读取系统敏感文件 |
| 危险命令 | `rm -rf /`、`curl | bash` 等无检测 | 系统破坏 |
| 资源耗尽 | 无 CPU/内存/磁盘限制 | DoS |
| 网络越权 | bash 可发起任意网络连接 | 数据泄露 |
| 权限提升 | 以 devrix 进程权限执行任意命令 | 提权风险 |

### 2.2 扩展性问题

```go
// tool_runner.go:41-50 — 硬编码，无法扩展
switch call.Name {
case "bash":
    return r.runBash(ctx, workDir, call.Input)
case "read_file":
    return r.runReadFile(workDir, call.Input)
case "write_file":
    return r.runWriteFile(workDir, call.Input)
default:
    return &ToolResult{Error: fmt.Sprintf("unknown tool: %s", call.Name)}, nil
}
```

新增工具（如 `grep`、`git`、`http_fetch`）需要修改核心 runner 代码。

## 3. Proposed Solution

### 3.1 沙箱策略：纵深防御

不引入完整容器沙箱（如 gVisor，过于重量），采用多层防御：

| 层 | 机制 | 说明 |
|----|------|------|
| **L1: 命令白名单** | 仅允许可配置的命令集合 | 默认：`ls`, `cat`, `grep`, `find`, `wc`, `head`, `tail`, `git`, `go`, `python`, `node` 等 |
| **L2: 危险模式拒绝** | 正则匹配危险模式 | `rm -rf /`、`curl | sh`、`sudo`、`chmod 777`、`/dev/` 写入等 |
| **L3: 工作目录锁定** | bash 命令在 chroot-like 约束下执行 | 复用 `resolveWorkspacePath`，bash 执行前设置 `HOME=$WORKDIR` |
| **L4: 资源限制** | 超时 + 输出大小限制 | 已有 60s timeout 和 64KB 输出限制，增加并发限制 |
| **L5: 审计日志** | 所有 bash 执行记录完整命令 | 用于事后审计 |

### 3.2 插件化工具注册

```go
// 新增 ToolRunner 接口
type ToolRunner interface {
    Name() string
    Execute(ctx context.Context, workDir, input string) (*ToolResult, error)
    Schema() ToolSchema
}

// 插件化注册
type ToolRegistry struct {
    mu       sync.RWMutex
    runners  map[string]ToolRunner
}
```

### 3.3 不改动的部分

- 不修改 PEV 引擎的核心流程
- 不修改 Communication Layer 的权限请求机制
- 不引入外部沙箱依赖（gVisor、Docker）
- 不改变现有 `read_file` / `write_file` 的行为

---

## 4. Success Metrics

| Metric | Target |
|--------|--------|
| 危险命令检测率 | 100%（已知危险模式全部拦截） |
| 路径穿越防护 | 100%（bash 无法访问工作目录外文件） |
| 插件工具注册 | 新增工具无需修改 tool_runner.go |
| L5 测试通过率 | 4/4 P0 |
| 回归测试通过率 | 100% |

---

## 5. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| 命令白名单过严影响正常使用 | 中 | 白名单可通过 YAML 配置扩展 |
| 危险模式误拦截正常命令 | 低 | 白名单优先于黑名单，提供 bypass 配置 |
| 插件接口变更影响现有代码 | 低 | `BuiltinToolRunner` 实现 `ToolRunner` 接口 |
| 并发限制影响多工具场景 | 低 | 默认并发限制 10，可配置 |

---

## 6. 任务估算

| Milestone | 任务数 | 预估 |
|-----------|--------|------|
| M1 Sandbox | 3 | 6h |
| M2 Plugin Registry | 2 | 5h |
| M3 Concurrency | 1 | 2h |
| M4 Test | 1 | 3h |
| **合计** | **7** | **~16h** |
