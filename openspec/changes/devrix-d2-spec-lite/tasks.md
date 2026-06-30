# Tasks: devrix-d2-spec-lite

**Change ID:** devrix-d2-spec-lite
**Demand ID:** DM-20260630-004
**Status:** S2_Design

## Phase 1：S3 设计

- [x] **T-1.1** 编写 `design.md`（六段式），含：
  - ① 架构目标：d2 spec.md ≤ 200 + CHANGELOG.md ≤ 300
  - ② 架构原则：复用 d7 lite-mode 模式
  - ③ 业务流程：精简模式 end-to-end（写代码 → 改 spec.md → 改 CHANGELOG.md → 归档）
  - ④ 领域模型：d2 spec.md 4 段结构 + CHANGELOG.md 表格结构 + d2-domain.md v9.0.0 SoT
  - ⑤ 核心链路图：读路径（spec.md → d2-domain.md → CHANGELOG.md → archive/）写路径（change → spec.md 修订 + CHANGELOG.md 追加）
  - ⑥ 接口/API：spec.md 顶部契约段 + CHANGELOG.md 行格式
  - 附录 A File Manifest / B Rollback / C 回归风险 / D S3-Gate 自评 / E 下一步
- [x] **T-1.2** design.md 附录 A 列出本 change 文件清单（d2 spec.md 重写 + d2 CHANGELOG.md NEW + 11 个 d2 子文档不动）

## Phase 2：S4 实现

- [ ] **T-2.1** 切出 `feat/devrix-d2-spec-lite` 分支（从 origin/master）
- [ ] **T-2.2** 重写 `openspec/specs/d2-context-engine/spec.md` 为精简设计契约
  - 保留行 1-37 段（Overview + v1-v8 历史摘要 → 精简为 1 段 + d2-domain.md 引用）
  - 删除行 38-1622 段（66 Requirement / 96 Scenario 详细文本，留在 archive/）
  - 添加 "## 核心设计原则" 段（5-8 条）
  - 添加 "## S 层职责" 段（canonical S15-S18 表格）
  - 添加 "## DSAFT 结构" 段（计数表）
  - 添加 "## Scenarios" 段（4 canonical S 状态表）
  - 添加 "## Architecture" 段（Leader/Follower 拓扑 + d7-boundary.md 引用）
  - 添加 "## 关键 Scenario 范式" 段（1-2 个典型 Gherkin 示例，如 S15 PrepareExecutionContext）
  - 添加 "## 关键链路口" 段（4-6 端到端路径）
  - 顶部声明：d2-domain.md v9.0.0 是 SoT，spec.md 是契约
  - 目标 ≤ 200 行
- [ ] **T-2.3** 新建 `openspec/specs/d2-context-engine/CHANGELOG.md`
  - 顶部说明：lite-mode 时间线，CHANGELOG.md 不复制 Scenario 文本
  - 表格：Date / Change ID / 摘要 / 归档链接（4 列）
  - 列出最近 30 天 28 条 d2 change（2026-06-01 到 2026-06-30）
  - 底部总览：当前活跃 Requirement 数 + 历史 Scenario 留 archive
  - 目标 ≤ 300 行
- [ ] **T-2.4** 验证：
  - `wc -l openspec/specs/d2-context-engine/spec.md` ≤ 200
  - `wc -l openspec/specs/d2-context-engine/CHANGELOG.md` ≤ 300
  - `git diff --stat openspec/specs/d2-context-engine/`（除 spec.md + 新增 CHANGELOG.md）= 0（11 个子文档全部不动）
  - `git diff --stat openspec/specs/d{1,3,4,5,6}-*/` （其他域）= 0
  - `git diff --stat internal/` = 0（不改 Go 代码）
  - 跨 archive/ 全局 grep `#### Scenario:` 总数 = 96（0 丢失）
  - `go vet ./...` PASS
  - `go test -race ./... -short` PASS（本 change 不改 Go 代码）

## Phase 3：S5 验收

- [ ] **T-3.1** 编写 `acceptance-report.md`（verdict: ACCEPTED，AC1-AC12 全列）
- [ ] **T-3.2** 跑 `./scripts/verify-archive.sh --changes devrix-d2-spec-lite`（PASS 或预期内 S5 阶段 fail）
- [ ] **T-3.3** 跑 `./scripts/test-all.sh`（P0 T 层 100% PASS + 覆盖率 ≥ 80%，本 change 不改 T 层，全量回归保平安）

## Phase 4：S6-交付

- [ ] **T-4.1** `git push -u origin feat/devrix-d2-spec-lite`
- [ ] **T-4.2** `gh pr create --draft --title "refactor(docs): d2 spec lite-mode + d2 CHANGELOG (DM-20260630-004)"`
- [ ] **T-4.3** S4 所有任务完成后 `gh pr ready`
- [ ] **T-4.4** `gh pr merge --auto --squash --delete-branch`（单人团队，0 approval）

## Phase 5：S6-归档（另开 PR）

- [ ] **T-5.1** 从 master 切出 `feat/archive-devrix-d2-spec-lite`
- [ ] **T-5.2** `mkdir -p openspec/archive/2026-06-30-devrix-d2-spec-lite/` + `cp -r openspec/changes/devrix-d2-spec-lite/* $ARCHIVE_DIR/`
- [ ] **T-5.3** `git rm -r openspec/changes/devrix-d2-spec-lite/`
- [ ] **T-5.4** 更新 `openspec/demand-archive-index.md`（新增 `| DM-20260630-004 | devrix-d2-spec-lite | 2026-06-30 | PR #<number> |`）
- [ ] **T-5.5** 更新 change `.openspec.yaml` `status: s7_archived`
- [ ] **T-5.6** `git commit` + `gh pr create` + `gh pr merge --auto --squash --delete-branch`
- [ ] **T-5.7** 跑 `./scripts/verify-archive.sh devrix-d2-spec-lite`（PASS，AC10 验证）

## Backlog（Out of Scope for this Change）

| 候选 Change ID | 范围 | 状态 |
|----------------|------|------|
| `devrix-d1-spec-lite` | d1-communication/spec.md 577 行精简 | 待立项（lite-mode 推广第二站） |
| `devrix-d3-spec-lite` | d3-llm-gateway/spec.md 1060 行精简 | 待立项 |
| `devrix-d4-spec-lite` | d4-multi-agent/spec.md 222 行（已合格）/design.md 1064 行精简 | 待立项 |
| `devrix-d7-design-split` | d7-orchestration/design.md 841 行拆分 | 待立项 |
| `devrix-d7-tregistry-split` | d7-orchestration/t-registry.md 1133 行拆分 | 待立项 |
| `devrix-d2-design-split` | d2-context-engine/design.md（待量）拆分 | 待立项 |
| `devrix-d2-tregistry-split` | d2-context-engine/t-registry.md（待量）拆分 | 待立项 |
| `devrix-verify-spec-links` | CI 工具：检查 CHANGELOG.md 链接有效性 | 待立项 |