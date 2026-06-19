# D6 Evolution Domain

**Domain ID:** D6
**Slug:** `evolution`
**Type:** Supporting Domain
**Status:** Active — Canonical S3–S5（v2.2.0 路径同步）
**Version:** 1.0.0
**Last Updated:** 2026-06-19
**Depends On:** D2（trace/probe 目标）、D3（LLM Judge 通道）、D4（agent 事件源）、D5（metrics/session isolation 交叉）、D7（编排事件 advisory）
**Depended By:** 横向支持域——不直接被业务路径消费；为内部 SRE/平台团队 + CI Delta Gate 提供评测与守卫能力
**Cross-Domain SoT:** `../architecture/cross-domain-boundaries.md` §2.6 · `../d7-orchestration/d7-boundary.md`（advisory 通道）

---

## North Star

**作为 Self-Eval + Guard + Verify 三大支撑能力，对系统自身进行评测、对运行时决策进行守卫、对不变量进行验证——不参与业务编排、不阻塞关键路径。**

| 可验证承诺 | Canonical S |
|-----------|-------------|
| 评测管道可重放 + Delta 检测 | D6-S3 Evaluate（v2.0 物理路径：evaluate/） |
| 决策校验 + 干预执行 | D6-S4 GuardRuntime（v2.0 物理路径：guard/） |
| Invariant 验证可独立运行 | D6-S5 VerifyInvariant（v2.0 新增物理独立：verify/） |

---

## Out of Scope

| 能力 | 归属 | 备注 |
|------|------|------|
| Session 上下文 / 工具执行 | D2 | D6 仅读 D2 trace 作 probe 输入 |
| LLM 网关实现 | D3 | D6 经 D3 调用 Judge（GatewayLLMClient） |
| Agent 生命周期 | D4 | D6 通过 GuardObserver 桥接 agent 事件 |
| 编排决策 | D7 | D6 为 advisory，**不阻塞** D7 Turn 主循环 |
| 可观测性基础设施 | D5 | D6 写指标到 D5（OpenTelemetry） |
| IM ingress / 用户交互 | D1 | D6 输出仅至 CI / SRE / 内部 dashboard |

---

## DSAFT 资产

### Canonical 价值流 — D6-S3–S5

| S ID | Scenario | 博弈角色 | Status |
|------|----------|----------|--------|
| D6-S3 | Evaluate | Honest Reporter | ACTIVE（v2.0 路径 evaluate/） |
| D6-S4 | GuardRuntime | Mechanism Designer | ACTIVE（v2.0 路径 guard/；曾因误删从 42bf1d7 恢复） |
| D6-S5 | VerifyInvariant | Invariant Enforcer | ACTIVE（v2.0 新增物理独立 verify/） |

### Legacy Module Index（冻结追溯）— D6-S1–S2

| Module ID | Scenario | Status | Canonical 映射 |
|-----------|----------|--------|----------------|
| D6-S1 | Skeleton | RETIRED | V1 占位 |
| D6-S2 | Self-Eval Pilot | RETIRED | → S3 |

### 物理路径映射表（Canonical S → 代码目录）

| Canonical S | Scenario | 物理路径 | 备注 |
|-------------|----------|----------|------|
| D6-S3 | Evaluate | `internal/layers/evolution/evaluate/` | v2.0 由 `eval/` 改名；probe 注册表 7 类内置探针 |
| D6-S4 | GuardRuntime | `internal/layers/evolution/guard/` | v2.0 由 `orchestration/` 改名；RuntimeGuardValidator + GuardObserver |
| D6-S5 | VerifyInvariant | `internal/layers/evolution/verify/` | v2.0 新增物理独立；`_invariant.go` + `plan.go` |

### 跨域契约

| 跨域方向 | 契约 | D6 实现 | 对端实现 | 状态 |
|---------|------|---------|----------|------|
| D6→D2 | `evaluate/probe.go` 读 D2 trace | `compression_recall_probe`, `path_regression_probe` | D2 trace exporter | ACTIVE |
| D6→D3 | `evaluate/judge.go` + `evaluate/gateway_llm.go` 经 D3 LLM Gateway | `RuntimeJudge`, `GatewayLLMClient` | `internal/layers/llmgateway/stream/gateway.go` | ACTIVE |
| D6→D4 | `guard/observer.go` 订阅 D4 agent 事件 | `GuardObserver` (AgentObserver 实现) | D4 agent runtime | ACTIVE |
| D6→D5 | `guard/metrics.go` + `guard/observer.go` 写指标到 D5 | OpenTelemetry counters/histograms | D5 observability | ACTIVE |
| D6→D7 | `guard/types.go` + `guard/validator.go` 接收 D7 编排事件 | `RuntimeGuardValidator.OnDecision` | D7 turn adapter | ADVISORY（不阻塞 D7 主循环）|

### v2.0 路径迁移记录

| 旧路径 | 新路径 | 落地提交 | 备注 |
|--------|--------|---------|------|
| `eval/` | `evaluate/` | DM-20260615-003 (2026-06-15) | 与 D3 evaluate/ 对齐 |
| `orchestration/` | `guard/` | DM-20260615-003 (2026-06-15) | 避免与 D7 同名冲突；曾因重命名误删，从 42bf1d7 提交恢复 |
| `exporter/` | `export/` | DM-20260615-003 (2026-06-15) | 命名统一（其他域无 -er 后缀）|
| (无) | `verify/` | DM-20260615-003 (2026-06-15) | Invariant 验证从 evaluate 物理独立 |
| (bridge.go ×11) | (deleted) | v2.0.1 cleanup | bridge 桥接文件全部移除 |

---

## 历史留痕

| 日期 | 事件 | 关联 ID |
|------|------|---------|
| 2026-06-15 | v2.0 物理路径迁移（eval→evaluate / orchestration→guard / 新增 verify/） | DM-20260615-003 |
| 2026-06-14 | `orchestration/` → `guard/` 重命名时曾被 `rm -rf` 误删；git 42bf1d7 提交恢复 guard 子包 7 个 .go 文件（validator.go / intervention.go / observer.go / judge_adapter.go / types.go / config.go / metrics.go）+ `validator_test.go` | 42bf1d7 |
| 2026-06-14 | 新增 3 个探针：path_regression / layer_violation / session_isolation | V2.1 |
| 2026-06-10 | D6-S3 Eval Engine 完整实现（Pilot） | DM-20260610-XXX |
| 2026-06-19 | d6-domain.md 创建，对齐 D2/D7 d{N}-domain.md 结构 | DM-20260619-003 |

> **关键警告**：未来任何对 `internal/layers/evolution/guard/` 的大规模 `rm -rf` 操作**必须**先执行 `git ls-files internal/layers/evolution/guard/` 确认追踪状态；如与 42bf1d7 误删事件相同，立即停止并恢复。

---

## 规格文档索引

| 文档 | 用途 |
|------|------|
| `spec.md` | Gherkin 验收规格（v2.3.0） |
| `design.md` | 六段式详细设计（v2.2.0；含 D6-S3/S4/S5 三大子系统） |
| `d6-domain.md` | 本文件：域描述 + 价值流 + 跨域契约 |
| `layer-delta.md` | V1→V2→V2.1→V2.2 演进 Delta（V2.2 含 v2.0 物理路径迁移） |
| `a-registry.md` / `f-registry.md` / `t-registry.md` | A/F/T 登记 SoT |