# Design: D5/D6 — 信誉、置信度与惩罚闭环

**Change ID:** devrix-reputation-feedback-loop
**Demand ID:** DM-20260614-008

> **归档说明 (2026-06-18):** 设计未完成（仅 S1 阶段），变更已取消。本文档记录初步思路作为历史参考。

## 1. 模块设计（草案）

### 1.1 信誉（Reputation）— D5 Observability

```
internal/layers/observability/reputation/
├── store.go           # 信誉记录持久化（in-memory + 周期性 flush）
├── scoring.go         # 信誉评分算法
├── decay.go           # 时间衰减
└── reputation_test.go
```

**数据结构**：
```go
type AgentReputation struct {
    AgentID    string
    Score      float64       // 0.0 - 1.0
    LastUpdate time.Time
    History    []ReputationEvent
}
```

### 1.2 置信度（Confidence）— D5 Observability

```
internal/layers/observability/confidence/
├── scorer.go          # 置信度评分（基于 LLM self-eval + 多次采样一致性）
├── aggregator.go      # 多源置信度聚合
└── confidence_test.go
```

**算法**：
- Self-eval: LLM 自评分数
- Consistency: 多次采样结果一致性
- 聚合：`confidence = 0.6 * consistency + 0.4 * self_eval`

### 1.3 惩罚（Penalty）— D6 Evolution

```
internal/layers/evolution/penalty/
├── policy.go          # 惩罚策略
├── enforcement.go     # 惩罚执行（限制调用频率 / 降级工具权限）
└── penalty_test.go
```

## 2. 闭环流程

```
Agent 提交结果
    ↓
[信誉加权] ← 历史信誉
    ↓
[置信度评估] ← self_eval + consistency
    ↓
[惩罚决策]
    ├─ 高信誉 + 高置信度 → 采纳
    ├─ 高信誉 + 低置信度 → 人工 review
    ├─ 低信誉 + 高置信度 → 自动重试
    └─ 低信誉 + 低置信度 → 拒绝 + 惩罚
```

## 3. 上游依赖（缺失）

- `devrix-d1-sa-refine` v1.1 的 S/A/F 层注册表重命名 → 未实施
- `internal/layers/observability/reputation/` 命名空间 → 未创建
- `internal/layers/evolution/penalty/` 命名空间 → 未创建

## 4. 取消决策

**Decision (2026-06-18):** 设计仅停留在草案；变更已取消，理由：
1. 依赖项未就绪
2. 实际痛点未达触发阈值
3. 资源优先级 → 让位给 devrix-tool-surface-contract 等活跃变更

## 5. 后续路径

- 监控 Agent 重复博弈场景的实际频率
- 如出现明确痛点 → 基于 `devrix-d1-sa-refine` v1.1 重开
- 可参考文档：demand-archive-index.md 中 DM-20260614-008 行