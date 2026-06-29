# Devrix 研发流程规范

> **规范权威来源：** `openspec/specs/project/master.md` — 本文件为辅助阅读，冲突时以 `openspec/specs/project/` 为准。

**版本:** 1.1.0（与 master.md v1.3.0 对齐）
**最后更新:** 2026-06-26

---

## 一、总览

Devrix 研发流程由两条主线交织而成：

| 主线 | 规范 | 工具 |
|------|------|------|
| 需求到验收 | OpenSpec 六阶段 (S1-S6) | 文件体系 (`openspec/`) |
| 代码协作 | GitHub Flow | `gh` CLI |

```
S1 需求 → S2 提案 → S3 设计 → S3-Gate → S4 实现 → S4-Gate → S5 验收 → S6-交付 → S6-归档
     │         │          │         │          │         │          │           │          │
  demand   branch+    design+    Review    commits+   Review   acceptance  PR merge   archive/
           proposal   specs/     通过      tasks.md    通过      report      master
```

---

## 二、分支策略

采用 **GitHub Flow**（单一主干 + 短期功能分支）。**本仓库生产分支为 `master`**：

```
master
  └── feat/<change-id>          # 功能分支（change-id 含 devrix- 前缀）
  └── fix/<change-id>           # 修复分支
  └── docs/<description>        # 文档分支
```

**规则：**
- `master` 始终保持可部署状态
- 一个 Change 一个分支，分支名与 Change ID 对应
- 分支从 `origin/master` 拉出，合并回 `master`
- 合并前必须通过 CI 门禁

```bash
# 创建功能分支
git checkout master
git pull origin master
git checkout -b feat/devrix-tool-security

# 推送并设置 upstream
git push -u origin feat/devrix-tool-security
```

---

## 三、OpenSpec 六阶段 + GitHub 操作

### S1: Demand（需求）

**目的：** 将问题陈述转化为结构化需求

**产出文件：**
```
openspec/changes/<change-id>/
  └── demand.md              # 需求文档（可选，轻量变更可跳过）
```

**操作流程：**

1. 在 `openspec/demand-archive-index.md` 分配 Demand ID（格式: `DM-YYYYMMDD-NNN`）
2. 创建 `demand.md`，包含：背景、问题陈述、成功指标、范围边界

**GitHub 操作：**

```bash
# 创建 GitHub Issue 追踪需求
gh issue create \
  --title "[<change-id>] <需求简述>" \
  --body "$(cat <<'EOF'
## 背景
...

## 成功指标
...

## 关联文档
- openspec/changes/<change-id>/
EOF
)" \
  --label "demand,<layer>"
```

---

### S2: Proposal（提案）

**目的：** 分析问题根因，提出解决方案，评估风险

**产出文件：**
```
openspec/changes/<change-id>/
  ├── .openspec.yaml          # 元数据：优先级、T层测试点、关联层
  └── proposal.md             # 提案：背景、方案、任务估算、风险
```

**`.openspec.yaml` 模板：**（完整字段见 `openspec/specs/project/architecture-design.md` §2）

```yaml
change_id: devrix-{module-name}
priority: P0 | P1 | P2
demand_id: DM-YYYYMMDD-NNN
status: s2_design
domains: [D1, D2, ...]
dsaft_scenarios: [D{X}-S{X}, ...]
dsaft_activities: [D{X}-S{X}-A{XX}, ...]
t_points: [D{X}-S{X}-A{XX}-T{XX}, ...]
```

> **禁止**在 proposal/design 中写工时估算；估算仅可在 `tasks.md` 作参考值。

**操作流程：**

1. 创建分支（如尚未创建）
2. 编写 `.openspec.yaml` 和 `proposal.md`
3. 在 `openspec/t-registry.md` 预登记 T 层测试点（状态: `PLANNED`）

**GitHub 操作：**

