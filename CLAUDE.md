# Devrix — 开放的大脑

多智能体协作开发助手。6 域架构，OpenSpec S1-S6 驱动开发。

## 架构

Devrix 遵循 DSAFT 五层架构方法论（详见 `docs/methodology/dsaft-methodology.md`）。

| Domain | 域 | 目录 | DSAFT 类型 |
|----|-----|------|-------------|
| D1 | 通信层 | `internal/layers/communication/` | 核心 |
| D2 | 上下文引擎 | `internal/layers/contextengine/` | 核心 |
| D3 | LLM 网关 | `internal/layers/llmgateway/` | 公共 |
| D4 | 多智能体 | `internal/layers/multiagent/` | 核心 |
| D5 | 可观测性 | `internal/layers/observability/` | 公共 |
| D6 | 演化层 | `internal/layers/evolution/` | 支撑 |
| D7 | 编排层 | `internal/layers/orchestration/` | 核心 |

域架构归档：`openspec/specs/d{N}-*/`（spec.md + A/F/T 注册表）

## 研发流程

```
S1 需求 → S2 提案 → S3 设计 → S3-Gate(Review) → S4 实现 → S4-Gate(Review) → S5 验收 → S6 归档
```

**所有开发活动遵循 OpenSpec 规范。** 规范权威来源：`openspec/specs/project/master.md`

## 阶段→规范路由（关键）

执行任何开发任务前，先读 `openspec/specs/project/master.md` 确定当前阶段，再加载对应子规范：

| 阶段 | 加载规范 | 门禁 |
|------|---------|------|
| S1 需求 | `requirements.md` | DM ID 合法 |
| S2 提案 | `requirements.md` + `architecture-design.md` | 文件完整性 |
| S3 设计 | `architecture-design.md` | — |
| **S3-Gate** | **`review-design.md`** | 设计审查通过 |
| S4 实现 | `coding.md` + `testing.md` | go vet + test-unit |
| **S4-Gate** | **`review-code.md`** | 代码审查通过 |
| S5 验收 | `testing.md` | P0 T 层 100% + 覆盖率 ≥ 80% |
| **S6 交付** | **`git-workflow.md`** | PR 合入 master（CI + Auto-merge） |
| S6 归档 | `archiving.md` + `scripts/verify-archive.sh` | `verify-archive.sh` 全部通过 |

所有子规范路径：`openspec/specs/project/<规范名>`

## 关键约定

- **错误处理**: 使用 `internal/shared/errors/` SentinelError 模式，禁止 `panic` 用于业务错误
- **配置**: `devrix.yaml`（默认）→ `config.yaml`（本地覆盖）→ 环境变量 → CLI flags
- **不可变性**: 值对象不可变（`With*` 返回新副本）；实体通过 method 加锁变更状态。详见 `openspec/specs/project/coding.md` §9
- **文件规模**: 函数 < 50 行，文件 < 800 行
- **Git**: GitHub Flow，`feat/<change-id>` 分支，squash merge + auto-merge；**流程 SoT：`openspec/specs/project/git-workflow.md`**
- **T 层测试点**: 编号 `D{X}-S{X}-A{XX}-T{XX}`（DSAFT 标准），索引 `openspec/t-registry.md`，各域 `openspec/specs/d{N}-*/t-registry.md`
- **Change 目录**: `openspec/changes/<change-id>/`，归档到 `openspec/archive/<YYYY-MM-DD>-<change-id>/`
