# Design: 架构分层命名迁移 L1-X → D1-D6

**Change ID:** devrix-d-layer-rename
**Demand ID:** DM-20260608-007
**Status:** Delivered

---

## 1. 命名映射

| Legacy | New | 缩写 |
|--------|-----|------|
| L1-1 | D1 | COMM |
| L1-2 | D2 | CTX |
| L1-3 | D3 | LLM |
| L1-4 | D4 | AGENT |
| L1-5 | D5 | OBS |
| L1-6 | D6 | EVO |
| L1-X-L2-Y | D{X}-S{Y} | — |

L5 ID 字符串不变：`L5-2-3-01` 等编号保持原样，格式说明改为 `L5-{D}-{S}-{NN}`。

## 2. 文档策略

- **重写** `openspec/specs/architecture/layering.md` 为 D-S 规范 v2.0.0
- **删除** `layering-v2.md`、`layering-standard.md`、`MIGRATION.md`
- **批量更新** `project.md`、`l5-registry.md`、`specs/project/*`、入口文件、layer_delta 标题
- **不修改** `internal/`、`devrix.yaml`、`openspec/archive/`

## 3. 验证

全项目 grep `L1-\d`（排除 archive、本 change 目录、layering.md Legacy 表）应无残留。