```bash
# 创建分支
git checkout -b feat/<change-id>
git push -u origin feat/<change-id>

# 提交 proposal
git add openspec/changes/<change-id>/
git add openspec/t-registry.md
git add openspec/demand-archive-index.md
git commit -m "$(cat <<'EOF'
proposal: <change-id> 提案

S2 阶段，包含问题分析、方案设计、任务估算。
EOF
)"

# 推送
git push
```

---

### S3: Design（设计）

**目的：** 详细技术设计，包含代码级方案

**产出文件：**
```
openspec/changes/<change-id>/
  └── design.md               # 技术设计：根因、方案、关键代码、文件清单、回归风险
  └── specs/
      └── <module>/
          └── spec.md         # Gherkin 规格（Feature + Scenario）
```

**操作流程：**

1. 编写 `design.md`（**六段式**，见 `docs/methodology/detail-design-framework.md`）
2. 编写 `specs/<module>/spec.md`（Gherkin 场景）
3. S3-Gate：按 `openspec/specs/project/review-design.md` 审查；结论写入 design.md 附录 D

**GitHub 操作：**

```bash
git add openspec/changes/<change-id>/design.md
git add openspec/changes/<change-id>/specs/
git commit -m "$(cat <<'EOF'
design: <change-id> 技术设计

S3 阶段，包含根因分析、修复方案、Gherkin 规格。
EOF
)"
git push
```

**此时创建 Draft PR 供方案评审：**

```bash
gh pr create \
  --title "<change-id>: <简短描述>" \
  --body "$(cat <<'EOF'
## 变更摘要
<1-3 条要点>

## 设计文档
- [design.md](openspec/changes/<change-id>/design.md)
- [spec.md](openspec/changes/<change-id>/specs/<module>/spec.md)

## 测试计划
- [ ] 单元测试
- [ ] 集成测试
- [ ] T 层验收测试（{T}-XXX-NN ~ {T}-XXX-MM）
- [ ] 回归测试通过

## 风险评估
<从 design.md 回归风险表复制>

---
> 此 PR 当前为 Draft 状态，S4 实现完成后请求 Review。
EOF
)" \
  --draft
```

---

### S4: Tasks（开发实现）

**目的：** 按任务拆解实现代码

**产出文件：**
```
openspec/changes/<change-id>/
  └── tasks.md                # 任务拆解：Milestone → Task → T 层映射

internal/                     # 源代码变更
  └── layers/
      └── <layer>/
          └── *.go
          └── *_test.go
```

**操作流程：**

1. 编写 `tasks.md`，拆解为 Milestone → Task，标注依赖关系和 T 层映射
2. 按 TDD 流程开发：先写测试（RED）→ 实现（GREEN）→ 重构（IMPROVE）
3. 每个 Task 完成后提交，commit message 包含 T 层引用

**Commit 规范：**

```bash
# 常规格式
<type>: <简短描述>

关联: {T}-XXX-NN
Change: <change-id>
```

**示例：**

```bash
# 新增功能模块
git commit -m "$(cat <<'EOF'
feat: 实现 CommandPolicy Validate 白名单校验

关联: {T}-TOOL-01
Change: devrix-tool-security
EOF
)"

# Bug 修复
git commit -m "$(cat <<'EOF'
fix: Half-Open 状态无限并发探测

关联: D3-LLM-T17
Change: devrix-llm-gateway-v2
EOF
)"

# 测试
git commit -m "$(cat <<'EOF'
test: 添加 CB + Retry 协调集成测试

关联: D3-LLM-T17, D3-LLM-T18
Change: devrix-llm-gateway-v2
EOF
)"
```

**每次提交前检查：**

```bash
# 本地门禁（必须通过）
./scripts/test-unit.sh           # 单元测试 < 2min
go vet ./...                      # 静态检查
go build ./...                    # 编译检查
```

**推送并标记 PR 就绪：**

```bash
git push

# 当所有 tasks 完成时，标记 PR 就绪
gh pr ready
```

---

### S5: Acceptance（验收）

**目的：** 运行全量测试，生成验收报告

**产出文件：**
```
openspec/changes/<change-id>/
  └── acceptance-report.md    # 验收报告（模板见 testing.md §8）
```

