# Proposal: 架构分层命名迁移 L1-X → D1-D6

**Change ID:** devrix-d-layer-rename
**Demand ID:** DM-20260608-007
**Status:** S5 Accepted

---

## 1. Background

当前架构分层使用 `L1-X` / `L1-X-L2-Y` 编号体系，存在以下问题：
- **语义模糊**：`L1` 指顶层，"L1-1"指第一个子层，但编号前缀相同导致混淆
- **新人认知成本高**：不看文档无法从编号推断层级关系
- **与 D-S-A-F-T 方向不一致**：之前已探讨过五层业务分层，`L1-X` 是纯技术分层

决策：**只落地 D 层（Domain）**，把 `L1-X` 改为 `D1-D6`，`L2` 改为 `S`（Scenario）。不引入完整的 D-S-A-F-T 五层 ID。

## 2. Scope

### 2.1 In Scope

| Phase | 内容 | 文件数 | 变更量 |
|-------|------|--------|--------|
| P1: 核心架构文档 | layering.md 重写（合并 4→1），project.md 更新，l5-registry.md 更新 | 6 | ~220 |
| P2: 项目规范 | specs/project/ 下 7 个文件更新 | 7 | ~13 |
| P3: 入口文件 | AGENTS/CLAUDE/GEMINI/Cursor 更新 | 4 | ~30 |
| P4: 外围文档 | 6 个 layer_delta + devrix-v3 更新 | 8 | ~8 |

### 2.2 Out of Scope

- D-S-A-F-T 五层完整 ID（S/A/F/T 层留待后续）
- `internal/` Go 源码
- `openspec/archive/` 已归档文件
- `devrix.yaml` / `config.yaml`
- L5 ID 字符串变更

## 3. Mapping

```
L1-1 通信层     → D1 通信域 (COMM)
L1-2 上下文引擎 → D2 上下文域 (CTX)
L1-3 LLM 网关   → D3 LLM 网关域 (LLM)
L1-4 多智能体   → D4 多智能体域 (AGENT)
L1-5 可观测性   → D5 可观测域 (OBS)
L1-6 演化层     → D6 演化域 (EVO)

L2 → S (Scenario)
L1-X-L2-Y → D{X}-S{Y}
```

L5 ID 规范：`L5-{D}-{S}-{NN}` — 字符串与原来完全一致，仅语义变化。

## 4. Key Decision: layering.md 重写 vs 逐行修改

当前有 4 个架构分层文档：

| 文件 | 状态 | 内容 |
|------|------|------|
| `layering.md` | 当前活跃 | L1-L2 规范（~200 行，99 处要改） |
| `layering-v2.md` | 提案中 | D-S-A-F-T 五层规范（~380 行） |
| `layering-standard.md` | 草稿 | 方案对比与推荐 |
| `MIGRATION.md` | 指南 | 迁移路径与混合方案 |

**决策：重写 layering.md，合并其他 3 个的核心内容，删除冗余文件。**

新 layering.md 结构：
1. D-S 两层定义（D1-D6 域 + 各域的 S 场景）
2. L5 ID 编号规范
3. 代码目录映射
4. 历史：L1-L2 方案的废弃记录

## 5. Risks

| 风险 | 缓解 |
|------|------|
| 新旧术语短期并存 | 一次性全量替换，grep 验证无残留 |
| `layering-v2.md` 中有用的 D-S-A-F-T 内容丢失 | 保留在废弃记录中，写明被本 change 取代 |
