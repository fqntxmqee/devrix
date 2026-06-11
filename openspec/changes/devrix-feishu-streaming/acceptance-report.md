# Acceptance Report: devrix-feishu-streaming

**Demand ID:** DM-20260611-006  
**Change ID:** devrix-feishu-streaming  
**Date:** 2026-06-11  
**Status:** S5_Pending（待真机）

## Summary

飞书 IM 2.0 回复卡 cardkit 元素级流式（打字机）Phase 1–3 代码与单测已完成。思考卡/工具卡仍走 `Im.Message.Patch`，与回复 cardkit sequence 隔离。

## Automated Verification

```bash
go test ./internal/layers/communication/adapters/... -count=1 -run 'Cardkit|Stream|Throttle|ToolCardPatch'
```

| L5 ID | 描述 | 结果 |
|-------|------|------|
| L5-1-2-04 | Cardkit 双步发卡 | PASS |
| L5-1-2-05 | 元素 PUT sequence 递增 | PASS |
| L5-1-2-06 | cardkit 失败降级 Patch | PASS |
| L5-1-2-07 | complete 关闭 streaming_mode | PASS |
| L5-1-2-08 | 流式节流配置 | PASS |
| — | T14 工具卡 Patch 不干扰 cardkit sequence | PASS |

## Manual E2E Checklist（待执行）

- [ ] `./scripts/devrix.sh restart` 后启动日志含 `cardkit streaming enabled` 提示
- [ ] 飞书发长回复：回复卡可见打字机效果（非整卡闪烁）
- [ ] 同轮含工具调用：工具卡正常 Patch，回复卡流式不中断
- [ ] cardkit 权限缺失时降级 Patch + WARN 日志
- [ ] `im.feishu.streaming.enabled=false` kill switch 生效

## Known Issues

- 真机验收未在本报告周期内执行
- Wave Scheduler Worker 卡（DM-007）不在本变更范围

## Sign-off

| Role | Name | Date | Verdict |
|------|------|------|---------|
| Dev | — | 2026-06-11 | 单测 PASS，待真机 |
| QA | — | — | Pending |
