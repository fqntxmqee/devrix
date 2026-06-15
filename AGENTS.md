# Devrix

> 本文件供贡献者与 IDE 工具阅读；Devrix 运行时默认只加载 `.devrix/AGENTS.md` 作为 Agent 规约。

## 研发流程

```
S1 需求 → S2 提案 → S3 设计 → S3-Gate(Review) → S4 实现 → S4-Gate(Review) → S5 验收 → S6 归档
```

**所有开发活动遵循 OpenSpec 规范。** 规范权威来源：`openspec/specs/project/master.md`

## 阶段→规范路由（关键）

执行任何开发任务前，先读 `openspec/specs/project/master.md` 确定当前阶段，再加载对应子规范：


| 阶段          | 加载规范                                         | 门禁                      |
| ----------- | -------------------------------------------- | ----------------------- |
| S1 需求       | `requirements.md`                            | DM ID 合法                |
| S2 提案       | `requirements.md` + `architecture-design.md` | 文件完整性                   |
| S3 设计       | `architecture-design.md`                     | —                       |
| **S3-Gate** | `**review-design.md`**                       | 设计审查通过                  |
| S4 实现       | `coding.md` + `testing.md`                   | go vet + test-unit      |
| **S4-Gate** | `**review-code.md`**                         | 代码审查通过                  |
| S5 验收       | `testing.md`                                 | P0 T 层 100% + 覆盖率 ≥ 80% |
| S6 归档       | `archiving.md`                               | 归档检查清单                  |


所有子规范路径：`openspec/specs/project/<规范名>`

## 关键约定

- **错误处理**: 使用 `internal/shared/errors/` SentinelError 模式，禁止 `panic` 用于业务错误
- **配置**: `devrix.yaml`（默认）→ `config.yaml`（本地覆盖）→ 环境变量 → CLI flags
- **不可变性**: 创建新对象，禁止原地修改
- **文件规模**: 函数 < 50 行，文件 < 800 行
- **Git**: GitHub Flow，`feat/<change-id>` 分支，squash merge
- **T 层测试点**: 编号 `D{X}-S{X}-T{NN}`（DSAFT 标准），注册在 `openspec/t-registry.md`
- **Change 目录**: `openspec/changes/<change-id>/`，归档到 `openspec/archive/<YYYY-MM-DD>-<change-id>/`

