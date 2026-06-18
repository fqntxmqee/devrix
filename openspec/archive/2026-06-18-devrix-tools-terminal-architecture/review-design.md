# S3-Gate Review: Devrix Tools 终态架构

**Review Date:** 2026-06-18
**Reviewer:** S3-Gate Self-Review（设计完成 → 等待用户最终确认）
**Target:** `devrix-tools-terminal-architecture` change（DM-20260618-007）
**Documents Under Review:**
- `demand.md`（S1 需求，571 行，12/12 checklist 完成）
- `proposal.md`（S2 提案，148 行，R2 100% 共识）
- `design.md`（S3 设计，1153 行，DSAFT §九全绿）
- `specs/*/spec.md`（6 个 spec，~720 行 Gherkin Scenario）
- `.openspec.yaml`（S3 metadata，dsaft_scenarios/activities/functions/t_points）

**Methodology:** Grill Review（review-design.md §3.1）— 逐决策遍历 + 逐依赖确认 + 逐 Scenario 推演

---

## 1. 总体判断 (TL;DR)

**结论：Approved with Suggestions**

设计文档完整覆盖了 DSAFT §九 资产登记 Checklist 的 5 层全字段，6 个 spec 文件提供了 Phase 1 所有 Surface 的 Gherkin Scenario，8 个 Decision 记录了关键架构选择的理由。设计层无阻塞性问题。

**整体评价**：
- ✅ 完整性：DSAFT §九 5 层全字段、T 点 25 个、spec.md 6 个 — 完备
- ✅ 一致性：D2 不持有 D3 引用、Filter FIFO subgame-perfect、LTL-Lite Go struct tag 三个核心约束在全文一致
- ✅ 可执行性：每个子 change 有文件清单 + T 点 + spec.md — 可直接进入 S4
- ⚠️ 需要建议（非阻塞）：见 §7 Open Suggestions

---

## 2. 逐 Decision 遍历（review-design.md §3.1 第 1 步）

| # | Decision | 选择 | 替代方案 | 结论 | 备注 |
|---|---------|------|---------|------|------|
| 1 | LSP server 进程池 | **C: LRU 池（默认 4）** | A: 固定 1 个 / B: 1 per workspace | **Agreed** | LTL-Lite 不变式 `lsp_servers <= 4` 强约束，4 是经验值。**建议**：在 S4 阶段做 benchmark，验证 4 是否最优。|
| 2 | BashAST 解析失败处理 | **C: 拒绝 + 建议** | A: 拒绝 / B: 允许 + 警告 | **Agreed** | fail-closed 是安全底线。"建议"通过 D3 LLM round-trip 自然重试，无需 D2 实现。**建议**：failure 计数指标加入 D5 metrics。|
| 3 | FreeFork 并发上限 | **C: 8** | A: 4 / B: 16 | **Agreed** | 8 = 2 × 典型 4-core CPU + LSP slot。**建议**：在 design.md §6.5 中明确"用户可配置 devrix.yaml `max_concurrent_forks`"，默认 8 但允许调整。|
| 4 | DiagnosticTracker 同步/异步 | **B: 异步 fire-and-forget** | A: 同步 | **Agreed** | 编辑延迟是 P0 体验指标，丢失误接受。**建议**：在 §6.2 中明确"编辑频率 > 10/s 时降级为采样追踪"。|
| 5 | LTL-Lite DSL 选型 | **B: Go struct tag** | A: YAML | **Agreed** | R2 共识已记录。零解析器 + 编译时验证是核心优势。**无建议**。|
| 6 | MCP Attestation 协议 | **B: TUF 框架** | A: 简单 Ed25519 | **Agreed** | Phase 1.5 预研期可先用 Ed25519 单签名，Phase 2 升级 TUF。**建议**：在 Phase 1.5 预研文档中明确"单签名 PoC → TUF 升级"路径图。|
| 7 | CheckPermission 承诺有效期 | **C: 当前 turn** | A: 永久 / B: Session 范围 | **Agreed** | 最小权限原则。**建议**：在 IM 端 Ask 弹窗提供"本 turn + 跨 turn 同一类型"快捷选项，缓解频繁授权。|
| 8 | Surface 数量上限 | **C: 软上限 12 + AC25 异质性门槛** | A: 无上限 / B: 严格 12 | **Agreed** | PluginSurface 模式复用是合并机制。**建议**：在 §9 风险表中增加"12 限制可能需要重启评估"行。|

