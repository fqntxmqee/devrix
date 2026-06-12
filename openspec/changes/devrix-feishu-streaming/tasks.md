# Tasks: devrix-feishu-streaming

**Demand ID:** DM-20260611-006  
**Design:** [design.md](./design.md)

---

## Phase 1 — Cardkit 基础设施（~1.5d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T1 | 新增 `feishu_cardkit.go`：`CreateCard` / `StreamElementContent` / `UpdateCard` | cardkit | D1-S2-T04 | ~180 | [x] |
| T2 | `feishu_cardkit_test.go`：mock HTTP + sequence 断言 | cardkit | D1-S2-T04 | ~120 | [x] |
| T3 | `BuildStreamingReplyCardJSON(streaming bool)` + `element_id: reply_text` | stream-reply | D1-S2-T04 | ~80 | [x] |
| T4 | 扩展 `feishuSessionStream`：`replyCardID`、`cardkitSequence`、`cardkitEnabled` | stream-reply | — | ~60 | [x] |
| T5 | 首包 `appendResponseText`：createCardEntity → card_id 引用回复 | stream-reply | D1-S2-T04 | ~150 | [x] |
| T6 | cardkit 失败降级：inline JSON + Patch（打 WARN 日志） | stream-reply | D1-S2-T06 | ~100 | [x] |

**Phase 1 验收**：D1-S2-T04、D1-S2-T06 单测绿 ✅

---

## Phase 2 — 元素流式 + 节流 + 结束（~2d）

| # | 任务 | L4 | L5 | PR 估算 | 状态 |
|---|------|-----|-----|---------|------|
| T7 | 后续 chunk 走 `StreamElementContent`（累积全文） | stream-reply | D1-S2-T05 | ~120 | [x] |
| T8 | 统一 sequence mutex（元素 PUT 与全卡 PUT 共用） | cardkit | D1-S2-T05 | ~60 | [x] |
| T9 | 限流 230020：跳过本帧、不降级 | cardkit | D1-S2-T05 | ~40 | [x] |
| T10 | `feishu_stream_throttle.go` + 配置默认值 | stream-reply | D1-S2-T08 | ~100 | [x] |
| T11 | `complete` / `finalizeStructuredSession`：最终 PUT + 关 streaming | stream-reply | D1-S2-T07 | ~120 | [x] |
| T12 | `user.go` + `im_hosts.go`：`im.feishu.streaming` 配置接线 | — | D1-S2-T08 | ~60 | [x] |
| T13 | `streaming.enabled=false` kill switch 测试 | stream-reply | D1-S2-T06 | ~40 | [x] |

**Phase 2 验收**：D1-S2-T05~08 单测绿 ✅

---

## Phase 3 — 多卡协同 + 验收（~1.5d）

| # | 任务 | L4 | L5 | PR 估算 |
|---|------|-----|-----|---------|
| T14 | 确认工具卡 Patch 与回复 cardkit 无 sequence 干扰（文档+测试） | stream-reply | — | ~40 | [x] |
| T15 | 飞书真机验收清单 + `acceptance-report.md` 草稿 | — | 全部 P0 | ~60 | [x] |
| T16 | 启动日志：cardkit 权限缺失时 WARN 指引 | cardkit | — | ~30 | [x] |
| T17 | 更新 `openspec/t-registry.md` 登记 D1-S2-T04~08 | — | — | ~40 | [x] |

**Phase 3 验收**：P0 T 层 全 PASS + 真机打字机可见

---

## 依赖顺序

```
T1 → T2 → T3 → T4 → T5 → T6
              ↘ T7 → T8 → T9
              ↘ T10 → T11 → T12 → T13
                        ↘ T14 → T15 → T16 → T17
```

## 分支建议

`feat/devrix-feishu-streaming` 或 `feat/DM-20260611-006-feishu-streaming`

## S4 开发准入

- [x] `demand.md` 已澄清（S2）
- [x] `proposal.md` / `design.md` / `specs/` / `tasks.md` 齐全（S3）
- [ ] 用户确认规划后进入 S4 编码

## S5 验收命令（草案）

```bash
go test ./internal/layers/communication/adapters/... -count=1 -run 'Cardkit|Stream|Throttle'
./scripts/devrix.sh restart
# 飞书真机：发长回复 + 多工具调用场景
```

## 不在本变更

- queryloop v3 T26 worker 任务树卡片
- 思考卡 element 流式（后续 v1.2 可选）
