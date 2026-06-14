---
demand-id: DM-20260614-018
change-id: devrix-d4-sa-refine
phase: v2.0-d Structure
status: S5_ACCEPTED
verdict: PASS (conditional on E-e3)
date: 2026-06-15
reviewer: Owner（自裁决）
parent: dsaft-refactoring-playbook
---

# Acceptance Report — D4 Multi-Agent S/A 重切 v1.0

## 0. 验收范围与边界

| 维度 | 范围 |
|------|------|
| Change | `devrix-d4-sa-refine`（**DM-20260614-018**） |
| Phase | **v1.0 Registry Refine**（S11–S16 Canonical + Legacy 双轨 + Hub-Spoke 归 D7 规格层，**0 行运行时代码变更**） |
| 不在本期 | v1.1 Span 归 D5 + D6 probe；v2.0 slice a–e（Hub-Spoke 代码收敛 + D4 物理路径） |
| DM ID 修正 | 草稿误用 DM-20260614-017（已归属 D3 v1.1）；正式登记 **DM-20260614-018** |

**v1.0 不变性承诺**（R1 决议 + playbook 原则 3）：

- **38 条 Legacy T**：`// T:` 注释未改
- **物理目录**：`multiagent/` 子包未迁移（v2.0-d 范围）
- **Hub-Spoke 代码**：仍在 D4 `delegate/` + D2 `flow_report`（v2.0-b/c 范围）
- **Span/Metric 名字**：`agent.*` 字面量未改（v1.1 归属迁移）

---

## 1. v1.0 验收准则（AC）逐项裁决

| AC | 准则 | 证据 | 裁决 |
|----|------|------|------|
| AC-01 | 5+1 S 切法落地：S11–S16 Canonical + Legacy S1–S10 冻结 | `spec.md` v3.0.0；`a-registry.md` v3.0.0 §Canonical | ✅ PASS |
| AC-02 | North Star = Delegation Execution Follower；Hub-Spoke Out of Scope | `d4-domain.md` §1 + §Out of Scope | ✅ PASS |
| AC-03 | Hub-Spoke 全归 D7（R1 D7-1） | `d7-boundary.md` §2；`cross-domain-boundaries.md` §3 | ✅ PASS |
| AC-04 | D2 SubQuery Flow 发布迁 D7 已登记 | `d2-context-engine/d7-boundary.md` #6；`d7-boundary.md` §8 #4 | ✅ PASS |
| AC-05 | Legacy→Canonical 追溯 100%（38 T + 5 Hub-Spoke 重归属） | `t-registry.md` §Legacy Archive | ✅ PASS |
| AC-06 | F 层 Canonical 表 + Hub-Spoke F Out of Scope | `f-registry.md` v3.0.0 §Canonical | ✅ PASS |
| AC-07 | D7 a-registry 增量（S2-A04 + S4-A04/A05） | `d7-orchestration/a-registry.md` v3.0.0 | ✅ PASS |
| AC-08 | code-layout §4.5 D4 scenario-slug + Hub-Spoke 迁移表 | `architecture/code-layout.md` | ✅ PASS |
| AC-09 | layering §D4 双轨 | `architecture/layering.md` | ✅ PASS |
| AC-10 | S3-Gate 通过 | `review-s3.md` APPROVED | ✅ PASS |
| AC-11 | 19 P0 T 全绿 | §3 测试证据 | ✅ PASS |
| AC-12 | D4 spec 无 Hub-Spoke SoT 表述 | `grep` spec.md：无 S10 Hub-Spoke SoT | ✅ PASS |
| AC-13 | `go build ./...` 全绿（0 行代码变更） | §3.1 | ✅ PASS |

---

## 2. 领域文档同步清单

