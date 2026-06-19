# Design: D3 LLM Gateway spec 补登

**Change ID:** devrix-spec-sync-d3-package-map
**Demand ID:** DM-20260619-002

> docs-only change，3 个 D3 spec 文档的逐文件变更映射如下。SoT 不动：D3 域代码（`internal/layers/llmgateway/**`）。

---

## 1. 设计原则

1. **单向对齐**：D3 代码 v2.0 是 SoT；D3 spec 单向对齐代码
2. **保留架构骨架**：spec.md / design.md 章节框架不动，仅补登缺失子包与状态
3. **状态显式标注**：v2.0 "Phase F 实施中" → "已完成（DM-20260614-019）"
4. **代码锚点 grep 验证**：所有引用的 `xxx.go:Function` 必须能 `git grep` 命中

## 2. 文件级变更映射

### 2.1 W1: `openspec/specs/d3-llm-gateway/spec.md`

| 段 | 旧内容 | 新内容 |
|----|--------|--------|
| §2.1 配置加载 | `internal/shared/config/llmgateway.go` | `internal/layers/llmgateway/configure/shared_config.go` |
| §10 Package Map | 缺 `protect/` + `protect/errorclass/` + `configure/` | **新增 3 行** + 详细子包说明 |
| §13 FR-5 | "待实施" | "已实施（v1.1 F4）— `stream/adapter/protocol.go::Protocol()`" |
| Last Updated | 2026-06-16 | 2026-06-19 |

**Package Map 新增 3 行**：

| 路径 | 内容 |
|------|------|
| `internal/layers/llmgateway/protect/` | 韧性（circuit_breaker + retry + observer）；v2.0 重命名自 `breaker/` + `retry/` |
| `internal/layers/llmgateway/protect/errorclass/` | 错误分类（classifier.go），用于 retry 决策 |
| `internal/layers/llmgateway/configure/` | 配置加载（loader + shared_config + tests）；v2.0 重命名自 `config/` + `shared/config/` |

### 2.2 W2: `openspec/specs/d3-llm-gateway/design.md`

| 段 | 旧内容 | 新内容 |
|----|--------|--------|
| Header | v3.2.0 / 2026-06-14 | **v3.3.0** / 2026-06-19 |
| §0 变更摘要 | V3.1.0 → V3.2.0，v2.0 子 change | V3.2.0 → V3.3.0，v2.0 物理路径完成 |
| §10.2 物理路径 | "Phase F 实施中" | **"已完成（DM-20260614-019）"** |
| §10.2 子条目 | "v2.0 启动时执行"占位 | 删除占位文字，加"7 路径迁移 + 8 bridge + contracts.go 拆分"完成声明 |

### 2.3 W3: `openspec/specs/d3-llm-gateway/model-resolution-trace.md`

| 段 | 旧内容 | 新内容 |
|----|--------|--------|
| Last Updated | 2026-06-14 | **2026-06-19** |
| §一 链路总览 | v2.0 实施中（隐含）| 加 v2.0 状态说明（路径已全迁移） |
| §核心文件 | 部分文件路径 v1 | 同步到 v2.0 路径（gateway/router.go → route/router.go 等）|

## 3. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 路径变更遗漏 | 提交前 `git grep` 验证所有路径命中实际代码 |
| v2.0 状态描述模糊 | 显式引用 DM-20260614-019 + 已合并 commit |
| 与 design.md 状态不一致 | W1/W2/W3 三文档 Last Updated 统一刷至 2026-06-19 |

## 4. 不变更（边界声明）

- `internal/layers/llmgateway/**` 全部代码
- D3 Scenarios 行为（仅同步状态与目录树）
- D-S 编号体系（D3-S/A/F/T）
- t-registry.md
