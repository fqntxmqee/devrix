---
demand-id: DM-20260617-002
title: 诊断工具能力 E2E 可达性 — 13 项 wiring 闭环
priority: P0
status: S1_Proposal
dsaft_domain: multi-domain
created: 2026-06-17
parent_doc: openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/
---

# Demand: 诊断工具能力 E2E 可达性 — 13 项 wiring 闭环

> **复用现有 DSAFT Activity 节点**
>
> 本 change **不新增** Activity 节点；DM-20260616-003 已确立的 14 个 Activity（详见 `openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/spec.md` §0 能力别名前缀）作为权威 ID。G1-G6 / A1-A7 保留为需求侧 alias。本 change 的全部工作围绕这些已有节点的 **Wiring 层（接入层）** 展开。

## 1. 背景

DM-20260616-003（`devrix-diagnostic-tools-parity`）已于 2026-06-17 ACCEPTED，交付 13 项诊断/开发辅助能力的 library 实现 + 单测。然而，经现场核查（详见 `openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/acceptance-report.md` §5 Cross-Domain Wiring 与实际 `internal/bootstrap/` 调用图对比），13 项能力中：

- **1 项**（G1 LSP Tool）注册为 LLM tool，但默认 disabled
- **2 项**（G2 Bash AST、G3 Task Notify）声明接口但部分 wiring 未注入
- **10 项**（G4 Verifier、G5 FreeFork、G6 Tracker、A1 Doctor、A2 DebugFilter、A3 Transcript、A4 FaultInject、A5 WindowAnalyzer、A6 ErrorClassifier、A7 ShortStack）零调用方，仅在单测中被覆盖

**结果**：通过飞书 IM 端到端验证（用户当前的核心诉求），仅 1/13 = 约 7.7% 的能力可触达。其余 12 项虽是 library-级可测，但用户/LLM/IM 路径完全无法抵达，DM-20260616-03 的实际可验收覆盖率不达预期。

## 2. 问题陈述

### 2.1 现状盘点（13 项 wiring 真实情况）

| Activity | Alias | 能力 | library | bootstrap 调用 | LLM tool | IM 可达 |
|----------|-------|------|---------|---------------|----------|---------|
| D2-S4-A01-ToolRegister | G1 | LSP Tool | ✅ | ⚠️ 注册为 `lsp` tool，`cfg.Enabled=false` | ✅ | ⚠️ 需 devrix.yaml 启用 |
| TOOL-SEC-2-A02-ShellASTPolicy | G2 | Bash AST | ✅ | ⚠️ `ASTAnalyzer` 字段声明但 `NewCommandPolicy` 未注入实例 | n/a | ⚠️ 仅 regex denylist 生效 |
| D4-S12-A03-NotifyChild | G3 | Task Notify Bus | ✅ | ⚠️ publish 已接 (`task_manager.go:218`)，consume 侧无订阅 | n/a | ❌ `<task_notifications>` 不进 IM |
| D6-S11-A02-VerifyPlanExec | G4 | Plan Verifier | ✅ | ❌ 零调用方 | ❌ | ❌ |
| D4-S11-A02-ForkAgent + D4-S13-A02-IsolateWorktree | G5 | Free Fork | ✅ | ❌ 零调用方 | ❌ | ❌ |
| D5-S23-A02-TrackDiagnostics | G6 | File Diagnostic Tracker | ✅ | ❌ 零调用方 | ❌ | ❌ |
| D5-S23-A03-RunDoctor | A1 | /doctor | ✅ | ❌ 零调用方 | ❌ | ❌ |
| D5-S24-A02-ConfigureDebugFilter | A2 | Debug Filter | ✅ | ⚠️ 仅 `--debug=` CLI flag 可启 | n/a | ⚠️ 需 CLI flag |
| D1-S2-A02-PersistTranscript | A3 | Transcript | ✅ | ❌ 零调用方（与 `capture.NewFileSessionStore` 重叠但不同物） | n/a | ❌ |
| D5-S23-A04-FaultInject | A4 | Fault Inject | ✅ | n/a（仅 `//go:build testbuild`） | n/a | n/a |
| D2-S6-A03-AnalyzeWindow | A5 | Window Analyzer | ✅ | ❌ 零调用方 | ❌ | ❌ |
| D3-S3-A02-ErrorMapping | A6 | Error Classifier | ✅ | ❌ `Classify()` 零调用方 | n/a | ❌ 错误响应无分类标签 |
| D2-S6-A02-TruncateError | A7 | Short Stack | ✅ | ⚠️ `WithShortStack` 包装器存在但零调用方 | n/a | ❌ 错误响应无短栈 |

