---
demand-id: DM-20260610-008
title: D6 Eval CLI — devrix eval run
source: devrix-d6-eval pilot follow-up
priority: P1
status: ACCEPTED
l1-domain: evolution
created: 2026-06-10
parent-demand: DM-20260610-006
---

# D6 Eval CLI

暴露 `devrix eval run --dataset <path>` 子命令，离线运行 EvalEngine 并输出 JSON 报告。

## 验收

- `devrix eval run --dataset openspec/eval-datasets/v1/dataset.yaml` 输出 EvalReport JSON
- 支持 `--output` 写文件、`--save-baseline` 保存基线
