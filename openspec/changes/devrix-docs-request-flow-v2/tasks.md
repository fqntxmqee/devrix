# Tasks: 链路图文档对齐 D7 v2.0

**Change ID:** devrix-docs-request-flow-v2
**Demand ID:** DM-20260619-001

---

## W1: 重写 `docs/architecture/request-flow.md`

**目标**：QueryLoop 时代 → D7 v2.0 时代

- [ ] 更新 Header（Last Updated 2026-06-19 + 4 个代码锚点）
- [ ] §1 总览时序图：Mermaid 重画（D1→D7-S2→4 IntentPath→D3/D2/D4）
- [ ] §2 Gateway 阶段：加 `d7_enabled` 路由开关说明
- [ ] §3 Process 管线：替换为 ClassifyIntent + 4 IntentKind dispatch 表
- [ ] §4 内部循环：`QueryLoop.Run` → `turn.RunTurn` resolve/decompose + v2.0 unified 标注
- [ ] §5 LLM 调用：D7 直调 D3（`GatewayInvoker.InvokeStream` via `bridges/llm`），D2→D3 import ban
- [ ] §6 工具与权限：保持
- [ ] §7 Delegate / Worker：保持 + 加 v2.0 unified（WorkItem + RunRegistry）
- [ ] §8 配置键：加 `d7.enabled` / `workmodel.*` / `wave.*` / `turn.focus.*`
- [ ] §9 进一步阅读：code-atlas v1.2.0 + dsaft-overview v1.1.0 + d7 spec v3.8.0

## W2: 更新 `openspec/specs/architecture/code-atlas.md` v1.1.0 → v1.2.0

**目标**：D-S Index 替换为 D7 v2.0 unified

- [ ] 更新 Version（v1.1.0 → v1.2.0）+ Last Updated（2026-06-19）+ Demand（DM-20260619-001）
- [ ] 替换 D-S Index 表（19 行 → 见 design.md §2.2）
- [ ] `query_loop` / `subquery` / `sidechain_transcript` 标 **DEPRECATED**
- [ ] 加 4 Shared Contracts：RunRegistry / ResolveAwaiter / WorkItem v2 / FocusHint
- [ ] 加 2 Shared Configs：workmodel.* / wave.*
- [ ] Bootstrap Wiring 表加 `wire_coordinator.go::WireD7`
- [ ] Dependency Direction 图重画（D1→D7→D2/D3/D4）
- [ ] Test Placement 加 `turn/` 目录

## W3: 更新 `docs/architecture/dsaft-overview.md` v1.0.0 → v1.1.0

**目标**：D7 升级核心域 + 切法 A 博弈角色 IMPLEMENTED 状态

- [ ] 更新 Header（Last Updated 2026-06-19）
- [ ] §2 域架构图重画：6 域 → 7 域，D7 显式标核心域
- [ ] 域职责表：ORCH → D7 Orchestration（含 S1-S5 子层）
- [ ] §3 改为 "D2 Follower 契约"（不再是"现行重点"）
- [ ] §4 改为 "主入口路径：D7 ProcessMessage + 4 IntentKind"
- [ ] §5 加 d7 spec v3.8.0 + code-atlas v1.2.0 入口
- [ ] 加 D7 S 层 5 子层 IMPLEMENTED 状态表

## W4: 验证 + PR + 归档

- [ ] `bash scripts/verify-archive.sh openspec/changes/devrix-docs-request-flow-v2` 全部 PASS
- [ ] `go vet ./...` 0 错
- [ ] `git grep` 验证 3 文档中所有代码锚点命中
- [ ] `grep -c "coordinator.Entry\|RunTurn\|v2.0 unified\|IntentCommand\|IntentFast\|IntentOrchestrate" docs/architecture/request-flow.md` ≥ 10
- [ ] commit: `docs(architecture): align request-flow / code-atlas / dsaft-overview to D7 v2.0 (DM-20260619-001)`
- [ ] push: `git push -u origin feat/docs-request-flow-v2`
- [ ] `gh pr create --title "..." --body "..." --base master`
- [ ] `gh pr merge --auto --squash --delete-branch`（按 Devrix PR Auto-Merge 偏好）
- [ ] 合并后：`mv openspec/changes/devrix-docs-request-flow-v2 openspec/archive/2026-06-19-devrix-docs-request-flow-v2/`
- [ ] 合并后：归档目录内 `.openspec.yaml` `status: s7_archived`
- [ ] 合并后：`openspec/demand-archive-index.md` 追加新行
- [ ] 合并后：`git pull` 验证工作区干净

## 依赖关系

```
W1 ─┐
W2 ─┼─→ W4
W3 ─┘
```

W1/W2/W3 互相独立可并行；W4 依赖前三者全部完成。