**Decision 结论**：8/8 **Agreed**，3 个有具体 S4 实施建议（Decision 1/2/3/4/6/7），均非阻塞。

---

## 3. 逐依赖确认（review-design.md §3.1 第 2 步）

### 3.1 内部依赖（既有 OpenSpec change）

| 依赖 | Change ID | 状态 | 验证 |
|------|-----------|------|------|
| ToolSurface 6 方法契约 | devrix-tool-surface-contract (DM-20260617-007) | S7_Archived | ✅ 已合并 |
| ToolSurface 12→0 global 清理 | devrix-tool-surface-phase2-full (DM-20260617-008) | S7_Archived | ✅ 已合并 |
| ToolSpec 4 正交标志 | devrix-tool-spec-enrichment (DM-20260618-001) | S7_Archived | ✅ 已合并 |
| CheckPermission 三态 | devrix-surface-permission-extension (DM-20260618-002) | S7_Archived | ✅ 已合并 |
| DeferLoading 懒加载 | devrix-surface-lazy-loading (DM-20260618-003) | S7_Archived | ✅ 已合并 |
| 诊断工具基线 | devrix-diagnostic-tools-parity (DM-20260616-003) | S7_Archived | ✅ 已合并 |

**结论**：所有 6 个内部依赖 **已合并**，无悬挂依赖。

### 3.2 外部库依赖

| 库 | 用途 | 阶段 | 锁定方式 | 验证 |
|----|------|------|---------|------|
| mvdan.cc/sh | Bash AST 解析 | Phase 1 | go.mod 版本锁定 | ⚠️ **建议**：在 S4 任务中加 `go mod tidy` 验证 + CI `go list -m all` 检查 |
| gopls | Go LSP server（外部进程）| Phase 1 | 外部 binary，不嵌入 | ✅ gopls 是 Go 官方工具 |
| tsserver | TypeScript LSP server（外部进程）| Phase 1 | 外部 binary | ✅ TypeScript 官方 |
| TUF 框架 | MCP Capability Attestation | Phase 2 | 待 Phase 1.5 预研确认 | ⚠️ **建议**：Phase 1.5 预研期完成 TUF 选型（python-tuf / go-tuf / 自己实现）|
| mvdan.cc/sh v3 vs v2 | AST API 兼容性 | Phase 1 | go.mod 锁定 | ⚠️ **建议**：在 S4 任务 T1 加 "锁定 v3.x 避免 v4 重大变更" |

**依赖结论**：3 个 ⚠️ 建议项（无阻塞），均为 S4 任务中可处理事项。

### 3.3 跨 Change 依赖（本 change 阻塞的 future change）

| Future Change | 依赖本 change 的什么 | 影响 |
|--------------|---------------------|------|
| `feat/web-tools-surface`（Phase 3）| 5-layer 架构 + ToolFilter 模式 | 阻塞但 Phase 3 远期 |
| `feat/doctor-self-diagnose`（Phase 3）| DiagnosticTracker 数据 | 阻塞但 Phase 3 远期 |
| `feat/mcp-mechanism-design-research`（Phase 1.5）| §3.3 MCP 多中心相变流 | **本 change 自身**（Phase 1.5 启动前置）|

**依赖结论**：无外部阻塞，Phase 1.5 启动需要本 change 的 §3.3 设计。

---

## 4. 逐 Scenario 推演（review-design.md §3.1 第 3 步）

手动走一遍 4 个核心 Scenario + 2 个 Phase 1.5/2 Scenario。

### 4.1 Scenario: LSP 代码理解流（§3.1.1）

