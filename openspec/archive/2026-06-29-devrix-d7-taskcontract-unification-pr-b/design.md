# Design: D7 TaskContract 统一 PR-B — Pessimistic Commit + Rule-based Fallback (L3 防御运行时层)

**Change ID:** devrix-d7-taskcontract-unification-pr-b
**Demand ID:** DM-20260629-008
**Phase:** S3 Design
**Status:** S3_Design
**Created:** 2026-06-29
**Parent Design:** `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` (DM-20260629-006, 648 行, DESIGN ONLY)
**Sister Change:** devrix-d7-taskcontract-unification-pr-a (DM-20260629-007, S7_Archived 2026-06-29)

> **本设计文档遵循 devrix-architecture-design-six-segment-migration (DM-20260629-007) 规定的六段式 (①-⑥) 模板。** 父设计文档 `archive/2026-06-29-devrix-d7-taskcontract-unification/design.md` (648 行) 含 6 主段 + 5 附录，本 PR-B 文档仅就 PR-B 增量部分做差异化设计（避免 60% 父设计内容复制），父设计引用处使用 `→ 父设计 §X` 标识。

---

## ① 设计目标与约束

### 1.1 业务目标

D7 v7.0 演进的 **L3 防御运行时层**第一阶段实施：补足"资源耗尽无降级输出"+"VERDICT 多轮 INDETERMINATE 无强制规则"+"5 层 CB L1 触发后无显式退出路径" 三大痛点。

### 1.2 设计约束

| 约束 | 来源 | 验证 |
|------|------|------|
| Pure types 原则 | PR-A IV-1 | `grep -r 'orchestration/' interfaces/` (excluding _test) → 0 行 |
| Feature Flag 灰度默认 disabled | 父设计 §3.4 + DM-20260629-006 §5.2 Layer 1 | `D7_PESSIMISTIC_COMMIT_ENABLED` env check |
| 0 行为变更 | PR-A 沿用 | 默认禁用下，老路径完全不变 |
| 4 AC + 7 T 矩阵 | 父设计 §3.4 + §5.3 | 全 S7 落地 |
| 5 层 CB 行为不变 | 父设计 §5.1 | 复用 PR-A TaskReport.Blockage 字段 |
| interfaces 包布局 | PR-A IV-2 | Layout guard (scripts/check-interfaces-imports.sh) 守护 |

### 1.3 关键决策

1. **PessimisticCommitGuard 在 interfaces/contracts.go**：与 PR-A NewTaskSpec/NewTaskReport 同一层（pure types interface），实现放 escape/fallback.go
2. **FallbackPolicy 3 态**：`FallbackPessimistic` (default) / `FallbackRuleBased` / `FallbackAbort` (向后兼容)
3. **ConvergenceBudget 值对象**：tokens 预算 + min_reserve + FallbackPolicy 字段
4. **MVPArtifact 内嵌 TaskReport**：避免新增独立类型，复用 PR-A 的 Report.* 字段（WithMVPArtifact 新方法）
5. **Span + Metric + ErrorCode 三件套**：可观测性完整链路

---

## ② 领域模型

### 2.1 新增 / 修改的聚合

→ 父设计 §④ (1) 详细列出 4 聚合根：TaskSpec / TaskReport / WorkItem.VersionChain / ConvergenceBudget。PR-B 落地 **ConvergenceBudget** + 新增 **MVPArtifact**（TaskReport 子结构）+ **FallbackPolicy**（值对象）。

#### ConvergenceBudget (interfaces/convergence_budget.go, NEW)

```go
type ConvergenceBudget struct {
    tokens_total     int       // 总会话 token 上限
    tokens_used      int       // 已用
    tokens_remaining int       // 剩余
    min_reserve      int       // 最小保留 (触发 Pessimistic 阈值)
    fallback_policy  FallbackPolicy  // 决策策略
    time_budget_ms   int64     // 时间预算
    step_budget      int       // ReAct iter 预算
}

func NewConvergenceBudget(tokens, reserve int, policy FallbackPolicy) ConvergenceBudget
func (b ConvergenceBudget) RemainingBelowReserve() bool  // 触发 Pessimistic
func (b ConvergenceBudget) ToFields() (TokenUsed, Remaining, Reserve int)
```

