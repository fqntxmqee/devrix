# Tasks: D3 spec 路径与 v2.0 状态同步

**Change ID:** devrix-spec-sync-d3-package-map
**Demand ID:** DM-20260619-002

---

## W1: 更新 `openspec/specs/d3-llm-gateway/design.md` v3.2.0 → v3.3.0

**目标**：v2.0 物理路径迁移状态同步 + 路径前缀修正

- [ ] Version: v3.2.0 → **v3.3.0**
- [ ] Last Updated: 2026-06-14 → **2026-06-19**
- [ ] Demand 标注：DM-20260619-002
- [ ] §5.2 路径：`internal/shared/config/llmgateway.go` → `internal/layers/llmgateway/configure/shared_config.go`
- [ ] §10.2 状态：`Phase F 实施中` → `✅ 已完成，DM-20260614-019, 2026-06-14`
- [ ] §Revision History 追加 3.3.0 行

## W2: 更新 `openspec/specs/d3-llm-gateway/model-resolution-trace.md`

**目标**：v2.0 路径状态同步

- [ ] Last Updated: 2026-06-14 → **2026-06-19**
- [ ] 头部 v2.0 状态注释：标注 DM-20260614-019 落地，import 路径变更（gateway/router → route/router；gateway/gateway → stream/gateway；adapter/openai_* → stream/adapter/openai_*；shared/config/llmgateway → layers/llmgateway/configure/shared_config）
- [ ] Tier 解析逻辑保持 `Router.Resolve()` + `Router.ResolveTier()` 不变（仅路径迁移）

## W3: 验证 + PR + 归档

- [ ] `bash scripts/verify-archive.sh openspec/changes/devrix-spec-sync-d3-package-map` 全部 PASS（docs-only 允许 WARN）
- [ ] `go vet ./...` 0 错
- [ ] `git grep` 验证新路径 `internal/layers/llmgateway/configure/shared_config.go` 命中
- [ ] commit: `docs(d3-spec): sync v2.0 physical paths and status (DM-20260619-002)`
- [ ] push: `git push -u origin feat/devrix-spec-sync-d3-package-map`
- [ ] `gh pr create --title "..." --body "..." --base master`
- [ ] `gh pr merge --auto --squash --delete-branch`（Devrix PR Auto-Merge 偏好）
- [ ] 合并后：`mv openspec/changes/devrix-spec-sync-d3-package-map openspec/archive/2026-06-19-devrix-spec-sync-d3-package-map/`
- [ ] 合并后：`.openspec.yaml` `status: s7_archived`
- [ ] 合并后：`openspec/demand-archive-index.md` 追加新行（DM-20260619-002 → archive）
- [ ] 合并后：`git pull` 验证工作区干净

## 依赖关系

```
W1 ─┐
W2 ─┼─→ W3
```

W1/W2 互相独立可并行；W3 依赖前两者全部完成。