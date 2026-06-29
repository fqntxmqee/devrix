# Proposal: D7 TaskContract 统一 — interfaces 包 + TaskSpec/TaskReport 双契约 + 4-Layer × 3-Phase 落地

**Change ID:** devrix-d7-taskcontract-unification
**Demand ID:** DM-20260629-006
**Status:** S6_Archived (2026-06-29, DESIGN ONLY — implementation deferred to v7.0)
**Priority:** P0
**Reporter:** 2026-06-29 多层递归设计指南对照分析 + Gemini 工程实践 review + v6.0.x 维护阶段向 v7.0 演进起点
**DSAFT Domain:** D7 Orchestration（核心域）
**DSAFT Layer:** Architecture (S2) + Design (S3) + Implementation (S4) 全栈

---

## 1. Background

### 1.1 触发事件

2026-06-29 用户提交《多层递归循环的向下传播与向上反馈：平衡发散与收敛的设计指南》，邀请对照 D7 编排领域识别可借鉴部分。实地阅读 D7 编排层 8 个子包代码（workmodel / sessionorchestrator / wavescheduler / decisionplanning / mups/execute / mups/learn / escape / orchtypes / hardening）后形成两个核心判断：

1. **D7 v6.0.x 已超过指南基线**：下行 `ChildDownlink` 7 字段 + `WorkItem` 22 字段 + 4 PlanKind 路由；上行 `Artifact` + `Verdict` + `UncertaintyCoord` 三件套；发散-收敛由 4 Channel / 5 CB / 3 Memory / LP-1 BayesianUpdate 闭环保障。
2. **缺契约不缺机制**：v6.0.x 机制层已就位，真正缺的是**接口强约束**：
   - TaskSpec 分散在 Plan / Channel / WorkItem 三处
   - TaskReport Verdict + Evidence 有，**Dissent / Blockage / Resource 三元素缺**

### 1.2 后续 Review 输入

- **2026-06-29 Gemini 工程实践 Review**：4 点补充（降级收敛 / CoW 物化 / 防御性 / 接口硬化）
- **2026-06-21 D7 深度 review**：15+ 改进点
- **2026-06-29 self-review**：AC 完整性 + 分层清晰度

### 1.3 与 v6.0.x 维护阶段的关系

`devrix-d7-dsaft-restructuring`（DM-20260629-001）已于 2026-06-29 S7_Archived 收官 v6.0.x 维护阶段（Span Evidence 30%→94%，god fn 拆完，6 子 Change 联动）。本 Change 是 **v7.0 演进的第一枪**，目标：从"机制层丰富但契约层分散"演进到"接口 + 行为 + 防御 + 治理"四维均衡。

## 2. Problem Statement

| ID | 问题 | 影响面 | 优先级 |
|----|------|--------|--------|
| **P1** | TaskSpec 在 Plan / Channel / WorkItem 三处定义，4 元素（目标/硬约束/软偏好/收敛预算）用不同字段名 | AdaptiveThreshold 接入 RunTurn（TD-WT-01）需三处 `map[string]interface{}` 推断，类型不安全 | P0 |
| **P2** | TaskReport 缺 Dissent / Blockage / Resource 三元素 | MUPS Learn 节点 AdaptivePrior 注入噪声大，无法做精细重规划 | P0 |
| **P3** | Pessimistic Commit 缺失 | 资源耗尽 + 置信度未达标时只能 502 而非输出 MVP，可用性差 | P0 |
| **P4** | Rule-based Fallback 缺失 | VERDICT 多轮 INDETERMINATE 时只能 abort，无法用规则选次优分支 | P0 |
| **P5** | CoW Persistent 缺失（WorkItem 无版本链） | 子层 Commit 可覆盖父层认知，重规划时无法回溯历史 | P0 |
| **P6** | Similarity Check 缺失 | 子层惰性层层转包会烧光 token 预算，无防御机制 | P1 |
| **P7** | Hard Evidence 缺失 | 子层用"官话废话"满足硬约束可被 Verifier 误判为 PASS（Collusion）| P0 |
| **P8** | Trace ID + Cost Metric 散落（SessionID + d7spans + ContextBudget Phase B） | 工业级监控缺字段，跨域追溯困难 | P0 |
| **P9** | Migration Plan 缺失 | v6.0.x → v7.0 无明确 deprecation timeline | P0 |
| **P10** | Cross-Domain Boundary 缺失 | D2/D4/D6 consumer 不知道新字段存在 → 静默忽略 | P0 |

