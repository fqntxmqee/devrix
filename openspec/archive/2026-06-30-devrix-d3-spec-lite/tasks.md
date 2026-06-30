# Tasks: devrix-d3-spec-lite

**Change ID:** devrix-d3-spec-lite
**Demand ID:** DM-20260630-006
**Status:** S2_Design

## Phase 1：S3 设计

- [x] **T-1.1** 编写 `design.md`（六段式），含：
  - ① 架构目标：d3 spec.md ≤ 200 + CHANGELOG.md ≤ 300
  - ② 架构原则：复用 d7/d2/d1 lite-mode 模式
  - ③ 业务流程：精简模式 end-to-end
  - ④ 领域模型：d3 spec.md 8 段结构 + CHANGELOG.md 表格结构 + d3-domain.md v1.6.0 SoT
  - ⑤ 核心链路图：读路径（spec.md → d3-domain.md → dsaf-architecture.md → CHANGELOG.md → archive/）
  - ⑥ 接口/API：spec.md 顶部契约段 + CHANGELOG.md 行格式
  - 附录 A File Manifest / B Rollback / C 回归风险 / D S3-Gate 自评 / E 下一步
- [x] **T-1.2** design.md 附录 A 列出本 change 文件清单（d3 spec.md 重写 + d3 CHANGELOG.md NEW + 11 个 d3 子文档不动）

## Phase 2：S4 实现

- [ ] **T-2.1** 切出 `feat/devrix-d3-spec-lite` 分支（从 origin/master）
- [ ] **T-2.2** 重写 `openspec/specs/d3-llm-gateway/spec.md` 为精简设计契约
  - 删除行 1-30 §0/§0.1 变更摘要
  - 删除行 31-100 §1/§2 North Star + DSAFT 结构详细
  - 删除行 ~85-1000 6 个 S 段（D3-S1..S6 + §9 CROSS）的 Feature + Scenario 详细文本
  - 删除行 ~1000+ 5 个 ADDED Requirements 段（§10..§14 V2/V3/V3.1/V4 跨域灰区/韧性可见性）
  - 删除 §15 Archive 段（链接到 CHANGELOG.md）
  - 顶部声明：d3-domain.md v1.6.0 是 SoT + guides 引用
  - 添加 "## Overview" 段（D3 LLM 网关 公共域 + 5 承诺 C1-C5 + 1 横切 + Bridge 跨域锚点）
  - 添加 "## 核心设计原则" 段（7-8 条：承诺装置 + 跨域锚点 + Tier 解析 + 灰区声明 + 启动 fail-fast + Breaker/Retry 合并 + 运行时 span 保持 + BREAKING 显式）
  - 添加 "## S 层职责" 段（canonical 6 + 1 CROSS 表格）
  - 添加 "## DSAFT 结构" 段（计数表 1 D + 7 S + A + F + T + Span ops）
  - 添加 "## Scenarios" 段（6 canonical S + 1 CROSS 状态表）
  - 添加 "## Architecture" 段（Adapter / Gateway / Breaker / Retry / Budget / Safety / Config + bridges/llm 引用）
  - 添加 "## 关键 Scenario 范式" 段（1 canonical: D3-S3 ProtectCall Breaker Open 路径）
  - 添加 "## 关键链路口" 段（4-6 端到端路径）
  - 目标 ≤ 200 行
- [ ] **T-2.3** 新建 `openspec/specs/d3-llm-gateway/CHANGELOG.md`
  - 顶部说明：lite-mode 时间线，CHANGELOG.md 不复制 Scenario 文本
  - 表格：Date / Change ID / 摘要 / 归档链接（4 列）
  - 列出最近 30 天 6+ d3 change（2026-06-07 到 2026-06-30）
  - 底部总览：当前活跃 Requirement 数 + 90 Scenario 5 类分布（happy 30 / sad 24 / boundary 18 / concurrent 9 / timeout 9）
  - 目标 ≤ 300 行
- [ ] **T-2.4** 验证：
  - `wc -l openspec/specs/d3-llm-gateway/spec.md` ≤ 200
  - `wc -l openspec/specs/d3-llm-gateway/CHANGELOG.md` ≤ 300
  - `git diff --stat openspec/specs/d3-llm-gateway/`（除 spec.md + 新增 CHANGELOG.md）= 0（11 个子文档全部不动）
  - `git diff --stat openspec/specs/d{1,2,4,5,6,7}-*/` （其他域）= 0
  - `git diff --stat internal/` = 0（不改 Go 代码）
  - 跨 archive/ 全局 grep `#### Scenario:` 总数 = 90（0 丢失）
  - `go vet ./...` PASS
  - `go test -race ./... -short` PASS（本 change 不改 Go 代码）

## Phase 3：S5 验收

- [ ] **T-3.1** 编写 `acceptance-report.md`（verdict: ACCEPTED，AC1-AC12 全列）
- [ ] **T-3.2** 跑 `./scripts/verify-archive.sh --changes devrix-d3-spec-lite`（PASS 或预期内 WARN：proposal.md 状态标记，与 d7/d2/d1 spec-lite 同）
- [ ] **T-3.3** 跑 `./scripts/test-all.sh`（P0 T 层 100% PASS + 覆盖率 ≥ 80%，本 change 不改 T 层，全量回归保平安）

## Phase 4：S6-交付

- [ ] **T-4.1** `git push -u origin feat/devrix-d3-spec-lite`
- [ ] **T-4.2** `gh pr create --draft --title "refactor(docs): d3 spec lite-mode + d3 CHANGELOG (DM-20260630-006)"`
- [ ] **T-4.3** S4 所有任务完成后 `gh pr ready`
- [ ] **T-4.4** `gh pr merge --auto --squash --delete-branch`（单人团队，0 approval）

## Phase 5：S6-归档（另开 PR）

- [ ] **T-5.1** 从 master 切出 `feat/archive-devrix-d3-spec-lite`
- [ ] **T-5.2** `mkdir -p openspec/archive/2026-06-30-devrix-d3-spec-lite/` + `cp -r openspec/changes/devrix-d3-spec-lite/* $ARCHIVE_DIR/`
- [ ] **T-5.3** `git rm -r openspec/changes/devrix-d3-spec-lite/`
- [ ] **T-5.4** 更新 `openspec/demand-archive-index.md`（新增 `| DM-20260630-006 | devrix-d3-spec-lite | 2026-06-30 | PR #<number> |`）
- [ ] **T-5.5** 更新 change `.openspec.yaml` `status: s7_archived`
- [ ] **T-5.6** `git commit` + `gh pr create` + `gh pr merge --auto --squash --delete-branch`
- [ ] **T-5.7** 跑 `./scripts/verify-archive.sh devrix-d3-spec-lite`（PASS，AC10 验证）

## Backlog（Out of Scope for this Change）

| 候选 Change ID | 范围 | 状态 |
|----------------|------|------|
| `devrix-d4-spec-lite` | d4-multi-agent/spec.md 222 行（已合格）/ design.md 1064 行精简 | 待立项 |
| `devrix-d5-spec-lite` | d5-observability/spec.md（待量化） | 待立项 |
| `devrix-d6-spec-lite` | d6-evolution/spec.md（待量化） | 待立项 |
| `devrix-d3-design-split` | d3-llm-gateway/design.md 1042 行拆分 | 待立项 |
| `devrix-d3-tregistry-split` | d3-llm-gateway/t-registry.md 296 行拆分（勉强合格） | 待立项 |
| `devrix-verify-spec-links` | CI 工具：检查 CHANGELOG.md 链接有效性 | 待立项 |
