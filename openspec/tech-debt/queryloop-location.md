# Tech Debt: D2 QueryLoop 位置错位 — Legacy Path Decommission

**TD ID:** TD-QL-LOC
**Status:** OPEN
**Severity:** Medium（架构债，不影响功能但限制演进）
**Created:** 2026-06-17
**Owner:** —（待指派）
**Linked Change:** `devrix-diagnostic-tools-parity` (DM-20260616-003) sub-change-Z0
**Related:** DM-020 (D7 Turn 编排上移), DM-20260616-001 (D7 Uncertainty Gaps), DM-20260616-002 (D7 Loop-First Routing)

---

## 1. 债务描述

D2 域内 `internal/layers/contextengine/query/loop.go` 的 `Loop.Run()` 仍持有"while 有 tool_use 就再来一次"的循环主逻辑。这是 PEV 时代（D2-S1）的历史产物。

DM-020 已经把编排回调、per-turn 上下文注入、Hub-Spoke drain 上移 D7；DM-20260616-002 又把 `loopFirst` 设为默认 ingress。**主路径已经走 D7 RunTurnLoop 直跑**（`prepare → llm → tools → persist`），但 D2.QueryLoop.Run 仍作为 `loopFirst=false` 的 legacy fallback 存在。

遗留矛盾：
1. `internal/shared/contracts/llm_facade.go:11` 注释自证"D2→D3 拆面出口" — 承认现状是绕道
2. `internal/layers/orchestration/turn/query_llm_caller.go:21` 注释自证"D2→D3 拆面 adapter" — 重复声明
3. D2.QueryLoop 结构体持有 LLMCaller / FallbackLLM 等字段，本质是 D2 在做"循环调度"，违反边界规则
4. 测试复杂度高：mock Loop 必须同时 mock LLM + Tools + Permission + Compress + UserContext

## 2. 现状证据

### 2.1 主路径已上移 D7（DM-20260616-002）

| 路径 | 文件:行 | 说明 |
|------|---------|------|
| D7 RunTurnLoop 入口 | `internal/layers/orchestration/turn/orchestrator.go:49` | `RunTurn executes prepare→llm→tools→persist (D7-S2-A06)` |
| D7 直调 D3 | `internal/layers/orchestration/turn/llm.go:50` | `GatewayInvoker.Stream` 直连 `llmgateway` (D3) |
| loopFirst 默认 true | `internal/layers/orchestration/coordinator/routing.go:24` | `IsLoopFirst()` 默认 `true` |
| loopFirst 测试 | `routing_test.go:11` | `if !cfg.IsLoopFirst()` 验证默认 |

### 2.2 Legacy 路径仍存在（D2.QueryLoop.Run）

| 路径 | 文件:行 | 说明 |
|------|---------|------|
| Legacy executor | `internal/bootstrap/wire_coordinator.go:75` | "replaces the legacy executor" — 说明 legacy 是被替换对象 |
| D2 QueryLoop 主循环 | `internal/layers/contextengine/query/loop.go` | `Loop.Run()` 跑 while tool_use 循环 |
| D2→D3 拆面 adapter | `internal/shared/contracts/llm_facade.go:11` | "D2 query loop 拆面出口" |
| D2→D3 拆面 adapter | `internal/layers/orchestration/turn/query_llm_caller.go:21` | "D2→D3 拆面 adapter" |
| Legacy 路径 T 层 | `internal/layers/contextengine/path_regression_integration_test.go:14` | `query_loop.enabled=true` 时走 D7，`false` 时走 legacy |

## 3. 理想架构（用户提的）

```
D7.RunTurnLoop (持有循环状态)
├── msgs := D2.PrepareMessages(sc, uc, attachments)    // 无状态组装
├── for {
│     resp := D3.StreamChat(msgs, tools)              // D7 → D3 直调（已有）
│     emit Event{type: tool_use, ...}
│     if !hasToolUse(resp) { return resp }
│     if D7.ShouldStop(msgs, resp, budget) { return partial }
│     if D7.ShouldCompress(msgs) { msgs = D2.Compress(msgs) }
│     result := D2.ExecuteTool(resp.ToolUse, sc)        // 无状态执行
│     D7.AuditToolCall(resp.ToolUse, result)
│     msgs = D2.AppendToolResult(msgs, result)          // 无状态追加
│   }
└── return final
```

D2 退化为 stateless 服务：`PrepareMessages` / `ExecuteTool` / `AppendToolResult` / `Compress` / `GetToolDefinitions`。**`D2.QueryLoop` 整个文件可删除**（loopFirst=true 时无任何调用方；legacy 路径标 deprecated）。

## 4. 当前架构 vs 理想架构对比

| 维度 | 当前（loopFirst=true 主路径） | 当前（legacy fallback） | 理想（全 loopFirst） |
|------|----------------------------|----------------------|-------------------|
| 循环调度权 | D7 | **D2** | D7 |
| LLM 调用方 | D7 → D3 直调 | **D2（通过拆面 adapter）** | D7 → D3 直调 |
| D2 形态 | D2 只是被 D7 调用的 PrepareMessages-like | **D2 持有完整 Loop 状态** | stateless 服务 |
| 拆面 adapter | 不需要 | **存在（自证矛盾）** | 不存在 |
| 测试 D7 循环 | 干净（D7 拥有循环） | N/A | 干净 |
| 测试 D2 组装 | D2 单测只测 PrepareMessages 即可 | **要 mock 整个 Loop 字段** | D2 单测只测 stateless 函数 |

## 5. 缓解（按 sub-change-Z0 推进）

### 5.1 Z0 范围（v1.0' — 本 change 内）

**目标：**让 legacy 路径退役有信号、有护栏、有时间表。