## 3. Proposed Solution

### 3.1 方案对比

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| **A. TaskContract 统一（推荐）** — 一次性引入 `interfaces` 包 + TaskSpec/TaskReport 双契约 + 23 AC 跨 4 Layer | 治本而非补丁；4-Layer × 3-Phase 二维分层清晰；Gemini 4 点全吸收 | 单 Change 23 AC，工作量大 | ✅ **采用** |
| B. 拆 3 个 Change（TaskSpec / TaskReport / Gemini 硬化）| 单一 PR 风险低 | 3 次 OpenSpec 流程；interfaces 包被 3 个 PR 重复 review | ❌ |
| C. 仅做机制层 hardening（不改契约）| scope 小 | 不解决 P1/P2 根因，6/21 review 的 15+ 改进点继续累积 | ❌ |
| D. 延迟到 v8.0 | 不阻塞 v7.0 维护 | MUPS Learn 节点 AdaptivePrior 注入噪声无法解决 | ❌ |

### 3.2 核心架构：4-Layer × 3-Phase 二维矩阵

```
                    PR-A（1 周，低风险）         PR-B（2 周，中风险）         PR-C（1.5 周，高风险）
                   ─────────────────────       ─────────────────────       ─────────────────────
L1 接口层          AC1 TaskSpec                                      
                   AC2 TaskReport                                     
                   ─────────────────────       ─────────────────────       ─────────────────────
L2 字段语义层      AC3 Dissent                                        
                   AC4 Blockage                                       
                   AC5 Resource                                       
                   ─────────────────────       ─────────────────────       ─────────────────────
L3 防御运行时层                            AC11 Pessimistic Commit    AC13 CoW VersionChain
                                            AC15 Hard Evidence        AC12 Rule-based Fallback
                                                                       AC14 Similarity Check
                   ─────────────────────       ─────────────────────       ─────────────────────
L4 治理横切层      AC17 spec 同步            AC9  race test             AC6  convergence span
                                            AC10 LP regression         AC7  AdaptiveThreshold
                                            AC16 Migration Plan        AC8  Layout guard
                                            AC21 Cross-Domain Boundary AC18 Coverage
                                            AC22 Feature Flag          AC19 Performance
                                            AC23 Error Code            AC20 Security Class
```

**Layer 维度（DSAFT-aligned）**：
- **L1 接口层** = "做什么"（Pure types，0 行为）
- **L2 字段语义层** = "数据怎么填"（5+2 字段运行时语义）
- **L3 防御运行时层** = "运行时怎么防御"（Gemini P1/P2/P3 行为落地）
- **L4 治理横切层** = "如何保证质量"（验证/observability/spec/迁移/灰度）

**Phase 维度（时间）**：
- **PR-A** = L1 + L2 + AC17（接口 + 字段语义 + spec）= 6 AC
- **PR-B** = L3 低风险 + L4 基础（防御先行 + 治理基础）= 8 AC
- **PR-C** = L3 高风险 + L4 收口（CoW + Fallback + 治理）= 9 AC

### 3.3 三层契约

1. **接口层（L1）**：`internal/layers/orchestration/interfaces/` 包
   - `task_spec.go` — TaskSpec struct（Goal/HardConstraints/SoftPreferences/ConvergenceBudget + TraceID/CostBudget）+ builder
   - `task_report.go` — TaskReport struct（Result/Evidence/Dissent/Blockage/Resource + TraceID/CostActual）+ builder
   - **Pure types**：不依赖 D7 任何子包（防 import cycle）

