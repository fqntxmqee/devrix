# S6 归档清单:devrix-im-card-ctx

**Demand ID:** DM-20260611-008
**Change ID:** devrix-im-card-ctx
**归档日期:** 2026-06-18
**归档状态:** s7_archived (ACCEPTED P1)

---

## 归档说明

飞书 IM 完成卡"消耗: 0 tokens" + 缺 ctx% 透传的需求,通过以下 3 个 PR 合并到 master:
- PR #27: D1 summary.go + 测试
- PR #28: D2 PEV 链路 span token 完整埋点
- PR #79: emitComplete 携带 lastPromptTokens(finalText 透传修复)

最终落地代码:
- `internal/layers/communication/capture/summary.go` (ctx_pct 计算 + token 链路)
- `internal/layers/orchestration/turn/orchestrator.go` (emitComplete Content + finalText 透传)
- `internal/layers/contextengine/engine_persist.go` (ctx% 计算)
- `internal/layers/communication/capture/summary_test.go` (回归测试)
- `internal/layers/orchestration/turn/orchestrator_test.go` (回归测试)

## 验证证据

- 飞书发送"跑 make lint"等指令,卡片已显示 `用时: X, 消耗: Y tokens, ctx: Z%, 模型: M` ✅
- 飞书卡片底部 done emoji 已观察到 ✅
- PR #79 回归测试 2/2 PASS (no-text + MaxTurns-exceeded) ✅

## 裁决

**ACCEPTED (P1)**
