# Devrix Agent 规约

本文件是 **Agent 行为指令**（ClawCode 对齐：工作区规约，经 user prepend 注入）。
通用编码/工具/输出原则见系统核心模板；本文件只保留 **本项目** 约束。

项目架构与 OpenSpec 流程见仓库根目录 `AGENTS.md`（供人类阅读，默认不注入 Agent）。

## 工程约束

- 遵循 OpenSpec **S1–S6 六阶段**（S3-Gate / S4-Gate 为阶段内门禁；**S6 含交付合入 + 文档归档** 两子步；终端元数据状态 `s7_archived` 不是第八阶段）
- 新功能 Change 目录：`openspec/changes/<change-id>/`（`change-id` 格式 `devrix-{module-name}`）；轻量变更可免 `demand.md`
- 使用 DSAFT 七域架构（D1–D7）；生产路径为 QueryLoop（`harness.enabled: false` 默认）
- Commit 使用 Conventional Commits；单 PR 变更控制在 400 行以内（超出需在 PR 说明理由）
- **Git / PR / CI / Auto-merge**：`openspec/specs/project/git-workflow.md`（push、开 PR、盯 CI、合入前必读）
- **Go 项目**：`internal/` 下 Go 源码；编码规范见 `openspec/specs/project/coding.md`

## 测试

- 测试命名：`should_行为_when_条件`（或 table-driven `TestXxx_场景`）
- 标注 `// T: D*-S*-A*-T*` 关联测试点
- P0 测试点至少 1 个可执行验收路径（单元/集成/E2E 按 T 层定义）

## 领域术语

- 顶层包/目录用 L1 领域名（`communication`, `contextengine`, `orchestration` 等）
- 域概念与边界：`openspec/specs/d{N}-*/design.md` 与 `openspec/specs/architecture/layering.md`

## 项目结构（D{N} → 路径）

用户提到 `D{N}` 或 `d{N} 域` 时直接定位：

| 域  | 中文     | 目录                                   | 备注                |
| --- | ------ | ------------------------------------ | ----------------- |
| D1  | 通信层    | `internal/layers/communication/`      | Im/adapter/bridge |
| D2  | 上下文引擎  | `internal/layers/contextengine/`     | prompt / recall   |
| D3  | LLM 网关 | `internal/layers/llmgateway/`        | 流式 / tool schema |
| D4  | 多智能体   | `internal/layers/multiagent/`        | subagent / fork   |
| D5  | 可观测性   | `internal/layers/observability/`     | span / metric     |
| D6  | 演化层    | `internal/layers/evolution/`         | guard / learn     |
| D7  | 编排层    | `internal/layers/orchestration/`     | SessionOrchestrator / MUPS |

- 跨域共享：`internal/shared/`（types / config / errors / contracts）
- 域架构归档：`openspec/specs/d{N}-*/`（spec.md + A/F/T 注册表）
- 不确定时先 `ls internal/layers/` 再继续，避免盲搜 `**/d2/**`
