# Proposal: 架构分层 v1.1 — 4 接口契约物理迁移

**Change ID:** devrix-layer-isolation-v1.1
**Demand ID:** DM-20260612-012
**Status:** S7_Archived (2026-06-18)
**Author:** Devrix Team
**Date:** 2026-06-12 → Archived 2026-06-18
**Parent Change:** devrix-layer-isolation (v1.0)

> **v1.1 闭环说明 (2026-06-18)：** v1.0 已通过 `docs/methodology/dsaft-methodology.md` 定义了 ILLMGateway / IToolRunner / IToolRegistry / IPermissionGate 四个接口契约。本 v1.1 仅做"物理迁移"——把已存在于具象实现里的同名方法签名迁移到对应接口文件中，作为后续多实现接入的准备。代码已落 master（commit 44ee469）；设计/任务文档未补全，作为 v1.0 的文档债务保留。

---

## 1. Background

v1.0 (`devrix-layer-isolation`, DM-20260604-003) 完成了架构分层 6 接口的**逻辑契约**定义（ILLMGateway / IToolRunner / IToolRegistry / IPermissionGate / IQueryLoop / IEvolution），落地为 `docs/methodology/dsaft-methodology.md` 中的契约表与目录约束。

v1.0 留有债务：**接口未做物理迁移**——同名方法签名仍写在具象实现里，外部模块若想"多实现接入"或"mock 注入"必须复制具象代码。

## 2. Problem Statement

| 问题 | 影响 |
|------|------|
| 接口契约未物理化为 Go interface | 多实现/mock 接入需复制具象代码 |
| ILLMGateway / IToolRunner / IToolRegistry / IPermissionGate 4 个核心接口未迁移 | 阻碍 devrix-tool-surface-contract 等后续变更的接口层支撑 |
| v1.0 仅写文档级契约，未对应代码层 | 文档与代码割裂，违反"代码是真相"的 devrix 原则 |

## 3. v1.1 Scope

**只做 4 接口的物理迁移**（最小动作）：
- ILLMGateway → `internal/layers/llmgateway/interfaces.go`
- IToolRunner → `internal/layers/contextengine/enforce/toolrunner/contracts.go`
- IToolRegistry → `internal/layers/contextengine/enforce/registry/contracts.go`
- IPermissionGate → `internal/layers/communication/permission/interfaces.go`

每个接口至少包含 1 个具象实现的对应方法签名（method-set 一致），具象类型通过 `_ = (*Concrete)(*interface{})` 形式隐式实现接口（无需显式声明，避免破坏现有调用）。

## 4. Non-Goals

- 不重命名具象类型
- 不调整目录结构
- 不引入新接口
- 不补 v1.0 的 design.md / tasks.md 债务（明确保留为 v1.0 文档债务）

## 5. Acceptance Criteria

| AC | 描述 | 状态 |
|----|------|------|
| AC1 | ILLMGateway 在 `internal/layers/llmgateway/interfaces.go` 定义，至少 1 个具象类型隐式实现 | ✅ PASS |
| AC2 | IToolRunner 在 `internal/layers/contextengine/enforce/toolrunner/contracts.go` 定义 | ✅ PASS |
| AC3 | IToolRegistry 在 `internal/layers/contextengine/enforce/registry/contracts.go` 定义 | ✅ PASS |
| AC4 | IPermissionGate 在 `internal/layers/communication/permission/interfaces.go` 定义 | ✅ PASS |
| AC5 | 全部 `go vet ./...` 与 `go test ./...` 通过 | ✅ PASS |

## 6. 归档

**Status:** S7_Archived (2026-06-18)
**Verdict:** v1.1 follow-up 完成；4 接口物理迁移已落 master；v1.0 design/tasks 文档债务保留。