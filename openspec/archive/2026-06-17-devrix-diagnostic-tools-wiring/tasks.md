# Tasks: devrix-diagnostic-tools-wiring

**Change ID:** devrix-diagnostic-tools-wiring
**Demand ID:** DM-20260617-002
**Status:** S4_Implementation
**估算参考（仅供参考，非承诺）:** 6 Phase × 14 W, ~1300 LOC + ~800 LOC tests

---

> **DSAFT Activity 一览**
>
> 本 change 复用 DM-016 已确立的 14 个 Activity 节点。W 编号按 Phase 组织，每个 W 标注关联 Activity（alias）。

## Phase 1: 错误路径接入（L3，最独立）

### W1 — D3-S3-A02 (alias A6) ErrorClassify 注入到 LLM 网关错误响应

- **文件:** `internal/layers/llmgateway/dispatch/invoke.go`
- **改动:** DispatchInvoke 错误路径增加 `errorclass.NewDefaultClassifier().Classify(...)` + `InjectClassification` + `[class=...]` 标签
- **测试:** `internal/layers/llmgateway/dispatch/invoke_classify_test.go` (新建, ~80 行)
  - HTTP 401 → AuthRequired
  - LLMError.Code="rate_limit" → RateLimit
  - 成功响应无分类
- **依赖:** 无（errorclass 已就绪）
- **AC:** AC7
- **T:** D3-S3-A02-T01
- **估时参考:** 30 min

### W2 — D2-S6-A02 (alias A7) ShortStack 包装 sandbox 拒绝错误

- **文件:** `internal/layers/contextengine/enforce/toolrunner/sandbox.go`
- **改动:** `sandbox: ast block: ...` 与 `sandbox: dangerous command pattern detected: ...` 错误用 `errors.WithShortStack` 包装
- **测试:** `internal/layers/contextengine/enforce/toolrunner/sandbox_shortstack_test.go` (新建, ~40 行)
  - 错误栈 ≤ 5 帧
  - runtime/testing/reflect 帧被过滤
- **依赖:** 无（shortstack 已就绪）
- **AC:** AC8 (前半)
- **T:** D2-S6-A02-T01
- **估时参考:** 20 min

### W3 — D2-S6-A02 (alias A7) ShortStack 包装 agent lifecycle 错误

- **文件:** `internal/layers/multiagent/agent/engine.go`
- **改动:** spawnChild 失败错误用 `errors.WithShortStack` 包装
- **测试:** `internal/layers/multiagent/agent/engine_shortstack_test.go` (新建, ~40 行)
- **依赖:** 无
- **AC:** AC8 (后半)
- **T:** D2-S6-A02-T01
- **估时参考:** 20 min

## Phase 2: AST 注入 + DebugFilter（L4）

### W4 — TOOL-SEC-2-A02 (alias G2) Bash AST 注入 bootstrap

- **文件 1:** `internal/bootstrap/context_engine_builder.go`
- **改动:** `execCfg.policy.ASTAnalyzer = sandboxast.NewPolicyAnalyzer()`（当 Sandbox.ASTEnabled=true）
- **文件 2:** `internal/shared/config/tool_config.go`
- **改动:** SandboxConfig 新增 `ASTEnabled bool` 字段，默认 true
- **测试:** `internal/bootstrap/context_engine_builder_ast_test.go` (新建, ~50 行)
  - ASTEnabled=true → ASTAnalyzer 不为 nil
  - ASTEnabled=false → ASTAnalyzer 为 nil
  - bash heredoc body with $(whoami) 被拦
- **依赖:** sandboxast 已就绪
- **AC:** AC10
- **T:** TOOL-SEC-2-A02-T01
- **估时参考:** 30 min

### W5 — D5-S24-A02 (alias A2) DebugFilter 接入 bootstrap

- **文件 1:** `internal/bootstrap/observability.go` (新建)
- **改动:** `InstallDebugFilter(categories)` 函数包装 slog.Default()
- **文件 2:** `cmd/devrix/main.go`
- **改动:** 解析 `--debug=api,hooks` flag 并调用 `InstallDebugFilter`
- **测试:** `internal/bootstrap/observability_test.go` (新建, ~40 行)
  - categories 非空 → filter 启用
  - categories 为空 → 跳过
- **依赖:** debugfilter 已就绪
- **AC:** AC12
- **T:** D5-S24-A02-T01
- **估时参考:** 30 min

## Phase 3: LLM Tool 暴露（L1，最大 surface）

