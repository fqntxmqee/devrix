# Proposal: 诊断工具能力 E2E 可达性 — 13 项 wiring 闭环

**Change ID:** devrix-diagnostic-tools-wiring
**Demand ID:** DM-20260617-002
**Status:** S2_Design
**关联 upstream:** DM-20260616-003 (devrix-diagnostic-tools-parity, ACCEPTED 2026-06-17)

---

> **能力别名前缀 (Capability Aliases)**
>
> 本 change **复用** DM-016 已确立的 14 个 DSAFT Activity 节点（详见 `openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/spec.md` §0）。G1-G6 / A1-A7 保留为需求侧 alias，便于对照 `docs/reference/clawcode-diagnostic-tools-analysis.md`。

## 1. Background

DM-20260616-003 交付了 13 项诊断/开发辅助能力的 library 实现 + 单测，全部 P0 T 测试点 PASS，被 S5 验收 ACCEPTED。然而 acceptance-report §5 Cross-Domain Wiring 列出的 6 项 wiring 中，仅有 2 项（G2 sandboxast interface + G3 task_manager publish）真实生效，其余 4 项（doctor / freefork / tracker / errorclass）只有 package 实现，**bootstrap 阶段零调用方**。这意味着 13 项能力中只有 G1 LSP（已注册为 LLM tool，但默认 disabled）能通过 LLM 路径触达。

E2E IM 可达率仅 7.7%（1/13），与 acceptance-report "全部完成" 的口径存在 gap。本次修复聚焦 **接入层（wiring layer）**，不动 DM-016 已交付的 library 行为。

## 2. Problem Statement

### 2.1 现状（13 项 wiring 真实情况）

| Activity | Alias | library | bootstrap | LLM tool | IM 可达 |
|----------|-------|---------|-----------|----------|---------|
| D2-S4-A01 | G1 LSP | ✅ | ✅ (default disabled) | ✅ | ⚠️ 需 devrix.yaml |
| TOOL-SEC-2-A02 | G2 Bash AST | ✅ | ⚠️ interface only | n/a | ⚠️ regex only |
| D4-S12-A03 | G3 Notify | ✅ | ⚠️ publish only | n/a | ❌ |
| D6-S11-A02 | G4 Verifier | ✅ | ❌ | ❌ | ❌ |
| D4-S11-A02 + D4-S13-A02 | G5 FreeFork | ✅ | ❌ | ❌ | ❌ |
| D5-S23-A02 | G6 Tracker | ✅ | ❌ | ❌ | ❌ |
| D5-S23-A03 | A1 Doctor | ✅ | ❌ | ❌ | ❌ |
| D5-S24-A02 | A2 DebugFilter | ✅ | ⚠️ CLI flag only | n/a | ⚠️ flag 启动 |
| D1-S2-A02 | A3 Transcript | ✅ | ❌ | n/a | ❌ |
| D5-S23-A04 | A4 FaultInject | ✅ | n/a (testbuild) | n/a | n/a (本期 P2) |
| D2-S6-A03 | A5 WindowAnalyzer | ✅ | ❌ | ❌ | ❌ |
| D3-S3-A02 | A6 ErrorClassifier | ✅ | ❌ | n/a | ❌ |
| D2-S6-A02 | A7 ShortStack | ✅ | ⚠️ 包装器 only | n/a | ❌ |

**结论**：13 项中 12 项的"可达"是 library-级，不是运行期可达。

### 2.2 用户痛感

- 通过飞书 IM 验证能力**没有可观测信号**：用户发指令，LLM 调用不到对应能力，IM 回复要么 "tool not found" 要么 LLM 自由发挥。
- acceptance-report 自报"全 PASS"，但实际 E2E IM 验证覆盖率仅 7.7%，**S5 验收口径（单测 PASS）与 S6 归档口径（用户可触达）不一致**。
- 错误响应缺诊断信息（无 class 标签、无短栈），用户只能 grep 日志排查。

## 3. Proposed Solution

### 3.1 方案概述

把 13 项能力的接入路径分为 **6 个 Level**，按"对运行期的影响半径"组织：

