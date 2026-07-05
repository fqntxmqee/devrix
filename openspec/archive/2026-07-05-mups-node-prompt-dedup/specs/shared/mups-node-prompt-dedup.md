# Delta: MUPS node LLM protocols — 三节点 prompt 去冗余

**Change ID:** `mups-node-prompt-dedup`
**Demand:** DM-20260705-004
**Base:** `openspec/specs/shared/mups-node-llm-protocols.md` §8

---

## MODIFIED: §8 Prompt 组装协议

### Observe 节点 (user frame)

**Before:**
```
Observe user frame (LLM-visible 字段):
  work_item_id: wi_1            ← control plane (去除)
  session_id: sess_1            ← control plane (去除)
  prior_mean: 0.45              ← control plane (去除)
  incremental_only: false       ← control plane (去除)
  [control]directive: hi        ← plane 行前缀 (去除)
  [data]signal: ...             ← plane 行前缀 (去除)
```

**After:**
```
Observe user frame (LLM-visible 字段):
  directive: hi                 ← data plane
  signal: ...                   ← data plane
  prior_parse_reject: null      ← 上一轮 reject 反馈 (DM-20260705-002)
```

### Plan 节点 (user frame)

**Before:**
```
Plan user frame:
  [control]execution_mode: ...
  [control]uncerainty_mean: ...
  [data]directive: ...
  [data]scope_in: [...]
  ...
```

**After:**
```
Plan user frame:
  execution_mode: ...
  uncertainty_mean: ...
  directive: ...
  scope_in: [...]
  ...
```

`BuildLineFrameFromStruct` 默认无 plane 前缀；guide header 单点声明 plane 语义。

### Execute 节点 (system)

**Before:**
```
Execute system (顺序):
  1. role: 你是 Execute 节点的工作项执行助手
  2. role (execute): 你是 Execute 节点...    ← 与上面 role 重复
  3. task body: ...
  4. output hints: ...
  5. field labels (硬编码英文): work_item_id, session_id, ...
```

**After:**
```
Execute system (顺序):
  1. role: 你是 Execute 节点的工作项执行助手
  2. task body: ...                          ← 在 outputHints 之前
  3. output hints: ...
  4. field labels (i18n): 工作项ID / 会话ID / 指令 / 信号 ...
```

调用方改走 `prompttags.LabelFor(fieldKey, locale)`，新 API 编译期类型校验。

### Appendix 装配

Observe/Plan phase appendix 不再重复 `observe.node_role` / `plan.node_role` i18n 段落；`semanticBlock` 之前仅放置 node role (单次出现)。

---

## Invariants

1. **去重不引入新字段** — 净减行；不增加任何 LLM 可见字段
2. **i18n locale 切换对称** — EN/ZH 双语同步去重
3. **golden hash 改写** — 预期内 (format 变化)，全部 golden test 同步更新
4. **BuildLineFrameFromStruct 无 plane 前缀** — 任何调用方都不应假设行首有 `[control]`/`[data]` 前缀

---

## 关联

| 变更 | 关系 |
|------|------|
| DM-20260705-003 (mups-semantics-schema-alignment) | 同步: 结构化 SemanticRule 替代 prose 重复 |
| DM-20260705-002 (mups-parse-reject-feedback) | 同步: `prior_parse_reject` 在 Observe user frame 保留 |
| DM-20260705-009 (d7-observe-closed-classifier-prompt) | 后续: 封闭式分类器定位在 Observe body 同步强化 |