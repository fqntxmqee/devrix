# Tasks: devrix-verify-spec-links

**Change ID:** devrix-verify-spec-links
**Demand ID:** DM-20260630-010
**Status:** S4_Implementation

## Phase 1：S3 设计

- [x] T-1.1 demand.md / proposal.md / design.md 撰写

## Phase 2：S4 实现

- [ ] T-2.1 切出 `feat/devrix-verify-spec-links` 分支
- [ ] T-2.2 编写 `scripts/verify-spec-links.sh` ~80 行
- [ ] T-2.3 chmod +x
- [ ] T-2.4 验证（bash -n + 实际运行 7 个 CHANGELOG.md）

## Phase 3：S5 验收

- [ ] T-3.1 acceptance-report.md

## Phase 4：S6-交付

- [ ] T-4.1 push + PR + auto-merge

## Phase 5：S6-归档

- [ ] T-5.1 git mv changes → archive + update demand-archive-index.md
- [ ] T-5.2 push + PR + auto-merge

## Backlog（Out of Scope）

- 无（lite-mode 7 站全部 S7_Archived，本 change 是最后一个 Backlog 项目）