**Legend**: ✅ 已实现；⚠️ 部分接入；❌ 未接入；n/a 不适用

### 2.2 用户/Agent 痛感

- **「通过飞书 IM 验证能力」成为一句空话**：用户在群里发 `/doctor`、`查一下 LSP 定义`、`fork 一个子代理排查 X`，LLM 收到指令但调用不到对应能力。
- **DM-20260616-003 acceptance-report 自报"全 PASS"，但 E2E IM 覆盖率 7.7%**——S5 验收口径（仅 P0 T 单测通过）与 S6 归档口径（用户/LLM 实际可触达）存在 gap，acceptance-report 标题"全部完成"具有误导性。
- **错误响应缺诊断信息**：LLM 网关报错时，用户看不到 `[auth_required]` / `[rate_limit]` 分类标签，也看不到短栈（要在 IM 端排查得手动 grep 日志）。

### 2.3 修复目标

把 13 项能力的 E2E IM 可达率从 7.7% 提升至 ≥ 92%（12/13；A4 FaultInject 例外，仍 build-tag 隔离）。具体接入方式见 §5 范围。

## 3. 验收标准

### 3.1 P0（必须达成，否则不交付）

| ID | 标准 | 度量 |
|----|------|------|
| AC1 | **A1 /doctor** 通过飞书 IM `/doctor` 触发后，IM 回复含 7 项 check 的 status 表 | 单测覆盖 7 check happy/sad path；集成测通过 CLI adapter 触发并断言输出含 `PASS/FAIL/WARN` |
| AC2 | **A5 /context analyze** 通过飞书 IM `/context` 触发后，IM 回复含 5 类 token 拆分的 ASCII 进度条 | 单测覆盖 5 category；集成测通过 CLI adapter 触发并断言含 `system/messages/tools/thinking/reminders` 5 行 |
| AC3 | **G1 LSP Tool** 在 `devrix.yaml` 配置 `lsp.enabled=true` + servers 后，飞书发 "查 X 的定义" LLM 自动调用 `lsp` tool 返回结果 | 已有 LSP tool 单测 + 新增 config-driven bootstrap wiring 单测 |
| AC4 | **G4 Verifier** 通过 LLM tool `verify_plan_execution` 触发，扫描 tasks.md done 项并返回 Verified/Unverified/Skipped 统计 | toolrunner 注册 + 单测；集成测模拟 tasks.md 注入 done 项并断言输出格式 |
| AC5 | **G5 FreeFork** 通过 LLM tool `free_fork` 触发，批量分叉 N 个子代理 + 失败回滚 + worktree 隔离 | toolrunner 注册 + 单测；集成测模拟 factory failure 触发回滚路径 |
| AC6 | **G6 Tracker** `edit_file` 后 5s 内 IM 回复含 `<file_diagnostics>` reminder block（仅当有新错误） | 单测覆盖 snapshot/diff/LRU；集成测通过 context engine 模拟 edit + tracker tick |
| AC7 | **A6 ErrorClassifier** LLM 网关错误响应中含 `[class=AuthRequired\|RateLimit\|...]` 标签 | `DispatchInvoke` 单测；集成测 mock 401/429 验证 ctx 注入 |
| AC8 | **A7 ShortStack** LLM 网关 / Sandbox / Agent 错误响应中调用 `errors.WithShortStack` 过滤 runtime/testing/reflect 帧 | `WithShortStack` 单测；接入点（sandbox + llmgateway）错误路径单测 |
| AC9 | **A3 Transcript** session 结束后 `<session_dir>/<id>.jsonl` 存在；`ListSessions()` 按 mtime 倒序返回 | writer 单测；`capture.SessionStore` 接入点单测 |