#### FallbackPolicy (interfaces/fallback_policy.go, NEW)

```go
type FallbackPolicy int

const (
    FallbackPessimistic FallbackPolicy = iota + 1  // default
    FallbackRuleBased                              // 4 候选规则
    FallbackAbort                                  // v6.0.x 向后兼容
)

func (p FallbackPolicy) String() string  // "pessimistic" | "rule_based" | "abort"
func (p FallbackPolicy) Valid() bool
```

#### MVPArtifact (interfaces/task_report.go extension, MODIFIED)

PR-A 已在 task_report.go 暴露 TaskReport struct，PR-B 新增：

```go
type MVPArtifact struct {
    Output        string   // 最小可行产物
    RiskWarnings  []string // 风险警告 (与 Dissent 字段区分：Dissent 是反驳，RiskWarnings 是已知风险)
    Trigger       string   // 触发原因 (resource_exhausted | cb_l1 | indeterminate_3x | empty_evidence | manual_abort)
    Traceback     string   // 触发时的简化 stack
    GeneratedAtMs int64
}

func (r TaskReport) WithMVPArtifact(mvp MVPArtifact) TaskReport  // immutable builder
```

#### PessimisticCommitGuard (interfaces/contracts.go, NEW)

```go
type PessimisticCommitGuard interface {
    // Evaluate returns (ok=true) if commit can proceed, (ok=false) with blocked reason.
    // Triggers: 5 类见父设计 §3.4
    Evaluate(spec *TaskSpec, report *TaskReport, budget ConvergenceBudget) (ok bool, blockedReason string, err error)

    // ResolveFallback returns the chosen FallbackPolicy + (if rule_based) selected rule name
    ResolveFallback(report *TaskReport) (policy FallbackPolicy, ruleName string)

    // BuildMVPArtifact synthesizes MVPArtifact from a blocked report
    BuildMVPArtifact(report *TaskReport, reason string) MVPArtifact
}
```

### 2.2 不变量

- **IV-1**: interfaces/ 包 0 import D7 任何子包（PR-A 立，PR-B 守）
- **IV-2**: FallbackPolicy immutable enum (无 setter)
- **IV-3**: ConvergenceBudget immutable value object (With* shallow copy)
- **IV-4**: MVPArtifact immutable (构造函数)
- **IV-5**: PessimisticCommitGuard 4 错误码仅在 escape + interfaces 使用，其他域不感知
- **IV-6** (新增): Feature Flag 默认 disabled，PR-B 合并 0 行为变更

---

## ③ 业务流程

### 3.1 完整流程：5 类触发条件 → 决策树

→ 父设计 §3.4 完整决策树（200+ 行）原样沿用。PR-B 实施点：

```
Channel.Execute 结束
    ↓
[NEW PR-B 注入点] PessimisticCommitGuard.Evaluate(spec, report, budget)
    ├─ ok=true: 正常返回 TaskReport (Result.Kind = Pass / Partial)
    └─ ok=false: 走 ResolveFallback(report)
         ↓
         ├─ FallbackPessimistic (default)
         │   ↓
         │   BuildMVPArtifact(report, blockedReason)
         │   ↓
         │   TaskReport.WithMVPArtifact(mvp) + FallbackUsed=true
         │   ↓
         │   Result.Kind = Indeterminate (强制)
         │   ↓
         │   [NEW Span] d7.s18.pessimistic.commit.emit (1 个)
         │   [NEW Metric] pessimistic_commit_trigger_count++
         │
         ├─ FallbackRuleBased
         │   ↓
         │   VERDICT 连续 ≥ 3 轮 INDETERMINATE 才触发
         │   ↓
         │   4 候选规则 (most_tests_passed | compiled_clean | min_cost | min_uncertainty)
         │   ↓
         │   env D7_RULE_FALLBACK_STRATEGY 切换 (default min_uncertainty)
         │   ↓
         │   选中规则 → Kind = Pass (强制覆盖 INDETERMINATE)
         │   ↓
         │   [NEW Metric] fallback_rule_select_total{rule=<name>}++
         │
         └─ FallbackAbort (v6.0.x 向后兼容)
               ↓
               直接 abort, Result.Kind = Failed
               ↓
               [NEW ErrorCode 7113] ORCH_FALLBACK_ABORT_TIMEOUT
```

