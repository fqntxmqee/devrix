# Demand: D1 DSAFT 架构重构

**Demand ID:** DM-20260628-003  
**Created:** 2026-06-28  
**Priority:** P1

---

## 1. 原始诉求

用 DSAFT 方法论对 D1 Communication 做整体重构：

- 删除冗余代码，清理不必要的扩展性
- 逻辑回归最初设计：**Trusted Intermediary**，只管用户可感知的进/看/收
- **D1 编排边界仅 D7**：入站 dispatch + EngineEvent 出站展示
- 四条流 + 命令通道：
  - **指令流** → S13 CaptureUserIntent
  - **思考流** → S14 PresentThinking
  - **任务流** → S15 PresentTaskProgress
  - **汇总信息流** → S16 DeliverConclusion
  - **命令通道** → S13-A05 + S17 ConnectChannel

## 2. 决策（2026-06-28）

| # | 问题 | 决议 |
|---|------|------|
| 1 | DingTalk | **保留** |
| 2 | 多 IM 实例 registry | **保持现状** |
| 3 | D4 Agent provision | **迁出 D1** → bootstrap / D7 侧 |

## 3. 可验证承诺

- [x] D1 `capture/` 生产代码 **零 import** `multiagent`、`orchestration/*`（channel Phase 3）
- [x] 入站仍仅 `IOrchestrationEntry.ProcessMessage`（DM-007 不回退）
- [x] 出站仍经 `SignalRouter` → S14/S15/S16 → S18 EventBus
- [x] Session leader agent 生命周期由 `bootstrap/sessionagents` 持有
- [x] DingTalk + instance registry 行为不回归（adapter 测试 + 保留决策 #1/#2）
- [x] Gateway 拆分 + channel DTO 解耦（Phase 2–3，见 design.md §9）

## 4. 不在范围

- D7 内部编排重构
- 删除 Feishu CardKit / Worker 双卡
- D5/D6 契约变更（仅 D1 侧 import 收敛）
