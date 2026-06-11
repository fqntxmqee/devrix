# Proposal: 飞书 IM 2.0 流式更新

**Change ID:** devrix-feishu-streaming  
**Demand ID:** DM-20260611-006  
**Status:** S3_Planning

## 1. Background

devrix 飞书适配已完成 JSON 2.0 结构化卡片（思考 / 工具 / 回复 / 任务分卡），但回复正文仍采用 **全卡 Patch** 伪流式：每个 LLM text chunk 重建整张卡片 JSON 并调用 `Im.Message.Patch`。这导致：

- 飞书客户端整卡重绘，视觉闪烁
- 流式中间态 Markdown（未闭合代码块等）渲染错乱
- 无法利用飞书 2.0 `streaming_mode` 与 cardkit 元素级打字机动画

cc-connect 已在生产环境验证 cardkit 双步发卡 + 元素 PUT 路径；devrix-feishu-cards 变更将 RichCardTextStreamer 标为 P2 backlog，至今未实现。

## 2. Problem Statement

| 问题 | 位置 | 影响 |
|------|------|------|
| 无 cardkit 卡片实体 | `feishu.go` | 无法元素级流式 |
| 无 `streaming_mode` / `element_id` | `feishu_card.go` | 非 2.0 流式语义 |
| 每 chunk 全卡 Patch | `feishu_progress.go:appendResponseText` | 闪烁 + Markdown 中间态差 |
| 无节流 | — | 易触发飞书限流 230020 |

## 3. Alternatives Considered

### 方案 A：Cardkit 元素流式（推荐）

对标 cc-connect：`createCardEntity` → 元素 PUT 流式 → `updateCardEntity` 结束。

- **优点**：原生打字机、Markdown 更稳、与 cc-connect 对齐
- **缺点**：需 cardkit 权限；实现量 ~4–5 人天

### 方案 B：继续全卡 Patch + 优化节流

仅加节流、减少 Patch 频率，不改 cardkit。

- **优点**：改动小（~1 人天）
- **缺点**：不解决闪烁与 Markdown 中间态根因

### 方案 C：不做

维持现状。

- **优点**：零成本
- **缺点**：v3.0 Feishu 流式 UX 目标无法达成；用户 IM 体验持续劣于 cc-connect

## 4. Decision

选择 **方案 A**。cardkit 失败时自动降级方案 B（现有 Patch），保证可用性。

## 5. What Changes

1. 新增 `feishu_cardkit.go`：cardkit Create / Element PUT / Card PUT 封装
2. 回复卡 JSON 增加 `streaming_mode: true`、`element_id: reply_text`
3. `appendResponseText` 分流：有 card_id → 元素 PUT；无 → Patch
4. `finalizeStructuredSession` / `complete`：关闭 streaming + footer
5. 配置 `im.feishu.streaming`（enabled、interval_ms、min_delta_chars）
6. L5-1-2-04~08 测试用例

## 6. Capabilities

| Capability | L1 | 说明 |
|------------|-----|------|
| `communication` | communication | IM 出站卡片流式语义（新增 delta spec） |

## 7. Impact

| 区域 | 影响 |
|------|------|
| `internal/layers/communication/adapters/` | 主要改动 |
| `internal/shared/config/user.go` | 新增配置 |
| `internal/bootstrap/im_hosts.go` | 接线 |
| Gateway / Context Engine | 无变更 |
| 飞书开放平台 | 需确认 cardkit scope |

## 8. Scope

### In Scope

见 `demand.md` §4.2

### Out of Scope

见 `demand.md` §4.2；queryloop v3 T26 worker 任务树卡片

## 9. Goals (SLO)

| 指标 | 目标 |
|------|------|
| 首字可见延迟 | 与现网持平（< 2s，不含 LLM） |
| 流式 PUT 频率 | 可配置，默认 ≤ 3 QPS / 会话 |
| 降级可用性 | cardkit 不可用时 Patch 路径 100% 可用 |
| P0 L5 | 全部 PASS 方可 S5 验收 |

## 10. Risks & Mitigations

| 风险 | 缓解 |
|------|------|
| cardkit 权限未开通 | 降级 Patch + 启动日志 WARN |
| sequence 竞态 | per-session mutex |
| 限流 230020 | 跳过帧、计数 metrics（P2） |
| 客户端版本 | 文档注明 7.20+ |

## 11. Timeline

| 阶段 | 内容 | 工作量 |
|------|------|--------|
| Phase 1 | Cardkit 基础设施 + 双步发卡 | ~1.5d |
| Phase 2 | 元素流式 + 节流 + complete | ~2d |
| Phase 3 | 多卡协同 + 真机验收 | ~1.5d |

**合计 ~4–5 人天**

## 12. Success Criteria

- [ ] P0 L5（L5-1-2-04~07）全绿
- [ ] 飞书真机：回复打字机效果可见
- [ ] cardkit 关闭时降级 Patch 内容完整
- [ ] `openspec/specs/communication/spec.md` 归档合并（S7）