### 3.2 5 类触发条件（PR-B 显式实施）

→ 父设计 §3.3 已列出 5 类。PR-B 实施点：

| 触发 | 检测位置 | 路径 |
|------|----------|------|
| 资源耗尽 | `mups/execute/channel.go::Execute` 出口 | `tokens_remaining <= min_reserve` → Pessimistic |
| EscapeForceExit (CB L1) | `escape/circuit_breaker.go::Apply` | CB L1 触发 → `engine.NotifyPessimistic()` → Pessimistic |
| VERDICT ≥ 3 轮 INDETERMINATE | `executionflow/verify/verdict.go::AggregateVerdicts` | 计数 ≥ 3 → Rule-based |
| Verifier "空证 PASS" | `executionflow/verify/verifier.go::extractVerdict` | 缺 test/log/artifact_hash → Partial + Blockage.RequiredExternal |
| 人工 abort (IM 通道关闭) | `sessionorchestrator/orchestrator.go::handleInterrupt` | Abort → FallbackAbort |

### 3.3 5 层 CB 升级路径（PR-B 修改）

→ 父设计 §5.1 完整 5 层 CB 状态机。PR-B 改动点：

| CB Level | 父设计行为 | PR-B 行为 |
|----------|-----------|-----------|
| L0 Normal | 正常 | 同 (不变) |
| L1 Single Failure | 触发 EscapeEngine | **+ NotifyPessimistic** (PR-B 增量) |
| L2 Repeated Failure | 切换 worker 池 | 同 (不变) |
| L3 Degraded Mode | 降级 worker 池 | 同 (不变) |
| L4 HardStop | 阻断新任务 | 同 (不变) |
| L5 BlackHole | 黑洞模式 | 同 (不变) |

PR-B 仅在 L1 触发现有 EscapeEngine.NotifyPessimistic()，L2-L5 行为完全不变。NotifyPessimistic 接收 TaskReport 引用（PR-A 加 Blockage 字段后即可读），不修改 CB 状态机本身。

---

## ④ 接口契约

### 4.1 PessimisticCommitGuard interface

→ 父设计 §④ (3) 已定义接口签名。PR-B 落地：

```go
// internal/layers/orchestration/interfaces/contracts.go
package interfaces

import "errors"

var (
    ErrORCHPessimisticTriggered   = sharederrors.WithCode(7110, "pessimistic commit triggered")
    ErrORCHPessimisticMVPEmpty    = sharederrors.WithCode(7111, "mvp artifact output is empty")
    ErrORCHFallbackRuleInvalid    = sharederrors.WithCode(7112, "fallback rule not recognized")
    ErrORCHFallbackAbortTimeout   = sharederrors.WithCode(7113, "fallback abort timeout")
)

type PessimisticCommitGuard interface {
    Evaluate(spec *TaskSpec, report *TaskReport, budget ConvergenceBudget) (bool, string, error)
    ResolveFallback(report *TaskReport) (FallbackPolicy, string)
    BuildMVPArtifact(report *TaskReport, reason string) MVPArtifact
}
```

### 4.2 ConvergenceBudget + FallbackPolicy 公共 API

```go
// internal/layers/orchestration/interfaces/fallback_policy.go
type FallbackPolicy int
const (
    FallbackPessimistic FallbackPolicy = iota + 1
    FallbackRuleBased
    FallbackAbort
)

// internal/layers/orchestration/interfaces/convergence_budget.go
type ConvergenceBudget struct { /* 见 ② §2.1 */ }
func NewConvergenceBudget(tokens, reserve int, policy FallbackPolicy) ConvergenceBudget
```

### 4.3 TaskReport 扩展 (additive)

PR-A 已有 TaskReport + WithBlockage / WithResource / WithDissent 等，PR-B 新增：

```go
// internal/layers/orchestration/interfaces/task_report.go
func (r TaskReport) WithMVPArtifact(mvp MVPArtifact) TaskReport
```

additive — 现有 With* 调用方 0 改动。

---

## ⑤ 实现路径

### 5.1 文件清单 (7 NEW + 4 MODIFIED)

→ 父设计 §6 文件清单 + proposal §5。本 PR-B 落地：

