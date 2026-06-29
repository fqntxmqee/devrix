---
demand-id: DM-20260629-006
title: D7 TaskContract 统一 — TaskSpec 四元组 + TaskReport 五元素契约整合
priority: P0
status: S6_Archived (2026-06-29, DESIGN ONLY)
dsaft_domain: orchestration
created: 2026-06-29
reporter: 2026-06-29 多层递归设计指南对照分析；v6.0.x 维护阶段向 v7.0 演进起点
related:
  - devrix-d7-dsaft-restructuring（DM-20260629-001）v6.0.x 维护阶段收官，Span Evidence 94%
  - devrix-d7-error-aggregation-and-metrics（DM-20260621-010）错误聚合闭环
  - devrix-d7-six-s-simplification（DM-20260626-001）14 S → 6 S 精简，v6.0.0 域升级
  - devrix-d7-mups-v5-escape-engine（DM-20260625-003）v5 EscapeEngine + CircuitBreaker L0-L5
  - devrix-d7-certainty-architecture UncertaintyCoord [0,1] 数值契约 + VERDICT 4 态
  - devrix-d7-multiturn-session-state（DM-20260628-003）多轮 session 串行化
  - Context Budget Phase A+B（DM-20260620-001）3-mode brief/fork/full + MaxSubagentDepth=3
  - 2026-06-29 Gemini 工程实践 review — 4 点补充（降级收敛/CoW 物化/防御性/接口硬化）
---

# D7 TaskContract 统一

## 1. 背景

2026-06-29 用户提交《多层递归循环的向下传播与向上反馈》设计指南，邀请对照 D7 编排领域识别可借鉴部分。实地阅读 D7 编排层 8 个子包代码（workmodel/sessionorchestrator/wavescheduler/decisionplanning/mups/execute、mups/learn、escape/orchtypes/hardening）后形成两个核心判断：

### 1.1 D7 v6.0.x 已超过指南基线

下行已实现 `ChildDownlink` 7 字段契约 + `WorkItem` 22 字段载体 + `ChannelRegistry` 4 PlanKind 路由；上行已实现 `Artifact` + `Verdict` + `UncertaintyCoord` 三件套数值契约；发散-收敛由 4 PlanKind + 5 层 CircuitBreaker + 3 通道记忆（Skill/Feedback/Scheduled）+ LP-1 BayesianUpdate 闭环保障。

### 1.2 但缺契约不缺机制

v6.0.x 的机制层（4 Channel / 5 CB / 3 Memory）已就位，**真正缺的是两个接口的强约束**：

- **TaskSpec**：分散在 Plan / Channel / WorkItem 三处定义，缺统一接口
- **TaskReport**：Verdict + Evidence + ExitReason 有；**Dissent / Blockage / Resource 三元素缺**

D7 深度 review（2026-06-21）暴露的 15+ 改进点反复出现在不同 PR，根因正是这两个契约没有强约束。`devrix-d7-dsaft-restructuring`（DM-20260629-001）已收官 v6.0.x 维护阶段，本 Change 是 v7.0 演进的第一枪。

### 1.3 Gemini 工程化 Review 4 点补充（2026-06-29 纳入）

用户在原指南对照分析后分享 Gemini 的"潜规则"补全，从工程实践、状态管理、极端异常处理 3 个维度扩展：

1. **降级收敛（Fallback to Heuristics）**：资源耗尽 + 置信度未达标时，强制 Rule-based 降级或 Pessimistic Commit（MVP + 风险警告）
2. **双态管理物化（Persistent Data Structure / CoW）**：子层只读父 Archive + Commit 仅追加 Delta + 版本链可回滚
3. **防御性设计（防递归塌陷 + 防上下层共谋）**：Downlink 相似度 > 80% 拦截；Reviewer 必须有硬证据（test_coverage_pct / log_excerpt / artifact_hash）
4. **接口契约补全（Trace ID + Cost Metric）**：TaskSpec/TaskReport 显式携带溯源与成本字段

