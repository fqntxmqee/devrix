# Acceptance Report: devrix-layer-isolation

**Demand ID:** DM-20260611-002  
**Change ID:** devrix-layer-isolation  
**Date:** 2026-06-12  
**Status:** S5_PASS（WARN：4 个接口契约迁移 + design/tasks.md 在 v1.1 跟进）

## Summary

完成 D2→D1 反向 import 清理（Layer-Lint CI 门禁）+ 跨层契约注册表文档 + `LayerViolationProbe` 评测探针落地。4 个跨层接口（`ILLMGateway` / `IToolRunner` / `IToolRegistry` / `IPermissionGate`）当前仍在 `contextengine/contracts.go` 内但已**全部标记为 `# DEPRECATED: scheduled for v1.1 migration to shared/contracts`**，v1.1 跟进项中逐个迁出。

## Scope Delivered

| Capability | Status | Note |
|---|---|---|
| LAYER-LINT | ✅ | `tools/layer-lint/layer-lint.go` + `internal/layers/architecture/layer_rules.go` + GitHub Actions 门禁 |
| LAYER-CONTRACT-REGISTRY | ✅ | `docs/architecture/contract-registry.md`（含 6 个跨层接口的定义域 / 消费域 / 迁移状态） |
| LAYER-VIOLATION-PROBE | ✅ | `internal/layers/evolution/eval/layer_violation_probe.go` + `layer_violation_probe_test.go` |
| LAYER-DEPRECATION-MARK | ✅ | 4 个接口在 `contextengine/contracts.go` 标 `# DEPRECATED` + Lint 警告引用 v1.1 |

## Automated Verification

```bash
go test -race -count=1 ./internal/layers/evolution/eval/...
go test -race -count=1 ./tools/layer-lint/...
```

| L5 ID | 描述 | 结果 |
|-------|------|------|
| L5-0-0-01 | Layer-Lint 阻断 D2→D1 反向 import | PASS |
| L5-0-0-02 | Layer-Lint 阻断 D3→D2 反向 import | PASS |
| L5-0-0-03 | LayerViolationProbe 评测探针在 D6 框架内注册 | PASS |
| L5-0-0-04 | Contract Registry 文档含完整 6 接口矩阵 | PASS |

覆盖率：`internal/layers/evolution/eval/` 84.8% ≥ 80% ✅；`tools/layer-lint/` 88.1% ≥ 80% ✅

## v1.1 Follow-ups（已列入 openspec/changes/ 跟进队列）

1. **ILLMGateway 接口迁移**：从 `contextengine/contracts.go` → `internal/layers/llmgateway/contracts.go`（D3 归属域）
2. **IToolRunner 接口迁移**：从 `contextengine/contracts.go` → `internal/layers/contextengine/toolrunner/contracts.go`（工具执行域）
3. **IToolRegistry 接口迁移**：从 `contextengine/contracts.go` → `internal/layers/contextengine/registry/contracts.go`（工具注册域）
4. **IPermissionGate 接口迁移**：从 `contextengine/contracts.go` → `internal/layers/communication/contracts.go`（D1 归属域）
5. 补 `design.md` + `tasks.md`（当前仅 demand.md / proposal.md / acceptance-report.md）

## Known Issues

- 4 个接口未完成物理迁移（v1.1 跟进）
- `design.md` / `tasks.md` 缺失（v1.1 跟进）

## S4-Gate Review

| Reviewer | Verdict | Date |
|---|---|---|
| code-reviewer (opus) | ⚠️ WARN（接口未迁移 + 文档缺） | 2026-06-12 |

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Dev | — | 2026-06-12 | 单测 + Lint PASS |
| QA | — | 2026-06-12 | L5 100% PASS；v1.1 跟进项已开 |
| S4-Gate | code-reviewer | 2026-06-12 | ⚠️ WARN（v1.1 follow-up） |
