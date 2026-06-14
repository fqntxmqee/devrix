# Proposal: 架构分层合规 — 消除反向依赖与契约错放

**Change ID:** devrix-layer-isolation
**Demand ID:** DM-20260611-002
**Status:** S2_Proposal
**Priority:** P0

## 1. Background

Devrix 明确定义 D1-D6 六层架构。当前实现存在两类问题：(a) D2 ContextEngine 反向依赖 D1 Communication 的具体类型；(b) 跨层契约接口（ILLMGateway、IToolRunner、IToolRegistry、IPermissionGate）集中在 D2 `contracts.go`，迫使其他层若需使用必须依赖 D2，扩散耦合。Claude Code 的"接口随使用域定义 + 注入式调用"是直接可借鉴的反例。

## 2. Problem Statement

| 问题 | 位置 | 严重度 |
|------|------|--------|
| `contextengine/engine.go` import `communication/gateway` | D2→D1 反向 | P0 |
| ILLMGateway 等接口集中在 D2 contracts.go | 跨层契约错放 | P0 |
| ContextEngine 直接调用网关方法（持有具体类型） | 上层无法独立测试 | P0 |
| 无分层 lint 规则 | CI 无门禁 | P0 |
| 无契约注册表 | 接口定位困难 | P1 |
| 无运行时反向依赖指标 | 迁移完成度不可观测 | P2 |

## 3. Proposed Solution

### 3.1 消除 D2→D1 反向依赖

- **D2 不再持有 D1 具体类型引用**：将 `EngineEvent` 等由 D2 触达 D1 的对象改为 D2 自有抽象，调用通过 `IEngine.Sink` 接口注入。
- **依赖反转**：D1 持有 D2 引用（启动时注入），D2 不知道 D1 存在。
- **契约迁移**：将跨层接口按"使用域"或"语义归属域"重新放置（见 3.2）。

### 3.2 跨层契约归位

| 契约 | 当前位置 | 目标位置 | 理由 |
|------|----------|----------|------|
| `ILLMGateway` | `contextengine/contracts.go` | `llmgateway/contracts.go` (D3) | 语义归属 LLM 网关域 |
| `IToolRunner` / `IToolRegistry` | `contextengine/contracts.go` | 工具执行域（新建或 `shared/contracts/tools`） | 语义归属工具域 |
| `IPermissionGate` | `contextengine/contracts.go` | `communication/contracts.go` (D1) | 由 D1 PermissionManager 实现 |
| `IEngine` / `EngineEvent` | `contextengine/contracts.go` 或 `shared/contracts` | `shared/contracts/` | 跨域共享，D2 仍是其主要实现者但接口定义在共享层 |

### 3.3 分层 lint 工具

- 新建 `scripts/layer-lint/main.go`：
  - 用 `golang.org/x/tools/go/packages` 加载 AST
  - 定义层间允许/禁止的 import 矩阵
  - 违规时输出 `file:line` + 修复建议
- 加 `.github/workflows/layer-lint.yml`：PR 触发，违规 exit 1
- 提供白名单 `layer-lint.yaml`（`scripts/layer-lint/allowlist.yaml`）处理合理例外

### 3.4 契约注册表

- 新建 `openspec/specs/architecture/contract-registry.md` 表格：列出所有跨层接口、定义域、消费域、最近一次审计日期
- 每个接口变更必须在该表登记，CI 检查一致性

### 3.5 D6 LayerViolationProbe

- 测量指标：反向 import 数量、违规 file:line 列表、运行时具体类型引用频次
- 注册到 D6 Eval 框架，CI 中输出基线 vs 当前
- 通过 D5 Metrics 暴露 `runtime.layer_violation_refs` Counter

## 4. Success Metrics

| 指标 | 基线 | 目标 |
|------|------|------|
| D2→D1 反向 import 数 | 1+ | 0 |
| 跨层接口错放数（契约定义域 ≠ 语义归属域） | 4 | 0 |
| `layer-lint` 阻断违规 PR | N/A | 100% |
| 契约注册表覆盖率 | 0% | 100% |
| `runtime.layer_violation_refs` 指标 | N/A | 暴露且 P0 0 |

## 5. Implementation Plan

| Phase | 内容 | 估时 |
|-------|------|------|
| P1 | 契约迁移（ILLMGateway/IToolRunner/IToolRegistry/IPermissionGate）到各自归属域 | 1.5d |
| P2 | D2 反向 import 消除：ContextEngine.Process 改为接受 SinkFunc/接口 | 2d |
| P3 | layer-lint 工具 + CI workflow + 白名单 | 1d |
| P4 | 契约注册表文档 + CI 一致性检查 | 0.5d |
| P5 | D6 LayerViolationProbe + D5 runtime 指标 | 0.5d |
| **Total** | | **5.5d** |

**合并策略**：3 个 PR（contracts 迁移 / D2 重构 / lint+CI），互相依赖。

## 6. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| 接口迁移后所有调用方需同步更新 | 自动化 `go vet` + `gopls` rename 验证；保留旧路径 deprecation 一个版本 |
| 移入 `shared/contracts` 后循环依赖 | 严格单向：D6→D5→D4→D3→D2→D1；shared 不依赖任何 D |
| 分层 lint 误报 | 白名单 + 团队 review；先 WARN 不阻断，跑稳后切阻断 |
| D2 重构范围大 | PathRegression 测试先到位，行为不变前提下逐步替换 |

## 7. Out of Scope

- 整体重命名为 L1-L2（已搁置，见 `devrix-layering-standard`）
- D4/D5/D6 内部架构调整（本次只关注跨层边界）
- 运行时反射式依赖检查（仅编译期 lint + 计数指标）
