# 归档规范

**版本:** 1.1.0
**状态:** Active
**所属阶段:** S6
**前置阶段:** S5 验收通过、PR 已合并到 main

---

## 1. 归档触发条件

S6 归档在以下条件**全部满足**后执行：

- [ ] PR 已合并到 `main` 分支
- [ ] S5 验收报告状态为 PASS
- [ ] 所有 P0 T 层测试 100% PASS
- [ ] T 层注册表对应条目已更新为 IMPLEMENTED（根索引 `openspec/t-registry.md` + 域注册表 `openspec/specs/d{N}-*/t-registry.md`）

---

## 2. 归档前检查清单

执行归档前逐项确认：

### 2.1 文件完整性

- [ ] `.openspec.yaml` 存在且 status = `s7_archived`
- [ ] `proposal.md` 存在且状态标记为 "Archived"
- [ ] `demand.md` 存在（如 S1 创建了的话）
- [ ] `design.md` 存在
- [ ] `tasks.md` 存在
- [ ] `specs/*/spec.md` 存在
- [ ] `acceptance-report.md` 存在且结论为 ACCEPTED

### 2.2 状态一致性

- [ ] `.openspec.yaml` 中的 `status` 与 `proposal.md` 头部一致
- [ ] `demand-id` 与 `demand-archive-index.md` 中的记录一致

### 2.3 索引更新

- [ ] T 层注册表对应条目状态为 IMPLEMENTED（根索引 + 域注册表 `openspec/specs/d{N}-*/t-registry.md`）
- [ ] `openspec/demand-archive-index.md` 将新增记录
- [ ] 域架构文档已评估是否需要同步更新（见 §2.4）

### 2.4 域文档同步

归档前，**必须评估**本次变更是否影响 `openspec/specs/d{N}-*/` 下的域架构文档。判断标准：

**需要同步的情况：**

| 变更类型 | 需更新的域文档 | 说明 |
|---------|---------------|------|
| 新增/修改 A（活动） | `a-registry.md` | 新增活动条目或修改活动描述 |
| 新增/修改 F（功能点） | `f-registry.md` | 新增功能点条目或修改代码位置 |
| 新增 T 层测试点 | `t-registry.md` | 状态从 PLANNED → IMPLEMENTED |
| 架构设计变更 | `design.md` | 领域模型、接口、业务流程变更 |
| 新增/修改 Gherkin 规格 | `spec.md` | 将 changes 下的 Scenario 合并到域 spec.md |
| 跨域影响 | 所有受影响域的上述文档 | 每个域独立评估 |

**不需要同步的情况：**

| 变更类型 | 原因 |
|---------|------|
| Bug 修复（无接口变更） | 不改变架构契约 |
| 纯重构（无行为变更） | 不改变架构契约 |
| 配置调整 | 不改变架构文档 |
| 测试补充（已有 T 层预登记） | 仅更新 t-registry 状态 |

**同步操作：**

```bash
# 1. 评估受影响域
# 查看 .openspec.yaml 中的 domains 字段
cat openspec/changes/<change-id>/.openspec.yaml | grep domains

# 2. 对于每个受影响域，检查是否需要更新
# - spec.md: 合并 changes/<id>/specs/ 下的 Gherkin Scenario
# - design.md: 更新领域模型/接口/流程图
# - a-registry.md: 新增/修改活动条目
# - f-registry.md: 新增/修改功能点条目
# - t-registry.md: 更新测试点状态

# 3. 在归档 commit 中包含域文档更新
git add openspec/specs/d{N}-*/
```

---

## 3. 归档操作

**关键约束：** 归档变更也必须遵循 GitHub Flow，通过 `feat/<change-id>` 分支 + PR 合并到 master。**禁止**直接在 master 上提交归档，**禁止**将归档混入其他功能分支。

### 3.0 分支策略（S6 专用）

```
场景 A: 独立归档分支（S6 在工作分支上触发）
    从 master 切出 feat/archive-<change-id> → 归档操作 → push → PR → squash merge → 删除分支

场景 B: S6 紧接 S4 实现（代码和归档在同一 change）
    在已有 feat/<change-id> 分支上追加归档 commit → push → PR → squash merge → 删除分支
```

| 场景 | 分支名 | 说明 |
|------|--------|------|
| A | `feat/archive-<change-id>` | 仅做归档，不包含代码变更 |
| B | `feat/<change-id>` | 代码实现和归档在同一分支，S6 在 PR 合并前追加 |

### 3.0.1 预检（强制）

**提交归档 commit 前必须运行验证脚本：**

