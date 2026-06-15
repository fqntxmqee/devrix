---
change-id: devrix-d7-turn-orchestration
demand-id: DM-20260614-020
review-phase: S3-Gate
verdict: APPROVED
date: 2026-06-15
reviewer: Owner（架构一致性审查）
---

# S3-Gate Review — D7 Turn 编排上移

## 1. 审查范围

| 维度 | 范围 |
|------|------|
| Change | `devrix-d7-turn-orchestration`（DM-20260614-020） |
| Phase | **S3 Design**（R1 决议闭合后） |
| 审查文档 | demand.md, proposal.md, design.md, gaming-analysis.md, bilateral-consensus.md |
| 不在审查范围 | v2.0 实现细节（slice a–f 待 Phase D） |

## 2. 逐项审查

### 2.1 设计完整性

| # | 检查项 | 证据 | 裁决 |
|---|--------|------|------|
| D1 | 目标接口契约明确 | design.md §2.1–2.3: TurnOrchestrator + ContextPreparer + ToolRoundExecutor + SessionPersister + Legacy adapter | ✅ |
| D2 | Turn 状态机完整 | design.md §3: START→PREPARE→ROUTE+LLM→TOOL_ROUND→PERSIST→COMPLETE，含 CompressHint 循环 | ✅ |
| D3 | Gherkin sad path 覆盖 | design.md §4: Breaker open、StreamChat timeout、Cancel propagate、SubQuery nested turn | ✅ |
| D4 | 跨域边界定义清晰 | design.md §7: D7↔D3 boundary 新建；cross-domain-boundaries.md 修订；D2→D3 禁止 | ✅ |
| D5 | 迁移路径分 slice | design.md §8: a–f 6 个 slice，bootstrap 接线明确 | ✅ |
| D6 | 回滚策略 | design.md §11: Legacy adapter 1 周期 + 日志 deprecation | ✅ |
| D7 | T 层映射草案 | design.md §6: Legacy→Canonical T 表；新增 4 个 P0 T | ✅ |

### 2.2 边界一致性

| 检查项 | 现状 | 目标 | 裁决 |
|--------|------|------|------|
| D2→D3 import | `query/adapters.go` 直连 D3 | D2 移除 ILLMGateway | ✅ 边界闭合 |
| D7→D3 | 黑盒 QueryLoopExecutor | TurnOrchestrator 直调 | ✅ Leader 获得 LLM 权 |
| Autocompact | D2 调 D3 摘要 | D7 调 D3，降级策略明确 | ✅ G-09 双边共识 |
| SubQuery LLM | D2 nested 内循环 | D7 包装 TurnScopeSubQuery | ✅ 与 Hub-Spoke 一致 |

### 2.3 灰区裁决

| 灰区 | 设计裁决 | 双边共识 | 裁决 |
|------|---------|---------|------|
| Autocompact 降级 | Truncation → Retry → Error | G-09 | ✅ |
| SubQuery 递归深度 | MaxDepth = 3 | G-08 | ✅ |
| D2-THIN-T01 lint | CI 硬阻断，博弈论 commitment device | G-10 | ✅ |
| Breaker sad path | EngineEvent error, no panic | G-12 | ✅ |

### 2.4 与已完成变更的关系

| 已完成 Change | 关系 | 冲突？ |
|-------------|------|--------|
| DM-018 D4 Hub-Spoke | 互补：D7 同时拥有 LLM 调用权 + Hub-Spoke 编排权 | 无冲突 |
| DM-009 D2 SA Refine | D2-S16 Legacy 冻结与 DM-009 Canonical 对齐 | 无冲突 |
| DM-008 D7 SA Refine | D7-S2 扩展 A06/A07 与 SA Refine 一致 | 无冲突 |
| DM-016 D3 SA Refine | ILLMGateway 消费方 D2→D7，Bridge 不变 | 无冲突 |

## 3. 风险评估

| 风险 | 等级 | 缓解 | 裁决 |
|------|------|------|------|
| FastPath 回归 | HIGH | slice c 独立 PR；P0 T 先绿 | 可接受 |
| Wave Worker 仍旧路径 | MEDIUM | v2.0-f 后改 wave/runners | 可接受 |
| 双接线期混乱 | MEDIUM | Legacy adapter 1 周期 | 可接受 |
| v1.0 零 Go 变更承诺 | LOW | Phase B 仅文档 | 可接受 |

## 4. 裁决

**Verdict: APPROVED — S3-Gate 通过**

准出条件均已满足：
- ✅ R1 决议 6 项全部闭合（demand.md §0）
- ✅ 双边共识 G-01~G-12 全部确认
- ✅ 接口契约、状态机、Gherkin 完整
- ✅ 灰区降级策略明确

可进入 **Phase B（v1.0 Registry）** 。

---

**Revision History**

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-15 | S3-Gate APPROVED |
