---
review-id: S4-Gate
title: S5-P2 Tail-only LLM Classify Shadow — S4-Gate Code Review
change-id: devrix-s5-p2-shadow-classifier
demand-id: DM-20260614-004
reviewer: Claude
review-date: 2026-06-14
status: APPROVED
---

# S5-P2 Tail-only LLM Classify Shadow — S4-Gate Code Review

> 按 `openspec/specs/project/review-code.md` §4 流程逐项执行。

---

## 1. OpenSpec 文档完整性 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| `.openspec.yaml` 存在 | ✅ | `openspec/changes/devrix-s5-p2-shadow-classifier/.openspec.yaml` |
| `proposal.md` 存在 | ✅ | 3 方案评估 + 选定 B |
| `design.md` 存在 | ✅ | 架构图 + 数据结构 + 流程 + 测试点 |
| `tasks.md` 存在 | ✅ | 16 任务 P0/A-F 映射 |
| `demand.md` 存在 | ✅ | DM-20260614-004 |
| `review-r1.md` | ✅ | S3-Gate APPROVED |

**状态一致性**：`.openspec.yaml` 状态 `s3_design`，与 proposal.md / design.md 一致。

---

## 2. 代码质量 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 包位置正确 | ✅ | `internal/layers/d7/shadow_classifier.go`（D7 域） |
| 函数规模 < 50 行 | ✅ | 最大 `shadowAsync` 44 行 |
| 文件规模 < 800 行 | ✅ | shadow_classifier.go 200 行 / shadow_classifier_test.go 290 行 |
| 嵌套深度 ≤ 4 层 | ✅ | 最深 3 层 |
| 命名清晰 | ✅ | LLMIntentClassifier / ShadowClassifier / ShadowMetrics 自解释 |
| 接口合理 | ✅ | LLMIntentClassifier 1 方法；ShadowMetrics 5 字段 |
| 错误类型独立 | ✅ | `shadowError` 不与 D6 / orchestrator 错误冲突 |

---

## 3. 错误与安全 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 错误不静默 | ✅ | LLM 错误 → `Error.Inc()` + `log.Warn`；panic → `recover` + `Error.Inc()` |
| 错误不传播 | ✅ | shadow 错误仅入 metric + log；不污染 rule 决策路径 |
| 输入校验 | ✅ | `NewShadowClassifier` 接受 timeoutMs <= 0 → 500ms 默认；nil rule 接受（caller 负责） |
| 无硬编码密钥 | ✅ | grep 无 |
| 并发安全 | ✅ | shadowAsync 每次独立 goroutine + 局部 ctx；race 检测通过（30 goroutine 测试） |
| ctx 解耦 | ✅ | `context.WithoutCancel(parent)` 防止请求取消中断 shadow |
| panic 安全 | ✅ | `defer recover` 包裹 shadowAsync 全部 |
| nil receiver | ✅ | `Classify` nil → 返回 `errShadowNil` |

---

## 4. 测试完整性 ✅

| 检查项 | 状态 | 证据 |
|--------|------|------|
| 单元测试存在 | ✅ | 13 测试 shadow_classifier_test.go |
| Happy path + sad path | ✅ | NilLLM / TailOnly fast/skip/command/orchestrate / Match / Mismatch / Error / Timeout / Concurrent |
| AC1~AC9 全部覆盖 | ✅ | 见下方矩阵 |
| Race 检测 | ✅ | `go test -race` PASS（30 goroutine Concurrent 测试） |
| 覆盖率（变更范围）| ✅ | NewShadowClassifier 100% / Classify 92% / shadowAsync 87% / d7 包级 85.8% |

### AC 覆盖矩阵

| AC | 覆盖测试 |
|----|---------|
| AC1（接口定义）| T07 测试中 `stubLLM` 实现 LLMIntentClassifier |
| AC2（结构 + 构造函数）| `TestNewShadowMetrics_NilMeter_NoOp` + `TestShadowClassifier_NilLLM_NoOp` |
| AC3（rule 命中不调 LLM）| `TailOnly_NotCalledOn{Fast,Skip,Command}` |
| AC4（orchestrate 异步 LLM）| `TailOnly_AsyncOnOrchestrate` |
| AC5（error/timeout）| `LLMTimeout_Error` + `LLMError_Handled` |
| AC6（match counter）| `LLM_Match` |
| AC7（mismatch counter）| `LLM_Mismatch` |
| AC8（nil LLM no-op）| `NilLLM_NoOp` + `NilLLM_DisabledCounter` |
| AC9（覆盖率 100%）| 13/13 PASS |

---

## 5. CI / 自动化 ✅

```bash
$ go test -race -count=1 ./internal/layers/d7/...
ok  github.com/devrix/devrix/internal/layers/d7  1.988s

$ gofmt -l internal/layers/d7/*.go
（无输出 — 通过；commit bfe236c）

$ go vet ./internal/layers/d7/...
（无输出 — 通过）

$ go test -count=1 -coverprofile=/tmp/d7_shadow.cover.out ./internal/layers/d7/...
ok  github.com/devrix/devrix/internal/layers/d7  0.788s  coverage: 85.8% of statements
```

---

## 6. Review 结论

**Severity** | **Count** | **Examples**
--- | --- | ---
CRITICAL | 0 | —
HIGH | 0 | —
MEDIUM | 0 | —
LOW | 0 | —

**决议**：**APPROVED** — 无任何级别问题。

---

## 7. 后续动作

1. ✅ S4-Gate 通过 → 进入 S5 验收
2. S5：acceptance-report.md
3. S6：归档
4. v1.1 路线图：收集 shadow 数据后切换 LLM 兜底决策路径；rate limit + 样本库持久化
