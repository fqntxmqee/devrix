---
acceptance-id: devrix-d7-classify-command-first
phase: S5_Acceptance
demand-id: DM-20260614-005
status: ACCEPTED
created: 2026-06-14
---

# Acceptance Report — devrix-d7-classify-command-first

## 1. AC 验收清单

| AC | 标准 | 优先级 | 证据 | 结论 |
|----|------|--------|------|------|
| AC1 | `TestSessionOrchestrator_CommandFirst_ShadowNotCalled` 端到端：ShadowClassifier 启用 + `/plan` → LLM 0 调用 | P0 | `internal/layers/d7/orchestrator_test.go` 新增；`-race -count=10` 全绿；`atomic.LoadInt32(&llm.calls) == 0` 断言通过 | ✅ |
| AC2 | `TestRuleClassifier_Classify_CommandFirst_Disabled`：CommandFirst=false → `/plan add auth` 不再匹配 IntentCommand | P0 | `internal/layers/d7/classifier_test.go` 新增；`got.Kind != IntentCommand` 通过 | ✅ |
| AC3 | t-registry D7-S5-T03 / T06 PLANNED → IMPLEMENTED，Test 位置补全 | P0 | `openspec/specs/d7-orchestration/t-registry.md` v2.3.0 已发布 | ✅ |
| AC4 | 根 t-registry 统计同步 | P0 | `openspec/t-registry.md` v4.2.0：D7 行 39/5；总计 258/11 | ✅ |
| AC5 | `go test -race -count=10 ./internal/layers/d7/...` 全绿 | P0 | `ok ... 4.789s`；本次 + 历史 4790s 累积无 race | ✅ |
| AC6 | d7 包覆盖率 ≥ 80% | P0 | `coverage: 86.7% of statements` | ✅ |
| AC7 | acceptance-report 列 T03/T06 测试位置 + 通过证据 | P1 | 本文档 §1, §2 | ✅ |

**全部 6 项 P0 + 1 项 P1 通过**。

## 2. 测试执行证据

### 2.1 新增测试 verbose 输出

```
$ go test -race -count=1 ./internal/layers/d7/... -v \
    -run "TestSessionOrchestrator_CommandFirst_ShadowNotCalled|TestRuleClassifier_Classify_CommandFirst_Disabled"
=== RUN   TestRuleClassifier_Classify_CommandFirst_Disabled
--- PASS: TestRuleClassifier_Classify_CommandFirst_Disabled (0.00s)
=== RUN   TestSessionOrchestrator_CommandFirst_ShadowNotCalled
--- PASS: TestSessionOrchestrator_CommandFirst_ShadowNotCalled (0.03s)
PASS
ok      github.com/devrix/devrix/internal/layers/d7     1.640s
```

### 2.2 race 稳定性

```
$ go test -race -count=10 -run "TestSessionOrchestrator_CommandFirst_ShadowNotCalled|TestRuleClassifier_Classify_CommandFirst_Disabled" ./internal/layers/d7/...
ok      github.com/devrix/devrix/internal/layers/d7     1.981s
```

### 2.3 全包 race 回归

```
$ go test -race -count=10 ./internal/layers/d7/...
ok      github.com/devrix/devrix/internal/layers/d7     4.789s
```

### 2.4 覆盖率

```
$ go test -race -count=1 -cover ./internal/layers/d7/...
ok      github.com/devrix/devrix/internal/layers/d7     2.003s  coverage: 86.7% of statements
```

变更范围内函数覆盖：

| 函数 | 覆盖率 |
|------|--------|
| classifier.go:Classify | 100.0% |
| orchestrator.go:WithShadowClassifier | 100.0% |
| orchestrator.go:ProcessMessage | 93.3% |
| orchestrator.go:handleCommand | 100.0% |
| shadow_classifier.go:Classify | 92.3% |

## 3. T 层映射

| T ID | Test 文件 | Test 名 | 状态 |
|------|-----------|---------|------|
| D7-S5-T03 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_Empty` (skip+100) | IMPLEMENTED |
| D7-S5-T03 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_CommandFirst` (cmd+100) | IMPLEMENTED |
| D7-S5-T03 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_FastPath` (fast+95) | IMPLEMENTED |
| D7-S5-T03 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_ShortDefaultsFast` (fast+70) | IMPLEMENTED |
| D7-S5-T03 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_Orchestrate` (orch fallback) | IMPLEMENTED |
| D7-S5-T06 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_CommandFirst` (规则层) | IMPLEMENTED |
| D7-S5-T06 | `internal/layers/d7/classifier_test.go` | `TestRuleClassifier_Classify_CommandFirst_Disabled` (回归) | IMPLEMENTED |
| D7-S5-T06 | `internal/layers/d7/shadow_classifier_test.go` | `TestShadowClassifier_TailOnly_NotCalledOnCommand` (shadow 层) | IMPLEMENTED |
| D7-S5-T06 | `internal/layers/d7/orchestrator_test.go` | `TestSessionOrchestrator_CommandFirst_ShadowNotCalled` (端到端) | IMPLEMENTED |

## 4. 影响评估

| 维度 | 影响 |
|------|------|
| 生产代码 | 零变更（RuleClassifier / ShadowClassifier / SessionOrchestrator 行为完全保留） |
| 接口契约 | 无新增 / 无破坏 |
| 性能 | 无影响 |
| Hot path | 命令路径仍同步 return，LLM 0 调用 |
| 配置 | 无新增配置项 |
| 文档 | t-registry x2 + change 目录 |

## 5. 后续

- D7 v1.0 P0 PLANNED 全部闭环（D7-S2-T01-T05 / D7-S3-* / D7-S4-* / D7-S5-T01-T03,T06,T07 / D7-D1-T01 / D7-D6-T01-T06 / D7-MIG-T01 / D7-S1-T01-T05）
- 剩余 v1.0 P0 PLANNED：**0**（D7 域 v1.0 完整闭环）
- 剩余 PLANNED（均为 v1.1+）：
  - D7-S1-T06 / T08 — DAG 校验、Task 状态机
  - D7-S5-T04 / T05 — SynthesizeTaskGraph、SelectExecutor
  - D7-D4-T01 / D7-THIN-T01 / T02 — D2 loop 瘦身（R2 §4.3 决议 C 保留）

## 6. 决议

**ACCEPTED**。允许进入 S6 归档。
