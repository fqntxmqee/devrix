# Devrix 研发流程规范

**版本:** 1.0.0
**最后更新:** 2026-06-08

---

## 一、总览

Devrix 研发流程由两条主线交织而成：

| 主线 | 规范 | 工具 |
|------|------|------|
| 需求到验收 | OpenSpec 六阶段 (S1-S6) | 文件体系 (`openspec/`) |
| 代码协作 | GitHub Flow | `gh` CLI |

```
S1 Demand ──> S2 Proposal ──> S3 Design ──> S4 Tasks ──> S5 Acceptance ──> S6 Archive
     │              │              │             │              │                │
     └── gh issue   └── branch     └── PR draft  └── commits   └── PR merge     └── archive
```

---

## 二、分支策略

采用 **GitHub Flow**（单一主干 + 短期功能分支）：

```
main
  └── feat/<change-id>          # 功能分支
  └── fix/<change-id>           # 修复分支
  └── docs/<description>        # 文档分支
```

**规则：**
- `main` 始终保持可部署状态
- 一个 Change 一个分支，分支名与 Change ID 对应
- 分支从 `main` 拉出，合并回 `main`
- 合并前必须通过 CI 门禁

```bash
# 创建功能分支
git checkout main
git pull origin main
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

**`.openspec.yaml` 模板：**

```yaml
change_id: <change-id>
priority: P0 | P1 | P2
demand_id: DM-YYYYMMDD-NNN
parent_change: <parent-change-id>  # 可选
layers: [D1, D2, ...]
t_points: [{T}-XXX-NN, ...]
estimated_hours: <hours>
```

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

1. 编写 `design.md`（参考现有变更的格式）
2. 编写 `specs/<module>/spec.md`（Gherkin 场景）
3. 如需详细架构文档，按 `docs/detail design framework.md` 六段式编写 `docs/<module>-design.md`

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
  └── acceptance-report.md    # 验收报告（由脚本生成）
```

**操作流程：**

1. 运行全量测试
2. 生成验收报告
3. 更新 `openspec/t-registry.md` 对应测试点状态为 `IMPLEMENTED`

```bash
# 全量测试
./scripts/test-all.sh

# 生成验收报告
./scripts/gen-acceptance-report.sh --change <change-id>

# 检查覆盖率
go test ./internal/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total
```

**验收通过标准：**
- P0 T 层测试 100% PASS
- 覆盖率 ≥ 80%
- 无 race condition（`-race` 通过）
- 回归测试 100% PASS

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

### S6: Archive（归档）

**目的：** 变更完成后归档到 `openspec/archive/`

**操作流程：**

1. PR 合并到 `main` 后
2. 将 `openspec/changes/<change-id>/` 移动到 `openspec/archive/<YYYY-MM-DD>-<change-id>/`
3. 更新 `demand-archive-index.md`（添加 PR 链接和归档路径）

```bash
# 合并后，在 main 分支上归档
git checkout main
git pull origin main

# 移动归档
mkdir -p openspec/archive/2026-06-08-<change-id>
cp -r openspec/changes/<change-id>/* openspec/archive/2026-06-08-<change-id>/
git rm -r openspec/changes/<change-id>/
git add openspec/archive/2026-06-08-<change-id>/

# 更新归档索引
git add openspec/demand-archive-index.md

git commit -m "$(cat <<'EOF'
archive: <change-id> 归档

PR: #<number>
EOF
)"
git push
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

# 4. 请求 Review
gh pr review --request @reviewer

# 5. 查看 CI 状态
gh pr checks

# 6. 合并（S5 验收通过后）
gh pr merge --squash --delete-branch

# 7. 查看已合并 PR
gh pr list --state merged --limit 10
```

---

## 五、Code Review 流程

### Review 触发条件

- 所有 PR 合并前必须至少一人 Review
- 安全相关变更必须 security-reviewer 参与

### Review 命令

```bash
# 查看 PR diff
gh pr diff

# 查看 PR 评论
gh pr view --comments

# 提交 Review
gh pr review --approve    # 通过
gh pr review --request-changes --body "需要修复: ..."   # 请求修改
gh pr review --comment --body "建议: ..."               # 仅评论

# 列出 Review 意见
gh pr view --json reviews
```

### Review Checklist

- [ ] OpenSpec 文档完整（proposal / design / specs / tasks）
- [ ] T 层测试点已在 `t-registry.md` 登记
- [ ] 代码符合 `CLAUDE.md` 编码风格（不可变、小函数、错误处理）
- [ ] 无硬编码密钥/凭证
- [ ] 覆盖率 ≥ 80%
- [ ] CI 全绿

---

## 六、CI/CD 门禁

CI 流水线定义于 `.github/workflows/ci.yml`：

```
git push ──> unit tests ──> gate (integration/e2e/acceptance) ──> coverage
                              (需 unit 通过)                    (需 gate 通过)
```

### 本地预检

提交前必须通过本地门禁：

```bash
# 等同于 CI unit 阶段
go mod verify
go build ./...
go vet ./...
./scripts/test-unit.sh

# 集成测试
./scripts/test-integration.sh
```

### PR 状态检查

```bash
# 查看 CI 运行状态
gh pr checks

# 查看特定 job 日志
gh run view --job <job-id>

# 重新运行失败的 CI
gh run rerun <run-id>
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
./scripts/gen-acceptance-report.sh --change devrix-tool-security
git commit -m "acceptance: devrix-tool-security 验收通过"
git push

# PR 合并
gh pr merge --squash --delete-branch

# S6: 归档
git checkout main && git pull
mkdir -p openspec/archive/2026-06-08-devrix-tool-security
cp -r openspec/changes/devrix-tool-security/* openspec/archive/2026-06-08-devrix-tool-security/
git rm -r openspec/changes/devrix-tool-security/
git add openspec/archive/
git add openspec/demand-archive-index.md
git commit -m "archive: devrix-tool-security 归档"
git push
```
