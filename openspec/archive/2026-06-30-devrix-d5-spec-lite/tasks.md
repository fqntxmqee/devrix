# Tasks: devrix-d5-spec-lite

**Change ID:** devrix-d5-spec-lite
**Demand ID:** DM-20260630-008
**Status:** S4_Implementation

## Phase 1：S3 设计

- [x] T-1.1 demand.md / proposal.md / design.md 撰写

## Phase 2：S4 实现

- [ ] T-2.1 切出 `feat/devrix-d5-spec-lite` 分支
- [ ] T-2.2 重写 `spec.md` 376 → ≤ 200（13 条 Requirements 详细 Gherkin → 1 行 reference + archive/）
- [ ] T-2.3 新建 `CHANGELOG.md` ≤ 300
- [ ] T-2.4 验证（wc -l / go vet / 0 Go diff / 0 d5 子文档 diff）

## Phase 3：S5 验收

- [ ] T-3.1 acceptance-report.md
- [ ] T-3.2 verify-archive.sh

## Phase 4：S6-交付

- [ ] T-4.1 push + PR + auto-merge

## Phase 5：S6-归档

- [ ] T-5.1 git mv changes → archive + update demand-archive-index.md
- [ ] T-5.2 push + PR + auto-merge

## Backlog（Out of Scope）

- devrix-d5-design-split（design.md 状态）
- devrix-d6-spec-lite（d6 spec.md 604 行）
- devrix-verify-spec-links（CI 工具）