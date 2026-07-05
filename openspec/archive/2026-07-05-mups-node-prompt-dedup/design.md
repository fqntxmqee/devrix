# Design: MUPS 三节点 Prompt 去冗余

**Change ID:** `mups-node-prompt-dedup`
**Demand ID:** DM-20260705-004 (注: 同一 DM-ID 此前曾被 `mups-plan-structbind` 使用, 见 §6)
**Status:** S4_Development (→ S5_Accepted)

---

## 1. 三层冗余分析

MUPS Observe/Plan/Execute 三节点的 prompt 组装存在四类冗余：

| 类型 | 位置 | 冗余形态 |
|------|------|----------|
| **角色重复** | Observe/Plan appendix | `observe.node_role` / `plan.node_role` i18n 与 body intro 段落重复 |
| **plane 标签三套同名** | LineFrame registry + prompt | `[control]`/`[data]` 行前缀 + `BuildLineFrameFromStruct` 前缀 + 前缀 guide 三重 |
| **user guide 列全字段但正文省略** | frame field guide | 已通过 fieldMap guide 列字段，正文还出现"省略 ..."引导反而误导 |
| **Execute 字段标签未走 i18n** | Execute system | 字段标签硬编码英文，破坏 locale 切换 |

---

## 2. 节点差异化设计

### Observe（user 帧净化）

```diff
- Observe user frame:
-   work_item_id: wi_1
-   session_id: sess_1
-   prior_mean: 0.45          ← orchestration-only control
-   incremental_only: false   ← orchestration-only control
-   [control]directive: ...   ← plane 行前缀
-   [data]signal: ...         ← plane 行前缀
+ Observe user frame:
+   directive: hi             ← LLM 可见 data 字段
+   signal: ...               ← LLM 可见 data 字段
+   prior_parse_reject: null  ← 上一轮 reject 反馈 (DM-002)
```

**保留字段**：仅 LLM 真正消费 + `prior_parse_reject` (parse-reject feedback 链路)。去掉 `work_item_id/prior_mean/incremental_only`（control plane，LLM 无需知道）。

### Plan（去掉 lineframe 行前缀与 planeGuide）

```diff
- Plan user frame:
-   [control]execution_mode: ...
-   [data]directive: ...
+ Plan user frame:
+   execution_mode: ...
+   directive: ...
```

**guide 仅列出现字段**：已有 fieldMap 渲染实际字段，`BuildLineFrameFromStruct` 默认无 plane 前缀。

### Execute（system 顺序 + i18n 标签）

```diff
- Execute system:
-   role: 你是 Execute 节点的工作项执行助手
-   role (execute): 你是 Execute 节点...   ← 与上面 role 重复
-   task body: ...
-   output hints: ...
+ Execute system:
+   role: 你是 Execute 节点的工作项执行助手
+   task body: ...                       ← 在 outputHints 之前
+   output hints: ...
+   field labels: 工作项ID / 会话ID / 指令 / 信号 ...  ← i18n 标签
```

---

## 3. 共用：`BuildLineFrameFromStruct` 默认无前缀

**Before**:
```go
func BuildLineFrameFromStruct(...) string {
    return buildWithPlanePrefix(...)  // 输出 "[control]key: val"
}
```

**After**:
```go
func BuildLineFrameFromStruct(...) string {
    return buildFlatKeyValue(...)      // 输出 "key: val"
}
```

plane 语义通过 guide header 一处表达，不在每行重复。

---

## 4. 共用：appendix 不再重复 node_role

**Before** (`prompttags_semantics_{zh,en}.go`):
```
observe.node_role: 你是编排 Observe 节点的封闭式分类助手。
[body intro 段落]
角色定位：你是编排 Observe 节点的封闭式分类助手。  ← 重复
```

**After**:
```
observe.node_role: 你是编排 Observe 节点的封闭式分类助手。
[body intro 段落，引用 node_role 作为前置，不重写]
```

---

## 5. 文件布局

```
internal/shared/prompttags/
  structbind.go                          # BuildLineFrameFromStruct 改造 (无 plane 前缀)

internal/layers/contextengine/i18n/
  prompttags_semantics_render.go         # appendix 渲染跳过 node_role 重复
  format_hints_mups_observer_test.go     # 测试更新
  format_hints_mups_test.go              # 测试更新
  prompttags_semantics_golden_test.go    # golden hash 改写
  workitem_execute.go                    # Execute system 顺序 + i18n 字段标签

internal/layers/contextengine/materialize/
  phase_prompts.go                       # phase appendix 跳过 duplicate node_role
  phase_prompts_test.go                  # golden hash 改写
  mups_materializer.go                   # materialize 顺序调整
  prompts.go                             # system prompt 装配
  prompts_test.go                        # 测试更新

internal/layers/orchestration/sessionorchestrator/
  llm_observation_proposer.go            # Observe user frame 字段过滤
  llm_observation_proposer_test.go       # 测试更新
  observation_closed_classifier_test.go  # 测试更新
  observe_structbind_test.go             # 测试更新
  plan_structbind_test.go                # 测试更新
  strategic_plan_proposer_test.go        # 测试更新

openspec/specs/shared/
  prompttags.md                          # 同步新契约
  mups-node-llm-protocols.md             # §8 增加 LLM prompt 协议
```

---

## 6. DM-ID Conflict Note (重要)

DM-20260705-004 此前已被 `mups-plan-structbind` 使用 (PR #405, 已 S7_Archived 2026-07-05, 详见 `openspec/archive/2026-07-05-mups-plan-structbind/acceptance-report.md`).

**两者实质不同**：
- 旧 `mups-plan-structbind`: Plan 节点反射驱动 struct I/O (M2 kernel 复用)
- 本 `mups-node-prompt-dedup`: 三节点 prompt 文本层去冗余 + structbind 对齐

DM-ID 误用属 metadata error。建议未来 change-id 命名遵循 `mups-` 前缀且 DM-ID 强唯一性。

---

## 7. 不变量

1. **去重不引入新字段** — 净减行；不增加任何 LLM 可见字段
2. **i18n locale 切换对称** — EN/ZH 双语同步去重，无遗漏
3. **golden hash 改写** — 预期内 (format 变化)，全部 golden test 同步更新
4. **Execute 字段标签 i18n 化** — 调用方改走 `prompttags.LabelFor(fieldKey, locale)`，新 API 编译期类型校验
5. **BuildLineFrameFromStruct 无 plane 前缀** — 任何调用方都不应假设行首有 `[control]`/`[data]` 前缀