**NEW (7)**:
1. `internal/layers/orchestration/interfaces/contracts.go` (PessimisticCommitGuard interface, 80 LOC)
2. `internal/layers/orchestration/interfaces/fallback_policy.go` (FallbackPolicy enum, 40 LOC)
3. `internal/layers/orchestration/interfaces/convergence_budget.go` (ConvergenceBudget 值对象, 60 LOC)
4. `internal/layers/orchestration/escape/fallback.go` (3 FallbackPolicy 实现, 200 LOC)
5. `internal/layers/orchestration/interfaces/contracts_test.go` (≥80% 覆盖, 150 LOC)
6. `internal/layers/orchestration/escape/fallback_test.go` (200 LOC)
7. `internal/layers/orchestration/escape/circuit_breaker_test.go` (CB L1 Pessimistic 升级测试, 100 LOC)

**MODIFIED (4)**:
1. `internal/layers/orchestration/escape/engine.go` (新增 NotifyPessimistic + 集成 PessimisticCommitGuard)
2. `internal/layers/orchestration/escape/circuit_breaker.go` (L1 触发 NotifyPessimistic)
3. `internal/layers/orchestration/mups/execute/channel.go` (Execute 出口 + PessimisticCommitGuard 决策)
4. `internal/layers/orchestration/d7-bootstrap/wire.go` (Feature Flag 注入 + 默认 disabled)
5. `internal/layers/orchestration/interfaces/task_report.go` (WithMVPArtifact 新增 immutable builder)

**SPEC DOCS (6 同步)**:
- `openspec/specs/d7-orchestration/spec.md` (新增 3 ADDED Requirements for D7-S18)
- `openspec/specs/d7-orchestration/d7-domain.md` (§8 Layer 4-Layer × 3-Phase PR-B 落地)
- `openspec/specs/d7-orchestration/a-registry.md` (D7-S18-A11/A12 2 A entries)
- `openspec/specs/d7-orchestration/f-registry.md` (D7-S22-F01 状态 PLANNED → IMPLEMENTED)
- `openspec/specs/d7-orchestration/span-registry.md` (1 个新 P0 span ops)
- `openspec/specs/d7-orchestration/t-registry.md` (7 个 P0 T 登记)

### 5.2 实施顺序

1. **S4-1**: interfaces/contracts.go + fallback_policy.go + convergence_budget.go (NEW)
2. **S4-2**: interfaces 单元测试 (contracts_test.go) → 覆盖率 ≥ 80%
3. **S4-3**: escape/fallback.go (3 FallbackPolicy 实现)
4. **S4-4**: escape/fallback_test.go (200 LOC) + circuit_breaker_test.go (100 LOC)
5. **S4-5**: escape/engine.go + circuit_breaker.go + channel.go 修改 (additive)
6. **S4-6**: d7-bootstrap/wire.go (Feature Flag 注入)
7. **S4-7**: spec 文档 6 文件同步 + commit
8. **S4-8**: LP-1/LP-2/LP-5 集成测试 + 灰度验证脚本

### 5.3 复用 PR-A 资产

→ 见 proposal §2.3 表格。

---

## ⑥ 验证与风险

### 6.1 验证策略

→ 父设计 §7 验证策略 + proposal §7。PR-B 落地：

| 项 | 目标 | 验证方式 |
|---|------|----------|
| `go test -race -count=1 ./internal/layers/orchestration/...` | 全绿 24+ packages | `make test-unit` |
| `go test -cover ./internal/layers/orchestration/interfaces/` | ≥ 80% | `go test -cover` |
| `go test -cover ./internal/layers/orchestration/escape/` | ≥ 80% | 同上 |
| LP-1/LP-2/LP-5 集成测试 | 100% 兼容 | `go test ./tests/integration/d7/...` |
| `grep -r 'orchestration/' internal/layers/orchestration/interfaces/ \| grep -v _test` | 0 行 | 守护 IV-1 |
| `D7_PESSIMISTIC_COMMIT_ENABLED=false` 默认 | 0 行为变更 | smoke test |

### 6.2 风险与缓解

→ 父设计 §B 风险表 + proposal §8。PR-B 增量风险：