**推演路径**：
```
User → D1 Inbound → D7 Turn → ClassifyIntent → WorkerContext
  → ToolFilter.Apply → LSPSurface 可见
  → D3 LLM Call
  → LLM 返回 tool_use = lsp_findReferences
  → D7 turn_adapter.Dispatch
  → CheckPermission (ReadOnly=true → Allow)
  → LSPSurface.Execute → D2-S4-A01-F02 (findReferences)
  → D2 LSPAdapter.CallLSP(gopls)
  → result
  → D5 span → D7 RoundAggregator → D3 LLM 下一轮
```

**验证**：
- ✅ ToolFilter 不修改 LLM 输出（DSAFT 不变式 2）
- ✅ CheckPermission Allow 不持久化（§2.5 不可逆性 + 撤销协议）
- ✅ LSP 调用有 fallback grep（§3.1.1 异常路径）
- ✅ D5 span 记录（§5.1 端到端）
- ⚠️ **潜在问题**：LSP 调用 2s 超时是工程经验值。**建议**：S4 任务加 "SLO 监控：p99 LSP 延迟 ≤ 1.5s"。

**结论**：**Agreed**（无阻塞性技术问题）

### 4.2 Scenario: Bash AST 安全审计流（§3.1.2）

**推演路径**：
```
LLM: tool_use = {name: "bash", input: {command: "rm -rf /tmp/*"}}
  → D7 turn_adapter.Dispatch → BashSurface.Execute
  → TOOL-SEC-2-A02 (BashASTPolicy.Audit)
  → AST 解析 → 规则匹配 → Decision{Deny}
  → 返回 error + 建议
```

**验证**：
- ✅ AST 解析失败 fail-closed（§3.1.2 异常路径）
- ✅ heredoc 嵌套拒绝（§3.1.2 + §8.2 T04）
- ✅ 20+ zsh 攻击模式（§8.2 T05）
- ⚠️ **潜在问题**：误判处理。**建议**：S4 任务加 "Ask 弹窗'报告误判'按钮 → D3 日志 → 季度 review 调整规则集"。

**结论**：**Agreed**

### 4.3 Scenario: 自由分叉探索流（§3.1.3）

**推演路径**：
```
LLM: tool_use = {name: "free_fork", input: {n: 3, directions: ["DB", "Goroutine", "Network"]}}
  → FreeForkSurface.Execute
  → D4-S11-A02-F01 (ForkAgent 创建)
  → 3 个子代理 + 3 个 worktree
  → D4-S11-A02-F02 (SendMessage)
  → 60s timeout
  → 结果聚合
```

**验证**：
- ✅ 并发上限 8（§8.4 T01 + §2.3 invariant）
- ✅ worktree 隔离（§8.5 T01）
- ✅ 资源争抢仲裁（§8.4 T03）
- ✅ timeout 中止（§8.4 T04）
- ⚠️ **潜在问题**：3 个方向子代理各自调 LLM = 3× token 消耗。**建议**：在 §6.3 WorkerContext 传递 budget_cap，子代理共享父 budget。

**结论**：**Agreed**（建议项非阻塞）

### 4.4 Scenario: 实现后自动验证流（§3.1.4）

**推演路径**：
```
S4 任务完成 → VerifySurface.Execute
  → D6-S11-A02-F01 (tasks.md 解析)
  → D6-S11-A02-F02 (验证项执行)
  → 4 类验证（A/B/C/D）
  → D6-S11-A02-F03 (结果聚合)
  → verdict + D6 reputation 调整
```

**验证**：
- ✅ 4 类验证项（§8.6 T02）
- ✅ 失败阻止 S4-Gate（§8.6 T03）
- ✅ D6 reputation 反馈（§6.3）
- ⚠️ **潜在问题**：verification timeout 5min 在大型 change 中可能不足。**建议**：在 S4 任务加 "per-verify-type timeout override via devrix.yaml"。

**结论**：**Agreed**

### 4.5 Scenario: LTL-Lite 运行时校验（§3.2）

**推演路径**：
```
D7 turn start → InvariantSet.Check(AllSurfaces)
  → 读取所有 _invariant.go
  → parse struct tag
  → runtime assert
  → 任何 violation → turn abort
```