2. **字段语义层（L2）**：Dissent / Blockage / Resource 3 字段的填充逻辑
   - **Dissent**：ExplorationChannel 全量结果 → top-3 保留 + summary 哈希引用
   - **Blockage**：来自 Verifier 拒绝原因 + LLM 二次分析
   - **Resource**：从 ContextBudget Phase B 现有 metric 抽取

3. **运行时层（L3）**：3 类防御行为
   - **Pessimistic Commit**：TaskReport.MVPArtifact 字段
   - **Rule-based Fallback**：候选规则可插拔（单测最多 / 编译通过 / 最小代价 / 最低不确定性）
   - **CoW Persistent**：WorkItem.VersionChain []Hash + 子层只读父 snapshot + Commit 仅追加 Delta
   - **Similarity Check**：embedding 哈希 + cosine 阈值（O(1)），仅在边界 0.7-0.85 升级 LLM 二次校验
   - **Hard Evidence**：TestCoveragePct / LogExcerpt / ArtifactHash 至少 1 项

### 3.4 关键决策

#### Decision 1: interfaces 包路径

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. `internal/layers/orchestration/interfaces/` | 与 D7 同包树，import 路径短；layout guard 自然纳入 | 与其他域的 interfaces 包需重命名协调 |
| B. `internal/layers/orchestration/contracts/` | D7 内部专用 | 与 `decisionplanning/filter_adapter.go` 等现有 contracts 文件重名 |
| C. `internal/shared/orchestration/interfaces/` | 跨域共享 | 当前无跨域需求，over-engineering |

**选择:** A
**理由:** D7 编排层是 TaskSpec/TaskReport 的唯一 owner；A 是命名最少惊讶方案；与 v6.0.x `coordinator/aliases.go` 的 legacy shim 路径明确区分。

#### Decision 2: CoW 版本链存储策略

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 只存 hash（VersionChain []Hash） | 存储最小；O(1) 查找 | 历史版本需另存（GC 复杂度）|
| B. 全 state inline（VersionChain []WorkItem） | 历史完整；回滚直接 | 存储爆炸（10x-100x）|
| C. hash + 后台 GC（hash 索引 + 24h 延迟清理）| 存储可控；回滚需 hash → 加载 | 实现复杂 |

**选择:** C
**理由:** 工业级需要"可回滚"+"存储可控"两条都要；C 是唯一同时满足的方案。GC 周期 24h（参考 D5 已有 `metrics.go` retention 经验）。

#### Decision 3: Pessimistic Commit 触发条件

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. EscapeForceExit OR budget exhausted | 触发明确；语义清楚 | 可能漏掉部分场景（如 verifier 持续 Indeterminate 但 budget 未耗尽）|
| B. 连续 3 轮 INDETERMINATE 强制触发 | 更激进 | 误伤"接近收敛"的慢任务 |
| C. A OR B 任一即触发 | 覆盖广 | 规则复杂；reviewer 难判断 |

**选择:** A
**理由:** Gemini P1 的本意是"资源耗尽时优雅降级"，A 是字面实现；B 的"连续 N 轮 INDETERMINATE"在 v4.4 VERDICT 4 态下已有 LearningPending 路径覆盖，不重复。

#### Decision 4: Similarity Check 的相似度算法

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. embedding + cosine（O(n) embedding 算） | 语义精准 | 每 Downlink 多 1 次 LLM embedding 调用，慢 |
| B. token-level Jaccard 相似度（O(1)） | 极快；纯字符串 | 误判（"你好"和"你好啊"被判定高相似）|
| C. B + 边界 LLM 二次校验（embedding 只在 0.7-0.85 边界触发）| 快 + 准 | 实现稍复杂 |

**选择:** C
**理由:** Gemini P3 防递归塌陷的"80% 阈值"是经验值，实际边界模糊；C 用 O(1) token-Jaccard 做粗筛，仅在边界区间调用 embedding，平衡性能与精度。

