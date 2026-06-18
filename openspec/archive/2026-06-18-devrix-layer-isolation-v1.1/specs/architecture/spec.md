# Spec: 架构分层 v1.1 — 4 接口契约物理迁移

**Change ID:** devrix-layer-isolation-v1.1
**Demand ID:** DM-20260612-012
**Status:** S7_Archived (2026-06-18)

## 1. 接口契约清单

| 接口 | 所在包 | 方法签名（至少） | 实现类型 |
|------|--------|----------------|---------|
| ILLMGateway | `internal/layers/llmgateway` | `Complete(ctx, req) (resp, error)` | LLMGateway |
| IToolRunner | `internal/layers/contextengine/enforce/toolrunner` | `Run(ctx, call) (result, error)` / `Register(plugin)` | ToolRunner |
| IToolRegistry | `internal/layers/contextengine/enforce/registry` | `Get(name) (Tool, error)` / `List() []Tool` | ToolRegistry |
| IPermissionGate | `internal/layers/communication/permission` | `Check(ctx, action) (Decision, error)` | PermissionGate |

## 2. 隐式实现规则

- 具象类型不显式 `var _ ILLMGateway = (*LLMGateway)(nil)`
- `go vet` 自动校验具象类型满足接口（隐式）
- godoc 注释在接口文件顶部注明"具象类型隐式实现"

## 3. 上游约束

- v1.0 已定义接口契约（`docs/methodology/dsaft-methodology.md`）
- v1.1 仅做物理迁移，契约不变

## 4. 后续兼容性

- 不影响 devrix-tool-surface-contract 拆面契约
- 不影响 devrix-d7-sa-refine 的 S 层注册表
- 隐式实现保证现有 mock/di 代码无需改动

## 5. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** 4 接口物理迁移完成；代码已落 master（commit 44ee469）。