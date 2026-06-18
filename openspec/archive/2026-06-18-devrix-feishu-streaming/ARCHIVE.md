# S6 归档清单:devrix-feishu-streaming

**Demand ID:** DM-20260611-006
**Change ID:** devrix-feishu-streaming
**归档日期:** 2026-06-18
**归档状态:** s7_archived

---

## 归档说明

飞书 IM 2.0 回复卡 cardkit 元素级流式（打字机）Phase 1–2 代码与单测已完成。
本次 cleanup 将其从 `openspec/changes/` 移动到 `openspec/archive/2026-06-18-devrix-feishu-streaming/`,
作为 S7_Archived 归档。

## 验收结果

| T ID | 描述 | 结果 |
|-------|------|------|
| D1-S2-T04 | Cardkit 双步发卡 | PASS |
| D1-S2-T05 | 元素 PUT sequence 递增 | PASS |
| D1-S2-T06 | cardkit 失败降级 Patch | PASS |
| D1-S2-T07 | complete 关闭 streaming_mode | PASS |
| D1-S2-T08 | 流式节流配置 | PASS |
| — | T14 工具卡 Patch 不干扰 cardkit sequence | PASS |

代码在 master:`feat: QueryLoop v6, Feishu CardKit streaming, YOLO mode, multi-agent isolation` (commit `44ee469`)

## 真机 E2E（Deferred）

5 项真机 E2E 勾选项未签(因 devrix 主要验收渠道是飞书 IM 消息,而 cardkit 真机效果需多客户端验证):
- [ ] `./scripts/devrix.sh restart` 后启动日志含 `cardkit streaming enabled` 提示
- [ ] 飞书发长回复：回复卡可见打字机效果（非整卡闪烁）
- [ ] 同轮含工具调用：工具卡正常 Patch，回复卡流式不中断
- [ ] cardkit 权限缺失时降级 Patch + WARN 日志
- [ ] `im.feishu.streaming.enabled=false` kill switch 生效

后续可作为 v1.1 跟进项单独建卡。

## 裁决

**S5_Accepted (real-device E2E deferred to v1.1)**