#### Decision 5: Hard Evidence 的最小集

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 固定 3 项（test_coverage_pct + log_excerpt + artifact_hash）至少 1 | 简单 | chat / Q&A 等轻量任务无 test 必失败 |
| B. Verifier.kind-specific 配置（code 任务要 test/log，chat 任务要 entity_hash/coherence_score）| 灵活 | 配置点增加 |
| C. A + Verifier 可声明禁用某项 | 折中 | 复杂度 |

**选择:** B
**理由:** Gemini P3 防共谋的"硬证据"语义在不同任务下不同；B 用 task-specific 配置保持严谨的同时避免误伤。

#### Decision 6: Migration Plan 路径

**选项:**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. v6.0.x 保留 1 minor 版本（type alias），deprecation warning，v8.0 移除 | 标准 deprecation 路径 | 用户需在 v7.x 周期内迁移 |
| B. 直接 breaking change，强升 v7.0 | 一次性 | 风险大（生产事故）|
| C. A + 灰度阶段（先纯 alias 编译警告，后运行时 warn，最后强制）| 渐进 | 周期长 |

**选择:** C
**理由:** devrix v6.0.x 是生产环境；breaking change 必须有 warning → 灰度 → 强制三段过渡；C 是 industry standard（Cobra / k8s 都在用）。

## 4. Success Metrics

| 指标 | 当前值（v6.0.x）| 目标值（v7.0）| 测量方式 |
|------|----------------|----------------|----------|
| `interfaces.TaskSpec` / `TaskReport` 覆盖率（3 处创建点）| 0% | 100% | grep `plan.New / channel.New / workitem.New` 必须返回 `interfaces.New*` |
| Dissent 字段填充率（VERDICT=INDETERMINATE 时）| 0% | 100% | LP-3 测试 + Jaeger span `taskreport.dissent_recorded` |
| Pessimistic Commit 触发准确率 | N/A | 误触发 < 1% / 漏触发 = 0 | 灰度期 metric：MVP emit count / budget exhausted count 比值 |
| CoW VersionChain 平均长度 | N/A | ≤ 10 / session | `worktree.go` 新增 metric |
| Similarity Check 拦截率（应拦截的塌陷）| 0% | ≥ 95% | 离线回放 100 条历史 session 验证 |
| Hard Evidence 误伤率（合法 Pass 被拒）| N/A | < 5% | Verifier.kind-specific 配置覆盖 5 类典型任务 |
| 22/22 orchestration packages `-race` PASS | 100%（v6.0.x baseline）| 100%（不退化）| `go test -race -count=1` |
| LP-1/LP-2/LP-5 100% 兼容 | 100%（v6.0.x baseline）| 100% | 回归测试集 |
| Test Coverage（新增 `interfaces` 包）| N/A | ≥ 80% | `go test -cover` |
| Performance Budget（TaskSpec 构造 P99）| N/A | < 1ms | `benchstat` + Jaeger histogram |

## 5. Implementation Plan

| PR | 内容 | Layer | 依赖 | 预估行数 |
|----|------|-------|------|---------|
| **PR-A** | interfaces 包 + 5+2 字段类型 + 填充逻辑 + spec 同步 | L1+L2+L4(AC17) | 无 | +800 / -50 |
| **PR-B** | Pessimistic Commit + Hard Evidence + 治理基础（迁移/边界/灰度/错误码）| L3(low) + L4(base) | PR-A | +1200 / -100 |
| **PR-C** | CoW VersionChain + Rule-based Fallback + Similarity Check + 治理收口（span/coverage/perf/security）| L3(high) + L4(finish) | PR-B | +1500 / -200 |

**总预估：~3500 行新增，~350 行修改**

PR 内部子任务拆分见 `tasks.md`（S4 阶段产出）；预估行数仅作方案级参考，最终以 S3-Gate review 后 tasks.md 拆解为准。

## 6. Risks & Mitigations

完整风险表见 `demand.md` §6（22 条风险）。本节只列**架构级 P0 风险**：

