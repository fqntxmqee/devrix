# Tasks: devrix-spec-lite-mode

**Change ID:** devrix-spec-lite-mode
**Demand ID:** DM-20260630-003
**Status:** S2_Design
**Replaces:** devrix-d7-spec-split (DM-20260630-002, s1_cancelled)

## Phase 1：S3 设计

- [ ] **T-1.1** 编写 `design.md`（六段式），含：
  - ① 架构目标：d7 spec.md ≤ 200 + CHANGELOG.md ≤ 300
  - ② 架构原则：specs = 精简契约 + 轻量 changelog
  - ③ 业务流程：精简模式 end-to-end（写代码 → 改 spec.md → 改 CHANGELOG.md → 归档）
  - ④ 领域模型：d7 spec.md 4 段结构 + CHANGELOG.md 表格结构
  - ⑤ 核心链路图：读路径（spec.md → CHANGELOG.md → archive/）写路径（change → spec.md 修订 + CHANGELOG.md 追加）
  - ⑥ 接口/API：spec.md 顶部 "Recent Changes" 段契约 + CHANGELOG.md 行格式
  - 附录 A File Manifest / B Rollback / C 回归风险 / D S3-Gate 自评
- [ ] **T-1.2** design.md 附录 A 列出本 change 文件清单（规范升级 + d7 spec.md 重写 + 新增 CHANGELOG.md + devrix-d7-spec-split 标 s1_cancelled）

## Phase 2：S4 实现

- [ ] **T-2.1** 切出 `feat/devrix-spec-lite-mode` 分支（从 origin/master）
- [ ] **T-2.2** 修改 `openspec/specs/project/architecture-design.md` v1.2.0 → v1.3.0
  - §6.4 重写：spec.md ≤ 200 / CHANGELOG.md ≤ 300 / 其他 d{N} 子文档 ≤ 800
  - 删除"按 S 分片"硬要求
  - 新增"specs = 精简设计契约 + 轻量 changelog"模式
  - 拆分原则改为：spec.md 为主索引，CHANGELOG.md 为时间线，archive/ 为历史
  - S3-Gate 检查项改为：spec.md ≤ 200 / CHANGELOG.md ≤ 300
- [ ] **T-2.3** 修改 `openspec/specs/project/archiving.md` v1.3.0 → v1.4.0
  - §2.4 改为：Scenario 留在 archive/<change>/specs/，不合并到域 spec.md
  - §2.5 删除（按 S 分片合并强制规则废弃）
  - 新增 §2.4 域文档同步规则：仅 spec.md 修订 + CHANGELOG.md 追加（按需）
- [ ] **T-2.4** 修改 `openspec/changes/devrix-d7-spec-split/.openspec.yaml`
  - `status: s1_cancelled`
  - 新增 `cancelled_reason` / `cancelled_at` / `replaced_by` 字段
- [ ] **T-2.5** 重写 `openspec/specs/d7-orchestration/spec.md` 为精简设计契约
  - 保留行 1-164 段（Overview / DSAFT / Scenarios / Architecture + 域边界）
  - 删除 165-2622 段（63 Requirement / 174 Scenario 详细文本，留在 archive/）
  - 添加 "## Recent Changes" 段（指向 CHANGELOG.md）
  - 添加 "## 关键 Scenario 范式" 段（1-2 个典型 Gherkin 示例）
  - 目标 ≤ 200 行
- [ ] **T-2.6** 新建 `openspec/specs/d7-orchestration/CHANGELOG.md`
  - 顶部说明
  - 表格：Date / Change ID / 摘要 / 状态 / 归档链接
  - 列出最近 30 天 ≥ 10 条 d7 change
  - 底部总览：当前活跃 Requirement 数 + 历史 Scenario 留 archive
  - 目标 ≤ 300 行
- [ ] **T-2.7** 验证：
  - `wc -l openspec/specs/d7-orchestration/spec.md` ≤ 200
  - `wc -l openspec/specs/d7-orchestration/CHANGELOG.md` ≤ 300
  - `git diff --stat openspec/specs/d7-orchestration/`（除 spec.md + 新增 CHANGELOG.md）= 0
  - `git diff --stat openspec/specs/d{1..6}-*/` （其他域）= 0
  - 跨 archive/ 全局 grep `#### Scenario:` 总数 = 174
  - `go vet ./...` PASS
  - `go test -race ./... -short` PASS（本 change 不改 Go 代码）
  - 规范版本号自检：architecture-design.md 头 v1.3.0 / archiving.md 头 v1.4.0

## Phase 3：S5 验收

- [ ] **T-3.1** 编写 `acceptance-report.md`（verdict: ACCEPTED，AC1-AC12 全列）
- [ ] **T-3.2** 跑 `./scripts/verify-archive.sh --changes devrix-spec-lite-mode`（PASS 或预期内 S5 阶段 fail）
- [ ] **T-3.3** 跑 `./scripts/test-all.sh`（P0 T 层 100% PASS + 覆盖率 ≥ 80%，本 change 不改 T 层，全量回归保平安）

## Phase 4：S6-交付

- [ ] **T-4.1** `git push -u origin feat/devrix-spec-lite-mode`
- [ ] **T-4.2** `gh pr create --draft --title "refactor(docs): specs lite-mode + d7 CHANGELOG (DM-20260630-003)"`
- [ ] **T-4.3** S4 所有任务完成后 `gh pr ready`
- [ ] **T-4.4** `gh pr merge --auto --squash --delete-branch`（单人团队，0 approval）

## Phase 5：S6-归档（另开 PR）

- [ ] **T-5.1** 从 master 切出 `feat/archive-devrix-spec-lite-mode`
- [ ] **T-5.2** `mkdir -p openspec/archive/2026-06-30-devrix-spec-lite-mode/` + `cp -r openspec/changes/devrix-spec-lite-mode/* $ARCHIVE_DIR/`
- [ ] **T-5.3** `git rm -r openspec/changes/devrix-spec-lite-mode/`
- [ ] **T-5.4** 更新 `openspec/demand-archive-index.md`（新增 `| DM-20260630-003 | devrix-spec-lite-mode | 2026-06-30 | PR #<number> |`）
- [ ] **T-5.5** 更新 change `.openspec.yaml` `status: s7_archived`
- [ ] **T-5.6** `git commit` + `gh pr create` + `gh pr merge --auto --squash --delete-branch`
- [ ] **T-5.7** 跑 `./scripts/verify-archive.sh devrix-spec-lite-mode`（PASS，AC12 验证）

## Backlog（Out of Scope for this Change）

| 候选 Change ID | 范围 | 状态 |
|----------------|------|------|
| `devrix-d7-design-split` | d7-orchestration/design.md 841 行拆分 | 待立项（lite-mode 后设计文档维护压力降低，可观察） |
| `devrix-d7-tregistry-split` | d7-orchestration/t-registry.md 1133 行拆分 | 待立项 |
| `devrix-d1-spec-lite` | d1-communication/spec.md 577 行精简 | 待立项（lite-mode 推广） |
| `devrix-d2-spec-lite` | d2-context-engine/spec.md 1622 行精简 | 待立项（lite-mode 推广） |
| `devrix-d3-spec-lite` | d3-llm-gateway/spec.md 1060 行精简 | 待立项（lite-mode 推广） |
| `devrix-d4-spec-lite` | d4-multi-agent/spec.md 222 行（已合格）/design.md 1064 行精简 | 待立项 |
| `devrix-verify-spec-links` | CI 工具：检查 CHANGELOG.md 链接有效性 | 待立项 |