```bash
# 场景 A（独立归档分支，已验证 changes/ 目录）
./scripts/verify-archive.sh --changes <change-id>

# 场景 B（在 feature 分支追加，change 仍在 changes/ 目录）
./scripts/verify-archive.sh --changes <change-id>

# 补充归档后（change 已移入 archive/）
./scripts/verify-archive.sh <change-id>
```

脚本对应 §2.1–§2.4 检查清单，退出码 0 = 全部通过。**✗ 项必须先修复再提交。**

### 3.1 执行步骤

```bash
# === 场景 A: 独立归档分支 ===

# 1. 从 master 切出归档分支
git checkout master
git pull origin master
git checkout -b feat/archive-<change-id>

# === 场景 B: 在已有 feat/<change-id> 分支上追加 ===

# 1. 切换到已有分支
git checkout feat/<change-id>
git pull origin feat/<change-id> 2>/dev/null || true

# === 以下步骤两种场景通用 ===

# 2. 确认前置条件
# - PR 已合并到 master（场景 A）或即将通过当前 PR 合并（场景 B）
# - acceptance-report.md 已创建且结论为 ACCEPTED
# - .openspec.yaml status 已更新为 s7_archived

# 3. 移动 change 到 archive
ARCHIVE_DIR="openspec/archive/$(date +%Y-%m-%d)-<change-id>"
mkdir -p "$ARCHIVE_DIR"
cp -r openspec/changes/<change-id>/* "$ARCHIVE_DIR/"
git add "$ARCHIVE_DIR"
git rm -r openspec/changes/<change-id>/

# 4. 更新索引
# 编辑 openspec/demand-archive-index.md，添加：
# | <demand-id> | <change-id> | YYYY-MM-DD | PR #<number> |

git add openspec/demand-archive-index.md

# 5. 同步域架构文档（按 §2.4 评估）
git add openspec/specs/d{N}-*/ 2>/dev/null || true

# 6. 提交归档
git commit -m "$(cat <<'EOF'
archive: <change-id> S6 归档

归档至 openspec/archive/YYYY-MM-DD-<change-id>/
PR: #<number>
EOF
)"
git push -u origin HEAD

# 7. 创建 PR 并合并
gh pr create --title "archive: <change-id> S6 归档" \
    --body "## Summary
- 归档 change: <change-id>
- Demand ID: <demand-id>
- S5 验收: ACCEPTED
- 代码 PR: #<number> (场景 A) / 当前 PR (场景 B)

## 归档内容
- acceptance-report.md
- 域文档同步（如适用）
- demand-archive-index.md 更新"

# 8. 合并后删除分支
git checkout master
git pull origin master
git branch -d feat/archive-<change-id>  # 场景 A
git branch -d feat/<change-id>           # 场景 B
```

---

## 4. 归档目录命名

```
openspec/archive/<YYYY-MM-DD>-<change-id>/
```

示例：
- `openspec/archive/2026-06-08-devrix-multi-agent/`
- `openspec/archive/2026-06-07-devrix-tool-security/`

---

## 5. 不应归档的情况

以下情况**禁止**归档：

| 情况 | 处理 |
|------|------|
| S5 验收未通过 | 回到 S4 修复 |
| PR 未合并 | 等待合并 |
| 文件不完整 | 补全后再归档 |
| 状态标记不一致 | 修复一致后再归档 |
| Change 被放弃 | 不归档，标记为 S0_Abandoned 并留在 changes/ |
| Change 仅设计阶段（S2）未实现 | 不归档，留在 changes/ 或标记 S0_Deferred |

---

## 6. 归档后验证

```bash
# 确认 changes/ 下已移除
ls openspec/changes/<change-id>/ 2>/dev/null && echo "ERROR: not removed" || echo "OK"

# 确认 archive/ 下存在
ls openspec/archive/YYYY-MM-DD-<change-id>/

# 确认 git 状态干净
git status
```

---

## 7. 分支清理

S6 归档完成后，**必须**清理对应分支：

| 步骤 | 命令 | 说明 |
|------|------|------|
| 确认已合并 | `git branch --merged origin/master \| grep <change-id>` | 分支必须在 master 中 |
| 删除本地分支 | `git branch -d feat/<change-id>` | 场景 A/B 通用 |
| 删除远程分支 | `git push origin --delete feat/<change-id>` | 如果 GitHub 未自动删除 |
| 验证 | `git branch -a \| grep <change-id>` | 应无输出 |

**禁止**保留已合并的 feature 分支，避免分支列表膨胀和职责混淆。

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2026-06-07 | 初始归档规范 |
| 1.1.0 | 2026-06-16 | 新增 §3.0 分支策略（场景 A/B）+ §7 分支清理；禁止在 master 直推归档、禁止混入其他功能分支 |