### 3.2 P1（本期交付）

| ID | 标准 | 度量 |
|----|------|------|
| AC10 | **G2 Bash AST** `bootstrap.NewCommandPolicy` 注入 `sandboxast.PolicyAnalyzer`，heredoc body 内 `$(...)` / zmodload / process substitution / eval 被 AST 拦 | 已有 sandboxast 10 case + 新增 bootstrap 注入单测 + 集成测 4 类 AST finding |
| AC11 | **G3 Notify consume** Task 完成时 `<task_notifications>` XML 块追加到下一个 IM 回复 `<reminder>` 块 | notify consume hook 接入 `output_assembler.go`；集成测模拟 task_complete → 断言下一个 reply 含 block |
| AC12 | **A2 DebugFilter** `--debug=api,hooks` 启动时 DEBUG 级 entry 仅在 enabled 组件通过；非 DEBUG 级不受影响 | 已有 filter 9 case + 新增 `observability.InstallSlogBridge` 注入点单测 |

### 3.3 P2（本期不交付，仅需求锁定）

| ID | 标准 |
|----|------|
| AC13 | **A4 FaultInject** IM 注入接口（生产 no-op 不变；testbuild 仍仅单测覆盖） |
| AC14 | 13 项 wiring 在 `devrix.yaml` 提供统一 `diagnostics:` 配置节（含 lsp / debug filter / tracker LRU 容量 等） |

### 3.4 质量基线

| ID | 标准 |
|----|------|
| AC15 | 不修改 DM-20260616-003 已交付的 library 代码（library 行为不变，仅做接入） |
| AC16 | 跨域新增 import 不得引入新的依赖环；D2/D3/D4/D5 wiring 走 `internal/shared/` 横切接口 |
| AC17 | 涉及 Bash AST 注入、Error classification 等安全敏感 wiring，必须经 `verify-security` 闸门 |
| AC18 | 所有 wiring 单测覆盖率 ≥ 80%；新增 wiring 集成测通过 E2E IM 路径断言 |
| AC19 | P0 T 层 13/13 PASS（沿用 DM-016 已登记 T 编号，不新增 T 节点） |
| AC20 | acceptance-report §5 Cross-Domain Wiring 表 13/13 真实生效（与本次 fix 同步更新） |

## 4. 依赖与约束

| 类型 | 内容 |
|------|------|
| 上游（已完成） | DM-20260616-003 `devrix-diagnostic-tools-parity` — 提供 13 个 library package + 单测 baseline |
| 上游（已完成） | DM-20260616-002 `devrix-d7-loop-first-routing` — D7 ingress 已统一，LLM tool 调用主路径稳定 |
| 上游（已完成） | DM-20260616-001 `devrix-d7-uncertainty-gaps` — D3 错误响应路径已具备接入点 |
| 约束 | 不修改 DM-016 任何 library 代码（library 行为是 SoT；本次只做接入层） |
| 约束 | 不得修改 D2 QueryLoop / D7 Turn 编排主路径；wiring 走 `toolrunner.PluginRunner` 入口 |
| 约束 | 不得在 `internal/layers/<domain>/` 跨层直接调用，必须走 `internal/shared/` 横切接口 |
| 约束 | Bash AST 注入（AC10）必须先经 `verify-security` skill 闸门 |
| 约束 | A4 FaultInject 仍仅在 `-tags testbuild` 下生效；生产 binary 不得引入 fault 注入面（AC13 锁定 P2） |

