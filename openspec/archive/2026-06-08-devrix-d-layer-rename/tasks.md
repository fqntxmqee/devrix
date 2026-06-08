# Tasks: 架构分层命名迁移

**Change ID:** devrix-d-layer-rename
**Status:** Completed
**估算总工时:** 4h
**阶段数:** 6

---

## T1: 重写 layering.md，合并冗余文档 (0.5h)

- [x] 重写 `openspec/specs/architecture/layering.md`
  - D-S 两层定义（D1-D6 域 + 各域 S 场景表）
  - L5 ID 编号规范（`L5-{D}-{S}-{NN}`）
  - 代码目录映射
  - 废弃记录（L1-L2 方案、D-S-A-F-T 方案的历史）
- [x] 删除 `openspec/specs/architecture/layering-v2.md`
- [x] 删除 `openspec/specs/architecture/layering-standard.md`
- [x] 删除 `openspec/specs/architecture/MIGRATION.md`

## T2: 更新 project.md (0.5h)

- [x] 所有表格 L1-X → D1-D6
- [x] 所有表格 L1-X-L2-Y → D{X}-S{Y}
- [x] 目录树注释 `# L1-X` → `# D{X}`
- [x] L2 ID 列 → Module ID
- [x] Layering Spec 引用路径确认正确

## T3: 更新 l5-registry.md (0.5h)

- [x] 编号格式说明 `L5-{L1}-{L2}-{NN}` → `L5-{D}-{S}-{NN}`
- [x] 6 个 L1 节标题 `## L1-X: ...` → `## D{X}: ... Domain`
- [x] 所有 L2 节标题 `### L1-X-L2-Y: ...` → `### D{X}-S{Y}: ...`
- [x] 列头 `L2 映射` → `S 映射`
- [x] 摘要表 `L1` → `Domain`
- [x] ID 字符串不变（76 个 L5 ID 保持原样）

## T4: 更新项目规范 specs/project/ (0.5h)

- [x] `master.md` — 提交格式说明、L5 编号格式说明
- [x] `coding.md` — 目录树 `# L1-X` → `# D{X}`、检查清单
- [x] `architecture-design.md` — `.openspec.yaml` 模板 `domains: [D1, ...]`
- [x] `review-design.md` — "L1-L2 层" → "D-S 层"
- [x] `review-code.md` — "L1-L2 目录" → "D-S 目录"
- [x] `testing.md` — 无 L1 引用（确认）
- [x] `archiving.md` — 无 L1 引用（确认）

## T5: 更新入口文件 (0.5h)

- [x] `AGENTS.md` — 架构表、架构描述、L5 格式
- [x] `CLAUDE.md` — 同上
- [x] `GEMINI.md` — 同上
- [x] `.cursor/rules/spec-routing.mdc` — 架构描述、L5 格式

## T6: 更新外围文档 + 验证 (1.5h)

- [x] 6 个 `openspec/specs/*_layer_delta.md` — 标题 `Layer X` → `Domain D{X}`
- [x] `openspec/changes/devrix-v3/design.md` — `Layer: 1` → `Domain: D1`
- [x] `openspec/changes/devrix-v3/tasks.md` — 同上
- [x] 全项目 grep 验证无残留 `L1-\d` 引用（排除 archive）
- [x] `openspec/specs/testing-quality/spec.md` — D-S 命名同步

---

## 依赖关系

```
T1（layering.md 重写，定义新命名标准）
  ├── T2（project.md 按新标准更新）
  ├── T3（l5-registry.md 按新标准更新）
  │     └── T4（specs/project/ 更新）
  │           └── T5（入口文件更新）
  └── T6（外围文档 + 验证）
```

T1 必须先完成（定义标准），T2-T5 可并行，T6 最后收尾。