| 步骤 | 产出 | 验收 |
|------|------|------|
| 1 | D2.QueryLoop.Run 顶部加 `// Deprecated:` 注释 + slog.Warn | 启用 legacy 时日志可见 |
| 2 | 新增 metric `d2_query_loop_legacy_invocations_total` | 监控 legacy 调用频次 |
| 3 | `loopFirst=false` 配置项加文档警告"仅作临时回滚使用" | 配置 help 输出警告 |
| 4 | 新增 T 层 `D7-S2-A06-T09`：loopFirst=true 时 D2→D3 拆面 adapter 绝不被调用 | 路径监控测试 |
| 5 | 新增 T 层 `D7-S2-A06-T10`：loopFirst=true 时 D2.QueryLoop.Run 绝不被调用 | 路径监控测试 |
| 6 | 本 change 的 G1 LSP Tool / A5 上下文窗口分析 等能力**仅在 loopFirst=true 路径下验证**，不写 legacy 兼容 | 测试覆盖差异 |

### 5.2 Z0 不做（留给后续）

- **不删除** D2.QueryLoop.Run()（保留回滚能力）
- **不删除** 拆面 adapter 文件（legacy 仍需要）
- **不动** 主路径代码（已经实现）
- **不迁移** 现有 D2-S10 T 层测试到 D7（保留为 LEGACY 标记）

### 5.3 后续演进（Z1+）

| 阶段 | 触发条件 | 工作 |
|------|---------|------|
| Z1 | legacy metric 连续 4 周 < 1 invocations/day | 把 D2.QueryLoop.Run 中的循环逻辑改为 thin wrapper，直接调 D7.RunTurnLoop（去重） |
| Z2 | legacy metric 连续 12 周 = 0 | 删除 D2.QueryLoop.Run；删除 `query_loop.enabled` 配置项；删除拆面 adapter 文件 |
| Z3 | Z2 完成后 | D2 域定位文档更新："D2 = 无状态上下文组装服务"；原 D2-S10 场景从 spec.md 移除或标 LEGACY |

## 6. 与 devrix-diagnostic-tools-parity 其他子能力的关系

| 子能力 | 与 Z0 关系 | 备注 |
|--------|----------|------|
| **G1 LSP Tool** | Z0 提供干净注册点（loopFirst=true 时通过 D7 RunTurnLoop 的 ToolRoundExecutor） | LSP Tool 实现只依赖 `ToolExecutor` 接口，loopFirst 切换无感 |
| **G3 后台任务通知** | loopFirst 路径下 emit 由 D7 持有，比 legacy 路径更直观 | — |
| **A5 上下文窗口分析** | D7 看到每轮 token，可决策压缩；legacy 路径下压缩触发在 D2 | loopFirst 切换后无需改 A5 实现 |
| **G2 Bash AST 安全** | tool-security 横切，与 loopFirst 路径无关 | — |
| **G4 实现后验证** | D6 Eval，独立于 loopFirst 路径 | — |
| **G5 自由分叉** | D4 Multi-Agent，与 loopFirst 路径无关 | — |
| **G6 文件诊断追踪** | edit_file/write_file 埋点，与 loopFirst 路径无关 | — |
| **A1 / A2 / A4** | D5 Observability，与 loopFirst 路径无关 | — |
| **A3 会话转录** | D1 Communication，与 loopFirst 路径无关 | — |
| **A6 / A7** | 错误分类 / 堆栈截断，与 loopFirst 路径无关 | — |

**核心结论**：Z0 是**所有依赖 D7 Turn 路径的诊断工具的强基线**，但**不阻塞**任何诊断工具的实现。其他 13 项能力可与 Z0 并行推进，但本 change 范围内优先 Z0 落地（确保后续演进可见）。

## 7. 验收标准（AC-Z0）

| ID | 标准 | 验证方式 |
|----|------|---------|
| AC-Z0-1 | loopFirst=true（默认）时，D2.QueryLoop.Run() 零调用 | 集成测试：注入调用计数器，断言 0 |
| AC-Z0-2 | loopFirst=true（默认）时，D2→D3 拆面 adapter 零调用 | 同上 |
| AC-Z0-3 | loopFirst=false 时 D2.QueryLoop.Run 顶部输出 `Deprecated:` slog.Warn | 捕获 slog 输出 |
| AC-Z0-4 | metric `d2_query_loop_legacy_invocations_total` 注册且可观测 | 启动后从 /metrics 端点读 |
| AC-Z0-5 | devrix.yaml 帮助输出"loopFirst 仅作临时回滚使用" | 集成测试：调 `--help` 验证警告文本 |
| AC-Z0-6 | 本 change 中所有新增能力（LSP Tool / 后台通知 / 上下文窗口）均**仅在 loopFirst=true 路径下验证** | 测试矩阵审查 |

## 8. 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| 现有用户依赖 loopFirst=false | 强制切换破坏其工作流 | Z0 不删除 legacy，只警告；Z1+ 才删除 |
| D2.QueryLoop.Run 仍被外部测试或脚本直接引用 | 删除会破坏 CI | 标记 Deprecated 后保留 1-2 release |
| 新增诊断工具能力写错路径，依赖 legacy | 路径混用导致主路径不生效 | AC-Z0-6 + 测试矩阵审查 |

## 9. 关联参考

- D2/D7 边界规范：`openspec/specs/d2-context-engine/d7-boundary.md`
- D7 Turn 编排设计：`internal/layers/orchestration/turn/orchestrator.go`
- LoopFirst 配置：`internal/layers/orchestration/coordinator/routing.go`
- 主路径 PR：DM-20260616-002（devrix-d7-loop-first-routing, PR #46）
- D7 状态归档：`openspec/changes/devrix-d7-uncertainty-gaps/` + `devrix-d7-loop-first-routing/`
