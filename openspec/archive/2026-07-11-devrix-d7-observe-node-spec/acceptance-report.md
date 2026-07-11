---
demand-id: DM-20260711-001
change-id: devrix-d7-observe-node-spec
title: D7 Observe 节点全协议修订 — 验收报告
executor: Agent S5
environment: local dev (go test -race)
date: 2026-07-11
verdict: ACCEPTED
---

# 验收报告：D7 Observe 节点全协议修订与实现债闭环

## 1. 执行摘要

| 项目 | 值 |
|------|---|
| 需求 ID | DM-20260711-001 |
| Change ID | devrix-d7-observe-node-spec |
| Activity ID | D7-S5-A122 |
| 总体结论 | **ACCEPTED**（P0 全绿；P1 AC5 PASS；P1 AC P5 记例外） |

Wave 1–2（文档 + fast-path）与 Wave 3（P2 promote + P3 scope + P4 signal v1）已落地；P5 pt-tag 派生延后另开 change。

### 测试命令与结果

| Check | Command | Result |
|-------|---------|--------|
| orchestration 全量 | `go test -race ./internal/layers/orchestration/...` | **PASS** (26/26 packages) |
| Observe trace | `go test -race -run TestObserveTraceE2E ./internal/layers/orchestration/sessionorchestrator/...` | **PASS** |
| 静态检查 | `go vet ./internal/layers/orchestration/...` | **PASS** |

## 2. AC 验收矩阵

| AC | 标准 | 结果 |
|----|------|------|
| AC1 | `observe-node-spec.md` §1–§12 + OBS-E/U/O | PASS |
| AC2 | 旧 spec §5 superseded 交叉引用 | PASS |
| AC3 | fast-path source 过滤（LLM 优先于 echo） | PASS |
| AC4 | trace e2e + prior≥0.85 回归 | PASS |
| AC5 | P2 CatSystem promote 无测试 hack | PASS（`promoteSystemCategory` 生产路径） |
| AC6 | 域文档同步 spec/CHANGELOG/t-registry | PASS |

## 3. P1 例外

| 项 | 说明 |
|----|------|
| T12 P5 | `observeLLMFieldMap` → pt-tag 派生未实现；手写 map 仍为 SoT，无行为回归风险 |

## 4. 领域文档同步清单

| 文档 | 状态 |
|------|------|
| `openspec/specs/d7-orchestration/observe-node-spec.md` | ✅ §4 5 标签 + §8 P2–P4 已实现 |
| `openspec/specs/d7-orchestration/spec.md` | ✅ §12 引用 |
| `openspec/specs/d7-orchestration/t-registry.md` | ✅ D7-S5-A122 |
| `openspec/specs/d7-orchestration/CHANGELOG.md` | ✅ 归档行更新 |
| `openspec/demand-archive-index.md` | ✅ S7 登记 |

## 5. 代码变更摘要

- `deliverable_execute.go` — `pickHighStrengthBusinessFact` 两遍扫描 + echo 排除
- `observe_category_promote.go` — P2 CatSystem promote
- `observe_signal_registry.go` — P4 signal 前缀注册表
- `llm_observation_proposer.go` — P3 省略 `scope_open_question`
- `observation_proposer.go` — promote wiring + signal 格式化