→ 已映射为 AC11-AC15，详细对照见 §3 与 §6 风险表。

## 2. 问题陈述

### 2.1 TaskSpec 缺统一契约

Plan（`mups/execute`）、Channel（`mups/execute/channel.go`）、WorkItem（`workmodel/workitem.go`）三处定义**目标切片 + 硬约束 + 软偏好 + 收敛预算**时使用不同字段名和粒度：

- `plan.Plan` 有 `Strength/Steps/PersistScope` 但无显式收敛预算
- `WorkItem` 有 `ExecPolicy/BlockedBy/Blocks` 但与 Plan 字段未对齐
- `ChannelRequest` 只有 `SessionID + PriorVerdictKinds`，无法携带子任务的 directive/scope

后果：AdaptiveThreshold 接入 RunTurn（已识别 P1，TD-WT-01）时要做三处 `map[string]interface{}` 推断，类型不安全。

### 2.2 TaskReport 缺 Dissent / Blockage / Resource

- **Dissent（少数派报告）**：ExplorationChannel 全量保留结果（`exploration.go:144-147`），但 Learn 节点只记 best（`learner.go:233`），无 minority_plan + 否决理由 → 下一轮重规划时无法快速回忆"上次为什么否决了 B 方案"
- **Blockage（阻塞信号）**：Artifact 有 `Error` 字段但无结构化"缺什么信息/哪条路径不可行"
- **Resource（资源消耗）**：Context Budget Phase B 仅在入口层度量，无 per-Plan 资源快照

后果：MUPS Learn 节点（5 节点管道最后一环）的反馈信号不足，AdaptivePrior 注入噪声大。

### 2.3 发散-收敛单调性缺 metric

- 指南强调"每轮可行空间应单调缩小"
- D7 缺 `convergence.feasible_space_width` 或等价 span 度量
- fallback rate / re-plan ratio 缺

## 3. 验收标准

