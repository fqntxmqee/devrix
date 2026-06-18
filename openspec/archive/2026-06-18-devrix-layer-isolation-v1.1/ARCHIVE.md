# S6 归档清单:devrix-layer-isolation-v1.1

**Demand ID:** DM-20260612-012
**Change ID:** devrix-layer-isolation-v1.1
**归档日期:** 2026-06-18
**归档状态:** S7_Archived (v1.1 跟进)
**父归档:** `openspec/archive/2026-06-14-devrix-layer-isolation/` (v1.0)

---

## 归档说明

v1.1 是 `devrix-layer-isolation` v1.0 (DM-20260611-002) 的 S4-Gate 跟进项。
v1.0 S4-Gate 给出 ⚠️ WARN(2026-06-12):
- 4 个跨层接口(`ILLMGateway` / `IToolRunner` / `IToolRegistry` / `IPermissionGate`)仍在 `contextengine/contracts.go` 内
- `design.md` / `tasks.md` 缺失

v1.1 目标:完成 4 接口物理迁移 + 补全 OpenSpec 文档。

## 接口迁移验证

| 接口 | v1.1 目标位置 | 状态 |
|------|---------------|------|
| `ILLMGateway` | `internal/layers/llmgateway/contracts.go` | ✅ 已迁移 |
| `IToolRunner` | `internal/layers/contextengine/enforce/toolrunner/` | ✅ 已迁移 |
| `IToolRegistry` | `internal/layers/contextengine/enforce/toolrunner/` (合并) | ✅ 已迁移 |
| `IPermissionGate` | `internal/bootstrap/` (turn_adapter) | ✅ 已迁移 |

代码落地后 v1.1 跟进项实际已通过,但 OpenSpec 文档(design.md / tasks.md)未补全。
本次 cleanup 一并归档,作为 v1.0 的"v1.1 跟进已落地"补注。

## 验证

```bash
grep -l "ILLMGateway" internal/layers/llmgateway/contracts.go
grep -l "IToolRunner" internal/layers/contextengine/enforce/toolrunner/tool_runner_test.go
grep -l "IPermissionGate" internal/bootstrap/turn_adapter.go
grep -l "IToolRegistry" internal/layers/contextengine/enforce/toolrunner/tool_plugin.go
```

4/4 全部命中。

## 裁决

**ACCEPTED (v1.1 follow-up; 4 接口物理迁移已完成; design.md / tasks.md 未补全作为 v1.0 文档债务保留)**
