# Spec: devrix-d2-agents-md-project-layout

## Feature: Project layout map in runtime AGENTS.md

作为 devrix 项目的运行时 AGENTS 规约，`.devrix/AGENTS.md` 必须暴露项目结构布局，让 LLM 在收到 D{N} 域相关指令时能直接定位到对应目录。

## Scenario: D{N} → path resolution

**Given:** `.devrix/AGENTS.md` 含 "## 项目结构（D{N} → 路径）" 章节
**When:** LLM 收到指令 "review d2 域代码"
**Then:** LLM 应直接定位 `internal/layers/contextengine/` 并产出该目录的代码 review

## Scenario: Project layout map content

**Given:** `.devrix/AGENTS.md` 项目结构表
**When:** LLM 查阅该表
**Then:** 应能看到：
- D1=communication, D2=contextengine, D3=llmgateway, D4=multiagent, D5=observability, D6=evolution, D7=orchestration
- 每个域对应 `internal/layers/<domain>/`
- 跨域共享 `internal/shared/`
- 域归档 `openspec/specs/d{N}-*/`

## Scenario: Fallback hint

**Given:** `.devrix/AGENTS.md` 含"不确定时先 ls"提示
**When:** LLM 收到不明确的域指令
**Then:** LLM 应优先 `ls internal/layers/` 探测目录结构，而非盲搜 `**/d2/**`