# 归档规范

**版本:** 1.0.0
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

```bash
# 1. 切换到 main 并拉取最新
git checkout main
git pull origin main

# 2. 移动 change 到 archive
ARCHIVE_DIR="openspec/archive/$(date +%Y-%m-%d)-<change-id>"
mkdir -p "$ARCHIVE_DIR"
cp -r openspec/changes/<change-id>/* "$ARCHIVE_DIR/"
git add "$ARCHIVE_DIR"
git rm -r openspec/changes/<change-id>/

# 3. 更新索引
# 编辑 openspec/demand-archive-index.md，添加：
# | <demand-id> | <change-id> | YYYY-MM-DD | PR #<number> |

git add openspec/demand-archive-index.md

# 4. 同步域架构文档（按 §2.4 评估）
# 如果变更影响域架构，更新对应的 openspec/specs/d{N}-*/ 文件
# - spec.md: 合并 Gherkin Scenario
# - design.md: 更新领域模型/接口/流程图
# - a-registry.md / f-registry.md / t-registry.md: 更新条目
git add openspec/specs/d{N}-*/ 2>/dev/null || true

# 5. 提交归档
git commit -m "$(cat <<'EOF'
archive: <change-id> 归档

归档至 openspec/archive/YYYY-MM-DD-<change-id>/
PR: #<number>
EOF
)"
git push
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