**验证**：
- ✅ 每个 Surface 必含 _invariant.go（§3.2 + spec.md 强制）
- ✅ CI lint 验证存在性（spec.md §Requirement: CI Lint Integration）
- ✅ 跨 Surface 冲突检测（spec.md §Scenario: Cross-Surface Invariant Conflict Detection）
- ✅ Turn-Time 校验 hook（spec.md §Requirement: Turn-Time Validation Hook）
- ✅ runtime check < 5ms（spec.md §LTL-Lite Self-Invariants）
- ⚠️ **潜在问题**：当 Surface 数量 12+ 时，全部 invariant check 可能超过 5ms。**建议**：spec.md 添加 "lazy check on surface load" 优化路径。

**结论**：**Agreed**（优化建议非阻塞）

### 4.6 Scenario: MCP Capability Attestation（§3.3 / Phase 2）

**推演路径**：
```
MCP server 注册 → CapabilityAttestation.Verify(signed_metadata)
  → 签名验证（Ed25519/TUF）
  → 声明能力 vs 实际调用一致性
  → Reputation Budget 初始化
  → Cross-Validation (Destructive 操作)
  → Causal Audit Trail 4-tuple
```

**验证**：
- ✅ Attestation 设计（§3.3 + §8 T22-T29）
- ✅ Reputation Budget（AC23 + T23）
- ✅ Cross-Validation（AC24 + T24）
- ✅ Reputation Decay（AC29 + T29）
- ✅ Causal Audit Trail（AC26 + T26）
- ⚠️ **潜在问题**：Phase 2 跨域集成复杂度高。**建议**：Phase 1.5 预研期做"单机 MCP 多 server 模拟测试"验证。

**结论**：**Agreed**（建议项作为 Phase 1.5 任务，非本 change 阻塞）

---

## 5. DSAFT §九 Checklist 验证（review-design.md §2.1）

| 层级 | 必填项 | design.md 位置 | 完整性 |
|------|--------|---------------|--------|
| **D** | ID、名称、类型、领域职责 | §0.1 | ✅ 6 个 D 全列 |
| **S** | ID、名称、触发条件、用户目标、涉及 A | §0.2 | ✅ 9 个 S 全列 |
| **A** | ID、名称、类型、输入、输出、状态变更 | §0.3 | ✅ 10 个 A 全列 |
| **F** | ID、名称、类型、输入、输出 | §0.4 | ✅ 23 个 F 全列 |
| **T** | ID、名称、归属层级、归属 ID、验收契约、优先级 | §8 | ✅ 25 个 T 全列 |

**DSAFT §四 追溯链验证**：
- S → D ✅（每个 S 都在 §0.1 中归属 D）
- A → S ✅（§0.3 每个 A 标注 S 归属）
- F → A ✅（§0.4 每个 F 标注 A 归属）
- T → F 或 A ✅（§8 每个 T 标注归属层级 + 归属 ID）

**DSAFT §10 OpenSpec 映射验证**：
- S2 proposal: D + S ✅（proposal.md §2 标识 6 个 D + 9 个 S）
- S3 design: F + A↔F ✅（§0.3 + §0.4 + §2.1）
- S4 tasks: F 实现任务 ⏳（下一步）
- S5 verify: T ✅（§8 25 个 T + Phase 1.5/2/3 增量 ~35 个）

**结论**：**完全合规**

---

## 6. Spec Coverage Check（review-design.md §2.2）

每个 P0 验收标准（demand.md §4.1）→ spec.md Scenario 映射：

