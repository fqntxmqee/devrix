---
demand-id: DM-20260610-009
title: D6 Eval Phase 2 — PEV probe, real Judge, tune
source: devrix-d6-eval-cli follow-up
priority: P1
status: ACCEPTED
l1-domain: evolution
created: 2026-06-10
parent-demand: DM-20260610-008
---

# D6 Eval Phase 2

按顺序交付：

1. PEV Tool 选择准确率探针（L5-6-3-06）
2. `devrix eval run` 接真实 LLM-as-Judge（L5-6-3-13）
3. `tune.go` 调优建议生成器（L5-6-3-12）

## 验收

- PEV probe 输出 precision/recall/F1，数据集含 3 条用例
- CLI `--mock-judge=false` 从 `devrix.yaml` 加载 llm_gateway
- Delta regression 时 EvalReport 含 TuneSuggestion