## 5. 变更范围

### 5.1 新增 wiring 入口（按层次组织）

**Level 1: LLM tool 暴露**（5 项 — AC3/AC4/AC5/AC6/AC11）
- `internal/layers/contextengine/enforce/toolrunner/verify_tool.go` — G4 verifier tool runner
- `internal/layers/contextengine/enforce/toolrunner/verify_register.go` — tool 注册入口
- `internal/layers/contextengine/enforce/toolrunner/freefork_tool.go` — G5 free fork tool runner
- `internal/layers/contextengine/enforce/toolrunner/freefork_register.go` — tool 注册入口
- `internal/layers/contextengine/enforce/toolrunner/tracker_tool.go` — G6 tracker query tool runner
- `internal/layers/contextengine/enforce/toolrunner/tracker_register.go` — tool 注册入口
- `internal/layers/observability/diagnose/tracker/wire.go` — Bridge → Tracker 单例 + tick 钩子

**Level 2: CLI slash command 暴露**（2 项 — AC1/AC2）
- `internal/cli/doctor/doctor.go` — `devrix doctor` 子命令
- `internal/cli/context_analyze/context_analyze.go` — `devrix context-analyze` 子命令
- `internal/layers/communication/channel/adapters/cli.go` — `/doctor` `/context` 路由扩展

**Level 3: 错误路径接入**（3 项 — AC7/AC8）
- `internal/layers/llmgateway/dispatch/invoke.go` — 错误响应前 `errorclass.InjectClassification`
- `internal/layers/contextengine/enforce/toolrunner/sandbox.go` — sandbox 拒绝错误 `errors.WithShortStack`
- `internal/layers/multiagent/agent/engine.go` — Agent lifecycle 错误 `WithShortStack`

**Level 4: AST 注入 + Debug Filter 接入**（2 项 — AC10/AC12）
- `internal/bootstrap/context_engine_builder.go` — `policy.ASTAnalyzer = sandboxast.NewPolicyAnalyzer()` 注入
- `internal/bootstrap/observability.go` — `InstallSlogBridge` 时 `debugfilter.New(...)` 包装

**Level 5: Transcript 持久化接入**（1 项 — AC9）
- `internal/layers/communication/capture/session_store.go` — `OnSessionClose` 钩子写 transcript
- `internal/layers/communication/capture/transcript/wire.go` — bootstrap 注入单例

**Level 6: Notify consume 接入**（1 项 — AC11）
- `internal/layers/contextengine/prepare/prompt/output_assembler.go` — Reply 渲染前 drain notify bus → `<task_notifications>` block

### 5.2 修改（4 个 wiring 入口文件）

- `internal/bootstrap/context_engine_builder.go` — 注册 4 个新 tool（verify / freefork / tracker / doctor-lite）
- `internal/bootstrap/observability.go`（新建 or 扩展）— debugfilter 包装
- `internal/cli/root.go` — 新增 `doctor` / `context-analyze` 子命令（可能新建文件而非修改 root.go，详见 S3）
- `internal/layers/communication/channel/adapters/cli.go` — `/doctor` `/context` 路由

### 5.3 不变更

- DM-20260616-003 已交付的 21 个 library 文件（library 行为是 SoT）
- D2 QueryLoop / D7 Turn 编排主路径
- D3 LLM Gateway 现有熔断/重试/超时逻辑
- D1 Communication 现有协议层
- A4 FaultInject 的 build-tag 行为（AC13 P2 锁定）
- 现有 `openspec/specs/d*-*/spec.md` Scenario（仅追加 wiring 层 Scenario）

