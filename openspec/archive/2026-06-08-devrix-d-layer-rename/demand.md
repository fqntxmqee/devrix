---
demand-id: DM-20260608-007
title: 架构分层命名从 L1-X 迁移至 D1-D6
source: 项目架构规范
priority: P1
status: ARCHIVED
l1-domain: architecture
created: 2026-06-08
---

# 架构分层命名迁移

## 1. 背景

当前架构分层使用 `L1-X` / `L1-X-L2-Y` 编号体系：
- `L1-1` 和 `L1-1-L2-1` 中"L1"的含义不清晰
- 新人不查文档无法区分 L1（顶层）和 L1-1（第一个子层）
- 之前已搁置 D-S-A-F-T 五层方案（`devrix-layering-standard`），但可以只落地 D 层

## 2. 目标

| 当前 | 改为 | 含义 |
|------|------|------|
| `L1-1` 通信层 | `D1` 通信域 (COMM) | Domain 1 |
| `L1-2` 上下文引擎层 | `D2` 上下文域 (CTX) | Domain 2 |
| `L1-3` LLM 网关层 | `D3` LLM 网关域 (LLM) | Domain 3 |
| `L1-4` 多智能体层 | `D4` 多智能体域 (AGENT) | Domain 4 |
| `L1-5` 可观测性层 | `D5` 可观测域 (OBS) | Domain 5 |
| `L1-6` 演化层 | `D6` 演化域 (EVO) | Domain 6 |
| `L1-X-L2-Y` | `D{X}-S{Y}` | Domain-Scenario |

L5 ID 字符串不变（`L5-2-3-01` 保持原样）。

## 3. 验收标准

| ID | 标准 | 优先级 |
|----|------|--------|
| AC1 | `layering.md` 重写为 D-S 两层规范，合并 layering-v2/layering-standard/MIGRATION 内容 | P0 |
| AC2 | `project.md` 所有表格从 L1-X 更新为 D1-D6 | P0 |
| AC3 | `l5-registry.md` 节标题更新，ID 字符串不变 | P0 |
| AC4 | 项目规范（specs/project/）全部更新 | P1 |
| AC5 | 入口文件（AGENTS/CLAUDE/GEMINI/Cursor）更新 | P1 |
| AC6 | 层级增量文件标题更新 | P2 |

## 4. 约束

- 不修改 `internal/` 下任何 Go 源码
- 不修改 `devrix.yaml` 和 `config.yaml`
- 不修改 `openspec/archive/` 下已归档文件
- L5 ID 字符串保持不变
- 不引入完整的 D-S-A-F-T 五层 ID

## 5. 风险评估

| 风险 | 影响 | 缓解 |
|------|------|------|
| 新旧术语并存 | 中 | 一次性全量替换，不留残留 L1 引用 |
| 文档链接断裂 | 低 | 文件名不变，仅内容更新 |
