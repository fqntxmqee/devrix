# D6 Validation Metric — 任务分解

**Demand:** DM-20260614-002
**Status:** S3_Design — 文档就绪
**Last Updated:** 2026-06-14

---

## 任务与 DSAFT 映射

| Task ID | 描述 | Phase | DSAFT | T 测试点 | 优先级 |
|---------|------|-------|-------|----------|--------|
| T-DV-A01 | 编写 demand.md + proposal.md + design.md + tasks.md | A | — | — | P0 |
| T-DV-B01 | 定义 D6ValidationMetrics 数据结构 + Counter 注入 | B | D5-METRICS | D7-D6-T03 | P0 |
| T-DV-C01 | 5min sliding window + timeout_rate 计算 | C | D5-ALERT | D7-D6-T04 | P0 |
| T-DV-C02 | AlertHook 接口 + 默认 WARN log 实现 | C | D5-ALERT | D7-D6-T04 | P0 |
| T-DV-D01 | Orchestrator 集成：WithMetrics option + 计时 + 分流 + panic-guard | D | D7-OBS | D7-D6-T05, D7-D6-T06 | P0 |
| T-DV-D02 | t-registry 同步：T01 IMPLEMENTED, T03/T04/T05/T06 新增 | D | D7 | — | P0 |
| T-DV-E01 | 4 counter 在 MemoryExporter 输出验证 | E | D5-EXPORT | D7-D6-T03 | P0 |
| T-DV-F01 | S5 验收 + acceptance-report | F | — | 全 P0 | P0 |

---

## Phase 依赖

```
A (文档) → B (数据结构) → C (rate + alert) → D (orchestrator 集成) → E (export 验证) → F (验收)
```

---

## 不在本 change 内

- 真实 D6 validator 实现（仍是接口，fake/no-op 注入）
- Prometheus AlertManager rules yaml 推送
- 跨进程 metric aggregation
- 历史回溯（counter 启动清零）
