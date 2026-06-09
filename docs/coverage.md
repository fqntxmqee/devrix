# 代码染色 (Code Coverage)

实时跟踪运行时 Span 调用，识别从未被触发过的操作（潜在无用代码）。

## 工作原理

```
Span 创建 → coverage.RecordHit() → 全局 Counter 原子计数
                            ↓
                    累积统计到内存
                            ↓
                    每日生成报告到 ~/.devrix/coverage/
```

## 操作注册表

所有 Span 操作必须在 `coverage/registry.go` 中注册：

```go
// internal/layers/observability/coverage/registry.go
func AllOperations() []OperationMeta {
    return []OperationMeta{
        {Name: "context.process", Layer: "context", Component: "context_engine", ...},
        {Name: "context.pev.run", Layer: "context", Component: "pev_engine", ...},
        {Name: "llm.stream", Layer: "llm", Component: "llm_gateway", ...},
        // ...
    }
}
```

### 层级结构

| Layer | 说明 | 示例 |
|-------|------|------|
| `context` | 上下文引擎层 | `context_engine`, `pev_engine` |
| `llm` | LLM 网关层 | `llm_gateway`, `llm_adapter` |
| `communication` | 通信层 | `gateway`, `adapter` |
| `agent` | Agent 工具层 | `agent_tool` |

### 命名规范

```
{layer}.{component}.{action}
```

- `context.process` - 上下文处理主入口
- `context.pev.llm_call` - PEV LLM 调用
- `context.pev.tool_execute` - 工具执行
- `llm.stream` - LLM 流式调用

## 新增操作

1. 在 `registry.go` 的 `AllOperations()` 中添加：

```go
{Name: "context.my_operation", Layer: "context", Component: "context_engine", SinceVersion: "2.1.0", Instrumented: true}
```

2. 在创建 Span 时使用相同名称：

```go
ctx, span := tracer.Start(ctx, "context.my_operation")
```

## 命令行工具

### 安装

```bash
cd cmd/coverage
go build -o ../bin/devrix-coverage .
```

或直接运行：

```bash
go run ./cmd/coverage
```

### 使用

```bash
# 列出所有报表
devrix-coverage --list

# 查看指定日期详细报表
devrix-coverage --date 2026-06-09

# 查看简洁统计（按 Layer 分组）
devrix-coverage --date 2026-06-09 --summary

# 查看 N 天趋势
devrix-coverage --trend 7

# 查看最新报表
devrix-coverage
```

## 输出示例

### 详细报表

```
========== Coverage Report: 2026-06-09 ==========
Coverage: 22.2% (4/18 operations hit)

┌─ CONTEXT ──────────────────────────────────────
│  Layer: context
│
│  ├─ pev_engine (3/8 hit)
│  │   ● context.pev.run [1.2.0] 5 hits
│  │   ● context.pev.llm_call [1.2.0] 8 hits
│  │   ○ context.pev.tool_execute [1.2.0] 0 hits
│  │   ● context.pev.verify [1.2.0] 5 hits
│
│  ├─ context_engine (1/10 hit)
│  │   ● context.process [1.2.0] 10 hits
│  │   ○ context.snapshot.load [1.2.0] 0 hits

┌─ LLM ──────────────────────────────────────
│  Layer: llm
│
│  ├─ llm_gateway (0/4 hit)
│  │   ○ llm.stream [1.2.0] 0 hits
│  │   ○ llm.retry [2.0.0] 0 hits
```

### 符号说明

| 符号 | 含义 |
|------|------|
| `●` | 已命中（运行时被调用） |
| `○` | 零命中（从未被调用，可能是无用代码） |

### 简洁统计

```
========== Coverage Summary: 2026-06-09 ==========
Total: 22.2% (4/18 operations)

context         |████░░░░░░░░░░░░░░░░| 22.2% (4/18)
communication   |░░░░░░░░░░░░░░░░░░░░|  0.0% (0/15)
agent           |░░░░░░░░░░░░░░░░░░░░|  0.0% (0/6)
llm             |░░░░░░░░░░░░░░░░░░░░|  0.0% (0/5)
```

### 趋势报表

```
========== Coverage Trend ==========
Period: 2026-06-07 → 2026-06-09

2026-06-07 |████████████░░░░░░░░░░░░░░░░░░░░░░░|  25.0% hit:11 zero:33
2026-06-08 |██████████░░░░░░░░░░░░░░░░░░░░░░░░░|  20.5% hit:9  zero:35
2026-06-09 |██████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░|  15.9% hit:7  zero:37
```

## 配置文件

```yaml
# devrix.yaml 或 .env
observability:
  coverage:
    enabled: true
    dir: "~/.devrix/coverage"  # 报表存储目录
    interval: 1h                       # 检查间隔
```

## 报表存储

```
~/.devrix/coverage/
├── coverage_2026-06-07.json
├── coverage_2026-06-08.json
└── coverage_2026-06-09.json
```

### JSON 格式

```json
{
  "date": "2026-06-09",
  "generated_at": "2026-06-09T00:00:00Z",
  "operations_total": 44,
  "operations_hit": 7,
  "operations_zero": 37,
  "coverage_ratio": 0.159,
  "zero_hit_operations": [
    {"operation": "context.snapshot.load", "layer": "context", "component": "context_engine", "since_version": "1.2.0"}
  ],
  "unknown_hits": 1,
  "hits": {
    "context.process": 5,
    "context.pev.run": 5,
    "context.pev.llm_call": 8
  }
}
```

## 识别无用代码

零命中的操作可能是：

1. **未启用功能** - 配置关闭了该功能
2. **死代码** - 代码存在但从未被调用
3. **条件分支** - 需要特定条件才会触发

### 处理流程

1. 查看零命中操作列表
2. 检查对应代码是否仍在使用
3. 如确认无用，清理代码或移除注册表项
4. 如需保留，添加测试覆盖

## API 接口

### HealthCheck

```bash
curl http://localhost:8080/health
```

响应包含：

```json
{
  "coverage": {
    "operations_total": 44,
    "operations_hit": 7,
    "coverage_ratio": 0.159,
    "zero_hit_count": 37
  }
}
```

### 编程接口

```go
import "github.com/devrix/devrix/internal/layers/observability"

// 获取当前报表
report := obs.CoverageReport(true)

// 生成日报
dailyReport, err := obs.GenerateCoverageReport()

// 获取趋势
trend, err := obs.CoverageReporter().GetTrend(7)
```

## 最佳实践

1. **新增 Span 时同步注册** - 保持 registry 与实际代码一致
2. **定期审查零命中** - 每周检查一次
3. **版本标记** - `since_version` 记录引入版本
4. **测试覆盖** - 确保关键路径有测试

## 故障排查

### 报表为空

- 检查 `observability.enabled` 配置
- 确认 tracer 正常初始化
- 查看日志中的 coverage reporter 启动信息

### 未知操作警告

```
WARN observability: unknown operation operation=my.operation
```

解决方案：在 `registry.go` 中添加注册

### 数据不累积

- 每次启动是新进程，计数从零开始
- 需要长期累积看报表目录
