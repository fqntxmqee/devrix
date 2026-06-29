# Demand: D7 MUPS 5-node Span 全覆盖 + 目录结构治理

**Demand ID:** DM-20260625-019
**Created:** 2026-06-25
**Reporter:** 用户直接反馈
**Priority:** P0
**Sprint:** d7-v6 follow-up

---

## 原始反馈

> "D7领域的5节点Span是没有生效吗？请修复。另外5节点的目录非常混乱。"

## 1. 现象

D7 5 节点（Observe/Plan/Wave/Execute/Verify/Learn）已 S7_Archived 多个 PR，但生产中：

1. **5 节点 Span 在 coverage scan 中显示 0% 覆盖**——Jaeger 中也无法按 5 节点 Op 检索
2. **mups/execute/**：5 个文件全部以 `channel_` 开头，目录浏览噪音大
3. **mups/learn/**：17 个文件平铺，asset/memory/reputation/prior 4 个子领域无边界

## 2. 影响

- 端到端 5 节点链路无法在 Jaeger 中串联（无共享根 Span）
- D5 可观测性面板显示"5-node Span 未实现"（误报）
- 新成员读 D7 目录需要额外认知开销
- 未来增加 6th node 时没有清晰边界

## 3. 验收标准

- 5 个节点 Span 在 `coverage registry` 中注册，scan 通过
- 新增 `D7_MUPS_Pipeline` 根 Span，5 个子 Span 作为其 children
- `mups/execute/` 文件名无 `channel_` 前缀
- `mups/learn/` 拆为 4 subpackage（asset/memory/reputation/prior）
- 无 import cycle
- 23 orchestration packages `-race` PASS
- 0 函数签名变化（pure physical migration）

## 4. 关联

- 上游：`devrix-d7-six-s-simplification` (DM-20260626-001) v6.0.0 域升级
- 上游：`devrix-d7-mups-v4-phase5-learn` (DM-20260623-003) Learn 节点升格
- 上游：`devrix-d7-mups-package-migration` (DM-20260626-002) 物理包路径迁移
- 后续：D7 维护期可基于 4 subpackage 边界添加新能力
