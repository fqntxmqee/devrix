---
demand-id: DM-20260610-011
title: D6 Eval Phase 4 — CI fast check + delta gate
source: devrix-d6-eval-phase3 follow-up
priority: P2
status: ACCEPTED
l1-domain: evolution
created: 2026-06-10
parent-demand: DM-20260610-010
---

# D6 Eval Phase 4

CI/CD 集成：PR 门禁快速评测 + delta 回归阻断。

## 验收

- `scripts/eval/run-eval.sh` 可在 CI 中运行（mock judge，分层抽样 ≤20 条）
- `devrix eval run --baseline --gate --summary` 在 regression 时非零退出
- 提交 `openspec/eval-datasets/v1/baseline.yaml` 作为质量基线
- CI workflow 增加 eval-fast-check job
