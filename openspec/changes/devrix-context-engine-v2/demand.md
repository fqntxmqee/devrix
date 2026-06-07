---
demand-id: DM-20260607-003
title: 上下文引擎 V2（Autocompact + Verify Commands + Token 统一）
source: 架构/产品
priority: P0
status: S3_READY
l1-domain: devrix
created: 2026-06-07
---

# 上下文引擎 V2

## 1. 原始描述

> V1 上下文引擎已落地（压缩 1–5+7、简化 PEV、Working+ShortTerm 记忆），但存在三类缺口：
> 1. 压缩步骤 6（Autocompact）被跳过，超长对话只能硬截断，丢失语义；
> 2. PEV Verify 仅 `basic` 模式，无法用真实命令（如 `go test`）验证工具结果；
> 3. Token 计数使用 char/4 启发式，与 LLM Gateway 的 cl100k_base 不一致。
>
> 按 OpenSpec 规范，为 Context Engine Layer V2 做完整规划（demand → proposal → design → specs → tasks）。

## 2. 澄清记录

### Q1: V2 与 V3 边界？

**A**: V2 只做 Autocompact、Verify `commands` 模式、Token 计数统一、压缩可观测增强；**不包含** PEV Plan、Milestone DAG、LongTerm Memory（归 V3）。 — 2026-06-07

### Q2: 是否依赖 LLM Gateway？

**A**: **硬依赖** `devrix-llm-gateway` V1 至少提供 `ILLMGateway.ChatStream` 与 `ITokenCounter`（cl100k_base）。Autocompact 与主对话共用 Gateway，摘要使用独立 `fast` model profile。 — 2026-06-07

### Q3: Autocompact 同步还是异步？

**A**: V2 **同步**执行（与 V1 压缩管道一致）；步骤 1–4 目标 <100ms，Autocompact LLM 调用 P99 延迟 <30s（配置 timeout）。异步预摘要留待后续迭代。 — 2026-06-07

### Q4: Verify commands 安全策略？

**A**: 配置为 `executable` + `args[]`（禁止 shell -c）；命令必须在白名单内；`WorkDir` 沙箱（`filepath.Clean` +  trusted root 前缀校验）+ 超时。默认：`go test ./...`、`go vet ./...`。 — 2026-06-07

### Q5: 快照加密？

**A**: V2 **不实施** AES 加密，维持 V1 明文 JSON；加密归后续安全专项。 — 2026-06-07

### Q6: L5-CTX-08（V1 跳过 Autocompact）如何处理？

**A**: V2 启用 Autocompact 后，L5-CTX-08 场景改为「`autocompact.enabled=false` 时仍跳过」；新增 L5-CTX-12/13 覆盖启用路径。 — 2026-06-07

### Q7: Mock LLM 是否保留？

**A**: 单元测试保留 `mock/llm.go`；集成测试默认 recorded fixture；`-tags=live` 可走真实 LLM Gateway。新增 L5-CTX-18 锚定主路径接线。 — 2026-06-07

### Q8: Autocompact 使用哪个模型？

**A**: 配置项 `autocompact.model` 直接指定模型名（如 `deepseek-v4-flash`），**不使用** abstract profile；LLM Gateway 按 `LLMRequest.Model` 路由。 — 2026-06-07

### Q9: 压缩管道步骤 6 插入位置？

**A**: 执行序：**1→2→3→4 → [6 Autocompact] → 5 Assembly → 7 TokenBlock**。步骤 6 在消息历史上操作（不含 system prompt），与 V1 代码插入点一致；canonical spec 步骤编号保持不变。 — 2026-06-07

### Q10: ITokenCounter 接口归属？

**A**: 定义于 `internal/shared/contracts/tokencounter.go`，L2/L3 共同依赖；Gateway 实现，Context Engine 注入适配器。 — 2026-06-07

## 3. 澄清范围

### 3.1 L1-L5 映射

| 层级 | 资产 ID | 名称 | 状态 |
|------|---------|------|------|
| L1 | devrix | 开发大脑 | 已有 |
| L2 | L2-DEVRIX-02 | 对话式开发助手 | 已有 |
| L3-BE | L3-BE-CTX-01 | 处理用户消息并维护上下文 | 复用（Token 统一） |
| L3-BE | L3-BE-CTX-02 | 超长对话压缩 | 复用（Autocompact） |
| L4-BE | L4-CTX-COMPRESS | 七步压缩管道 | 增强（步骤 6） |
| L4-BE | L4-CTX-PEV | PEV 执行循环 | 增强（verify commands） |
| L4-BE | L4-CTX-STATE | 上下文状态管理 | 增强（ITokenCounter 注入） |
| L4-BE | L4-CTX-OBS | 压缩/验证可观测 | **新增** |
| L5 | L5-CTX-12 ~ L5-CTX-18 | 见 `l5-registry.md` | 草拟 |

### 3.2 范围

**In Scope（本变更）**:
- OpenSpec 四件套 + L5 登记
- Autocompact（压缩步骤 6）
- PEV Verify `commands` 模式
- `ITokenCounter` 由 LLM Gateway 注入
- 压缩/Verify 的 Observability span 与 metrics
- `main.go` 替换 Mock LLM 为真实 Gateway（集成路径）

**Out of Scope（本变更）**:
- PEV Plan + Milestone（V3）
- LongTerm Memory SQLite（V3）
- 快照 AES 加密
- 异步 Autocompact / 后台预摘要
- Multi-Agent 完整 ToolRunner（可用 BuiltinVerifyRunner）

### 3.3 前置依赖

| 变更 | 最低版本 | 说明 |
|------|----------|------|
| `devrix-context-engine` | V1 已归档 | 基线实现 |
| `devrix-llm-gateway` | V1 M1+M3 | TokenCounter（M1）+ ChatStream/Adapter（M3） |
