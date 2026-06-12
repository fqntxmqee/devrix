---
demand-id: DM-20260612-012
title: 架构分层 v1.1 — 4 接口契约物理迁移 + design/tasks.md 补全
source: devrix-layer-isolation v1.0 S4-Gate WARN（2026-06-12）
priority: P0
status: S1_Proposal
l1-domain: architecture
parent_change: devrix-layer-isolation
created: 2026-06-12
---

# 架构分层 v1.1

## 1. 背景

`devrix-layer-isolation` v1.0 S4-Gate 给出 **⚠️ WARN** 裁决（2026-06-12）：
- **D2→D1 反向 import 全部清理、Layer-Lint 落地、跨层契约注册表（Go code）已交付、LayerViolationProbe 注册 D6** — S4 v1.0 范围内 ✅
- **4 个跨层接口（`ILLMGateway` / `IToolRunner` / `IToolRegistry` / `IPermissionGate`）当前仍在 `contextengine/contracts.go` 内** — 仅 `# DEPRECATED` 标记，**未做物理迁移**
- **`design.md` / `tasks.md` 缺失**（v1.0 仅交付 demand.md / proposal.md / acceptance-report.md）

本 v1.1 跟进项目标：完成 4 个接口的物理迁移 + 补全 OpenSpec 文档。

## 2. 接口迁移映射

| 接口 | 当前位置 | v1.1 目标位置 | 理由 |
|------|----------|---------------|------|
| `ILLMGateway` | `contextengine/contracts.go` | `internal/layers/llmgateway/contracts.go` | D3 LLM Gateway 是 LLM 调用接口的归属域；D2 是消费者 |
| `IToolRunner` | `contextengine/contracts.go` | `internal/layers/contextengine/toolrunner/contracts.go` | 工具执行域内（D2 子域）— 接口使用就近 |
| `IToolRegistry` | `contextengine/contracts.go` | `internal/layers/contextengine/registry/contracts.go` | 工具注册域内（D2 子域）— 接口使用就近 |
| `IPermissionGate` | `contextengine/contracts.go` | `internal/layers/communication/contracts.go` | 权限判定属 D1 communication；D2 是消费者 |

## 3. 验收标准

### P0（阻止合并）

- [ ] 4 个接口物理迁移完成，`contextengine/contracts.go` 移除对应定义
- [ ] 所有引用方（`grep -r "contextengine.ILLMGateway\|contextengine.IToolRunner\|contextengine.IToolRegistry\|contextengine.IPermissionGate"`）切换为新路径
- [ ] `go build ./...` 通过
- [ ] `tools/layer-lint` 通过（v1.0 已交付）

### P1（必须完成）

- [ ] 补全 `devrix-layer-isolation/design.md`：接口迁移细节、回归风险、迁移顺序图
- [ ] 补全 `devrix-layer-isolation/tasks.md`：迁移任务拆解（接口 / 引用方 / 测试 / 文档）
- [ ] `LayerViolationProbe` 增加"4 接口迁移完成"维度（v1.0 probe 仅检查 D2→D1 import，v1.1 扩展检查接口定义位置）

### P2（建议完成）

- [ ] 自动化架构合规报告，PR 评论中标注分层违规（v1.0 v2.0 跟进项，本次前移）

## 4. 依赖与顺序

```
v1.0（已合）→ v1.1（本需求）→ v1.2 自动化 PR 评论
```

## 5. 回归风险

- 接口迁移后所有调用方需同步更新引用路径（grep 全量检查）
- 需确保移入目标域后不产生循环依赖（v1.0 layer-lint 已能检测）
- 接口实现类（如 `llmgateway.RealGateway`）需同时迁移到目标域