| 风险 | 影响 | 缓解 |
|------|------|------|
| interfaces 包 import cycle | 编译失败 | interfaces 包只放 type + builder，不依赖 D7 任何子包（Pure types）|
| Pessimistic Commit 误触发 | 提前退出，产物质量低 | 仅 EscapeForceExit 或 budget 真正归零时触发；MVP 必须保留上游 ChainHash 引用 |
| CoW VersionChain 膨胀 | WorkItem 存储变大 | hash 索引 + 24h 后台 GC；VersionChain 只保留 hash 不 inline state |
| Hard Evidence 误伤轻量任务 | Verifier 误判 FAIL | Verifier.kind-specific 配置（code 要 test/log，chat 要 entity_hash/coherence_score）|
| 跨域消费点静默忽略新字段 | D2/D4/D6 边界泄漏 | 每个跨域消费点必须写 boundary_test.go；CI grep 对账 |
| Feature Flag 灰度失败 | 生产事故 | 灰度失败自动 `RolloutDisable()`；rollback 命令封装到 `./scripts/devrix.sh rollback-flag` |
| 23 AC 跨 3 PR 累积延迟 | 节奏失控 | 每个 PR 独立 S3/S4 Gate；AC9/AC10 验证跨 PR 累积；AC17 spec 同步是 PR 合并前置条件 |

## 7. Out of Scope

- **interfaces/v2 子包演进** — v8.0 规划（保留 `WithPriorContextRounds` functional option 风格）
- **Reference Adapter 全量实现** — v7.0.x 维护期补 `TaskSpec → plan.Plan` / `TaskReport → Artifact`
- **Operator Runbook** — fallback / collapse 触发运维手册与 `hardening/metrics.go` 配套
- **Cross-session UncertaintyCoord 合并** — v7.0.x 维护期
- **MUPS Learn 节点的 BayesianUpdate 重构** — 独立 Change（Learn v2.7）

## 8. 关联变更

| Change ID | DM ID | 关系 | 说明 |
|-----------|-------|------|------|
| devrix-d7-dsaft-restructuring | DM-20260629-001 | 前置 | v6.0.x 维护收官，本 Change 是 v7.0 第一枪 |
| devrix-d7-six-s-simplification | DM-20260626-001 | 前置 | 14 S → 6 S 精简，本 Change 落在 L1/L2 |
| devrix-d7-mups-v5-escape-engine | DM-20260625-003 | 前置 | 5 层 CB，本 Change AC11 接入 |
| devrix-d7-certainty-architecture | (project memory) | 前置 | UncertaintyCoord + VERDICT 4 态，本 Change 复用 |
| devrix-d7-multiturn-session-state | DM-20260628-003 | 并行 | 同周活跃，S3-Gate 互审 |
| devrix-d7-mups-v4-5node-coverage-orchestration | (active) | 并行 | AC7 接线可能涉及 |
| devrix-d2-mock-semantic-split | (merged PR #117) | 参照 | Layout guard allow-list 维护经验 |

## 9. 备注

本次 S2 proposal 同步推进的子规范符合度：
- ✅ `architecture-design.md §1.1` DSAFT 五层（D + S + A + F + T）已纳入 §1.1 + §3.2
- ✅ `architecture-design.md §3` proposal 7 sections 已完整（+ §3.4 关键决策 6 条 + §8 关联变更 + §9 备注）
- ✅ `architecture-design.md §5` 禁止工时估算：本提案无 hours 估算，仅 line counts（§5 PR 表）
- ✅ `architecture-design.md §7` 设计决策记录：§3.4 已列 6 个 Decision
- ✅ `architecture-design.md §8` S2 检查清单：`.openspec.yaml` 字段 + `dsaft_scenarios` 标注见 S3 design.md

下一阶段 S3 design.md 将产出：
- 根因分析（6/21 deep review 的 15+ 改进点如何被本 Change 系统解决）
- 关键接口/类型完整定义
- 数据流图（TaskSpec / TaskReport 在 D7 6 S + Hardening 横切的全链路）
- 文件清单（新增 interfaces 包 + 9 个修改文件 + 跨域 boundary_test）
- 回归风险评估
- Rollback 计划（Feature Flag + VersionChain GC 触发）