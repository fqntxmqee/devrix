# Design: .devrix/AGENTS.md project layout map

## 决策：为什么加到 .devrix/AGENTS.md 而不是 workspace_guidance.zh.md

`workspace_guidance.zh.md` 是 devrix 系统级 prompt 模板，所有用户共享。如果在那里加 D{N}→path 表：
- 其他用户的 devrix 项目（如 openspec-scaffold）没有 D{N} 概念
- 加进去反而误导其他项目

`.devrix/AGENTS.md` 是 devrix 项目特定的运行时 AGENTS 规约，加 D{N}→path 表正好服务本项目。

## 文件位置选择

devrix 默认 AGENTS.md 发现 sources：`[".devrix/AGENTS.md", "AGENTS.md"]`，`.devrix/AGENTS.md` 优先。所以改 `.devrix/AGENTS.md` 一定生效。

## 内容设计

```markdown
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
```

**为什么加"不确定先 ls"：** LLM 行为模式是盲搜失败就放弃（minimax M2.7），给个 fallback 提示让 LLM 在主搜索失败后能换探测路径。

## Cache Invalidation

`globalAgentsContextCache.entries` 是 package-level 全局 map，只在 snapshot corrupt 时清。修：
- **必须 process restart** 让 cache 空
- 不依赖 `mtime` 检查（设计上没做）

## 验证

跑飞书指令 "review d2 域代码" 后：
- LLM 应在 system prompt 看到 D{N}→path 表
- 第一次工具调用应是 `ls internal/layers/` 或 `glob internal/layers/*/` 而非 `**/d2/**`
- 后续 review 应针对 `internal/layers/contextengine/` 目录