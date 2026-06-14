---
tasks-id: devrix-s5-p2-shadow-classifier
title: S5-P2 Tail-only LLM Classify Shadow — 实施任务
demand-id: DM-20260614-004
status: S3_Tasks
created: 2026-06-14
last-updated: 2026-06-14
---

# S5-P2 Tail-only LLM Classify Shadow — 实施任务

## 1. 任务分解

| T ID | 任务 | 归属 A/F | 估算 | 状态 |
|------|------|----------|------|------|
| T-SC-A01 | 定义 `LLMIntentClassifier` 接口（1 方法） | D7-S5-A05 | 10 分钟 | PLANNED |
| T-SC-A02 | 定义 `ShadowMetrics` 结构 + `NewShadowMetrics(meter)` 构造函数 | D7-S5-A05 | 20 分钟 | PLANNED |
| T-SC-A03 | 定义 `ShadowClassifier` 结构 + 构造函数 | D7-S5-A05 | 15 分钟 | PLANNED |
| T-SC-F01 | 实现 `(*ShadowClassifier).Classify` 同步返回 rule + 异步触发 LLM | D7-S5-A05-F01 | 30 分钟 | PLANNED |
| T-SC-F02 | 实现 `(*ShadowClassifier).shadowAsync` 内部方法（panic recovery + metric + log） | D7-S5-A05-F01 | 30 分钟 | PLANNED |
| T-SC-C01 | 在 `config.go` `Config` 结构新增 `ShadowLLMClassify bool` + `ShadowLLMTimeoutMs int` | D7-S5-A05 | 10 分钟 | PLANNED |
| T-SC-C02 | 在 `DefaultConfig()` 设默认 `false` + `500` | D7-S5-A05 | 5 分钟 | PLANNED |
| T-SC-O01 | 在 `orchestrator.go` 新增 `shadowClassifier` 字段 + `WithShadowClassifier` option | D7-S5-S2 | 20 分钟 | PLANNED |
| T-SC-O02 | 在 `ProcessMessage` 接入 `ShadowClassifier`（nil fallback 到原 `classifier`） | D7-S5-S2 | 20 分钟 | PLANNED |
| T-SC-T01 | 编写 `shadow_classifier_test.go` 9 个测试（AC3~AC9 + nil-safe） | D7-S5-A05 | 60 分钟 | PLANNED |
| T-SC-T02 | 跑 `go test -race -count=1 ./internal/layers/d7/...` | D7-S5-A05 | 10 分钟 | PLANNED |
| T-SC-T03 | 跑 `gofmt -l` + `go vet` | D7-S5-A05 | 5 分钟 | PLANNED |
| T-SC-D01 | 更新 `t-registry.md` D7-S5-T07 新增 | D7-S5-A05 | 5 分钟 | PLANNED |
| T-SC-D02 | 编写 `review-code.md` (S4-Gate) | D7-S5-A05 | 15 分钟 | PLANNED |
| T-SC-D03 | 编写 `acceptance-report.md` (S5) | D7-S5-A05 | 20 分钟 | PLANNED |
| T-SC-D04 | S6 归档 | D7-S5-A05 | 10 分钟 | PLANNED |

**总计**：约 4 小时 30 分钟

## 2. 依赖关系

```
T-SC-A01, T-SC-A02, T-SC-A03 (接口/结构) ──┐
                                            ├─▶ T-SC-F01 (Classify 同步) ──▶ T-SC-F02 (shadowAsync)
                                            │                                          │
T-SC-C01, T-SC-C02 (config) ───────────────┤                                          │
                                            │                                          │
T-SC-O01 (orchestrator 字段) ──────────────┤                                          │
                                            │                                          ▼
                                            │                                T-SC-T01 (9 测试)
                                            │                                          │
                                            │                                          ▼
                                            │                                T-SC-T02/T03 (lint+test)
                                            │                                          │
                                            │                                          ▼
                                            │                                T-SC-D01 (t-registry)
                                            │                                          │
                                            │                                          ▼
                                            │                                T-SC-D02 (S4-Gate)
                                            │                                          │
                                            │                                          ▼
                                            │                                T-SC-D03 (S5)
                                            │                                          │
                                            │                                          ▼
                                            │                                T-SC-D04 (S6 归档)
                                            │
T-SC-O02 (orchestrator 接入) ───────────────┘
```

## 3. 验收任务（AC → T 映射）

| AC | 覆盖测试 |
|----|---------|
| AC1 | T-SC-A01（接口定义）+ T-SC-T01 接口存在性断言 |
| AC2 | T-SC-A02/A03（结构 + 构造函数） |
| AC3 | T-SC-T01 `TestShadowClassifier_TailOnly_NotCalledOnFast` |
| AC4 | T-SC-T01 `TestShadowClassifier_TailOnly_AsyncOnOrchestrate` |
| AC5 | T-SC-T01 `TestShadowClassifier_LLMTimeout_Error` |
| AC6 | T-SC-T01 `TestShadowClassifier_LLM_Match` |
| AC7 | T-SC-T01 `TestShadowClassifier_LLM_Mismatch` |
| AC8 | T-SC-T01 `TestShadowClassifier_NilLLM_NoOp` |
| AC9 | T-SC-T01 全部 9 测试 PASS |
| AC10 | T-SC-C01/C02（config 字段） |

## 4. 风险任务

| 风险 | 任务 | 说明 |
|------|------|------|
| shadow goroutine 测试 flaky | T-SC-T01 | 使用 `sync.WaitGroup` 等待；或 `time.Sleep` + 重试 |
| 现有 orchestrator 测试破坏 | T-SC-T01 + T-SC-O02 | 默认 `WithShadowClassifier` 不传入；行为等价 |
| go vet 警告 | T-SC-T03 | 跑完后立即 fix |

## 5. 完成判定

- [x] 所有 16 个任务完成
- [x] `go test -race -count=1` PASS
- [x] `gofmt -l` 无输出
- [x] `go vet` 无输出
- [x] d7 包覆盖率 ≥ 80%
- [x] D7-S5-T07 PLANNED 登记
- [x] S4-Gate review-code.md APPROVED
- [x] S5 acceptance-report.md PASS
- [x] S6 archive 完成
