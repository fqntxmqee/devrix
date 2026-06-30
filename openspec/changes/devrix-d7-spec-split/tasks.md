# Tasks: devrix-d7-spec-split

**Change ID:** devrix-d7-spec-split
**Demand ID:** DM-20260630-002
**Status:** S2_Design

## Phase 0：规范升级（与本 change 同步 PR 提交）

- [ ] **T-0.1** 更新 `openspec/specs/project/architecture-design.md` 至 v1.2.0，新增 §6.4 文档规模约束条款
- [ ] **T-0.2** 更新 `openspec/specs/project/archiving.md` 至 v1.3.0，§2.4 域文档同步表细化 + 新增 §2.5 spec.md 按 S 分片合并规则

## Phase 1：S3 设计

- [ ] **T-1.1** 编写 `design.md`（六段式），含：
  - ① 架构目标：主 ≤ 200 行 / 子 ≤ 600 行（软上限）
  - ② 架构原则：拆分粒度 = 按 S（决策已记录于 proposal.md §3.2）
  - ③ 业务流程：N/A（纯文档治理，无业务流变更）
  - ④ 领域模型：d7 各 S 域归属图（含跨 S Requirement 归属判定）
  - ⑤ 核心链路图：N/A
  - ⑥ 接口/API 设计：spec 文件间的引用契约（相对路径 + Scenario Index 段格式）
  - 附录 D：S3-Gate Review 结论
- [ ] **T-1.2** 在 design.md 附录新增"精确切分点表"（行号 + Requirement 范围 + 子文件名）
- [ ] **T-1.3** 创建 `git tag d7-spec-pre-split-2026-06-30` 备份基线

## Phase 2：S4 实现

- [ ] **T-2.1** 切出 `feat/devrix-d7-spec-split` 分支（从 origin/master）
- [ ] **T-2.2** 按 design.md 切分点表，依次 `git mv` Requirement/Scenario 段到 `spec-s{XX}.md`
  - 主 spec.md 主体内容清空，保留 Overview / DSAFT 结构 / Scenarios 总览 / Architecture / Scenario Index
  - 子文件头部模板：`# D7-S{XX} {Theme} Specification` + `## Requirement` 列表
- [ ] **T-2.3** 主 spec.md 顶部新增"## Scenario Index"段（参考 proposal.md §3.4 格式）
- [ ] **T-2.4** 主 spec.md 末尾追加 Revision History 段（参考 proposal.md §3.4 格式）
- [ ] **T-2.5** 验证：
  - `wc -l openspec/specs/d7-orchestration/spec.md` ≤ 200
  - 各 `wc -l openspec/specs/d7-orchestration/spec-s{XX}.md` ≤ 600
  - `grep -r "D7-S[0-9]*-A[0-9]*-T[0-9]*" spec-s*.md` 数量与原 spec.md 一致（T 层 ID 零损失）
  - `diff <(原 spec.md Requirement/Scenario 段) <(新 spec.md + 所有 spec-s{XX}.md Requirement/Scenario 段)` 应为空
- [ ] **T-2.6** 跑 `go vet ./...` + `go test -race ./... -short`（应 PASS，无代码变更但确保 imports 不破坏）

## Phase 3：S5 验收

- [ ] **T-3.1** 编写 `acceptance-report.md`（verdict: ACCEPTED，AC1-AC10 全列）
- [ ] **T-3.2** 跑 `./scripts/verify-archive.sh --changes devrix-d7-spec-split`（PASS）
- [ ] **T-3.3** 跑 `./scripts/test-all.sh`（P0 T 层 100% PASS + 覆盖率 ≥ 80%，实际本 change 不改 T 层，但全量回归保平安）

## Phase 4：S6-交付

- [ ] **T-4.1** `git push -u origin feat/devrix-d7-spec-split`
- [ ] **T-4.2** `gh pr create --draft --title "refactor(d7): split spec.md per S (DM-20260630-002)"`
- [ ] **T-4.3** S4 所有任务完成后 `gh pr ready`
- [ ] **T-4.4** `gh pr merge --auto --squash --delete-branch`（单人团队，0 approval）

## Phase 5：S6-归档

- [ ] **T-5.1** 从 master 切出 `feat/archive-devrix-d7-spec-split`
- [ ] **T-5.2** `mkdir -p openspec/archive/2026-06-30-devrix-d7-spec-split/` + `cp -r openspec/changes/devrix-d7-spec-split/* $ARCHIVE_DIR/`
- [ ] **T-5.3** `git rm -r openspec/changes/devrix-d7-spec-split/`
- [ ] **T-5.4** 更新 `openspec/demand-archive-index.md`（新增一行 `| DM-20260630-002 | devrix-d7-spec-split | 2026-06-30 | PR #<number> |`）
- [ ] **T-5.5** 更新 change `.openspec.yaml` `status: s7_archived`
- [ ] **T-5.6** `git commit` + `gh pr create` + `gh pr merge --auto --squash --delete-branch`

## Backlog（Out of Scope for this Change）

| 候选 Change ID | 范围 | 状态 |
|----------------|------|------|
| `devrix-d7-design-split` | d7-orchestration/design.md 841 行拆分 | 待立项 |
| `devrix-d7-tregistry-split` | d7-orchestration/t-registry.md 1133 行拆分 | 待立项 |
| `devrix-d2-spec-split` | d2-context-engine/spec.md 1622 行拆分 | 待立项 |
| `devrix-d3-spec+design-split` | d3-llm-gateway/spec.md 1060 + design.md 1042 行拆分 | 待立项 |
| `devrix-d4-design-split` | d4-multi-agent/design.md 1064 行拆分 | 待立项 |
| `devrix-verify-spec-links` | CI 工具：检查 spec.md / spec-s{XX}.md 链接有效性 | 待立项 |