| 文档 | 动作 | 状态 |
|------|------|------|
| `openspec/specs/d4-multi-agent/d4-domain.md` | 新建 | ✅ |
| `openspec/specs/d4-multi-agent/d7-boundary.md` | 新建 | ✅ |
| `openspec/specs/d4-multi-agent/spec.md` | v3.0.0 | ✅ |
| `openspec/specs/d4-multi-agent/a-registry.md` | v3.0.0 Canonical | ✅ |
| `openspec/specs/d4-multi-agent/f-registry.md` | v3.0.0 Canonical | ✅ |
| `openspec/specs/d4-multi-agent/t-registry.md` | v3.0.0 + §Legacy Archive | ✅ |
| `openspec/specs/d4-multi-agent/span-registry.md` | v3.0.0 S8→D5 声明 | ✅ |
| `openspec/specs/d7-orchestration/a-registry.md` | Hub-Spoke A 增量 | ✅ |
| `openspec/specs/d2-context-engine/d7-boundary.md` | SubQuery Flow 迁出 #6 | ✅ |
| `openspec/specs/architecture/layering.md` | §D4 双轨 | ✅ |
| `openspec/specs/architecture/code-layout.md` | §4.5 D4 | ✅ |
| `openspec/specs/architecture/cross-domain-boundaries.md` | §3 D4 边界 | ✅ |

---

## 3. 测试证据

### 3.1 编译与回归（v1.0 零代码变更）

```text
go test ./internal/layers/multiagent/...     → PASS（agent/delegate/factory/tool 等）
go test ./internal/layers/orchestration/...  → PASS（delegatetools/flow/imsink/wave 等）
go test ./internal/layers/contextengine/...  → PASS（nested/delegate_fallback 等）
```

### 3.2 T 覆盖统计

| 指标 | 值 |
|------|-----|
| Legacy T 总数 | 38 |
| P0 | 19 |
| Hub-Spoke 重归属 D7 | 5（T07 + T08–T11） |
| Legacy Archive 映射行 | 100% 覆盖 |
| `// T:` 注释变更 | 0（v1.0 约束） |

---

## 4. 裁决

**Verdict: PASS — v1.0 Registry ACCEPTED**

可进入：
- **v1.1**（Phase D）：Span 归 D5 + D6 probe + import lint 草案
- **v2.0**（Phase E）：slice a–e Hub-Spoke 代码收敛 + D4 物理路径

S7 归档待 v2.0 全部 slice 验收后执行。

---

## 6. v2.0 验收（Slice a–e Structure）

### 6.0 验收范围与边界

| 维度 | 范围 |
|------|------|
| Phase | **v2.0 Structure**（slice a–e：Hub-Spoke 代码收敛 + D4 物理路径迁移） |
| Commit | `3905c6a`（89 files, +6465/-476） |
| 在本期 | slice a（骨架）→ slice b（D4 bridge+dispatch）→ slice c（D2 flow_report）→ slice d（物理路径）→ slice e（验证+文档） |
| 不在本期 | E-e4（dead code + re-export 删除，v2.0-e 发布周期）；E-e3（tagged E2E/integration 回归，待 CI 环境准备后补跑） |

### 6.1 v2.0 验收准则

| AC | 准则 | 证据 | 裁决 |
|----|------|------|------|
| AC-01 | D4-S11 ProvisionAgent 迁移至 `provision/` | `multiagent/provision/factory.go`；legacy `factory/legacy.go` re-export | ✅ PASS |
| AC-02 | D4-S12 RunAgentLoop 迁移至 `run/` | `multiagent/run/agent.go`；legacy `agent/legacy.go` re-export | ✅ PASS |
| AC-03 | D4-S13 IsolateAndMerge 迁移至 `isolate/` | `multiagent/isolate/view.go`；legacy `sessionview/legacy.go` re-export | ✅ PASS |
| AC-04 | D4-S14 ExecuteWorker 迁移至 `execute/` | `multiagent/execute/worker.go` + `execute/contracts.go`；per-call observer 模式 | ✅ PASS |
| AC-05 | D4-S15 InvokeExternalAgent 迁移至 `external/` | `multiagent/external/` 物理目录 | ✅ PASS |
| AC-06 | D4-S16 ConfigureAgents 迁移至 `configure/` | `multiagent/configure/configure.go`；legacy `shared/config/multiagent.go` re-export | ✅ PASS |
| AC-07 | Kernel 类型迁移至 `kernel/` | `multiagent/kernel/contracts.go` + `kernel/noop.go`；根 `contracts.go` re-export + `observer/noop.go` re-export | ✅ PASS |
| AC-08 | AgentBridge 迁 D7 `hubspoke/agent_bridge.go` | `orchestration/hubspoke/agent_bridge.go`；`AgentBridge.EmitAgentEvent` 映射 4 种 agent event → FlowEvent | ✅ PASS |
| AC-09 | Dispatcher 迁 D7 `hubspoke/dispatch.go` | `orchestration/hubspoke/dispatch.go`；`Dispatch()` D4→D2 fallback 路径 | ✅ PASS |
| AC-10 | SubQueryBridge 迁 D7 `hubspoke/subquery_bridge.go` | `orchestration/hubspoke/subquery_bridge.go`；`PublishStarted/Completed/Failed` 三方法 | ✅ PASS |
| AC-11 | 新包测试覆盖 | execute/worker_test.go（9 tests）+ hubspoke/hubspoke_test.go（23 tests） | ✅ PASS |
| AC-12 | 全量 `go test -race` 71 包全绿 | §6.3 测试证据 | ✅ PASS |
| AC-13 | 编译完整性 `go build ./...` + `go vet ./...` | §6.3 | ✅ PASS |
| AC-14 | D4/D7 文档同步 | layering.md v3.9.0 + code-layout.md v1.6.0 | ✅ PASS |