**操作流程：**

1. 运行全量测试（S5 必须；PR CI 仅 unit smoke）
2. 编写或生成验收报告（frontmatter **`verdict: ACCEPTED`**）
3. 更新 T 层注册表状态为 `IMPLEMENTED`

```bash
# 全量测试（S5 门禁）
./scripts/test-all.sh

# 可选：生成报告骨架
./scripts/gen-acceptance-report.sh --change <change-id>

# 检查覆盖率（目标 ≥ 80%）
go test ./internal/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
```

**验收通过标准：**
- P0 T 层测试 100% **PASS**（矩阵行）
- 报告 **`verdict: ACCEPTED`**
- 覆盖率 ≥ 80%
- 无 race condition（`-race` 通过）

**提交验收：**

```bash
git add openspec/changes/<change-id>/acceptance-report.md
git add openspec/t-registry.md
git commit -m "$(cat <<'EOF'
acceptance: <change-id> 验收通过

S5 阶段，T 层测试 N/N PASS，覆盖率 XX%。
EOF
)"
git push
```

---

### S6: 交付与归档

**S6 分两子步（顺序固定）：**

| 子步 | 操作 | 规范 |
|------|------|------|
| **S6-交付** | PR squash 合入 `master` | `git-workflow.md` |
| **S6-归档** | 移入 `openspec/archive/` + 索引/域文档 | `archiving.md` |

**S6-归档操作流程：**

1. S6-交付完成（代码 PR 已合入 `master`）
2. 在功能分支或 `feat/archive-<change-id>` 上执行归档（**禁止**直推 `master`）
3. 运行 `./scripts/verify-archive.sh --changes <change-id>`
4. 更新 `demand-archive-index.md`；`.openspec.yaml` → `s7_archived`

详见 `openspec/specs/project/archiving.md` §3（场景 A/B）。

```bash
git checkout master && git pull origin master
git checkout -b feat/archive-<change-id>   # 或继续在 feat/<change-id>

./scripts/verify-archive.sh --changes <change-id>

ARCHIVE_DIR="openspec/archive/$(date +%Y-%m-%d)-<change-id>"
mkdir -p "$ARCHIVE_DIR"
cp -r openspec/changes/<change-id>/* "$ARCHIVE_DIR/"
git rm -r openspec/changes/<change-id>/
git add "$ARCHIVE_DIR" openspec/demand-archive-index.md

git commit -m "archive: <change-id> S6 归档"
git push -u origin HEAD
gh pr create --base master --title "archive: <change-id> S6 归档"
gh pr merge --auto --squash
```

---

## 四、PR 规范

### PR 标题

```
<change-id>: <简短描述（50 字以内）>
```

### PR 描述模板

```markdown
## 变更摘要
- <要点1>
- <要点2>

## 设计文档
- [design.md](openspec/changes/<change-id>/design.md)
- [spec.md](openspec/changes/<change-id>/specs/<module>/spec.md)

## 测试计划
- [ ] 单元测试 (<2min)
- [ ] 集成测试
- [ ] T 层验收测试（P0 全部 PASS）
- [ ] 回归测试 100%
- [ ] 覆盖率 ≥ 80%

## 风险评估
| 变更 | 风险 | 缓解 |
|------|------|------|
| ... | ... | ... |

## 关联 Issue
Closes #<issue-number>
```

### PR 生命周期

```bash
# 1. 创建 Draft PR（S3 设计阶段）
gh pr create --title "..." --body "..." --draft

# 2. 开发过程中查看 PR 状态
gh pr view
gh pr status

# 3. S4 完成后标记就绪
gh pr ready

# 4. 请求 Review（单人团队：S4-Gate 自检 + CI，见 git-workflow.md §8）
# gh pr review --approve   # 仅在有第二 Reviewer 时使用

# 5. 查看 CI 状态（required: unit tests）
gh pr checks

# 6. S5 验收通过后合入（推荐 auto-merge）
gh pr merge --auto --squash --delete-branch

# 7. 查看已合并 PR
gh pr list --state merged --limit 10
```