| ID | 标准 | 优先级 | Layer / Phase | 对应 P0/P1/P2 / Gemini 点 |
|----|------|--------|--------------|--------------------------|
| AC1 | `interfaces.TaskSpec` struct 定义，4 字段：Goal/HardConstraints/SoftPreferences/ConvergenceBudget + 2 元字段：TraceID/CostBudget；Plan/Channel/WorkItem 三处创建点统一通过该 type 构造 | P0 | **L1 / PR-A** | P1 + Gemini P4 |
| AC2 | `interfaces.TaskReport` struct 定义，5 字段：Result/Evidence/Dissent/Blockage/Resource + 2 元字段：TraceID/CostActual；Channel.Execute 输出 + Learn 节点输入统一通过该 type | P0 | **L1 / PR-A** | P0 + Gemini P4 |
| AC3 | `Dissent` 字段携带"少数派方案 + 否决理由 + 否决者"，Learn 节点沉淀至 SkillMemory.SOP，触发条件 = VERDICT=INDETERMINATE 或 fallback_used=true | P0 | **L2 / PR-A** | P0 |
| AC4 | `Blockage` 字段携带结构化"missing info / infeasible path / required external"，驱动重规划决策 | P1 | **L2 / PR-A** | P0 |
| AC5 | `Resource` 字段携带 per-Plan token / time / step 消耗，从 ContextBudget 现有埋点抽取 | P1 | **L2 / PR-A** | P0 |
| AC6 | 新增 span `convergence.feasible_space_width`（每次聚合后采样 W_up/W_down 比值） | P2 | **L4 / PR-C** | P2 |
| AC7 | AdaptiveThreshold 接入 RunTurn（解 TD-WT-01），不再需要三处 map[string]interface{} 推断 | P0 | **L4 / PR-C** | P1 |
| AC8 | Layout guard `interfaces` 包 + TaskSpec/TaskReport 创建点合规检查 | P1 | **L4 / PR-C** | P0 |
| AC9 | 22/22 orchestration packages `go test -race -count=1` PASS | P0 | **L4 / PR-B+C** | 验证 |
| AC10 | LP-1/LP-2/LP-5 100% 兼容（regression 测试集） | P0 | **L4 / PR-B+C** | 验证 |
| **AC11** | **Pessimistic Commit**：TaskReport.MVPArtifact 字段，资源耗尽时产出最小可行产物 + 风险警告，不无限期挂起；触发条件 = EscapeForceExit 或 budget exhausted | **P0** | **L3 / PR-B** | **Gemini P1** |
| **AC12** | **Rule-based Fallback**：VERDICT 多轮 INDETERMINATE 时强制规则降级；候选规则可插拔：单测最多 / 编译通过 / 最小代价 / 最低不确定性 | **P0** | **L3 / PR-C** | **Gemini P1** |
| **AC13** | **CoW Persistent**：`WorkItem.VersionChain []Hash` 字段 + 子层只读父 Archive snapshot + Commit 仅追加 Delta；支持任意历史版本 rollback | **P0** | **L3 / PR-C** | **Gemini P2** |
| **AC14** | **Similarity Check（防递归塌陷）**：Downlink 接收时校验"父 directive 与子 directive"语义相似度 > 80% 直接拦截，触发 Refine 或报错 | **P1** | **L3 / PR-C** | **Gemini P3** |
| **AC15** | **Hard Evidence（防共谋）**：Verifier 拒绝 Kind=Pass 但 `TestCoveragePct == 0 && LogExcerpt == "" && ArtifactHash == ""` 的"空证 Pass"；硬证据至少 1 项 | **P0** | **L3 / PR-B** | **Gemini P3** |
| **AC16** | **Migration Plan**：v6.0.x 类型别名（`type TaskSpec = interfaces.TaskSpecV1`）保留 1 个 minor 版本 + Deprecation warning 在 v7.0 输出 + v8.0 移除计划 | **P0** | **L4 / PR-B** | **DSAFT 治理** |
| **AC17** | **Spec 文档同步**：`openspec/specs/d7-orchestration/spec.md` v7.0 spec 必含 5+1+2 字段定义 + Layer 分层；`openspec/specs/d7-orchestration/d7-domain.md` v7.0 同步；`tasks.md` 拆解到 S3-Gate | **P0** | **L4 / PR-A** | **DSAFT 治理** |
| **AC18** | **Test Coverage ≥ 80%**：新增 `interfaces` 包 + `workmodel.VersionChain` + `escape/fallback` 全量测试覆盖率 ≥ 80%（遵循 `openspec/specs/project/testing.md`） | **P0** | **L4 / PR-C** | **DSAFT 治理** |
| **AC19** | **Performance Budget**：TaskSpec/TaskReport 构造 P99 < 1ms；VersionChain 查找 O(1)（hash index）；Similarity Check embedding 命中 O(1)（缓存） | **P1** | **L4 / PR-C** | **DSAFT 治理** |
| **AC20** | **Security/Privacy Classification**：`Dissent.Reason` + `LogExcerpt` 字段打 `Classification` 标签（`internal` / `confidential` / `secret`），Learn 节点沉淀时按标签过滤；不强制 sanitize（保留原始内容便于 review） | **P1** | **L4 / PR-C** | **DSAFT 治理** |
| **AC21** | **Cross-Domain Boundary**：TaskSpec 在 D2 (context budget 注入) / D4 (multi-agent worker consume) / D6 (evolution observer) 三处的边界检查；每个跨域消费点必须写 boundary test | **P0** | **L4 / PR-B** | **DSAFT 治理** |
| **AC22** | **Feature Flag 灰度**：AC11 (Pessimistic) + AC13 (CoW) 必须 env-gated（参考 `D7 6s-bootstrap-slim` 经验），默认 `disabled`，prod 灰度 1% → 10% → 50% → 100%，每步观察 24h | **P0** | **L4 / PR-B** | **DSAFT 治理** |
| **AC23** | **Error Code 闭合**：AC11-AC15 新错误必须挂 `ORCH_*` SentinelError（参考 `internal/shared/errors/`），每个新错误有 `Code + Message + Remediation` 三元组；missing 时 CI gate 拦截 | **P0** | **L4 / PR-B** | **DSAFT 治理** |