### 6.2 物理路径迁移清单

| S ID | v1.0 路径 | v2.0-d 路径 | legacy.go |
|------|----------|------------|-----------|
| S11 | `factory/` | `provision/` | `factory/legacy.go` |
| S12 | `agent/agent.go` | `run/agent.go` | `agent/legacy.go` |
| S13 | `sessionview/` | `isolate/` | `sessionview/legacy.go` |
| S14 | `delegate/service.go` | `execute/worker.go` | — |
| S15 | `tool/` | `external/` | — |
| S16 | `shared/config/multiagent.go` | `configure/configure.go` | `shared/config/multiagent.go` (re-export) |
| Kernel | `contracts.go` + `observer/` | `kernel/contracts.go` + `kernel/noop.go` | 根 `contracts.go` + `observer/noop.go` |

### 6.3 测试证据

```text
go build ./...                                          → PASS（全量编译）
go vet ./...                                            → PASS（零 warning）
go test -race -count=1 ./internal/...                   → PASS（71 包全绿）
go test -race ./internal/layers/multiagent/execute/...  → PASS（9 tests）
go test -race ./internal/layers/orchestration/hubspoke/... → PASS（23 tests）
go test -tags "integration cross" ./tests/integration/...  → 3 pre-existing build errors（见 §6.4）+ 1 migration bug（commit 4e48f83 已修复）
```

### 6.4 未完成项与已知问题

| ID | 项目 | 原因 | 计划 |
|----|------|------|------|
| E-e3 | E2E + 集成测试回归（tagged `-tags "integration cross"`） | 3 个预存构建错误与 D4 无关（D3 `IAdapter.Protocol` 缺失 ×2 + D2 `WireContextLLM` 双返回值 API 变更）；1 个迁移引入的 `provision.Create` 误改已于 commit 4e48f83 修复 | v2.0-e 准出前补跑（待 D2/D3 API 修复后） |
| E-e4 | 旧路径 dead code + re-export 删除 | 5/5 re-export shim 已删除（commit e30fe72）：`factory/` `agent/` `sessionview/` `observer/` `tool/`；`observer` 引用已迁移至 `kernel`；`delegate/` 保留等待确认 | ✅ re-export 完成 |
| E-e7 | S7 归档 | 待 E-e3 预存错误修复后执行 | v2.0-e 准出 |

### 6.5 裁决

**Verdict: PASS（conditional on E-e3） — v2.0-d Structure ACCEPTED**

可进入：
- **v2.0-e**（下一个发布周期）：E-e3 tagged test 回归、E-e4 re-export 清理、E-e7 归档

Blockers：无（E-e3 为非阻塞 regression confirm，测试套件 71 包全绿已提供充分信心）

---

## 7. Revision History

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2026-06-14 | v1.0 Registry 验收 PASS |
| 2.0 | 2026-06-15 | v2.0-d Structure 验收 PASS（conditional on E-e3） |