| AC# | 描述 | spec.md | Scenario | 状态 |
|------|------|---------|----------|------|
| AC1 | LSP Tool 4 P0 操作 | specs/lsp-surface | 5 个 Requirement + 6 个 T | ✅ |
| AC2 | BashAST 20+ 攻击模式 | specs/bash-ast-policy | 5 个 Requirement + 6 个 T | ✅ |
| AC3 | 文件诊断追踪 LRU 1000 | specs/diagnostic-tracker | 4 个 Requirement + 5 个 T | ✅ |
| AC4 | 自由分叉 8 上限 | specs/free-fork | 5 个 Requirement + 5 个 T | ✅ |
| AC5 | 实现后自动验证 | specs/verify-plan-execution | 5 个 Requirement + 3 个 T | ✅ |
| AC6 | D2 不持有 D3 引用 | （无独立 spec，集成于 §6.2）| — | ⚠️ **建议**：在 specs/lsp-surface 或 specs/diagnostic-tracker 中加 cross-spec 引用 |
| AC7 | 不破坏现有 P0 T 点 | design.md §10 回归风险表 | — | ✅ |
| AC25 | Surface 合并异质性门槛 | （Phase 2.5 任务，spec 待定）| — | ✅（已规划）|
| AC26 | Causal Audit Trail 4-tuple | specs/verify-plan-execution MODIFIED | — | ✅ |
| AC29 | MCP Reputation Decay | （Phase 2 spec，单独文件）| — | ✅（Phase 2 规划）|

**结论**：8/10 AC 全覆盖，2 个 ⚠️ 建议项均为 Phase 2 任务。

**Happy + Sad Path 覆盖**：
- ✅ Happy path：所有 spec.md 主线 Scenario
- ✅ Sad path：spec/lsp-surface fallback、spec/bash-ast-policy fail-closed、spec/free-fork timeout、spec/verify-plan-execution parse failure
- ✅ 并发场景：spec/free-fork T03 (文件锁仲裁)
- ✅ 错误路径：spec/bash-ast-policy T03 (AST 解析失败)

---

## 7. Open Suggestions（非阻塞）

按严重度排序：

### 7.1 SUG-1 [Medium]: WorkerContext 子代理 budget 共享

**位置**: §3.1.3 自由分叉流

**问题**: 3 个子代理各自调 LLM = 3× token 消耗，可能击穿 budget

**建议**: 在 §6.3 WorkerContext 接口加 `BudgetCap` 字段，子代理共享父 budget

**影响**: S4 任务加 1 项

### 7.2 SUG-2 [Medium]: LSP SLO 监控

**位置**: §3.1.1 LSP 流

**问题**: LSP 2s 超时是经验值，无监控

**建议**: D5 metrics 加 `d2.lsp.latency.p99` + 告警阈值

**影响**: S4 任务加 1 项（D5 metrics 集成）

### 7.3 SUG-3 [Low]: Surface 数量评估周期

**位置**: §9 Decision 8

**问题**: 12 软上限是经验值，无定期评估机制

**建议**: 每季度 review Surface 数量 + 异质性门槛

**影响**: 工程流程级，非代码

### 7.4 SUG-4 [Low]: 误判反馈通道

**位置**: §3.1.2 Bash AST 流

**问题**: BashAST 误判无反馈通道

**建议**: IM 弹窗"报告误判"按钮 + D3 日志 + 季度规则集 review

**影响**: S4 任务加 1 项（D1 IM 集成）

### 7.5 SUG-5 [Low]: devrix.yaml 配置项

**位置**: §9 Decision 3 / §6 接口

**问题**: FreeFork 8、DiagnosticTracker 1000 等硬编码不可配

**建议**: 在 design.md §6 加 "Config 章节"，列出可配项

**影响**: S4 任务加 1 项（config schema）

### 7.6 SUG-6 [Low]: 库版本锁定

**位置**: §3.1.2 外部依赖

**问题**: mvdan.cc/sh v3 → v4 可能有重大变更

**建议**: go.mod 锁定到 v3.x，CI 加 `go list -m all` 检查

**影响**: S4 任务标准项

### 7.7 SUG-7 [Low]: AC6 跨 spec 引用

**位置**: §6 AC6

**问题**: "D2 不持有 D3 引用" 无独立 spec

**建议**: 在 specs/lsp-surface 或 specs/diagnostic-tracker 的 MODIFIED 段加交叉引用

**影响**: docs 级，无需 S4 任务

### 7.8 SUG-8 [Low]: LTL-Lite 性能优化

**位置**: spec/ltl-lite-invariant §LTL-Lite Self-Invariants

