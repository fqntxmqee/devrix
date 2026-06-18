# Design: 架构分层 v1.1 — 4 接口契约物理迁移

**Change ID:** devrix-layer-isolation-v1.1
**Demand ID:** DM-20260612-012
**Parent Change:** devrix-layer-isolation (v1.0)

> **v1.0 文档债务说明 (2026-06-18):** 本设计文档作为 v1.1 follow-up 补全；v1.0 的 design.md / tasks.md 仍未补全（按用户 2026-06-18 决策保留为 v1.0 文档债务，不再回填）。

## 1. 设计原则

- **最小迁移**: 仅做"物理化"，不改语义、不改具象实现
- **隐式实现**: 具象类型不显式 `var _ Interface = (*Concrete)(nil)`（避免破坏现有 mock 与 DI）
- **向后兼容**: 接口位置选择当前具象实现的同包或邻近包，不引入跨层 import

## 2. 4 接口落地映射

| 接口 | 文件 | 来源具象类型 | 备注 |
|------|------|-------------|------|
| ILLMGateway | `internal/layers/llmgateway/interfaces.go` | LLMGateway | 至少含 `Complete(ctx, req)` 方法签名 |
| IToolRunner | `internal/layers/contextengine/enforce/toolrunner/contracts.go` | ToolRunner / FreeForkSurface | 含 `Run(ctx, call)` / `Register(plugin)` |
| IToolRegistry | `internal/layers/contextengine/enforce/registry/contracts.go` | ToolRegistry | 含 `Get(name)` / `List()` |
| IPermissionGate | `internal/layers/communication/permission/interfaces.go` | PermissionGate | 含 `Check(ctx, action)` |

## 3. 验收机制

- 编译通过 = `go vet ./...` 无错误 = 具象类型满足接口（隐式）
- 不需要单独的"接口实现测试"——`go vet` 已覆盖
- 行为不变 = 现有所有单元测试 + 集成测试不变

## 4. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 隐式实现导致外部包无法发现接口 | 在接口文件加 godoc 注释，注明"具象类型隐式实现此接口" |
| 后续重构破坏接口签名 | v1.0 design 债务保留——后续 devrix-tool-surface-contract 已加 ToolSurface 拆面契约，互不干扰 |
| v1.0 design.md 缺失导致 v1.1 缺少上游参考 | 不回填；本 design 已自包含 |

## 5. 已知债务（明确不修复）

- v1.0 design.md / tasks.md 缺失 → 保留为 v1.0 文档债务
- 接口未配 mock 实现 → 由后续变更（如 devrix-tool-surface-contract）按需补
- 接口未注册到 `openspec/specs/d{N}-*/` → 留待 d7-sa-refine 后续迭代