**Layer 维度（DSAFT-aligned）**：
- **Layer 1（接口层）**：AC1, AC2 — Pure types，0 行为
- **Layer 2（字段语义层）**：AC3, AC4, AC5 — 5+2 字段运行时语义
- **Layer 3（防御运行时层）**：AC11, AC15（PR-B 低风险）→ AC13, AC12, AC14（PR-C 高风险）
- **Layer 4（治理横切层）**：AC6-AC10（验证/observability/tooling）+ AC16-AC23（迁移/spec/coverage/perf/security/边界/灰度/错误码）

**总计 23 AC**（原 15 + 新 8），其中 P0 × 14，P1 × 4，验证 × 2，P2 × 1；跨 4 Layer + 3 Phase。

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 依赖 | `ChildDownlink`（已存在）作为 Downlink 端参考 |
| 依赖 | `UncertaintyCoord` + `Verdict`（已存在）作为 Uplink 端基础 |
| 依赖 | `Context Budget Phase B`（已存在）作为 Resource 度量基础 |
| 依赖 | `MUPS Learn` 5 节点管道（已存在）作为 Dissent 沉淀目标 |
| 约束 | 不破坏现有 v6.0.x API，向后兼容到 v7.0 |
| 约束 | P0/P1 复用 v6.0.x 已就位机制，不引入新外部依赖 |
| 约束 | 符合 D7 六 S（WorkModel / SessionOrchestrator / WaveScheduler / ExecutionFlow+Verify / DecisionPlanning+Observe / MUPS Pipeline）+ Hardening 横切 |

## 5. 变更范围

### 新增

- `internal/layers/orchestration/interfaces/` 包（新）
  - `task_spec.go` — TaskSpec struct + 4 字段 + builder
  - `task_report.go` — TaskReport struct + 5 字段 + builder
- `internal/layers/orchestration/interfaces/testdata/` — golden test fixtures

### 修改

- `mups/execute/channel.go` — `ChannelRequest` 升级为 `TaskSpec`
- `mups/execute/exploration.go` — 全量结果 → `TaskReport.Dissent` 字段填充
- `mups/learn/learner.go` — `LearnRequest` 升级为 `TaskReport`，Learn.资产化时记录 Dissent
- `workmodel/workitem.go` — `WorkItem` 创建路径统一返回 `TaskSpec`
- `decisionplanning/decomposer.go` — 分解产出 `TaskSpec` 而非裸 `Plan`
- `escape/circuit_breaker.go` — 5 层 CB 接入 `TaskReport.Blockage` 作为升级信号
- `d7-domain.md` — v7.0 spec 升级：双契约纳入

### 不变更

