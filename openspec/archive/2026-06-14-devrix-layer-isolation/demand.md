---
demand-id: DM-20260611-002
title: 架构分层合规 — 消除反向依赖与契约错放
source: devrix-harness-architecture-audit
priority: P0
status: S1_Proposal
dsaft_domain: architecture
created: 2026-06-11
---

# 架构分层合规

## 1. 背景

Devrix 明确定义了 D1-D6 六层架构，D2 ContextEngine 处于 D1 Communication 上层。但当前实现中存在 D2 反向依赖 D1 的违规，以及跨层契约定义位置错乱。Claude Code 虽无分层架构，但其依赖管理方式值得参考：所有接口（Tool、ToolUseContext、ToolPermissionContext）定义在 `Tool.ts`，在 `tools.ts` 注册，通过函数参数（`useTool()`）而非全局位置传递。这种"定义→注册→注入"的契约流程自然消除了反向依赖问题。

## 2. 问题陈述

### 2.1 D2→D1 反向依赖

| 违规 | 位置 | 影响 |
|------|------|------|
| `contextengine/engine.go` import `communication/gateway` | `engine.go` import 语句 | D2 直接依赖 D1，违反分层单向依赖原则 |
| ContextEngine.Process() 直接调用网关方法 | `engine.go` L42-58 | 上层无法独立测试、无法替换网关实现 |

**证据**：`internal/layers/contextengine/engine.go` 包含对 `internal/layers/communication/gateway` 的 import，形成 D2 → D1 反向引用链。

### 2.2 跨层契约错放

| 契约接口 | 当前位置 | 应放置位置 |
|----------|----------|-----------|
| ILLMGateway | `contextengine/contracts.go` | `shared/contracts/` 或使用域 |
| IToolRunner | `contextengine/contracts.go` | 工具执行域 |
| IToolRegistry | `contextengine/contracts.go` | 工具注册域 |
| IPermissionGate | `contextengine/contracts.go` | Communication 或共享域 |

当前 D2 既是契约定义者又是使用者，其他层若需使用同一契约必须依赖 D2，加剧耦合。

### 2.3 架构合规门禁缺失

| 缺失项 | 应有行为 |
|--------|----------|
| 分层 lint 规则 | Go 静态分析禁止层间反向 import |
| CI 门禁 | PR 引入反向依赖时阻断合并 |
| 契约注册表 | 所有跨层接口的"定义域 + 消费域"映射 |

### 2.4 Claude Code 的契约管理方式

Claude Code 没有分层架构约束，但其接口组织对 Devrix 有直接借鉴意义：

| 原则 | Claude Code 实践 | Devrix 当前问题 |
|------|-----------------|----------------|
| **使用点定义** | `ToolUseContext` 定义在使用它的 `query.ts` 附近 | ILLMGateway 等接口集中在 D2 `contracts.go`，使用者被迫依赖 D2 |
| **调用处注入** | 工具通过 `tool.call(input, context, canUseTool, ...)` 参数传递，而非全局单例 | D2 直接 import D1 的具体实现而非接收接口 |
| **就近注册** | `tools.ts` 集中注册，插件通过 `builtinPlugins.ts` 扩展 | 跨层接口无注册表，定位困难 |

**建议方向**：Devrix 应借鉴"接口随使用域定义，通过构造函数注入"的模式，将 ILLMGateway 移至 D3 `llmgateway/contracts.go`（定义在归属域），IPermissionGate 移至 D1 `communication/contracts.go`，IToolRunner 移至工具执行域。D2 仅保留 PEV 相关的自有接口。

## 3. 验收标准

### P0 (阻止合并)

- [ ] 消除 D2→D1 所有反向 import 依赖，D2 仅通过接口与 D1 交互
- [ ] 将跨层契约（ILLMGateway、IToolRunner、IToolRegistry、IPermissionGate）迁移至 `shared/contracts/` 或各自归属域
- [ ] 实现 `go vet` 级别的分层依赖检查，违规时报错

### P1 (必须完成)

- [ ] 引入分层 lint 规则（`layer-linter`），在 CI 中自动执行
- [ ] 建立契约注册表文档，记录每个跨层接口的定义域和消费域
- [ ] ContextEngine 不再持有 Communication 的具体类型引用，仅依赖接口

### P2 (建议完成)

- [ ] 自动化架构合规报告，PR 评论中标注分层违规
- [ ] 实现分层合规评测探针（`LayerViolationProbe`）：CI 中自动统计反向 import 数量并记录基线，注册到 D6 Eval 框架
- [ ] 运行时检测降级/兼容路径使用，通过 D5 指标暴露旧接口调用频次，辅助迁移完成决策

## 4. 领域映射

| 子域 | 影响范围 | 预期工作量 |
|------|----------|-----------|
| `shared/contracts` | 契约接口迁移 | 中 |
| `contextengine` | 移除 D2→D1 反向依赖 | 高 |
| `communication` | 接口抽取与适配 | 中 |
| `ci/quality` | 分层 lint + CI 门禁 | 低 |
| `docs` | 契约注册表文档 | 低 |
| `observability/metrics` | 运行时反向依赖告警指标 | 低 |
| `d6/eval` | LayerViolation 探针 | 低 |

## 5. 回归风险

- 接口迁移后所有调用方需同步更新引用路径
- 需确保移入 `shared/contracts` 后的接口不产生循环依赖
- 分层 lint 规则可能误报，需配置允许的白名单例外