| Level | 类型 | 数量 | 接入方式 |
|-------|------|------|----------|
| L1 | LLM tool 暴露 | 4 | `toolrunner.PluginRunner` 注册 |
| L2 | CLI slash command | 2 | `cli/<name>.go` 子命令 + `adapters/cli.go` 路由 |
| L3 | 错误路径接入 | 3 | `errors.WithShortStack` + `errorclass.InjectClassification` |
| L4 | AST 注入 + DebugFilter | 2 | `bootstrap.NewCommandPolicy` 注入 + `InstallSlogBridge` 包装 |
| L5 | Transcript 持久化 | 1 | `capture.SessionStore.OnSessionClose` 钩子 |
| L6 | Notify consume | 1 | `output_assembler` drain → `<task_notifications>` block |
| **合计** | | **13** | |

**复用策略**：所有 wiring 走 `internal/shared/` 横切接口 + `internal/bootstrap/` 启动注入；**不修改 DM-016 任何 library 文件**。

### 3.2 关键决策

#### Decision: wiring 层用 LLM tool 还是 CLI slash command？

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| **A. 全部 LLM tool** | 模型可自主调用 | 需 prompt 调优；某些场景（/doctor）用户期待直接触发 |
| **B. 全部 CLI slash** | 用户直接触发 | 模型看不到，无法自主调用 |
| **C. 混合（推荐）** | 各取所长 | 需明确分类 |

**选择：** C
- **G4/G5/G6** 走 LLM tool — 模型在编辑/计划/排错时自主调用
- **A1/A5** 走 CLI slash + IM 路由 — 用户在 IM 主动触发（/doctor /context）
- **A2/A3/A4/A6/A7** 走 bootstrap 自动接入 — 用户不直接触发，运行期可观察
- **G2/G3** 走 bootstrap 自动注入 — 静默生效

**理由**：匹配 clawcode 的 `<task_notifications>` / `/doctor` 触发模式，且不破坏现有 IM 消息路由。

#### Decision: G5 FreeFork 是否暴露为 LLM tool？

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 暴露为 `free_fork` tool | LLM 可即兴分叉 | 增加 tool surface area |
| B. 仅走 D4 delegate_explore | 已有路径 | DAG 拓扑限制 |
| **C. 暴露为 LLM tool (推荐)** | 与 DM-016 library 1:1 对应 | 需防止滥用 |

**选择：** C
- `free_fork` tool 接受 `[{prompt, isolation: "worktree"|"none"}, ...]` 参数，**默认 `worktree`**
- LLM 调用需满足 prompt 提示词约束（"想从多个方向并行调查时"）
- 失败回滚走 library 自带逻辑（DM-016 已实现）

**理由**：与 DM-016 library 实现 1:1 暴露，避免引入新的 fork 抽象层。

#### Decision: G6 Tracker 异步 tick vs 同步？

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 同步（在 edit_file 后立即跑 linter） | 简单 | 编辑延迟 +5s |
| **B. 异步 tick（推荐）** | edit 主路径不阻塞 | 需新 goroutine + 生命周期管理 |
| C. lazy（下一个 LLM 请求前跑） | 零常驻 | 下次 IM 回复延迟 |

**选择：** B
- `tracker.Tracker` 在 `bootstrap` 阶段启动一个 goroutine，每 1s tick 一次扫描待办 list
- `edit_file` 完成后仅 enqueue（O(1)），不阻塞
- tick 任务受 ctx 控制，shutdown 时优雅退出
- 输出通过 `<file_diagnostics>` reminder block 注入下一次 LLM 请求

**理由**：与 clawcode `diagnosticTracking.ts` 的 debounce 策略对齐，editor UX 优先。

#### Decision: G3 Notify consume 是否修改 output_assembler？

**选项：**
| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 在 output_assembler 注入 | 唯一入口 | 修改 contextengine 主路径 |
| B. 在 D1 outbound message 渲染前注入 | 解耦 | 需在 D1 引用 contextengine 类型 |
| **C. 通过 prompt assembler 注入到 system prompt (推荐)** | 不动 message 渲染 | system prompt 长度增加 |

