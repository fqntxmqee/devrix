---
report-id: SC-S5-AR
title: S5-P2 Tail-only LLM Classify Shadow — S5 验收报告
change-id: devrix-s5-p2-shadow-classifier
demand-id: DM-20260614-004
review-date: 2026-06-14
status: PASS
---

# S5-P2 Tail-only LLM Classify Shadow — S5 验收报告

> 按 `openspec/specs/project/testing.md` §7 S5 验收清单逐项核对。

---

## 1. 测试金字塔

### 1.1 单元测试

```bash
$ go test -race -count=1 ./internal/layers/d7/...
ok  github.com/devrix/devrix/internal/layers/d7  1.988s
```

**Shadow 新增测试**（13 个，全部 PASS）：

| 测试 | 覆盖 | 描述 |
|------|------|------|
| `TestShadowClassifier_NilLLM_NoOp` | AC8 | nil LLM 不调用，结果为 rule |
| `TestShadowClassifier_TailOnly_NotCalledOnFast` | AC3 | "hi" → IntentFast，无 LLM 调用 |
| `TestShadowClassifier_TailOnly_NotCalledOnSkip` | AC3 | "" → IntentSkip，无 LLM 调用 |
| `TestShadowClassifier_TailOnly_NotCalledOnCommand` | AC3 | "/plan x" → IntentCommand，无 LLM 调用 |
| `TestShadowClassifier_TailOnly_AsyncOnOrchestrate` | AC4 | 长 query → IntentOrchestrate，异步触发 LLM |
| `TestShadowClassifier_LLM_Match` | AC6 | LLM match rule → Match.Inc |
| `TestShadowClassifier_LLM_Mismatch` | AC7 | LLM ≠ rule → Mismatch.Inc |
| `TestShadowClassifier_LLMTimeout_Error` | AC5 | 50ms timeout + 200ms LLM → Error.Inc |
| `TestShadowClassifier_LLMError_Handled` | AC5b | LLM 返回 err → Error.Inc + caller 不知情 |
| `TestShadowClassifier_NilReceiver_ReturnsError` | AC9 | nil receiver 不 panic，返回 err |
| `TestShadowClassifier_Concurrent` | — | 30 goroutine race-free |
| `TestNewShadowMetrics_NilMeter_NoOp` | — | nil meter → nil metrics |
| `TestShadowClassifier_NilLLM_DisabledCounter` | — | nil LLM → Disabled.Inc |
| `TestShadowClassifier_Latency_Recorded` | — | latency histogram 有观测值 |

### 1.2 全量回归

```bash
$ go test -race -count=1 ./internal/...
```
**结果**：64 packages OK，0 FAIL。

---

## 2. 覆盖率

```bash
$ go test -count=1 -coverprofile=/tmp/d7_shadow.cover.out ./internal/layers/d7/...
ok  github.com/devrix/devrix/internal/layers/d7  0.788s  coverage: 85.8% of statements
```

**d7 包级覆盖率**：**85.8%** > 80% 阈值 ✅

按变更范围内函数：
- `NewShadowMetrics`：56.5%（错误分支未全测；正常路径覆盖）
- `NewShadowClassifier`：**100%** ✅
- `Classify`：92.3%（rule 错误传播路径未测）
- `shadowAsync`：87.5%（LLM panic 路径难构造，未测）

---

## 3. P0 T 层 100% PASS

| T ID | 描述 | 状态 |
|------|------|------|
| **D7-S5-T07** | Tail-only LLM classify shadow（rule 未命中时异步 LLM，结果只入 metric） | ✅ **PASS**（AC1~AC9 全部覆盖） |

**P0 PASS：1/1 = 100%** ✅

---

## 4. 检查清单

- [x] 所有 P0 T 层测试已编写（D7-S5-T07）
- [x] 测试代码标注了 T 层编号（`T: D7-S5-T07 AC1` 等）
- [x] `go test -race -count=1` 通过
- [x] 并发代码通过 `-race` 检测（30 goroutine Concurrent 测试）
- [x] 测试文件无 `t.Skip`
- [x] d7 包覆盖率 ≥ 80%（85.8%）
- [x] `acceptance-report.md` 已生成（本文件）
- [x] `t-registry.md` D7-S5-T07 状态新增为 IMPLEMENTED
- [x] OpenSpec 文档齐全且状态一致
- [x] 无 CRITICAL/HIGH/MEDIUM/LOW 问题
- [x] `go vet` 和 `gofmt` 通过
- [x] S4-Gate review-code.md APPROVED

---

## 5. 验收决议

| 维度 | 结果 |
|------|------|
| P0 T 层 100% PASS | ✅ 1/1（D7-S5-T07） |
| 覆盖率 ≥ 80% | ✅ 85.8% |
| OpenSpec 文档齐全 | ✅ |
| go vet / gofmt | ✅ |
| go test -race | ✅ |
| S4-Gate 审查 | ✅ APPROVED |
| 整体验收 | ✅ **PASS** |

**决议**：S5 验收 **通过**，可进入 S6 归档。

---

## 6. 后续动作

### v1.0 release 后

- D7 Shadow Classifier 已就位 — `orchestration.intent.classify.shadow.*` 5 个 metric 可观测
- 默认 `ShadowLLMClassify=false`，hot path 零成本
- 启用时仅对 IntentOrchestrate tail（~20%）异步 LLM 调用，500ms 超时

### v1.1+ 路线图

1. 收集 shadow 数据后，将 LLM classify 作为 v1.1 兜底（rule 未命中时切换 LLM 决策）
2. 配置 rate limit（按 session / IP）
3. shadow 样本库持久化（JSONL 入 `var/log/devrix/shadow/`）
4. 离线评估（Devrix Eval Phase 5 — shadow 对比报告）
5. 三模型合并决策清单（命题 B — 替代 Shapley 附录）
6. D2 瘦身（D7-D4-T01 / D7-THIN-T01-T02）
7. SynthesizeTaskGraph（D7-S5-T04）
8. D6 validator 真实实现（当前仍是接口）
9. AlertManager Webhook 集成
