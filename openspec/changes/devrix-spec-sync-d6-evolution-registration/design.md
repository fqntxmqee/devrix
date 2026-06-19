# Design: D6 Evolution spec 补登

**Change ID:** devrix-spec-sync-d6-evolution-registration
**Demand ID:** DM-20260619-003

> docs-only change，D6 spec 三份文档的逐文件变更映射 + 新建 d6-domain.md。SoT 不动：D6 域代码（`internal/layers/evolution/**`）。

---

## 1. 设计原则

1. **单向对齐**：D6 代码 v2.0 是 SoT；D6 spec 单向对齐代码
2. **结构对齐**：新建 d6-domain.md 对齐 D2/D4/D5/D7 `d{N}-domain.md` 结构（域描述 + 价值流 S 层 + 跨域契约）
3. **历史留痕**：guard 子包误删 → 42bf1d7 恢复 在 d6-domain.md 留痕（避免再次误删）
4. **路径重命名显式声明**：eval→evaluate, orchestration→guard 在 spec 三份文档统一加 ADDED/CHANGED 章节

## 2. 文件级变更映射

### 2.1 W1: `openspec/specs/d6-evolution/spec.md`

| 段 | 旧内容 | 新内容 |
|----|--------|--------|
| §Package Map | `eval/engine.go` `eval/judge.go` 等 8 行 | **改用 `evaluate/` 前缀**：evaluate/engine.go / evaluate/judge.go 等 |
| §Package Map | `RuntimeOrchestrationValidator | orchestration/validator.go` | **删除**（已迁至 guard/） |
| §Package Map | 缺 `guard/` + `verify/` | **新增 2 行** |
| §Last Updated | 2026-06-14 | 2026-06-19 |
| §Version | v2.2.0 | **v2.3.0**（v2.0 物理路径完整同步） |

**Package Map 新增 2 行**：

| 路径 | 内容 |
|------|------|
| `internal/layers/evolution/guard/` | Guard 韧性（config + intervention + judge_adapter + metrics + observer + types + validator）；v2.0 重命名自 `orchestration/`，**曾因误删从 42bf1d7 恢复** |
| `internal/layers/evolution/verify/` | Invariant 验证（`_invariant.go` + `plan.go`）；v2.0 物理独立 |

**Package Map 改路径 8 行**（`eval/` → `evaluate/`）：
- `eval/engine.go` → `evaluate/engine.go`
- `eval/judge.go` → `evaluate/judge.go`
- `eval/delta.go` → `evaluate/delta.go`
- `eval/tune.go` → `evaluate/tune.go`
- `eval/dataset.go` → `evaluate/dataset.go`
- `eval/probe.go` → `evaluate/probe.go`
- `eval/gateway_llm.go` → `evaluate/gateway_llm.go`
- `eval/mock_llm.go` → `evaluate/mock_llm.go`

### 2.2 W2: `openspec/specs/d6-evolution/design.md`

| 段 | 旧内容 | 新内容 |
|----|--------|--------|
| Header | v2.1.0 / 2026-06-14 | **v2.2.0** / 2026-06-19 |
| §Package Map | `eval/` + `orchestration/` 旧路径 | `evaluate/` + `guard/` + `verify/` |
| §v2.0 状态 | 实施中 | 已完成（DM-20260615-003）|

### 2.3 W3: `openspec/specs/d6-evolution/layer-delta.md`

**追加 v2.0 物理路径迁移章节**：

```markdown
## ADDED — D6 v2.0 物理路径迁移（2026-06-15，DM-20260615-003）

### Requirement: D6 Subpackage Renames + New Subpackage

v2.0 物理路径迁移落地 3 包重命名 + 1 包新增：

| 旧路径 | 新路径 | 原因 |
|--------|--------|------|
| `eval/` | `evaluate/` | 与 D3 evaluate/ 命名对齐；避免 eval 关键字歧义 |
| `orchestration/` | `guard/` | 与 D7 orchestration/ 同名冲突；guard 更准确反映"决策入口 + 干预触发"职责 |
| `exporter/` | `export/` | 命名统一（其他域均无 -er 后缀） |
| (无) | `verify/` | Invariant 验证从 evaluate 物理独立 |

#### Scenario: 路径迁移完整
- GIVEN 6 个重命名/新增子包
- WHEN v2.0 落地
- THEN `internal/layers/evolution/` 含 `eval/ + evaluate/ + guard/ + orchestration/ + verify/ + export/ + exporter/ + spans.go`（含 bridge 桥接）
- AND bridge.go 在 v2.0.1 cleanup 后全部删除（11 个 bridge.go 移除）

#### Scenario: guard 误删恢复
- GIVEN guard 子包曾因 orchestration→guard 重命名误删
- WHEN 42bf1d7 提交恢复
- THEN guard 子包 7 个 .go 文件完整存在
- AND spec d6-domain.md §历史留痕 显式标注该事件
```

### 2.4 W4: 新建 `openspec/specs/d6-evolution/d6-domain.md`

**结构对齐 D2/D4/D5/D7 `d{N}-domain.md`**：

```markdown
# D6 Evolution Domain Specification

**Capability:** evolution
**Status:** Active
**Version:** 1.0.0
**Last Updated:** 2026-06-19

## 1. 域描述
...（Self-Eval + Guard + Verify 三大价值流）

## 2. 价值流 S 层
- D6-S11 Evaluate（v2.0 物理路径：evaluate/）
- D6-S12 GuardRuntime（v2.0 物理路径：guard/；**曾因误删从 42bf1d7 恢复**）
- D6-S13 VerifyInvariant（v2.0 新增物理独立：verify/）

## 3. 跨域契约
- 与 D2: evaluate/probe.go 读取 D2 trace（compression_recall / path_regression）
- 与 D3: evaluate/judge.go + evaluate/gateway_llm.go 经 D3 LLM Gateway
- 与 D5: guard/metrics.go + guard/observer.go 写入 D5
- 与 D7: guard/types.go + guard/validator.go 接收 D7 编排事件

## 4. 历史留痕
- 2026-06-15 DM-20260615-003: v2.0 物理路径迁移（eval→evaluate / orchestration→guard / 新增 verify/）
- 2026-06-14 42bf1d7: guard 子包从误删恢复（orchestration→guard 重命名时曾被 `rm -rf` 误操作，git 42bf1d7 提交恢复全部 7 个 .go 文件）
```

## 3. 风险与缓解

| 风险 | 缓解 |
|------|------|
| guard 误删再次发生 | d6-domain.md §历史留痕 显式标注 42bf1d7 恢复事件 |
| eval→evaluate 路径遗漏 | spec.md / design.md / layer-delta.md 三文档统一替换 + grep 验证 |
| d6-domain.md 与 D2/D4 不对称 | 直接对齐 D2/D4 章节结构 |

## 4. 不变更（边界声明）

- `internal/layers/evolution/**` 全部代码
- D6 Scenarios 行为
- D-S 编号体系（D6-S/A/F/T）
- D6 t-registry.md