## 6. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| **G6 Tracker 后台 tick 阻塞 edit 主路径** | 编辑延迟 +5s → 用户体感差 | tick 走 `obsBridge` 异步 goroutine；edit 主路径不阻塞；超时（5s）回退 fallback |
| **G5 FreeFork 失败回滚不完整导致子 agent 泄漏** | 进程/端口泄漏 | `DefaultForker.Fork` 已有失败回滚单测（10 case）；本次接入点确保 ctx 取消传播到所有 Handle |
| **G3 Notify consume 注入 `<task_notifications>` 干扰 LLM 输出格式** | LLM 误解上下文 | block 严格遵循 clawcode XML 格式；用 `<reminder>` 包裹；assembler 单测验证不破坏现有 reply schema |
| **G2 Bash AST 注入改变 CommandPolicy 默认行为 → 误杀现有 allowlist 命令** | 合法 bash 命令被拒绝 | AST 仅对 regex 已通过的命令二次审计；AC10 单测覆盖"AST 不应拦 `ls -la` / `go test ./...`"白名单 |
| **AC1/AC2 /doctor /context 在 IM 路径触发时无 current session** | 输出为空 | `cmdDoctor` / `cmdContextAnalyze` 接受 `--session-id` flag 或默认取最新 session；fallback 输出空表 |
| **A3 Transcript 与 capture.FileSessionStore 双写冲突** | 同一 session 两份存储 | Transcript 仅写 `transcript/*.jsonl`，session_store 仍负责 session metadata；接口隔离 |
| **A6 ErrorClassify 在 ctx 取消场景下丢失 classification** | 错误响应不含分类 | `InjectClassification` 注入到 ctx 后立即缓存到 `errors.Is()` 包装；assembler 在 ctx cancel 时从 cache 读 |

## 7. Out of Scope

- **不修改** DM-20260616-003 已交付的 library 代码（library 行为不变，仅做接入）
- **不引入** 新 LLM provider / 新通信协议 / 新持久化后端
- **不实现** A4 FaultInject 的 IM 注入（仍 build-tag 隔离）
- **不覆盖** 跨 IM 平台差异（飞书 / Slack / CLI 行为对齐先不动）
- **不重构** `capture.FileSessionStore` 与 `transcript.Writer` 的关系（共存 6 个月再说）
- **不实现** clawcode `_claude_fs_right:` 等私有协议

## 8. 关联参考

- 上游 change：`openspec/archive/2026-06-17-devrix-diagnostic-tools-parity/`（13 项 library 实现）
- 上游 change：`openspec/archive/2026-06-17-devrix-queryloop-legacy-decommission/`（DM-017 QueryLoop 退役）
- 上游 change：`openspec/changes/devrix-d7-loop-first-routing/`（D7 ingress）
- 上游 change：`openspec/changes/devrix-d7-uncertainty-gaps/`（D3 错误响应路径）
- DSAFT 方法论：`openspec/specs/project/master.md` + `docs/methodology/dsaft-methodology.md`
- 域归档：`openspec/specs/d1-communication/` `d2-context-engine/` `d3-llm-gateway/` `d4-multi-agent/` `d5-observability/` `d6-evolution/` `tool-security/`
- T 注册表：`openspec/t-registry.md`（根索引）+ 各域 `openspec/specs/d{N}-*/t-registry.md`

## 9. 检查清单（S1 完成确认）

- [x] DM ID 已分配：`DM-20260617-002`（当日序号 002）
- [x] demand.md 包含背景、问题、验收标准、范围
- [x] 9 个 P0 验收标准（AC1-AC9）+ 3 个 P1（AC10-AC12）+ 2 个 P2（AC13-AC14）
- [x] Out of Scope 已明确（§7）
- [x] DSAFT 域标注正确（multi-domain，含 D1/D2/D3/D4/D5/D6 + tool-security 横切）
- [x] 复用现有 14 个 Activity 节点，不新增
- [x] 风险评估含影响与缓解（§6）
- [x] 跨域边界已声明（§4 约束 + §5.3 不变更）
- [x] 不动 DM-20260616-003 library 代码（§5.3 + §7）