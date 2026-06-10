# Proposal: devrix-d6-eval-phase2

D6 评测引擎 Phase 2：确定性 PEV 探针、真实 Judge 接入、调优建议生成。

## Capabilities

| ID | 说明 |
|----|------|
| probe-pev-tool-accuracy | PEV tool 选择 precision/recall/F1 |
| eval-cli-real-judge | GatewayLLMClient + `--mock-judge=false` |
| eval-tune | TuneGenerator 基于 delta regressions |

## L5

- L5-6-3-06 PEV Tool 选择准确率探针
- L5-6-3-12 调优建议生成
- L5-6-3-13 eval run 真实 Judge 接入
