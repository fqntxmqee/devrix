---
proposal-id: devrix-d6-validation-metric
demand-id: DM-20260614-002
status: S2_Proposal
created: 2026-06-14
last-updated: 2026-06-14
---

# D6 校验指标可观测性 — 提案

## 1. 目标

把 D7 → D6 advisory 校验从"沉默同意"反模式改造为"可观测可告警"，分两阶段：

**v1.0 P1（本 change）**：指标可观测 + 进程内 WARN log 告警

**v1.1+**：与 Prometheus AlertManager 规则集成（运维侧接入，留作下个 change）

## 2. 方案概览

```
D7 SessionOrchestrator.ProcessMessage
   │
   ├── ClassifyIntent
   │
   └── if validator != nil
        ├── t0 := now
        ├── defer guard
        ├── vctx with 50ms timeout
        ├── result := validator.ValidateOrchestration(...)
        ├── elapsed := now - t0
        └── d6Metrics.Record(result, elapsed)
            │
            ├── if panic recovered           → counter(error) + WARN log
            ├── if elapsed > 2*timeout (100ms) → counter(error) + WARN log
            ├── if elapsed > timeout (50ms)    → counter(timeout) + WARN log
            ├── if result.Pass == true         → counter(pass)
            └── else                          → counter(fail)
```

## 3. 替代方案评估

### 方案 A：直接 Record 后让 Prometheus alert 触发告警

- **优点**：与现有 D5 exporter 路径一致，运维标准化
- **缺点**：本仓库无 AlertManager 集成（v1.0 P1 范围外），需运维侧额外部署
- **结论**：v1.1 路线

### 方案 B：方案 A + 进程内 WARN log 告警（本 change 选定）

- **优点**：v1.0 不依赖外部基础设施，立即可见
- **缺点**：双轨制（log + counter）需对齐
- **结论**：✅ 选定

### 方案 C：仅 log，无 counter

- **优点**：最小实现
- **缺点**：与 S4 已落地的 D5 multiagent metrics 风格不一致
- **结论**：❌ 拒绝（与"指标作为唯一观察位"原则冲突）

## 4. 关键决策

| 决策点 | 选项 | 选定 | 理由 |
|--------|------|------|------|
| Metric 类型 | counter / histogram | counter × 4 | R2 §5 P1 明确指定 4 counter |
| Timeout 检测 | 单阈值 (50ms) / 双阈值 (50ms+100ms) | 双阈值 | timeout 50ms = 校验慢，error 100ms = 实际故障 |
| Rate 滑窗 | 5min fixed / 5min sliding | 5min sliding | R2 §5 P1 明确"连续 5min" |
| Alert 触发 | log only / log + hook | log + hook | hook 留给 v1.1 AlertManager 集成 |
| Metric 注入路径 | 进程内 / 通过 d7.WithMetrics() | WithMetrics() option | 与现有 WithSink/WithValidator 模式一致 |

## 5. 任务分解

| Task | Phase | DSAFT | T 测试点 | 优先级 |
|------|-------|-------|----------|--------|
| T-DV-A01 | A | — | — | P0 |
| T-DV-B01 | B | D7-D6 | D7-D6-T01 | P0 |
| T-DV-C01 | C | D5-METRICS | D7-D6-T03 | P0 |
| T-DV-C02 | C | D5-ALERT | D7-D6-T04 | P0 |
| T-DV-D01 | D | D7-OBS | 全 P0 | P0 |

## 6. 风险与回滚

- **风险 1**：D6 校验慢时 D7 主路径被拖 100ms+（双阈值 error 兜底）
  - **缓解**：`vctx, cancel := context.WithTimeout(...)` 已被现有代码使用，超时返回是 Go context 默认行为
- **风险 2**：D5 counter 注册名冲突
  - **缓解**：使用 `orchestration.d6.validation.*` 命名空间，与 `multiagent.policy.*` 风格一致
- **回滚**：删 `WithMetrics` option 调用，counter 仍注册但无累加（成本可忽略）

## 7. 评审关注项

1. 双阈值 (50ms / 100ms) 是否合理？需要 R3 review。
2. WARN log 频率是否会被校验风暴撑爆？需要测试 1000+ QPS 下 log 行为。
3. WithMetrics option 与 WithValidator 的初始化顺序是否需要约束？