- `UncertaintyCoord` / `Verdict` / `ChildDownlink` 现有结构（向后兼容）
- 4 PlanKind → 4 Channel 路由
- 5 层 CircuitBreaker 阈值

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| TaskSpec/TaskReport 引入后老调用方断裂 | 大面积编译失败 | 保留 v6.0.x 类型别名 + 渐进迁移 + Layout guard 仅检查新文件 |
| Dissent 字段数据量大，SkillMemory 写入变慢 | Learn 节点延迟 | Dissent 只保留 top-N（默认 3）+ summary 哈希引用 |
| Resource 字段需重新埋点 | 增加 instrumentation 改动 | 复用 Context Budget Phase B 现有 metric，仅做字段抽取 |
| AC7 接入 RunTurn 触发未识别 bug | 上线回归 | 分两阶段：PR-A 接入但默认禁用（env flag），PR-B 启用 |
| 跨包 import cycle | interfaces 包 vs mups/learn 互引 | interfaces 包只放 type + builder，不依赖 D7 任何子包（Pure types） |
| **AC11 Pessimistic Commit 误触发**（资源尚有富余时降级）| 提前退出导致产物质量降低 | 仅在 EscapeForceExit 或 budget 真正归零时触发；MVP 输出必须保留上游 ChainHash 引用 |
| **AC12 Rule-based Fallback 规则选择错误** | 选错"次优"分支反而比随机差 | 候选规则配 A/B test env flag；fallback 路径全量埋 span + 离线评估 |
| **AC13 CoW 引入版本链膨胀** | WorkItem 存储变大 | 旧版本压缩归档（GC 周期 24h）；VersionChain 只保留 hash 而非 full state |
| **AC14 Similarity Check LLM 调用开销** | 每 Downlink 多一次 LLM 调用 | 先做 embedding 哈希 + cosine 阈值（O(1)），仅在边界 0.7-0.85 才升级 LLM 二次校验 |
| **AC15 Hard Evidence 误伤合法轻量任务**（如聊天/Q&A 无测试） | Verifier 误判导致 FAIL | Hard Evidence 改为 Verifier.kind-specific 配置：code 任务要 test/log，chat 任务要 entity_hash/coherence_score |
| **AC16 Migration Plan 类型别名混乱** | v6.0.x 老调用方 + v7.0 新调用方并存期长，deprecation warning 噪声大 | 仅在 changelog + devrix.log 输出一次 warning；CI 加 `find . -name "*.go" | xargs grep "interfaces\.TaskSpecV1" | wc -l` 必须 ≤ N |
| **AC17 Spec 文档漂移**（代码改了 spec 没改）| 后续 reviewer 误判设计意图 | PR-CI gate：`git diff --stat openspec/specs/d7-orchestration/ \| grep -v "spec.md\|domain.md"` 必须为 0（除非有显式 reason）|
| **AC18 Coverage 不达标** | 跨 8 个文件改动，覆盖率稀释 | 每个 PR 必须带 coverage delta 报告；下降 > 2% 触发 S4-Gate 拒绝 |
| **AC19 Performance Budget 不达标**（VersionChain 慢于 O(1)）| WorkItem 访问全链路变慢 | P99 metric 在 PR-B 灰度期持续监控；超阈值自动 rollback 到 v6.0.x WorkItem 实现 |
| **AC20 Security Classification 误标** | 敏感数据漏过滤 → 合规问题；过严 sanitization → 失去 review 价值 | Classification 标签由 producer 侧（产生方）打，consumer 侧（Learn 节点）只 read 不改；review 期抽样 100 条人工核对 |
| **AC21 Cross-Domain Boundary 漏检** | D2/D4/D6 consumer 不知道新字段存在 → 静默忽略 | 每个跨域消费点必须写 boundary test（`boundary_test.go` 后缀）；CI grep `import.*orchestration/interfaces` 必须对账到 boundary_test 文件 |
| **AC22 Feature Flag 灰度失败** | AC11/AC13 全量上线触发生产事故 | 灰度失败自动 `RolloutDisable()`；rollback 命令封装到 `./scripts/devrix.sh rollback-flag` |
| **AC23 Error Code 不闭合**（新错误未挂 ORCH_*）| 上层无法分类处理，故障定位慢 | CI gate：`grep -r "errors.New\|fmt.Errorf" --include="*.go" internal/layers/orchestration/interfaces/` 必须匹配 `ORCH_*` 模式，否则拒绝合入 |
| **全部 23 AC 的执行风险** | 3.5-4 周跨多个 PR，节奏失控 | 每个 PR 配独立 S3-Gate + S4-Gate；AC9-AC10 验证跨 PR 累积；AC17 spec 同步是 PR 合并前置条件 |

## 7. 后续 Phase 关联 — DSAFT 4-Layer × 3-Phase 二维矩阵

### 7.1 矩阵视图