| 风险 | 概率 | 缓解 |
|------|------|------|
| Feature Flag 灰度期间 false negative | 中 | 灰度分桶 1% → 10% → 50% → 100% 渐进 |
| 4 候选规则实现复杂度 | 低 | 先实现 min_uncertainty（默认），其他 3 个 stub + TODO 留给 v7.0.1 |
| CB L1 Pessimistic action 状态机复杂度 | 中 | 复用 PR-A TaskReport.Blockage 字段，L1 仅是 action 之一 |
| 4 个 ORCH_* 错误码与 PR-A 5 个共占 7100-7113 区间 | 0 | 区间划分：7100-7109 PR-A / 7110-7119 PR-B / 7120-7129 PR-C |

### 6.3 Rollback Plan

PR-B 启用 `D7_PESSIMISTIC_COMMIT_ENABLED` 后可立即关停降级到 PR-A 状态：

```bash
# 1. 关停 Pessimistic (灰度回滚)
D7_PESSIMISTIC_COMMIT_ENABLED=false ./scripts/devrix.sh restart

# 2. 紧急回滚 (commit revert)
git revert <pr-b-commit-sha>
git push origin master  # 触发 CI auto-revert
```

### 6.4 回归风险

| 维度 | 风险 | 缓解 |
|------|------|------|
| PR-A 已 S7_Archived 的 11/11 T | 0 回归 | 回归测试套件 (LP-1/LP-2/LP-5) |
| 24+ orchestration packages -race | 0 race | `go test -race -count=1` |
| escape/ 包 (V5 EscapeEngine) | CB L1 升级可能影响 | 单元测试覆盖 (circuit_breaker_test.go 100 LOC) |
| mups/execute/channel.go | Execute 出口决策 | 加固单测 (existing test 0 改动) |

---

## 附录 A: File Manifest

| Path | LOC | 状态 | AC |
|------|-----|------|-----|
| `internal/layers/orchestration/interfaces/contracts.go` | 80 | NEW | AC11 |
| `internal/layers/orchestration/interfaces/fallback_policy.go` | 40 | NEW | AC11, AC12 |
| `internal/layers/orchestration/interfaces/convergence_budget.go` | 60 | NEW | AC11 |
| `internal/layers/orchestration/interfaces/task_report.go` (WithMVPArtifact) | +20 | MOD | AC11 |
| `internal/layers/orchestration/escape/fallback.go` | 200 | NEW | AC11, AC12 |
| `internal/layers/orchestration/escape/engine.go` (NotifyPessimistic) | +30 | MOD | AC11 |
| `internal/layers/orchestration/escape/circuit_breaker.go` (L1 → Pessimistic) | +20 | MOD | AC11 |
| `internal/layers/orchestration/mups/execute/channel.go` (Evaluate) | +15 | MOD | AC11 |
| `internal/layers/orchestration/d7-bootstrap/wire.go` (Feature Flag) | +25 | MOD | AC16 |
| `internal/layers/orchestration/interfaces/contracts_test.go` | 150 | NEW | AC11 |
| `internal/layers/orchestration/escape/fallback_test.go` | 200 | NEW | AC11, AC12 |
| `internal/layers/orchestration/escape/circuit_breaker_test.go` | 100 | NEW | AC11 |
| **Total NEW+MOD** | **940** | | |
| `openspec/specs/d7-orchestration/*.md` (6 files) | +300 | MOD | AC18 |

## 附录 B: Rollback Plan

见 §6.3。

## 附录 C: 回归风险评估

见 §6.4。

## 附录 D: S3 Checklist (六段式自检)

- [x] ① 设计目标与约束 (3 类约束显式列出)
- [x] ② 领域模型 (3 聚合根 + 1 interface)
- [x] ③ 业务流程 (5 类触发 + 5 层 CB 升级)
- [x] ④ 接口契约 (PessimisticCommitGuard interface + 2 值对象 + 1 builder 扩展)
- [x] ⑤ 实现路径 (7 NEW + 4 MODIFIED + 8 步实施顺序)
- [x] ⑥ 验证与风险 (6 验证项 + 4 风险 + Rollback + 回归)

## 附录 E: 下一步 (PR-B → PR-C)

PR-B S7_Archived 后启动 PR-C OpenSpec：
- AC13 CoW VersionChain (WorkItem.VersionChain 不可变追加 + 24h GC)
- AC14 Similarity Check > 80% 拦截
- AC15 Hard Evidence kind-specific 缓解
- 预计 1 周
