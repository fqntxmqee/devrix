# 需求与架构 Review 规范

**版本:** 1.0.0
**状态:** Active
**所属阶段:** S3-Gate
**适用对象:** 所有 Change 的 proposal、design、spec 文档

---

## 1. 触发条件

以下任一情况发生时，必须进行设计 Review：

- S3 设计阶段完成，design.md 和 specs/*/spec.md 已编写
- 涉及跨域（D 层）变更
- 涉及新模块/新包的创建
- 变更影响 > 3 个文件
- 新增或修改公开接口（`internal/shared/contracts/` 或层间接口）

轻量变更（单一文件修复、配置调整）可跳过，但仍需在 PR 中说明理由。

---

## 2. Review 维度

### 2.1 架构决策审查

| 检查项 | 说明 |
|--------|------|
| 层归属正确 | 新代码放在正确的 D-S 层 |
| 接口方向正确 | 高层不依赖低层实现细节 |
| 不重复造轮子 | 未重复实现已有能力（检查 existing specs） |
| 跨层依赖最小 | 跨层调用通过 `contracts/` 或 `bridges/` |
| 设计决策有记录 | 非平凡选择有 Decision 节记录 |

### 2.2 需求完整性审查

| 检查项 | 说明 |
|--------|------|
| 需求可追溯 | demand.md → proposal.md → design.md → specs 链路完整 |
| 验收标准覆盖 | 每个 P0 验收标准有对应 Scenario |
| Out of Scope 明确 | proposal.md 声明了不做的事 |
| DM ID 无冲突 | 与 `demand-archive-index.md` 交叉检查 |

### 2.3 规格质量审查

| 检查项 | 说明 |
|--------|------|
| Gherkin 格式正确 | GIVEN/WHEN/THEN 结构完整 |
| Happy path 和 sad path | 两者都有对应 Scenario |
| 并发场景覆盖 | 共享状态操作有并发安全 Scenario |
| 错误路径覆盖 | 超时、取消、权限拒绝等有 Scenario |
| T 层映射完整 | 每个 Requirement 标注了 T 层 ID |

### 2.4 风险审查

| 检查项 | 说明 |
|--------|------|
| 回归风险已评估 | design.md 包含回归风险表 |
| 回滚方案可行 | 不可逆变更有回滚计划 |
| 性能影响已评估 | 新增操作有复杂度分析 |

---

## 3. Review 流程

### 3.1 Grill Review（推荐用于架构级变更）

```
1. 逐决策遍历：对每个设计决策问"为什么选 A 不选 B？"
2. 逐依赖确认：每个外部依赖是否必要、版本是否锁定
3. 逐 Scenario 推演：手动走一遍关键 Scenario
4. 记录结论：每个决策点标记 Agreed / Revised / Deferred
```

### 3.2 标准 Review

```
1. 按 §2 四个维度逐项检查
2. 提交 Review 结论
3. 作者修改后 Reviewer 确认
```

---

## 4. Review 结论

| 结论 | 含义 | 后续行动 |
|------|------|---------|
| **Approved** | 无阻塞问题 | 进入 S4 实现 |
| **Approved with Suggestions** | 有建议但非强制 | 可选采纳，进入 S4 |
| **Changes Requested** | 有必须修改的问题 | 修改后重新 Review |
| **Rejected** | 方案不可行 | 回到 S2 重新提案 |

---

## 5. 检查清单

Review 完成时确认：

- [ ] 层归属和接口方向正确
- [ ] 不重复现有能力
- [ ] demand → proposal → design → specs 追溯链完整
- [ ] 所有 P0 验收标准有对应 Scenario
- [ ] Happy path 和 sad path 均有 Scenario
- [ ] 回归风险已评估
- [ ] S3-Gate Review 结论已记录在 design.md **附录 D**
- [ ] Review 结论明确（Approved / Changes Requested）