**选择：** C
- 在 `prepare/prompt/assembler.go` 渲染 system prompt 时，drain `notify.GlobalBus()` 并把 `<task_notifications>` 块追加到 reminder 段
- 与现有 `<reminder>` 块一致；LLM 已熟悉该语法
- 不动 D1 outbound 渲染

**理由**：最小侵入 + 复用现有 reminder 注入路径。

### 3.3 不修改的边界

- DM-016 已交付的 21 个 library 文件（library 行为是 SoT；本次只做接入层）
- D2 QueryLoop / D7 Turn 编排主路径
- D3 LLM Gateway 现有熔断/重试/超时逻辑
- D1 Communication 现有协议层
- A4 FaultInject 的 build-tag 行为（AC13 锁定 P2）

## 4. Success Metrics

| Metric | Baseline | Target |
|--------|----------|--------|
| 13 项能力 IM 可达率 | 7.7% (1/13) | ≥ 92% (12/13) |
| 飞书 IM 触发 G1 LSP tool 调用成功率 | n/a | ≥ 95% (cfg enabled) |
| G6 Tracker `<file_diagnostics>` reminder 注入延迟 | n/a | P95 < 5s |
| A6 ErrorClassifier 错误响应含 class 标签比例 | 0% | 100% |
| A7 ShortStack 错误响应帧数压缩比例 | n/a | 平均 ≤ 5 帧 |
| A1 /doctor 7 check 在 IM 触发完整率 | n/a | 100% |
| 13 项 library 单测通过率 | 100% | 100%（不破坏 baseline） |
| 新增 wiring 单测覆盖率 | n/a | ≥ 80% |
| P0 T 层 PASS 率 | 100% | 100% |
| layer-lint (strict gate) | pass | pass |

## 5. Implementation Plan

### 5.1 Phase 划分（按 Layer 组织）

**Phase 1: 错误路径接入**（L3，最独立）
- A6 ErrorClassifier → `llmgateway/dispatch/invoke.go` 注入
- A7 ShortStack → sandbox + agent engine 错误包装
- 价值：每次错误响应立刻可见

**Phase 2: AST 注入 + DebugFilter**（L4）
- G2 Bash AST → `bootstrap.NewCommandPolicy` 注入
- A2 DebugFilter → `InstallSlogBridge` 包装
- 价值：安全 + 可观测性

**Phase 3: LLM tool 暴露**（L1，最大 surface）
- G4 Verifier tool + bootstrap 注册
- G5 FreeFork tool + bootstrap 注册
- G6 Tracker tool + bootstrap 注册（含 tick goroutine）
- A5 WindowAnalyzer tool（optional，CLI 已覆盖）

**Phase 4: CLI slash + IM 路由**（L2）
- A1 /doctor CLI 子命令 + `adapters/cli.go` 路由
- A5 /context analyze CLI 子命令 + `adapters/cli.go` 路由

**Phase 5: Transcript + Notify consume**（L5 + L6）
- A3 Transcript → `capture.SessionStore.OnSessionClose` 钩子
- G3 Notify consume → `prepare/prompt/assembler.go` drain → `<task_notifications>`

### 5.2 文件清单（高层）

**新增（约 14 个文件）：**
- `internal/layers/contextengine/enforce/toolrunner/{verify,freefork,tracker}_tool.go` + 各自 `_register.go`
- `internal/layers/contextengine/enforce/toolrunner/windowanalyzer_tool.go`（optional）
- `internal/layers/observability/diagnose/tracker/wire.go`
- `internal/cli/doctor/doctor.go` + `internal/cli/context_analyze/context_analyze.go`
- `internal/layers/communication/capture/transcript/wire.go`
- `internal/bootstrap/observability.go`（新增）

**修改（约 5 个文件）：**
- `internal/bootstrap/context_engine_builder.go`（tool 注册）
- `internal/layers/communication/channel/adapters/cli.go`（slash 路由）
- `internal/layers/communication/capture/session_store.go`（close 钩子）
- `internal/layers/contextengine/prepare/prompt/assembler.go`（notify drain）
- `internal/layers/llmgateway/dispatch/invoke.go`（classify 注入）