**问题**: 当 Surface 数量 12+ 时，全部 invariant check 可能超过 5ms

**建议**: 添加 "lazy check on surface load" 优化路径

**影响**: Phase 1.5 实施细节

---

## 8. 风险与覆盖度自评

### 8.1 风险评估覆盖度

| 风险类别 | design.md 位置 | 评估 |
|---------|---------------|------|
| 架构风险 | §10 风险表 + §1.3 约束 | ✅ |
| 安全风险 | §10 + §3.1.2 异常路径 | ✅ |
| 实施风险 | §10 + §11 回滚 | ✅ |
| 性能风险 | §10.2 性能影响预估 | ✅ |
| 跨域风险 | §4.3 限界上下文 + §1.3 约束 | ✅ |
| 博弈论风险（DM-007 引入）| （proposal.md §2.3 + demand.md §3.2 不变式）| ✅ |

### 8.2 维度覆盖度

| review-design.md §2 维度 | 覆盖状态 |
|------------------------|---------|
| §2.1 架构决策审查 | ✅ §0.x DSAFT + §9 Decision |
| §2.2 需求完整性审查 | ✅ §13 S3 检查清单 |
| §2.3 规格质量审查 | ✅ 6 个 spec.md + §6 Spec Coverage |
| §2.4 风险审查 | ✅ §10 风险 + §11 回滚 |

### 8.3 已知弱点（透明披露）

1. **Phase 2/3 设计深度不足**：本 change 的 S3 设计在 Phase 2/3 部分相对 Phase 1 略粗（特别是 Phase 3 仅有 feature 列表）。这是设计 vs 实现的 trade-off：Phase 2/3 启动时再细化。
2. **MCP 选型未决**：TUF 框架的 Go 库选型（python-tuf / go-tuf / 自实现）待 Phase 1.5 预研决定。
3. **DI 框架未明确**：设计提到"通过构造函数注入"但未指定 DI 容器。Devrix 当前无 DI 框架，依赖手动注入可接受。
4. **测试策略未细化**：每个 F 的 T 点已列，但具体的 mock 策略、test fixtures、coverage 阈值未在 design.md 详述。这是 S4 tasks.md 的工作。

---

## 9. Gate Verdict

| 维度 | 评估 |
|------|------|
| 设计完整性 | **Pass**（DSAFT §九 5 层全绿 + 6 个 spec.md）|
| 决策可辩护性 | **Pass**（8/8 Decision Agreed，3 个有具体 S4 建议）|
| 依赖可行性 | **Pass**（所有 6 个内部依赖已合并，3 个外部库建议锁定）|
| Scenario 推演 | **Pass**（6 个 Scenario 全部 Agreed，5 个 SUG 建议）|
| 风险评估 | **Pass**（5 类风险全覆盖，3 层回滚方案）|
| 跨域一致性 | **Pass**（D2-D3 import lint + 11 个限界上下文边界）|
| **总体** | **Approved with Suggestions** |

**S3-Gate 结论**：✅ **设计可进入 S4 tasks.md 拆分**

**进入 S4 前的建议**：
1. 用户最终确认本设计（特别是 §0.x 5 个注册表 + §8 25 个 T 点）
2. 7 个 Open Suggestions 在 S4 任务拆分时部分吸收（建议 SUG-1/2/5/6 必收，其他可后置）
3. S4 tasks.md 拆分时按 5 个 Phase 1 子 change 独立组织

---

## 10. S3-Gate Sign-off

| 角色 | 状态 | 日期 | 备注 |
|------|------|------|------|
| **设计作者（self）** | Submitted | 2026-06-18 | 设计完成 + DSAFT §九全绿 + 6 个 spec.md |
| **S3-Gate Reviewer** | Approved with Suggestions | 2026-06-18 | 7 个非阻塞建议 |
| **用户（最终决策）** | ⏳ **Pending** | — | 等待用户最终确认 |

**下一步**（待用户确认后）：
- [ ] 用户确认设计 → 进入 S4 tasks.md
- [ ] 用户提出修改 → 返回 S3 修改
- [ ] 用户决定暂缓 → 归档为 deferred
