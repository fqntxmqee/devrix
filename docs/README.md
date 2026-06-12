# Devrix 架构文档

## 文档体系

| 文档 | 说明 |
|------|------|
| [detail design framework.md](./detail%20design%20framework.md) | **详细设计六段式框架**（①目标 → ⑥接口） |
| [context-engine-design.md](./context-engine-design.md) | 上下文引擎 Layer 2 详细设计（按框架展开） |

## OpenSpec 映射

| 层级 | OpenSpec 路径 |
|------|---------------|
| 项目元数据 | `openspec/project.md` |
| 层 Delta | `openspec/specs/*_layer_delta.md` |
| 变更设计 | `openspec/changes/{slug}/` |
| T 层注册表 | `openspec/t-registry.md` |
| 测试框架 | `openspec/specs/testing-framework/spec.md` |

## 新增模块设计流程

1. 按 `detail design framework.md` 六段式编写 `docs/{module}-design.md`
2. 在 `openspec/changes/{slug}/` 创建 demand / proposal / design / specs / tasks
3. 在 `openspec/t-registry.md` 登记 T 层测试点
4. S4 开发 → S5 `gen-acceptance-report.sh` 验收