### W6 — D6-S11-A02 (alias G4) verify_plan_execution LLM tool

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/verify_tool.go` (新建, ~60 行)
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/verify_register.go` (新建, ~15 行)
- **文件 3:** `internal/bootstrap/context_engine_builder.go`
- **改动:** 调用 `RegisterVerifyTool`
- **测试:** `internal/layers/contextengine/enforce/toolrunner/verify_tool_test.go` (新建, ~80 行)
  - done items verified
  - missing file → unverified
  - _test.go without func TestXxx → unverified
- **依赖:** evolution/verify 已就绪
- **AC:** AC4
- **T:** D6-S11-A02-T01
- **估时参考:** 60 min

### W7 — D4-S11-A02 + D4-S13-A02 (alias G5) free_fork LLM tool

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/freefork_tool.go` (新建, ~80 行)
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/freefork_register.go` (新建, ~25 行)
- **文件 3:** `internal/bootstrap/multiagent.go` (新建 or 扩展)
- **改动:** `freefork.NewDefaultForker(...)` + `toolrunner.SetGlobalForker(...)`
- **文件 4:** `internal/bootstrap/context_engine_builder.go`
- **改动:** 调用 `RegisterFreeForkTool` (使用 GlobalForker)
- **测试:** `internal/layers/contextengine/enforce/toolrunner/freefork_tool_test.go` (新建, ~100 行)
  - batch fork 3 success
  - factory failure rollback
  - requests count > 5 reject
- **依赖:** multiagent/provision/freefork 已就绪, agentFactory bootstrap 已就绪
- **AC:** AC5
- **T:** D4-S11-A02-T01/T02
- **估时参考:** 90 min

### W8 — D5-S23-A02 (alias G6) query_diagnostics LLM tool + 异步 tick

- **文件 1:** `internal/layers/contextengine/enforce/toolrunner/tracker_tool.go` (新建, ~50 行)
- **文件 2:** `internal/layers/contextengine/enforce/toolrunner/tracker_register.go` (新建, ~25 行)
- **文件 3:** `internal/layers/observability/diagnose/tracker/wire.go` (新建, ~20 行)
- **改动:** `GlobalTracker` 单例 + `TickOnce` 函数
- **文件 4:** `internal/bootstrap/context_engine_builder.go`
- **改动:** 创建 tracker + RegisterTrackerTool + 启动 tick goroutine (1s 间隔, ctx 控制)
- **测试:** `internal/layers/contextengine/enforce/toolrunner/tracker_tool_test.go` (新建, ~80 行)
  - tracker tick 后 query_diagnostics 返回新错误
  - 无 tick 时 query_diagnostics 返回空
- **依赖:** observability/diagnose/tracker 已就绪
- **AC:** AC6
- **T:** D5-S23-A02-T01/T02
- **估时参考:** 90 min

## Phase 4: CLI Slash Command 暴露（L2）

### W9 — D5-S23-A03 (alias A1) /doctor CLI 子命令

- **文件 1:** `internal/cli/doctor/doctor.go` (新建, ~80 行)
- **文件 2:** `cmd/devrix/main.go`
- **改动:** `if os.Args[1] == "doctor" { doctor.CLI{}.Run(...); return }`
- **文件 3:** `internal/layers/communication/channel/adapters/cli.go`
- **改动:** switch 增加 `CommandDoctor` case
- **文件 4:** `internal/shared/types/command.go` (新建 or 修改)
- **改动:** 增加 `CommandDoctor` 常量
- **测试:** `internal/cli/doctor/doctor_test.go` (新建, ~50 行)
  - doctor.Run 返回 7 项 check
  - lsp_fake 返回 fail
- **依赖:** observability/diagnose/doctor 已就绪
- **AC:** AC1
- **T:** D5-S23-A03-T01/T02
- **估时参考:** 60 min

### W10 — D2-S6-A03 (alias A5) /context analyze CLI 子命令

- **文件 1:** `internal/cli/context_analyze/context_analyze.go` (新建, ~60 行)
- **文件 2:** `cmd/devrix/main.go`
- **改动:** `if os.Args[1] == "context-analyze" { ... }`
- **文件 3:** `internal/layers/communication/channel/adapters/cli.go`
- **改动:** switch 增加 `CommandContextAnalyze` case
- **测试:** `internal/cli/context_analyze/context_analyze_test.go` (新建, ~50 行)
- **依赖:** windowanalyzer + capture.FileSessionStore 已就绪
- **AC:** AC2
- **T:** D2-S6-A03-T01
- **估时参考:** 60 min

