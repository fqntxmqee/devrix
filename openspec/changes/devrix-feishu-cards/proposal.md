# Proposal: Feishu Card Adapter Redesign

## Summary

参考 cc-connect 的设计，重新设计 devrix 飞书适配层的卡片系统，实现统一的 Card 模型、完整的元素支持、平台回退机制和进度事件对齐。

## Problem Statement

1. **Card 元素类型有限** - 缺少 CardListItem、CardSelect 等交互元素
2. **无 RenderText 回退** - 不支持卡片的平台会直接失败
3. **进度事件不对齐** - ProgressEntry 类型与 cc-connect 不一致
4. **颜色支持不完整** - 只有 7 种颜色，cc-connect 有 12 种

## Proposed Solution

### 方案 A: 完全对齐 cc-connect (推荐)

完全采用 cc-connect 的 Card 模型和渲染逻辑，确保最大兼容性。

**优点:**
- 与 cc-connect 完全兼容
- 减少维护成本
- 功能完整

**缺点:**
- 需要较大重构
- 可能破坏现有测试

### 方案 B: 增量改进

保留现有 feishu_card.go，增量添加缺失的 Card 类型。

**优点:**
- 改动较小
- 风险低

**缺点:**
- 模型不统一
- 维护成本高

## Decision

选择 **方案 A**，因为：
1. cc-connect 已经过生产验证
2. devrix 的 communication 层设计本来就是参考 cc-connect
3. 统一模型减少长期维护成本

## Scope

### In Scope
- Card 模型重构
- Feishu 卡片渲染器重写
- 12 种颜色支持
- RenderText 回退机制
- ProgressEntry 类型对齐
- 单元测试覆盖

### Out of Scope
- 其他平台适配器修改
- Gateway 核心逻辑变更
- 流式预览接口实现 (P2)

## Risks

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| 破坏现有测试 | 中 | 高 | 先备份测试，逐步迁移 |
| 颜色名称不一致 | 低 | 低 | 对照表验证 |
| 性能下降 | 低 | 中 | benchmark 测试 |

## Timeline

预计 3.25 天完成，见 design.md 实施计划。

## Success Criteria

- [ ] Card 模型与 cc-connect 兼容
- [ ] 12 种颜色全部支持
- [ ] RenderText 回退正常工作
- [ ] four_flows_test.go 测试通过
- [ ] 卡片测试覆盖率 > 70%
