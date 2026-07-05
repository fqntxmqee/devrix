# Proposal: Observe 节点封闭式分类器定位强化

**Change ID:** `d7-observe-closed-classifier-prompt`
**Demand:** DM-20260705-009
**Status:** S2_Proposal

## Why

MUPS 5 节点重构(M1+M2+M3+M4+M5)在 2026-07-05 完成。Observe 节点在 M1 走 go-struct-driven,9 字段契约 + `[data]/[control]` 包裹 + i18n guide header + 4 alias 解析 + `prior_parse_reject` 反馈链路全部就位。

但用户报告 LLM 调用 Observe 节点时出现 3 个症状:

1. **"已经修改了用户语义"** — directive 被 `[data] directive:` 包裹
2. **"对应的动态提示词也不对"** — system_prompt 让 LLM 困惑
3. **"大模型返回了错误的格式"** — LLM 返 markdown / 单文本 / 空数组

**根因诊断**(详见 `demand.md` §2):
- 症状 1 = M1 契约(不是 bug,design intent)
- 症状 2 = system_prompt 缺"封闭式分类器"定位声明(是 Gap)
- 症状 3 = 症状 2 的衍生(是 Gap)

**结论**: M1 9 字段契约 / i18n guide header / parse 链路全部 OK,需要修的只是 system_prompt 措辞强化。

## What

| Capability | Description |
|------------|-------------|
| **D7-S5-A99** (MODIFIED) | `format_hints_mups.go::observationTaskAppendixZHIntro/ENIntro/ZHSuffix/ENSuffix` 措辞强化 |
| **D7-S5-A99** (MODIFIED) | `prompttags_semantics_{zh,en}.go::observe.node_role` 同步改写 |
| **D7-S5-A99** (NEW test) | `format_hints_mups_observer_test.go` golden snapshot 覆盖"封闭式分类器" |
| **D7-S5-A99** (NEW test) | `observation_closed_classifier_test.go` 集成测试覆盖 4 alias |

## Scope

- **本 change**: system_prompt 措辞强化 + 新增 2 个测试,不动 9 字段契约
- **Out of scope**: 
  - 9 字段契约 / i18n guide header / parse 链路 (M1 锁定)
  - LLM invocation 路径 (D3 不动)
  - PlanKind 路由 (M3 已闭环)
  - 任务类型感知 system_prompt 分支 (下个 change)

## Architecture decision

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: 仅强化 system_prompt 措辞(采纳)** | 最小改动,不动 M1 契约,精准修复 | LLM 调用频率不变,token 数略增 |
| B: 改 user frame 形式(directive 不包裹) | 满足用户"语义不被修改"诉求 | **破坏 M1 契约**,所有现有 M1 测试 fail,9 字段设计推倒重来 |
| C: 引入任务类型感知 system_prompt 分支 | 完美匹配不同任务 | 过大范围,推迟到下个 change;本 change 仅修根因(封闭式分类器定位) |
| D: 回退到 tag-driven 设计 | 兼容老 design | **已拒绝**: go-struct-driven 是 DM-20260705-003 锁定设计,回退等于推倒 5 节点重构 |

**选 A**。理由:
- 精准修复 system_prompt 措辞,不动 M1 契约(用户明确说"go-struct-driven 是否更好,对我来说不要有重复链路")
- 8 现有测试 0 修改 PASS(它们只验证"包含某 marker",不验证整段措辞)
- 0 行为变化(只有 system_prompt 文本变化,不影响 parse / 字段 / 反馈链路)

## Risks & Mitigations

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| 现有 8 测试 fail(因为措辞改了 marker 字符串) | Low | Med | 8 测试只验证子串,改动后子串保留;AC5 P0 强制 |
| LLM 仍不理解"封闭式分类器"措辞 | Low | Med | golden snapshot 测试覆盖关键 marker;如真 fail,迭代措辞 |
| 改了 i18n 同步遗漏 en/zh | Low | Low | en/zh 同步改 + 双语测试 |
| 用户仍不接受 wrapper 形式 | Low | Low | M1 契约说明在 demand.md §7 Out of Scope |

## Success Metrics

- 8 现有测试 0 修改 PASS
- 新增 2 测试 PASS (golden snapshot + 集成)
- LLM 在"开放式 directive + 无 signal"场景下,system_prompt 包含"封闭式分类器"和"signal 不足 → obs_uncertainty"措辞
- 0 行为变化 (parse 链路、字段、反馈链路不动)
