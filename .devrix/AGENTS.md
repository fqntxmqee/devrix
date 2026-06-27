# Devrix Agent 规约

本文件是 **Agent 行为指令**（ClawCode 对齐：工作区规约，经 user prepend 注入）。
项目架构与 OpenSpec 流程见仓库根目录 `AGENTS.md`（供人类阅读，默认不注入 Agent）。

## 工程约束

- 遵循 OpenSpec S1–S7 交付管线；新功能需有 `openspec/changes/{slug}/demand.md`
- 使用 DSAFT 六域架构；生产路径为 QueryLoop（`harness.enabled: false` 默认）
- Commit 使用 Conventional Commits；单 PR 变更控制在 400 行以内
- **Git / PR / CI / Auto-merge**：`openspec/specs/project/git-workflow.md`（push、开 PR、盯 CI、合入前必读）
- Java 17+ / Spring Boot 3.x；TypeScript strict，禁止 `any`

## 编码原则

- 最小改动：只改用户请求范围内的代码
- 不为不可能发生的情况加错误处理
- 不为一次性操作抽象
- 改完必须验证：跑测试或检查输出
- 破坏性操作（删除、force push、覆盖未提交更改）先确认用户

## 工具使用

- 读文件用 `read_file`/`glob`，搜索用 `grep`，编辑用 `edit_file`
- 有专用工具时不用 bash 替代
- 独立工具调用可并行

## 测试

- 测试命名：`should_行为_when_条件`
- 标注 `// T: D*-S*-A*-T*` 关联测试点
- P0 测试点至少 1 个集成或 E2E 测试

## 领域术语

- 顶层包/目录用 L1 领域名（`trade`, `contextengine`）
- API URL kebab-case 复数：`/api/v1/order-items`
- 详见 `specs/02-domain-concepts.md`

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