详细 File Manifest 见 S3 design.md §5。

### 5.3 T 层测试点登记

本次 change **不新增 T 节点**，沿用 DM-016 已登记的 22 个 T 编号。S2 阶段把状态从 `IMPLEMENTED` 保持为 `IMPLEMENTED`（library 行为已在 DM-016 验收），新增 wiring 单测通过现有 T 编号做集成断言。

T 编号清单见 `.openspec.yaml` `t_points` 字段。

## 6. Risks & Mitigations

| Risk | 影响 | 概率 | 缓解 |
|------|------|------|------|
| G6 Tracker tick goroutine 泄漏 | 内存增长 | 中 | ctx 控制 + WaitGroup + shutdown hook |
| G5 FreeFork tool 被滥用（LLM 频繁调用） | 子 agent 爆炸 | 中 | `free_fork` tool 默认 max=3，warning > 5 |
| G3 Notify consume 注入 reminder 干扰 LLM 输出 | 幻觉率上升 | 低 | `<task_notifications>` XML 严格格式 + reminder 段不进入 system prompt |
| G2 Bash AST 注入改变 CommandPolicy 默认行为 | 误杀合法命令 | 中 | AST 仅对 regex 已通过的命令二次审计；白名单覆盖常见命令 |
| AC1/AC2 /doctor /context 在 IM 无 current session | 输出为空 | 低 | CLI 子命令接受 `--session-id` flag + 默认取最新 |
| A3 Transcript 与 capture.FileSessionStore 双写冲突 | 重复存储 | 中 | 接口隔离（transcript 仅写 jsonl，session_store 负责 metadata） |
| A6 ErrorClassify 在 ctx cancel 场景下丢失 | 错误无分类 | 低 | 注入后立即缓存到 errors.Is() 包装 |
| 涉及 Bash AST、ErrorClassify 安全敏感变更 | 安全风险 | 高 | AC17：必须经 verify-security skill 闸门 |

## 7. Out of Scope

- **不修改** DM-20260616-003 已交付的 library 代码（library 行为是 SoT）
- **不实现** A4 FaultInject 的 IM 注入（仍 build-tag 隔离）
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端
- **不覆盖** 跨 IM 平台差异（飞书 / Slack / CLI 行为对齐先不动）
- **不重构** `capture.FileSessionStore` 与 `transcript.Writer` 关系
- **不实现** clawcode `_claude_fs_right:` 等私有协议

## 8. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/`（13 项 library）
- 上游 change：`openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/`（QueryLoop 退役）
- 上游 change：`openspec/changes/devrix-d7-loop-first-routing/`（D7 ingress）
- 上游 change：`openspec/changes/devrix-d7-uncertainty-gaps/`（D3 错误响应路径）
- DSAFT 方法论：`openspec/specs/project/master.md` + `docs/methodology/dsaft-methodology.md`
- 域归档：`openspec/specs/d1-communication/` `d2-context-engine/` `d3-llm-gateway/` `d4-multi-agent/` `d5-observability/` `d6-evolution/` `tool-security/`
- T 注册表：`openspec/t-registry.md` + 各域 `openspec/specs/d{N}-*/t-registry.md`

## 9. 检查清单（S2 完成确认）

- [x] `.openspec.yaml` 所有字段已填写（含 dsaft_scenarios / dsaft_activities / t_points / version_scope / capability_aliases）
- [x] `dsaft_scenarios` 标注 11 个 S 节点（D1-S2/D2-S4/D2-S6/D3-S3/D4-S11/D4-S12/D4-S13/D5-S23/D5-S24/D6-S11 + TOOL-SEC-2）
- [x] `dsaft_activities` 标注 14 个 Activity 节点（复用 DM-016）
- [x] `proposal.md` 包含方案对比与风险评估（§3.2 + §6）
- [x] `demand.md` → `proposal.md` 追溯链完整（DM-017-002 已分配）
- [x] Out of Scope 已声明（§7）
- [x] 重大决策已记录 Decision 节（§3.2 共 4 项）
- [x] Success Metrics 已声明量化目标（§4）