---

## 五、Code Review 流程

> 单人团队：**S4-Gate 自检**（`review-code.md`）+ CI 全绿，无需他人 Approve。详见 `git-workflow.md` §8。

### Review 触发条件

- S4 完成、PR 从 Draft 转 Ready
- 安全相关变更须完成 `review-code.md` 全表自检

### Review Checklist

- [ ] OpenSpec 文档完整（proposal / design / specs / tasks）
- [ ] T 层测试点已在注册表登记
- [ ] PR 描述含 S4-Gate 自检章节
- [ ] `./scripts/test-unit.sh` + `go vet` 通过
- [ ] 覆盖率 ≥ 80%（S5 目标；PR CI 基线 20%）

---

## 六、CI/CD 门禁

> **PR 合并 required check：仅 `unit tests`。** 合入前本地建议跑 `test-all.sh`（S5）。详见 `git-workflow.md` §4。

CI 流水线：`.github/workflows/ci.yml`

```
git push ──> unit tests (required on PR)
          └── layer-lint (warn-only，不阻断 merge)
```

### 本地预检

```bash
go mod verify
go build ./...
go vet ./...
./scripts/test-unit.sh          # PR 门禁
./scripts/test-all.sh           # S5 全量（合入前建议）
```

---

## 七、常用 gh 命令速查

### 仓库信息

```bash
gh repo view                    # 查看仓库信息
gh repo list                    # 列出仓库
```

### Issue

```bash
gh issue list                   # 列出 Issue
gh issue view <number>          # 查看 Issue
gh issue create                 # 创建 Issue
gh issue close <number>         # 关闭 Issue
gh issue comment <number> --body "..." # 评论
```

### PR

```bash
gh pr list                      # 列出 PR
gh pr view                      # 查看当前分支 PR
gh pr create                    # 创建 PR
gh pr diff                      # 查看 PR diff
gh pr merge                     # 合并 PR
gh pr close                     # 关闭 PR（不合并）
gh pr ready                     # 标记 Draft PR 就绪
```

### CI

```bash
gh run list                     # 列出 workflow 运行
gh run view <id>                # 查看运行详情
gh run watch <id>               # 实时查看运行日志
gh pr checks                    # 查看 PR CI 状态
```

---

## 八、完整示例

以 `devrix-tool-security` 为例：

```bash
# S1: Demand
gh issue create --title "[devrix-tool-security] 工具执行安全增强" \
  --label "demand,L4" --body "..."

# S2: Proposal
git checkout -b feat/devrix-tool-security
# 编写 .openspec.yaml + proposal.md
git add openspec/changes/devrix-tool-security/
git commit -m "proposal: devrix-tool-security 提案"
git push -u origin feat/devrix-tool-security

# S3: Design
# 编写 design.md + specs/tool-security/spec.md
git add openspec/changes/devrix-tool-security/
git commit -m "design: devrix-tool-security 技术设计"
git push
gh pr create --title "devrix-tool-security: 工具执行安全增强" --body "..." --draft

# S4: 开发
# 按 tasks.md 逐任务实现
git commit -m "$(cat <<'EOF'
feat: 实现 CommandPolicy 命令白名单校验

关联: {T}-TOOL-01
Change: devrix-tool-security
EOF
)"
git push

# ... 多个 commits ...
gh pr ready  # 所有任务完成后

# S5: 验收
./scripts/test-all.sh
# 编写 acceptance-report.md（verdict: ACCEPTED，见 testing.md §8）
git commit -m "acceptance: devrix-tool-security 验收通过"
git push

# S6-交付: PR 合入
gh pr merge --auto --squash --delete-branch

# S6-归档（独立 PR，见 archiving.md）
git checkout master && git pull origin master
git checkout -b feat/archive-devrix-tool-security
./scripts/verify-archive.sh --changes devrix-tool-security
# ... 归档步骤见 § S6 ...
```