| Layer ↓ \ Phase → | **PR-A**（1 周，低风险） | **PR-B**（2 周，中风险） | **PR-C**（1.5 周，高风险） |
|---|---|---|---|
| **L1 接口层** | AC1, AC2 | — | — |
| **L2 字段语义层** | AC3, AC4, AC5 | — | — |
| **L3 防御运行时层** | — | AC11（低风险）, AC15（低风险）| AC13（高）, AC12（中）, AC14（中）|
| **L4 治理横切层** | AC17 (spec 同步) | AC9, AC10, AC16, AC21, AC22, AC23 | AC6, AC7, AC8, AC18, AC19, AC20 |

### 7.2 阶段详细说明

**Phase A — 接口 + 字段语义 + spec 同步**（PR-A，1 周）
- **L1 接口层**：AC1 (TaskSpec), AC2 (TaskReport) — Pure types，0 行为改动
- **L2 字段语义层**：AC3 (Dissent), AC4 (Blockage), AC5 (Resource) — 5+2 字段填充逻辑
- **L4 治理**：AC17 (spec 文档同步) — PR 合并前置条件
- **风险**：低 | **依赖**：无 | **可独立合入**

**Phase B — 防御运行时低风险 + 治理基础**（PR-B，2 周）
- **L3 防御运行时（低风险先行）**：AC11 (Pessimistic Commit), AC15 (Hard Evidence) — 资源耗尽与空证拦截
- **L4 治理基础**：AC9, AC10（验证）, AC16 (Migration Plan), AC21 (Cross-Domain), AC22 (Feature Flag), AC23 (Error Code)
- **风险**：中 | **依赖**：PR-A 合入 | **feature flag 灰度**：AC11/AC15 默认 disabled

**Phase C — 防御运行时高风险 + 治理收口**（PR-C，1.5 周）
- **L3 防御运行时（高风险）**：AC13 (CoW VersionChain), AC12 (Rule-based Fallback), AC14 (Similarity Check)
- **L4 治理收口**：AC6 (convergence span), AC7 (AdaptiveThreshold wiring), AC8 (Layout guard), AC18 (Coverage), AC19 (Performance), AC20 (Security Classification)
- **风险**：高 | **依赖**：PR-B 合入 | **灰度节奏**：AC13 1% → 10% → 50% → 100%，每步 24h 观察

### 7.3 工作量估算

| Phase | AC 数 | 估时 | 累计 |
|-------|------|------|------|
| PR-A | 6 AC（AC1-5 + AC17）| 1 周 | 1 周 |
| PR-B | 8 AC（AC9-11 + AC15-16 + AC21-23）| 2 周 | 3 周 |
| PR-C | 9 AC（AC6-8 + AC12-14 + AC18-20）| 1.5 周 | 4.5 周 |
| **总计** | **23 AC** | **4.5 周** | — |

**说明**：原提案 2.5 周 → 新提案 4.5 周（增 80%），但通过二维矩阵分层，每个 PR 仍是单一主题、可独立 S3/S4 Gate。

### 7.4 可选延后项（不阻塞主线）

- Reference Adapter：`TaskSpec → plan.Plan` / `TaskReport → Artifact` 参考实现 — 留作 v7.0.x 维护期
- Operator Runbook：fallback / collapse 触发运维手册 — 与 hardening/metrics.go 配套
- Interface Semver：`interfaces/v2` 子包路径，便于未来 v2 演进 — v8.0 规划

## 8. 参考资料

- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-v7-horizon-taskcontract.md` — v7.0 horizon 立项分析
- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-downlink-uplink-code-2026-06-29.md` — code-level 现状核实
- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-gemini-engineering-review-2026-06-29.md` — Gemini 4 点工程化补充
- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-certainty-architecture.md` — UncertaintyCoord + VERDICT 4 态
- `~/.claude/projects/-Users-fukai-workspace/memory/devrix-d7-multiturn-session-state.md` — 多轮 session 串行化
- 指南原文（用户提供）：《多层递归循环的向下传播与向上反馈：平衡发散与收敛的设计指南》
- Gemini 工程实践 review（用户提供，2026-06-29）：降级收敛 / CoW 物化 / 防御性 / 接口硬化 4 点补充