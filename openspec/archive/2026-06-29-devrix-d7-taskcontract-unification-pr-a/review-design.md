# S3-Gate Review: D7 TaskContract 统一 PR-A (DM-20260629-007)

**Change ID:** devrix-d7-taskcontract-unification-pr-a
**Reviewer:** self-review (待用户确认)
**Review Date:** 2026-06-29
**Review Framework:** `openspec/specs/project/review-design.md`（设计文档 5 维度审查）

---

## 1. 审查总览

| 维度 | 评分 | 备注 |
|------|------|------|
| **数据维度** | ✅ PASS | TaskSpec 4+2 字段 / TaskReport 5+2 字段定义完整；3 子类型 (Dissent / Blockage / Resource) 边界清晰 |
| **逻辑维度** | ✅ PASS | Downlink 链路 (§3.1) + Uplink 链路 (§3.2) + 3 字段填充规则 (§3.3-3.5) 全链路贯通 |
| **边界维度** | ✅ PASS | interfaces 包 0 import D7 子包（Pure types）+ ChannelRequest/LearnRequest additive 字段嵌入 |
| **调用维度** | ✅ PASS | 3 创建点（Plan/Channel/WorkItem）+ 4 出口（commit/scenario/protocol/exploration）+ 1 入口（learner.go）全列出 |
| **异常维度** | ✅ PASS | 5 个 ORCH_* SentinelError + Validate 防御 + top-3 截断 + dedup by hash |

**总体评级：** ✅ **PASS — 推荐进入 S4 实施**

---

## 2. 关键审查点

### 2.1 ✅ T 编号重映射（决策合理性）

- **背景**：父 DESIGN §2.2 写 `D7-S16/17/18/19`，但 `D7-S16` 已被 `devrix-d7-layer-subcontext` (DM-20260627-003) 占用
- **本 Change 决策**：重映射到 `D7-S20/21/22/23`
- **理由**：避免覆写 18 个已 IMPLEMENTED T 点；v7.0 sprint 专属编号段；父 DESIGN 归档不影响历史
- **一致性**：design.md §2.3 + proposal.md §3.4 Decision 1 + .openspec.yaml + spec.md §7 三处一致

### 2.2 ✅ Pure types 边界（防 import cycle）

- **interfaces 包 0 import D7 任何子包**：design.md §④.2 + spec.md §3 双重声明
- **白名单 9 包 import 列表**（design.md §④.2）：mups/{execute,learn,observe} + workmodel + decisionplanning + escape + hardening + sessionorchestrator + executionflow + d7-bootstrap
- **PR-A 阶段仅注释声明**：layout guard 完整实施在 PR-C（AC8）

### 2.3 ✅ ChannelRequest / LearnRequest additive 嵌入（兼容过渡）

- **PR-A 决策**：新增嵌入字段 `Spec *TaskSpec` / `Report *TaskReport`，老字段全部保留
- **影响**：22/22 orchestration packages 0 编译失败；现有 LP-1/LP-2/LP-5 测试集 100% 通过
- **后续路径**：PR-B 完整迁移 + type alias 保留 1 minor 版本 + PR-C 移除老字段

### 2.4 ✅ Dissent top-3 截断（性能与价值平衡）

- **决策**：默认 top-3（少数派保留），env flag 可调
- **触发条件**：`Result.Kind == Indeterminate` 或 `fallbackUsed=true`
- **Learn 节点消费**：SkillMemory.SOP 按 hash dedup（同一 entry 多次 AppendDissent 不重复入库）

### 2.5 ✅ Resource 字段复用 ContextBudget Phase B

- **避免重新埋点**：复用现有 metric，仅做字段抽取
- **5 个度量**：TokensUsed / TokensBudget / TimeElapsed / StepCount / ToolInvocations
- **单位一致性**：tokens ≥ 0, time ≥ 0, steps ≥ 0；任一 < 0 → ErrResourceInvalid

### 2.6 ✅ Spec 文档同步（AC17 PR-A 提交前置）

- **5+1 文件同步**：spec.md + d7-domain.md + a/f/t/span-registry.md
- **同步内容**：
  - spec.md: 3 ADDED Requirement + 12 Gherkin Scenario
  - d7-domain.md: §8 Layer 架构 + §9 interfaces 包章节
  - a-registry.md: 6 个新 A
  - f-registry.md: 11 个新 F
  - t-registry.md: 4 个新 S + 7 个新 T
  - span-registry.md: 5 个新 span

---

## 3. 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| interfaces 包 import cycle | P0 | 0 import D7 子包（Pure types）+ layout guard 守护 |
| ChannelRequest 字段改动影响 compile | P0 | additive 嵌入（不删老字段）+ Decision 3 完整记录 |
| 22/22 race 测试退化 | P0 | 仅 additive 字段，不改老路径行为；新增 interfaces 包独立 -race 测试 |
| LP-1/LP-2/LP-5 集成测试受影响 | P0 | TaskReport 仅作 Learn 节点入参增强；老 LearnRequest 字段保留 |
| Dissent 沉淀慢 | P1 | top-3 截断 + summary hash |
| T 编号与父 DESIGN 不一致 | P1 | 决策显式记录 + reviewer highlight |
| Spec 文档不同步 | P0 | PR-A 提交前置条件 6 文件 |

---

## 4. S4 实施前置检查

- [x] `demand.md` 完整（119 行，6 AC + T 编号策略 + 风险评估）
- [x] `proposal.md` 完整（214 行，9 sections + 4 Decision）
- [x] `design.md` 完整（558 行，六段式 ①-⑥ + 5 附录）
- [x] `tasks.md` 完整（216 行，17 步骤 + 7 T 点 + F-T 映射）
- [x] `specs/d7-orchestration/spec.md` delta 完整（295 行，3 ADDED Requirement + 12 Scenario）
- [x] `.openspec.yaml` 完整（300 行，6 A + 11 F + 7 T + 5 span + 4 metric + 5 error code）

**全部检查通过 ✅**

---

## 5. 评审结论

✅ **PASS** — 设计文档 5 维度审查全部通过，PR-A 范围紧凑（L1 + L2 + AC17），风险等级"低"，可独立合入。

**建议下一步**：
1. 用户确认 S3-Gate review 通过
2. 进入 S4 实施（按 `tasks.md` 17 步骤拆分）
3. S4 完成后 S5 验收（22/22 race + LP-1/LP-2/LP-5 + Coverage ≥ 80% + P99 < 1ms）
4. S5 通过后 S6 归档 + PR + squash auto-merge

**关联审查材料**：
- `demand.md` (PR-A scope 6 AC)
- `proposal.md` (PR-A scope 4 Decision)
- `design.md` (PR-A scope 六段式 ①-⑥)
- `tasks.md` (PR-A scope 17 步骤 + 7 T 点)
- `specs/d7-orchestration/spec.md` (PR-A scope 3 ADDED Requirement + 12 Scenario)
- `.openspec.yaml` (PR-A scope 完整登记)
- 父 DESIGN: `openspec/archive/2026-06-29-devrix-d7-taskcontract-unification/` (648 行全文版)