## Phase 5: Transcript + Notify Consume（L5 + L6）

### W11 — D1-S2-A02 (alias A3) Transcript OnSessionClose 持久化

- **文件 1:** `internal/layers/communication/capture/transcript/wire.go` (新建, ~15 行)
- **改动:** `GlobalWriter` 单例
- **文件 2:** `internal/layers/communication/capture/session_store.go`
- **改动:** Close 钩子写 transcript
- **文件 3:** `internal/bootstrap/context_engine_builder.go`
- **改动:** 创建 transcript.Writer + SetGlobalWriter
- **测试:** `internal/layers/communication/capture/session_store_transcript_test.go` (新建, ~60 行)
  - session close 后 .jsonl 写入
  - ListSessions 按 mtime 倒序
- **依赖:** capture/transcript 已就绪
- **AC:** AC9
- **T:** D1-S2-A02-T01
- **估时参考:** 60 min

### W12 — D4-S12-A03 (alias G3) Task Notify consume via prompt assembler

- **文件:** `internal/layers/contextengine/prepare/prompt/assembler.go`
- **改动:** AssembleReminder 在 reminder 段追加 `<task_notifications>` drain
- **测试:** `internal/layers/contextengine/prepare/prompt/assembler_notify_test.go` (新建, ~50 行)
  - notify bus 有 1 event → block 注入
  - notify bus 空 → block 不注入
- **依赖:** notify.GlobalBus 已就绪
- **AC:** AC11
- **T:** D4-S12-A03-T01/T02
- **估时参考:** 45 min

## Phase 6: 集成 + 验收

### W13 — 配置 + Bootstrap 总集成

- **文件 1:** `internal/shared/config/contextengine.go`
- **改动:** `DiagnosticsConfig` 结构体（含 TrackerLRUCapacity, TrackerTickIntervalMs, LSPEnabled, LSPServers, DebugCategories, TranscriptDir）
- **文件 2:** `internal/bootstrap/context_engine_builder.go`
- **改动:** 把 DiagnosticsConfig 注入到所有 wiring 入口
- **测试:** `internal/bootstrap/diagnostics_config_test.go` (新建, ~40 行)
- **AC:** AC14 (P2 锁定)
- **估时参考:** 30 min

### W14 — 集成测试 + E2E IM 验证脚本

- **文件:** `tests/integration/diagnostic_tools_wiring_test.go` (新建, ~150 行)
- **覆盖:**
  - 全量 wiring 单测跑通
  - mock feishu adapter 发 "/doctor" → 收到 7 check 表
  - mock feishu adapter 发 "/context" → 收到 5 类 token 拆分
  - LLM tool 调用 G4/G5/G6 模拟
  - 错误注入验证 A6/A7 标签
- **AC:** AC18, AC20
- **估时参考:** 120 min

## 总览

| Phase | W 编号 | Activity | 估时参考 | AC |
|-------|--------|----------|---------|----|
| P1 错误路径 | W1-W3 | A6, A7 | 70 min | AC7, AC8 |
| P2 AST + Filter | W4-W5 | G2, A2 | 60 min | AC10, AC12 |
| P3 LLM tools | W6-W8 | G4, G5, G6 | 240 min | AC4, AC5, AC6 |
| P4 CLI slash | W9-W10 | A1, A5 | 120 min | AC1, AC2 |
| P5 Transcript + Notify | W11-W12 | A3, G3 | 105 min | AC9, AC11 |
| P6 集成 | W13-W14 | ALL | 150 min | AC14, AC18, AC20 |
| **合计** | **14** | **10 个 Activity + ALL** | **~12.4 h** | **AC1-AC12, AC18, AC20** |

> **注意：** 估时仅供参考，非承诺。实际进度按 Phase 推进。

## 执行顺序

1. **W1 → W2 → W3**（Phase 1 — 最独立，先做）
2. **W4 → W5**（Phase 2）
3. **W6 → W7 → W8**（Phase 3 — 并行可能但需要隔离 commit）
4. **W9 → W10**（Phase 4）
5. **W11 → W12**（Phase 5）
6. **W13 → W14**（Phase 6 — 集成）

每个 W 完成后立即 `git add` + `git commit`（独立 commit），便于回滚与 review。

## 文件交付清单（按 W 汇总）

详见 design.md §5 File Manifest。